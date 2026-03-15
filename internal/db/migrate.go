package db

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
)

// RunMigrations applies all pending SQL migrations from the local migrations directory.
func RunMigrations(connStr string) error {
	migrationsPath, err := resolveMigrationsPath()
	if err != nil {
		return err
	}

	sourceURL := "file://" + migrationsPath
	databaseURL := connStr + "&x-migrations-table=rom_agent_schema_migrations"
	m, err := migrate.New(sourceURL, databaseURL)
	if err != nil {
		return fmt.Errorf("create migrator: %w", err)
	}
	defer func() {
		_, _ = m.Close()
	}()

	if err := m.Up(); err != nil && !errors.Is(err, migrate.ErrNoChange) {
		return fmt.Errorf("apply database migrations: %w", err)
	}
	return nil
}

func resolveMigrationsPath() (string, error) {
	candidates := []string{
		"migrations",
		filepath.Join("..", "..", "migrations"),
		filepath.Join("..", "migrations"),
	}

	for _, candidate := range candidates {
		info, err := os.Stat(candidate)
		if err != nil || !info.IsDir() {
			continue
		}

		absPath, err := filepath.Abs(candidate)
		if err != nil {
			return "", fmt.Errorf("resolve migrations path %q: %w", candidate, err)
		}
		return absPath, nil
	}

	return "", fmt.Errorf("resolve migrations path: no migrations directory found")
}
