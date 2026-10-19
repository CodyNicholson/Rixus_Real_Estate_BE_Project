package handlers

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestCFSessionNoHeader(t *testing.T) {
	s := &Server{
		JWTSecret:  "0123456789abcdef0123456789abcdef",
		CORSOrigin: "*",
	}
	handler := s.Routes()

	req := httptest.NewRequest(http.MethodGet, "/api/auth/cf-session", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rec.Code)
	}

	var res CFSessionResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &res); err != nil {
		t.Fatalf("failed to parse JSON: %v", err)
	}

	if res.Authenticated {
		t.Errorf("expected authenticated=false when no CF header, got true")
	}
	if res.Registered {
		t.Errorf("expected registered=false, got true")
	}
}

func TestCFSessionWithHeaderUnregistered(t *testing.T) {
	s := &Server{
		JWTSecret:  "0123456789abcdef0123456789abcdef",
		CORSOrigin: "*",
	}
	handler := s.Routes()

	req := httptest.NewRequest(http.MethodGet, "/api/auth/cf-session", nil)
	req.Header.Set("Cf-Access-Authenticated-User-Email", "googleuser@gmail.com")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rec.Code)
	}

	var res CFSessionResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &res); err != nil {
		t.Fatalf("failed to parse JSON: %v", err)
	}

	if !res.Authenticated {
		t.Errorf("expected authenticated=true with CF header, got false")
	}
	if res.Registered {
		t.Errorf("expected registered=false for unseeded DB, got true")
	}
	if res.Email != "googleuser@gmail.com" {
		t.Errorf("expected email googleuser@gmail.com, got %q", res.Email)
	}
}

func TestCFRegisterPasswordlessValidation(t *testing.T) {
	s := &Server{
		JWTSecret:  "0123456789abcdef0123456789abcdef",
		CORSOrigin: "*",
	}
	handler := s.Routes()

	// 1. Non-CF registration without password fails
	bodyNoPass, _ := json.Marshal(map[string]any{
		"name":      "Test User",
		"email":     "test@example.com",
		"user_role": "VIEWER",
	})
	req := httptest.NewRequest(http.MethodPost, "/api/auth/register", bytes.NewReader(bodyNoPass))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for non-CF registration without password, got %d. Body: %s", rec.Code, rec.Body.String())
	}

	// 2. CF registration where email mismatches authenticated Google email fails with 403
	reqMismatch := httptest.NewRequest(http.MethodPost, "/api/auth/register", bytes.NewReader(bodyNoPass))
	reqMismatch.Header.Set("Content-Type", "application/json")
	reqMismatch.Header.Set("Cf-Access-Authenticated-User-Email", "different@gmail.com")
	recMismatch := httptest.NewRecorder()
	handler.ServeHTTP(recMismatch, reqMismatch)

	if recMismatch.Code != http.StatusForbidden {
		t.Fatalf("expected 403 for email mismatch, got %d. Body: %s", recMismatch.Code, recMismatch.Body.String())
	}
}

