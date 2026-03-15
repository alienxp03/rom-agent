package db

import (
	"database/sql"
	"fmt"
	"time"

	"github.com/lib/pq"
)

var scanTargetServerIDs = map[string]int{
	"sea_el":      0,
	"sea_mp":      1,
	"sea_mof":     2,
	"sea_vg":      3,
	"eu_el":       4,
	"rom_classic": 5,
}

type ScanTarget struct {
	ThingID             int64
	Server              string
	EquipType           string
	BrokenState         string
	RefineMin           *int
	RefineMax           *int
	EnchantIDs          []int
	SnapIDs             []int64
	SnapCount           int
	Active              bool
	ProjectionSignature string
	ProjectedAt         time.Time
	DeactivatedAt       *time.Time
}

type ScanTargetStore struct {
	db *DB
}

func NewScanTargetStore(db *DB) *ScanTargetStore {
	return &ScanTargetStore{db: db}
}

func (s *ScanTargetStore) ListActiveByServer(server string) ([]*ScanTarget, error) {
	serverID, ok := scanTargetServerIDs[server]
	if !ok {
		return nil, fmt.Errorf("unsupported scan target server %q", server)
	}

	rows, err := s.db.Query(`
		SELECT
			thing_id, server, equip_type, broken_state, refine_min, refine_max,
			enchant_ids, snap_ids, snap_count, active, projection_signature,
			projected_at, deactivated_at
		FROM scan_targets
		WHERE active = TRUE
		  AND server = $1
		ORDER BY thing_id ASC
	`, serverID)
	if err != nil {
		return nil, fmt.Errorf("query active scan targets for %s: %w", server, err)
	}
	defer rows.Close()

	var targets []*ScanTarget
	for rows.Next() {
		target, err := scanTargetRow(rows)
		if err != nil {
			return nil, err
		}
		targets = append(targets, target)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate active scan targets for %s: %w", server, err)
	}
	return targets, nil
}

func scanTargetRow(scanner interface {
	Scan(dest ...interface{}) error
}) (*ScanTarget, error) {
	var (
		serverID      int
		equipTypeID   int
		brokenStateID int
		refineMin     sql.NullInt64
		refineMax     sql.NullInt64
		enchantIDs    []int64
		snapIDs       []int64
		deactivatedAt sql.NullTime
		target        ScanTarget
	)

	if err := scanner.Scan(
		&target.ThingID,
		&serverID,
		&equipTypeID,
		&brokenStateID,
		&refineMin,
		&refineMax,
		pq.Array(&enchantIDs),
		pq.Array(&snapIDs),
		&target.SnapCount,
		&target.Active,
		&target.ProjectionSignature,
		&target.ProjectedAt,
		&deactivatedAt,
	); err != nil {
		return nil, fmt.Errorf("scan target row: %w", err)
	}

	target.Server = scanTargetServerName(serverID)
	target.EquipType = scanTargetEquipTypeName(equipTypeID)
	target.BrokenState = scanTargetBrokenStateName(brokenStateID)
	target.RefineMin = nullableInt(refineMin)
	target.RefineMax = nullableInt(refineMax)
	target.EnchantIDs = make([]int, 0, len(enchantIDs))
	for _, id := range enchantIDs {
		target.EnchantIDs = append(target.EnchantIDs, int(id))
	}
	target.SnapIDs = snapIDs
	if deactivatedAt.Valid {
		target.DeactivatedAt = &deactivatedAt.Time
	}
	return &target, nil
}

func scanTargetServerName(serverID int) string {
	for name, id := range scanTargetServerIDs {
		if id == serverID {
			return name
		}
	}
	return ""
}

func scanTargetEquipTypeName(equipTypeID int) string {
	switch equipTypeID {
	case 1:
		return "equip"
	default:
		return "non_equip"
	}
}

func scanTargetBrokenStateName(brokenStateID int) string {
	switch brokenStateID {
	case 1:
		return "broken"
	case 2:
		return "non_broken"
	default:
		return "any"
	}
}

func nullableInt(v sql.NullInt64) *int {
	if !v.Valid {
		return nil
	}
	value := int(v.Int64)
	return &value
}
