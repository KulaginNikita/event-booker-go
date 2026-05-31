package migrator

import (
	"database/sql"
	"fmt"

	"github.com/pressly/goose/v3"
)

type Migrator struct {
	db  *sql.DB
	dir string
}

func NewMigrator(db *sql.DB, dir string) *Migrator {
	return &Migrator{db: db, dir: dir}
}

func (m *Migrator) Up() error {
	if m.db == nil {
		return fmt.Errorf("db is nil")
	}
	if m.dir == "" {
		return fmt.Errorf("migration dir is empty")
	}
	if err := goose.SetDialect("postgres"); err != nil {
		return fmt.Errorf("set goose dialect: %w", err)
	}
	if err := goose.Up(m.db, m.dir); err != nil {
		return fmt.Errorf("goose up: %w", err)
	}
	return nil
}
