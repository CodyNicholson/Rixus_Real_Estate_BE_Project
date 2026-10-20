package handlers

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/CodyNicholson/Rixus_Real_Estate_BE_Project/internal/auth"
)

func TestTenantEndpointsAuthAndInvalidIDs(t *testing.T) {
	secret := "0123456789abcdef0123456789abcdef"
	s := &Server{
		JWTSecret:  secret,
		CORSOrigin: "http://localhost:3000",
	}
	handler := s.Routes()

	viewerToken5, err := auth.NewAccessToken(secret, 5, "viewer5@example.com", "Viewer 5", "VIEWER")
	if err != nil {
		t.Fatalf("failed to create token: %v", err)
	}

	routes := []struct{ method, path string }{
		{http.MethodGet, "/api/properties/{p}/tenants"},
		{http.MethodGet, "/api/properties/{p}/tenants/all"},
		{http.MethodPost, "/api/properties/{p}/tenants"},
		{http.MethodGet, "/api/properties/{p}/tenants/{t}"},
		{http.MethodPut, "/api/properties/{p}/tenants/{t}"},
		{http.MethodDelete, "/api/properties/{p}/tenants/{t}"},
	}
	path := func(tmpl, propID, tenantID string) string {
		return strings.NewReplacer("{p}", propID, "{t}", tenantID).Replace(tmpl)
	}

	// 1. Every tenant endpoint requires a token
	for _, rt := range routes {
		req := httptest.NewRequest(rt.method, path(rt.path, "1", "1"), strings.NewReader(`{}`))
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		if rec.Code != http.StatusUnauthorized {
			t.Fatalf("expected 401 for %s %s without token, got %d", rt.method, rt.path, rec.Code)
		}
	}

	// 2. Non-numeric or non-positive property or tenant ids return 400 Bad Request
	for _, rt := range routes {
		for _, bad := range []string{"abc", "0", "-3"} {
			paths := []string{path(rt.path, bad, "1")}
			if strings.Contains(rt.path, "{t}") {
				paths = append(paths, path(rt.path, "1", bad))
			}
			for _, p := range paths {
				req := httptest.NewRequest(rt.method, p, strings.NewReader(`{"tenant_initials": "AB", "monthly_rent": 1000}`))
				req.Header.Set("Content-Type", "application/json")
				req.Header.Set("Authorization", "Bearer "+viewerToken5)
				rec := httptest.NewRecorder()
				handler.ServeHTTP(rec, req)
				if rec.Code != http.StatusBadRequest {
					t.Fatalf("expected 400 for %s %s, got %d. Body: %s", rt.method, p, rec.Code, rec.Body.String())
				}
			}
		}
	}

	// 3. Malformed JSON body on create returns 400 Bad Request
	req := httptest.NewRequest(http.MethodPost, "/api/properties/1/tenants", strings.NewReader(`{not json`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+viewerToken5)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for malformed body, got %d. Body: %s", rec.Code, rec.Body.String())
	}
}

func TestTenantInputValidate(t *testing.T) {
	intp := func(v int) *int { return &v }
	floatp := func(v float64) *float64 { return &v }
	jan := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	feb := time.Date(2026, 2, 1, 0, 0, 0, 0, time.UTC)

	invalid := map[string]tenantInput{
		"missing initials":     {MonthlyRent: floatp(1000)},
		"blank initials":       {TenantInitials: "   ", MonthlyRent: floatp(1000)},
		"initials too long":    {TenantInitials: "ABCDE", MonthlyRent: floatp(1000)},
		"missing rent":         {TenantInitials: "AB"},
		"negative rent":        {TenantInitials: "AB", MonthlyRent: floatp(-1)},
		"rent too large":       {TenantInitials: "AB", MonthlyRent: floatp(1e10)},
		"unit zero":            {TenantInitials: "AB", MonthlyRent: floatp(1000), UnitNumber: intp(0)},
		"unit above units":     {TenantInitials: "AB", MonthlyRent: floatp(1000), UnitNumber: intp(5)},
		"negative pets":        {TenantInitials: "AB", MonthlyRent: floatp(1000), PetCount: intp(-1)},
		"too many pets":        {TenantInitials: "AB", MonthlyRent: floatp(1000), PetCount: intp(40000)},
		"move out before move": {TenantInitials: "AB", MonthlyRent: floatp(1000), MoveInDate: &feb, MoveOutDate: &jan},
	}
	for name, in := range invalid {
		if msg := in.validate(4); msg == "" {
			t.Errorf("%s: expected validation error", name)
		}
	}

	in := tenantInput{TenantInitials: " ÉLÉA ", MonthlyRent: floatp(1850.5), MoveInDate: &jan, MoveOutDate: &feb}
	if msg := in.validate(4); msg != "" {
		t.Fatalf("expected valid input, got %q", msg)
	}
	if in.TenantInitials != "ÉLÉA" || *in.UnitNumber != 1 || *in.PetCount != 0 {
		t.Fatalf("expected trimmed initials and defaults unit_number=1 pet_count=0, got %q %d %d",
			in.TenantInitials, *in.UnitNumber, *in.PetCount)
	}
}
