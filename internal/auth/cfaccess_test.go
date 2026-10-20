package auth

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

type mockTransport struct {
	statusCode int
	body       string
}

func (m *mockTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	code := m.statusCode
	if code == 0 {
		code = http.StatusOK
	}
	return &http.Response{
		StatusCode: code,
		Body:       io.NopCloser(strings.NewReader(m.body)),
		Header:     make(http.Header),
	}, nil
}

func TestCFValidator(t *testing.T) {
	// 1. Generate RSA keypair for test
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("failed to generate RSA key: %v", err)
	}

	pubKeyBytes, err := x509.MarshalPKIXPublicKey(&privateKey.PublicKey)
	if err != nil {
		t.Fatalf("failed to marshal public key: %v", err)
	}
	pubPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "PUBLIC KEY",
		Bytes: pubKeyBytes,
	})

	testKID := "test-key-id-1"
	teamDomain := "https://myteam.cloudflareaccess.com"
	policyAUD := "expected-policy-aud-tag"

	res := certsResponse{
		PublicCerts: []publicCert{
			{
				KID:  testKID,
				Cert: string(pubPEM),
			},
		},
	}
	resBytes, _ := json.Marshal(res)

	validator := NewCFValidator(teamDomain, policyAUD, "https://myteam.cloudflareaccess.com/cdn-cgi/access/certs")
	validator.HTTPClient = &http.Client{
		Transport: &mockTransport{body: string(resBytes)},
	}

	// 2. Helper to generate tokens
	createToken := func(kid, aud, iss, email string, exp time.Time) string {
		claims := CFAccessClaims{
			Email: email,
			RegisteredClaims: jwt.RegisteredClaims{
				Audience:  jwt.ClaimStrings{aud},
				Issuer:    iss,
				ExpiresAt: jwt.NewNumericDate(exp),
				IssuedAt:  jwt.NewNumericDate(time.Now().Add(-1 * time.Minute)),
			},
		}
		token := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
		token.Header["kid"] = kid
		signed, err := token.SignedString(privateKey)
		if err != nil {
			t.Fatalf("failed to sign token: %v", err)
		}
		return signed
	}

	// Test 1: Valid token
	validToken := createToken(testKID, policyAUD, teamDomain, "testuser@gmail.com", time.Now().Add(1*time.Hour))
	claims, err := validator.ValidateToken(validToken)
	if err != nil {
		t.Fatalf("expected valid token to pass, got error: %v", err)
	}
	if claims.Email != "testuser@gmail.com" {
		t.Errorf("expected email testuser@gmail.com, got %q", claims.Email)
	}

	// Test 2: Audience mismatch
	wrongAudToken := createToken(testKID, "wrong-aud", teamDomain, "testuser@gmail.com", time.Now().Add(1*time.Hour))
	if _, err := validator.ValidateToken(wrongAudToken); err == nil {
		t.Error("expected error for audience mismatch, got nil")
	}

	// Test 3: Issuer mismatch
	wrongIssToken := createToken(testKID, policyAUD, "https://attacker.cloudflareaccess.com", "testuser@gmail.com", time.Now().Add(1*time.Hour))
	if _, err := validator.ValidateToken(wrongIssToken); err == nil {
		t.Error("expected error for issuer mismatch, got nil")
	}

	// Test 4: Expired token
	expiredToken := createToken(testKID, policyAUD, teamDomain, "testuser@gmail.com", time.Now().Add(-10*time.Minute))
	if _, err := validator.ValidateToken(expiredToken); err == nil {
		t.Error("expected error for expired token, got nil")
	}

	// Test 5: Unknown kid
	unknownKidToken := createToken("unknown-kid", policyAUD, teamDomain, "testuser@gmail.com", time.Now().Add(1*time.Hour))
	if _, err := validator.ValidateToken(unknownKidToken); err == nil {
		t.Error("expected error for unknown kid, got nil")
	}

	// Test 6: Empty token
	if _, err := validator.ValidateToken(""); err == nil {
		t.Error("expected error for empty token, got nil")
	}
}
