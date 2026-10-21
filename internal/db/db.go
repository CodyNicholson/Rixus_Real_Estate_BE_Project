package db

import (
	"context"
	_ "embed"
	"log"

	"github.com/jackc/pgx/v5/pgxpool"
)

//go:embed schema.sql
var schema string

func Connect(ctx context.Context, url string) (*pgxpool.Pool, error) {
	pool, err := pgxpool.New(ctx, url)
	if err != nil {
		return nil, err
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, err
	}
	if _, err := pool.Exec(ctx, schema); err != nil {
		pool.Close()
		return nil, err
	}
	// Ensure testing data for rixusrealestate@gmail.com is seeded
	if err := SeedRixusRealEstateIfNotExists(ctx, pool); err != nil {
		log.Printf("database: notice during rixusrealestate seed: %v", err)
	}
	return pool, nil
}
