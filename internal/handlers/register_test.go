package handlers

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestRegisterValidation(t *testing.T) {
	s := &Server{
		JWTSecret:  "0123456789abcdef0123456789abcdef",
		CORSOrigin: "*",
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
				"email":    "user@example.com",
				"password": "password123",
			},
			wantStatus: http.StatusBadRequest,
			wantError:  "name is required",
		},
		{
			name: "empty name with whitespace",
			body: map[string]any{
				"name":     "   ",
				"email":    "user@example.com",
				"password": "password123",
			},
			wantStatus: http.StatusBadRequest,
			wantError:  "name is required",
		},
		{
			name: "invalid email",
			body: map[string]any{
				"name":     "Alice",
				"email":    "not-an-email",
				"password": "password123",
			},
			wantStatus: http.StatusBadRequest,
			wantError:  "invalid email",
		},
		{
			name: "password too short",
			body: map[string]any{
				"name":     "Alice",
				"email":    "alice@example.com",
				"password": "short",
			},
			wantStatus: http.StatusBadRequest,
			wantError:  "password must be 8-72 characters",
		},
		{
			name: "invalid user role",
			body: map[string]any{
				"name":      "Alice",
				"email":     "alice@example.com",
				"password":  "password123",
				"user_role": "SUPERUSER",
			},
			wantStatus: http.StatusBadRequest,
			wantError:  "user_role must be ADMIN or VIEWER",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			b, _ := json.Marshal(tt.body)
			req := httptest.NewRequest(http.MethodPost, "/api/auth/register", bytes.NewReader(b))
			req.Header.Set("Content-Type", "application/json")
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

