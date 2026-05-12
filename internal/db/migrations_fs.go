package db

import (
	"io/fs"
)

// MigrationsFS returns the postgres migrations sub-FS from an embedded migrations FS.
func MigrationsFS(embedded fs.FS) (fs.FS, error) {
	return fs.Sub(embedded, "postgres/migrations")
}
