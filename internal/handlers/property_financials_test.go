package handlers

import (
	"context"
	"encoding/json"
	"math"
	"net/http"
	"net/http/httptest"
	"os"
	"strconv"
	"strings"
	"testing"

	"github.com/CodyNicholson/Rixus_Real_Estate_BE_Project/internal/auth"
	"github.com/CodyNicholson/Rixus_Real_Estate_BE_Project/internal/db"
)

func TestPropertyFinancialEndpointsAuthAndValidation(t *testing.T) {
	secret := "0123456789abcdef0123456789abcdef"
	handler := (&Server{JWTSecret: secret, CORSOrigin: "http://localhost:3000"}).Routes()
	token, err := auth.NewAccessToken(secret, 5, "viewer5@example.com", "Viewer 5", "VIEWER")
	if err != nil {
		t.Fatalf("failed to create token: %v", err)
	}
	do := func(method, path, body, tok string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(method, path, strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		if tok != "" {
			req.Header.Set("Authorization", "Bearer "+tok)
		}
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		return rec
	}

	validUnit := `{"unit_number": "1A", "rentable_sq_ft": 800, "market_rent": 1500, "in_place_rent": 1450}`
	validLoan := `{"original_loan_amount": 100000, "current_principal_balance": 95000, "interest_rate": 0.065}`
	validLedger := `{"transaction_date": "2026-01-01T00:00:00Z", "category": "gross_rent", "amount": 1500}`

	// 1. Every new endpoint requires a token
	for _, c := range [][2]string{
		{http.MethodPut, "/api/properties/1"}, {http.MethodDelete, "/api/properties/1"},
		{http.MethodPost, "/api/properties/1/units"}, {http.MethodPut, "/api/properties/1/units/2"}, {http.MethodDelete, "/api/properties/1/units/2"},
		{http.MethodGet, "/api/properties/1/loans"}, {http.MethodPost, "/api/properties/1/loans"},
		{http.MethodPut, "/api/properties/1/loans/2"}, {http.MethodDelete, "/api/properties/1/loans/2"},
		{http.MethodGet, "/api/properties/1/underwriting"}, {http.MethodPut, "/api/properties/1/underwriting"},
		{http.MethodGet, "/api/properties/1/ledger"}, {http.MethodPost, "/api/properties/1/ledger"},
		{http.MethodPut, "/api/properties/1/ledger/2"}, {http.MethodDelete, "/api/properties/1/ledger/2"},
	} {
		if rec := do(c[0], c[1], "{}", ""); rec.Code != http.StatusUnauthorized {
			t.Errorf("expected 401 for %s %s without token, got %d", c[0], c[1], rec.Code)
		}
	}

	// 2. Bad ids and bad bodies are rejected before touching the database; valid input then fails on the missing database
	cases := []struct {
		method, path, body string
		want               int
	}{
		{http.MethodDelete, "/api/properties/abc", "", http.StatusBadRequest},
		{http.MethodDelete, "/api/properties/7", "", http.StatusInternalServerError},
		{http.MethodPut, "/api/properties/7", `{"street_address": ""}`, http.StatusBadRequest},
		{http.MethodPut, "/api/properties/7", `{"street_address": "1 Main", "category": "CASTLE"}`, http.StatusBadRequest},
		{http.MethodPut, "/api/properties/7", `{"street_address": "1 Main", "land_value_allocation_pct": 1.5}`, http.StatusBadRequest},
		{http.MethodPut, "/api/properties/7", `{"street_address": "1 Main", "purchase_price": -1}`, http.StatusBadRequest},
		{http.MethodPut, "/api/properties/7", `{"street_address": "1 Main", "utility_config": {"cable": "TENANT"}}`, http.StatusBadRequest},
		{http.MethodPut, "/api/properties/7", `{"street_address": "1 Main", "depreciation_schedule_years": 0}`, http.StatusBadRequest},
		{http.MethodPut, "/api/properties/abc", `{"street_address": "1 Main"}`, http.StatusBadRequest},
		{http.MethodPut, "/api/properties/7", `{"street_address": "1 Main", "category": "condo_townhome", "purchase_price": 250000}`, http.StatusInternalServerError},
		{http.MethodPost, "/api/properties/7/units", `{"unit_number": "1A"}`, http.StatusBadRequest},
		{http.MethodPost, "/api/properties/7/units", strings.Replace(validUnit, "800", "-5", 1), http.StatusBadRequest},
		{http.MethodPost, "/api/properties/7/units", strings.Replace(validUnit, `"1A"`, `"1A", "status": "HAUNTED"`, 1), http.StatusBadRequest},
		{http.MethodPost, "/api/properties/7/units", strings.Replace(validUnit, `"1A"`, `"1A", "lease_start_date": "2026-02-01T00:00:00Z", "lease_end_date": "2026-01-01T00:00:00Z"`, 1), http.StatusBadRequest},
		{http.MethodPost, "/api/properties/abc/units", validUnit, http.StatusBadRequest},
		{http.MethodPost, "/api/properties/7/units", validUnit, http.StatusInternalServerError},
		{http.MethodPost, "/api/properties/7/loans", `{"original_loan_amount": 100000}`, http.StatusBadRequest},
		{http.MethodPost, "/api/properties/7/loans", strings.Replace(validLoan, "0.065", "6.5", 1), http.StatusBadRequest},
		{http.MethodPost, "/api/properties/7/loans", strings.Replace(validLoan, "}", `, "amortization_months": 0}`, 1), http.StatusBadRequest},
		{http.MethodPost, "/api/properties/7/loans", validLoan, http.StatusInternalServerError},
		{http.MethodPut, "/api/properties/7/underwriting", `{"physical_vacancy_pct": 5}`, http.StatusBadRequest},
		{http.MethodPut, "/api/properties/7/underwriting", `{"annual_property_taxes": -10}`, http.StatusBadRequest},
		{http.MethodPut, "/api/properties/7/underwriting", `{"annual_rent_growth_pct": -0.02}`, http.StatusInternalServerError},
		{http.MethodGet, "/api/properties/7/ledger?from=01/01/2026", "", http.StatusBadRequest},
		{http.MethodPost, "/api/properties/7/ledger", `{"category": "GROSS_RENT", "amount": 10}`, http.StatusBadRequest},
		{http.MethodPost, "/api/properties/7/ledger", strings.Replace(validLedger, "gross_rent", "LOTTERY", 1), http.StatusBadRequest},
		{http.MethodPost, "/api/properties/7/ledger", validLedger, http.StatusInternalServerError},
		{http.MethodPut, "/api/properties/7/ledger/2", validLedger, http.StatusInternalServerError},
	}
	for _, c := range cases {
		if rec := do(c.method, c.path, c.body, token); rec.Code != c.want {
			t.Errorf("expected %d for %s %s %s, got %d. Body: %s", c.want, c.method, c.path, c.body, rec.Code, rec.Body.String())
		}
	}
}

func TestPropertyFinancialsDatabase(t *testing.T) {
	url := os.Getenv("TEST_DATABASE_URL")
	if url == "" {
		t.Skip("TEST_DATABASE_URL not set")
	}
	ctx := context.Background()
	pool, err := db.Connect(ctx, url)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	defer pool.Close()

	var userID, otherID, entityID, otherEntityID int64
	mustScan := func(dest *int64, sql string, args ...any) {
		t.Helper()
		if err := pool.QueryRow(ctx, sql, args...).Scan(dest); err != nil {
			t.Fatalf("%s: %v", sql, err)
		}
	}
	mustScan(&userID, `INSERT INTO users (email, name) VALUES ('pf-'||gen_random_uuid()||'@example.com', 'PF') RETURNING id`)
	mustScan(&otherID, `INSERT INTO users (email, name) VALUES ('pf-'||gen_random_uuid()||'@example.com', 'Other') RETURNING id`)
	t.Cleanup(func() { _, _ = pool.Exec(ctx, `DELETE FROM users WHERE id = ANY($1)`, []int64{userID, otherID}) })
	mustScan(&entityID, `INSERT INTO legal_entities (user_id, name, entity_type) VALUES ($1, 'PF LLC', 'HOLDING_LLC') RETURNING id`, userID)
	mustScan(&otherEntityID, `INSERT INTO legal_entities (user_id, name, entity_type) VALUES ($1, 'Other LLC', 'HOLDING_LLC') RETURNING id`, otherID)

	secret := "0123456789abcdef0123456789abcdef"
	handler := (&Server{DB: pool, JWTSecret: secret, CORSOrigin: "http://localhost:3000"}).Routes()
	token, _ := auth.NewAccessToken(secret, userID, "pf@example.com", "PF", "VIEWER")
	otherToken, _ := auth.NewAccessToken(secret, otherID, "other@example.com", "Other", "VIEWER")
	call := func(tok, method, path, body string, want int, out any) {
		t.Helper()
		req := httptest.NewRequest(method, path, strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+tok)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		if rec.Code != want {
			t.Fatalf("%s %s: expected %d, got %d. Body: %s", method, path, want, rec.Code, rec.Body.String())
		}
		if out != nil {
			if err := json.Unmarshal(rec.Body.Bytes(), out); err != nil {
				t.Fatalf("%s %s: bad json: %v", method, path, err)
			}
		}
	}

	// Create with only an address, then fill in the acquisition basis
	var prop Property
	call(token, http.MethodPost, "/api/properties", `{"street_address": "9 Ledger Ln", "units": 2}`, http.StatusCreated, &prop)
	if prop.Category != "SINGLE_FAMILY" || prop.LandValueAllocationPct != 0.2 || !prop.IsActive || prop.UtilityConfig["water"] != "LANDLORD" {
		t.Fatalf("unexpected defaults on create: %+v", prop)
	}
	base := "/api/properties/" + strconv.FormatInt(prop.ID, 10)
	update := `{"street_address": "9 Ledger Ln", "city": "Chicago", "units": 2, "name": "Ledger Duplex",
		"category": "MULTI_FAMILY_2_4", "legal_entity_id": ` + strconv.FormatInt(entityID, 10) + `, "year_built": 1910,
		"acquisition_date": "2024-05-01T00:00:00Z", "purchase_price": 400000, "closing_costs": 8000,
		"after_repair_value": 450000, "utility_config": {"water": "tenant"}}`
	call(token, http.MethodPut, base, update, http.StatusOK, &prop)
	if prop.Name != "Ledger Duplex" || prop.LegalEntityName == nil || *prop.LegalEntityName != "PF LLC" ||
		prop.PurchasePrice != 400000 || prop.UtilityConfig["water"] != "TENANT" || prop.UtilityConfig["trash"] != "LANDLORD" {
		t.Fatalf("unexpected property after update: %+v", prop)
	}
	call(token, http.MethodPut, base, strings.Replace(update, `"legal_entity_id": `+strconv.FormatInt(entityID, 10),
		`"legal_entity_id": `+strconv.FormatInt(otherEntityID, 10), 1), http.StatusBadRequest, nil)
	call(otherToken, http.MethodPut, base, update, http.StatusForbidden, nil)

	// Rent roll
	var unitA, unitB Unit
	call(token, http.MethodPost, base+"/units", `{"unit_number": "1", "rentable_sq_ft": 900, "market_rent": 2000, "in_place_rent": 1900}`, http.StatusCreated, &unitA)
	call(token, http.MethodPost, base+"/units", `{"unit_number": "2", "rentable_sq_ft": 900, "market_rent": 2000, "in_place_rent": 0, "status": "vacant_ready"}`, http.StatusCreated, &unitB)
	call(token, http.MethodPost, base+"/units", `{"unit_number": "2", "rentable_sq_ft": 900, "market_rent": 2000, "in_place_rent": 0}`, http.StatusConflict, nil)
	if unitA.Bedrooms != 1 || unitB.Status != "VACANT_READY" {
		t.Fatalf("unexpected units %+v %+v", unitA, unitB)
	}
	unitBPath := base + "/units/" + strconv.FormatInt(unitB.ID, 10)
	call(token, http.MethodPut, unitBPath, `{"unit_number": "2", "bedrooms": 2, "rentable_sq_ft": 950, "market_rent": 2100, "in_place_rent": 2050}`, http.StatusOK, &unitB)
	if unitB.Status != "OCCUPIED" || unitB.Bedrooms != 2 || unitB.InPlaceRent != 2050 {
		t.Fatalf("unexpected unit after update %+v", unitB)
	}
	call(otherToken, http.MethodGet, base+"/units", "", http.StatusForbidden, nil)

	// Debt: $300k at 6% over 30 years => $1,798.65/mo plus $50 PMI
	var loan Loan
	call(token, http.MethodPost, base+"/loans", `{"lender_name": "Test Bank", "original_loan_amount": 300000,
		"current_principal_balance": 300000, "interest_rate": 0.06, "monthly_pmi": 50}`, http.StatusCreated, &loan)
	if loan.LoanType != "CONVENTIONAL" || loan.AmortizationMonths != 360 || math.Abs(loan.MonthlyPayment-1848.65) > 0.01 {
		t.Fatalf("unexpected loan %+v", loan)
	}
	var loans []Loan
	call(token, http.MethodGet, base+"/loans", "", http.StatusOK, &loans)
	if len(loans) != 1 {
		t.Fatalf("expected 1 loan, got %d", len(loans))
	}

	// Underwriting: defaults until saved, then an upsert of the single primary model
	var uw UnderwritingModel
	call(token, http.MethodGet, base+"/underwriting", "", http.StatusOK, &uw)
	if uw.ID != nil || uw.PhysicalVacancyPct != 0.05 || uw.MarginalIncomeTaxPct != 0.32 {
		t.Fatalf("unexpected default underwriting %+v", uw)
	}
	call(token, http.MethodPut, base+"/underwriting", `{"annual_property_taxes": 7000, "annual_insurance": 1800}`, http.StatusOK, &uw)
	firstID := uw.ID
	call(token, http.MethodPut, base+"/underwriting", `{"annual_property_taxes": 8000, "annual_insurance": 1800, "physical_vacancy_pct": 0.04}`, http.StatusOK, &uw)
	if uw.ID == nil || firstID == nil || *uw.ID != *firstID || uw.AnnualPropertyTaxes != 8000 || uw.PhysicalVacancyPct != 0.04 || uw.ManagementFeePctOfEGI != 0.08 {
		t.Fatalf("unexpected underwriting after upsert %+v", uw)
	}

	// Ledger
	var tx LedgerTransaction
	unitAID := strconv.FormatInt(unitA.ID, 10)
	call(token, http.MethodPost, base+"/ledger", `{"unit_id": `+unitAID+`, "transaction_date": "2026-03-01T00:00:00Z",
		"category": "GROSS_RENT", "amount": 1900, "vendor_or_payor": "Tenant 1"}`, http.StatusCreated, &tx)
	call(token, http.MethodPost, base+"/ledger", `{"transaction_date": "2026-04-02T00:00:00Z", "category": "PROPERTY_TAX", "amount": 4000}`, http.StatusCreated, nil)
	call(token, http.MethodPut, base+"/ledger/"+strconv.FormatInt(tx.ID, 10), `{"unit_id": `+unitAID+`, "transaction_date": "2026-03-01T00:00:00Z",
		"category": "GROSS_RENT", "amount": 1950}`, http.StatusOK, &tx)
	if tx.Amount != 1950 || tx.VendorOrPayor != nil || tx.UnitID == nil || *tx.UnitID != unitA.ID {
		t.Fatalf("unexpected transaction after update %+v", tx)
	}
	var txs []LedgerTransaction
	call(token, http.MethodGet, base+"/ledger?from=2026-04-01", "", http.StatusOK, &txs)
	if len(txs) != 1 || txs[0].Category != "PROPERTY_TAX" {
		t.Fatalf("unexpected filtered ledger %+v", txs)
	}

	// A unit from another property can't be tagged on this property's ledger
	var otherProp Property
	call(token, http.MethodPost, "/api/properties", `{"street_address": "10 Other St"}`, http.StatusCreated, &otherProp)
	var otherUnit Unit
	call(token, http.MethodPost, "/api/properties/"+strconv.FormatInt(otherProp.ID, 10)+"/units",
		`{"unit_number": "MAIN", "rentable_sq_ft": 1000, "market_rent": 1800, "in_place_rent": 1800}`, http.StatusCreated, &otherUnit)
	call(token, http.MethodPost, base+"/ledger", `{"unit_id": `+strconv.FormatInt(otherUnit.ID, 10)+`, "transaction_date": "2026-03-01T00:00:00Z",
		"category": "GROSS_RENT", "amount": 1}`, http.StatusBadRequest, nil)
	// Children can't be edited or deleted through another property's path
	call(token, http.MethodDelete, "/api/properties/"+strconv.FormatInt(otherProp.ID, 10)+"/units/"+unitAID, "", http.StatusNotFound, nil)

	// The analysis view picks up the edits
	var dash FinancialDashboard
	call(token, http.MethodGet, "/api/financials/dashboard?property_id="+strconv.FormatInt(prop.ID, 10), "", http.StatusOK, &dash)
	if m := dash.Summary; m.PurchasePrice != 400000 || m.AnnualPropertyTaxes != 8000 || m.GPRMarket != (2000+2100)*12 || m.OccupiedUnits != 2 {
		t.Fatalf("dashboard did not reflect edits: %+v", m)
	}

	// Tenants pin the unit count: it can't drop below the highest occupied unit
	if _, err := pool.Exec(ctx, `INSERT INTO tenants (tenant_initials, property_id, unit_number, monthly_rent) VALUES ('AB', $1, 2, 1900)`, prop.ID); err != nil {
		t.Fatalf("insert tenant: %v", err)
	}
	call(token, http.MethodPut, base, strings.Replace(update, `"units": 2`, `"units": 1`, 1), http.StatusBadRequest, nil)

	// Deleting the property removes every related row, but not the shared legal entity
	call(otherToken, http.MethodDelete, base, "", http.StatusForbidden, nil)
	call(token, http.MethodDelete, base, "", http.StatusNoContent, nil)
	call(token, http.MethodGet, base, "", http.StatusNotFound, nil)
	call(token, http.MethodGet, base+"/ledger", "", http.StatusNotFound, nil)
	for _, table := range []string{"units", "property_loans", "underwriting_models", "ledger_transactions", "tenants"} {
		var n int64
		mustScan(&n, `SELECT COUNT(*) FROM `+table+` WHERE property_id = $1`, prop.ID)
		if n != 0 {
			t.Errorf("expected no %s rows after delete, got %d", table, n)
		}
	}
	var entities int64
	mustScan(&entities, `SELECT COUNT(*) FROM legal_entities WHERE id = $1`, entityID)
	var otherUnits int64
	mustScan(&otherUnits, `SELECT COUNT(*) FROM units WHERE property_id = $1`, otherProp.ID)
	if entities != 1 || otherUnits != 1 {
		t.Errorf("delete touched unrelated rows: entities=%d otherUnits=%d", entities, otherUnits)
	}
}
