package db

import (
	"context"
	_ "embed"
	"fmt"
	"log"

	"github.com/jackc/pgx/v5/pgxpool"
)

//go:embed seed_010.sql
var seedRixusRealEstateSQL string

// SeedRixusRealEstate populates comprehensive financial and tenant testing data for rixusrealestate@gmail.com.
func SeedRixusRealEstate(ctx context.Context, pool *pgxpool.Pool) error {
	if pool == nil {
		return fmt.Errorf("database pool is nil")
	}
	_, err := pool.Exec(ctx, seedRixusRealEstateSQL)
	if err != nil {
		return fmt.Errorf("seed rixusrealestate testing data: %w", err)
	}
	return nil
}

// SeedRixusRealEstateIfNotExists checks if properties exist for rixusrealestate@gmail.com
// and seeds the testing data if not already present.
func SeedRixusRealEstateIfNotExists(ctx context.Context, pool *pgxpool.Pool) error {
	if pool == nil {
		return nil
	}
	var propCount int
	err := pool.QueryRow(ctx, `
		SELECT COUNT(p.id) 
		FROM properties p 
		JOIN users u ON u.id = p.user_id 
		WHERE u.email = 'rixusrealestate@gmail.com'
	`).Scan(&propCount)
	if err == nil && propCount > 0 {
		return nil
	}
	log.Printf("database: auto-seeding financial and tenant testing data for rixusrealestate@gmail.com")
	return SeedRixusRealEstate(ctx, pool)
}
