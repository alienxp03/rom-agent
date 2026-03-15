package db

import (
	"database/sql"
	"testing"
	"time"
)

func TestExchangeRunStoreStartReusesIncompleteAndDeletesOldRows(t *testing.T) {
	testDB := openTestDatabase(t)
	store := NewExchangeRunStore(testDB)

	oldStartedAt := time.Now().UTC().AddDate(0, 0, -15).Truncate(time.Second)
	if _, err := testDB.Exec(`
		INSERT INTO exchange_runs (market, started_at, completed_at, duration_minutes)
		VALUES ($1, $2, $3, $4)
	`, "sea_shared", oldStartedAt, oldStartedAt.Add(time.Minute), 1.0); err != nil {
		t.Fatalf("insert old exchange run: %v", err)
	}

	incompleteStartedAt := time.Now().UTC().Add(-2 * time.Hour).Truncate(time.Second)
	var existingRunID int64
	if err := testDB.QueryRow(`
		INSERT INTO exchange_runs (market, started_at)
		VALUES ($1, $2)
		RETURNING id
	`, "sea_shared", incompleteStartedAt).Scan(&existingRunID); err != nil {
		t.Fatalf("insert incomplete exchange run: %v", err)
	}

	startedAt := time.Now().UTC().Truncate(time.Second)
	runID, err := store.Start("sea_shared", startedAt)
	if err != nil {
		t.Fatalf("Start() error = %v", err)
	}
	if runID != existingRunID {
		t.Fatalf("Start() runID = %d, want reused incomplete id %d", runID, existingRunID)
	}

	var gotStartedAt time.Time
	var completedAt sql.NullTime
	var durationMinutes sql.NullFloat64
	if err := testDB.QueryRow(`
		SELECT started_at, completed_at, duration_minutes
		FROM exchange_runs
		WHERE id = $1
	`, runID).Scan(&gotStartedAt, &completedAt, &durationMinutes); err != nil {
		t.Fatalf("query reused exchange run: %v", err)
	}
	if !gotStartedAt.Equal(startedAt) {
		t.Fatalf("started_at = %v, want %v", gotStartedAt, startedAt)
	}
	if completedAt.Valid {
		t.Fatalf("completed_at = %v, want nil", completedAt.Time)
	}
	if durationMinutes.Valid {
		t.Fatalf("duration_minutes = %v, want nil", durationMinutes.Float64)
	}

	var oldCount int
	if err := testDB.QueryRow(`
		SELECT count(*)
		FROM exchange_runs
		WHERE started_at = $1
	`, oldStartedAt).Scan(&oldCount); err != nil {
		t.Fatalf("count old exchange runs: %v", err)
	}
	if oldCount != 0 {
		t.Fatalf("old exchange run count = %d, want 0", oldCount)
	}
}

func TestExchangeRunStoreCompleteStoresCompletion(t *testing.T) {
	testDB := openTestDatabase(t)
	store := NewExchangeRunStore(testDB)

	startedAt := time.Unix(1_000, 0).UTC()
	runID, err := store.Start("sea_shared", startedAt)
	if err != nil {
		t.Fatalf("Start() error = %v", err)
	}

	completedAt := startedAt.Add(150 * time.Second)
	durationMinutes, err := store.Complete(runID, completedAt)
	if err != nil {
		t.Fatalf("Complete() error = %v", err)
	}
	if durationMinutes != 2.5 {
		t.Fatalf("Complete() durationMinutes = %v, want 2.5", durationMinutes)
	}

	var gotCompletedAt time.Time
	var gotDurationMinutes float64
	if err := testDB.QueryRow(`
		SELECT completed_at, duration_minutes
		FROM exchange_runs
		WHERE id = $1
	`, runID).Scan(&gotCompletedAt, &gotDurationMinutes); err != nil {
		t.Fatalf("query completed exchange run: %v", err)
	}
	if !gotCompletedAt.Equal(completedAt) {
		t.Fatalf("completed_at = %v, want %v", gotCompletedAt, completedAt)
	}
	if gotDurationMinutes != 2.5 {
		t.Fatalf("stored duration_minutes = %v, want 2.5", gotDurationMinutes)
	}
}
