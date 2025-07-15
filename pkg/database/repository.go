package database

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/alim08/fin_line/pkg/models"
	"github.com/alim08/fin_line/pkg/metrics"
	"github.com/alim08/fin_line/pkg/logger"
	"go.uber.org/zap"
)

// QuoteRepository defines the interface for quote data access
type QuoteRepository interface {
	SaveQuote(ctx context.Context, quote *models.NormalizedTick) error
	GetLatestQuotes(ctx context.Context) ([]*models.NormalizedTick, error)
	GetQuotesByTicker(ctx context.Context, ticker string, limit int) ([]*models.NormalizedTick, error)
	GetQuotesBySector(ctx context.Context, sector string, limit int) ([]*models.NormalizedTick, error)
	GetQuotesByTimeRange(ctx context.Context, ticker string, start, end int64) ([]*models.NormalizedTick, error)
	GetQuoteStats(ctx context.Context) (*QuoteStats, error)
}

// AnomalyRepository defines the interface for anomaly data access
type AnomalyRepository interface {
	SaveAnomaly(ctx context.Context, anomaly *models.Anomaly) error
	GetAnomaliesByTicker(ctx context.Context, ticker string, limit int) ([]*models.Anomaly, error)
	GetAnomaliesByTimeRange(ctx context.Context, start, end int64) ([]*models.Anomaly, error)
	GetAnomaliesByZScore(ctx context.Context, minZScore float64, limit int) ([]*models.Anomaly, error)
}

// RawEventRepository defines the interface for raw event data access
type RawEventRepository interface {
	SaveRawEvent(ctx context.Context, event *models.RawTick) error
	GetRawEventsBySource(ctx context.Context, source string, limit int) ([]*models.RawTick, error)
	GetRawEventsByTimeRange(ctx context.Context, start, end time.Time) ([]*models.RawTick, error)
}

// QuoteStats represents statistics about quotes
type QuoteStats struct {
	TotalQuotes   int64     `json:"total_quotes"`
	TotalTickers  int64     `json:"total_tickers"`
	LastUpdate    time.Time `json:"last_update"`
	AvgPrice      float64   `json:"avg_price"`
	TotalSectors  int64     `json:"total_sectors"`
}

// quoteRepository implements QuoteRepository
type quoteRepository struct {
	db *DB
}

// NewQuoteRepository creates a new quote repository
func NewQuoteRepository(db *DB) QuoteRepository {
	return &quoteRepository{db: db}
}

// SaveQuote saves a quote to the database
func (r *quoteRepository) SaveQuote(ctx context.Context, quote *models.NormalizedTick) error {
	start := time.Now()
	defer func() {
		metrics.DatabaseOperationDuration.WithLabelValues("save_quote", "success").Observe(time.Since(start).Seconds())
	}()

	// Sanitize and validate the quote
	quote.Sanitize()
	if err := quote.Validate(); err != nil {
		metrics.DatabaseOperationDuration.WithLabelValues("save_quote", "validation_error").Observe(time.Since(start).Seconds())
		return fmt.Errorf("quote validation failed: %w", err)
	}

	query := `
		INSERT INTO quotes (ticker, price, timestamp, sector)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (ticker, timestamp) DO UPDATE SET
			price = EXCLUDED.price,
			sector = EXCLUDED.sector,
			updated_at = NOW()
	`

	_, err := r.db.ExecContext(ctx, query, quote.Ticker, quote.Price, quote.Timestamp, quote.Sector)
	if err != nil {
		metrics.DatabaseOperationDuration.WithLabelValues("save_quote", "error").Observe(time.Since(start).Seconds())
		metrics.DatabaseErrors.WithLabelValues("save_quote").Inc()
		return fmt.Errorf("failed to save quote: %w", err)
	}

	metrics.DatabaseOperations.WithLabelValues("save_quote", "success").Inc()
	return nil
}

// GetLatestQuotes retrieves the latest quote for each ticker
func (r *quoteRepository) GetLatestQuotes(ctx context.Context) ([]*models.NormalizedTick, error) {
	start := time.Now()
	defer func() {
		metrics.DatabaseOperationDuration.WithLabelValues("get_latest_quotes", "success").Observe(time.Since(start).Seconds())
	}()

	query := `
		SELECT ticker, price, timestamp, sector
		FROM latest_quotes
		ORDER BY ticker
	`

	rows, err := r.db.QueryContext(ctx, query)
	if err != nil {
		metrics.DatabaseOperationDuration.WithLabelValues("get_latest_quotes", "error").Observe(time.Since(start).Seconds())
		metrics.DatabaseErrors.WithLabelValues("get_latest_quotes").Inc()
		return nil, fmt.Errorf("failed to get latest quotes: %w", err)
	}
	defer rows.Close()

	var quotes []*models.NormalizedTick
	for rows.Next() {
		var quote models.NormalizedTick
		if err := rows.Scan(&quote.Ticker, &quote.Price, &quote.Timestamp, &quote.Sector); err != nil {
			return nil, fmt.Errorf("failed to scan quote: %w", err)
		}
		quotes = append(quotes, &quote)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating quotes: %w", err)
	}

	metrics.DatabaseOperations.WithLabelValues("get_latest_quotes", "success").Inc()
	return quotes, nil
}

// GetQuotesByTicker retrieves quotes for a specific ticker
func (r *quoteRepository) GetQuotesByTicker(ctx context.Context, ticker string, limit int) ([]*models.NormalizedTick, error) {
	start := time.Now()
	defer func() {
		metrics.DatabaseOperationDuration.WithLabelValues("get_quotes_by_ticker", "success").Observe(time.Since(start).Seconds())
	}()

	if limit <= 0 || limit > 1000 {
		limit = 100
	}

	query := `
		SELECT ticker, price, timestamp, sector
		FROM quotes
		WHERE ticker = $1
		ORDER BY timestamp DESC
		LIMIT $2
	`

	rows, err := r.db.QueryContext(ctx, query, ticker, limit)
	if err != nil {
		metrics.DatabaseOperationDuration.WithLabelValues("get_quotes_by_ticker", "error").Observe(time.Since(start).Seconds())
		metrics.DatabaseErrors.WithLabelValues("get_quotes_by_ticker").Inc()
		return nil, fmt.Errorf("failed to get quotes by ticker: %w", err)
	}
	defer rows.Close()

	var quotes []*models.NormalizedTick
	for rows.Next() {
		var quote models.NormalizedTick
		if err := rows.Scan(&quote.Ticker, &quote.Price, &quote.Timestamp, &quote.Sector); err != nil {
			return nil, fmt.Errorf("failed to scan quote: %w", err)
		}
		quotes = append(quotes, &quote)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating quotes: %w", err)
	}

	metrics.DatabaseOperations.WithLabelValues("get_quotes_by_ticker", "success").Inc()
	return quotes, nil
}

// GetQuotesBySector retrieves quotes for a specific sector
func (r *quoteRepository) GetQuotesBySector(ctx context.Context, sector string, limit int) ([]*models.NormalizedTick, error) {
	start := time.Now()
	defer func() {
		metrics.DatabaseOperationDuration.WithLabelValues("get_quotes_by_sector", "success").Observe(time.Since(start).Seconds())
	}()

	if limit <= 0 || limit > 1000 {
		limit = 100
	}

	query := `
		SELECT ticker, price, timestamp, sector
		FROM quotes
		WHERE sector = $1
		ORDER BY timestamp DESC
		LIMIT $2
	`

	rows, err := r.db.QueryContext(ctx, query, sector, limit)
	if err != nil {
		metrics.DatabaseOperationDuration.WithLabelValues("get_quotes_by_sector", "error").Observe(time.Since(start).Seconds())
		metrics.DatabaseErrors.WithLabelValues("get_quotes_by_sector").Inc()
		return nil, fmt.Errorf("failed to get quotes by sector: %w", err)
	}
	defer rows.Close()

	var quotes []*models.NormalizedTick
	for rows.Next() {
		var quote models.NormalizedTick
		if err := rows.Scan(&quote.Ticker, &quote.Price, &quote.Timestamp, &quote.Sector); err != nil {
			return nil, fmt.Errorf("failed to scan quote: %w", err)
		}
		quotes = append(quotes, &quote)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating quotes: %w", err)
	}

	metrics.DatabaseOperations.WithLabelValues("get_quotes_by_sector", "success").Inc()
	return quotes, nil
}

// GetQuotesByTimeRange retrieves quotes within a time range
func (r *quoteRepository) GetQuotesByTimeRange(ctx context.Context, ticker string, start, end int64) ([]*models.NormalizedTick, error) {
	startTime := time.Now()
	defer func() {
		metrics.DatabaseOperationDuration.WithLabelValues("get_quotes_by_time_range", "success").Observe(time.Since(startTime).Seconds())
	}()

	query := `
		SELECT ticker, price, timestamp, sector
		FROM quotes
		WHERE ticker = $1 AND timestamp BETWEEN $2 AND $3
		ORDER BY timestamp ASC
	`

	rows, err := r.db.QueryContext(ctx, query, ticker, start, end)
	if err != nil {
		metrics.DatabaseOperationDuration.WithLabelValues("get_quotes_by_time_range", "error").Observe(time.Since(startTime).Seconds())
		metrics.DatabaseErrors.WithLabelValues("get_quotes_by_time_range").Inc()
		return nil, fmt.Errorf("failed to get quotes by time range: %w", err)
	}
	defer rows.Close()

	var quotes []*models.NormalizedTick
	for rows.Next() {
		var quote models.NormalizedTick
		if err := rows.Scan(&quote.Ticker, &quote.Price, &quote.Timestamp, &quote.Sector); err != nil {
			return nil, fmt.Errorf("failed to scan quote: %w", err)
		}
		quotes = append(quotes, &quote)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating quotes: %w", err)
	}

	metrics.DatabaseOperations.WithLabelValues("get_quotes_by_time_range", "success").Inc()
	return quotes, nil
}

// GetQuoteStats retrieves statistics about quotes
func (r *quoteRepository) GetQuoteStats(ctx context.Context) (*QuoteStats, error) {
	start := time.Now()
	defer func() {
		metrics.DatabaseOperationDuration.WithLabelValues("get_quote_stats", "success").Observe(time.Since(start).Seconds())
	}()

	query := `
		SELECT 
			COUNT(*) as total_quotes,
			COUNT(DISTINCT ticker) as total_tickers,
			MAX(created_at) as last_update,
			AVG(price) as avg_price,
			COUNT(DISTINCT sector) as total_sectors
		FROM quotes
	`

	var stats QuoteStats
	err := r.db.QueryRowContext(ctx, query).Scan(
		&stats.TotalQuotes,
		&stats.TotalTickers,
		&stats.LastUpdate,
		&stats.AvgPrice,
		&stats.TotalSectors,
	)
	if err != nil {
		metrics.DatabaseOperationDuration.WithLabelValues("get_quote_stats", "error").Observe(time.Since(start).Seconds())
		metrics.DatabaseErrors.WithLabelValues("get_quote_stats").Inc()
		return nil, fmt.Errorf("failed to get quote stats: %w", err)
	}

	metrics.DatabaseOperations.WithLabelValues("get_quote_stats", "success").Inc()
	return &stats, nil
}

// anomalyRepository implements AnomalyRepository
type anomalyRepository struct {
	db *DB
}

// NewAnomalyRepository creates a new anomaly repository
func NewAnomalyRepository(db *DB) AnomalyRepository {
	return &anomalyRepository{db: db}
}

// SaveAnomaly saves an anomaly to the database
func (r *anomalyRepository) SaveAnomaly(ctx context.Context, anomaly *models.Anomaly) error {
	start := time.Now()
	defer func() {
		metrics.DatabaseOperationDuration.WithLabelValues("save_anomaly", "success").Observe(time.Since(start).Seconds())
	}()

	// Sanitize and validate the anomaly
	anomaly.Sanitize()
	if err := anomaly.Validate(); err != nil {
		metrics.DatabaseOperationDuration.WithLabelValues("save_anomaly", "validation_error").Observe(time.Since(start).Seconds())
		return fmt.Errorf("anomaly validation failed: %w", err)
	}

	query := `
		INSERT INTO anomalies (ticker, price, z_score, timestamp)
		VALUES ($1, $2, $3, $4)
	`

	_, err := r.db.ExecContext(ctx, query, anomaly.Ticker, anomaly.Price, anomaly.ZScore, anomaly.Timestamp)
	if err != nil {
		metrics.DatabaseOperationDuration.WithLabelValues("save_anomaly", "error").Observe(time.Since(start).Seconds())
		metrics.DatabaseErrors.WithLabelValues("save_anomaly").Inc()
		return fmt.Errorf("failed to save anomaly: %w", err)
	}

	metrics.DatabaseOperations.WithLabelValues("save_anomaly", "success").Inc()
	return nil
}

// GetAnomaliesByTicker retrieves anomalies for a specific ticker
func (r *anomalyRepository) GetAnomaliesByTicker(ctx context.Context, ticker string, limit int) ([]*models.Anomaly, error) {
	start := time.Now()
	defer func() {
		metrics.DatabaseOperationDuration.WithLabelValues("get_anomalies_by_ticker", "success").Observe(time.Since(start).Seconds())
	}()

	if limit <= 0 || limit > 1000 {
		limit = 100
	}

	query := `
		SELECT ticker, price, z_score, timestamp
		FROM anomalies
		WHERE ticker = $1
		ORDER BY timestamp DESC
		LIMIT $2
	`

	rows, err := r.db.QueryContext(ctx, query, ticker, limit)
	if err != nil {
		metrics.DatabaseOperationDuration.WithLabelValues("get_anomalies_by_ticker", "error").Observe(time.Since(start).Seconds())
		metrics.DatabaseErrors.WithLabelValues("get_anomalies_by_ticker").Inc()
		return nil, fmt.Errorf("failed to get anomalies by ticker: %w", err)
	}
	defer rows.Close()

	var anomalies []*models.Anomaly
	for rows.Next() {
		var anomaly models.Anomaly
		if err := rows.Scan(&anomaly.Ticker, &anomaly.Price, &anomaly.ZScore, &anomaly.Timestamp); err != nil {
			return nil, fmt.Errorf("failed to scan anomaly: %w", err)
		}
		anomalies = append(anomalies, &anomaly)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating anomalies: %w", err)
	}

	metrics.DatabaseOperations.WithLabelValues("get_anomalies_by_ticker", "success").Inc()
	return anomalies, nil
}

// GetAnomaliesByTimeRange retrieves anomalies within a time range
func (r *anomalyRepository) GetAnomaliesByTimeRange(ctx context.Context, start, end int64) ([]*models.Anomaly, error) {
	startTime := time.Now()
	defer func() {
		metrics.DatabaseOperationDuration.WithLabelValues("get_anomalies_by_time_range", "success").Observe(time.Since(startTime).Seconds())
	}()

	query := `
		SELECT ticker, price, z_score, timestamp
		FROM anomalies
		WHERE timestamp BETWEEN $1 AND $2
		ORDER BY timestamp DESC
	`

	rows, err := r.db.QueryContext(ctx, query, start, end)
	if err != nil {
		metrics.DatabaseOperationDuration.WithLabelValues("get_anomalies_by_time_range", "error").Observe(time.Since(startTime).Seconds())
		metrics.DatabaseErrors.WithLabelValues("get_anomalies_by_time_range").Inc()
		return nil, fmt.Errorf("failed to get anomalies by time range: %w", err)
	}
	defer rows.Close()

	var anomalies []*models.Anomaly
	for rows.Next() {
		var anomaly models.Anomaly
		if err := rows.Scan(&anomaly.Ticker, &anomaly.Price, &anomaly.ZScore, &anomaly.Timestamp); err != nil {
			return nil, fmt.Errorf("failed to scan anomaly: %w", err)
		}
		anomalies = append(anomalies, &anomaly)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating anomalies: %w", err)
	}

	metrics.DatabaseOperations.WithLabelValues("get_anomalies_by_time_range", "success").Inc()
	return anomalies, nil
}

// GetAnomaliesByZScore retrieves anomalies with z-score above threshold
func (r *anomalyRepository) GetAnomaliesByZScore(ctx context.Context, minZScore float64, limit int) ([]*models.Anomaly, error) {
	start := time.Now()
	defer func() {
		metrics.DatabaseOperationDuration.WithLabelValues("get_anomalies_by_zscore", "success").Observe(time.Since(start).Seconds())
	}()

	if limit <= 0 || limit > 1000 {
		limit = 100
	}

	query := `
		SELECT ticker, price, z_score, timestamp
		FROM anomalies
		WHERE z_score >= $1
		ORDER BY z_score DESC, timestamp DESC
		LIMIT $2
	`

	rows, err := r.db.QueryContext(ctx, query, minZScore, limit)
	if err != nil {
		metrics.DatabaseOperationDuration.WithLabelValues("get_anomalies_by_zscore", "error").Observe(time.Since(start).Seconds())
		metrics.DatabaseErrors.WithLabelValues("get_anomalies_by_zscore").Inc()
		return nil, fmt.Errorf("failed to get anomalies by z-score: %w", err)
	}
	defer rows.Close()

	var anomalies []*models.Anomaly
	for rows.Next() {
		var anomaly models.Anomaly
		if err := rows.Scan(&anomaly.Ticker, &anomaly.Price, &anomaly.ZScore, &anomaly.Timestamp); err != nil {
			return nil, fmt.Errorf("failed to scan anomaly: %w", err)
		}
		anomalies = append(anomalies, &anomaly)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating anomalies: %w", err)
	}

	metrics.DatabaseOperations.WithLabelValues("get_anomalies_by_zscore", "success").Inc()
	return anomalies, nil
}

// rawEventRepository implements RawEventRepository
type rawEventRepository struct {
	db *DB
}

// NewRawEventRepository creates a new raw event repository
func NewRawEventRepository(db *DB) RawEventRepository {
	return &rawEventRepository{db: db}
}

// SaveRawEvent saves a raw event to the database
func (r *rawEventRepository) SaveRawEvent(ctx context.Context, event *models.RawTick) error {
	start := time.Now()
	defer func() {
		metrics.DatabaseOperationDuration.WithLabelValues("save_raw_event", "success").Observe(time.Since(start).Seconds())
	}()

	// Sanitize and validate the event
	event.Sanitize()
	if err := event.Validate(); err != nil {
		metrics.DatabaseOperationDuration.WithLabelValues("save_raw_event", "validation_error").Observe(time.Since(start).Seconds())
		return fmt.Errorf("raw event validation failed: %w", err)
	}

	query := `
		INSERT INTO raw_events (source, symbol, price, timestamp)
		VALUES ($1, $2, $3, $4)
	`

	_, err := r.db.ExecContext(ctx, query, event.Source, event.Symbol, event.Price, event.Timestamp)
	if err != nil {
		metrics.DatabaseOperationDuration.WithLabelValues("save_raw_event", "error").Observe(time.Since(start).Seconds())
		metrics.DatabaseErrors.WithLabelValues("save_raw_event").Inc()
		return fmt.Errorf("failed to save raw event: %w", err)
	}

	metrics.DatabaseOperations.WithLabelValues("save_raw_event", "success").Inc()
	return nil
}

// GetRawEventsBySource retrieves raw events for a specific source
func (r *rawEventRepository) GetRawEventsBySource(ctx context.Context, source string, limit int) ([]*models.RawTick, error) {
	start := time.Now()
	defer func() {
		metrics.DatabaseOperationDuration.WithLabelValues("get_raw_events_by_source", "success").Observe(time.Since(start).Seconds())
	}()

	if limit <= 0 || limit > 1000 {
		limit = 100
	}

	query := `
		SELECT source, symbol, price, timestamp
		FROM raw_events
		WHERE source = $1
		ORDER BY timestamp DESC
		LIMIT $2
	`

	rows, err := r.db.QueryContext(ctx, query, source, limit)
	if err != nil {
		metrics.DatabaseOperationDuration.WithLabelValues("get_raw_events_by_source", "error").Observe(time.Since(start).Seconds())
		metrics.DatabaseErrors.WithLabelValues("get_raw_events_by_source").Inc()
		return nil, fmt.Errorf("failed to get raw events by source: %w", err)
	}
	defer rows.Close()

	var events []*models.RawTick
	for rows.Next() {
		var event models.RawTick
		if err := rows.Scan(&event.Source, &event.Symbol, &event.Price, &event.Timestamp); err != nil {
			return nil, fmt.Errorf("failed to scan raw event: %w", err)
		}
		events = append(events, &event)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating raw events: %w", err)
	}

	metrics.DatabaseOperations.WithLabelValues("get_raw_events_by_source", "success").Inc()
	return events, nil
}

// GetRawEventsByTimeRange retrieves raw events within a time range
func (r *rawEventRepository) GetRawEventsByTimeRange(ctx context.Context, start, end time.Time) ([]*models.RawTick, error) {
	startTime := time.Now()
	defer func() {
		metrics.DatabaseOperationDuration.WithLabelValues("get_raw_events_by_time_range", "success").Observe(time.Since(startTime).Seconds())
	}()

	query := `
		SELECT source, symbol, price, timestamp
		FROM raw_events
		WHERE timestamp BETWEEN $1 AND $2
		ORDER BY timestamp ASC
	`

	rows, err := r.db.QueryContext(ctx, query, start, end)
	if err != nil {
		metrics.DatabaseOperationDuration.WithLabelValues("get_raw_events_by_time_range", "error").Observe(time.Since(startTime).Seconds())
		metrics.DatabaseErrors.WithLabelValues("get_raw_events_by_time_range").Inc()
		return nil, fmt.Errorf("failed to get raw events by time range: %w", err)
	}
	defer rows.Close()

	var events []*models.RawTick
	for rows.Next() {
		var event models.RawTick
		if err := rows.Scan(&event.Source, &event.Symbol, &event.Price, &event.Timestamp); err != nil {
			return nil, fmt.Errorf("failed to scan raw event: %w", err)
		}
		events = append(events, &event)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating raw events: %w", err)
	}

	metrics.DatabaseOperations.WithLabelValues("get_raw_events_by_time_range", "success").Inc()
	return events, nil
} 