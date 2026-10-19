package handlers

import (
	"errors"
	"log"
	"net/http"
	"strconv"
	"strings"

	"github.com/CodyNicholson/Rixus_Real_Estate_BE_Project/internal/auth"
	"github.com/jackc/pgx/v5"
)

type Property struct {
	ID            int64   `json:"id"`
	UserID        int64   `json:"user_id"`
	StreetAddress string  `json:"street_address"`
	Address       string  `json:"address,omitempty"`
	City          string  `json:"city"`
	State         string  `json:"state"`
	Zip           string  `json:"zip"`
	MonthlyRent   float64 `json:"monthlyRent"`
}

// registerPropertyRoutes registers endpoints on the property controller.
func (s *Server) registerPropertyRoutes(mux *http.ServeMux, authMW func(http.Handler) http.Handler) {
	mux.Handle("GET /api/properties", authMW(http.HandlerFunc(s.listProperties)))
	mux.Handle("GET /api/properties/{id}", authMW(http.HandlerFunc(s.getProperty)))
	mux.Handle("POST /api/properties", authMW(http.HandlerFunc(s.createProperty)))
	mux.Handle("GET /api/users/{id}/properties", authMW(http.HandlerFunc(s.listProperties)))
	mux.Handle("POST /api/users/{id}/properties", authMW(http.HandlerFunc(s.createProperty)))
}

func (s *Server) listProperties(w http.ResponseWriter, r *http.Request) {
	callerID := auth.UserID(r.Context())
	callerRole := auth.UserRole(r.Context())
	targetUserID := callerID

	if idStr := r.PathValue("id"); idStr != "" {
		uid, err := strconv.ParseInt(idStr, 10, 64)
		if err != nil || uid <= 0 {
			writeErr(w, http.StatusBadRequest, "invalid user id")
			return
		}
		if callerID != uid && callerRole != "ADMIN" {
			log.Printf("listProperties: forbidden - caller %d (role %s) cannot access properties for user %d", callerID, callerRole, uid)
			writeErr(w, http.StatusForbidden, "forbidden")
			return
		}
		targetUserID = uid
	} else if uidStr := r.URL.Query().Get("user_id"); uidStr != "" {
		uid, err := strconv.ParseInt(uidStr, 10, 64)
		if err != nil || uid <= 0 {
			writeErr(w, http.StatusBadRequest, "invalid user id")
			return
		}
		if callerID != uid && callerRole != "ADMIN" {
			log.Printf("listProperties: forbidden - caller %d (role %s) cannot access properties for user %d", callerID, callerRole, uid)
			writeErr(w, http.StatusForbidden, "forbidden")
			return
		}
		targetUserID = uid
	}

	if s.DB == nil {
		writeErr(w, http.StatusInternalServerError, "database not configured")
		return
	}

	rows, err := s.DB.Query(r.Context(),
		`SELECT id, user_id, street_address, city, state, zip, monthly_rent::float8 FROM properties WHERE user_id = $1 ORDER BY id`,
		targetUserID)
	if err != nil {
		log.Printf("listProperties: query error: %v", err)
		writeErr(w, http.StatusInternalServerError, "internal error")
		return
	}
	defer rows.Close()
	props := []Property{}
	for rows.Next() {
		var p Property
		if err := rows.Scan(&p.ID, &p.UserID, &p.StreetAddress, &p.City, &p.State, &p.Zip, &p.MonthlyRent); err != nil {
			log.Printf("listProperties: scan error: %v", err)
			writeErr(w, http.StatusInternalServerError, "internal error")
			return
		}
		p.Address = p.StreetAddress
		props = append(props, p)
	}
	if rows.Err() != nil {
		log.Printf("listProperties: rows iteration error: %v", rows.Err())
		writeErr(w, http.StatusInternalServerError, "internal error")
		return
	}
	writeJSON(w, http.StatusOK, props)
}

func (s *Server) getProperty(w http.ResponseWriter, r *http.Request) {
	idStr := r.PathValue("id")
	propID, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil || propID <= 0 {
		writeErr(w, http.StatusBadRequest, "invalid property id")
		return
	}
	callerID := auth.UserID(r.Context())
	callerRole := auth.UserRole(r.Context())

	if s.DB == nil {
		writeErr(w, http.StatusInternalServerError, "database not configured")
		return
	}

	var p Property
	err = s.DB.QueryRow(r.Context(),
		`SELECT id, user_id, street_address, city, state, zip, monthly_rent::float8 FROM properties WHERE id = $1`,
		propID).Scan(&p.ID, &p.UserID, &p.StreetAddress, &p.City, &p.State, &p.Zip, &p.MonthlyRent)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			writeErr(w, http.StatusNotFound, "property not found")
			return
		}
		log.Printf("getProperty: query error: %v", err)
		writeErr(w, http.StatusInternalServerError, "internal error")
		return
	}
	if p.UserID != callerID && callerRole != "ADMIN" {
		log.Printf("getProperty: forbidden - caller %d (role %s) cannot access property %d owned by user %d", callerID, callerRole, propID, p.UserID)
		writeErr(w, http.StatusForbidden, "forbidden")
		return
	}
	p.Address = p.StreetAddress
	writeJSON(w, http.StatusOK, p)
}

func (s *Server) createProperty(w http.ResponseWriter, r *http.Request) {
	callerID := auth.UserID(r.Context())
	callerRole := auth.UserRole(r.Context())
	targetUserID := callerID

	if idStr := r.PathValue("id"); idStr != "" {
		uid, err := strconv.ParseInt(idStr, 10, 64)
		if err != nil || uid <= 0 {
			writeErr(w, http.StatusBadRequest, "invalid user id")
			return
		}
		if callerID != uid && callerRole != "ADMIN" {
			log.Printf("createProperty: forbidden - caller %d (role %s) cannot create property for user %d", callerID, callerRole, uid)
			writeErr(w, http.StatusForbidden, "forbidden")
			return
		}
		targetUserID = uid
	} else if uidStr := r.URL.Query().Get("user_id"); uidStr != "" {
		uid, err := strconv.ParseInt(uidStr, 10, 64)
		if err != nil || uid <= 0 {
			writeErr(w, http.StatusBadRequest, "invalid user id")
			return
		}
		if callerID != uid && callerRole != "ADMIN" {
			log.Printf("createProperty: forbidden - caller %d (role %s) cannot create property for user %d", callerID, callerRole, uid)
			writeErr(w, http.StatusForbidden, "forbidden")
			return
		}
		targetUserID = uid
	}

	var p struct {
		StreetAddress  string  `json:"street_address"`
		Address        string  `json:"address"`
		City           string  `json:"city"`
		State          string  `json:"state"`
		Zip            string  `json:"zip"`
		MonthlyRent    float64 `json:"monthlyRent"`
		MonthlyRentAlt float64 `json:"monthly_rent"`
	}
	if !decode(w, r, &p) {
		return
	}
	addr := strings.TrimSpace(p.StreetAddress)
	if addr == "" {
		addr = strings.TrimSpace(p.Address)
	}
	rent := p.MonthlyRent
	if rent == 0 && p.MonthlyRentAlt > 0 {
		rent = p.MonthlyRentAlt
	}
	if addr == "" || rent < 0 {
		log.Printf("createProperty: invalid input: street_address=%q monthlyRent=%.2f", addr, rent)
		writeErr(w, http.StatusBadRequest, "street_address is required and monthlyRent must be non-negative")
		return
	}
	if s.DB == nil {
		writeErr(w, http.StatusInternalServerError, "database not configured")
		return
	}
	var id int64
	err := s.DB.QueryRow(r.Context(),
		`INSERT INTO properties (user_id, street_address, city, state, zip, monthly_rent)
		 VALUES ($1,$2,$3,$4,$5,$6) RETURNING id`,
		targetUserID, addr, p.City, p.State, p.Zip, rent).Scan(&id)
	if err != nil {
		log.Printf("createProperty: database insert error: %v", err)
		writeErr(w, http.StatusInternalServerError, "internal error")
		return
	}
	out := Property{
		ID:            id,
		UserID:        targetUserID,
		StreetAddress: addr,
		Address:       addr,
		City:          p.City,
		State:         p.State,
		Zip:           p.Zip,
		MonthlyRent:   rent,
	}
	writeJSON(w, http.StatusCreated, out)
}

