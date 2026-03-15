package db

import (
	"database/sql"
	"fmt"
	"strings"
	"time"
)

type ExchangeThing struct {
	ThingID int64
	Name    string
}

type SourceThingStore struct {
	db *DB
}

func NewSourceThingStore(db *DB) *SourceThingStore {
	return &SourceThingStore{db: db}
}

func (s *SourceThingStore) ListExchangeEnabledThings() ([]*ExchangeThing, error) {
	rows, err := s.db.Query(`
		SELECT id, name
		FROM things
		WHERE exchange = TRUE
		ORDER BY id ASC
	`)
	if err != nil {
		return nil, fmt.Errorf("query exchange-enabled things: %w", err)
	}
	defer rows.Close()

	var things []*ExchangeThing
	for rows.Next() {
		var (
			thingID int64
			name    sql.NullString
		)
		if err := rows.Scan(&thingID, &name); err != nil {
			return nil, fmt.Errorf("scan exchange-enabled thing: %w", err)
		}

		trimmedName := strings.TrimSpace(name.String)
		if !name.Valid || trimmedName == "" {
			continue
		}

		things = append(things, &ExchangeThing{
			ThingID: thingID,
			Name:    trimmedName,
		})
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate exchange-enabled things: %w", err)
	}
	return things, nil
}

type ExchangeThingSnapshotStore struct {
	db *DB
}

func NewExchangeThingSnapshotStore(db *DB) *ExchangeThingSnapshotStore {
	return &ExchangeThingSnapshotStore{db: db}
}

func (s *ExchangeThingSnapshotStore) ReplaceSnapshot(things []*ExchangeThing, refreshedAt time.Time) error {
	tx, err := s.db.Begin()
	if err != nil {
		return fmt.Errorf("begin exchange thing snapshot replace transaction: %w", err)
	}
	defer tx.Rollback()

	if _, err := tx.Exec(`
		CREATE TEMP TABLE tmp_exchange_thing_snapshots (
			thing_id BIGINT PRIMARY KEY,
			name TEXT NOT NULL
		) ON COMMIT DROP
	`); err != nil {
		return fmt.Errorf("create temp exchange thing snapshot table: %w", err)
	}

	if len(things) > 0 {
		stmt, err := tx.Prepare(`
			INSERT INTO tmp_exchange_thing_snapshots (thing_id, name)
			VALUES ($1, $2)
		`)
		if err != nil {
			return fmt.Errorf("prepare temp exchange thing snapshot insert: %w", err)
		}
		defer stmt.Close()

		for _, thing := range things {
			if thing == nil {
				continue
			}
			if _, err := stmt.Exec(thing.ThingID, thing.Name); err != nil {
				return fmt.Errorf("insert temp exchange thing snapshot %d: %w", thing.ThingID, err)
			}
		}
	}

	if _, err := tx.Exec(`
		INSERT INTO exchange_thing_snapshots (thing_id, name, refreshed_at)
		SELECT thing_id, name, $1
		FROM tmp_exchange_thing_snapshots
		ON CONFLICT (thing_id) DO UPDATE SET
			name = EXCLUDED.name,
			refreshed_at = EXCLUDED.refreshed_at,
			updated_at = CURRENT_TIMESTAMP
	`, refreshedAt); err != nil {
		return fmt.Errorf("upsert exchange thing snapshots: %w", err)
	}

	if _, err := tx.Exec(`
		DELETE FROM exchange_thing_snapshots snapshot
		WHERE NOT EXISTS (
			SELECT 1
			FROM tmp_exchange_thing_snapshots tmp
			WHERE tmp.thing_id = snapshot.thing_id
		)
	`); err != nil {
		return fmt.Errorf("delete stale exchange thing snapshots: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit exchange thing snapshot replace: %w", err)
	}
	return nil
}

func (s *ExchangeThingSnapshotStore) GetName(thingID int) string {
	if s == nil || s.db == nil {
		return ""
	}

	var name string
	err := s.db.QueryRow(`
		SELECT name
		FROM exchange_thing_snapshots
		WHERE thing_id = $1
	`, thingID).Scan(&name)
	if err != nil {
		return ""
	}
	return name
}

type ExchangeThingSnapshotRefresher struct {
	source   *SourceThingStore
	snapshot *ExchangeThingSnapshotStore
}

func NewExchangeThingSnapshotRefresher(source *SourceThingStore, snapshot *ExchangeThingSnapshotStore) *ExchangeThingSnapshotRefresher {
	return &ExchangeThingSnapshotRefresher{
		source:   source,
		snapshot: snapshot,
	}
}

func (r *ExchangeThingSnapshotRefresher) Refresh() (int, error) {
	if r == nil || r.source == nil || r.snapshot == nil {
		return 0, fmt.Errorf("refresh exchange thing snapshot: missing store")
	}

	things, err := r.source.ListExchangeEnabledThings()
	if err != nil {
		return 0, err
	}
	if err := r.snapshot.ReplaceSnapshot(things, time.Now().UTC()); err != nil {
		return 0, err
	}
	return len(things), nil
}
