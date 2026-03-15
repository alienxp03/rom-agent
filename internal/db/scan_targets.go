package db

import (
	"database/sql"
	"fmt"
	"log/slog"
	"sort"
	"strconv"
	"strings"
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

func (s *ScanTargetStore) ListActiveByMarket(market string, marketAliases map[string]string) ([]*ScanTarget, error) {
	serverIDs := scanTargetServerIDsForMarket(market, marketAliases)
	if len(serverIDs) == 0 {
		return nil, fmt.Errorf("no scan target servers configured for market %q", market)
	}
	slog.Info("Resolved exchange market to scan target servers",
		"exchange_market", market,
		"server_ids", serverIDs)

	rows, err := s.db.Query(`
		SELECT
			thing_id, server, equip_type, broken_state, refine_min, refine_max,
			enchant_ids, snap_ids, snap_count, active, projection_signature,
			projected_at, deactivated_at
		FROM scan_targets
		WHERE active = TRUE
		  AND server = ANY($1)
		ORDER BY thing_id ASC
	`, pq.Array(serverIDs))
	if err != nil {
		return nil, fmt.Errorf("query active scan targets for market %s: %w", market, err)
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
		return nil, fmt.Errorf("iterate active scan targets for market %s: %w", market, err)
	}
	merged := mergeScanTargetsByMarket(targets, market)
	slog.Info("Loaded active scan targets for market",
		"exchange_market", market,
		"raw_target_count", len(targets),
		"merged_target_count", len(merged))
	return merged, nil
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

func scanTargetServerIDsForMarket(market string, marketAliases map[string]string) []int {
	serverIDs := make([]int, 0, len(scanTargetServerIDs))
	for server, id := range scanTargetServerIDs {
		resolvedMarket := server
		if alias, ok := marketAliases[server]; ok && alias != "" {
			resolvedMarket = alias
		}
		if resolvedMarket == market {
			serverIDs = append(serverIDs, id)
		}
	}
	sort.Ints(serverIDs)
	return serverIDs
}

func mergeScanTargetsByMarket(targets []*ScanTarget, market string) []*ScanTarget {
	if len(targets) == 0 {
		return nil
	}

	mergedByKey := make(map[string]*ScanTarget, len(targets))
	order := make([]string, 0, len(targets))
	for _, target := range targets {
		if target == nil {
			continue
		}

		key := scanTargetMergeKey(target)
		existing, ok := mergedByKey[key]
		if !ok {
			copyTarget := *target
			copyTarget.Server = market
			copyTarget.EnchantIDs = append([]int(nil), target.EnchantIDs...)
			copyTarget.SnapIDs = append([]int64(nil), target.SnapIDs...)
			mergedByKey[key] = &copyTarget
			order = append(order, key)
			continue
		}

		existing.SnapCount += target.SnapCount
		existing.SnapIDs = mergeInt64Slices(existing.SnapIDs, target.SnapIDs)
	}

	merged := make([]*ScanTarget, 0, len(order))
	for _, key := range order {
		merged = append(merged, mergedByKey[key])
	}
	return merged
}

func scanTargetMergeKey(target *ScanTarget) string {
	return strings.Join([]string{
		strconv.FormatInt(target.ThingID, 10),
		target.EquipType,
		target.BrokenState,
		intPtrString(target.RefineMin),
		intPtrString(target.RefineMax),
		intSliceString(target.EnchantIDs),
	}, "|")
}

func intPtrString(value *int) string {
	if value == nil {
		return ""
	}
	return strconv.Itoa(*value)
}

func intSliceString(values []int) string {
	if len(values) == 0 {
		return ""
	}
	parts := make([]string, 0, len(values))
	for _, value := range values {
		parts = append(parts, strconv.Itoa(value))
	}
	return strings.Join(parts, ",")
}

func mergeInt64Slices(left, right []int64) []int64 {
	if len(right) == 0 {
		return left
	}

	seen := make(map[int64]struct{}, len(left)+len(right))
	merged := make([]int64, 0, len(left)+len(right))
	for _, value := range left {
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		merged = append(merged, value)
	}
	for _, value := range right {
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		merged = append(merged, value)
	}
	return merged
}
