package config

import (
	"fmt"
	"net/url"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// Account holds account credentials
type Account struct {
	Sid        string `yaml:"sid"`
	ClientId   string `yaml:"client_id"`
	MacKey     string `yaml:"mac_key"`
	DeviceId   string `yaml:"device_id"`
	SmDeviceId string `yaml:"sm_device_id"`
}

// CombatConfig holds combat automation settings
type CombatConfig struct {
	SkillId            int     `yaml:"skill_id"`
	SkillSpCost        int     `yaml:"skill_sp_cost"`
	SkillTargetDamage  int     `yaml:"skill_target_damage"`
	AttackRangeM       float64 `yaml:"attack_range_m"`
	AttackSpeedMs      int     `yaml:"attack_speed_ms"`
	AttackCountPerLoop int     `yaml:"attack_count_per_loop"`
	ShouldTargetBoss   bool    `yaml:"should_target_boss"`
	TargetMobIds       []int   `yaml:"target_mob_ids"`
	GoToMap            int     `yaml:"go_to_map"`
}

// Client holds individual client configuration
type Client struct {
	Name               string        `yaml:"name"`
	Account            Account       `yaml:"account"`
	Character          string        `yaml:"character"`
	Use                bool          `yaml:"use"`
	SetZone            string        `yaml:"set_zone"`
	MvpJumpZonePattern string        `yaml:"mvp_jump_zone_pattern"`
	Combat             *CombatConfig `yaml:"combat"`
	DoNotRevive        bool          `yaml:"do_not_revive"`
	AutoParty          bool          `yaml:"auto_party"`
	EnableExchange     bool          `yaml:"enable_exchange"`
	EnableAuction      bool          `yaml:"enable_auction"`
	EnableBoss         bool          `yaml:"enable_boss"`
	EnablePvp          bool          `yaml:"enable_pvp"`
	EnableWoe          bool          `yaml:"enable_woe"`
	EnableWoc          bool          `yaml:"enable_woc"`
	EnableCombat       bool          `yaml:"enable_combat"`
}

// DatabaseConfig holds database connection settings
type DatabaseConfig struct {
	Host     string `yaml:"host"`
	Port     int    `yaml:"port"`
	User     string `yaml:"user"`
	Password string `yaml:"password"`
	DBName   string `yaml:"dbname"`
	SSLMode  string `yaml:"sslmode"`
}

var SupportedServers = []string{
	"sea_el",
	"sea_mp",
	"sea_mof",
	"sea_vg",
	"eu_el",
	"rom_classic",
}

// Config holds the main application configuration
type Config struct {
	Clients        []Client       `yaml:"clients"`
	Database       DatabaseConfig `yaml:"database"`
	SourceDatabase DatabaseConfig `yaml:"source_database"`
	ResultDatabase DatabaseConfig `yaml:"result_database"`
	ActiveServer   string         `yaml:"active_server"`
	Lang           int            `yaml:"lang"`
	AppPreVersion  int            `yaml:"app_pre_version"`
	ClientVersion  int            `yaml:"client_version"`
	Plat           int            `yaml:"plat"`
	ClientCode     int            `yaml:"client_code"`
	AuthBaseUrl    string         `yaml:"auth_base_url"`
	GameLinegroup  int            `yaml:"game_linegroup"`
}

// Load reads and parses the configuration file
func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read config: %w", err)
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("failed to parse config: %w", err)
	}

	databasePath := filepath.Join(filepath.Dir(path), "database.yaml")
	dbSettings, err := loadDatabaseSettings(databasePath)
	if err != nil {
		return nil, err
	}
	cfg.Database = dbSettings.Database
	if cfg.SourceDatabase.DBName == "" {
		cfg.SourceDatabase = dbSettings.SourceDatabase
	}
	if cfg.ResultDatabase.DBName == "" {
		cfg.ResultDatabase = dbSettings.ResultDatabase
	}

	if err := cfg.Validate(); err != nil {
		return nil, err
	}

	return &cfg, nil
}

type databaseSettings struct {
	Database       DatabaseConfig
	SourceDatabase DatabaseConfig
	ResultDatabase DatabaseConfig
}

func loadDatabaseSettings(path string) (databaseSettings, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return databaseSettings{}, fmt.Errorf("failed to read database config %q: %w", path, err)
	}

	var wrapper struct {
		Database       DatabaseConfig `yaml:"database"`
		SourceDatabase DatabaseConfig `yaml:"source_database"`
		ResultDatabase DatabaseConfig `yaml:"result_database"`
	}
	if err := yaml.Unmarshal(data, &wrapper); err != nil {
		return databaseSettings{}, fmt.Errorf("failed to parse database config %q: %w", path, err)
	}

	settings := databaseSettings{
		Database:       wrapper.Database,
		SourceDatabase: wrapper.SourceDatabase,
		ResultDatabase: wrapper.ResultDatabase,
	}
	switch {
	case settings.ResultDatabase.DBName != "":
	case settings.Database.DBName != "":
		settings.ResultDatabase = settings.Database
	default:
		return databaseSettings{}, fmt.Errorf("database config %q is missing result_database or database", path)
	}

	if settings.SourceDatabase.DBName == "" {
		settings.SourceDatabase = settings.Database
	}
	if settings.Database.DBName == "" {
		settings.Database = settings.ResultDatabase
	}
	return settings, nil
}

// Validate checks if the configuration is valid
func (c *Config) Validate() error {
	if len(c.Clients) == 0 {
		return fmt.Errorf("no clients configured")
	}

	for i, client := range c.Clients {
		if client.Use {
			if client.Account.Sid == "" {
				return fmt.Errorf("client %d: sid is required", i)
			}
			if client.Account.ClientId == "" {
				return fmt.Errorf("client %d: client_id is required", i)
			}
			if client.Account.MacKey == "" {
				return fmt.Errorf("client %d: mac_key is required", i)
			}
		}
	}

	if c.AuthBaseUrl == "" {
		return fmt.Errorf("auth_base_url is required")
	}

	if c.usesExchange() {
		if c.ActiveServer == "" {
			return fmt.Errorf("active_server is required when exchange scanning is enabled")
		}
		if !isSupportedServer(c.ActiveServer) {
			return fmt.Errorf("active_server %q is not supported; expected one of %v", c.ActiveServer, SupportedServers)
		}
		if err := validateDatabaseConfig("source_database", c.GetSourceDatabaseConfig()); err != nil {
			return err
		}
		if err := validateDatabaseConfig("result_database", c.GetResultDatabaseConfig()); err != nil {
			return err
		}
		return nil
	}

	if err := validateDatabaseConfig("database", c.GetResultDatabaseConfig()); err != nil {
		return err
	}

	return nil
}

// GetDatabaseURL returns the PostgreSQL connection string
func (c *Config) GetDatabaseURL() string {
	return c.GetResultDatabaseConfig().URL()
}

func (c DatabaseConfig) URLWithDBName(dbName string) string {
	copy := c
	copy.DBName = dbName
	return copy.URL()
}

func (c DatabaseConfig) URL() string {
	sslMode := c.SSLMode
	if sslMode == "" {
		sslMode = "disable"
	}

	user := url.User(c.User)
	if c.Password != "" {
		user = url.UserPassword(c.User, c.Password)
	}

	return (&url.URL{
		Scheme: "postgres",
		User:   user,
		Host:   fmt.Sprintf("%s:%d", c.Host, c.Port),
		Path:   c.DBName,
		RawQuery: url.Values{
			"sslmode": []string{sslMode},
		}.Encode(),
	}).String()
}

func (c DatabaseConfig) ReadOnlyURL() string {
	values := url.Values{}
	parsed, err := url.Parse(c.URL())
	if err != nil {
		return c.URL()
	}
	values = parsed.Query()
	values.Set("default_transaction_read_only", "on")
	parsed.RawQuery = values.Encode()
	return parsed.String()
}

func (c *Config) GetSourceDatabaseConfig() DatabaseConfig {
	if c.SourceDatabase.DBName != "" {
		return c.SourceDatabase
	}
	return c.Database
}

func (c *Config) GetResultDatabaseConfig() DatabaseConfig {
	if c.ResultDatabase.DBName != "" {
		return c.ResultDatabase
	}
	return c.Database
}

func (c *Config) usesExchange() bool {
	for _, client := range c.Clients {
		if client.Use && client.EnableExchange {
			return true
		}
	}
	return false
}

func validateDatabaseConfig(name string, cfg DatabaseConfig) error {
	if cfg.Host == "" {
		return fmt.Errorf("%s host is required", name)
	}
	if cfg.DBName == "" {
		return fmt.Errorf("%s dbname is required", name)
	}
	return nil
}

func isSupportedServer(server string) bool {
	for _, supported := range SupportedServers {
		if server == supported {
			return true
		}
	}
	return false
}
