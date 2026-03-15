package main

import (
	"context"
	"encoding/json"
	"flag"
	"log/slog"
	"os"
	"os/signal"
	"path/filepath"
	"sync"
	"syscall"

	"github.com/alienxp03/rom-agent/internal/bot"
	"github.com/alienxp03/rom-agent/internal/config"
	"github.com/alienxp03/rom-agent/internal/db"
	"github.com/alienxp03/rom-agent/internal/resources"
)

var (
	configPath   = flag.String("config", "config/config.yaml", "Path to configuration file")
	debug        = flag.Bool("debug", false, "Enable debug logging")
	exchangeOnly = flag.Bool("exchange-only", false, "Run focused exchange capture only")
)

func main() {
	flag.Parse()

	// Setup logging
	logLevel := slog.LevelInfo
	if *debug {
		logLevel = slog.LevelDebug
	}

	handler := slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: logLevel,
	})
	slog.SetDefault(slog.New(handler))

	slog.Info("ROM Agent starting",
		"config", *configPath,
		"debug", *debug,
		"exchange_only", *exchangeOnly)

	// Load configuration
	cfg, err := config.Load(*configPath)
	if err != nil {
		slog.Error("Failed to load configuration", "error", err)
		os.Exit(1)
	}
	if *exchangeOnly {
		cfg = focusedExchangeConfig(cfg)
	}

	slog.Info("Configuration loaded",
		"clients", len(cfg.Clients),
		"auth_url", cfg.AuthBaseUrl,
		"exchange_only", *exchangeOnly)

	resultDBCfg := cfg.GetResultDatabaseConfig()
	sourceDBCfg := cfg.GetSourceDatabaseConfig()

	if err := db.EnsureDatabaseExists(resultDBCfg); err != nil {
		slog.Error("Failed to ensure result database exists", "error", err)
		os.Exit(1)
	}

	resultDBConnStr := cfg.GetDatabaseURL()
	if err := db.RunMigrations(resultDBConnStr); err != nil {
		slog.Error("Failed to initialize result database schema", "error", err)
		os.Exit(1)
	}

	resultDatabase, err := db.Open(resultDBConnStr)
	if err != nil {
		slog.Error("Failed to connect to result database", "error", err)
		os.Exit(1)
	}
	defer resultDatabase.Close()

	sourceDatabase, err := db.Open(sourceDBCfg.ReadOnlyURL())
	if err != nil {
		slog.Error("Failed to connect to source database", "error", err)
		os.Exit(1)
	}
	defer sourceDatabase.Close()

	slog.Info("Connected to databases",
		"active_server", cfg.ActiveServer,
		"source_host", sourceDBCfg.Host,
		"source_dbname", sourceDBCfg.DBName,
		"result_host", resultDBCfg.Host,
		"result_dbname", resultDBCfg.DBName)

	slog.Info("Result database schema initialized")

	// Setup context with cancellation
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Setup signal handling
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	go func() {
		sig := <-sigChan
		slog.Info("Received signal, shutting down", "signal", sig)
		cancel()
	}()

	blueberry, version := loadGameVersion(cfg)

	// Start bots for each enabled client
	exchangeDB := db.NewExchangeDb(resultDatabase)
	scanTargetDB := db.NewScanTargetStore(sourceDatabase)
	scanResultDB := db.NewScanResultStore(resultDatabase)
	var wg sync.WaitGroup
	for i, client := range cfg.Clients {
		if !client.Use {
			slog.Info("Skipping disabled client", "name", client.Name)
			continue
		}

		slog.Info("Starting bot",
			"index", i,
			"name", client.Name,
			"character", client.Character)

		// Create bot instance
		b := bot.New(
			cfg,
			i,
			blueberry,
			version,
			resources.AllCategories,
			exchangeDB,
			scanTargetDB,
			scanResultDB,
		)

		// Run bot in goroutine
		wg.Add(1)
		go func(botInstance *bot.Bot, clientIndex int) {
			defer wg.Done()
			if err := botInstance.Run(ctx); err != nil {
				if err != context.Canceled {
					slog.Error("Bot stopped with error",
						"client", cfg.Clients[clientIndex].Name,
						"error", err)
				}
			}
		}(b, i)
	}

	// Wait for context cancellation
	<-ctx.Done()
	wg.Wait()
	slog.Info("ROM Agent stopped")
}

func focusedExchangeConfig(cfg *config.Config) *config.Config {
	clone := *cfg
	clone.Clients = make([]config.Client, len(cfg.Clients))
	copy(clone.Clients, cfg.Clients)

	for i := range clone.Clients {
		clientCfg := clone.Clients[i]
		clientCfg.AutoParty = false
		clientCfg.EnableAuction = false
		clientCfg.EnableBoss = false
		clientCfg.EnablePvp = false
		clientCfg.EnableWoe = false
		clientCfg.EnableWoc = false
		clientCfg.EnableCombat = false
		clientCfg.Combat = nil
		clientCfg.EnableExchange = clientCfg.Use
		clone.Clients[i] = clientCfg
	}

	return &clone
}

func loadGameVersion(cfg *config.Config) (int, int) {
	type blueberryFile struct {
		Blueberry int `json:"blueberry"`
		Version   int `json:"version"`
	}

	paths := []string{
		filepath.Join("..", "src", "resources", "blueberry.json"),
		filepath.Join("src", "resources", "blueberry.json"),
	}

	for _, path := range paths {
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}

		var meta blueberryFile
		if err := json.Unmarshal(data, &meta); err != nil {
			slog.Warn("Failed to parse blueberry metadata", "path", path, "error", err)
			break
		}

		version := meta.Version
		if cfg.ClientVersion > 0 {
			version = cfg.ClientVersion
		}

		slog.Info("Loaded game version metadata",
			"path", path,
			"blueberry", meta.Blueberry,
			"version", version)
		return meta.Blueberry, version
	}

	version := cfg.ClientVersion
	if version < 0 {
		version = 0
	}
	slog.Warn("Falling back to config/default game version metadata", "blueberry", 0, "version", version)
	return 0, version
}
