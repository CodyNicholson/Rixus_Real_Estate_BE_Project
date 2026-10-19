package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/CodyNicholson/Rixus_Real_Estate_BE_Project/internal/auth"
	"github.com/golang-jwt/jwt/v5"
)

func TestRefreshFlow(t *testing.T) {
	secret := "0123456789abcdef0123456789abcdef"
	s := &Server{
		JWTSecret:  secret,
		CORSOrigin: "http://localhost:3000",
	}
	handler := s.Routes()

	// 1. Missing refresh token cookie returns 401
	req := httptest.NewRequest(http.MethodPost, "/api/refresh", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 without cookie, got %d", rec.Code)
	}

	// 2. Invalid refresh token cookie returns 401
	req = httptest.NewRequest(http.MethodPost, "/api/refresh", nil)
	req.AddCookie(&http.Cookie{Name: "refresh_token", Value: "invalid.token.here"})
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 with invalid cookie, got %d", rec.Code)
	}

	// 3. Valid refresh token cookie returns 200, access_token in body, and updated cookie
	validRefreshToken, err := auth.NewRefreshToken(secret, 42)
	if err != nil {
		t.Fatalf("failed to create refresh token: %v", err)
	}

	req = httptest.NewRequest(http.MethodPost, "/api/refresh", nil)
	req.AddCookie(&http.Cookie{Name: "refresh_token", Value: validRefreshToken})
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 with valid cookie, got %d. Body: %s", rec.Code, rec.Body.String())
	}

	var res AuthResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &res); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if res.AccessToken == "" {
		t.Fatal("expected access_token in response body")
	}

	// Verify the new access token can be parsed
	userID, err := auth.ParseAccessToken(secret, res.AccessToken)
	if err != nil || userID != 42 {
		t.Fatalf("new access token validation failed: %v, userID=%d", err, userID)
	}

	// Verify the response sets the refresh_token cookie with secure attributes
	cookies := rec.Result().Cookies()
	var refreshCookie *http.Cookie
	for _, c := range cookies {
		if c.Name == "refresh_token" {
			refreshCookie = c
			break
		}
	}
	if refreshCookie == nil {
		t.Fatal("expected Set-Cookie with refresh_token")
	}
	if !refreshCookie.HttpOnly {
		t.Errorf("expected HttpOnly: true on refresh_token cookie")
	}
	if refreshCookie.Path != "/api/refresh" {
		t.Errorf("expected Path: /api/refresh, got %q", refreshCookie.Path)
	}
	if refreshCookie.SameSite != http.SameSiteLaxMode {
		t.Errorf("expected SameSite: Lax, got %v", refreshCookie.SameSite)
	}
}

func TestLogout(t *testing.T) {
	s := &Server{
		JWTSecret:  "0123456789abcdef0123456789abcdef",
		CORSOrigin: "http://localhost:3000",
	}
	handler := s.Routes()

	req := httptest.NewRequest(http.MethodPost, "/api/logout", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 for logout, got %d", rec.Code)
	}

	cookies := rec.Result().Cookies()
	var refreshCookie *http.Cookie
	for _, c := range cookies {
		if c.Name == "refresh_token" {
			refreshCookie = c
			break
		}
	}
	if refreshCookie == nil {
		t.Fatal("expected clearing refresh_token cookie")
	}
	if refreshCookie.MaxAge >= 0 && refreshCookie.Expires.After(time.Now()) {
		t.Errorf("expected cookie to be expired, got MaxAge=%d, Expires=%v", refreshCookie.MaxAge, refreshCookie.Expires)
	}
}

func TestCORSWithCredentials(t *testing.T) {
	s := &Server{
		JWTSecret:  "0123456789abcdef0123456789abcdef",
		CORSOrigin: "http://localhost:3000",
	}
	handler := s.Routes()

	// Preflight OPTIONS request
	req := httptest.NewRequest(http.MethodOptions, "/api/refresh", nil)
	req.Header.Set("Origin", "http://localhost:3000")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("expected 204 for OPTIONS, got %d", rec.Code)
	}
	if allowCreds := rec.Header().Get("Access-Control-Allow-Credentials"); allowCreds != "true" {
		t.Errorf("expected Access-Control-Allow-Credentials: true, got %q", allowCreds)
	}
	if allowOrigin := rec.Header().Get("Access-Control-Allow-Origin"); allowOrigin != "http://localhost:3000" {
		t.Errorf("expected Access-Control-Allow-Origin: http://localhost:3000, got %q", allowOrigin)
	}
}

func TestMeEndpointUnauthorized(t *testing.T) {
	s := &Server{
		JWTSecret:  "0123456789abcdef0123456789abcdef",
		CORSOrigin: "http://localhost:3000",
	}
	handler := s.Routes()

	for _, path := range []string{"/api/users/me", "/api/me", "/api/auth/me"} {
		req := httptest.NewRequest(http.MethodGet, path, nil)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusUnauthorized {
			t.Fatalf("path %s: expected 401 for unauthorized, got %d", path, rec.Code)
		}
	}
}

func TestAccessTokenClaims(t *testing.T) {
	secret := "0123456789abcdef0123456789abcdef"
	tok, err := auth.NewAccessToken(secret, 99, "jane@example.com", "Jane Doe", "ADMIN")
	if err != nil {
		t.Fatalf("failed to create access token: %v", err)
	}

	var claims auth.TokenClaims
	_, err = jwt.ParseWithClaims(tok, &claims, func(token *jwt.Token) (interface{}, error) {
		return []byte(secret), nil
	})
	if err != nil {
		t.Fatalf("failed to parse claims: %v", err)
	}
	if claims.Email != "jane@example.com" {
		t.Errorf("expected Email jane@example.com, got %q", claims.Email)
	}
	if claims.Name != "Jane Doe" {
		t.Errorf("expected Name Jane Doe, got %q", claims.Name)
	}
	if claims.UserRole != "ADMIN" {
		t.Errorf("expected UserRole ADMIN, got %q", claims.UserRole)
	}
}

func TestCORSRejectsUnlistedOrigin(t *testing.T) {
	for _, cfg := range []string{"http://localhost:3000", "*", ""} {
		s := &Server{
			JWTSecret:  "0123456789abcdef0123456789abcdef",
			CORSOrigin: cfg,
		}
		handler := s.Routes()

		req := httptest.NewRequest(http.MethodOptions, "/api/refresh", nil)
		req.Header.Set("Origin", "https://evil.example")
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusForbidden {
			t.Errorf("CORSOrigin=%q: expected 403 for disallowed preflight, got %d", cfg, rec.Code)
		}
		if got := rec.Header().Get("Access-Control-Allow-Origin"); got != "" {
			t.Errorf("CORSOrigin=%q: expected no Access-Control-Allow-Origin, got %q", cfg, got)
		}
		if got := rec.Header().Get("Access-Control-Allow-Credentials"); got != "" {
			t.Errorf("CORSOrigin=%q: expected no Access-Control-Allow-Credentials, got %q", cfg, got)
		}
	}
}

func TestCORSAllowsListedOrigins(t *testing.T) {
	s := &Server{
		JWTSecret:  "0123456789abcdef0123456789abcdef",
		CORSOrigin: "https://app.example.com, http://localhost:3000",
	}
	handler := s.Routes()

	req := httptest.NewRequest(http.MethodOptions, "/api/refresh", nil)
	req.Header.Set("Origin", "https://app.example.com")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if got := rec.Header().Get("Access-Control-Allow-Origin"); got != "https://app.example.com" {
		t.Errorf("expected Access-Control-Allow-Origin: https://app.example.com, got %q", got)
	}
}
