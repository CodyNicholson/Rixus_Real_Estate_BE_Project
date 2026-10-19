package handlers

import (
	"bytes"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/CodyNicholson/Rixus_Real_Estate_BE_Project/internal/auth"
	"github.com/golang-jwt/jwt/v5"
)

type mockTransport struct {
	body string
}

func (m *mockTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	return &http.Response{
		StatusCode: http.StatusOK,
		Body:       io.NopCloser(strings.NewReader(m.body)),
		Header:     make(http.Header),
	}, nil
}

func TestCFSessionSecurity(t *testing.T) {
	// 1. Setup RSA Key & Mock Cloudflare Certs
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("failed to generate RSA key: %v", err)
	}
	pubKeyBytes, _ := x509.MarshalPKIXPublicKey(&privateKey.PublicKey)
	pubPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "PUBLIC KEY",
		Bytes: pubKeyBytes,
	})

	testKID := "test-cf-kid"
	teamDomain := "https://myteam.cloudflareaccess.com"
	policyAUD := "cf-policy-aud-12345"

	resMap := map[string]any{
		"public_certs": []map[string]string{
			{"kid": testKID, "cert": string(pubPEM)},
		},
	}
	resBytes, _ := json.Marshal(resMap)

	validator := auth.NewCFValidator(teamDomain, policyAUD, "https://myteam.cloudflareaccess.com/cdn-cgi/access/certs")
	validator.HTTPClient = &http.Client{
		Transport: &mockTransport{body: string(resBytes)},
	}

	s := &Server{
		JWTSecret:   "0123456789abcdef0123456789abcdef",
		CORSOrigin:  "*",
		CFValidator: validator,
	}
	handler := s.Routes()

	// Helper to generate a valid signed JWT assertion
	createAssertion := func(email string) string {
		claims := auth.CFAccessClaims{
			Email: email,
			RegisteredClaims: jwt.RegisteredClaims{
				Audience:  jwt.ClaimStrings{policyAUD},
				Issuer:    teamDomain,
				ExpiresAt: jwt.NewNumericDate(time.Now().Add(1 * time.Hour)),
				IssuedAt:  jwt.NewNumericDate(time.Now().Add(-1 * time.Minute)),
			},
		}
		token := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
		token.Header["kid"] = testKID
		signed, err := token.SignedString(privateKey)
		if err != nil {
			t.Fatalf("failed to sign assertion: %v", err)
		}
		return signed
	}

	// Test 1: No headers at all -> returns authenticated: false
	req := httptest.NewRequest(http.MethodGet, "/api/auth/cf-session", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	var res CFSessionResponse
	_ = json.Unmarshal(rec.Body.Bytes(), &res)
	if res.Authenticated {
		t.Error("expected authenticated=false without headers, got true")
	}

	// Test 2: SPOOF ATTACK: Attacker sends only Cf-Access-Authenticated-User-Email
	// CRITICAL SECURITY TEST: Must NOT authenticate the user!
	spoofReq := httptest.NewRequest(http.MethodGet, "/api/auth/cf-session", nil)
	spoofReq.Header.Set("Cf-Access-Authenticated-User-Email", "admin@rixus.com")
	spoofRec := httptest.NewRecorder()
	handler.ServeHTTP(spoofRec, spoofReq)

	if spoofRec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", spoofRec.Code)
	}
	var spoofRes CFSessionResponse
	_ = json.Unmarshal(spoofRec.Body.Bytes(), &spoofRes)
	if spoofRes.Authenticated {
		t.Fatal("SECURITY VULNERABILITY: Plain Cf-Access-Authenticated-User-Email header was trusted without JWT assertion!")
	}

	// Test 3: LEGITIMATE REQUEST: User provides valid Cf-Access-Jwt-Assertion
	validAssertion := createAssertion("verified-google-user@gmail.com")
	validReq := httptest.NewRequest(http.MethodGet, "/api/auth/cf-session", nil)
	validReq.Header.Set("Cf-Access-Jwt-Assertion", validAssertion)
	validRec := httptest.NewRecorder()
	handler.ServeHTTP(validRec, validReq)

	if validRec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", validRec.Code)
	}
	var validRes CFSessionResponse
	_ = json.Unmarshal(validRec.Body.Bytes(), &validRes)
	if !validRes.Authenticated {
		t.Fatal("expected authenticated=true with valid JWT assertion, got false")
	}
	if validRes.Email != "verified-google-user@gmail.com" {
		t.Errorf("expected email verified-google-user@gmail.com, got %q", validRes.Email)
	}

	// Test 4: REGISTRATION: Email mismatch with verified token is rejected with 403
	mismatchBody, _ := json.Marshal(map[string]any{
		"name":  "Hacker",
		"email": "victim@example.com",
	})
	mismatchReq := httptest.NewRequest(http.MethodPost, "/api/auth/register", bytes.NewReader(mismatchBody))
	mismatchReq.Header.Set("Content-Type", "application/json")
	mismatchReq.Header.Set("Cf-Access-Jwt-Assertion", validAssertion) // token has "verified-google-user@gmail.com"
	mismatchRec := httptest.NewRecorder()
	handler.ServeHTTP(mismatchRec, mismatchReq)

	if mismatchRec.Code != http.StatusForbidden {
		t.Fatalf("expected 403 when registration email mismatches verified JWT email, got %d", mismatchRec.Code)
	}
}

func TestDevLogoutAndGoogleAuthRedirect(t *testing.T) {
	s := &Server{
		JWTSecret:    "0123456789abcdef0123456789abcdef",
		CORSOrigin:   "*",
		AllowDevAuth: true,
		DevCFEmail:   "dev@example.com",
	}
	handler := s.Routes()

	// 1. Initial dev state: authenticated with DevCFEmail
	req1 := httptest.NewRequest(http.MethodGet, "/api/auth/cf-session", nil)
	rec1 := httptest.NewRecorder()
	handler.ServeHTTP(rec1, req1)
	var res1 CFSessionResponse
	_ = json.Unmarshal(rec1.Body.Bytes(), &res1)
	if !res1.Authenticated || res1.Email != "dev@example.com" {
		t.Fatalf("expected dev session authenticated as dev@example.com, got %v (%s)", res1.Authenticated, res1.Email)
	}

	// 2. User logs out: sets dev_cf_logged_out=true
	logoutReq := httptest.NewRequest(http.MethodPost, "/api/auth/logout", nil)
	logoutRec := httptest.NewRecorder()
	handler.ServeHTTP(logoutRec, logoutReq)
	if logoutRec.Code != http.StatusOK {
		t.Fatalf("expected 200 on logout, got %d", logoutRec.Code)
	}
	var devLogoutCookie *http.Cookie
	for _, c := range logoutRec.Result().Cookies() {
		if c.Name == "dev_cf_logged_out" {
			devLogoutCookie = c
			break
		}
	}
	if devLogoutCookie == nil || devLogoutCookie.Value != "true" {
		t.Fatalf("expected dev_cf_logged_out=true cookie, got %v", devLogoutCookie)
	}

	// 3. User checks session after logout: MUST be unauthenticated! (Not assumed to be logged in)
	req2 := httptest.NewRequest(http.MethodGet, "/api/auth/cf-session", nil)
	req2.AddCookie(devLogoutCookie)
	rec2 := httptest.NewRecorder()
	handler.ServeHTTP(rec2, req2)
	var res2 CFSessionResponse
	_ = json.Unmarshal(rec2.Body.Bytes(), &res2)
	if res2.Authenticated {
		t.Fatal("expected unauthenticated after dev logout, but server assumed user is still logged in!")
	}

	// 4. Production server: GET /api/auth/google without credentials redirects to /cdn-cgi/access/login
	prodServer := &Server{
		JWTSecret:    "0123456789abcdef0123456789abcdef",
		CORSOrigin:   "*",
		AllowDevAuth: false,
	}
	prodHandler := prodServer.Routes()
	googleReq := httptest.NewRequest(http.MethodGet, "/api/auth/google", nil)
	googleRec := httptest.NewRecorder()
	prodHandler.ServeHTTP(googleRec, googleReq)
	if googleRec.Code != http.StatusFound {
		t.Fatalf("expected 302 redirect for unauthenticated Google login, got %d", googleRec.Code)
	}
	if loc := googleRec.Header().Get("Location"); loc != "/cdn-cgi/access/login" {
		t.Fatalf("expected Location /cdn-cgi/access/login, got %q", loc)
	}
}
