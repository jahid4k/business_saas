package database

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/redis/go-redis/v9"

	"github.com/mridha/businesssaas/internal/config"
)

// NewRedisClient creates and validates a Redis client using the provided
// Redis configuration.
//
// The caller is responsible for closing the client on shutdown
// via client.Close().
func NewRedisClient(ctx context.Context, cfg config.RedisConfig) (*redis.Client, error) {
	client := redis.NewClient(&redis.Options{
		Addr:     cfg.Addr(),
		Password: cfg.Password,
		DB:       cfg.DB,

		// Connection pool settings
		PoolSize:        10,
		MinIdleConns:    2,
		MaxIdleConns:    5,
		ConnMaxLifetime: 30 * time.Minute,
		ConnMaxIdleTime: 10 * time.Minute,

		// Timeouts
		DialTimeout:  5 * time.Second,
		ReadTimeout:  3 * time.Second,
		WriteTimeout: 3 * time.Second,
	})

	// Verify connectivity at startup
	pingCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	if err := client.Ping(pingCtx).Err(); err != nil {
		_ = client.Close()
		return nil, fmt.Errorf("redis: ping failed: %w", err)
	}

	slog.Info("redis connected",
		"addr", cfg.Addr(),
		"db", cfg.DB,
	)

	return client, nil
}

// PingRedis checks whether the Redis client is healthy.
// Returns nil if reachable, an error otherwise.
// Used by the health endpoint.
func PingRedis(ctx context.Context, client *redis.Client) error {
	pingCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()

	return client.Ping(pingCtx).Err()
}
