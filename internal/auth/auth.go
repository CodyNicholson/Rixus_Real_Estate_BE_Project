package auth

import (
	"context"
	"errors"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

type ctxKey struct{}

const (
	AccessTokenDuration  = 15 * time.Minute
	RefreshTokenDuration = 7 * 24 * time.Hour
)

type TokenClaims struct {
	TokenType string `json:"token_type"`
	Email     string `json:"email,omitempty"`
	Name      string `json:"name,omitempty"`
	UserRole  string `json:"user_role,omitempty"`
	jwt.RegisteredClaims
}

// NewAccessToken issues a short-lived access JWT (15 minutes), optionally including user details.
func NewAccessToken(secret string, userID int64, extra ...string) (string, error) {
	var email, name, role string
	if len(extra) > 0 {
		email = extra[0]
	}
	if len(extra) > 1 {
		name = extra[1]
	}
	if len(extra) > 2 {
		role = extra[2]
	}
	claims := TokenClaims{
		TokenType: "access",
		Email:     email,
		Name:      name,
		UserRole:  role,
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   strconv.FormatInt(userID, 10),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(AccessTokenDuration)),
		},
	}
	return jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString([]byte(secret))
}

// NewRefreshToken issues a long-lived refresh JWT (7 days).
func NewRefreshToken(secret string, userID int64) (string, error) {
	claims := TokenClaims{
		TokenType: "refresh",
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   strconv.FormatInt(userID, 10),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(RefreshTokenDuration)),
		},
	}
	return jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString([]byte(secret))
}

// NewToken is a backward-compatible alias for NewAccessToken.
func NewToken(secret string, userID int64) (string, error) {
	return NewAccessToken(secret, userID)
}

type roleCtxKey struct{}

// ParseAccessTokenClaims validates an access token and returns its claims.
func ParseAccessTokenClaims(secret, token string) (*TokenClaims, error) {
	var claims TokenClaims
	t, err := jwt.ParseWithClaims(token, &claims, func(*jwt.Token) (any, error) {
		return []byte(secret), nil
	}, jwt.WithValidMethods([]string{"HS256"}), jwt.WithExpirationRequired())
	if err != nil || !t.Valid {
		return nil, errors.New("invalid token")
	}
	if claims.TokenType != "" && claims.TokenType != "access" {
		return nil, errors.New("invalid token type")
	}
	return &claims, nil
}

// ParseAccessToken validates an access token.
func ParseAccessToken(secret, token string) (int64, error) {
	claims, err := ParseAccessTokenClaims(secret, token)
	if err != nil {
		return 0, err
	}
	return strconv.ParseInt(claims.Subject, 10, 64)
}

// ParseRefreshToken validates a refresh token.
func ParseRefreshToken(secret, token string) (int64, error) {
	var claims TokenClaims
	t, err := jwt.ParseWithClaims(token, &claims, func(*jwt.Token) (any, error) {
		return []byte(secret), nil
	}, jwt.WithValidMethods([]string{"HS256"}), jwt.WithExpirationRequired())
	if err != nil || !t.Valid {
		return 0, errors.New("invalid refresh token")
	}
	if claims.TokenType != "refresh" {
		return 0, errors.New("not a refresh token")
	}
	return strconv.ParseInt(claims.Subject, 10, 64)
}

// ParseToken is a backward-compatible alias for ParseAccessToken.
func ParseToken(secret, token string) (int64, error) {
	return ParseAccessToken(secret, token)
}

// Middleware requires a valid bearer access token and stores the user id and user role in the context.
// For browser top-level requests (like downloading or viewing files in a new tab), it also falls back
// to query parameter "token" or "access_token".
func Middleware(secret string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			h := r.Header.Get("Authorization")
			tok, ok := strings.CutPrefix(h, "Bearer ")
			if !ok || strings.TrimSpace(tok) == "" {
				// Fallback to ?token= or ?access_token= query parameters
				tok = r.URL.Query().Get("token")
				if tok == "" {
					tok = r.URL.Query().Get("access_token")
				}
				if tok != "" {
					ok = true
				}
			}
			if !ok || tok == "" {
				log.Printf("auth: missing or malformed Bearer authorization header for %s %s", r.Method, r.URL.Path)
				http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
				return
			}
			claims, err := ParseAccessTokenClaims(secret, tok)
			if err != nil {
				log.Printf("auth: invalid token for %s %s: %v", r.Method, r.URL.Path, err)
				http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
				return
			}
			id, err := strconv.ParseInt(claims.Subject, 10, 64)
			if err != nil {
				log.Printf("auth: invalid token subject for %s %s: %v", r.Method, r.URL.Path, err)
				http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
				return
			}
			ctx := WithUser(r.Context(), id, claims.UserRole)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// WithUser adds the authenticated user ID and role to the context.
func WithUser(ctx context.Context, id int64, role string) context.Context {
	ctx = context.WithValue(ctx, ctxKey{}, id)
	return context.WithValue(ctx, roleCtxKey{}, role)
}

func UserID(ctx context.Context) int64 {
	id, _ := ctx.Value(ctxKey{}).(int64)
	return id
}

func UserRole(ctx context.Context) string {
	role, _ := ctx.Value(roleCtxKey{}).(string)
	return role
}
