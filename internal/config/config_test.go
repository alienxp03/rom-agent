package config

import (
	"strings"
	"testing"
)

func TestValidateRequiresActiveServerForExchange(t *testing.T) {
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
	if err == nil || err.Error() != "active_server is required when exchange scanning is enabled" {
		t.Fatalf("Validate() error = %v, want missing active_server", err)
	}
}

func TestValidateRejectsUnsupportedServer(t *testing.T) {
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
		AuthBaseUrl:  "https://example.com",
		ActiveServer: "rom_el",
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
		t.Fatal("Validate() error = nil, want unsupported server error")
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
