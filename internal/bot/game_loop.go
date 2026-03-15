package bot

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/alienxp03/rom-agent/internal/client"
	"github.com/alienxp03/rom-agent/internal/config"
)

// gameLoop implements the main game loop
func (b *Bot) gameLoop(ctx context.Context) error {
	clientCfg := b.GetClient()
	slog.Info("Client loop started", "name", clientCfg.Name)

	// Initialize game client
	b.gameClient = client.NewGameClient(
		b.blueberry,
		b.version,
		b.clientConfig.Lang,
		b.clientConfig.AppPreVersion,
		clientCfg.Account.SmDeviceId,
		b.clientConfig.GameLinegroup,
	)
	defer b.gameClient.Close()

	slog.Info("Attempting login", "character", clientCfg.Character)
	authClient := client.NewAuthClient(b.clientConfig, clientCfg.Account, b.blueberry, b.version)
	authData, err := authClient.Authenticate(ctx)
	if err != nil {
		return fmt.Errorf("auth failed: %w", err)
	}
	if err := b.gameClient.LoginWithAuth(ctx, authData, clientCfg.Account.DeviceId, clientCfg.Character); err != nil {
		return fmt.Errorf("login failed: %w", err)
	}

	// Post-login setup
	if err := b.gameClient.PostLogin(ctx); err != nil {
		return err
	}

	b.server = b.gameClient.GetServer()
	b.zone = b.gameClient.GetZoneIdStr()
	slog.Info("Logged in",
		"name", clientCfg.Name,
		"server", b.server,
		"zone", b.zone)

	// Setup MVP jump zones if configured
	var mvpJumpZones []string
	if clientCfg.EnableBoss && clientCfg.MvpJumpZonePattern != "" {
		zones, err := b.gameClient.GetMatchingZones(clientCfg.MvpJumpZonePattern)
		if err != nil {
			slog.Warn("Failed to get MVP zones", "error", err)
		} else if len(zones) > 0 {
			mvpJumpZones = zones
			slog.Info("MVP jump zones configured",
				"count", len(zones),
				"zones", zones)
		}
	}

	// Get combat time if combat is enabled
	if clientCfg.EnableCombat && clientCfg.Combat != nil {
		if err := b.gameClient.GetCombatTime(ctx); err != nil {
			slog.Warn("Failed to get combat time", "error", err)
		}
		time.Sleep(500 * time.Millisecond)
	}

	// Leave party if auto-party is enabled
	if clientCfg.AutoParty && b.gameClient.IsInParty() {
		if err := b.gameClient.LeaveParty(ctx); err != nil {
			slog.Warn("Failed to leave party", "error", err)
		}
	}

	// Enter main loop
	return b.mainLoop(ctx, clientCfg, mvpJumpZones)
}

// mainLoop implements the persistent query loop
func (b *Bot) mainLoop(ctx context.Context, clientCfg *config.Client, mvpJumpZones []string) error {
	// Loop state variables
	exchangeIsOpen := false
	lastBossQueryTime := time.Time{}
	nextBossWaveTime := time.Time{}
	lastAliveBossCount := int64(0)
	lastBossZoneJumpTime := time.Time{}
	lastPvpQueryTime := time.Time{}
	lastWocQueryTime := time.Time{}
	lastWoeQueryTime := time.Time{}

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		b.gameClient.ClearRecvBuffer()

		// Revive if dead
		if b.gameClient.IsDead() &&
			!clientCfg.EnableExchange &&
			!clientCfg.DoNotRevive &&
			!b.gameClient.IsInParty() {
			slog.Info("Attempting to revive dead character")
			if err := b.gameClient.ReviveToTown(ctx); err != nil {
				slog.Error("Revive failed", "error", err)
			} else {
				b.saveMapId = b.gameClient.GetMapId()
			}
		}

		// Move to correct zone if needed
		if clientCfg.SetZone != "" &&
			clientCfg.SetZone != b.zone &&
			len(b.jumpZoneList) == 0 {
			slog.Info("Jumping to configured zone", "zone", clientCfg.SetZone)
			if err := b.gameClient.JumpZone(ctx, clientCfg.SetZone); err != nil {
				slog.Error("Zone jump failed", "error", err)
			} else {
				b.zone = b.gameClient.GetZoneIdStr()
			}
		}

		// Check party status
		if clientCfg.AutoParty {
			if err := b.doParty(ctx); err != nil {
				slog.Warn("Party check failed", "error", err)
			}
		}

		// Sync map if needed
		if err := b.gameClient.SyncMapIfNeeded(ctx); err != nil {
			slog.Warn("Map sync failed", "error", err)
		}

		now := b.gameClient.GetServerTime()

		// Determine what to query
		shouldQueryBoss := b.shouldQueryBoss(
			clientCfg, now, lastBossQueryTime, nextBossWaveTime, lastAliveBossCount)
		shouldQueryBossAndJumpZones := b.shouldQueryBossAndJumpZones(
			mvpJumpZones, now, nextBossWaveTime, lastBossZoneJumpTime)
		shouldQueryPvp := b.shouldQueryPvp(clientCfg, now, lastPvpQueryTime)
		shouldQueryWoc := b.shouldQueryWoc(clientCfg, now, lastWocQueryTime)
		shouldQueryWoe := b.shouldQueryWoe(clientCfg, now, lastWoeQueryTime)

		// Close exchange if we need to do other queries
		if shouldQueryBoss || shouldQueryPvp || shouldQueryWoc ||
			shouldQueryWoe || clientCfg.EnableAuction || clientCfg.EnableCombat {
			if exchangeIsOpen {
				b.gameClient.ExitExchange(ctx)
				exchangeIsOpen = false
				time.Sleep(2 * time.Second)
			}
		}

		// Query boss list
		if shouldQueryBoss {
			lastBossQueryTime = now
			// TODO: Implement boss query
			slog.Debug("Boss query triggered")
		}

		// Jump zones for boss survey
		if shouldQueryBossAndJumpZones {
			lastBossZoneJumpTime = now
			// TODO: Implement boss zone jumping
			slog.Debug("Boss zone jumping triggered")
		}

		// Query PvP
		if shouldQueryPvp {
			lastPvpQueryTime = now
			if err := b.doPvp(ctx); err != nil {
				slog.Error("PvP query failed", "error", err)
			}
		}

		// Query WoC
		if shouldQueryWoc {
			lastWocQueryTime = now
			if err := b.doWoc(ctx); err != nil {
				slog.Error("WoC query failed", "error", err)
			}
		}

		// Query WoE
		if shouldQueryWoe {
			lastWoeQueryTime = now
			if err := b.doWoe(ctx); err != nil {
				slog.Error("WoE query failed", "error", err)
			}
		}

		// Query auction
		if clientCfg.EnableAuction {
			if err := b.doAuction(ctx); err != nil {
				if clientCfg.EnableExchange {
					slog.Error("Auction query failed, continuing", "error", err)
				} else {
					return err
				}
			}
		}

		// Check party status again
		if clientCfg.AutoParty {
			b.doParty(ctx)
		}

		// Combat
		if clientCfg.EnableCombat {
			if err := b.doCombat(ctx, clientCfg.Combat, clientCfg.AutoParty); err != nil {
				slog.Error("Combat failed", "error", err)
			}
		}

		// Exchange scraping
		if !clientCfg.EnableExchange {
			if !clientCfg.EnableCombat {
				time.Sleep(1 * time.Second)
			}
			continue
		}

		// Open exchange if not already open
		if !exchangeIsOpen {
			if err := b.openExchange(ctx); err != nil {
				slog.Error("Failed to open exchange", "error", err)
				continue
			}
			exchangeIsOpen = true
		}

		// Run exchange scraping
		if err := b.doExchange(ctx); err != nil {
			slog.Error("Exchange scraping failed", "error", err)
			return err
		}

		// Save state after completing exchange
		if err := b.SaveState(); err != nil {
			slog.Warn("Failed to save state", "error", err)
		}

		// Wait before starting next exchange cycle
		// Default 30 minutes between full scans to avoid spamming
		exchangeCycleDelay := 30 * time.Minute
		slog.Info("Exchange cycle completed, waiting before next cycle",
			"delay_minutes", int(exchangeCycleDelay.Minutes()))

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(exchangeCycleDelay):
			// Continue to next cycle
		}
	}
}

// Timing helper methods

func (b *Bot) shouldQueryBoss(
	clientCfg *config.Client,
	now, lastQueryTime, nextWaveTime time.Time,
	lastAliveCount int64,
) bool {
	if !clientCfg.EnableBoss {
		return false
	}

	bossQueryInterval := 60 * time.Second // 1 min default
	fastQueryWindow := 105 * time.Minute  // 105 mins before next wave

	if !nextWaveTime.IsZero() {
		if now.After(nextWaveTime) || now.Equal(nextWaveTime) {
			bossQueryInterval = 5 * time.Second // Already next wave
		} else if lastAliveCount == 0 && now.Before(nextWaveTime) {
			bossQueryInterval = 5 * time.Minute // All bosses dead
		} else if now.Before(nextWaveTime) && nextWaveTime.Sub(now) > fastQueryWindow {
			bossQueryInterval = 5 * time.Second // Start of wave
		}
	}

	return lastQueryTime.IsZero() || now.Sub(lastQueryTime) > bossQueryInterval
}

func (b *Bot) shouldQueryBossAndJumpZones(
	mvpJumpZones []string,
	now, nextWaveTime, lastJumpTime time.Time,
) bool {
	if len(mvpJumpZones) == 0 {
		return false
	}

	// Only jump 60-15 mins before next wave
	if !nextWaveTime.IsZero() && now.Before(nextWaveTime) {
		timeUntilWave := nextWaveTime.Sub(now)
		if timeUntilWave > 15*time.Minute && timeUntilWave < 60*time.Minute {
			jumpInterval := 75 * time.Minute
			return lastJumpTime.IsZero() || now.Sub(lastJumpTime) > jumpInterval
		}
	}

	return false
}

func (b *Bot) shouldQueryPvp(clientCfg *config.Client, now, lastQueryTime time.Time) bool {
	if !clientCfg.EnablePvp {
		return false
	}
	pvpQueryInterval := 20 * time.Minute
	return lastQueryTime.IsZero() || now.Sub(lastQueryTime) > pvpQueryInterval
}

func (b *Bot) shouldQueryWoc(clientCfg *config.Client, now, lastQueryTime time.Time) bool {
	if !clientCfg.EnableWoc {
		return false
	}
	wocQueryInterval := 60 * time.Minute
	return lastQueryTime.IsZero() || now.Sub(lastQueryTime) > wocQueryInterval
}

func (b *Bot) shouldQueryWoe(clientCfg *config.Client, now, lastQueryTime time.Time) bool {
	if !clientCfg.EnableWoe {
		return false
	}
	woeQueryInterval := 30 * time.Minute
	return lastQueryTime.IsZero() || now.Sub(lastQueryTime) > woeQueryInterval
}

func (b *Bot) doParty(ctx context.Context) error {
	if b.gameClient.IsInParty() && b.gameClient.GetPartyMemberCount() <= 1 {
		if err := b.gameClient.LeaveParty(ctx); err != nil {
			return err
		}
	}

	if b.gameClient.HasPendingPartyInvite() {
		if err := b.gameClient.AcceptPartyInvite(ctx); err != nil {
			return err
		}
	}

	return nil
}

// Placeholder implementations for sub-tasks

func (b *Bot) doPvp(ctx context.Context) error {
	// TODO: Implement PvP query
	return nil
}

func (b *Bot) doWoc(ctx context.Context) error {
	// TODO: Implement WoC query
	return nil
}

func (b *Bot) doWoe(ctx context.Context) error {
	// TODO: Implement WoE query
	return nil
}

func (b *Bot) doAuction(ctx context.Context) error {
	// TODO: Implement auction query
	return nil
}

func (b *Bot) doCombat(ctx context.Context, combat *config.CombatConfig, autoParty bool) error {
	// TODO: Implement combat logic
	return nil
}

func (b *Bot) openExchange(ctx context.Context) error {
	// Find exchange NPC
	exchangeNpcIds := []int64{2159, 6517, 821506}
	var exchangeNpcGuid int64

	for _, npcId := range exchangeNpcIds {
		guid := b.gameClient.GetNpcGuid(npcId)
		if guid > 0 {
			exchangeNpcGuid = guid
			break
		}
	}

	if exchangeNpcGuid > 0 {
		if err := b.gameClient.ClickAndVisitNpc(ctx, exchangeNpcGuid); err != nil {
			slog.Warn("Failed to click NPC", "error", err)
		}
	} else {
		slog.Warn("Could not find exchange NPC, proceeding anyway")
	}

	return b.gameClient.OpenExchange(ctx)
}
