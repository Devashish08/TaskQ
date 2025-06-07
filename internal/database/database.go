package database

import (
	"database/sql"
	"fmt"
	"log"
	"time"

	"github.com/Devashish08/taskq/internal/config"
	_ "github.com/jackc/pgx/v5/stdlib"
)

func ConnectDB(cfg *config.DBConfig) (*sql.DB, error) {
	dsn := fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=%s sslmode=disable",
		cfg.Host, cfg.Port, cfg.User, cfg.Password, cfg.DBName)

	log.Println("Connecting to database with DSN:", dsn)
	db, err := sql.Open("pgx", dsn)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to database: %w", err)
	}

	db.SetMaxOpenConns(25)
	db.SetMaxIdleConns(25)
	db.SetConnMaxLifetime(5 * time.Minute)

	log.Println("Pinging Database...")
	err = db.Ping()
	if err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	log.Println("Database connection established")
	return db, nil
}

func MigrateDB(db *sql.DB) error {
	log.Println("Running database migration (CREATE TABLE IF NOT EXISTS)...")
	_, err := db.Exec(createJobsTableSQL)
	if err != nil {
		return fmt.Errorf("failed to run migration: %w", err)
	}
	log.Println("Database migration check completed successfully.")
	return nil
}
