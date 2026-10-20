package handlers

import (
	"log"
	"net/http"
	"time"

	"github.com/CodyNicholson/Rixus_Real_Estate_BE_Project/internal/auth"
)

// UserResponse represents user profile data returned to clients.
type UserResponse struct {
	ID        int64      `json:"id"`
	Name      string     `json:"name"`
	Email     string     `json:"email"`
	UserRole  string     `json:"user_role"`
	CreatedAt *time.Time `json:"created_at,omitempty"`
}

// registerUserRoutes registers endpoints on the user controller.
func (s *Server) registerUserRoutes(mux *http.ServeMux, authMW func(http.Handler) http.Handler) {
	// Primary endpoint: GET /api/users/me
	mux.Handle("GET /api/users/me", authMW(http.HandlerFunc(s.me)))

	// Backward-compatibility aliases
	mux.Handle("GET /api/auth/me", authMW(http.HandlerFunc(s.me)))
	mux.Handle("GET /api/me", authMW(http.HandlerFunc(s.me)))
}

// me returns the profile of the currently authenticated user.
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

