package handlers

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/CodyNicholson/Rixus_Real_Estate_BE_Project/internal/auth"
)

func TestRegisterRequiresCloudflareAccess(t *testing.T) {
	s := &Server{
		JWTSecret:  "0123456789abcdef0123456789abcdef",
		CORSOrigin: "*",
	}
	handler := s.Routes()

	// Calling registration without Cloudflare Access credentials returns 401 Unauthorized
	req := httptest.NewRequest(http.MethodPost, "/api/auth/register", strings.NewReader(`{"name":"Alice","email":"alice@example.com"}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 without Cloudflare Access credentials, got %d. Body: %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "Cloudflare Access Google OAuth authentication required") {
		t.Fatalf("expected error message to mention Cloudflare Access Google OAuth, got %s", rec.Body.String())
	}
}

func TestRegisterValidation(t *testing.T) {
	s := &Server{
		JWTSecret:    "0123456789abcdef0123456789abcdef",
		CORSOrigin:   "*",
		AllowDevAuth: true,
	}
	handler := s.Routes()

	tests := []struct {
		name       string
		body       map[string]any
		wantStatus int
		wantError  string
	}{
		{
			name: "missing name",
			body: map[string]any{
				"email": "user@example.com",
			},
			wantStatus: http.StatusBadRequest,
			wantError:  "name is required",
		},
		{
			name: "empty name with whitespace",
			body: map[string]any{
				"name":  "   ",
				"email": "user@example.com",
			},
			wantStatus: http.StatusBadRequest,
			wantError:  "name is required",
		},
		{
			name: "invalid email",
			body: map[string]any{
				"name":  "Alice",
				"email": "not-an-email",
			},
			wantStatus: http.StatusBadRequest,
			wantError:  "invalid email",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			b, _ := json.Marshal(tt.body)
			req := httptest.NewRequest(http.MethodPost, "/api/auth/register", bytes.NewReader(b))
			req.Header.Set("Content-Type", "application/json")
			if emailStr, ok := tt.body["email"].(string); ok {
				req.Header.Set("X-Dev-Email", emailStr)
			}
			rec := httptest.NewRecorder()

			handler.ServeHTTP(rec, req)

			if rec.Code != tt.wantStatus {
				t.Fatalf("expected status %d, got %d. Body: %s", tt.wantStatus, rec.Code, rec.Body.String())
			}
			if !strings.Contains(rec.Body.String(), tt.wantError) {
				t.Fatalf("expected error containing %q, got %s", tt.wantError, rec.Body.String())
			}
		})
	}
}

func TestAdminPromoteEndpoint(t *testing.T) {
	secret := "0123456789abcdef0123456789abcdef"
	s := &Server{
		JWTSecret:  secret,
		CORSOrigin: "*",
	}
	handler := s.Routes()

	// 1. Calling /api/admin/promote without token returns 401 Unauthorized
	req := httptest.NewRequest(http.MethodPost, "/api/admin/promote", strings.NewReader(`{"user_id": 1}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 without auth token, got %d", rec.Code)
	}

	// 2. Calling with VIEWER token returns 403 Forbidden
	viewerToken, err := auth.NewAccessToken(secret, 10, "viewer@example.com", "Viewer User", "VIEWER")
	if err != nil {
		t.Fatalf("failed to create viewer token: %v", err)
	}
	req = httptest.NewRequest(http.MethodPost, "/api/admin/promote", strings.NewReader(`{"user_id": 2}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+viewerToken)
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403 when VIEWER calls promote endpoint, got %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "admin privileges required") {
		t.Fatalf("expected error 'admin privileges required', got %s", rec.Body.String())
	}

	// 3. Calling with ADMIN token but no target user_id or email returns 400 Bad Request
	adminToken, err := auth.NewAccessToken(secret, 1, "admin@example.com", "Admin User", "ADMIN")
	if err != nil {
		t.Fatalf("failed to create admin token: %v", err)
	}
	req = httptest.NewRequest(http.MethodPost, "/api/admin/promote", strings.NewReader(`{}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+adminToken)
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 when missing target user, got %d. Body: %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "user_id or email is required") {
		t.Fatalf("expected 'user_id or email is required', got %s", rec.Body.String())
	}

	// 4. Calling with ADMIN token and invalid role returns 400 Bad Request
	req = httptest.NewRequest(http.MethodPost, "/api/admin/promote", strings.NewReader(`{"user_id": 2, "role": "INVALID"}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+adminToken)
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for invalid role, got %d. Body: %s", rec.Code, rec.Body.String())
	}

	// 5. Calling path /api/admin/users/abc/promote with invalid path ID returns 400 Bad Request
	req = httptest.NewRequest(http.MethodPost, "/api/admin/users/abc/promote", nil)
	req.Header.Set("Authorization", "Bearer "+adminToken)
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for invalid id in path, got %d. Body: %s", rec.Code, rec.Body.String())
	}

	// 6. Calling with ADMIN token and valid target user passes authorization and reaches DB call
	req = httptest.NewRequest(http.MethodPost, "/api/admin/promote", strings.NewReader(`{"user_id": 2}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+adminToken)
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusInternalServerError || !strings.Contains(rec.Body.String(), "database not configured") {
		t.Fatalf("expected 500 'database not configured', got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestAdminGetUsersEndpoint(t *testing.T) {
	secret := "0123456789abcdef0123456789abcdef"
	s := &Server{
		JWTSecret:  secret,
		CORSOrigin: "*",
	}
	handler := s.Routes()

	// 1. Calling GET /api/admin/users without auth token returns 401 Unauthorized
	req := httptest.NewRequest(http.MethodGet, "/api/admin/users", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 without auth token, got %d", rec.Code)
	}

	// 2. Calling with VIEWER token returns 403 Forbidden
	viewerToken, err := auth.NewAccessToken(secret, 10, "viewer@example.com", "Viewer User", "VIEWER")
	if err != nil {
		t.Fatalf("failed to create viewer token: %v", err)
	}
	req = httptest.NewRequest(http.MethodGet, "/api/admin/users", nil)
	req.Header.Set("Authorization", "Bearer "+viewerToken)
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403 when VIEWER calls get users endpoint, got %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "admin privileges required") {
		t.Fatalf("expected error 'admin privileges required', got %s", rec.Body.String())
	}

	// 3. Calling with ADMIN token reaches DB check (returns 500 when DB is nil)
	adminToken, err := auth.NewAccessToken(secret, 1, "admin@example.com", "Admin User", "ADMIN")
	if err != nil {
		t.Fatalf("failed to create admin token: %v", err)
	}
	req = httptest.NewRequest(http.MethodGet, "/api/admin/users", nil)
	req.Header.Set("Authorization", "Bearer "+adminToken)
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusInternalServerError || !strings.Contains(rec.Body.String(), "database not configured") {
		t.Fatalf("expected 500 'database not configured', got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestRegisterIgnoresUserRole(t *testing.T) {
	s := &Server{
		JWTSecret:    "0123456789abcdef0123456789abcdef",
		CORSOrigin:   "*",
		AllowDevAuth: true,
		DevCFEmail:   "attacker@example.com",
	}
	handler := s.Routes()

	// Calling registration with "user_role": "ADMIN" should NOT fail validation and should NOT set ADMIN
	body := map[string]any{
		"name":      "Attacker",
		"email":     "attacker@example.com",
		"user_role": "ADMIN",
	}
	b, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPost, "/api/auth/register", bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	// Since s.DB is nil, valid registration passes validation and reaches the database check (500),
	// rather than failing on role validation.
	if rec.Code != http.StatusInternalServerError || !strings.Contains(rec.Body.String(), "database not configured") {
		t.Fatalf("expected 500 'database not configured', got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestLoginRequiresCloudflareAccess(t *testing.T) {
	s := &Server{
		JWTSecret:  "0123456789abcdef0123456789abcdef",
		CORSOrigin: "*",
	}
	handler := s.Routes()

	// 1. Password login is removed: POST /api/login without Cloudflare Access credentials returns 401
	req := httptest.NewRequest(http.MethodPost, "/api/login", strings.NewReader(`{"email":"user@example.com","password":"password123"}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 without Cloudflare Access, got %d. Body: %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "Cloudflare Access Google OAuth authentication required") {
		t.Fatalf("expected CF OAuth error message, got %s", rec.Body.String())
	}

	// 2. Spoofed email header without assertion is rejected
	req = httptest.NewRequest(http.MethodPost, "/api/login", nil)
	req.Header.Set("Cf-Access-Authenticated-User-Email", "admin@example.com")
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 for spoofed header, got %d. Body: %s", rec.Code, rec.Body.String())
	}
}

func TestUserResponseCreatedAtSerialization(t *testing.T) {
	fixedTime := time.Date(2026, 10, 19, 20, 0, 0, 0, time.UTC)
	uWithTime := UserResponse{
		ID:        1,
		Name:      "Admin User",
		Email:     "admin@example.com",
		UserRole:  "ADMIN",
		CreatedAt: &fixedTime,
	}

	data, err := json.Marshal(uWithTime)
	if err != nil {
		t.Fatalf("failed to marshal UserResponse with CreatedAt: %v", err)
	}

	var m map[string]interface{}
	if err := json.Unmarshal(data, &m); err != nil {
		t.Fatalf("failed to unmarshal JSON: %v", err)
	}

	if val, ok := m["created_at"]; !ok || val != "2026-10-19T20:00:00Z" {
		t.Fatalf("expected created_at to be '2026-10-19T20:00:00Z', got %v", val)
	}

	uWithoutTime := UserResponse{
		ID:       2,
		Name:     "Viewer User",
		Email:    "viewer@example.com",
		UserRole: "VIEWER",
	}

	dataWithout, err := json.Marshal(uWithoutTime)
	if err != nil {
		t.Fatalf("failed to marshal UserResponse without CreatedAt: %v", err)
	}

	var mWithout map[string]interface{}
	if err := json.Unmarshal(dataWithout, &mWithout); err != nil {
		t.Fatalf("failed to unmarshal JSON: %v", err)
	}

	if _, ok := mWithout["created_at"]; ok {
		t.Fatalf("expected created_at to be omitted when nil, but found %v", mWithout["created_at"])
	}
}


