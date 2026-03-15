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
	if got := cfg.ExchangeTargetRefreshInterval(); got != 5*time.Minute {
		t.Fatalf("ExchangeTargetRefreshInterval() = %v, want %v", got, 5*time.Minute)
	}
	if got := cfg.ExchangeLowResultBackoffInterval(); got != 10*time.Minute {
		t.Fatalf("ExchangeLowResultBackoffInterval() = %v, want %v", got, 10*time.Minute)
	}
	if got := cfg.ExchangeLowResultThresholdValue(); got != 10 {
		t.Fatalf("ExchangeLowResultThresholdValue() = %d, want 10", got)
	}
}
