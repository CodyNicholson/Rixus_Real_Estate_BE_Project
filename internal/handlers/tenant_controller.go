package handlers

import (
	"context"
	"errors"
	"log"
	"math"
	"net/http"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/CodyNicholson/Rixus_Real_Estate_BE_Project/internal/auth"
	"github.com/jackc/pgx/v5"
)

// maxMonthlyRent is the largest value NUMERIC(12,2) can hold.
const maxMonthlyRent = 9999999999.99

type Tenant struct {
	TenantID       int64      `json:"tenant_id"`
	TenantInitials string     `json:"tenant_initials"`
	PropertyID     int64      `json:"property_id"`
	UnitNumber     int        `json:"unit_number"`
	MonthlyRent    float64    `json:"monthly_rent"`
	PetCount       int        `json:"pet_count"`
	MoveInDate     *time.Time `json:"move_in_date"`
	MoveOutDate    *time.Time `json:"move_out_date"`
	CreatedAt      time.Time  `json:"created_at"`
}

// tenantInput is the request body for creating or replacing a tenant.
type tenantInput struct {
	TenantInitials string     `json:"tenant_initials"`
	UnitNumber     *int       `json:"unit_number"`
	MonthlyRent    *float64   `json:"monthly_rent"`
	PetCount       *int       `json:"pet_count"`
	MoveInDate     *time.Time `json:"move_in_date"`
	MoveOutDate    *time.Time `json:"move_out_date"`
}

const tenantColumns = `tenant_id, tenant_initials, property_id, unit_number, monthly_rent::float8, pet_count, move_in_date, move_out_date, created_at`

var errPropertyNotFound = errors.New("property not found")

// registerTenantRoutes registers endpoints on the tenant controller.
func (s *Server) registerTenantRoutes(mux *http.ServeMux, authMW func(http.Handler) http.Handler) {
	mux.Handle("GET /api/properties/{id}/tenants", authMW(http.HandlerFunc(s.listTenants)))
	mux.Handle("POST /api/properties/{id}/tenants", authMW(http.HandlerFunc(s.createTenant)))
	mux.Handle("GET /api/properties/{id}/tenants/all", authMW(http.HandlerFunc(s.listTenants)))
	mux.Handle("GET /api/properties/{id}/tenants/{tenantId}", authMW(http.HandlerFunc(s.getTenant)))
	mux.Handle("PUT /api/properties/{id}/tenants/{tenantId}", authMW(http.HandlerFunc(s.updateTenant)))
	mux.Handle("DELETE /api/properties/{id}/tenants/{tenantId}", authMW(http.HandlerFunc(s.deleteTenant)))
}

func scanTenant(row pgx.Row) (Tenant, error) {
	var t Tenant
	err := row.Scan(&t.TenantID, &t.TenantInitials, &t.PropertyID, &t.UnitNumber, &t.MonthlyRent,
		&t.PetCount, &t.MoveInDate, &t.MoveOutDate, &t.CreatedAt)
	return t, err
}

// validate trims the input, applies defaults and checks it against the property's unit count.
func (in *tenantInput) validate(propertyUnits int) string {
	in.TenantInitials = strings.TrimSpace(in.TenantInitials)
	if n := utf8.RuneCountInString(in.TenantInitials); n < 1 || n > 4 {
		return "tenant_initials is required and must be at most 4 characters"
	}
	if in.UnitNumber == nil {
		one := 1
		in.UnitNumber = &one
	}
	if *in.UnitNumber < 1 || *in.UnitNumber > propertyUnits {
		return "unit_number must be between 1 and the property's unit count (" + strconv.Itoa(propertyUnits) + ")"
	}
	if in.MonthlyRent == nil || *in.MonthlyRent < 0 || *in.MonthlyRent > maxMonthlyRent || math.IsNaN(*in.MonthlyRent) {
		return "monthly_rent is required and must be between 0 and 9999999999.99"
	}
	if in.PetCount == nil {
		zero := 0
		in.PetCount = &zero
	}
	if *in.PetCount < 0 || *in.PetCount > math.MaxInt16 {
		return "pet_count must be between 0 and 32767"
	}
	if in.MoveInDate != nil && in.MoveOutDate != nil && in.MoveOutDate.Before(*in.MoveInDate) {
		return "move_out_date cannot be before move_in_date"
	}
	return ""
}

func parsePathID(r *http.Request, name string) (int64, bool) {
	id, err := strconv.ParseInt(r.PathValue(name), 10, 64)
	return id, err == nil && id > 0
}

// propertyAccess loads the owner and unit count of a property.
func (s *Server) propertyAccess(ctx context.Context, propID int64) (ownerID int64, units int, err error) {
	err = s.DB.QueryRow(ctx, `SELECT user_id, units FROM properties WHERE id = $1`, propID).Scan(&ownerID, &units)
	if errors.Is(err, pgx.ErrNoRows) {
		err = errPropertyNotFound
	}
	return ownerID, units, err
}

// authorizeProperty writes an error response and returns false unless the caller owns the property or is an ADMIN.
func (s *Server) authorizeProperty(w http.ResponseWriter, r *http.Request, fn string, propID int64) (units int, ok bool) {
	ownerID, units, err := s.propertyAccess(r.Context(), propID)
	if err != nil {
		if errors.Is(err, errPropertyNotFound) {
			writeErr(w, http.StatusNotFound, "property not found")
			return 0, false
		}
		log.Printf("%s: property query error: %v", fn, err)
		writeErr(w, http.StatusInternalServerError, "internal error")
		return 0, false
	}
	callerID := auth.UserID(r.Context())
	callerRole := auth.UserRole(r.Context())
	if ownerID != callerID && callerRole != "ADMIN" {
		log.Printf("%s: forbidden - caller %d (role %s) cannot access property %d owned by user %d", fn, callerID, callerRole, propID, ownerID)
		writeErr(w, http.StatusForbidden, "forbidden")
		return 0, false
	}
	return units, true
}

// loadTenant checks the caller may access the path's property, then fetches the path's tenant
// from that property. A tenant belonging to a different property is reported as not found.
func (s *Server) loadTenant(w http.ResponseWriter, r *http.Request, fn string) (Tenant, int, bool) {
	propID, ok := parsePathID(r, "id")
	if !ok {
		writeErr(w, http.StatusBadRequest, "invalid property id")
		return Tenant{}, 0, false
	}
	tenantID, ok := parsePathID(r, "tenantId")
	if !ok {
		writeErr(w, http.StatusBadRequest, "invalid tenant id")
		return Tenant{}, 0, false
	}
	if s.DB == nil {
		writeErr(w, http.StatusInternalServerError, "database not configured")
		return Tenant{}, 0, false
	}
	units, ok := s.authorizeProperty(w, r, fn, propID)
	if !ok {
		return Tenant{}, 0, false
	}
	t, err := scanTenant(s.DB.QueryRow(r.Context(),
		`SELECT `+tenantColumns+` FROM tenants WHERE tenant_id = $1 AND property_id = $2`, tenantID, propID))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			writeErr(w, http.StatusNotFound, "tenant not found")
			return Tenant{}, 0, false
		}
		log.Printf("%s: query error: %v", fn, err)
		writeErr(w, http.StatusInternalServerError, "internal error")
		return Tenant{}, 0, false
	}
	return t, units, true
}

func (s *Server) listTenants(w http.ResponseWriter, r *http.Request) {
	propID, ok := parsePathID(r, "id")
	if !ok {
		writeErr(w, http.StatusBadRequest, "invalid property id")
		return
	}
	if s.DB == nil {
		writeErr(w, http.StatusInternalServerError, "database not configured")
		return
	}
	if _, ok := s.authorizeProperty(w, r, "listTenants", propID); !ok {
		return
	}

	rows, err := s.DB.Query(r.Context(),
		`SELECT `+tenantColumns+` FROM tenants WHERE property_id = $1 ORDER BY unit_number, tenant_id`, propID)
	if err != nil {
		log.Printf("listTenants: query error: %v", err)
		writeErr(w, http.StatusInternalServerError, "internal error")
		return
	}
	defer rows.Close()
	tenants := []Tenant{}
	for rows.Next() {
		t, err := scanTenant(rows)
		if err != nil {
			log.Printf("listTenants: scan error: %v", err)
			writeErr(w, http.StatusInternalServerError, "internal error")
			return
		}
		tenants = append(tenants, t)
	}
	if rows.Err() != nil {
		log.Printf("listTenants: rows iteration error: %v", rows.Err())
		writeErr(w, http.StatusInternalServerError, "internal error")
		return
	}
	writeJSON(w, http.StatusOK, tenants)
}

func (s *Server) getTenant(w http.ResponseWriter, r *http.Request) {
	t, _, ok := s.loadTenant(w, r, "getTenant")
	if !ok {
		return
	}
	writeJSON(w, http.StatusOK, t)
}

func (s *Server) createTenant(w http.ResponseWriter, r *http.Request) {
	propID, ok := parsePathID(r, "id")
	if !ok {
		writeErr(w, http.StatusBadRequest, "invalid property id")
		return
	}
	var in tenantInput
	if !decode(w, r, &in) {
		return
	}
	if s.DB == nil {
		writeErr(w, http.StatusInternalServerError, "database not configured")
		return
	}
	units, ok := s.authorizeProperty(w, r, "createTenant", propID)
	if !ok {
		return
	}
	if msg := in.validate(units); msg != "" {
		writeErr(w, http.StatusBadRequest, msg)
		return
	}

	t, err := scanTenant(s.DB.QueryRow(r.Context(),
		`INSERT INTO tenants (tenant_initials, property_id, unit_number, monthly_rent, pet_count, move_in_date, move_out_date)
		 VALUES ($1,$2,$3,$4,$5,$6,$7) RETURNING `+tenantColumns,
		in.TenantInitials, propID, *in.UnitNumber, *in.MonthlyRent, *in.PetCount, in.MoveInDate, in.MoveOutDate))
	if err != nil {
		log.Printf("createTenant: database insert error: %v", err)
		writeErr(w, http.StatusInternalServerError, "internal error")
		return
	}
	writeJSON(w, http.StatusCreated, t)
}

// updateTenant replaces a tenant's fields; omitted unit_number/pet_count reset to their defaults
// and omitted dates are cleared. The tenant stays on its property.
func (s *Server) updateTenant(w http.ResponseWriter, r *http.Request) {
	existing, units, ok := s.loadTenant(w, r, "updateTenant")
	if !ok {
		return
	}
	var in tenantInput
	if !decode(w, r, &in) {
		return
	}
	if msg := in.validate(units); msg != "" {
		writeErr(w, http.StatusBadRequest, msg)
		return
	}

	t, err := scanTenant(s.DB.QueryRow(r.Context(),
		`UPDATE tenants SET tenant_initials = $1, unit_number = $2, monthly_rent = $3, pet_count = $4,
		 move_in_date = $5, move_out_date = $6
		 WHERE tenant_id = $7 RETURNING `+tenantColumns,
		in.TenantInitials, *in.UnitNumber, *in.MonthlyRent, *in.PetCount, in.MoveInDate, in.MoveOutDate, existing.TenantID))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			writeErr(w, http.StatusNotFound, "tenant not found")
			return
		}
		log.Printf("updateTenant: database update error: %v", err)
		writeErr(w, http.StatusInternalServerError, "internal error")
		return
	}
	writeJSON(w, http.StatusOK, t)
}

func (s *Server) deleteTenant(w http.ResponseWriter, r *http.Request) {
	existing, _, ok := s.loadTenant(w, r, "deleteTenant")
	if !ok {
		return
	}
	if _, err := s.DB.Exec(r.Context(), `DELETE FROM tenants WHERE tenant_id = $1`, existing.TenantID); err != nil {
		log.Printf("deleteTenant: database delete error: %v", err)
		writeErr(w, http.StatusInternalServerError, "internal error")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
