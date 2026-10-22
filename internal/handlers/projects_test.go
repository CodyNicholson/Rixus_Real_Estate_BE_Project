package handlers

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/CodyNicholson/Rixus_Real_Estate_BE_Project/internal/auth"
)

func TestProjectsAndContractorsEndpoints(t *testing.T) {
	jwtSecret := "test_secret_must_be_at_least_32_chars_long"
	s := &Server{
		JWTSecret: jwtSecret,
	}
	handler := s.Routes()

	// 1. GET /api/projects without token -> 401
	req := httptest.NewRequest(http.MethodGet, "/api/projects", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 for GET /api/projects without token, got %d", rec.Code)
	}

	// 2. GET /api/contractors without token -> 401
	req = httptest.NewRequest(http.MethodGet, "/api/contractors", nil)
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 for GET /api/contractors without token, got %d", rec.Code)
	}

	// Create test token
	token, err := auth.NewAccessToken(jwtSecret, 1, "test@example.com", "Test User", "ADMIN")
	if err != nil {
		t.Fatalf("failed to create access token: %v", err)
	}

	// 3. GET /api/projects/abc with token -> 400 Bad Request
	req = httptest.NewRequest(http.MethodGet, "/api/projects/abc", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for GET /api/projects/abc, got %d", rec.Code)
	}

	// 4. POST /api/projects with empty title -> 400 Bad Request
	body := []byte(`{"property_id": 1, "title": ""}`)
	req = httptest.NewRequest(http.MethodPost, "/api/projects", bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for POST /api/projects with empty title, got %d", rec.Code)
	}

	// 5. POST /api/contractors with empty name -> 400 Bad Request
	body = []byte(`{"name": ""}`)
	req = httptest.NewRequest(http.MethodPost, "/api/contractors", bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for POST /api/contractors with empty name, got %d", rec.Code)
	}

	// 6. GET /api/project-images/1/view without token -> 401
	req = httptest.NewRequest(http.MethodGet, "/api/project-images/1/view", nil)
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 for GET /api/project-images/1/view without token, got %d", rec.Code)
	}
}
