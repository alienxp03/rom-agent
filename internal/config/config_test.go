package config

import (
	"strings"
	"testing"
	"time"
)

func TestValidateRequiresRuntimeServerForExchange(t *testing.T) {
	cfg := &Config{
		Clients: []Client{
			{
				Use:            true,
				EnableExchange: true,
				Account: Account{
					Sid:      "sid",
					ClientId: "client",
					MacKey:   "mac",
				},
			},
		},
		AuthBaseUrl: "https://example.com",
		ExchangeTarget: ExchangeTargetConfig{
			Market: "sea_shared",
		},
		ResultDatabase: DatabaseConfig{
			Host:   "localhost",
			DBName: "rom_agent",
		},
		SourceDatabase: DatabaseConfig{
			Host:   "localhost",
			DBName: "romhandbook",
		},
	}

	err := cfg.Validate()
	if err == nil || err.Error() != "runtime_server is required when exchange scanning is enabled" {
		t.Fatalf("Validate() error = %v, want missing runtime_server", err)
	}
}

func TestValidateRequiresExchangeTargetMarketForExchange(t *testing.T) {
	cfg := &Config{
		Clients: []Client{
			{
				Use:            true,
				EnableExchange: true,
				Account: Account{
					Sid:      "sid",
					ClientId: "client",
					MacKey:   "mac",
				},
			},
		},
		AuthBaseUrl:   "https://example.com",
		RuntimeServer: "sea_mp",
		ResultDatabase: DatabaseConfig{
			Host:   "localhost",
			DBName: "rom_agent",
		},
		SourceDatabase: DatabaseConfig{
			Host:   "localhost",
			DBName: "romhandbook",
		},
	}

	err := cfg.Validate()
	if err == nil || err.Error() != "exchange_target.market is required when exchange scanning is enabled" {
		t.Fatalf("Validate() error = %v, want missing exchange_target.market", err)
	}
}

func TestValidateRequiresRuntimeServerToResolveToExchangeMarket(t *testing.T) {
	cfg := &Config{
		Clients: []Client{
			{
				Use:            true,
				EnableExchange: true,
				Account: Account{
					Sid:      "sid",
					ClientId: "client",
					MacKey:   "mac",
				},
			},
		},
		AuthBaseUrl:   "https://example.com",
		RuntimeServer: "sea_mp",
		ExchangeTarget: ExchangeTargetConfig{
			Market: "sea_shared",
		},
		ExchangeMarketAliases: map[string]string{
			"sea_el": "sea_shared",
		},
		ResultDatabase: DatabaseConfig{
			Host:   "localhost",
			DBName: "rom_agent",
		},
		SourceDatabase: DatabaseConfig{
			Host:   "localhost",
			DBName: "romhandbook",
		},
	}

	if err := cfg.Validate(); err == nil {
		t.Fatal("Validate() error = nil, want runtime_server market mismatch")
	}
}

func TestDatabaseConfigReadOnlyURL(t *testing.T) {
	cfg := DatabaseConfig{
		Host:    "localhost",
		Port:    5432,
		User:    "reader",
		DBName:  "romhandbook",
		SSLMode: "disable",
	}

	got := cfg.ReadOnlyURL()
	want := "default_transaction_read_only=on"
	if !strings.Contains(got, want) {
		t.Fatalf("ReadOnlyURL() = %q, want substring %q", got, want)
	}
}

func TestExchangeIntervalsUseDefaults(t *testing.T) {
	cfg := &Config{}

	if got := cfg.ExchangeThingSnapshotRefreshInterval(); got != 12*time.Hour {
		t.Fatalf("ExchangeThingSnapshotRefreshInterval() = %v, want %v", got, 12*time.Hour)
	}
	if got := cfg.ExchangeRefreshInterval(); got != 5*time.Minute {
		t.Fatalf("ExchangeRefreshInterval() = %v, want %v", got, 5*time.Minute)
	}
	if got := cfg.BotRetryDelayInterval(); got != 40*time.Second {
		t.Fatalf("BotRetryDelayInterval() = %v, want %v", got, 40*time.Second)
	}
	if got := cfg.CombatTimeSettleDelayInterval(); got != 500*time.Millisecond {
		t.Fatalf("CombatTimeSettleDelayInterval() = %v, want %v", got, 500*time.Millisecond)
	}
	if got := cfg.ExchangeCloseDelayInterval(); got != 2*time.Second {
		t.Fatalf("ExchangeCloseDelayInterval() = %v, want %v", got, 2*time.Second)
	}
	if got := cfg.IdleLoopDelayInterval(); got != 1*time.Second {
		t.Fatalf("IdleLoopDelayInterval() = %v, want %v", got, 1*time.Second)
	}
	if got := cfg.BossQueryIntervalValue(); got != 60*time.Second {
		t.Fatalf("BossQueryIntervalValue() = %v, want %v", got, 60*time.Second)
	}
	if got := cfg.BossFastQueryWindowInterval(); got != 105*time.Minute {
		t.Fatalf("BossFastQueryWindowInterval() = %v, want %v", got, 105*time.Minute)
	}
	if got := cfg.BossWaveActiveQueryIntervalValue(); got != 5*time.Second {
		t.Fatalf("BossWaveActiveQueryIntervalValue() = %v, want %v", got, 5*time.Second)
	}
	if got := cfg.BossAllDeadQueryIntervalValue(); got != 5*time.Minute {
		t.Fatalf("BossAllDeadQueryIntervalValue() = %v, want %v", got, 5*time.Minute)
	}
	if got := cfg.BossWaveStartQueryIntervalValue(); got != 5*time.Second {
		t.Fatalf("BossWaveStartQueryIntervalValue() = %v, want %v", got, 5*time.Second)
	}
	if got := cfg.BossJumpMinBeforeWaveInterval(); got != 15*time.Minute {
		t.Fatalf("BossJumpMinBeforeWaveInterval() = %v, want %v", got, 15*time.Minute)
	}
	if got := cfg.BossJumpMaxBeforeWaveInterval(); got != 60*time.Minute {
		t.Fatalf("BossJumpMaxBeforeWaveInterval() = %v, want %v", got, 60*time.Minute)
	}
	if got := cfg.BossJumpIntervalValue(); got != 75*time.Minute {
		t.Fatalf("BossJumpIntervalValue() = %v, want %v", got, 75*time.Minute)
	}
	if got := cfg.PvpQueryIntervalValue(); got != 20*time.Minute {
		t.Fatalf("PvpQueryIntervalValue() = %v, want %v", got, 20*time.Minute)
	}
	if got := cfg.WocQueryIntervalValue(); got != 60*time.Minute {
		t.Fatalf("WocQueryIntervalValue() = %v, want %v", got, 60*time.Minute)
	}
	if got := cfg.WoeQueryIntervalValue(); got != 30*time.Minute {
		t.Fatalf("WoeQueryIntervalValue() = %v, want %v", got, 30*time.Minute)
	}
}
