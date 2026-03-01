// Package store manages the database and Redis client lifecycle.
package store

import (
	"context"
	"fmt"

	_ "github.com/lib/pq" // postgres driver
	"github.com/redis/go-redis/v9"

	"github.com/liukai/farmer/server/config"
	"github.com/liukai/farmer/server/ent"
)

// Open creates an Ent client backed by PostgreSQL, runs auto-migration,
// and returns the ready-to-use client.
func Open(cfg *config.Config) (*ent.Client, error) {
	client, err := ent.Open("postgres", cfg.Database.DSN)
	if err != nil {
		return nil, fmt.Errorf("store: open db: %w", err)
	}

	if err := client.Schema.Create(context.Background()); err != nil {
		client.Close()
		return nil, fmt.Errorf("store: auto-migrate: %w", err)
	}

	return client, nil
}

// OpenRedis creates a go-redis client and verifies connectivity.
// Returns (nil, nil) if Redis address is empty so callers can handle gracefully.
func OpenRedis(cfg *config.Config) (*redis.Client, error) {
	if cfg.Redis.Addr == "" {
		return nil, nil
	}
	rdb := redis.NewClient(&redis.Options{
		Addr:     cfg.Redis.Addr,
		Password: cfg.Redis.Password,
		DB:       cfg.Redis.DB,
	})
	ctx := context.Background()
	if err := rdb.Ping(ctx).Err(); err != nil {
		// Non-fatal: server starts without Redis; caching/LLM quota disabled.
		rdb.Close()
		return nil, nil
	}
	return rdb, nil
}
