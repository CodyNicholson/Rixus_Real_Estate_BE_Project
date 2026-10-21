package db

import (
	"context"
	"os"
	"strings"
	"testing"
)

func TestSeedRixusRealEstateSQLIntegrity(t *testing.T) {
	if len(seedRixusRealEstateSQL) == 0 {
		t.Fatalf("expected embedded seed SQL to not be empty")
	}

	requiredSubstrings := []string{
		"rixusrealestate@gmail.com",
		"Rixus Real Estate",
		"Lincoln Park Luxury 3-Flat",
		"Wicker Park Modern 4-Flat",
		"West Loop High-Rise Condo #502",
		"Fulton Market Mixed-Use",
		"Oak Park Prairie Home",
		"Logan Square 6-Unit",
		"INSERT INTO legal_entities",
		"INSERT INTO properties",
		"INSERT INTO units",
		"INSERT INTO tenants",
		"INSERT INTO property_loans",
		"INSERT INTO underwriting_models",
		"INSERT INTO ledger_transactions",
	}

	for _, sub := range requiredSubstrings {
		if !strings.Contains(seedRixusRealEstateSQL, sub) {
			t.Errorf("seed SQL missing expected string %q", sub)
		}
	}
}

func TestSeedRixusRealEstateDatabase(t *testing.T) {
	url := os.Getenv("TEST_DATABASE_URL")
	if url == "" {
		t.Skip("TEST_DATABASE_URL not set")
	}

	ctx := context.Background()
	pool, err := Connect(ctx, url)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	defer pool.Close()

	if err := SeedRixusRealEstate(ctx, pool); err != nil {
		t.Fatalf("SeedRixusRealEstate failed: %v", err)
	}

	// Verify user exists
	var userID int64
	var role string
	err = pool.QueryRow(ctx, `SELECT id, user_role FROM users WHERE email = 'rixusrealestate@gmail.com'`).Scan(&userID, &role)
	if err != nil {
		t.Fatalf("user rixusrealestate@gmail.com not found: %v", err)
	}
	if role != "ADMIN" {
		t.Errorf("expected role ADMIN, got %s", role)
	}

	// Verify properties count
	var propCount int
	err = pool.QueryRow(ctx, `SELECT COUNT(*) FROM properties WHERE user_id = $1`, userID).Scan(&propCount)
	if err != nil {
		t.Fatalf("properties count query failed: %v", err)
	}
	if propCount != 6 {
		t.Errorf("expected 6 properties, got %d", propCount)
	}

	// Verify tenants count
	var tenantCount int
	err = pool.QueryRow(ctx, `
		SELECT COUNT(*) 
		FROM tenants t 
		JOIN properties p ON p.id = t.property_id 
		WHERE p.user_id = $1
	`, userID).Scan(&tenantCount)
	if err != nil {
		t.Fatalf("tenants count query failed: %v", err)
	}
	if tenantCount < 15 {
		t.Errorf("expected at least 15 tenants, got %d", tenantCount)
	}

	// Verify units count
	var unitCount int
	err = pool.QueryRow(ctx, `
		SELECT COUNT(*) 
		FROM units u 
		JOIN properties p ON p.id = u.property_id 
		WHERE p.user_id = $1
	`, userID).Scan(&unitCount)
	if err != nil {
		t.Fatalf("units count query failed: %v", err)
	}
	if unitCount != 18 {
		t.Errorf("expected 18 units, got %d", unitCount)
	}

	// Verify ledger actuals count
	var ledgerCount int
	err = pool.QueryRow(ctx, `
		SELECT COUNT(*) 
		FROM ledger_transactions lt 
		JOIN properties p ON p.id = lt.property_id 
		WHERE p.user_id = $1
	`, userID).Scan(&ledgerCount)
	if err != nil {
		t.Fatalf("ledger count query failed: %v", err)
	}
	if ledgerCount == 0 {
		t.Errorf("expected ledger transactions to be seeded, got 0")
	}
}

