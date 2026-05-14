// Package database provides initialised clients for PostgreSQL and Redis.
// Both clients are created once at startup and shared across the application
// via dependency injection — never via package-level globals.
package database

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/mridha/businesssaas/internal/config"
)

// NewPostgresPool creates and validates a pgxpool connection pool using
// the provided database configuration.
//
// The pool is configured with:
//   - MaxConns: DATABASE_MAX_OPEN_CONNS
//   - MinConns: DATABASE_MAX_IDLE_CONNS
//   - MaxConnLifetime: DATABASE_CONN_MAX_LIFETIME
//
// The caller is responsible for closing the pool on shutdown via pool.Close().
func NewPostgresPool(ctx context.Context, cfg config.DatabaseConfig) (*pgxpool.Pool, error) {
	poolCfg, err := pgxpool.ParseConfig(cfg.DSN())
	if err != nil {
		return nil, fmt.Errorf("postgres: failed to parse DSN: %w", err)
	}

	// Connection pool sizing
	poolCfg.MaxConns = int32(cfg.MaxOpenConns) //nolint:gosec // bounds checked in config
	poolCfg.MinConns = int32(cfg.MaxIdleConns) //nolint:gosec // bounds checked in config
	poolCfg.MaxConnLifetime = cfg.ConnMaxLifetime
	poolCfg.MaxConnIdleTime = 30 * time.Minute
	poolCfg.HealthCheckPeriod = 1 * time.Minute

	pool, err := pgxpool.NewWithConfig(ctx, poolCfg)
	if err != nil {
		return nil, fmt.Errorf("postgres: failed to create pool: %w", err)
	}

	// Verify connectivity — fail fast at startup rather than at first request
	pingCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	if err := pool.Ping(pingCtx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("postgres: ping failed: %w", err)
	}

	stat := pool.Stat()
	slog.Info("postgres connected",
		"host", cfg.Host,
		"port", cfg.Port,
		"database", cfg.Name,
		"max_conns", stat.MaxConns(),
	)

	return pool, nil
}

// Ping checks whether the PostgreSQL connection pool is healthy.
// Returns nil if the database is reachable, an error otherwise.
// Used by the health endpoint.
func Ping(ctx context.Context, pool *pgxpool.Pool) error {
	pingCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()

	return pool.Ping(pingCtx)
}
