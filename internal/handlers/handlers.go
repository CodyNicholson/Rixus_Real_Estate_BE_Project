package handlers

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"net/mail"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/CodyNicholson/Rixus_Real_Estate_BE_Project/internal/auth"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Server struct {
	DB           *pgxpool.Pool
	JWTSecret    string
	CORSOrigin   string
	CFValidator  *auth.CFValidator
	DevCFEmail   string
	AllowDevAuth bool
}

func (s *Server) Routes() http.Handler {
	mux := http.NewServeMux()
	authMW := auth.Middleware(s.JWTSecret)
	adminMW := s.adminMiddleware
	mux.HandleFunc("GET /health", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})

	// Auth endpoints (both /api/... and /api/auth/... supported)
	mux.HandleFunc("GET /api/auth/cf-session", s.cfSession)
	mux.HandleFunc("GET /api/cf-session", s.cfSession)
	mux.HandleFunc("POST /api/auth/register", s.register)
	mux.HandleFunc("POST /api/register", s.register)
	mux.HandleFunc("POST /api/auth/login", s.login)
	mux.HandleFunc("POST /api/login", s.login)
	mux.HandleFunc("POST /api/auth/refresh", s.refresh)
	mux.HandleFunc("POST /api/refresh", s.refresh)
	mux.HandleFunc("POST /api/auth/logout", s.logout)
	mux.HandleFunc("POST /api/logout", s.logout)
	mux.Handle("GET /api/auth/me", authMW(http.HandlerFunc(s.me)))
	mux.Handle("GET /api/me", authMW(http.HandlerFunc(s.me)))

	// Admin-only endpoints
	mux.Handle("POST /api/admin/promote", authMW(adminMW(http.HandlerFunc(s.promoteUser))))
	mux.Handle("POST /api/admin/users/{id}/promote", authMW(adminMW(http.HandlerFunc(s.promoteUser))))

	mux.Handle("GET /api/properties", authMW(http.HandlerFunc(s.listProperties)))
	mux.Handle("POST /api/properties", authMW(http.HandlerFunc(s.createProperty)))

	mux.HandleFunc("GET /{$}", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/swagger/", http.StatusMovedPermanently)
	})
	mux.HandleFunc("GET /swagger", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/swagger/", http.StatusMovedPermanently)
	})
	mux.HandleFunc("GET /swagger/", s.swaggerUI)
	mux.HandleFunc("GET /swagger/openapi.json", s.openapiSpec)
	mux.HandleFunc("GET /docs", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/swagger/", http.StatusMovedPermanently)
	})
	return s.logging(s.cors(mux))
}

type statusWriter struct {
	http.ResponseWriter
	status int
}

func (w *statusWriter) WriteHeader(status int) {
	w.status = status
	w.ResponseWriter.WriteHeader(status)
}

func (s *Server) logging(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		sw := &statusWriter{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(sw, r)
		log.Printf("%s %s %d %s", r.Method, r.URL.Path, sw.status, time.Since(start).Round(time.Microsecond))
	})
}

func (s *Server) cors(next http.Handler) http.Handler {
	allowed := map[string]bool{}
	for _, o := range strings.Split(s.CORSOrigin, ",") {
		o = strings.TrimRight(strings.TrimSpace(o), "/")
		// Fail closed: wildcards are never honored since we allow credentials.
		if o != "" && !strings.Contains(o, "*") {
			allowed[o] = true
		}
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Add("Vary", "Origin")
		origin := r.Header.Get("Origin")
		if origin == "" {
			next.ServeHTTP(w, r)
			return
		}
		if !allowed[origin] {
			if r.Method == http.MethodOptions {
				w.WriteHeader(http.StatusForbidden)
				return
			}
			next.ServeHTTP(w, r)
			return
		}

		w.Header().Set("Access-Control-Allow-Origin", origin)
		w.Header().Set("Access-Control-Allow-Credentials", "true")
		w.Header().Set("Access-Control-Allow-Headers", "Authorization, Content-Type, Accept, X-Requested-With")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")

		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeErr(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}

func decode(w http.ResponseWriter, r *http.Request, v any) bool {
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	if err := json.NewDecoder(r.Body).Decode(v); err != nil {
		log.Printf("decode error on %s %s: %v", r.Method, r.URL.Path, err)
		writeErr(w, http.StatusBadRequest, "invalid request body")
		return false
	}
	return true
}

type registerRequest struct {
	Name  string `json:"name"`
	Email string `json:"email"`
}

type UserResponse struct {
	ID       int64  `json:"id"`
	Name     string `json:"name"`
	Email    string `json:"email"`
	UserRole string `json:"user_role"`
}

type AuthResponse struct {
	AccessToken string        `json:"access_token"`
	Token       string        `json:"token,omitempty"` // For backward compatibility
	User        *UserResponse `json:"user,omitempty"`
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
	if s.AllowDevAuth && s.DevCFEmail != "" {
		log.Printf("cf_auth: WARNING: using DEV_CF_EMAIL=%q for development testing", s.DevCFEmail)
		return s.DevCFEmail, nil
	}

	return "", errors.New("no Cloudflare Access credentials provided")
}

type CFSessionResponse struct {
	Authenticated bool          `json:"authenticated"`
	Registered    bool          `json:"registered"`
	Email         string        `json:"email,omitempty"`
	User          *UserResponse `json:"user,omitempty"`
	AccessToken   string        `json:"access_token,omitempty"`
	Token         string        `json:"token,omitempty"`
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
	cfEmail, _ := s.getVerifiedCFUserEmail(r)

	req.Name = strings.TrimSpace(req.Name)
	if req.Name == "" {
		log.Printf("register: name is required")
		writeErr(w, http.StatusBadRequest, "name is required")
		return
	}

	// If authenticated via Cloudflare Access, enforce that the email matches Google OAuth
	if cfEmail != "" {
		if req.Email == "" {
			req.Email = cfEmail
		} else if !strings.EqualFold(strings.TrimSpace(req.Email), cfEmail) {
			log.Printf("register: email mismatch: req=%q cf=%q", req.Email, cfEmail)
			writeErr(w, http.StatusForbidden, "registration email must match authenticated Google account")
			return
		}
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
	err := s.DB.QueryRow(r.Context(),
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
	cfEmail, err := s.getVerifiedCFUserEmail(r)
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
	writeJSON(w, http.StatusOK, map[string]string{"status": "logged out"})
}

func (s *Server) me(w http.ResponseWriter, r *http.Request) {
	var u UserResponse
	err := s.DB.QueryRow(r.Context(),
		`SELECT id, name, email, user_role FROM users WHERE id = $1`,
		auth.UserID(r.Context())).Scan(&u.ID, &u.Name, &u.Email, &u.UserRole)
	if err != nil {
		log.Printf("me: user query error for id %d: %v", auth.UserID(r.Context()), err)
		writeErr(w, http.StatusNotFound, "user not found")
		return
	}
	writeJSON(w, http.StatusOK, u)
}

type Property struct {
	ID            int64   `json:"id"`
	StreetAddress string  `json:"street_address"`
	City          string  `json:"city"`
	State         string  `json:"state"`
	Zip           string  `json:"zip"`
	MonthlyRent   float64 `json:"monthlyRent"`
}

func (s *Server) listProperties(w http.ResponseWriter, r *http.Request) {
	rows, err := s.DB.Query(r.Context(),
		`SELECT id, street_address, city, state, zip, monthly_rent::float8 FROM properties WHERE user_id = $1 ORDER BY id`,
		auth.UserID(r.Context()))
	if err != nil {
		log.Printf("listProperties: query error: %v", err)
		writeErr(w, http.StatusInternalServerError, "internal error")
		return
	}
	defer rows.Close()
	props := []Property{}
	for rows.Next() {
		var p Property
		if err := rows.Scan(&p.ID, &p.StreetAddress, &p.City, &p.State, &p.Zip, &p.MonthlyRent); err != nil {
			log.Printf("listProperties: scan error: %v", err)
			writeErr(w, http.StatusInternalServerError, "internal error")
			return
		}
		props = append(props, p)
	}
	if rows.Err() != nil {
		log.Printf("listProperties: rows iteration error: %v", rows.Err())
		writeErr(w, http.StatusInternalServerError, "internal error")
		return
	}
	writeJSON(w, http.StatusOK, props)
}

func (s *Server) createProperty(w http.ResponseWriter, r *http.Request) {
	var p struct {
		StreetAddress string  `json:"street_address"`
		Address       string  `json:"address"`
		City          string  `json:"city"`
		State         string  `json:"state"`
		Zip           string  `json:"zip"`
		MonthlyRent   float64 `json:"monthlyRent"`
	}
	if !decode(w, r, &p) {
		return
	}
	addr := strings.TrimSpace(p.StreetAddress)
	if addr == "" {
		addr = strings.TrimSpace(p.Address)
	}
	if addr == "" || p.MonthlyRent < 0 {
		log.Printf("createProperty: invalid input: street_address=%q monthlyRent=%.2f", addr, p.MonthlyRent)
		writeErr(w, http.StatusBadRequest, "street_address is required and monthlyRent must be non-negative")
		return
	}
	var id int64
	err := s.DB.QueryRow(r.Context(),
		`INSERT INTO properties (user_id, street_address, city, state, zip, monthly_rent)
		 VALUES ($1,$2,$3,$4,$5,$6) RETURNING id`,
		auth.UserID(r.Context()), addr, p.City, p.State, p.Zip, p.MonthlyRent).Scan(&id)
	if err != nil {
		log.Printf("createProperty: database insert error: %v", err)
		writeErr(w, http.StatusInternalServerError, "internal error")
		return
	}
	out := Property{
		ID:            id,
		StreetAddress: addr,
		City:          p.City,
		State:         p.State,
		Zip:           p.Zip,
		MonthlyRent:   p.MonthlyRent,
	}
	writeJSON(w, http.StatusCreated, out)
}

func (s *Server) adminMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callerID := auth.UserID(r.Context())
		if callerID == 0 {
			writeErr(w, http.StatusUnauthorized, "unauthorized")
			return
		}

		var role string
		if s.DB != nil && r.Context() != nil {
			err := s.DB.QueryRow(r.Context(), `SELECT user_role FROM users WHERE id = $1`, callerID).Scan(&role)
			if err != nil {
				if errors.Is(err, pgx.ErrNoRows) {
					log.Printf("adminMiddleware: caller id %d not found in database", callerID)
					writeErr(w, http.StatusUnauthorized, "user not found")
					return
				}
				log.Printf("adminMiddleware: database query error for caller %d: %v", callerID, err)
				writeErr(w, http.StatusInternalServerError, "internal error")
				return
			}
		} else {
			// In unit tests without database pool, check the role stored in token context
			role = auth.UserRole(r.Context())
		}

		if role != "ADMIN" {
			log.Printf("adminMiddleware: forbidden - caller %d has role %q, ADMIN required", callerID, role)
			writeErr(w, http.StatusForbidden, "admin privileges required")
			return
		}

		next.ServeHTTP(w, r)
	})
}

type promoteRequest struct {
	UserID int64  `json:"user_id"`
	Email  string `json:"email"`
	Role   string `json:"role,omitempty"`
}

func (s *Server) promoteUser(w http.ResponseWriter, r *http.Request) {
	var targetID int64
	var targetEmail string
	newRole := "ADMIN"

	if idStr := r.PathValue("id"); idStr != "" {
		parsedID, err := strconv.ParseInt(idStr, 10, 64)
		if err != nil || parsedID <= 0 {
			writeErr(w, http.StatusBadRequest, "invalid user id in path")
			return
		}
		targetID = parsedID
	}

	if r.Body != nil && r.ContentLength > 0 {
		var req promoteRequest
		if decode(w, r, &req) {
			if targetID == 0 && req.UserID > 0 {
				targetID = req.UserID
			}
			if req.Email != "" {
				targetEmail = strings.TrimSpace(req.Email)
			}
			if req.Role != "" {
				rUpper := strings.ToUpper(strings.TrimSpace(req.Role))
				if rUpper != "ADMIN" && rUpper != "VIEWER" {
					writeErr(w, http.StatusBadRequest, "role must be ADMIN or VIEWER")
					return
				}
				newRole = rUpper
			}
		} else {
			return
		}
	}

	if targetID == 0 && targetEmail == "" {
		writeErr(w, http.StatusBadRequest, "user_id or email is required")
		return
	}

	if s.DB == nil {
		writeErr(w, http.StatusInternalServerError, "database not configured")
		return
	}

	var u UserResponse
	var err error
	if targetID > 0 {
		err = s.DB.QueryRow(r.Context(),
			`UPDATE users SET user_role = $1 WHERE id = $2 RETURNING id, name, email, user_role`,
			newRole, targetID).Scan(&u.ID, &u.Name, &u.Email, &u.UserRole)
	} else {
		err = s.DB.QueryRow(r.Context(),
			`UPDATE users SET user_role = $1 WHERE LOWER(email) = LOWER($2) RETURNING id, name, email, user_role`,
			newRole, targetEmail).Scan(&u.ID, &u.Name, &u.Email, &u.UserRole)
	}

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			writeErr(w, http.StatusNotFound, "user not found")
			return
		}
		log.Printf("promoteUser: database update error: %v", err)
		writeErr(w, http.StatusInternalServerError, "internal error")
		return
	}

	log.Printf("promoteUser: user %d (%s) set to role %s by admin %d", u.ID, u.Email, u.UserRole, auth.UserID(r.Context()))
	writeJSON(w, http.StatusOK, u)
}
