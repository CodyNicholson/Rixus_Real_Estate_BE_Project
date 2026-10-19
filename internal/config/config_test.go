package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadDotEnv(t *testing.T) {
	tmpDir := t.TempDir()
	envPath := filepath.Join(tmpDir, ".env")

	content := `
# A comment
TEST_PORT=9090
TEST_QUOTED="hello world"
TEST_SINGLE='single quoted'
`
	if err := os.WriteFile(envPath, []byte(content), 0600); err != nil {
		t.Fatal(err)
	}

	// Pre-set a variable to ensure existing env vars take precedence
	t.Setenv("TEST_PORT", "8888")

	loadDotEnv(envPath)

	if got := os.Getenv("TEST_PORT"); got != "8888" {
		t.Errorf("expected TEST_PORT=8888 (pre-existing), got %q", got)
	}
	if got := os.Getenv("TEST_QUOTED"); got != "hello world" {
		t.Errorf("expected TEST_QUOTED='hello world', got %q", got)
	}
	if got := os.Getenv("TEST_SINGLE"); got != "single quoted" {
		t.Errorf("expected TEST_SINGLE='single quoted', got %q", got)
	}
}

func TestLoadValidation(t *testing.T) {
	t.Setenv("DATABASE_URL", "")
	t.Setenv("DB_HOST", "")
	t.Setenv("JWT_SECRET", "")

	// Missing DATABASE_URL
	if _, err := Load(); err == nil {
		t.Error("expected error for missing DATABASE_URL")
	}

	// Short JWT_SECRET
	t.Setenv("DATABASE_URL", "postgres://localhost:5432/rixus")
	t.Setenv("JWT_SECRET", "too-short")
	if _, err := Load(); err == nil {
		t.Error("expected error for short JWT_SECRET")
	}

	// Valid config
	t.Setenv("JWT_SECRET", "0123456789abcdef0123456789abcdef")
	cfg, err := Load()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Port != "8080" {
		t.Errorf("expected default Port 8080, got %q", cfg.Port)
	}
}

func TestLoadCloudflareConfig(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://localhost:5432/rixus")
	t.Setenv("JWT_SECRET", "0123456789abcdef0123456789abcdef")
	t.Setenv("CF_TEAM_DOMAIN", "https://myteam.cloudflareaccess.com")
	t.Setenv("CF_POLICY_AUD", "test-aud-12345")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.CFTeamDomain != "https://myteam.cloudflareaccess.com" {
		t.Errorf("expected CFTeamDomain %q, got %q", "https://myteam.cloudflareaccess.com", cfg.CFTeamDomain)
	}
	if cfg.CFPolicyAUD != "test-aud-12345" {
		t.Errorf("expected CFPolicyAUD %q, got %q", "test-aud-12345", cfg.CFPolicyAUD)
	}
}

func TestLoadWithDBHost(t *testing.T) {
	t.Setenv("DATABASE_URL", "")
	t.Setenv("DB_HOST", "db")
	t.Setenv("DB_PORT", "5432")
	t.Setenv("DB_USER", "postgres")
	t.Setenv("DB_PASSWORD", "secret")
	t.Setenv("DB_NAME", "rixus")
	t.Setenv("JWT_SECRET", "0123456789abcdef0123456789abcdef")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	expectedURL := "postgres://postgres:secret@db:5432/rixus?sslmode=disable"
	if cfg.DatabaseURL != expectedURL {
		t.Errorf("expected %q, got %q", expectedURL, cfg.DatabaseURL)
	}
}
