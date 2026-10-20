package auth

import (
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// CFAccessClaims contains the verified claims in a Cloudflare Access JWT.
type CFAccessClaims struct {
	Email string `json:"email"`
	Type  string `json:"type"`
	Sub   string `json:"sub"`
	jwt.RegisteredClaims
}

type jwkKey struct {
	KID string `json:"kid"`
	Kty string `json:"kty"`
	Alg string `json:"alg"`
	Use string `json:"use"`
	N   string `json:"n"`
	E   string `json:"e"`
}

type publicCert struct {
	KID  string `json:"kid"`
	Cert string `json:"cert"`
}

type certsResponse struct {
	Keys        []jwkKey     `json:"keys"`
	PublicCert  *publicCert  `json:"public_cert"`
	PublicCerts []publicCert `json:"public_certs"`
}

// CFValidator validates Cloudflare Access JWT assertions against Cloudflare's public keys.
type CFValidator struct {
	TeamDomain     string
	PolicyAUD      string
	CertsURL       string
	ExpectedIssuer string
	HTTPClient     *http.Client

	mu        sync.RWMutex
	keys      map[string]*rsa.PublicKey
	expiresAt time.Time
}

// NewCFValidator constructs a validator for Cloudflare Access JWT assertions.
func NewCFValidator(teamDomain, policyAUD, certsURL string) *CFValidator {
	teamDomain = strings.TrimSpace(teamDomain)
	policyAUD = strings.TrimSpace(policyAUD)
	certsURL = strings.TrimSpace(certsURL)

	var expectedIssuer string
	if teamDomain != "" {
		if !strings.HasPrefix(teamDomain, "http://") && !strings.HasPrefix(teamDomain, "https://") {
			teamDomain = "https://" + teamDomain
		}
		if !strings.Contains(teamDomain, ".cloudflareaccess.com") && !strings.Contains(teamDomain, "/") {
			teamDomain = teamDomain + ".cloudflareaccess.com"
		}
		expectedIssuer = strings.TrimRight(teamDomain, "/")
	}

	if certsURL == "" && expectedIssuer != "" {
		certsURL = expectedIssuer + "/cdn-cgi/access/certs"
	}

	return &CFValidator{
		TeamDomain:     teamDomain,
		PolicyAUD:      policyAUD,
		CertsURL:       certsURL,
		ExpectedIssuer: expectedIssuer,
		HTTPClient:     &http.Client{Timeout: 10 * time.Second},
		keys:           make(map[string]*rsa.PublicKey),
	}
}

// ValidateToken parses, cryptographically verifies, and checks claims of a Cf-Access-Jwt-Assertion token.
func (v *CFValidator) ValidateToken(tokenString string) (*CFAccessClaims, error) {
	tokenString = strings.TrimSpace(tokenString)
	if tokenString == "" {
		return nil, errors.New("empty Cf-Access-Jwt-Assertion token")
	}

	var claims CFAccessClaims
	token, err := jwt.ParseWithClaims(tokenString, &claims, func(token *jwt.Token) (any, error) {
		if token.Method.Alg() != "RS256" {
			return nil, fmt.Errorf("unexpected signing algorithm %q: expected RS256", token.Method.Alg())
		}
		kid, _ := token.Header["kid"].(string)
		if kid == "" {
			return nil, errors.New("missing kid header in JWT")
		}
		return v.getKey(kid)
	}, jwt.WithValidMethods([]string{"RS256"}), jwt.WithExpirationRequired())

	if err != nil || !token.Valid {
		return nil, fmt.Errorf("invalid token: %w", err)
	}

	// Verify audience claim
	if v.PolicyAUD != "" {
		hasAud := false
		for _, a := range claims.Audience {
			if a == v.PolicyAUD {
				hasAud = true
				break
			}
		}
		if !hasAud {
			return nil, fmt.Errorf("token audience mismatch: expected %q, got %v", v.PolicyAUD, claims.Audience)
		}
	}

	// Verify issuer claim if configured
	if v.ExpectedIssuer != "" && claims.Issuer != "" {
		if !strings.EqualFold(strings.TrimRight(claims.Issuer, "/"), v.ExpectedIssuer) {
			return nil, fmt.Errorf("token issuer mismatch: expected %q, got %q", v.ExpectedIssuer, claims.Issuer)
		}
	}

	email := strings.TrimSpace(claims.Email)
	if email == "" {
		return nil, errors.New("token does not contain an email claim")
	}
	claims.Email = email

	return &claims, nil
}

func (v *CFValidator) getKey(kid string) (*rsa.PublicKey, error) {
	v.mu.RLock()
	if time.Now().Before(v.expiresAt) {
		if key, ok := v.keys[kid]; ok {
			v.mu.RUnlock()
			return key, nil
		}
	}
	v.mu.RUnlock()

	v.mu.Lock()
	defer v.mu.Unlock()

	// Double-check after acquiring write lock
	if time.Now().Before(v.expiresAt) {
		if key, ok := v.keys[kid]; ok {
			return key, nil
		}
	}

	if err := v.fetchKeysLocked(); err != nil {
		return nil, fmt.Errorf("failed to fetch Cloudflare certs: %w", err)
	}

	key, ok := v.keys[kid]
	if !ok {
		return nil, fmt.Errorf("public key not found for kid %q", kid)
	}
	return key, nil
}

func (v *CFValidator) fetchKeysLocked() error {
	if v.CertsURL == "" {
		return errors.New("no Cloudflare certs URL configured")
	}

	resp, err := v.HTTPClient.Get(v.CertsURL)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return fmt.Errorf("certs endpoint returned status %d: %s", resp.StatusCode, string(body))
	}

	var data certsResponse
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return fmt.Errorf("failed to decode certs response: %w", err)
	}

	newKeys := make(map[string]*rsa.PublicKey)

	// 1. Try parsing from public_certs (PEM format)
	for _, pc := range data.PublicCerts {
		if pc.KID != "" && pc.Cert != "" {
			pubKey, err := jwt.ParseRSAPublicKeyFromPEM([]byte(pc.Cert))
			if err == nil {
				newKeys[pc.KID] = pubKey
			}
		}
	}
	if data.PublicCert != nil && data.PublicCert.KID != "" && data.PublicCert.Cert != "" {
		pubKey, err := jwt.ParseRSAPublicKeyFromPEM([]byte(data.PublicCert.Cert))
		if err == nil {
			newKeys[data.PublicCert.KID] = pubKey
		}
	}

	// 2. Also parse JWK keys
	for _, k := range data.Keys {
		if k.KID != "" && k.Kty == "RSA" && k.N != "" && k.E != "" {
			if _, exists := newKeys[k.KID]; !exists {
				pubKey, err := parseJWKKey(k.N, k.E)
				if err == nil {
					newKeys[k.KID] = pubKey
				}
			}
		}
	}

	if len(newKeys) == 0 {
		return errors.New("no valid RSA public keys found in certs response")
	}

	v.keys = newKeys
	v.expiresAt = time.Now().Add(1 * time.Hour)
	return nil
}

func parseJWKKey(nStr, eStr string) (*rsa.PublicKey, error) {
	nBytes, err := base64.RawURLEncoding.DecodeString(nStr)
	if err != nil {
		nBytes, err = base64.URLEncoding.DecodeString(nStr)
		if err != nil {
			return nil, fmt.Errorf("failed to decode modulus: %w", err)
		}
	}
	eBytes, err := base64.RawURLEncoding.DecodeString(eStr)
	if err != nil {
		eBytes, err = base64.URLEncoding.DecodeString(eStr)
		if err != nil {
			return nil, fmt.Errorf("failed to decode exponent: %w", err)
		}
	}
	var eInt int
	for _, b := range eBytes {
		eInt = (eInt << 8) | int(b)
	}
	if eInt == 0 {
		return nil, errors.New("invalid RSA exponent")
	}
	return &rsa.PublicKey{
		N: new(big.Int).SetBytes(nBytes),
		E: eInt,
	}, nil
}
