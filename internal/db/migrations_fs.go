package db

import (
	"fmt"
	"io/fs"

	"github.com/memohai/memoh/internal/config"
)

func MigrationsFSForConfig(cfg config.Config, embedded fs.FS) (fs.FS, error) {
	if driver := DriverFromConfig(cfg); driver != DriverPostgres {
		return nil, fmt.Errorf("unsupported database driver %q (only %q is supported)", driver, DriverPostgres)
	}
	return fs.Sub(embedded, "postgres/migrations")
}
