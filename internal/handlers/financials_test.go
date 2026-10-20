package handlers

import (
	"context"
	"encoding/json"
	"math"
	"net/http"
	"net/http/httptest"
	"os"
	"strconv"
	"testing"

	"github.com/CodyNicholson/Rixus_Real_Estate_BE_Project/internal/auth"
	"github.com/CodyNicholson/Rixus_Real_Estate_BE_Project/internal/db"
)

func TestFinancialEndpointsAuthAndValidation(t *testing.T) {
	secret := "0123456789abcdef0123456789abcdef"
	s := &Server{
		JWTSecret:  secret,
		CORSOrigin: "http://localhost:3000",
	}
	handler := s.Routes()

	viewerToken5, err := auth.NewAccessToken(secret, 5, "viewer5@example.com", "Viewer 5", "VIEWER")
	if err != nil {
		t.Fatalf("failed to create token: %v", err)
	}

	// 1. Every financial endpoint requires a token
	for _, p := range []string{"/api/financials/dashboard", "/api/properties/1/units"} {
		req := httptest.NewRequest(http.MethodGet, p, nil)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		if rec.Code != http.StatusUnauthorized {
			t.Fatalf("expected 401 for %s without token, got %d", p, rec.Code)
		}
	}

	// 2. Bad filters, ids and another user's dashboard are rejected before touching the database
	cases := map[string]int{
		"/api/financials/dashboard?category=CASTLE":         http.StatusBadRequest,
		"/api/financials/dashboard?legal_entity_id=abc":     http.StatusBadRequest,
		"/api/financials/dashboard?property_id=1,0":         http.StatusBadRequest,
		"/api/financials/dashboard?user_id=-1":              http.StatusBadRequest,
		"/api/financials/dashboard?user_id=10":              http.StatusForbidden,
		"/api/properties/abc/units":                         http.StatusBadRequest,
		"/api/properties/0/units":                           http.StatusBadRequest,
		"/api/financials/dashboard?category=single_family":  http.StatusInternalServerError, // valid, then no database
		"/api/financials/dashboard?user_id=5&property_id=3": http.StatusInternalServerError,
	}
	for p, want := range cases {
		req := httptest.NewRequest(http.MethodGet, p, nil)
		req.Header.Set("Authorization", "Bearer "+viewerToken5)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		if rec.Code != want {
			t.Errorf("expected %d for %s, got %d. Body: %s", want, p, rec.Code, rec.Body.String())
		}
	}
}

func TestParseFinancialFilter(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet,
		"/api/financials/dashboard?category=single_family,%20CONDO_TOWNHOME&legal_entity_id=7&property_id=3,4", nil)
	f, msg := parseFinancialFilter(req)
	if msg != "" {
		t.Fatalf("expected valid filter, got %q", msg)
	}
	if len(f.Categories) != 2 || f.Categories[0] != "SINGLE_FAMILY" || f.Categories[1] != "CONDO_TOWNHOME" {
		t.Fatalf("unexpected categories %v", f.Categories)
	}
	if f.LegalEntityID == nil || *f.LegalEntityID != 7 || len(f.PropertyIDs) != 2 || f.PropertyIDs[1] != 4 {
		t.Fatalf("unexpected filter %+v", f)
	}

	f, msg = parseFinancialFilter(httptest.NewRequest(http.MethodGet, "/api/financials/dashboard", nil))
	if msg != "" || f.Categories != nil || f.LegalEntityID != nil || f.PropertyIDs != nil {
		t.Fatalf("expected empty filter, got %+v %q", f, msg)
	}
}

// TestFinancialDashboardDatabase runs the schema and the dashboard against a real Postgres.
// It is skipped unless TEST_DATABASE_URL points at a disposable database.
func TestFinancialDashboardDatabase(t *testing.T) {
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

	var userID, entityID, sfrID, duplexID int64
	mustScan := func(dest *int64, sql string, args ...any) {
		t.Helper()
		if err := pool.QueryRow(ctx, sql, args...).Scan(dest); err != nil {
			t.Fatalf("%s: %v", sql, err)
		}
	}
	mustScan(&userID, `INSERT INTO users (email, name) VALUES ('fin-'||gen_random_uuid()||'@example.com', 'Fin') RETURNING id`)
	t.Cleanup(func() { _, _ = pool.Exec(ctx, `DELETE FROM users WHERE id = $1`, userID) })
	mustScan(&entityID, `INSERT INTO legal_entities (user_id, name, entity_type) VALUES ($1, 'Test LLC', 'HOLDING_LLC') RETURNING id`, userID)

	// SFR: $200k, 80% loan at 6% over 30 years, $2,000/mo market rent
	mustScan(&sfrID, `INSERT INTO properties (user_id, legal_entity_id, name, category, street_address, units, purchase_price, closing_costs)
		VALUES ($1, $2, 'Test SFR', 'SINGLE_FAMILY', '1 Test St', 1, 200000, 5000) RETURNING id`, userID, entityID)
	// Duplex with no debt and one vacant unit
	mustScan(&duplexID, `INSERT INTO properties (user_id, name, category, street_address, units, purchase_price)
		VALUES ($1, 'Test Duplex', 'MULTI_FAMILY_2_4', '2 Test St', 2, 300000) RETURNING id`, userID)
	for _, sql := range []string{
		`INSERT INTO units (property_id, unit_number, rentable_sq_ft, market_rent, in_place_rent) VALUES ($1, 'MAIN', 1000, 2000, 1900)`,
		`INSERT INTO property_loans (property_id, original_loan_amount, current_principal_balance, interest_rate) VALUES ($1, 160000, 160000, 0.06)`,
		`INSERT INTO underwriting_models (property_id, annual_property_taxes, annual_insurance) VALUES ($1, 3000, 1200)`,
	} {
		if _, err := pool.Exec(ctx, sql, sfrID); err != nil {
			t.Fatalf("%s: %v", sql, err)
		}
	}
	for _, sql := range []string{
		`INSERT INTO units (property_id, unit_number, rentable_sq_ft, market_rent, in_place_rent) VALUES ($1, '1', 900, 1500, 1500)`,
		`INSERT INTO units (property_id, unit_number, rentable_sq_ft, market_rent, in_place_rent, status) VALUES ($1, '2', 900, 1500, 0, 'VACANT_READY')`,
		`INSERT INTO underwriting_models (property_id, annual_property_taxes) VALUES ($1, 6000)`,
	} {
		if _, err := pool.Exec(ctx, sql, duplexID); err != nil {
			t.Fatalf("%s: %v", sql, err)
		}
	}

	secret := "0123456789abcdef0123456789abcdef"
	handler := (&Server{DB: pool, JWTSecret: secret, CORSOrigin: "http://localhost:3000"}).Routes()
	token, err := auth.NewAccessToken(secret, userID, "fin@example.com", "Fin", "VIEWER")
	if err != nil {
		t.Fatalf("token: %v", err)
	}
	get := func(path string, out any) {
		t.Helper()
		req := httptest.NewRequest(http.MethodGet, path, nil)
		req.Header.Set("Authorization", "Bearer "+token)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("GET %s: expected 200, got %d. Body: %s", path, rec.Code, rec.Body.String())
		}
		if err := json.Unmarshal(rec.Body.Bytes(), out); err != nil {
			t.Fatalf("GET %s: bad json: %v", path, err)
		}
	}
	near := func(name string, got, want float64) {
		t.Helper()
		if math.Abs(got-want) > 0.011 {
			t.Errorf("%s: got %.2f, want %.2f", name, got, want)
		}
	}

	var all FinancialDashboard
	get("/api/financials/dashboard", &all)
	if all.Summary.ScopeID != "PORTFOLIO" || all.Summary.PropertyCount != 2 || all.Summary.TotalUnits != 3 || len(all.Properties) != 2 {
		t.Fatalf("unexpected portfolio summary %+v", all.Summary)
	}
	if len(all.LegalEntities) != 1 || all.LegalEntities[0].Name != "Test LLC" {
		t.Fatalf("unexpected legal entities %+v", all.LegalEntities)
	}

	var sfr FinancialDashboard
	get("/api/financials/dashboard?property_id="+strconv.FormatInt(sfrID, 10), &sfr)
	m := sfr.Summary
	if m.ScopeName != "Test SFR" || len(sfr.Properties) != 1 || *sfr.Properties[0].Category != "SINGLE_FAMILY" {
		t.Fatalf("unexpected single-property scope %+v", m)
	}
	// GPR 24,000; vacancy 5% and credit 1% => EGI 22,560; mgmt 8% + repairs 5% of EGI = 2,932.80
	near("egi", m.EffectiveGrossIncome, 22560)
	near("opex", m.TotalOperatingExpenses, 3000+1200+2932.80)
	near("noi", m.NOI, 22560-7132.80)
	// 160k at 6%/30yr => $959.28/mo
	near("debt service", m.AnnualDebtService, 959.28*12)
	near("capex", m.AnnualCapexReserve, 1128)
	near("cash flow", m.LeveredCashFlowPreTax, 15427.20-1128-11511.36)
	near("cash invested", m.TotalCashInvested, 45000)
	near("loss to lease", m.LossToLease, 1200)
	if m.DSCR == nil || *m.DSCR != 1.34 || m.CapRatePct == nil || *m.CapRatePct != 7.71 {
		t.Errorf("unexpected ratios dscr=%v cap=%v", m.DSCR, m.CapRatePct)
	}

	// The single-property summary matches that property's own row to the penny
	if p := sfr.Properties[0]; p.NOI != m.NOI || p.LeveredCashFlowAfterTax != m.LeveredCashFlowAfterTax || *p.CashOnCashPct != *m.CashOnCashPct {
		t.Errorf("property row and summary disagree: %+v vs %+v", p, m)
	}

	// Weighted portfolio ratios use summed dollars, not averages of property ratios
	duplex := all.Properties[0]
	if duplex.PropertyName == nil || *duplex.PropertyName != "Test Duplex" {
		duplex = all.Properties[1]
	}
	if duplex.DSCR != nil || duplex.OccupiedUnits != 1 {
		t.Errorf("expected debt-free duplex to have null dscr and 1 occupied unit, got %v %d", duplex.DSCR, duplex.OccupiedUnits)
	}
	wantCap := math.Round((m.NOI+duplex.NOI)/500000*100*100) / 100
	if all.Summary.CapRatePct == nil || *all.Summary.CapRatePct != wantCap {
		t.Errorf("weighted cap rate: got %v, want %.2f", all.Summary.CapRatePct, wantCap)
	}

	var multi FinancialDashboard
	get("/api/financials/dashboard?category=MULTI_FAMILY_2_4", &multi)
	if len(multi.Properties) != 1 || multi.Summary.NOI != duplex.NOI || multi.Summary.ScopeID != "CATEGORY" {
		t.Errorf("category filter: unexpected %+v", multi.Summary)
	}
	var entity FinancialDashboard
	get("/api/financials/dashboard?legal_entity_id="+strconv.FormatInt(entityID, 10), &entity)
	if len(entity.Properties) != 1 || entity.Summary.ScopeName != "Test LLC" {
		t.Errorf("entity filter: unexpected %+v", entity.Summary)
	}

	var units []Unit
	get("/api/properties/"+strconv.FormatInt(duplexID, 10)+"/units", &units)
	if len(units) != 2 || units[1].Status != "VACANT_READY" || units[0].MarketRent != 1500 {
		t.Errorf("unexpected units %+v", units)
	}
}
