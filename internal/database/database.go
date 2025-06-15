// Package database handles PostgreSQL connection management and schema operations
// for the TaskQ application. It provides connection pooling, migrations, and
// database health checks.
package database

import (
	"database/sql"
	"fmt"
	"time"

	"github.com/Devashish08/taskq/internal/config"
	_ "github.com/jackc/pgx/v5/stdlib"
)

// ConnectDB establishes a connection to PostgreSQL with optimized pool settings.
// It configures connection pooling parameters suitable for production workloads.
//
// Parameters:
//   - cfg: Database configuration including host, port, credentials, and database name
//
// Returns:
//   - *sql.DB: Database connection pool ready for use
//   - error: Connection or configuration errors
func ConnectDB(cfg *config.DBConfig) (*sql.DB, error) {
	dsn := fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=%s sslmode=disable",
		cfg.Host, cfg.Port, cfg.User, cfg.Password, cfg.DBName)

	db, err := sql.Open("pgx", dsn)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to database: %w", err)
	}

	// Configure connection pool for production use
	db.SetMaxOpenConns(25)
	db.SetMaxIdleConns(25)
	db.SetConnMaxLifetime(5 * time.Minute)

	// Verify connection
	if err = db.Ping(); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	return db, nil
}

// CheckConnectionInfo is a diagnostic function for development environments.
// It logs the current database and user information.
//
// Parameters:
//   - db: Active database connection
//   - location: Context string for logging
//
// Note: This function should be disabled in production environments
func CheckConnectionInfo(db *sql.DB, location string) {
	// Remove this function or wrap in development flag for production
}

// VerifyTables checks if required database tables exist by performing
// simple queries. This is used after migrations to ensure schema integrity.
//
// Parameters:
//   - db: Active database connection
//
// Returns:
//   - error: nil if all tables exist, error otherwise
func VerifyTables(db *sql.DB) error {
	// Verify jobs table exists
	if _, err := db.Query("SELECT COUNT(*) FROM jobs"); err != nil {
		return fmt.Errorf("jobs table verification failed: %w", err)
	}

	// Verify job_queue_outbox table exists
	if _, err := db.Query("SELECT COUNT(*) FROM job_queue_outbox"); err != nil {
		return fmt.Errorf("job_queue_outbox table verification failed: %w", err)
	}

	return nil
}
