package handlers

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"net/mail"
	"os"
	"strings"
	"time"

	"github.com/CodyNicholson/Rixus_Real_Estate_BE_Project/internal/auth"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

type registerRequest struct {
	Name  string `json:"name"`
	Email string `json:"email"`
}

type loginRequest struct {
	Email string `json:"email,omitempty"`
}

type AuthResponse struct {
	AccessToken string        `json:"access_token"`
	Token       string        `json:"token,omitempty"` // For backward compatibility
	User        *UserResponse `json:"user,omitempty"`
}

type CFSessionResponse struct {
	Authenticated bool          `json:"authenticated"`
	Registered    bool          `json:"registered"`
	Email         string        `json:"email,omitempty"`
	User          *UserResponse `json:"user,omitempty"`
	AccessToken   string        `json:"access_token,omitempty"`
	Token         string        `json:"token,omitempty"`
}

// registerAuthRoutes registers endpoints on the auth controller.
func (s *Server) registerAuthRoutes(mux *http.ServeMux, authMW func(http.Handler) http.Handler, limitMW func(http.Handler) http.Handler) {
	mux.HandleFunc("GET /api/auth/google", s.authGoogle)
	mux.HandleFunc("GET /api/auth/login", s.authGoogle)
	mux.HandleFunc("GET /api/auth/cf-session", s.cfSession)
	mux.HandleFunc("GET /api/cf-session", s.cfSession)
	mux.Handle("POST /api/auth/register", limitMW(http.HandlerFunc(s.register)))
	mux.Handle("POST /api/register", limitMW(http.HandlerFunc(s.register)))
	mux.Handle("POST /api/auth/login", limitMW(http.HandlerFunc(s.login)))
	mux.Handle("POST /api/login", limitMW(http.HandlerFunc(s.login)))
	mux.HandleFunc("POST /api/auth/refresh", s.refresh)
	mux.HandleFunc("POST /api/refresh", s.refresh)
	mux.HandleFunc("POST /api/auth/logout", s.logout)
	mux.HandleFunc("POST /api/logout", s.logout)
}

func (s *Server) setRefreshTokenCookie(w http.ResponseWriter, r *http.Request, refreshToken string) {
	secure := true
	if cs := os.Getenv("COOKIE_SECURE"); cs != "" {
		secure = cs == "true" || cs == "1"
	} else if r.TLS == nil && r.Header.Get("X-Forwarded-Proto") != "https" {
		// When developing locally over plain HTTP without TLS, default to false so browsers accept it
		secure = false
	}

	http.SetCookie(w, &http.Cookie{
		Name:     "refresh_token",
		Value:    refreshToken,
		Path:     "/api/refresh",       // Restrict cookie to the refresh endpoint only
		Expires:  time.Now().Add(auth.RefreshTokenDuration),
		HttpOnly: true,                 // Prevents XSS from reading the cookie
		Secure:   secure,               // Requires HTTPS (turn off in local dev HTTP)
		SameSite: http.SameSiteLaxMode, // Mitigates CSRF
	})
}

func (s *Server) clearRefreshTokenCookie(w http.ResponseWriter, r *http.Request) {
	secure := true
	if cs := os.Getenv("COOKIE_SECURE"); cs != "" {
		secure = cs == "true" || cs == "1"
	} else if r.TLS == nil && r.Header.Get("X-Forwarded-Proto") != "https" {
		secure = false
	}

	http.SetCookie(w, &http.Cookie{
		Name:     "refresh_token",
		Value:    "",
		Path:     "/api/refresh",
		Expires:  time.Unix(0, 0),
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   secure,
		SameSite: http.SameSiteLaxMode,
	})
}

func (s *Server) issueAuthTokens(w http.ResponseWriter, r *http.Request, id int64, status int) {
	var u UserResponse
	if s.DB != nil && r != nil && r.Context() != nil {
		_ = s.DB.QueryRow(r.Context(),
			`SELECT id, name, email, user_role FROM users WHERE id = $1`, id).
			Scan(&u.ID, &u.Name, &u.Email, &u.UserRole)
	}

	accessToken, err := auth.NewAccessToken(s.JWTSecret, id, u.Email, u.Name, u.UserRole)
	if err != nil {
		log.Printf("issueAuthTokens: access token generation error for user %d: %v", id, err)
		writeErr(w, http.StatusInternalServerError, "internal error")
		return
	}
	refreshToken, err := auth.NewRefreshToken(s.JWTSecret, id)
	if err != nil {
		log.Printf("issueAuthTokens: refresh token generation error for user %d: %v", id, err)
		writeErr(w, http.StatusInternalServerError, "internal error")
		return
	}

	s.setRefreshTokenCookie(w, r, refreshToken)

	res := AuthResponse{
		AccessToken: accessToken,
		Token:       accessToken,
	}
	if u.ID != 0 {
		res.User = &u
	}
	writeJSON(w, status, res)
}

func (s *Server) issueToken(w http.ResponseWriter, id int64, status int) {
	s.issueAuthTokens(w, &http.Request{}, id, status)
}

func (s *Server) getVerifiedCFUserEmail(r *http.Request) (string, error) {
	// 1. Verify Cf-Access-Jwt-Assertion header against Cloudflare public certs
	tokenStr := strings.TrimSpace(r.Header.Get("Cf-Access-Jwt-Assertion"))
	if tokenStr != "" {
		if s.CFValidator == nil {
			log.Printf("cf_auth: Cf-Access-Jwt-Assertion received but CFValidator is not configured on server")
			return "", errors.New("Cloudflare Access JWT verification not configured")
		}
		claims, err := s.CFValidator.ValidateToken(tokenStr)
		if err != nil {
			log.Printf("cf_auth: Cf-Access-Jwt-Assertion validation failed: %v", err)
			return "", fmt.Errorf("invalid Cf-Access-Jwt-Assertion: %w", err)
		}
		return claims.Email, nil
	}

	// 2. Reject unverified spoofed header
	// CRITICAL SECURITY ENFORCEMENT:
	// Do NOT trust Cf-Access-Authenticated-User-Email header without a valid Cf-Access-Jwt-Assertion!
	if spoofed := r.Header.Get("Cf-Access-Authenticated-User-Email"); spoofed != "" {
		log.Printf("cf_auth: REJECTED unverified Cf-Access-Authenticated-User-Email header %q without valid Cf-Access-Jwt-Assertion", spoofed)
		return "", errors.New("unverified header rejected: Cf-Access-Jwt-Assertion is required")
	}

	// 3. Optional local development override: ONLY active if explicitly enabled
	if s.AllowDevAuth {
		// If the dev user has explicitly logged out, do not auto-authenticate via DEV_CF_EMAIL
		if cookie, err := r.Cookie("dev_cf_logged_out"); err == nil && cookie.Value == "true" {
			if devHeader := r.Header.Get("X-Dev-Email"); devHeader != "" {
				log.Printf("cf_auth: WARNING: using explicit X-Dev-Email=%q for development testing", devHeader)
				return strings.TrimSpace(devHeader), nil
			}
			return "", errors.New("user has logged out in development mode")
		}

		if devHeader := r.Header.Get("X-Dev-Email"); devHeader != "" {
			log.Printf("cf_auth: WARNING: using X-Dev-Email=%q for development testing", devHeader)
			return strings.TrimSpace(devHeader), nil
		}
		if s.DevCFEmail != "" {
			log.Printf("cf_auth: WARNING: using DEV_CF_EMAIL=%q for development testing", s.DevCFEmail)
			return s.DevCFEmail, nil
		}
	}

	return "", errors.New("no Cloudflare Access credentials provided")
}

func (s *Server) cfSession(w http.ResponseWriter, r *http.Request) {
	cfEmail, err := s.getVerifiedCFUserEmail(r)
	if err != nil || cfEmail == "" {
		writeJSON(w, http.StatusOK, CFSessionResponse{
			Authenticated: false,
			Registered:    false,
		})
		return
	}

	if s.DB == nil {
		writeJSON(w, http.StatusOK, CFSessionResponse{
			Authenticated: true,
			Registered:    false,
			Email:         cfEmail,
		})
		return
	}

	var u UserResponse
	err = s.DB.QueryRow(r.Context(),
		`SELECT id, name, email, user_role FROM users WHERE LOWER(email) = LOWER($1)`,
		cfEmail).Scan(&u.ID, &u.Name, &u.Email, &u.UserRole)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			// User is authenticated with Cloudflare Access Google OAuth, but not yet registered in application DB
			writeJSON(w, http.StatusOK, CFSessionResponse{
				Authenticated: true,
				Registered:    false,
				Email:         cfEmail,
			})
			return
		}
		log.Printf("cfSession: database query error for %q: %v", cfEmail, err)
		writeErr(w, http.StatusInternalServerError, "internal error")
		return
	}

	// User exists in database - issue auth tokens immediately
	accessToken, err := auth.NewAccessToken(s.JWTSecret, u.ID, u.Email, u.Name, u.UserRole)
	if err != nil {
		log.Printf("cfSession: access token generation error for user %d: %v", u.ID, err)
		writeErr(w, http.StatusInternalServerError, "internal error")
		return
	}
	refreshToken, err := auth.NewRefreshToken(s.JWTSecret, u.ID)
	if err != nil {
		log.Printf("cfSession: refresh token generation error for user %d: %v", u.ID, err)
		writeErr(w, http.StatusInternalServerError, "internal error")
		return
	}

	s.setRefreshTokenCookie(w, r, refreshToken)

	writeJSON(w, http.StatusOK, CFSessionResponse{
		Authenticated: true,
		Registered:    true,
		Email:         u.Email,
		User:          &u,
		AccessToken:   accessToken,
		Token:         accessToken,
	})
}

func (s *Server) register(w http.ResponseWriter, r *http.Request) {
	var req registerRequest
	if !decode(w, r, &req) {
		return
	}

	cfEmail, err := s.getVerifiedCFUserEmail(r)
	if err != nil || cfEmail == "" {
		log.Printf("register: unverified Cloudflare Access attempt: %v", err)
		writeErr(w, http.StatusUnauthorized, "Cloudflare Access Google OAuth authentication required to register")
		return
	}

	req.Name = strings.TrimSpace(req.Name)
	if req.Name == "" {
		log.Printf("register: name is required")
		writeErr(w, http.StatusBadRequest, "name is required")
		return
	}

	// Enforce that the registration email matches the Cloudflare validated email
	if req.Email == "" {
		req.Email = cfEmail
	} else if !strings.EqualFold(strings.TrimSpace(req.Email), cfEmail) {
		log.Printf("register: email mismatch: req=%q cf=%q", req.Email, cfEmail)
		writeErr(w, http.StatusForbidden, "registration email must match authenticated Google account")
		return
	}

	if _, err := mail.ParseAddress(req.Email); err != nil {
		log.Printf("register: invalid email %q: %v", req.Email, err)
		writeErr(w, http.StatusBadRequest, "invalid email")
		return
	}

	if s.DB == nil {
		writeErr(w, http.StatusInternalServerError, "database not configured")
		return
	}

	var id int64
	// New user registrations always receive the VIEWER role. Admins must be promoted via the admin-only endpoint.
	// Authentication is mediated solely by Cloudflare Access Google OAuth; no password hashes are stored.
	err = s.DB.QueryRow(r.Context(),
		`INSERT INTO users (name, email, user_role) VALUES ($1, $2, 'VIEWER') RETURNING id`,
		req.Name, req.Email).Scan(&id)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			log.Printf("register: email %q already registered", req.Email)
			writeErr(w, http.StatusConflict, "email already registered")
			return
		}
		log.Printf("register: database insert error: %v", err)
		writeErr(w, http.StatusInternalServerError, "internal error")
		return
	}
	s.issueAuthTokens(w, r, id, http.StatusCreated)
}

func (s *Server) login(w http.ResponseWriter, r *http.Request) {
	if s.AllowDevAuth {
		http.SetCookie(w, &http.Cookie{
			Name:     "dev_cf_logged_out",
			Value:    "",
			Path:     "/",
			MaxAge:   -1,
			Expires:  time.Unix(0, 0),
			HttpOnly: false,
			SameSite: http.SameSiteLaxMode,
		})
	}

	cfEmail, err := s.getVerifiedCFUserEmail(r)
	if s.AllowDevAuth && (err != nil || cfEmail == "") {
		var req loginRequest
		_ = json.NewDecoder(r.Body).Decode(&req)
		if req.Email != "" {
			cfEmail = strings.TrimSpace(req.Email)
			err = nil
		} else if devHeader := r.Header.Get("X-Dev-Email"); devHeader != "" {
			cfEmail = strings.TrimSpace(devHeader)
			err = nil
		} else if s.DevCFEmail != "" {
			cfEmail = s.DevCFEmail
			err = nil
		}
	}

	if err != nil || cfEmail == "" {
		writeErr(w, http.StatusUnauthorized, "Cloudflare Access Google OAuth authentication required")
		return
	}

	if s.DB == nil {
		writeErr(w, http.StatusInternalServerError, "database not configured")
		return
	}

	var id int64
	err = s.DB.QueryRow(r.Context(),
		`SELECT id FROM users WHERE LOWER(email) = LOWER($1)`, cfEmail).Scan(&id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			log.Printf("login: user not found: %s", cfEmail)
			writeErr(w, http.StatusNotFound, "user not registered; please complete registration")
			return
		}
		log.Printf("login: database query error: %v", err)
		writeErr(w, http.StatusInternalServerError, "internal error")
		return
	}
	s.issueAuthTokens(w, r, id, http.StatusOK)
}

func (s *Server) authGoogle(w http.ResponseWriter, r *http.Request) {
	if s.AllowDevAuth {
		http.SetCookie(w, &http.Cookie{
			Name:     "dev_cf_logged_out",
			Value:    "",
			Path:     "/",
			MaxAge:   -1,
			Expires:  time.Unix(0, 0),
			HttpOnly: false,
			SameSite: http.SameSiteLaxMode,
		})
	}

	cfEmail, err := s.getVerifiedCFUserEmail(r)
	if err != nil || cfEmail == "" {
		if s.AllowDevAuth {
			if s.DevCFEmail != "" {
				cfEmail = s.DevCFEmail
			} else {
				http.Redirect(w, r, "/?auth=dev_login", http.StatusFound)
				return
			}
		} else {
			// In production, unauthenticated requests trigger Cloudflare Access login challenge
			http.Redirect(w, r, "/cdn-cgi/access/login", http.StatusFound)
			return
		}
	}

	if s.DB == nil {
		http.Redirect(w, r, "/?auth=error&msg=database_not_configured", http.StatusFound)
		return
	}

	var u UserResponse
	err = s.DB.QueryRow(r.Context(),
		`SELECT id, name, email, user_role FROM users WHERE LOWER(email) = LOWER($1)`,
		cfEmail).Scan(&u.ID, &u.Name, &u.Email, &u.UserRole)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			http.Redirect(w, r, "/?auth=new_user", http.StatusFound)
			return
		}
		log.Printf("authGoogle: database query error for %q: %v", cfEmail, err)
		http.Redirect(w, r, "/?auth=error&msg=db_error", http.StatusFound)
		return
	}

	refreshToken, err := auth.NewRefreshToken(s.JWTSecret, u.ID)
	if err == nil {
		s.setRefreshTokenCookie(w, r, refreshToken)
	}
	http.Redirect(w, r, "/?auth=success", http.StatusFound)
}

func (s *Server) refresh(w http.ResponseWriter, r *http.Request) {
	cookie, err := r.Cookie("refresh_token")
	if err != nil || cookie.Value == "" {
		log.Printf("refresh: missing or empty refresh_token cookie on %s %s", r.Method, r.URL.Path)
		writeErr(w, http.StatusUnauthorized, "missing refresh token")
		return
	}

	id, err := auth.ParseRefreshToken(s.JWTSecret, cookie.Value)
	if err != nil {
		log.Printf("refresh: invalid or expired refresh token: %v", err)
		writeErr(w, http.StatusUnauthorized, "invalid or expired refresh token")
		return
	}

	// Issue a brand new Access Token and rotated Refresh Token cookie
	s.issueAuthTokens(w, r, id, http.StatusOK)
}

func (s *Server) logout(w http.ResponseWriter, r *http.Request) {
	s.clearRefreshTokenCookie(w, r)
	if s.AllowDevAuth {
		http.SetCookie(w, &http.Cookie{
			Name:     "dev_cf_logged_out",
			Value:    "true",
			Path:     "/",
			HttpOnly: false,
			SameSite: http.SameSiteLaxMode,
		})
	}
	resp := map[string]string{"status": "logged out"}
	// The app-domain /cdn-cgi/access/logout only clears this hostname's cookie; the global
	// Access session lives on the team domain and would silently re-issue the same identity.
	if s.CFValidator != nil && s.CFValidator.TeamDomain != "" {
		resp["cf_logout_url"] = strings.TrimRight(s.CFValidator.TeamDomain, "/") + "/cdn-cgi/access/logout"
	}
	writeJSON(w, http.StatusOK, resp)
}

