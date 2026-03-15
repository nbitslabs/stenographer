package database

import (
	"context"
	"database/sql"
	"embed"
	"io/fs"

	"github.com/pressly/goose/v3"
	_ "modernc.org/sqlite"
)

//go:embed migrations/*.sql
var embedMigrations embed.FS

func Open(path string) (*sql.DB, error) {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, err
	}

	// SQLite pragmas for performance and correctness.
	for _, pragma := range []string{
		"PRAGMA journal_mode=WAL",
		"PRAGMA foreign_keys=ON",
		"PRAGMA busy_timeout=5000",
	} {
		if _, err := db.Exec(pragma); err != nil {
			db.Close()
			return nil, err
		}
	}

	if err := migrate(db); err != nil {
		db.Close()
		return nil, err
	}

	return db, nil
}

func migrate(db *sql.DB) error {
	migrationFS, err := fs.Sub(embedMigrations, "migrations")
	if err != nil {
		return err
	}

	provider, err := goose.NewProvider(goose.DialectSQLite3, db, migrationFS)
	if err != nil {
		return err
	}

	_, err = provider.Up(context.Background())
	return err
}
