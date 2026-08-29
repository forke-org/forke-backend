// Copyright (c) 2026 Forke Inc. (https://www.forke.space/)
// Source-Available License (Non-Commercial / Fair Source).
// Open for inspection, learning, and non-commercial development.
// Commercial use, hosting, or resale without authorization is strictly prohibited.

package database

import (
	"context"
	_ "embed"
	"fmt"
	"log"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

//go:embed migrations/0000_forke_db.sql
var initialSchemaSQL string

type DB struct {
	Pool *pgxpool.Pool
}

func Connect(ctx context.Context, databaseURL string) (*DB, error) {
	config, err := pgxpool.ParseConfig(databaseURL)
	if err != nil {
		return nil, fmt.Errorf("unable to parse database config: %w", err)
	}

	config.MaxConns = 25
	config.MinConns = 2
	config.MaxConnLifetime = 1 * time.Hour
	config.MaxConnIdleTime = 30 * time.Minute

	pool, err := pgxpool.NewWithConfig(ctx, config)
	if err != nil {
		return nil, fmt.Errorf("unable to connect to database pool: %w", err)
	}

	// Test connection
	pingCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	if err := pool.Ping(pingCtx); err != nil {
		return nil, fmt.Errorf("database ping failed: %w", err)
	}

	log.Println("[Database] Successfully connected to PostgreSQL")

	db := &DB{Pool: pool}

	// Auto-run initial schema migration
	if err := db.Migrate(ctx); err != nil {
		log.Printf("[Database Warning] Migration notice: %v", err)
	}

	return db, nil
}

func (db *DB) Migrate(ctx context.Context) error {
	log.Println("[Database] Running schema migrations...")
	_, err := db.Pool.Exec(ctx, initialSchemaSQL)
	if err != nil {
		return fmt.Errorf("failed to apply migrations: %w", err)
	}
	log.Println("[Database] Migrations applied successfully")
	return nil
}

func (db *DB) Close() {
	if db.Pool != nil {
		db.Pool.Close()
		log.Println("[Database] Connection pool closed")
	}
}
