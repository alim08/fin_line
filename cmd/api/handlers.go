package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/alim08/fin_line/pkg/logger"
	"github.com/go-chi/chi/v5"
	"github.com/go-redis/redis/v8"
	"go.uber.org/zap"
)

// Response represents a standard API response
type Response struct {
	Success bool        `json:"success"`
	Data    interface{} `json:"data,omitempty"`
	Error   string      `json:"error,omitempty"`
	Meta    *Meta       `json:"meta,omitempty"`
}

// Meta contains pagination and metadata information
type Meta struct {
	Total    int64 `json:"total"`
	Page     int   `json:"page"`
	PerPage  int   `json:"per_page"`
	HasMore  bool  `json:"has_more"`
	Duration int64 `json:"duration_ms"`
}

// Quote represents a market quote
type Quote struct {
	Ticker    string  `json:"ticker"`
	Price     float64 `json:"price"`
	Timestamp int64   `json:"timestamp"`
	Sector    string  `json:"sector"`
}

// Anomaly represents a detected market anomaly
type Anomaly struct {
	ID        string  `json:"id"`
	Ticker    string  `json:"ticker"`
	Price     float64 `json:"price"`
	Threshold float64 `json:"threshold"`
	Type      string  `json:"type"` // "spike", "drop", "volatility"
	Timestamp int64   `json:"timestamp"`
	Severity  string  `json:"severity"` // "low", "medium", "high"
}

// MarketStats represents market statistics
type MarketStats struct {
	TotalTickers int     `json:"total_tickers"`
	TotalQuotes  int64   `json:"total_quotes"`
	AvgPrice     float64 `json:"avg_price"`
	LastUpdate   int64   `json:"last_update"`
}

// writeJSON writes a JSON response with proper headers
func (s *Server) writeJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(data); err != nil {
		logger.Log.Error("JSON encoding error", zap.Error(err))
		http.Error(w, "Internal server error", http.StatusInternalServerError)
	}
}

// writeError writes an error response
func (s *Server) writeError(w http.ResponseWriter, status int, message string) {
	s.writeJSON(w, status, Response{
		Success: false,
		Error:   message,
	})
}

// healthHandler returns server health status
func (s *Server) healthHandler(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	// Check Redis connection
	_, err := s.redis.Client().Ping(ctx).Result()
	if err != nil {
		s.writeError(w, http.StatusServiceUnavailable, "Redis connection failed")
		return
	}

	s.writeJSON(w, http.StatusOK, Response{
		Success: true,
		Data: map[string]interface{}{
			"status":    "healthy",
			"timestamp": time.Now().Unix(),
			"version":   "1.0.0",
		},
	})
}

// getQuotesHandler retrieves quotes with pagination and filtering
func (s *Server) getQuotesHandler(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	ctx := r.Context()

	// Parse query parameters
	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	if page < 1 {
		page = 1
	}
	perPage, _ := strconv.Atoi(r.URL.Query().Get("per_page"))
	if perPage < 1 || perPage > 1000 {
		perPage = 100
	}
	ticker := r.URL.Query().Get("ticker")
	sector := r.URL.Query().Get("sector")
	limit := r.URL.Query().Get("limit")
	if limit == "" {
		limit = "100"
	}

	// Build Redis query
	streamKey := "normalized:quotes"
	args := &redis.XReadArgs{
		Streams: []string{streamKey, "0"},
		Count:   int64(perPage),
		Block:   100 * time.Millisecond,
	}

	// Get latest quotes from Redis stream
	streams, err := s.redis.Client().XRead(ctx, args).Result()
	if err != nil && err != redis.Nil {
		logger.Log.Error("Redis XRead error", zap.Error(err))
		s.writeError(w, http.StatusInternalServerError, "Failed to retrieve quotes")
		return
	}

	var quotes []Quote
	if len(streams) > 0 && len(streams[0].Messages) > 0 {
		for _, msg := range streams[0].Messages {
			// Parse the message values
			tickerVal, _ := msg.Values["ticker"].(string)
			priceStr, _ := msg.Values["price"].(string)
			tsMs, _ := msg.Values["ts_ms"].(string)
			sectorVal, _ := msg.Values["sector"].(string)

			// Apply filters
			if ticker != "" && tickerVal != ticker {
				continue
			}
			if sector != "" && sectorVal != sector {
				continue
			}

			price, _ := strconv.ParseFloat(priceStr, 64)
			timestamp, _ := strconv.ParseInt(tsMs, 10, 64)

			quotes = append(quotes, Quote{
				Ticker:    tickerVal,
				Price:     price,
				Timestamp: timestamp,
				Sector:    sectorVal,
			})
		}
	}

	// Calculate metadata
	duration := time.Since(start).Milliseconds()
	total := int64(len(quotes))
	hasMore := total >= int64(perPage)

	s.writeJSON(w, http.StatusOK, Response{
		Success: true,
		Data:    quotes,
		Meta: &Meta{
			Total:    total,
			Page:     page,
			PerPage:  perPage,
			HasMore:  hasMore,
			Duration: duration,
		},
	})
}

// getQuoteByTickerHandler retrieves the latest quote for a specific ticker
func (s *Server) getQuoteByTickerHandler(w http.ResponseWriter, r *http.Request) {
	ticker := chi.URLParam(r, "ticker")
	if ticker == "" {
		s.writeError(w, http.StatusBadRequest, "Ticker parameter is required")
		return
	}

	ctx := r.Context()

	// Get the latest quote for this ticker from Redis
	streamKey := "normalized:quotes"
	args := &redis.XReadArgs{
		Streams: []string{streamKey, "0"},
		Count:   1000, // Get more to filter
		Block:   100 * time.Millisecond,
	}

	streams, err := s.redis.Client().XRead(ctx, args).Result()
	if err != nil && err != redis.Nil {
		logger.Log.Error("Redis XRead error", zap.Error(err))
		s.writeError(w, http.StatusInternalServerError, "Failed to retrieve quote")
		return
	}

	var latestQuote *Quote
	if len(streams) > 0 && len(streams[0].Messages) > 0 {
		// Find the latest message for this ticker
		for i := len(streams[0].Messages) - 1; i >= 0; i-- {
			msg := streams[0].Messages[i]
			tickerVal, _ := msg.Values["ticker"].(string)
			
			if tickerVal == ticker {
				priceStr, _ := msg.Values["price"].(string)
				tsMs, _ := msg.Values["ts_ms"].(string)
				sectorVal, _ := msg.Values["sector"].(string)

				price, _ := strconv.ParseFloat(priceStr, 64)
				timestamp, _ := strconv.ParseInt(tsMs, 10, 64)

				latestQuote = &Quote{
					Ticker:    tickerVal,
					Price:     price,
					Timestamp: timestamp,
					Sector:    sectorVal,
				}
				break
			}
		}
	}

	if latestQuote == nil {
		s.writeError(w, http.StatusNotFound, fmt.Sprintf("No quote found for ticker: %s", ticker))
		return
	}

	s.writeJSON(w, http.StatusOK, Response{
		Success: true,
		Data:    latestQuote,
	})
}

// getLatestQuotesHandler retrieves the latest quotes for all tickers
func (s *Server) getLatestQuotesHandler(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Get latest quotes from Redis
	streamKey := "normalized:quotes"
	args := &redis.XReadArgs{
		Streams: []string{streamKey, "0"},
		Count:   1000,
		Block:   100 * time.Millisecond,
	}

	streams, err := s.redis.Client().XRead(ctx, args).Result()
	if err != nil && err != redis.Nil {
		logger.Log.Error("Redis XRead error", zap.Error(err))
		s.writeError(w, http.StatusInternalServerError, "Failed to retrieve latest quotes")
		return
	}

	// Group by ticker and keep only the latest
	tickerQuotes := make(map[string]*Quote)
	if len(streams) > 0 && len(streams[0].Messages) > 0 {
		for _, msg := range streams[0].Messages {
			tickerVal, _ := msg.Values["ticker"].(string)
			priceStr, _ := msg.Values["price"].(string)
			tsMs, _ := msg.Values["ts_ms"].(string)
			sectorVal, _ := msg.Values["sector"].(string)

			price, _ := strconv.ParseFloat(priceStr, 64)
			timestamp, _ := strconv.ParseInt(tsMs, 10, 64)

			// Only update if this is a newer timestamp
			if existing, exists := tickerQuotes[tickerVal]; !exists || timestamp > existing.Timestamp {
				tickerQuotes[tickerVal] = &Quote{
					Ticker:    tickerVal,
					Price:     price,
					Timestamp: timestamp,
					Sector:    sectorVal,
				}
			}
		}
	}

	// Convert map to slice
	var quotes []*Quote
	for _, quote := range tickerQuotes {
		quotes = append(quotes, quote)
	}

	s.writeJSON(w, http.StatusOK, Response{
		Success: true,
		Data:    quotes,
	})
}

// getAnomaliesHandler retrieves anomalies with filtering
func (s *Server) getAnomaliesHandler(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Parse query parameters
	severity := r.URL.Query().Get("severity")
	anomalyType := r.URL.Query().Get("type")
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	if limit < 1 || limit > 1000 {
		limit = 100
	}

	// Get anomalies from Redis
	anomalies, err := s.redis.Client().LRange(ctx, "anomalies", 0, int64(limit-1)).Result()
	if err != nil && err != redis.Nil {
		logger.Log.Error("Redis LRange error", zap.Error(err))
		s.writeError(w, http.StatusInternalServerError, "Failed to retrieve anomalies")
		return
	}

	var result []Anomaly
	for _, anomalyStr := range anomalies {
		var anomaly Anomaly
		if err := json.Unmarshal([]byte(anomalyStr), &anomaly); err != nil {
			logger.Log.Warn("Failed to unmarshal anomaly", zap.Error(err))
			continue
		}

		// Apply filters
		if severity != "" && anomaly.Severity != severity {
			continue
		}
		if anomalyType != "" && anomaly.Type != anomalyType {
			continue
		}

		result = append(result, anomaly)
	}

	s.writeJSON(w, http.StatusOK, Response{
		Success: true,
		Data:    result,
	})
}

// getAnomaliesByTickerHandler retrieves anomalies for a specific ticker
func (s *Server) getAnomaliesByTickerHandler(w http.ResponseWriter, r *http.Request) {
	ticker := chi.URLParam(r, "ticker")
	if ticker == "" {
		s.writeError(w, http.StatusBadRequest, "Ticker parameter is required")
		return
	}

	ctx := r.Context()

	// Get all anomalies and filter by ticker
	anomalies, err := s.redis.Client().LRange(ctx, "anomalies", 0, -1).Result()
	if err != nil && err != redis.Nil {
		logger.Log.Error("Redis LRange error", zap.Error(err))
		s.writeError(w, http.StatusInternalServerError, "Failed to retrieve anomalies")
		return
	}

	var result []Anomaly
	for _, anomalyStr := range anomalies {
		var anomaly Anomaly
		if err := json.Unmarshal([]byte(anomalyStr), &anomaly); err != nil {
			logger.Log.Warn("Failed to unmarshal anomaly", zap.Error(err))
			continue
		}

		if anomaly.Ticker == ticker {
			result = append(result, anomaly)
		}
	}

	s.writeJSON(w, http.StatusOK, Response{
		Success: true,
		Data:    result,
	})
}

// createAnomalyHandler creates a new anomaly
func (s *Server) createAnomalyHandler(w http.ResponseWriter, r *http.Request) {
	var anomaly Anomaly
	if err := json.NewDecoder(r.Body).Decode(&anomaly); err != nil {
		s.writeError(w, http.StatusBadRequest, "Invalid JSON payload")
		return
	}

	// Validate required fields
	if anomaly.Ticker == "" {
		s.writeError(w, http.StatusBadRequest, "Ticker is required")
		return
	}
	if anomaly.Price <= 0 {
		s.writeError(w, http.StatusBadRequest, "Price must be positive")
		return
	}
	if anomaly.Type == "" {
		s.writeError(w, http.StatusBadRequest, "Type is required")
		return
	}

	// Set default values
	if anomaly.Timestamp == 0 {
		anomaly.Timestamp = time.Now().UnixMilli()
	}
	if anomaly.Severity == "" {
		anomaly.Severity = "medium"
	}
	if anomaly.ID == "" {
		anomaly.ID = fmt.Sprintf("%s_%d", anomaly.Ticker, anomaly.Timestamp)
	}

	ctx := r.Context()

	// Store anomaly in Redis
	anomalyJSON, err := json.Marshal(anomaly)
	if err != nil {
		logger.Log.Error("JSON marshal error", zap.Error(err))
		s.writeError(w, http.StatusInternalServerError, "Failed to create anomaly")
		return
	}

	err = s.redis.Client().LPush(ctx, "anomalies", anomalyJSON).Err()
	if err != nil {
		logger.Log.Error("Redis LPush error", zap.Error(err))
		s.writeError(w, http.StatusInternalServerError, "Failed to store anomaly")
		return
	}

	// Publish to Redis channel for real-time updates
	s.redis.Publish(ctx, "anomalies", anomalyJSON)

	s.writeJSON(w, http.StatusCreated, Response{
		Success: true,
		Data:    anomaly,
	})
}

// getTickersHandler retrieves all available tickers
func (s *Server) getTickersHandler(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Get unique tickers from Redis
	tickers, err := s.redis.Client().SMembers(ctx, "tickers").Result()
	if err != nil && err != redis.Nil {
		logger.Log.Error("Redis SMembers error", zap.Error(err))
		s.writeError(w, http.StatusInternalServerError, "Failed to retrieve tickers")
		return
	}

	s.writeJSON(w, http.StatusOK, Response{
		Success: true,
		Data:    tickers,
	})
}

// getSectorsHandler retrieves all available sectors
func (s *Server) getSectorsHandler(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Get unique sectors from Redis
	sectors, err := s.redis.Client().SMembers(ctx, "sectors").Result()
	if err != nil && err != redis.Nil {
		logger.Log.Error("Redis SMembers error", zap.Error(err))
		s.writeError(w, http.StatusInternalServerError, "Failed to retrieve sectors")
		return
	}

	s.writeJSON(w, http.StatusOK, Response{
		Success: true,
		Data:    sectors,
	})
}

// getMarketStatsHandler retrieves market statistics
func (s *Server) getMarketStatsHandler(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Get basic stats from Redis
	tickers, err := s.redis.Client().SCard(ctx, "tickers").Result()
	if err != nil && err != redis.Nil {
		logger.Log.Error("Redis SCard error", zap.Error(err))
		s.writeError(w, http.StatusInternalServerError, "Failed to retrieve market stats")
		return
	}

	// Get total quotes count (approximate)
	streamLen, err := s.redis.Client().XLen(ctx, "normalized:quotes").Result()
	if err != nil && err != redis.Nil {
		logger.Log.Error("Redis XLen error", zap.Error(err))
		s.writeError(w, http.StatusInternalServerError, "Failed to retrieve market stats")
		return
	}

	stats := MarketStats{
		TotalTickers: int(tickers),
		TotalQuotes:  streamLen,
		LastUpdate:   time.Now().Unix(),
	}

	s.writeJSON(w, http.StatusOK, Response{
		Success: true,
		Data:    stats,
	})
} 