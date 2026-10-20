package handlers

import (
	"errors"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/CodyNicholson/Rixus_Real_Estate_BE_Project/internal/auth"
	"github.com/jackc/pgx/v5"
)

type promoteRequest struct {
	UserID int64  `json:"user_id"`
	Email  string `json:"email"`
	Role   string `json:"role,omitempty"`
}

// registerAdminRoutes registers endpoints on the admin controller.
func (s *Server) registerAdminRoutes(mux *http.ServeMux, authMW func(http.Handler) http.Handler, adminMW func(http.Handler) http.Handler) {
	mux.Handle("GET /api/admin/users", authMW(adminMW(http.HandlerFunc(s.listUsers))))
	mux.Handle("POST /api/admin/promote", authMW(adminMW(http.HandlerFunc(s.promoteUser))))
	mux.Handle("POST /api/admin/users/{id}/promote", authMW(adminMW(http.HandlerFunc(s.promoteUser))))
}

// listUsers retrieves all registered users. Only accessible by ADMINs.
func (s *Server) listUsers(w http.ResponseWriter, r *http.Request) {
	if s.DB == nil {
		writeErr(w, http.StatusInternalServerError, "database not configured")
		return
	}

	rows, err := s.DB.Query(r.Context(),
		`SELECT id, name, email, user_role, created_at FROM users ORDER BY id ASC`)
	if err != nil {
		log.Printf("listUsers: database query error: %v", err)
		writeErr(w, http.StatusInternalServerError, "internal error")
		return
	}
	defer rows.Close()

	users := []UserResponse{}
	for rows.Next() {
		var u UserResponse
		var createdAt time.Time
		if err := rows.Scan(&u.ID, &u.Name, &u.Email, &u.UserRole, &createdAt); err != nil {
			log.Printf("listUsers: row scan error: %v", err)
			writeErr(w, http.StatusInternalServerError, "internal error")
			return
		}
		u.CreatedAt = &createdAt
		users = append(users, u)
	}
	if err := rows.Err(); err != nil {
		log.Printf("listUsers: rows iteration error: %v", err)
		writeErr(w, http.StatusInternalServerError, "internal error")
		return
	}

	writeJSON(w, http.StatusOK, users)
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
