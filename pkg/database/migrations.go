package database

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/alim08/fin_line/pkg/logger"
	"go.uber.org/zap"
)

// Migration represents a database migration
type Migration struct {
	Version     int
	Description string
	UpSQL       string
	DownSQL     string
}

// Migrations holds all database migrations
var Migrations = []Migration{
	{
		Version:     1,
		Description: "Create initial schema",
		UpSQL: `
			-- Create quotes table
			CREATE TABLE IF NOT EXISTS quotes (
				id BIGSERIAL PRIMARY KEY,
				ticker VARCHAR(10) NOT NULL,
				price DECIMAL(20,8) NOT NULL CHECK (price > 0),
				timestamp BIGINT NOT NULL,
				sector VARCHAR(50) NOT NULL DEFAULT 'unknown',
				created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
				updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
			);

			-- Create indexes for performance
			CREATE INDEX IF NOT EXISTS idx_quotes_ticker ON quotes(ticker);
			CREATE INDEX IF NOT EXISTS idx_quotes_timestamp ON quotes(timestamp);
			CREATE INDEX IF NOT EXISTS idx_quotes_sector ON quotes(sector);
			CREATE INDEX IF NOT EXISTS idx_quotes_ticker_timestamp ON quotes(ticker, timestamp DESC);

			-- Create anomalies table
			CREATE TABLE IF NOT EXISTS anomalies (
				id BIGSERIAL PRIMARY KEY,
				ticker VARCHAR(10) NOT NULL,
				price DECIMAL(20,8) NOT NULL CHECK (price > 0),
				z_score DECIMAL(10,4) NOT NULL CHECK (z_score >= 0),
				timestamp BIGINT NOT NULL,
				created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
			);

			-- Create indexes for anomalies
			CREATE INDEX IF NOT EXISTS idx_anomalies_ticker ON anomalies(ticker);
			CREATE INDEX IF NOT EXISTS idx_anomalies_timestamp ON anomalies(timestamp);
			CREATE INDEX IF NOT EXISTS idx_anomalies_z_score ON anomalies(z_score);
			CREATE INDEX IF NOT EXISTS idx_anomalies_ticker_timestamp ON anomalies(ticker, timestamp DESC);

			-- Create raw_events table for audit trail
			CREATE TABLE IF NOT EXISTS raw_events (
				id BIGSERIAL PRIMARY KEY,
				source VARCHAR(100) NOT NULL,
				symbol VARCHAR(10) NOT NULL,
				price DECIMAL(20,8) NOT NULL CHECK (price > 0),
				timestamp TIMESTAMP WITH TIME ZONE NOT NULL,
				created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
			);

			-- Create indexes for raw_events
			CREATE INDEX IF NOT EXISTS idx_raw_events_source ON raw_events(source);
			CREATE INDEX IF NOT EXISTS idx_raw_events_symbol ON raw_events(symbol);
			CREATE INDEX IF NOT EXISTS idx_raw_events_timestamp ON raw_events(timestamp);

			-- Create sectors table for reference data
			CREATE TABLE IF NOT EXISTS sectors (
				id SERIAL PRIMARY KEY,
				name VARCHAR(50) UNIQUE NOT NULL,
				description TEXT,
				created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
			);

			-- Insert default sectors
			INSERT INTO sectors (name, description) VALUES
				('crypto', 'Cryptocurrency assets'),
				('stocks', 'Stock market securities'),
				('forex', 'Foreign exchange markets'),
				('commodities', 'Commodity markets'),
				('unknown', 'Unknown or unclassified assets')
			ON CONFLICT (name) DO NOTHING;

			-- Create tickers table for reference data
			CREATE TABLE IF NOT EXISTS tickers (
				id SERIAL PRIMARY KEY,
				symbol VARCHAR(10) UNIQUE NOT NULL,
				name VARCHAR(100),
				sector_id INTEGER REFERENCES sectors(id),
				active BOOLEAN DEFAULT TRUE,
				created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
				updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
			);

			-- Create indexes for tickers
			CREATE INDEX IF NOT EXISTS idx_tickers_symbol ON tickers(symbol);
			CREATE INDEX IF NOT EXISTS idx_tickers_sector_id ON tickers(sector_id);
			CREATE INDEX IF NOT EXISTS idx_tickers_active ON tickers(active);

			-- Create latest_quotes view for efficient latest quote retrieval
			CREATE OR REPLACE VIEW latest_quotes AS
			SELECT DISTINCT ON (ticker) 
				ticker,
				price,
				timestamp,
				sector,
				created_at
			FROM quotes
			ORDER BY ticker, timestamp DESC;

			-- Create function to update updated_at timestamp
			CREATE OR REPLACE FUNCTION update_updated_at_column()
			RETURNS TRIGGER AS $$
			BEGIN
				NEW.updated_at = NOW();
				RETURN NEW;
			END;
			$$ language 'plpgsql';

			-- Create triggers for updated_at
			CREATE TRIGGER update_quotes_updated_at BEFORE UPDATE ON quotes
				FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

			CREATE TRIGGER update_tickers_updated_at BEFORE UPDATE ON tickers
				FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();
		`,
		DownSQL: `
			DROP TRIGGER IF EXISTS update_tickers_updated_at ON tickers;
			DROP TRIGGER IF EXISTS update_quotes_updated_at ON quotes;
			DROP FUNCTION IF EXISTS update_updated_at_column();
			DROP VIEW IF EXISTS latest_quotes;
			DROP TABLE IF EXISTS tickers;
			DROP TABLE IF EXISTS sectors;
			DROP TABLE IF EXISTS raw_events;
			DROP TABLE IF EXISTS anomalies;
			DROP TABLE IF EXISTS quotes;
		`,
	},
	{
		Version:     2,
		Description: "Add partitioning for quotes table",
		UpSQL: `
			-- Create partitioned quotes table
			CREATE TABLE IF NOT EXISTS quotes_partitioned (
				LIKE quotes INCLUDING ALL
			) PARTITION BY RANGE (timestamp);

			-- Create partitions for current month and next month
			CREATE TABLE IF NOT EXISTS quotes_2024_01 PARTITION OF quotes_partitioned
				FOR VALUES FROM (1704067200000) TO (1706745600000);
			CREATE TABLE IF NOT EXISTS quotes_2024_02 PARTITION OF quotes_partitioned
				FOR VALUES FROM (1706745600000) TO (1709251200000);

			-- Create indexes on partitions
			CREATE INDEX IF NOT EXISTS idx_quotes_2024_01_ticker ON quotes_2024_01(ticker);
			CREATE INDEX IF NOT EXISTS idx_quotes_2024_01_timestamp ON quotes_2024_01(timestamp);
			CREATE INDEX IF NOT EXISTS idx_quotes_2024_02_ticker ON quotes_2024_02(ticker);
			CREATE INDEX IF NOT EXISTS idx_quotes_2024_02_timestamp ON quotes_2024_02(timestamp);
		`,
		DownSQL: `
			DROP TABLE IF EXISTS quotes_2024_02;
			DROP TABLE IF EXISTS quotes_2024_01;
			DROP TABLE IF EXISTS quotes_partitioned;
		`,
	},
}

// MigrationStatus represents the status of a migration
type MigrationStatus struct {
	Version     int       `json:"version"`
	Applied     bool      `json:"applied"`
	AppliedAt   time.Time `json:"applied_at,omitempty"`
	Description string    `json:"description"`
}

// RunMigrations runs all pending database migrations
func (db *DB) RunMigrations(ctx context.Context) error {
	logger.Log.Info("starting database migrations")

	// Create migrations table if it doesn't exist
	if err := db.createMigrationsTable(ctx); err != nil {
		return fmt.Errorf("failed to create migrations table: %w", err)
	}

	// Get applied migrations
	applied, err := db.getAppliedMigrations(ctx)
	if err != nil {
		return fmt.Errorf("failed to get applied migrations: %w", err)
	}

	// Run pending migrations
	for _, migration := range Migrations {
		if applied[migration.Version] {
			logger.Log.Debug("migration already applied", zap.Int("version", migration.Version))
			continue
		}

		logger.Log.Info("applying migration", 
			zap.Int("version", migration.Version),
			zap.String("description", migration.Description))

		if err := db.applyMigration(ctx, migration); err != nil {
			return fmt.Errorf("failed to apply migration %d: %w", migration.Version, err)
		}

		logger.Log.Info("migration applied successfully", zap.Int("version", migration.Version))
	}

	logger.Log.Info("database migrations completed")
	return nil
}

// createMigrationsTable creates the migrations tracking table
func (db *DB) createMigrationsTable(ctx context.Context) error {
	query := `
		CREATE TABLE IF NOT EXISTS migrations (
			version INTEGER PRIMARY KEY,
			description TEXT NOT NULL,
			applied_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
		);
	`
	_, err := db.ExecContext(ctx, query)
	return err
}

// getAppliedMigrations returns a map of applied migration versions
func (db *DB) getAppliedMigrations(ctx context.Context) (map[int]bool, error) {
	query := `SELECT version FROM migrations ORDER BY version`
	rows, err := db.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	applied := make(map[int]bool)
	for rows.Next() {
		var version int
		if err := rows.Scan(&version); err != nil {
			return nil, err
		}
		applied[version] = true
	}

	return applied, rows.Err()
}

// applyMigration applies a single migration
func (db *DB) applyMigration(ctx context.Context, migration Migration) error {
	// Start transaction
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// Execute migration SQL
	if _, err := tx.ExecContext(ctx, migration.UpSQL); err != nil {
		return fmt.Errorf("failed to execute migration SQL: %w", err)
	}

	// Record migration as applied
	query := `INSERT INTO migrations (version, description) VALUES ($1, $2)`
	if _, err := tx.ExecContext(ctx, query, migration.Version, migration.Description); err != nil {
		return fmt.Errorf("failed to record migration: %w", err)
	}

	// Commit transaction
	return tx.Commit()
}

// GetMigrationStatus returns the status of all migrations
func (db *DB) GetMigrationStatus(ctx context.Context) ([]MigrationStatus, error) {
	applied, err := db.getAppliedMigrations(ctx)
	if err != nil {
		return nil, err
	}

	var status []MigrationStatus
	for _, migration := range Migrations {
		ms := MigrationStatus{
			Version:     migration.Version,
			Applied:     applied[migration.Version],
			Description: migration.Description,
		}

		if ms.Applied {
			// Get applied timestamp
			query := `SELECT applied_at FROM migrations WHERE version = $1`
			var appliedAt time.Time
			if err := db.QueryRowContext(ctx, query, migration.Version).Scan(&appliedAt); err == nil {
				ms.AppliedAt = appliedAt
			}
		}

		status = append(status, ms)
	}

	return status, nil
}

// RollbackMigration rolls back the last applied migration
func (db *DB) RollbackMigration(ctx context.Context) error {
	// Get the last applied migration
	query := `SELECT version, description FROM migrations ORDER BY version DESC LIMIT 1`
	var version int
	var description string
	err := db.QueryRowContext(ctx, query).Scan(&version, &description)
	if err != nil {
		return fmt.Errorf("no migrations to rollback: %w", err)
	}

	// Find the migration
	var migration Migration
	for _, m := range Migrations {
		if m.Version == version {
			migration = m
			break
		}
	}

	if migration.Version == 0 {
		return fmt.Errorf("migration version %d not found", version)
	}

	logger.Log.Info("rolling back migration", 
		zap.Int("version", version),
		zap.String("description", description))

	// Start transaction
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// Execute rollback SQL
	if migration.DownSQL != "" {
		if _, err := tx.ExecContext(ctx, migration.DownSQL); err != nil {
			return fmt.Errorf("failed to execute rollback SQL: %w", err)
		}
	}

	// Remove migration record
	query = `DELETE FROM migrations WHERE version = $1`
	if _, err := tx.ExecContext(ctx, query, version); err != nil {
		return fmt.Errorf("failed to remove migration record: %w", err)
	}

	// Commit transaction
	return tx.Commit()
} 