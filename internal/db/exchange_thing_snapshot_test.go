package db

import (
	"testing"
	"time"
)

func TestSourceThingStoreListExchangeEnabledThings(t *testing.T) {
	testDB := openTestDatabase(t)
	mustExecSourceThingsDDL(t, testDB)

	_, err := testDB.Exec(`
		INSERT INTO things (id, name, exchange) VALUES
			(501, 'Angeling Card', TRUE),
			(502, '  ', TRUE),
			(503, 'Ignore Me', FALSE)
	`)
	if err != nil {
		t.Fatalf("insert things rows: %v", err)
	}

	store := NewSourceThingStore(testDB)
	things, err := store.ListExchangeEnabledThings()
	if err != nil {
		t.Fatalf("ListExchangeEnabledThings() error = %v", err)
	}
	if len(things) != 1 {
		t.Fatalf("len(ListExchangeEnabledThings()) = %d, want 1", len(things))
	}
	if things[0].ThingID != 501 || things[0].Name != "Angeling Card" {
		t.Fatalf("unexpected thing row: %#v", things[0])
	}
}

func TestExchangeThingSnapshotStoreReplaceSnapshot(t *testing.T) {
	testDB := openTestDatabase(t)

	store := NewExchangeThingSnapshotStore(testDB)
	firstRefresh := time.Unix(1000, 0).UTC()
	if err := store.ReplaceSnapshot([]*ExchangeThing{
		{ThingID: 501, Name: "Angeling Card"},
		{ThingID: 502, Name: "Poring Card"},
	}, firstRefresh); err != nil {
		t.Fatalf("ReplaceSnapshot(first) error = %v", err)
	}

	if got := store.GetName(501); got != "Angeling Card" {
		t.Fatalf("GetName(501) = %q, want %q", got, "Angeling Card")
	}

	secondRefresh := firstRefresh.Add(6 * time.Hour)
	if err := store.ReplaceSnapshot([]*ExchangeThing{
		{ThingID: 501, Name: "Renamed Card"},
	}, secondRefresh); err != nil {
		t.Fatalf("ReplaceSnapshot(second) error = %v", err)
	}

	if got := store.GetName(501); got != "Renamed Card" {
		t.Fatalf("GetName(501) after rename = %q, want %q", got, "Renamed Card")
	}
	if got := store.GetName(502); got != "" {
		t.Fatalf("GetName(502) after delete = %q, want empty", got)
	}

	var refreshedAt time.Time
	if err := testDB.QueryRow(`
		SELECT refreshed_at
		FROM exchange_thing_snapshots
		WHERE thing_id = 501
	`).Scan(&refreshedAt); err != nil {
		t.Fatalf("query refreshed_at: %v", err)
	}
	if !refreshedAt.Equal(secondRefresh) {
		t.Fatalf("refreshed_at = %v, want %v", refreshedAt, secondRefresh)
	}
}

func mustExecSourceThingsDDL(t *testing.T, db *DB) {
	t.Helper()

	_, err := db.Exec(`
		CREATE TABLE things (
			id BIGINT PRIMARY KEY,
			name TEXT,
			exchange BOOLEAN NOT NULL DEFAULT FALSE
		)
	`)
	if err != nil {
		t.Fatalf("create things table: %v", err)
	}
}
