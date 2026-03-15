package db

import (
	"testing"
	"time"

	"github.com/lib/pq"
)

func TestScanTargetStoreListActiveByMarket(t *testing.T) {
	testDB := openTestDatabase(t)
	mustExecScanTargetsDDL(t, testDB)

	projectedAt := time.Now().UTC().Truncate(time.Second)
	_, err := testDB.Exec(`
		INSERT INTO scan_targets (
			thing_id, server, equip_type, broken_state, refine_min, refine_max,
			enchant_ids, snap_ids, snap_count, active, projection_signature, projected_at
		) VALUES
			(501, 0, 1, 2, 5, 10, $1, $2, 2, TRUE, 'sea_el:501', $3),
			(501, 1, 1, 2, 5, 10, $1, $8, 1, TRUE, 'sea_mp:501', $3),
			(502, 5, 0, 0, NULL, NULL, $4, $5, 1, TRUE, 'rom_classic:502', $3),
			(503, 0, 0, 0, NULL, NULL, $6, $7, 1, FALSE, 'sea_el:503', $3)
	`, pq.Array([]int{500041, 500011}), pq.Array([]int64{1001, 1002}), projectedAt, pq.Array([]int{}), pq.Array([]int64{1003}), pq.Array([]int{}), pq.Array([]int64{1004}), pq.Array([]int64{1005}))
	if err != nil {
		t.Fatalf("insert scan_targets rows: %v", err)
	}

	store := NewScanTargetStore(testDB)
	targets, err := store.ListActiveByMarket("sea_shared", map[string]string{
		"sea_el": "sea_shared",
		"sea_mp": "sea_shared",
	})
	if err != nil {
		t.Fatalf("ListActiveByMarket() error = %v", err)
	}
	if len(targets) != 1 {
		t.Fatalf("len(ListActiveByMarket()) = %d, want 1", len(targets))
	}

	got := targets[0]
	if got.ThingID != 501 || got.Server != "sea_shared" {
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
	if got.SnapCount != 3 {
		t.Fatalf("unexpected snap count: %d", got.SnapCount)
	}
	if len(got.SnapIDs) != 3 {
		t.Fatalf("unexpected snap ids: %#v", got.SnapIDs)
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
