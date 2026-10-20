package handlers

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/CodyNicholson/Rixus_Real_Estate_BE_Project/internal/auth"
)

func TestPropertyEndpointsAuthAndPermissions(t *testing.T) {
	secret := "0123456789abcdef0123456789abcdef"
	s := &Server{
		JWTSecret:  secret,
		CORSOrigin: "http://localhost:3000",
	}
	handler := s.Routes()

	// 1. GET /api/properties without token returns 401 Unauthorized
	req := httptest.NewRequest(http.MethodGet, "/api/properties", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 for GET /api/properties without token, got %d", rec.Code)
	}

	// 2. GET /api/users/5/properties without token returns 401 Unauthorized
	req = httptest.NewRequest(http.MethodGet, "/api/users/5/properties", nil)
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 for GET /api/users/5/properties without token, got %d", rec.Code)
	}

	viewerToken5, err := auth.NewAccessToken(secret, 5, "viewer5@example.com", "Viewer 5", "VIEWER")
	if err != nil {
		t.Fatalf("failed to create token: %v", err)
	}
	adminToken1, err := auth.NewAccessToken(secret, 1, "admin1@example.com", "Admin 1", "ADMIN")
	if err != nil {
		t.Fatalf("failed to create token: %v", err)
	}

	// 3. VIEWER (id=5) querying user 10 properties via /api/users/10/properties returns 403 Forbidden
	req = httptest.NewRequest(http.MethodGet, "/api/users/10/properties", nil)
	req.Header.Set("Authorization", "Bearer "+viewerToken5)
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403 when VIEWER requests another user's properties, got %d. Body: %s", rec.Code, rec.Body.String())
	}

	// 4. VIEWER (id=5) querying user 10 properties via ?user_id=10 returns 403 Forbidden
	req = httptest.NewRequest(http.MethodGet, "/api/properties?user_id=10", nil)
	req.Header.Set("Authorization", "Bearer "+viewerToken5)
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403 when VIEWER requests another user's properties via query, got %d. Body: %s", rec.Code, rec.Body.String())
	}

	// 5. Invalid user ID in path returns 400 Bad Request
	req = httptest.NewRequest(http.MethodGet, "/api/users/abc/properties", nil)
	req.Header.Set("Authorization", "Bearer "+viewerToken5)
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for invalid user id in path, got %d. Body: %s", rec.Code, rec.Body.String())
	}

	// 6. Invalid user ID in query returns 400 Bad Request
	req = httptest.NewRequest(http.MethodGet, "/api/properties?user_id=invalid", nil)
	req.Header.Set("Authorization", "Bearer "+viewerToken5)
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for invalid user id in query, got %d. Body: %s", rec.Code, rec.Body.String())
	}

	// 7. VIEWER (id=5) querying their own properties via /api/users/5/properties passes permission check
	// (Database is uninitialized in unit test so expect 500 database not configured / internal error, NOT 403 or 401)
	req = httptest.NewRequest(http.MethodGet, "/api/users/5/properties", nil)
	req.Header.Set("Authorization", "Bearer "+viewerToken5)
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code == http.StatusForbidden || rec.Code == http.StatusUnauthorized {
		t.Fatalf("expected authorized access for user's own properties, got %d", rec.Code)
	}

	// 8. ADMIN (id=1) querying user 10 properties via /api/users/10/properties passes permission check
	req = httptest.NewRequest(http.MethodGet, "/api/users/10/properties", nil)
	req.Header.Set("Authorization", "Bearer "+adminToken1)
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code == http.StatusForbidden || rec.Code == http.StatusUnauthorized {
		t.Fatalf("expected ADMIN to be authorized for any user's properties, got %d", rec.Code)
	}

	// 9. POST /api/properties validation: empty street_address returns 400 Bad Request
	req = httptest.NewRequest(http.MethodPost, "/api/properties", strings.NewReader(`{"units": 2}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+viewerToken5)
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for empty street_address, got %d. Body: %s", rec.Code, rec.Body.String())
	}

	// 10. POST /api/properties validation: units below 1 or above SMALLINT range returns 400 Bad Request
	for _, body := range []string{
		`{"street_address": "123 Main St", "units": 0}`,
		`{"street_address": "123 Main St", "units": -2}`,
		`{"street_address": "123 Main St", "units": 40000}`,
	} {
		req = httptest.NewRequest(http.MethodPost, "/api/properties", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+viewerToken5)
		rec = httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("expected 400 for invalid units in %s, got %d. Body: %s", body, rec.Code, rec.Body.String())
		}
	}

	// 10b. POST /api/properties with units omitted passes validation (defaults to 1)
	// (Database is uninitialized in unit test so expect 500, NOT 400)
	req = httptest.NewRequest(http.MethodPost, "/api/properties", strings.NewReader(`{"street_address": "123 Main St"}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+viewerToken5)
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code == http.StatusBadRequest {
		t.Fatalf("expected omitted units to default to 1, got 400. Body: %s", rec.Body.String())
	}

	// 11. GET /api/properties/abc returns 400 Bad Request for invalid id
	req = httptest.NewRequest(http.MethodGet, "/api/properties/abc", nil)
	req.Header.Set("Authorization", "Bearer "+viewerToken5)
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for invalid property id, got %d. Body: %s", rec.Code, rec.Body.String())
	}
}
