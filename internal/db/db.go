package db

import (
	"database/sql"
	"fmt"
	"time"

	_ "github.com/lib/pq"
)

// DB wraps a PostgreSQL database connection
type DB struct {
	*sql.DB
}

// Open creates a new database connection
func Open(connStr string) (*DB, error) {
	db, err := sql.Open("postgres", connStr)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	// Configure connection pool
	db.SetMaxOpenConns(25)
	db.SetMaxIdleConns(10)
	db.SetConnMaxLifetime(5 * time.Minute)
	db.SetConnMaxIdleTime(1 * time.Minute)

	// Verify connection
	if err := db.Ping(); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	return &DB{db}, nil
}

// BeginTx starts a new transaction
func (db *DB) BeginTx() (*sql.Tx, error) {
	return db.Begin()
}

// Health checks the database connection health
func (db *DB) Health() error {
	return db.Ping()
}
