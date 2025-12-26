package main

import (
	"database/sql"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/joho/godotenv"
	_ "github.com/lib/pq"

	"github.com/Zerr0-C00L/StreamArr/internal/config"
)

func main() {
	// Load .env file
	if err := godotenv.Load(); err != nil {
		log.Println("Warning: .env file not found, using environment variables")
	}

	log.Println("StreamArr Database Migration Tool")

	if len(os.Args) < 2 {
		log.Fatal("Usage: migrate [up|down]")
	}

	command := os.Args[1]

	// Load configuration
	cfg := config.Load()

	// Connect to database
	db, err := sql.Open("postgres", cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	defer db.Close()

	// Test connection
	if err := db.Ping(); err != nil {
		log.Fatalf("Failed to ping database: %v", err)
	}

	switch command {
	case "up":
		if err := migrateUp(db); err != nil {
			log.Fatalf("Migration failed: %v", err)
		}
		log.Println("Migration completed successfully")
	case "down":
		if err := migrateDown(db); err != nil {
			log.Fatalf("Migration rollback failed: %v", err)
		}
		log.Println("Migration rolled back successfully")
	default:
		log.Fatalf("Unknown command: %s. Use 'up' or 'down'", command)
	}
}

func migrateUp(db *sql.DB) error {
	log.Println("Running migrations...")

	// Create schema_migrations table if not exists
	if err := createMigrationsTable(db); err != nil {
		return fmt.Errorf("failed to create migrations table: %w", err)
	}

	// Get list of applied migrations
	appliedMigrations, err := getAppliedMigrations(db)
	if err != nil {
		return fmt.Errorf("failed to get applied migrations: %w", err)
	}

	// Find all migration files
	migrationFiles, err := filepath.Glob("migrations/*_*.up.sql")
	if err != nil {
		return fmt.Errorf("failed to find migration files: %w", err)
	}

	// Sort migrations by filename (which includes the number prefix)
	sort.Strings(migrationFiles)

	// Execute each migration that hasn't been applied
	appliedCount := 0
	for _, file := range migrationFiles {
		migrationName := strings.TrimSuffix(filepath.Base(file), ".up.sql")

		// Check if migration already applied
		if appliedMigrations[migrationName] {
			log.Printf("  ✓ %s (already applied)", migrationName)
			continue
		}

		log.Printf("  → Applying %s...", migrationName)

		// Read migration file
		migration, err := os.ReadFile(file)
		if err != nil {
			return fmt.Errorf("failed to read migration file %s: %w", file, err)
		}

		// Execute migration in a transaction
		tx, err := db.Begin()
		if err != nil {
			return fmt.Errorf("failed to begin transaction: %w", err)
		}

		// Execute migration SQL
		if _, err := tx.Exec(string(migration)); err != nil {
			tx.Rollback()
			return fmt.Errorf("failed to execute migration %s: %w", migrationName, err)
		}

		// Record migration as applied
		if _, err := tx.Exec("INSERT INTO schema_migrations (version, applied_at) VALUES ($1, NOW())", migrationName); err != nil {
			tx.Rollback()
			return fmt.Errorf("failed to record migration %s: %w", migrationName, err)
		}

		if err := tx.Commit(); err != nil {
			return fmt.Errorf("failed to commit migration %s: %w", migrationName, err)
		}

		log.Printf("  ✓ %s applied successfully", migrationName)
		appliedCount++
	}

	if appliedCount == 0 {
		log.Println("No new migrations to apply")
	} else {
		log.Printf("Applied %d migration(s)", appliedCount)
	}

	return nil
}

func createMigrationsTable(db *sql.DB) error {
	query := `
		CREATE TABLE IF NOT EXISTS schema_migrations (
			version VARCHAR(255) PRIMARY KEY,
			applied_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
		)
	`
	_, err := db.Exec(query)
	return err
}

func getAppliedMigrations(db *sql.DB) (map[string]bool, error) {
	rows, err := db.Query("SELECT version FROM schema_migrations")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	applied := make(map[string]bool)
	for rows.Next() {
		var version string
		if err := rows.Scan(&version); err != nil {
			return nil, err
		}
		applied[version] = true
	}

	return applied, rows.Err()
}

func migrateDown(db *sql.DB) error {
	log.Println("Rolling back migrations...")

	// Drop all tables in reverse order
	tables := []string{
		"watch_history",
		"activity_log",
		"available_streams",
		"library_episodes",
		"library_series",
		"library_movies",
	}

	for _, table := range tables {
		log.Printf("Dropping table %s...", table)
		if _, err := db.Exec(fmt.Sprintf("DROP TABLE IF EXISTS %s CASCADE", table)); err != nil {
			return fmt.Errorf("failed to drop table %s: %w", table, err)
		}
	}

	return nil
}
