package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestSwaggerRoutes(t *testing.T) {
	s := &Server{
		JWTSecret:  "0123456789abcdef0123456789abcdef",
		CORSOrigin: "*",
	}
	handler := s.Routes()

	// Test GET / root redirect
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusMovedPermanently {
		t.Fatalf("expected 301 for /, got %d", rec.Code)
	}
	if loc := rec.Header().Get("Location"); loc != "/swagger/" {
		t.Fatalf("expected Location /swagger/, got %q", loc)
	}

	// Test GET /swagger redirect
	req = httptest.NewRequest(http.MethodGet, "/swagger", nil)
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusMovedPermanently {
		t.Fatalf("expected 301 for /swagger, got %d", rec.Code)
	}
	if loc := rec.Header().Get("Location"); loc != "/swagger/" {
		t.Fatalf("expected Location /swagger/, got %q", loc)
	}

	// Test GET /swagger/ UI page
	req = httptest.NewRequest(http.MethodGet, "/swagger/", nil)
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 for /swagger/, got %d", rec.Code)
	}
	if ct := rec.Header().Get("Content-Type"); !strings.Contains(ct, "text/html") {
		t.Fatalf("expected text/html for /swagger/, got %q", ct)
	}
	if body := rec.Body.String(); !strings.Contains(body, "SwaggerUIBundle") {
		t.Fatalf("expected SwaggerUIBundle in HTML body")
	}

	// Test GET /swagger/openapi.json
	req = httptest.NewRequest(http.MethodGet, "/swagger/openapi.json", nil)
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 for /swagger/openapi.json, got %d", rec.Code)
	}
	if ct := rec.Header().Get("Content-Type"); !strings.Contains(ct, "application/json") {
		t.Fatalf("expected application/json, got %q", ct)
	}
	var spec map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &spec); err != nil {
		t.Fatalf("invalid json spec: %v", err)
	}
	if spec["openapi"] != "3.0.3" {
		t.Fatalf("expected openapi version 3.0.3, got %v", spec["openapi"])
	}
}
