package handlers

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/mail"

	"github.com/CodyNicholson/Rixus_Real_Estate_BE_Project/internal/auth"
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
	mux.HandleFunc("POST /api/auth/register", s.register)
	mux.HandleFunc("POST /api/auth/login", s.login)
	mux.Handle("GET /api/properties", authMW(http.HandlerFunc(s.listProperties)))
	mux.Handle("POST /api/properties", authMW(http.HandlerFunc(s.createProperty)))
	return s.cors(mux)
}

func (s *Server) cors(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", s.CORSOrigin)
		w.Header().Set("Access-Control-Allow-Headers", "Authorization, Content-Type")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
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
		writeErr(w, http.StatusBadRequest, "invalid request body")
		return false
	}
	return true
}

type credentials struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

func (s *Server) register(w http.ResponseWriter, r *http.Request) {
	var c credentials
	if !decode(w, r, &c) {
		return
	}
	if _, err := mail.ParseAddress(c.Email); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid email")
		return
	}
	// bcrypt only uses the first 72 bytes
	if len(c.Password) < 8 || len(c.Password) > 72 {
		writeErr(w, http.StatusBadRequest, "password must be 8-72 characters")
		return
	}
	hash, err := auth.HashPassword(c.Password)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "internal error")
		return
	}
	var id int64
	err = s.DB.QueryRow(r.Context(),
		`INSERT INTO users (email, password_hash) VALUES ($1, $2) RETURNING id`, c.Email, hash).Scan(&id)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			writeErr(w, http.StatusConflict, "email already registered")
			return
		}
		writeErr(w, http.StatusInternalServerError, "internal error")
		return
	}
	s.issueToken(w, id, http.StatusCreated)
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
	if err != nil || !auth.CheckPassword(hash, c.Password) {
		writeErr(w, http.StatusUnauthorized, "invalid email or password")
		return
	}
	s.issueToken(w, id, http.StatusOK)
}

func (s *Server) issueToken(w http.ResponseWriter, id int64, status int) {
	tok, err := auth.NewToken(s.JWTSecret, id)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "internal error")
		return
	}
	writeJSON(w, status, map[string]string{"token": tok})
}

type Property struct {
	ID          int64   `json:"id"`
	Address     string  `json:"address"`
	City        string  `json:"city"`
	State       string  `json:"state"`
	Zip         string  `json:"zip"`
	MonthlyRent float64 `json:"monthlyRent"`
}

func (s *Server) listProperties(w http.ResponseWriter, r *http.Request) {
	rows, err := s.DB.Query(r.Context(),
		`SELECT id, address, city, state, zip, monthly_rent::float8 FROM properties WHERE user_id = $1 ORDER BY id`,
		auth.UserID(r.Context()))
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "internal error")
		return
	}
	defer rows.Close()
	props := []Property{}
	for rows.Next() {
		var p Property
		if err := rows.Scan(&p.ID, &p.Address, &p.City, &p.State, &p.Zip, &p.MonthlyRent); err != nil {
			writeErr(w, http.StatusInternalServerError, "internal error")
			return
		}
		props = append(props, p)
	}
	if rows.Err() != nil {
		writeErr(w, http.StatusInternalServerError, "internal error")
		return
	}
	writeJSON(w, http.StatusOK, props)
}

func (s *Server) createProperty(w http.ResponseWriter, r *http.Request) {
	var p Property
	if !decode(w, r, &p) {
		return
	}
	if p.Address == "" || p.MonthlyRent < 0 {
		writeErr(w, http.StatusBadRequest, "address is required and monthlyRent must be non-negative")
		return
	}
	err := s.DB.QueryRow(r.Context(),
		`INSERT INTO properties (user_id, address, city, state, zip, monthly_rent)
		 VALUES ($1,$2,$3,$4,$5,$6) RETURNING id`,
		auth.UserID(r.Context()), p.Address, p.City, p.State, p.Zip, p.MonthlyRent).Scan(&p.ID)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "internal error")
		return
	}
	writeJSON(w, http.StatusCreated, p)
}
