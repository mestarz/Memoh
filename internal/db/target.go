package db

import (
	"fmt"
	"strings"

	"github.com/memohai/memoh/internal/config"
)

// DriverPostgres is the only supported database driver.
const DriverPostgres = "postgres"

type MigrationTarget struct {
	Driver string
	DSN    string
}

func DriverFromConfig(cfg config.Config) string {
	return strings.TrimSpace(strings.ToLower(cfg.Database.DriverOrDefault()))
}

func MigrationTargetFromConfig(cfg config.Config) (MigrationTarget, error) {
	if driver := DriverFromConfig(cfg); driver != DriverPostgres {
		return MigrationTarget{}, fmt.Errorf("unsupported database driver %q (only %q is supported)", driver, DriverPostgres)
	}
	return MigrationTarget{Driver: DriverPostgres, DSN: DSN(cfg.Postgres)}, nil
}
