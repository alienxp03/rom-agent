package bot

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"time"

	"github.com/alienxp03/rom-agent/internal/client"
	"github.com/alienxp03/rom-agent/internal/config"
	"github.com/alienxp03/rom-agent/internal/db"
	"github.com/alienxp03/rom-agent/internal/resources"
)

// Bot represents a game bot instance
type Bot struct {
	// Config
	clientConfig *config.Config
	clientIndex  int
	blueberry    int
	version      int

	// Resources (will be implemented later)
	// itemDb       *resources.ItemDb
	// monsterDb    *resources.MonsterDb
	// bossDb       *resources.BossDb
	// bufferDb     *resources.BufferDb

	// Databases
	exchangeDb         *db.ExchangeDb
	exchangeRunStore   *db.ExchangeRunStore
	thingSnapshotStore *db.ExchangeThingSnapshotStore
	scanTargetDb       *db.ScanTargetStore
	runtimeServer      string
	exchangeMarket     string
	// bossBoardDb  *db.BossBoardDb
	// pvpDb        *db.PvpDb
	// auctionDb    *db.AuctionDb

	// Shared data (will be implemented later)
	// sharedData   *db.SharedDataWrapper

	// Exchange categories
	exchangeCategories []resources.ExchangeCategory

	// State
	categoryIndex           int
	jumpZoneList            []string
	server                  string
	zone                    string
	saveMapId               int
	lastExchangeRecordCount int

	// Game client
	gameClient *client.GameClient
}

// BotState represents the persistent state of a bot
type BotState struct {
	CategoryIndex int      `json:"category_index"`
	JumpZoneList  []string `json:"jump_zone_list"`
	Server        string   `json:"server"`
	Zone          string   `json:"zone"`
	SaveMapId     int      `json:"save_map_id"`
	Timestamp     int64    `json:"timestamp"`
}

// New creates a new bot instance
func New(
	cfg *config.Config,
	clientIndex int,
	blueberry, version int,
	exchangeCategories []resources.ExchangeCategory,
	exchangeDb *db.ExchangeDb,
	exchangeRunStore *db.ExchangeRunStore,
	thingSnapshotStore *db.ExchangeThingSnapshotStore,
	scanTargetDb *db.ScanTargetStore,
) *Bot {
	return &Bot{
		clientConfig:       cfg,
		clientIndex:        clientIndex,
		blueberry:          blueberry,
		version:            version,
		exchangeCategories: exchangeCategories,
		exchangeDb:         exchangeDb,
		exchangeRunStore:   exchangeRunStore,
		thingSnapshotStore: thingSnapshotStore,
		scanTargetDb:       scanTargetDb,
		runtimeServer:      cfg.RuntimeServer,
		exchangeMarket:     cfg.ExchangeTarget.Market,
		categoryIndex:      0,
		jumpZoneList:       []string{},
	}
}

// Run starts the bot main loop with automatic recovery
func (b *Bot) Run(ctx context.Context) error {
	clientName := b.clientConfig.Clients[b.clientIndex].Name

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
			if err := b.runImpl(ctx); err != nil {
				slog.Error("Bot error, retrying",
					"client", clientName,
					"error", err)

				// Wait before retrying
				select {
				case <-ctx.Done():
					return ctx.Err()
				case <-time.After(b.clientConfig.BotRetryDelayInterval()):
					// Continue to next iteration
				}
			}
		}
	}
}

// runImpl implements the main bot logic with error recovery
func (b *Bot) runImpl(ctx context.Context) error {
	// Load saved state if available
	if err := b.LoadState(); err != nil {
		slog.Warn("Failed to load bot state", "error", err)
	}

	// Run the game loop
	if err := b.gameLoop(ctx); err != nil {
		// Save state before returning error
		if saveErr := b.SaveState(); saveErr != nil {
			slog.Warn("Failed to save bot state", "error", saveErr)
		}
		return err
	}

	return nil
}

// SaveState saves the current bot state to disk
func (b *Bot) SaveState() error {
	state := BotState{
		CategoryIndex: b.categoryIndex,
		JumpZoneList:  b.jumpZoneList,
		Server:        b.server,
		Zone:          b.zone,
		SaveMapId:     b.saveMapId,
		Timestamp:     time.Now().Unix(),
	}

	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal state: %w", err)
	}

	filename := fmt.Sprintf("bot_state_%d.json", b.clientIndex)
	if err := os.WriteFile(filename, data, 0644); err != nil {
		return fmt.Errorf("failed to write state file: %w", err)
	}

	slog.Debug("Bot state saved", "filename", filename)
	return nil
}

// LoadState loads the bot state from disk
func (b *Bot) LoadState() error {
	filename := fmt.Sprintf("bot_state_%d.json", b.clientIndex)
	data, err := os.ReadFile(filename)
	if err != nil {
		if os.IsNotExist(err) {
			return nil // No saved state is OK
		}
		return fmt.Errorf("failed to read state file: %w", err)
	}

	var state BotState
	if err := json.Unmarshal(data, &state); err != nil {
		return fmt.Errorf("failed to unmarshal state: %w", err)
	}

	// Restore state
	b.categoryIndex = state.CategoryIndex
	b.jumpZoneList = state.JumpZoneList
	b.server = state.Server
	b.zone = state.Zone
	b.saveMapId = state.SaveMapId

	slog.Info("Bot state loaded",
		"filename", filename,
		"category_index", b.categoryIndex,
		"server", b.server,
		"zone", b.zone)

	return nil
}

// GetClient returns the client configuration
func (b *Bot) GetClient() *config.Client {
	return &b.clientConfig.Clients[b.clientIndex]
}

// SetServer sets the server name
func (b *Bot) SetServer(server string) {
	b.server = server
}

// SetZone sets the zone
func (b *Bot) SetZone(zone string) {
	b.zone = zone
}

// GetZone returns the current zone
func (b *Bot) GetZone() string {
	return b.zone
}
