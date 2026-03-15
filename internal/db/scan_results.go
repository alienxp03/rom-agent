package db

import (
	"fmt"
	"time"

	"github.com/lib/pq"
)

type ScanResultRecord struct {
	RecordID            string
	ThingID             int64
	Price               int64
	StockCount          int
	RefineLevel         int
	Enchant             *int
	EnchantLevel        int
	Broken              bool
	SnapAt              time.Time
	SnapEndAt           *time.Time
	ProjectionSignature string
	SnapIDs             []int64
	CreatedAt           time.Time
	UpdatedAt           time.Time
}

type ScanResultStore struct {
	db *DB
}

func NewScanResultStore(db *DB) *ScanResultStore {
	return &ScanResultStore{db: db}
}

func (s *ScanResultStore) ReplaceForTarget(target *ScanTarget, records []*ScanResultRecord) error {
	if target == nil {
		return fmt.Errorf("replace scan results: target is nil")
	}

	tx, err := s.db.Begin()
	if err != nil {
		return fmt.Errorf("begin scan result replace transaction: %w", err)
	}
	defer tx.Rollback()

	if _, err := tx.Exec(`DELETE FROM scan_results WHERE thing_id = $1`, target.ThingID); err != nil {
		return fmt.Errorf("delete scan results for thing %d: %w", target.ThingID, err)
	}

	if len(records) == 0 {
		if err := tx.Commit(); err != nil {
			return fmt.Errorf("commit empty scan result replace: %w", err)
		}
		return nil
	}

	stmt, err := tx.Prepare(`
		INSERT INTO scan_results (
			record_id, thing_id, price, stock_count, refine_level,
			enchant, enchant_level, broken, snap_at, snap_end_at,
			projection_signature, snap_ids
		) VALUES (
			$1, $2, $3, $4, $5,
			$6, $7, $8, $9, $10,
			$11, $12
		)
	`)
	if err != nil {
		return fmt.Errorf("prepare scan result insert: %w", err)
	}
	defer stmt.Close()

	for _, record := range records {
		if record == nil {
			continue
		}
		if _, err := stmt.Exec(
			record.RecordID,
			record.ThingID,
			record.Price,
			record.StockCount,
			record.RefineLevel,
			record.Enchant,
			record.EnchantLevel,
			record.Broken,
			record.SnapAt,
			record.SnapEndAt,
			record.ProjectionSignature,
			pq.Array(record.SnapIDs),
		); err != nil {
			return fmt.Errorf("insert scan result %q for thing %d: %w", record.RecordID, record.ThingID, err)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit scan result replace: %w", err)
	}
	return nil
}
