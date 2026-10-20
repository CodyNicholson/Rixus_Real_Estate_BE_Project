package auth

import "testing"

func TestTokenRoundTrip(t *testing.T) {
	secret := "0123456789abcdef0123456789abcdef"
	tok, err := NewToken(secret, 42)
	if err != nil {
		t.Fatal(err)
	}
	id, err := ParseToken(secret, tok)
	if err != nil || id != 42 {
		t.Fatalf("got %d, %v", id, err)
	}
	if _, err := ParseToken("another-secret-another-secret-0000", tok); err == nil {
		t.Fatal("expected error for wrong secret")
	}
}

func TestAccessAndRefreshToken(t *testing.T) {
	secret := "0123456789abcdef0123456789abcdef"

	accessTok, err := NewAccessToken(secret, 100)
	if err != nil {
		t.Fatalf("NewAccessToken failed: %v", err)
	}
	id, err := ParseAccessToken(secret, accessTok)
	if err != nil || id != 100 {
		t.Fatalf("ParseAccessToken failed: %v, id=%d", err, id)
	}

	refreshTok, err := NewRefreshToken(secret, 200)
	if err != nil {
		t.Fatalf("NewRefreshToken failed: %v", err)
	}
	id, err = ParseRefreshToken(secret, refreshTok)
	if err != nil || id != 200 {
		t.Fatalf("ParseRefreshToken failed: %v, id=%d", err, id)
	}

	// Cross-type validation: access token cannot be used as refresh token
	if _, err := ParseRefreshToken(secret, accessTok); err == nil {
		t.Fatal("expected error using access token as refresh token")
	}

	// Cross-type validation: refresh token cannot be used as access token
	if _, err := ParseAccessToken(secret, refreshTok); err == nil {
		t.Fatal("expected error using refresh token as access token")
	}
}

func TestParseAccessTokenClaims(t *testing.T) {
	secret := "0123456789abcdef0123456789abcdef"
	tok, err := NewAccessToken(secret, 55, "admin@example.com", "Admin User", "ADMIN")
	if err != nil {
		t.Fatalf("failed to create access token: %v", err)
	}

	claims, err := ParseAccessTokenClaims(secret, tok)
	if err != nil {
		t.Fatalf("failed to parse claims: %v", err)
	}
	if claims.Email != "admin@example.com" {
		t.Errorf("expected email admin@example.com, got %q", claims.Email)
	}
	if claims.UserRole != "ADMIN" {
		t.Errorf("expected role ADMIN, got %q", claims.UserRole)
	}
	if claims.Subject != "55" {
		t.Errorf("expected subject 55, got %q", claims.Subject)
	}
}
