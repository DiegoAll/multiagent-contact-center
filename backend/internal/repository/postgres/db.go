// Package postgres contiene las implementaciones concretas de los
// repositorios sobre PostgreSQL (database/sql + pgx).
package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib" // registra el driver "pgx"

	"github.com/diego-all/atento-agent-lab/backend/internal/domain"
)

// Open abre el pool de conexiones y espera a que la base responda.
func Open(ctx context.Context, url string) (*sql.DB, error) {
	db, err := sql.Open("pgx", url)
	if err != nil {
		return nil, fmt.Errorf("open db: %w", err)
	}
	db.SetMaxOpenConns(10)
	db.SetMaxIdleConns(5)
	db.SetConnMaxLifetime(30 * time.Minute)

	var lastErr error
	for attempt := 1; attempt <= 10; attempt++ {
		pingCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
		lastErr = db.PingContext(pingCtx)
		cancel()
		if lastErr == nil {
			return db, nil
		}
		select {
		case <-ctx.Done():
			_ = db.Close()
			return nil, ctx.Err()
		case <-time.After(time.Duration(attempt) * 500 * time.Millisecond):
		}
	}
	_ = db.Close()
	return nil, fmt.Errorf("ping db: %w", lastErr)
}

// scanner abstrae *sql.Row y *sql.Rows para reutilizar el escaneo.
type scanner interface {
	Scan(dest ...any) error
}

// mapErr traduce sql.ErrNoRows a domain.ErrNotFound.
func mapErr(err error) error {
	if errors.Is(err, sql.ErrNoRows) {
		return domain.ErrNotFound
	}
	return err
}
