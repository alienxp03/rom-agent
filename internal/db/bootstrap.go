package db

import (
	"database/sql"
	"fmt"

	"github.com/alienxp03/rom-agent/internal/config"
	"github.com/lib/pq"
)

// EnsureDatabaseExists creates the configured database if it does not already exist.
func EnsureDatabaseExists(cfg config.DatabaseConfig) error {
	adminConn := cfg.URLWithDBName("postgres")
	db, err := sql.Open("postgres", adminConn)
	if err != nil {
		return fmt.Errorf("open admin database connection: %w", err)
	}
	defer db.Close()

	if err := db.Ping(); err != nil {
		return fmt.Errorf("ping admin database connection: %w", err)
	}

	var exists bool
	if err := db.QueryRow(`SELECT EXISTS (SELECT 1 FROM pg_database WHERE datname = $1)`, cfg.DBName).Scan(&exists); err != nil {
		return fmt.Errorf("check database existence: %w", err)
	}
	if exists {
		return nil
	}

	if _, err := db.Exec(`CREATE DATABASE ` + pq.QuoteIdentifier(cfg.DBName)); err != nil {
		return fmt.Errorf("create database %q: %w", cfg.DBName, err)
	}
	return nil
}
