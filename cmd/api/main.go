package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/alim08/fin_line/pkg/auth"
	"github.com/alim08/fin_line/pkg/config"
	"github.com/alim08/fin_line/pkg/database"
	"github.com/alim08/fin_line/pkg/logger"
	"github.com/alim08/fin_line/pkg/metrics"
	"github.com/alim08/fin_line/pkg/redisclient"
	"github.com/gorilla/mux"
	"go.uber.org/zap"
)

func main() {
	// Initialize logger
	logger.Init()
	log := logger.Log

	log.Info("starting fin-line API server")

	// Load configuration
	cfg := config.Load()
	log.Info("configuration loaded", zap.String("environment", cfg.Environment))

	// Initialize database
	dbConfig := database.NewConfig()
	db, err := database.New(dbConfig)
	if err != nil {
		log.Fatal("failed to connect to database", zap.Error(err))
	}
	defer db.Close()

	// Run database migrations
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	
	if err := db.RunMigrations(ctx); err != nil {
		log.Fatal("failed to run database migrations", zap.Error(err))
	}
	log.Info("database migrations completed")

	// Initialize repositories
	quoteRepo := database.NewQuoteRepository(db)
	anomalyRepo := database.NewAnomalyRepository(db)
	rawEventRepo := database.NewRawEventRepository(db)

	// Initialize Redis client
	redisClient, err := redisclient.New(cfg.Redis)
	if err != nil {
		log.Fatal("failed to connect to Redis", zap.Error(err))
	}
	defer redisClient.Close()

	// Initialize authentication service
	authConfig := auth.NewConfig()
	authService, err := auth.NewAuthService(authConfig)
	if err != nil {
		log.Fatal("failed to initialize authentication service", zap.Error(err))
	}

	// Create router
	router := mux.NewRouter()

	// Add middleware
	router.Use(loggingMiddleware)
	router.Use(corsMiddleware)
	router.Use(metricsMiddleware)

	// Health check endpoint (no auth required)
	router.HandleFunc("/health", healthHandler(db, redisClient)).Methods("GET")
	router.HandleFunc("/ready", readyHandler(db, redisClient)).Methods("GET")

	// API routes with authentication
	apiRouter := router.PathPrefix("/api/v1").Subrouter()
	
	// Public endpoints (no auth required)
	apiRouter.HandleFunc("/quotes/latest", getLatestQuotesHandler(quoteRepo)).Methods("GET")
	apiRouter.HandleFunc("/quotes/{ticker}", getQuotesByTickerHandler(quoteRepo)).Methods("GET")
	apiRouter.HandleFunc("/stats", getStatsHandler(quoteRepo)).Methods("GET")

	// Protected endpoints (auth required)
	protectedRouter := apiRouter.PathPrefix("").Subrouter()
	protectedRouter.Use(authService.AuthMiddleware)

	// User-level endpoints
	protectedRouter.HandleFunc("/quotes/sector/{sector}", getQuotesBySectorHandler(quoteRepo)).Methods("GET")
	protectedRouter.HandleFunc("/quotes/{ticker}/history", getQuoteHistoryHandler(quoteRepo)).Methods("GET")
	protectedRouter.HandleFunc("/anomalies", getAnomaliesHandler(anomalyRepo)).Methods("GET")
	protectedRouter.HandleFunc("/anomalies/{ticker}", getAnomaliesByTickerHandler(anomalyRepo)).Methods("GET")

	// Admin endpoints (admin role required)
	adminRouter := protectedRouter.PathPrefix("/admin").Subrouter()
	adminRouter.Use(authService.RoleMiddleware("admin"))
	
	adminRouter.HandleFunc("/raw-events", getRawEventsHandler(rawEventRepo)).Methods("GET")
	adminRouter.HandleFunc("/raw-events/source/{source}", getRawEventsBySourceHandler(rawEventRepo)).Methods("GET")
	adminRouter.HandleFunc("/migrations/status", getMigrationStatusHandler(db)).Methods("GET")

	// GraphQL endpoint (auth required)
	graphQLRouter := router.PathPrefix("/graphql").Subrouter()
	graphQLRouter.Use(authService.AuthMiddleware)
	// GraphQL handler setup would go here

	// Metrics endpoint (no auth required)
	router.Handle("/metrics", metrics.Handler())

	// Create HTTP server
	server := &http.Server{
		Addr:         fmt.Sprintf(":%d", cfg.API.Port),
		Handler:      router,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  120 * time.Second,
	}

	// Start server in goroutine
	go func() {
		log.Info("starting HTTP server", zap.String("addr", server.Addr))
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatal("failed to start server", zap.Error(err))
		}
	}()

	// Wait for interrupt signal
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Info("shutting down server...")

	// Graceful shutdown
	ctx, cancel = context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := server.Shutdown(ctx); err != nil {
		log.Error("server forced to shutdown", zap.Error(err))
	}

	log.Info("server exited")
}

// Health check handler
func healthHandler(db *database.DB, redisClient *redisclient.Client) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
		defer cancel()

		// Check database health
		if err := db.HealthCheck(ctx); err != nil {
			http.Error(w, "Database health check failed", http.StatusServiceUnavailable)
			return
		}

		// Check Redis health
		if err := redisClient.Ping(ctx); err != nil {
			http.Error(w, "Redis health check failed", http.StatusServiceUnavailable)
			return
		}

		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"healthy"}`))
	}
}

// Readiness check handler
func readyHandler(db *database.DB, redisClient *redisclient.Client) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
		defer cancel()

		// Check database readiness
		if err := db.HealthCheck(ctx); err != nil {
			http.Error(w, "Database not ready", http.StatusServiceUnavailable)
			return
		}

		// Check Redis readiness
		if err := redisClient.Ping(ctx); err != nil {
			http.Error(w, "Redis not ready", http.StatusServiceUnavailable)
			return
		}

		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"ready"}`))
	}
}

// Latest quotes handler
func getLatestQuotesHandler(quoteRepo database.QuoteRepository) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
		defer cancel()

		quotes, err := quoteRepo.GetLatestQuotes(ctx)
		if err != nil {
			logger.Log.Error("failed to get latest quotes", zap.Error(err))
			http.Error(w, "Internal server error", http.StatusInternalServerError)
			return
		}

		// Return JSON response
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		// JSON marshaling would go here
	}
}

// Quotes by ticker handler
func getQuotesByTickerHandler(quoteRepo database.QuoteRepository) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		vars := mux.Vars(r)
		ticker := vars["ticker"]

		// Validate ticker
		if ticker == "" {
			http.Error(w, "Ticker is required", http.StatusBadRequest)
			return
		}

		ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
		defer cancel()

		quotes, err := quoteRepo.GetQuotesByTicker(ctx, ticker, 100)
		if err != nil {
			logger.Log.Error("failed to get quotes by ticker", zap.Error(err), zap.String("ticker", ticker))
			http.Error(w, "Internal server error", http.StatusInternalServerError)
			return
		}

		// Return JSON response
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		// JSON marshaling would go here
	}
}

// Stats handler
func getStatsHandler(quoteRepo database.QuoteRepository) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
		defer cancel()

		stats, err := quoteRepo.GetQuoteStats(ctx)
		if err != nil {
			logger.Log.Error("failed to get quote stats", zap.Error(err))
			http.Error(w, "Internal server error", http.StatusInternalServerError)
			return
		}

		// Return JSON response
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		// JSON marshaling would go here
	}
}

// Quotes by sector handler
func getQuotesBySectorHandler(quoteRepo database.QuoteRepository) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		vars := mux.Vars(r)
		sector := vars["sector"]

		// Validate sector
		if sector == "" {
			http.Error(w, "Sector is required", http.StatusBadRequest)
			return
		}

		ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
		defer cancel()

		quotes, err := quoteRepo.GetQuotesBySector(ctx, sector, 100)
		if err != nil {
			logger.Log.Error("failed to get quotes by sector", zap.Error(err), zap.String("sector", sector))
			http.Error(w, "Internal server error", http.StatusInternalServerError)
			return
		}

		// Return JSON response
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		// JSON marshaling would go here
	}
}

// Quote history handler
func getQuoteHistoryHandler(quoteRepo database.QuoteRepository) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		vars := mux.Vars(r)
		ticker := vars["ticker"]

		// Parse query parameters for time range
		startStr := r.URL.Query().Get("start")
		endStr := r.URL.Query().Get("end")

		// Validate parameters
		if ticker == "" || startStr == "" || endStr == "" {
			http.Error(w, "Ticker, start, and end parameters are required", http.StatusBadRequest)
			return
		}

		// Parse timestamps (simplified)
		start := int64(0) // Parse from startStr
		end := int64(0)   // Parse from endStr

		ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
		defer cancel()

		quotes, err := quoteRepo.GetQuotesByTimeRange(ctx, ticker, start, end)
		if err != nil {
			logger.Log.Error("failed to get quote history", zap.Error(err), zap.String("ticker", ticker))
			http.Error(w, "Internal server error", http.StatusInternalServerError)
			return
		}

		// Return JSON response
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		// JSON marshaling would go here
	}
}

// Anomalies handler
func getAnomaliesHandler(anomalyRepo database.AnomalyRepository) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Parse query parameters
		minZScoreStr := r.URL.Query().Get("min_zscore")
		limitStr := r.URL.Query().Get("limit")

		// Parse parameters (simplified)
		minZScore := 2.0 // Default threshold
		limit := 100     // Default limit

		ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
		defer cancel()

		anomalies, err := anomalyRepo.GetAnomaliesByZScore(ctx, minZScore, limit)
		if err != nil {
			logger.Log.Error("failed to get anomalies", zap.Error(err))
			http.Error(w, "Internal server error", http.StatusInternalServerError)
			return
		}

		// Return JSON response
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		// JSON marshaling would go here
	}
}

// Anomalies by ticker handler
func getAnomaliesByTickerHandler(anomalyRepo database.AnomalyRepository) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		vars := mux.Vars(r)
		ticker := vars["ticker"]

		// Validate ticker
		if ticker == "" {
			http.Error(w, "Ticker is required", http.StatusBadRequest)
			return
		}

		ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
		defer cancel()

		anomalies, err := anomalyRepo.GetAnomaliesByTicker(ctx, ticker, 100)
		if err != nil {
			logger.Log.Error("failed to get anomalies by ticker", zap.Error(err), zap.String("ticker", ticker))
			http.Error(w, "Internal server error", http.StatusInternalServerError)
			return
		}

		// Return JSON response
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		// JSON marshaling would go here
	}
}

// Raw events handler (admin only)
func getRawEventsHandler(rawEventRepo database.RawEventRepository) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
		defer cancel()

		// Get events from last 24 hours
		end := time.Now()
		start := end.Add(-24 * time.Hour)

		events, err := rawEventRepo.GetRawEventsByTimeRange(ctx, start, end)
		if err != nil {
			logger.Log.Error("failed to get raw events", zap.Error(err))
			http.Error(w, "Internal server error", http.StatusInternalServerError)
			return
		}

		// Return JSON response
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		// JSON marshaling would go here
	}
}

// Raw events by source handler (admin only)
func getRawEventsBySourceHandler(rawEventRepo database.RawEventRepository) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		vars := mux.Vars(r)
		source := vars["source"]

		// Validate source
		if source == "" {
			http.Error(w, "Source is required", http.StatusBadRequest)
			return
		}

		ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
		defer cancel()

		events, err := rawEventRepo.GetRawEventsBySource(ctx, source, 100)
		if err != nil {
			logger.Log.Error("failed to get raw events by source", zap.Error(err), zap.String("source", source))
			http.Error(w, "Internal server error", http.StatusInternalServerError)
			return
		}

		// Return JSON response
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		// JSON marshaling would go here
	}
}

// Migration status handler (admin only)
func getMigrationStatusHandler(db *database.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
		defer cancel()

		status, err := db.GetMigrationStatus(ctx)
		if err != nil {
			logger.Log.Error("failed to get migration status", zap.Error(err))
			http.Error(w, "Internal server error", http.StatusInternalServerError)
			return
		}

		// Return JSON response
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		// JSON marshaling would go here
	}
}

// Middleware functions
func loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		next.ServeHTTP(w, r)
		logger.Log.Info("HTTP request",
			zap.String("method", r.Method),
			zap.String("path", r.URL.Path),
			zap.String("remote_addr", r.RemoteAddr),
			zap.Duration("duration", time.Since(start)))
	})
}

func corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
		
		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}
		
		next.ServeHTTP(w, r)
	})
}

func metricsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		next.ServeHTTP(w, r)
		duration := time.Since(start).Seconds()
		
		metrics.APIRequestDuration.WithLabelValues(r.Method, r.URL.Path, "200").Observe(duration)
		metrics.APIRequestTotal.WithLabelValues(r.Method, r.URL.Path, "200").Inc()
	})
} 