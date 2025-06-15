package database

import (
	"database/sql"
	"embed"
	"fmt"
	"io/fs"
	"log"
	"sort"
)

//go:embed migrations/*.sql
var migrationsFS embed.FS

// In internal/database/migrations.go

// In internal/database/migrations.go
// In internal/database/migrations.go

func MigrateDB(db *sql.DB) error {
	log.Println("Running database migrations...")

	// --- Read the SQL files first ---
	files, err := fs.ReadDir(migrationsFS, "migrations")
	if err != nil {
		return fmt.Errorf("could not read embedded migrations directory: %w", err)
	}
	sort.Slice(files, func(i, j int) bool { return files[i].Name() < files[j].Name() })

	// --- Start Transaction ---
	tx, err := db.Begin()
	if err != nil {
		return fmt.Errorf("could not begin migration transaction: %w", err)
	}

	// --- Execute migrations within the transaction ---
	for _, file := range files {
		if file.IsDir() {
			continue
		}

		log.Printf("Applying migration: %s", file.Name())
		migrationSQL, ferr := migrationsFS.ReadFile("migrations/" + file.Name())
		if ferr != nil {
			_ = tx.Rollback() // Attempt to rollback on error
			return fmt.Errorf("could not read migration file %s: %w", file.Name(), ferr)
		}

		if _, execErr := tx.Exec(string(migrationSQL)); execErr != nil {
			_ = tx.Rollback() // Attempt to rollback on error
			return fmt.Errorf("failed to execute migration file %s: %w", file.Name(), execErr)
		}
	}

	// --- Explicitly Commit and Check Error ---
	log.Println("Committing migration transaction...")
	if commitErr := tx.Commit(); commitErr != nil {
		// If commit fails, we still try to rollback, though it might also fail.
		_ = tx.Rollback()
		return fmt.Errorf("failed to commit migration transaction: %w", commitErr)
	}

	log.Println("Database migrations completed successfully.")
	return nil
}
