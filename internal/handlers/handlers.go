package handlers

import (
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"net/mail"
	"os"
	"strings"
	"time"

	"github.com/CodyNicholson/Rixus_Real_Estate_BE_Project/internal/auth"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Server struct {
	DB         *pgxpool.Pool
	JWTSecret  string
	CORSOrigin string
}

func (s *Server) Routes() http.Handler {
	mux := http.NewServeMux()
	authMW := auth.Middleware(s.JWTSecret)
	mux.HandleFunc("GET /health", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})

	// Auth endpoints (both /api/... and /api/auth/... supported)
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
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")
		allowedOrigin := s.CORSOrigin
		if allowedOrigin == "" || allowedOrigin == "*" {
			if origin != "" {
				allowedOrigin = origin
			} else {
				allowedOrigin = "http://localhost:3000"
			}
		}

		w.Header().Set("Access-Control-Allow-Origin", allowedOrigin)
		w.Header().Set("Access-Control-Allow-Credentials", "true")
		w.Header().Set("Access-Control-Allow-Headers", "Authorization, Content-Type, Accept, X-Requested-With")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		w.Header().Set("Vary", "Origin")

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

type credentials struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type registerRequest struct {
	Name      string `json:"name"`
	Email     string `json:"email"`
	Password  string `json:"password"`
	UserRole  string `json:"user_role"`
	UserRoleC string `json:"userRole"`
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

func (s *Server) register(w http.ResponseWriter, r *http.Request) {
	var req registerRequest
	if !decode(w, r, &req) {
		return
	}
	req.Name = strings.TrimSpace(req.Name)
	if req.Name == "" {
		log.Printf("register: name is required")
		writeErr(w, http.StatusBadRequest, "name is required")
		return
	}
	if _, err := mail.ParseAddress(req.Email); err != nil {
		log.Printf("register: invalid email %q: %v", req.Email, err)
		writeErr(w, http.StatusBadRequest, "invalid email")
		return
	}
	// bcrypt only uses the first 72 bytes
	if len(req.Password) < 8 || len(req.Password) > 72 {
		log.Printf("register: password must be 8-72 characters (got %d)", len(req.Password))
		writeErr(w, http.StatusBadRequest, "password must be 8-72 characters")
		return
	}
	role := req.UserRole
	if role == "" {
		role = req.UserRoleC
	}
	role = strings.ToUpper(strings.TrimSpace(role))
	if role == "" {
		role = "VIEWER"
	}
	if role != "ADMIN" && role != "VIEWER" {
		log.Printf("register: invalid user_role %q (must be ADMIN or VIEWER)", role)
		writeErr(w, http.StatusBadRequest, "user_role must be ADMIN or VIEWER")
		return
	}

	hash, err := auth.HashPassword(req.Password)
	if err != nil {
		log.Printf("register: password hashing error: %v", err)
		writeErr(w, http.StatusInternalServerError, "internal error")
		return
	}
	var id int64
	err = s.DB.QueryRow(r.Context(),
		`INSERT INTO users (name, email, password_hash, user_role) VALUES ($1, $2, $3, $4) RETURNING id`,
		req.Name, req.Email, hash, role).Scan(&id)
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
	var c credentials
	if !decode(w, r, &c) {
		return
	}
	var id int64
	var hash string
	err := s.DB.QueryRow(r.Context(),
		`SELECT id, password_hash FROM users WHERE email = $1`, c.Email).Scan(&id, &hash)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			log.Printf("login: user not found: %s", c.Email)
		} else {
			log.Printf("login: database query error: %v", err)
		}
		writeErr(w, http.StatusUnauthorized, "invalid email or password")
		return
	}
	if !auth.CheckPassword(hash, c.Password) {
		log.Printf("login: invalid password for user: %s", c.Email)
		writeErr(w, http.StatusUnauthorized, "invalid email or password")
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
