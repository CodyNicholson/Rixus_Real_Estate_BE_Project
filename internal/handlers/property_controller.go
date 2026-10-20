package handlers

import (
	"errors"
	"log"
	"math"
	"net/http"
	"slices"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/CodyNicholson/Rixus_Real_Estate_BE_Project/internal/auth"
	"github.com/jackc/pgx/v5"
)

// maxBasisAmount is the largest value NUMERIC(14,2) can hold.
const maxBasisAmount = 999999999999.99

var (
	utilityKeys  = []string{"water", "electric", "gas", "trash"}
	utilityPayer = []string{"LANDLORD", "TENANT"}
)

// Property is a property with its acquisition basis and depreciation inputs.
type Property struct {
	ID                        int64             `json:"id"`
	UserID                    int64             `json:"user_id"`
	StreetAddress             string            `json:"street_address"`
	Address                   string            `json:"address,omitempty"`
	City                      string            `json:"city"`
	State                     string            `json:"state"`
	Zip                       string            `json:"zip"`
	Units                     int               `json:"units"`
	Name                      string            `json:"name"`
	Category                  string            `json:"category"`
	LegalEntityID             *int64            `json:"legal_entity_id"`
	LegalEntityName           *string           `json:"legal_entity_name"`
	ParcelPIN                 *string           `json:"parcel_pin"`
	YearBuilt                 *int              `json:"year_built"`
	GrossSqFt                 *int              `json:"gross_sq_ft"`
	AcquisitionDate           *time.Time        `json:"acquisition_date"`
	PurchasePrice             float64           `json:"purchase_price"`
	ClosingCosts              float64           `json:"closing_costs"`
	InitialRehabBudget        float64           `json:"initial_rehab_budget"`
	InitialOperatingReserve   float64           `json:"initial_operating_reserve"`
	AfterRepairValue          *float64          `json:"after_repair_value"`
	LandValueAllocationPct    float64           `json:"land_value_allocation_pct"`
	DepreciationScheduleYears float64           `json:"depreciation_schedule_years"`
	UtilityConfig             map[string]string `json:"utility_config"`
	IsActive                  bool              `json:"is_active"`
	CreatedAt                 time.Time         `json:"created_at"`
}

// propertyInput is the request body for creating or replacing a property. Omitted financial fields take their
// defaults, so a create with only an address and unit count still works.
type propertyInput struct {
	StreetAddress             string            `json:"street_address"`
	Address                   string            `json:"address"`
	City                      string            `json:"city"`
	State                     string            `json:"state"`
	Zip                       string            `json:"zip"`
	Units                     *int              `json:"units"`
	Name                      string            `json:"name"`
	Category                  string            `json:"category"`
	LegalEntityID             *int64            `json:"legal_entity_id"`
	ParcelPIN                 *string           `json:"parcel_pin"`
	YearBuilt                 *int              `json:"year_built"`
	GrossSqFt                 *int              `json:"gross_sq_ft"`
	AcquisitionDate           *time.Time        `json:"acquisition_date"`
	PurchasePrice             float64           `json:"purchase_price"`
	ClosingCosts              float64           `json:"closing_costs"`
	InitialRehabBudget        float64           `json:"initial_rehab_budget"`
	InitialOperatingReserve   float64           `json:"initial_operating_reserve"`
	AfterRepairValue          *float64          `json:"after_repair_value"`
	LandValueAllocationPct    *float64          `json:"land_value_allocation_pct"`
	DepreciationScheduleYears *float64          `json:"depreciation_schedule_years"`
	UtilityConfig             map[string]string `json:"utility_config"`
	IsActive                  *bool             `json:"is_active"`
}

// propertyColumns selects a Property from properties p joined to legal_entities le.
const propertyColumns = `p.id, p.user_id, p.street_address, p.city, p.state, p.zip, p.units, p.name, p.category::text,
	p.legal_entity_id, le.name, p.parcel_pin, p.year_built, p.gross_sq_ft, p.acquisition_date, p.purchase_price,
	p.closing_costs, p.initial_rehab_budget, p.initial_operating_reserve, p.after_repair_value,
	p.land_value_allocation_pct, p.depreciation_schedule_years, p.utility_config, p.is_active, p.created_at`

// registerPropertyRoutes registers endpoints on the property controller.
func (s *Server) registerPropertyRoutes(mux *http.ServeMux, authMW func(http.Handler) http.Handler) {
	mux.Handle("GET /api/properties", authMW(http.HandlerFunc(s.listProperties)))
	mux.Handle("GET /api/properties/{id}", authMW(http.HandlerFunc(s.getProperty)))
	mux.Handle("POST /api/properties", authMW(http.HandlerFunc(s.createProperty)))
	mux.Handle("PUT /api/properties/{id}", authMW(http.HandlerFunc(s.updateProperty)))
	mux.Handle("DELETE /api/properties/{id}", authMW(http.HandlerFunc(s.deleteProperty)))
	mux.Handle("GET /api/users/{id}/properties", authMW(http.HandlerFunc(s.listProperties)))
	mux.Handle("POST /api/users/{id}/properties", authMW(http.HandlerFunc(s.createProperty)))
}

func scanProperty(row pgx.Row) (Property, error) {
	var p Property
	err := row.Scan(&p.ID, &p.UserID, &p.StreetAddress, &p.City, &p.State, &p.Zip, &p.Units, &p.Name, &p.Category,
		&p.LegalEntityID, &p.LegalEntityName, &p.ParcelPIN, &p.YearBuilt, &p.GrossSqFt, &p.AcquisitionDate, &p.PurchasePrice,
		&p.ClosingCosts, &p.InitialRehabBudget, &p.InitialOperatingReserve, &p.AfterRepairValue,
		&p.LandValueAllocationPct, &p.DepreciationScheduleYears, &p.UtilityConfig, &p.IsActive, &p.CreatedAt)
	p.Address = p.StreetAddress
	return p, err
}

// validAmount reports whether v is a finite amount between 0 and max.
func validAmount(v, max float64) bool {
	return !math.IsNaN(v) && v >= 0 && v <= max
}

// optText trims an optional string, maps blank to nil and reports whether it fits in max characters.
func optText(p *string, max int) (*string, bool) {
	if p == nil {
		return nil, true
	}
	v := strings.TrimSpace(*p)
	if v == "" {
		return nil, true
	}
	return &v, utf8.RuneCountInString(v) <= max
}

// validate trims the input, applies defaults and returns an error message on bad input.
func (in *propertyInput) validate() string {
	in.StreetAddress = strings.TrimSpace(in.StreetAddress)
	if in.StreetAddress == "" {
		in.StreetAddress = strings.TrimSpace(in.Address)
	}
	if in.Units == nil {
		one := 1
		in.Units = &one
	}
	if in.StreetAddress == "" || *in.Units < 1 || *in.Units > math.MaxInt16 {
		return "street_address is required and units must be between 1 and 32767"
	}
	in.City, in.State, in.Zip = strings.TrimSpace(in.City), strings.TrimSpace(in.State), strings.TrimSpace(in.Zip)
	in.Name = strings.TrimSpace(in.Name)
	if utf8.RuneCountInString(in.Name) > 150 {
		return "name must be at most 150 characters"
	}
	in.Category = strings.ToUpper(strings.TrimSpace(in.Category))
	if in.Category == "" {
		in.Category = "SINGLE_FAMILY"
	}
	if !slices.Contains(propertyCategories, in.Category) {
		return "category must be one of " + strings.Join(propertyCategories, ", ")
	}
	if in.LegalEntityID != nil && *in.LegalEntityID <= 0 {
		return "invalid legal_entity_id"
	}
	var ok bool
	if in.ParcelPIN, ok = optText(in.ParcelPIN, 50); !ok {
		return "parcel_pin must be at most 50 characters"
	}
	if in.YearBuilt != nil && (*in.YearBuilt < 1600 || *in.YearBuilt > 2100) {
		return "year_built must be between 1600 and 2100"
	}
	if in.GrossSqFt != nil && (*in.GrossSqFt < 0 || *in.GrossSqFt > math.MaxInt32) {
		return "gross_sq_ft cannot be negative"
	}
	for name, v := range map[string]float64{
		"purchase_price":            in.PurchasePrice,
		"closing_costs":             in.ClosingCosts,
		"initial_rehab_budget":      in.InitialRehabBudget,
		"initial_operating_reserve": in.InitialOperatingReserve,
	} {
		if !validAmount(v, maxBasisAmount) {
			return name + " must be between 0 and 999999999999.99"
		}
	}
	if in.AfterRepairValue != nil && !validAmount(*in.AfterRepairValue, maxBasisAmount) {
		return "after_repair_value must be between 0 and 999999999999.99"
	}
	if in.LandValueAllocationPct == nil {
		v := 0.2
		in.LandValueAllocationPct = &v
	}
	if !validAmount(*in.LandValueAllocationPct, 1) {
		return "land_value_allocation_pct must be a fraction between 0 and 1"
	}
	if in.DepreciationScheduleYears == nil {
		v := 27.5
		in.DepreciationScheduleYears = &v
	}
	if math.IsNaN(*in.DepreciationScheduleYears) || *in.DepreciationScheduleYears <= 0 || *in.DepreciationScheduleYears > 999.9 {
		return "depreciation_schedule_years must be greater than 0 and at most 999.9"
	}
	cfg := map[string]string{"water": "LANDLORD", "electric": "TENANT", "gas": "TENANT", "trash": "LANDLORD"}
	for k, v := range in.UtilityConfig {
		v = strings.ToUpper(strings.TrimSpace(v))
		if !slices.Contains(utilityKeys, k) || !slices.Contains(utilityPayer, v) {
			return "utility_config keys must be water, electric, gas or trash, each LANDLORD or TENANT"
		}
		cfg[k] = v
	}
	in.UtilityConfig = cfg
	if in.IsActive == nil {
		active := true
		in.IsActive = &active
	}
	return ""
}

// args returns the column values in propertyWriteColumns order, after user_id.
func (in *propertyInput) args() []any {
	return []any{in.StreetAddress, in.City, in.State, in.Zip, *in.Units, in.Name, in.Category, in.LegalEntityID,
		in.ParcelPIN, in.YearBuilt, in.GrossSqFt, in.AcquisitionDate, in.PurchasePrice, in.ClosingCosts,
		in.InitialRehabBudget, in.InitialOperatingReserve, in.AfterRepairValue, *in.LandValueAllocationPct,
		*in.DepreciationScheduleYears, in.UtilityConfig, *in.IsActive}
}

// checkLegalEntity writes an error and returns false unless the entity is unset or belongs to the property owner.
func (s *Server) checkLegalEntity(w http.ResponseWriter, r *http.Request, fn string, entityID *int64, ownerID int64) bool {
	if entityID == nil {
		return true
	}
	var exists bool
	err := s.DB.QueryRow(r.Context(),
		`SELECT EXISTS (SELECT 1 FROM legal_entities WHERE id = $1 AND user_id = $2)`, *entityID, ownerID).Scan(&exists)
	if err != nil {
		log.Printf("%s: legal entity query error: %v", fn, err)
		writeErr(w, http.StatusInternalServerError, "internal error")
		return false
	}
	if !exists {
		writeErr(w, http.StatusBadRequest, "legal_entity_id does not belong to the property owner")
		return false
	}
	return true
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
		`SELECT `+propertyColumns+` FROM properties p LEFT JOIN legal_entities le ON le.id = p.legal_entity_id
		 WHERE p.user_id = $1 ORDER BY p.id`,
		targetUserID)
	if err != nil {
		log.Printf("listProperties: query error: %v", err)
		writeErr(w, http.StatusInternalServerError, "internal error")
		return
	}
	defer rows.Close()
	props := []Property{}
	for rows.Next() {
		p, err := scanProperty(rows)
		if err != nil {
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

	p, err := scanProperty(s.DB.QueryRow(r.Context(),
		`SELECT `+propertyColumns+` FROM properties p LEFT JOIN legal_entities le ON le.id = p.legal_entity_id WHERE p.id = $1`,
		propID))
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

	var in propertyInput
	if !decode(w, r, &in) {
		return
	}
	if msg := in.validate(); msg != "" {
		log.Printf("createProperty: invalid input: %s", msg)
		writeErr(w, http.StatusBadRequest, msg)
		return
	}
	if s.DB == nil {
		writeErr(w, http.StatusInternalServerError, "database not configured")
		return
	}
	if !s.checkLegalEntity(w, r, "createProperty", in.LegalEntityID, targetUserID) {
		return
	}
	p, err := scanProperty(s.DB.QueryRow(r.Context(),
		`WITH p AS (
			INSERT INTO properties (user_id, street_address, city, state, zip, units, name, category, legal_entity_id,
				parcel_pin, year_built, gross_sq_ft, acquisition_date, purchase_price, closing_costs, initial_rehab_budget,
				initial_operating_reserve, after_repair_value, land_value_allocation_pct, depreciation_schedule_years,
				utility_config, is_active)
			VALUES ($1,$2,$3,$4,$5,$6,$7,$8::text::property_category,$9,$10,$11,$12,$13,$14,$15,$16,$17,$18,$19,$20,$21,$22)
			RETURNING *)
		 SELECT `+propertyColumns+` FROM p LEFT JOIN legal_entities le ON le.id = p.legal_entity_id`,
		append([]any{targetUserID}, in.args()...)...))
	if err != nil {
		log.Printf("createProperty: database insert error: %v", err)
		writeErr(w, http.StatusInternalServerError, "internal error")
		return
	}
	writeJSON(w, http.StatusCreated, p)
}

// updateProperty replaces a property's address, unit count and financial fields; omitted financial fields
// reset to their defaults. The unit count cannot drop below the highest unit a tenant occupies.
func (s *Server) updateProperty(w http.ResponseWriter, r *http.Request) {
	propID, ok := parsePathID(r, "id")
	if !ok {
		writeErr(w, http.StatusBadRequest, "invalid property id")
		return
	}
	var in propertyInput
	if !decode(w, r, &in) {
		return
	}
	if msg := in.validate(); msg != "" {
		writeErr(w, http.StatusBadRequest, msg)
		return
	}
	if s.DB == nil {
		writeErr(w, http.StatusInternalServerError, "database not configured")
		return
	}
	if _, ok := s.authorizeProperty(w, r, "updateProperty", propID); !ok {
		return
	}
	ownerID, _, err := s.propertyAccess(r.Context(), propID)
	if err != nil {
		log.Printf("updateProperty: property query error: %v", err)
		writeErr(w, http.StatusInternalServerError, "internal error")
		return
	}
	if !s.checkLegalEntity(w, r, "updateProperty", in.LegalEntityID, ownerID) {
		return
	}
	var maxTenantUnit int
	if err := s.DB.QueryRow(r.Context(),
		`SELECT COALESCE(MAX(unit_number), 0) FROM tenants WHERE property_id = $1`, propID).Scan(&maxTenantUnit); err != nil {
		log.Printf("updateProperty: tenant query error: %v", err)
		writeErr(w, http.StatusInternalServerError, "internal error")
		return
	}
	if *in.Units < maxTenantUnit {
		writeErr(w, http.StatusBadRequest, "units cannot be less than "+strconv.Itoa(maxTenantUnit)+", the highest unit a tenant occupies")
		return
	}

	p, err := scanProperty(s.DB.QueryRow(r.Context(),
		`WITH p AS (
			UPDATE properties SET street_address = $2, city = $3, state = $4, zip = $5, units = $6, name = $7,
				category = $8::text::property_category, legal_entity_id = $9, parcel_pin = $10, year_built = $11,
				gross_sq_ft = $12, acquisition_date = $13, purchase_price = $14, closing_costs = $15,
				initial_rehab_budget = $16, initial_operating_reserve = $17, after_repair_value = $18,
				land_value_allocation_pct = $19, depreciation_schedule_years = $20, utility_config = $21, is_active = $22
			WHERE id = $1 RETURNING *)
		 SELECT `+propertyColumns+` FROM p LEFT JOIN legal_entities le ON le.id = p.legal_entity_id`,
		append([]any{propID}, in.args()...)...))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			writeErr(w, http.StatusNotFound, "property not found")
			return
		}
		log.Printf("updateProperty: database update error: %v", err)
		writeErr(w, http.StatusInternalServerError, "internal error")
		return
	}
	writeJSON(w, http.StatusOK, p)
}

// deleteProperty removes a property. Its tenants, units, loans, underwriting models and ledger
// transactions are removed with it by the ON DELETE CASCADE foreign keys, in the same statement.
func (s *Server) deleteProperty(w http.ResponseWriter, r *http.Request) {
	propID, ok := parsePathID(r, "id")
	if !ok {
		writeErr(w, http.StatusBadRequest, "invalid property id")
		return
	}
	if s.DB == nil {
		writeErr(w, http.StatusInternalServerError, "database not configured")
		return
	}
	if _, ok := s.authorizeProperty(w, r, "deleteProperty", propID); !ok {
		return
	}
	if _, err := s.DB.Exec(r.Context(), `DELETE FROM properties WHERE id = $1`, propID); err != nil {
		log.Printf("deleteProperty: database delete error: %v", err)
		writeErr(w, http.StatusInternalServerError, "internal error")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
