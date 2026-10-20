package handlers

import (
	"encoding/json"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/CodyNicholson/Rixus_Real_Estate_BE_Project/internal/auth"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Server struct {
	DB           *pgxpool.Pool
	JWTSecret    string
	CORSOrigin   string
	CFValidator  *auth.CFValidator
	DevCFEmail   string
	AllowDevAuth bool
	AuthLimiter  *AuthLimiter // brute-force protection for /login and /register; defaults applied if nil
}

func (s *Server) Routes() http.Handler {
	mux := http.NewServeMux()
	authMW := auth.Middleware(s.JWTSecret)
	adminMW := s.adminMiddleware

	if s.AuthLimiter == nil {
		s.AuthLimiter = NewAuthLimiter()
	}
	limitMW := s.AuthLimiter.Middleware

	mux.HandleFunc("GET /health", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})

	// Register endpoints from separated controllers
	s.registerAuthRoutes(mux, authMW, limitMW)
	s.registerUserRoutes(mux, authMW)
	s.registerPropertyRoutes(mux, authMW)
	s.registerTenantRoutes(mux, authMW)
	s.registerAdminRoutes(mux, authMW, adminMW)

	// Swagger documentation
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
		w.Header().Set("Access-Control-Allow-Headers", "Authorization, Content-Type, Accept, X-Requested-With, X-Dev-Email")
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
