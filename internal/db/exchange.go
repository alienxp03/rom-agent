package db

import (
	"database/sql"
	"fmt"
	"log/slog"
	"time"

	"github.com/alienxp03/rom-agent/internal/resources"
)

// ExchangeDb handles exchange/market data persistence.
type ExchangeDb struct {
	db *DB
}

// ExchangeItemRecord represents the latest known state of one exchange item variant.
type ExchangeItemRecord struct {
	IdentityKey    string
	ItemID         int
	Name           string
	CategoryID     int
	Server         string
	Zone           string
	Price          int64
	ListingCount   int
	BuyerCount     int
	Quota          int64
	EndTime        *int64
	InStock        bool
	LastSeenAt     time.Time
	Modified       bool
	RefineLevel    int
	IsBroken       bool
	BuffID         *int
	BuffAttr1Name  *string
	BuffAttr1Value *int
	BuffAttr2Name  *string
	BuffAttr2Value *int
	BuffAttr3Name  *string
	BuffAttr3Value *int
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

func NewExchangeDb(db *DB) *ExchangeDb {
	return &ExchangeDb{db: db}
}

// UpsertLatestRecords writes the latest known state for each observed item variant.
func (edb *ExchangeDb) UpsertLatestRecords(records []*ExchangeItemRecord) error {
	if len(records) == 0 {
		return nil
	}

	tx, err := edb.db.Begin()
	if err != nil {
		return fmt.Errorf("begin exchange upsert transaction: %w", err)
	}
	defer tx.Rollback()

	selectStmt, err := tx.Prepare(`SELECT id FROM exchange_items WHERE identity_key = $1`)
	if err != nil {
		return fmt.Errorf("prepare exchange identity lookup: %w", err)
	}
	defer selectStmt.Close()

	insertStmt, err := tx.Prepare(`
		INSERT INTO exchange_items (
			identity_key, item_id, name, category_id, server, zone,
			price, quantity, listing_count, buyer_count, quota, end_time,
			in_stock, last_seen_at, modified, refine_level, is_broken,
			buff_id, buff_attr1_name, buff_attr1_value,
			buff_attr2_name, buff_attr2_value,
			buff_attr3_name, buff_attr3_value
		) VALUES (
			$1, $2, $3, $4, $5, $6,
			$7, $8, $9, $10, $11, $12,
			$13, $14, $15, $16, $17,
			$18, $19, $20,
			$21, $22,
			$23, $24
		)
	`)
	if err != nil {
		return fmt.Errorf("prepare exchange insert: %w", err)
	}
	defer insertStmt.Close()

	updateStmt, err := tx.Prepare(`
		UPDATE exchange_items SET
			item_id = $2,
			name = $3,
			category_id = $4,
			server = $5,
			zone = $6,
			price = $7,
			quantity = $8,
			listing_count = $9,
			buyer_count = $10,
			quota = $11,
			end_time = $12,
			in_stock = $13,
			last_seen_at = $14,
			modified = $15,
			refine_level = $16,
			is_broken = $17,
			buff_id = $18,
			buff_attr1_name = $19,
			buff_attr1_value = $20,
			buff_attr2_name = $21,
			buff_attr2_value = $22,
			buff_attr3_name = $23,
			buff_attr3_value = $24,
			updated_at = CURRENT_TIMESTAMP
		WHERE identity_key = $1
	`)
	if err != nil {
		return fmt.Errorf("prepare exchange update: %w", err)
	}
	defer updateStmt.Close()

	for _, record := range records {
		if record == nil {
			continue
		}

		var rowID int64
		err := selectStmt.QueryRow(record.IdentityKey).Scan(&rowID)
		switch {
		case err == sql.ErrNoRows:
			if _, err := insertStmt.Exec(
				record.IdentityKey,
				record.ItemID,
				record.Name,
				record.CategoryID,
				record.Server,
				record.Zone,
				record.Price,
				record.ListingCount,
				record.ListingCount,
				record.BuyerCount,
				record.Quota,
				record.EndTime,
				record.InStock,
				record.LastSeenAt,
				record.Modified,
				record.RefineLevel,
				record.IsBroken,
				record.BuffID,
				record.BuffAttr1Name,
				record.BuffAttr1Value,
				record.BuffAttr2Name,
				record.BuffAttr2Value,
				record.BuffAttr3Name,
				record.BuffAttr3Value,
			); err != nil {
				slog.Error("Failed to insert exchange item row",
					"identity_key", record.IdentityKey,
					"item_id", record.ItemID,
					"category_id", record.CategoryID,
					"server", record.Server,
					"zone", record.Zone,
					"error", err)
			}
		case err != nil:
			return fmt.Errorf("lookup exchange identity %q: %w", record.IdentityKey, err)
		default:
			if _, err := updateStmt.Exec(
				record.IdentityKey,
				record.ItemID,
				record.Name,
				record.CategoryID,
				record.Server,
				record.Zone,
				record.Price,
				record.ListingCount,
				record.ListingCount,
				record.BuyerCount,
				record.Quota,
				record.EndTime,
				record.InStock,
				record.LastSeenAt,
				record.Modified,
				record.RefineLevel,
				record.IsBroken,
				record.BuffID,
				record.BuffAttr1Name,
				record.BuffAttr1Value,
				record.BuffAttr2Name,
				record.BuffAttr2Value,
				record.BuffAttr3Name,
				record.BuffAttr3Value,
			); err != nil {
				slog.Error("Failed to update exchange item row",
					"identity_key", record.IdentityKey,
					"item_id", record.ItemID,
					"category_id", record.CategoryID,
					"server", record.Server,
					"zone", record.Zone,
					"error", err)
			}
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit exchange upsert transaction: %w", err)
	}
	return nil
}

// MarkSoldOut marks latest-state item variants as out of stock when they were not
// observed during the current full category snapshot.
func (edb *ExchangeDb) MarkSoldOut(category resources.ExchangeCategory, server, zone string, seenBefore time.Time) (int, error) {
	result, err := edb.db.Exec(`
		UPDATE exchange_items
		SET in_stock = FALSE, updated_at = CURRENT_TIMESTAMP
		WHERE category_id = $1
		  AND server = $2
		  AND zone = $3
		  AND last_seen_at < $4
		  AND in_stock = TRUE
	`, category.ID, server, zone, seenBefore)
	if err != nil {
		return 0, fmt.Errorf("mark exchange items sold out: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return 0, fmt.Errorf("get sold-out row count: %w", err)
	}
	return int(rows), nil
}

// GetLatestPrices retrieves the latest in-stock rows for one category.
func (edb *ExchangeDb) GetLatestPrices(categoryID int, limit int) ([]*ExchangeItemRecord, error) {
	rows, err := edb.db.Query(`
		SELECT
			identity_key, item_id, name, category_id, server, zone,
			price, listing_count, buyer_count, quota, end_time,
			in_stock, last_seen_at, modified, refine_level, is_broken,
			buff_id, buff_attr1_name, buff_attr1_value,
			buff_attr2_name, buff_attr2_value,
			buff_attr3_name, buff_attr3_value,
			created_at, updated_at
		FROM exchange_items
		WHERE category_id = $1
		  AND in_stock = TRUE
		ORDER BY updated_at DESC
		LIMIT $2
	`, categoryID, limit)
	if err != nil {
		return nil, fmt.Errorf("query latest exchange prices: %w", err)
	}
	defer rows.Close()

	return scanExchangeItemRows(rows)
}

// GetItemPriceHistory returns the latest known rows for an item. Historical rows are
// intentionally out of scope in the current phase.
func (edb *ExchangeDb) GetItemPriceHistory(itemID int, _ int) ([]*ExchangeItemRecord, error) {
	rows, err := edb.db.Query(`
		SELECT
			identity_key, item_id, name, category_id, server, zone,
			price, listing_count, buyer_count, quota, end_time,
			in_stock, last_seen_at, modified, refine_level, is_broken,
			buff_id, buff_attr1_name, buff_attr1_value,
			buff_attr2_name, buff_attr2_value,
			buff_attr3_name, buff_attr3_value,
			created_at, updated_at
		FROM exchange_items
		WHERE item_id = $1
		ORDER BY updated_at DESC
	`, itemID)
	if err != nil {
		return nil, fmt.Errorf("query exchange latest rows for item: %w", err)
	}
	defer rows.Close()

	return scanExchangeItemRows(rows)
}

func (edb *ExchangeDb) CleanupOldRecords(days int) (int64, error) {
	cutoff := time.Now().AddDate(0, 0, -days)

	result, err := edb.db.Exec(`
		DELETE FROM exchange_items
		WHERE in_stock = FALSE
		  AND updated_at < $1
	`, cutoff)
	if err != nil {
		return 0, fmt.Errorf("cleanup old exchange rows: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return 0, fmt.Errorf("get cleanup row count: %w", err)
	}
	return rows, nil
}

func scanExchangeItemRows(rows *sql.Rows) ([]*ExchangeItemRecord, error) {
	var records []*ExchangeItemRecord
	for rows.Next() {
		var record ExchangeItemRecord
		var endTime sql.NullInt64
		var buffID sql.NullInt64
		var attr1Name sql.NullString
		var attr1Value sql.NullInt64
		var attr2Name sql.NullString
		var attr2Value sql.NullInt64
		var attr3Name sql.NullString
		var attr3Value sql.NullInt64

		if err := rows.Scan(
			&record.IdentityKey,
			&record.ItemID,
			&record.Name,
			&record.CategoryID,
			&record.Server,
			&record.Zone,
			&record.Price,
			&record.ListingCount,
			&record.BuyerCount,
			&record.Quota,
			&endTime,
			&record.InStock,
			&record.LastSeenAt,
			&record.Modified,
			&record.RefineLevel,
			&record.IsBroken,
			&buffID,
			&attr1Name,
			&attr1Value,
			&attr2Name,
			&attr2Value,
			&attr3Name,
			&attr3Value,
			&record.CreatedAt,
			&record.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan exchange row: %w", err)
		}

		if endTime.Valid {
			value := endTime.Int64
			record.EndTime = &value
		}
		if buffID.Valid {
			value := int(buffID.Int64)
			record.BuffID = &value
		}
		if attr1Name.Valid {
			value := attr1Name.String
			record.BuffAttr1Name = &value
		}
		if attr1Value.Valid {
			value := int(attr1Value.Int64)
			record.BuffAttr1Value = &value
		}
		if attr2Name.Valid {
			value := attr2Name.String
			record.BuffAttr2Name = &value
		}
		if attr2Value.Valid {
			value := int(attr2Value.Int64)
			record.BuffAttr2Value = &value
		}
		if attr3Name.Valid {
			value := attr3Name.String
			record.BuffAttr3Name = &value
		}
		if attr3Value.Valid {
			value := int(attr3Value.Int64)
			record.BuffAttr3Value = &value
		}

		records = append(records, &record)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate exchange rows: %w", err)
	}
	return records, nil
}
