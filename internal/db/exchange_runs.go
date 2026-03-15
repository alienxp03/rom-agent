package db

import (
	"database/sql"
	"fmt"
	"time"
)

type ExchangeRunStore struct {
	db *DB
}

func NewExchangeRunStore(db *DB) *ExchangeRunStore {
	return &ExchangeRunStore{db: db}
}

func (s *ExchangeRunStore) Start(market string, startedAt time.Time) (int64, error) {
	if s == nil || s.db == nil {
		return 0, fmt.Errorf("start exchange run: store is nil")
	}
	if market == "" {
		return 0, fmt.Errorf("start exchange run: market is required")
	}

	tx, err := s.db.Begin()
	if err != nil {
		return 0, fmt.Errorf("begin exchange run start transaction: %w", err)
	}
	defer tx.Rollback()

	if _, err := tx.Exec(`
		DELETE FROM exchange_runs
		WHERE started_at < $1
	`, startedAt.AddDate(0, 0, -14)); err != nil {
		return 0, fmt.Errorf("delete stale exchange runs: %w", err)
	}

	var runID int64
	err = tx.QueryRow(`
		UPDATE exchange_runs
		SET started_at = $2,
			completed_at = NULL,
			duration_minutes = NULL,
			updated_at = CURRENT_TIMESTAMP
		WHERE market = $1
		  AND completed_at IS NULL
		RETURNING id
	`, market, startedAt).Scan(&runID)
	switch {
	case err == nil:
	case err == sql.ErrNoRows:
		if err := tx.QueryRow(`
			INSERT INTO exchange_runs (market, started_at)
			VALUES ($1, $2)
			RETURNING id
		`, market, startedAt).Scan(&runID); err != nil {
			return 0, fmt.Errorf("insert exchange run: %w", err)
		}
	default:
		return 0, fmt.Errorf("update incomplete exchange run: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return 0, fmt.Errorf("commit exchange run start: %w", err)
	}
	return runID, nil
}

func (s *ExchangeRunStore) Complete(runID int64, completedAt time.Time) (float64, error) {
	if s == nil || s.db == nil {
		return 0, fmt.Errorf("complete exchange run: store is nil")
	}
	if runID <= 0 {
		return 0, fmt.Errorf("complete exchange run: invalid run id %d", runID)
	}

	tx, err := s.db.Begin()
	if err != nil {
		return 0, fmt.Errorf("begin exchange run complete transaction: %w", err)
	}
	defer tx.Rollback()

	var startedAt time.Time
	if err := tx.QueryRow(`
		SELECT started_at
		FROM exchange_runs
		WHERE id = $1
		  AND completed_at IS NULL
		FOR UPDATE
	`, runID).Scan(&startedAt); err != nil {
		if err == sql.ErrNoRows {
			return 0, fmt.Errorf("complete exchange run: run %d is not incomplete", runID)
		}
		return 0, fmt.Errorf("load exchange run %d: %w", runID, err)
	}

	durationMinutes := completedAt.Sub(startedAt).Minutes()
	if durationMinutes < 0 {
		durationMinutes = 0
	}

	if _, err := tx.Exec(`
		UPDATE exchange_runs
		SET completed_at = $2,
			duration_minutes = $3,
			updated_at = CURRENT_TIMESTAMP
		WHERE id = $1
	`, runID, completedAt, durationMinutes); err != nil {
		return 0, fmt.Errorf("update exchange run %d: %w", runID, err)
	}

	if err := tx.Commit(); err != nil {
		return 0, fmt.Errorf("commit exchange run completion: %w", err)
	}
	return durationMinutes, nil
}
