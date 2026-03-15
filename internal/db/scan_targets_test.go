package db

import (
	"testing"
	"time"

	"github.com/lib/pq"
)

func TestScanTargetStoreListActiveByServer(t *testing.T) {
	testDB := openTestDatabase(t)
	mustExecScanTargetsDDL(t, testDB)

	projectedAt := time.Now().UTC().Truncate(time.Second)
	_, err := testDB.Exec(`
		INSERT INTO scan_targets (
			thing_id, server, equip_type, broken_state, refine_min, refine_max,
			enchant_ids, snap_ids, snap_count, active, projection_signature, projected_at
		) VALUES
			(501, 0, 1, 2, 5, 10, $1, $2, 2, TRUE, 'sea_el:501', $3),
			(502, 5, 0, 0, NULL, NULL, $4, $5, 1, TRUE, 'rom_classic:502', $3),
			(503, 0, 0, 0, NULL, NULL, $6, $7, 1, FALSE, 'sea_el:503', $3)
	`, pq.Array([]int{500041, 500011}), pq.Array([]int64{1001, 1002}), projectedAt, pq.Array([]int{}), pq.Array([]int64{1003}), pq.Array([]int{}), pq.Array([]int64{1004}))
	if err != nil {
		t.Fatalf("insert scan_targets rows: %v", err)
	}

	store := NewScanTargetStore(testDB)
	targets, err := store.ListActiveByServer("sea_el")
	if err != nil {
		t.Fatalf("ListActiveByServer() error = %v", err)
	}
	if len(targets) != 1 {
		t.Fatalf("len(ListActiveByServer()) = %d, want 1", len(targets))
	}

	got := targets[0]
	if got.ThingID != 501 || got.Server != "sea_el" {
		t.Fatalf("unexpected target identity: %#v", got)
	}
	if got.EquipType != "equip" || got.BrokenState != "non_broken" {
		t.Fatalf("unexpected target filters: %#v", got)
	}
	if got.RefineMin == nil || *got.RefineMin != 5 || got.RefineMax == nil || *got.RefineMax != 10 {
		t.Fatalf("unexpected refine bounds: %#v", got)
	}
	if len(got.EnchantIDs) != 2 || got.EnchantIDs[0] != 500041 || got.EnchantIDs[1] != 500011 {
		t.Fatalf("unexpected enchant ids: %#v", got.EnchantIDs)
	}
}

func TestScanResultStoreReplaceForTarget(t *testing.T) {
	testDB := openTestDatabase(t)

	store := NewScanResultStore(testDB)
	target := &ScanTarget{
		ThingID:             501,
		ProjectionSignature: "sea_el:501",
		SnapIDs:             []int64{1001, 1002},
	}
	now := time.Now().UTC().Truncate(time.Second)

	first := []*ScanResultRecord{
		{
			RecordID:            "order:1",
			ThingID:             501,
			Price:               1000,
			StockCount:          2,
			RefineLevel:         5,
			SnapAt:              now,
			ProjectionSignature: target.ProjectionSignature,
			SnapIDs:             target.SnapIDs,
		},
	}
	if err := store.ReplaceForTarget(target, first); err != nil {
		t.Fatalf("ReplaceForTarget(first) error = %v", err)
	}

	second := []*ScanResultRecord{
		{
			RecordID:            "order:2",
			ThingID:             501,
			Price:               2000,
			StockCount:          1,
			RefineLevel:         8,
			SnapAt:              now.Add(time.Minute),
			ProjectionSignature: target.ProjectionSignature,
			SnapIDs:             target.SnapIDs,
		},
	}
	if err := store.ReplaceForTarget(target, second); err != nil {
		t.Fatalf("ReplaceForTarget(second) error = %v", err)
	}

	rows, err := testDB.Query(`SELECT record_id, price, stock_count, refine_level FROM scan_results WHERE thing_id = $1`, target.ThingID)
	if err != nil {
		t.Fatalf("query scan_results: %v", err)
	}
	defer rows.Close()

	count := 0
	for rows.Next() {
		count++
		var recordID string
		var price int64
		var stockCount int
		var refineLevel int
		if err := rows.Scan(&recordID, &price, &stockCount, &refineLevel); err != nil {
			t.Fatalf("scan scan_results row: %v", err)
		}
		if recordID != "order:2" || price != 2000 || stockCount != 1 || refineLevel != 8 {
			t.Fatalf("unexpected stored row: %q %d %d %d", recordID, price, stockCount, refineLevel)
		}
	}
	if count != 1 {
		t.Fatalf("scan_results row count = %d, want 1", count)
	}
}

func mustExecScanTargetsDDL(t *testing.T, db *DB) {
	t.Helper()

	_, err := db.Exec(`
		CREATE TABLE scan_targets (
			id BIGSERIAL PRIMARY KEY,
			thing_id BIGINT NOT NULL,
			server INTEGER NOT NULL,
			equip_type INTEGER NOT NULL DEFAULT 0,
			broken_state INTEGER NOT NULL DEFAULT 0,
			refine_min INTEGER,
			refine_max INTEGER,
			enchant_ids INTEGER[] NOT NULL DEFAULT '{}',
			snap_ids BIGINT[] NOT NULL DEFAULT '{}',
			snap_count INTEGER NOT NULL DEFAULT 0,
			active BOOLEAN NOT NULL DEFAULT TRUE,
			projection_signature TEXT NOT NULL,
			projected_at TIMESTAMP WITH TIME ZONE NOT NULL,
			deactivated_at TIMESTAMP WITH TIME ZONE
		)
	`)
	if err != nil {
		t.Fatalf("create scan_targets table: %v", err)
	}
}
