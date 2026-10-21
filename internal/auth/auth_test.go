package auth

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

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

func TestMiddlewareWithBearerAndQueryToken(t *testing.T) {
	secret := "0123456789abcdef0123456789abcdef"
	tok, err := NewAccessToken(secret, 99, "user99@example.com", "User 99", "USER")
	if err != nil {
		t.Fatalf("failed to create access token: %v", err)
	}

	mw := Middleware(secret)
	target := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := UserID(r.Context())
		role := UserRole(r.Context())
		if id != 99 || role != "USER" {
			t.Errorf("expected id=99 role=USER, got id=%d role=%s", id, role)
		}
		w.WriteHeader(http.StatusOK)
	})
	handler := mw(target)

	// 1. Missing token -> 401
	req, _ := http.NewRequest(http.MethodGet, "/test", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("expected 401 for missing token, got %d", rec.Code)
	}

	// 2. Bearer header -> 200
	req, _ = http.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("Authorization", "Bearer "+tok)
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Errorf("expected 200 for Bearer header, got %d", rec.Code)
	}

	// 3. Query param ?token= -> 200
	req, _ = http.NewRequest(http.MethodGet, "/test?token="+tok, nil)
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Errorf("expected 200 for ?token= query param, got %d", rec.Code)
	}

	// 4. Query param ?access_token= -> 200
	req, _ = http.NewRequest(http.MethodGet, "/test?access_token="+tok, nil)
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Errorf("expected 200 for ?access_token= query param, got %d", rec.Code)
	}
}
