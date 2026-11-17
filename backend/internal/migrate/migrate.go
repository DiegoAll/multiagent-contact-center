// Package migrate aplica las migraciones de goose embebidas en el binario.
package migrate

import (
	"database/sql"
	"fmt"

	"github.com/pressly/goose/v3"

	"github.com/diego-all/atento-agent-lab/backend/migrations"
)

// Up aplica todas las migraciones pendientes.
func Up(db *sql.DB) error {
	goose.SetBaseFS(migrations.FS)
	if err := goose.SetDialect("postgres"); err != nil {
		return fmt.Errorf("goose dialect: %w", err)
	}
	if err := goose.Up(db, "."); err != nil {
		return fmt.Errorf("goose up: %w", err)
	}
	return nil
}
