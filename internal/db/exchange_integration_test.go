package db

import (
	"database/sql"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/alienxp03/rom-agent/internal/resources"
	"github.com/lib/pq"
)

func TestRunMigrationsAndUpsertLatestRecords(t *testing.T) {
	testDB := openTestDatabase(t)

	edb := NewExchangeDb(testDB)
	seenAt := time.Now().UTC().Truncate(time.Second)
	record := &ExchangeItemRecord{
		IdentityKey:  "test|item-1",
		ItemID:       1001,
		Name:         "Test Card",
		CategoryID:   resources.Material.ID,
		Server:       "SEA2",
		Zone:         "4201",
		Price:        123456,
		ListingCount: 2,
		BuyerCount:   1,
		Quota:        3,
		InStock:      true,
		LastSeenAt:   seenAt,
		Modified:     true,
		RefineLevel:  5,
	}

	if err := edb.UpsertLatestRecords([]*ExchangeItemRecord{record}); err != nil {
		t.Fatalf("UpsertLatestRecords() error = %v", err)
	}

	var got ExchangeItemRecord
	if err := testDB.QueryRow(`
		SELECT identity_key, item_id, name, category_id, server, zone, price, quantity,
		       listing_count, buyer_count, quota, in_stock, last_seen_at, modified, refine_level
		FROM exchange_items
		WHERE identity_key = $1
	`, record.IdentityKey).Scan(
		&got.IdentityKey,
		&got.ItemID,
		&got.Name,
		&got.CategoryID,
		&got.Server,
		&got.Zone,
		&got.Price,
		new(int),
		&got.ListingCount,
		&got.BuyerCount,
		&got.Quota,
		&got.InStock,
		&got.LastSeenAt,
		&got.Modified,
		&got.RefineLevel,
	); err != nil {
		t.Fatalf("query inserted exchange row: %v", err)
	}

	if got.ItemID != record.ItemID || got.Name != record.Name {
		t.Fatalf("unexpected stored row: %#v", got)
	}
	if got.ListingCount != record.ListingCount || got.BuyerCount != record.BuyerCount {
		t.Fatalf("unexpected counts: %#v", got)
	}
	if !got.InStock || !got.Modified || got.RefineLevel != record.RefineLevel {
		t.Fatalf("unexpected flags: %#v", got)
	}
}

func TestMarkSoldOutScopesToCategoryServerZone(t *testing.T) {
	testDB := openTestDatabase(t)
	edb := NewExchangeDb(testDB)

	oldSeenAt := time.Now().UTC().Add(-1 * time.Hour).Truncate(time.Second)
	currentSeenAt := time.Now().UTC().Truncate(time.Second)

	records := []*ExchangeItemRecord{
		{
			IdentityKey:  "sold-out|old",
			ItemID:       2001,
			Name:         "Old Card",
			CategoryID:   resources.Material.ID,
			Server:       "SEA2",
			Zone:         "4201",
			Price:        100,
			ListingCount: 1,
			InStock:      true,
			LastSeenAt:   oldSeenAt,
		},
		{
			IdentityKey:  "sold-out|current",
			ItemID:       2002,
			Name:         "Current Card",
			CategoryID:   resources.Material.ID,
			Server:       "SEA2",
			Zone:         "4201",
			Price:        200,
			ListingCount: 1,
			InStock:      true,
			LastSeenAt:   currentSeenAt,
		},
		{
			IdentityKey:  "sold-out|other-zone",
			ItemID:       2003,
			Name:         "Other Zone Card",
			CategoryID:   resources.Material.ID,
			Server:       "SEA2",
			Zone:         "9999",
			Price:        300,
			ListingCount: 1,
			InStock:      true,
			LastSeenAt:   oldSeenAt,
		},
	}

	if err := edb.UpsertLatestRecords(records); err != nil {
		t.Fatalf("UpsertLatestRecords() error = %v", err)
	}

	updated, err := edb.MarkSoldOut(resources.Material, "SEA2", "4201", currentSeenAt)
	if err != nil {
		t.Fatalf("MarkSoldOut() error = %v", err)
	}
	if updated != 1 {
		t.Fatalf("MarkSoldOut() updated %d rows, want 1", updated)
	}

	rows, err := testDB.Query(`SELECT identity_key, in_stock FROM exchange_items ORDER BY identity_key`)
	if err != nil {
		t.Fatalf("query exchange rows: %v", err)
	}
	defer rows.Close()

	got := map[string]bool{}
	for rows.Next() {
		var identityKey string
		var inStock bool
		if err := rows.Scan(&identityKey, &inStock); err != nil {
			t.Fatalf("scan exchange row: %v", err)
		}
		got[identityKey] = inStock
	}

	if got["sold-out|old"] {
		t.Fatal("old row should have been marked out of stock")
	}
	if !got["sold-out|current"] {
		t.Fatal("current row should remain in stock")
	}
	if !got["sold-out|other-zone"] {
		t.Fatal("other-zone row should remain in stock")
	}
}

func TestReplaceTargetedRecordsReplacesCurrentRows(t *testing.T) {
	testDB := openTestDatabase(t)
	edb := NewExchangeDb(testDB)
	seenAt := time.Now().UTC().Truncate(time.Second)

	first := []*ExchangeItemRecord{
		{
			IdentityKey:  "targeted|1",
			ItemID:       40633,
			Name:         "Staff of Element Fusion [1]",
			CategoryID:   0,
			Server:       "sea_mp",
			Zone:         "",
			Price:        7176342,
			ListingCount: 2,
			InStock:      true,
			LastSeenAt:   seenAt,
		},
	}
	if err := edb.ReplaceTargetedRecords(40633, "sea_mp", first); err != nil {
		t.Fatalf("ReplaceTargetedRecords(first) error = %v", err)
	}

	second := []*ExchangeItemRecord{
		{
			IdentityKey:  "targeted|2",
			ItemID:       40633,
			Name:         "Staff of Element Fusion [1]",
			CategoryID:   0,
			Server:       "sea_mp",
			Zone:         "",
			Price:        8000000,
			ListingCount: 1,
			InStock:      true,
			LastSeenAt:   seenAt.Add(time.Minute),
		},
	}
	if err := edb.ReplaceTargetedRecords(40633, "sea_mp", second); err != nil {
		t.Fatalf("ReplaceTargetedRecords(second) error = %v", err)
	}

	rows, err := testDB.Query(`
		SELECT identity_key, price, listing_count
		FROM exchange_items
		WHERE item_id = 40633 AND server = 'sea_mp' AND category_id = 0
	`)
	if err != nil {
		t.Fatalf("query targeted exchange rows: %v", err)
	}
	defer rows.Close()

	count := 0
	for rows.Next() {
		count++
		var identityKey string
		var price int64
		var listingCount int
		if err := rows.Scan(&identityKey, &price, &listingCount); err != nil {
			t.Fatalf("scan targeted exchange row: %v", err)
		}
		if identityKey != "targeted|2" || price != 8000000 || listingCount != 1 {
			t.Fatalf("unexpected targeted row: %q %d %d", identityKey, price, listingCount)
		}
	}
	if count != 1 {
		t.Fatalf("targeted exchange row count = %d, want 1", count)
	}
}

func openTestDatabase(t *testing.T) *DB {
	t.Helper()

	adminURL := os.Getenv("ROM_AGENT_TEST_ADMIN_DB_URL")
	if adminURL == "" {
		adminURL = "postgres://postgres@localhost:5432/postgres?sslmode=disable"
	}

	adminDB, err := sql.Open("postgres", adminURL)
	if err != nil {
		t.Skipf("open admin database: %v", err)
	}
	defer adminDB.Close()

	if err := adminDB.Ping(); err != nil {
		t.Skipf("ping admin database: %v", err)
	}

	dbName := fmt.Sprintf("rom_agent_test_%d", time.Now().UnixNano())
	if _, err := adminDB.Exec(`CREATE DATABASE ` + pq.QuoteIdentifier(dbName)); err != nil {
		t.Fatalf("create test database %q: %v", dbName, err)
	}

	t.Cleanup(func() {
		_, _ = adminDB.Exec(`
			SELECT pg_terminate_backend(pid)
			FROM pg_stat_activity
			WHERE datname = $1
			  AND pid <> pg_backend_pid()
		`, dbName)
		_, _ = adminDB.Exec(`DROP DATABASE IF EXISTS ` + pq.QuoteIdentifier(dbName))
	})

	databaseURL := fmt.Sprintf("postgres://postgres@localhost:5432/%s?sslmode=disable", dbName)
	if err := RunMigrations(databaseURL); err != nil {
		t.Fatalf("RunMigrations(%q) error = %v", dbName, err)
	}

	testDB, err := Open(databaseURL)
	if err != nil {
		t.Fatalf("Open(%q) error = %v", dbName, err)
	}

	t.Cleanup(func() {
		_ = testDB.Close()
	})

	return testDB
}
