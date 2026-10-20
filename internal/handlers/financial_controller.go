package handlers

import (
	"log"
	"net/http"
	"slices"
	"strconv"
	"strings"

	"github.com/CodyNicholson/Rixus_Real_Estate_BE_Project/internal/auth"
	"github.com/jackc/pgx/v5"
)

var propertyCategories = []string{"SINGLE_FAMILY", "CONDO_TOWNHOME", "MULTI_FAMILY_2_4", "MULTI_FAMILY_5_PLUS", "MIXED_USE"}

// FinancialMetrics is the shared shape of one property's analysis (v_property_financial_analysis)
// and of a weighted rollup over many properties (fn_portfolio_summary). Ratios are null when
// their denominator is zero, e.g. dscr on a property with no debt.
type FinancialMetrics struct {
	ScopeID         string  `json:"scope_id" db:"-"`
	ScopeName       string  `json:"scope_name" db:"-"`
	PropertyID      *int64  `json:"property_id,omitempty" db:"property_id"`
	PropertyName    *string `json:"property_name,omitempty" db:"property_name"`
	UserID          *int64  `json:"-" db:"user_id"`
	LegalEntityID   *int64  `json:"legal_entity_id,omitempty" db:"legal_entity_id"`
	LegalEntityName *string `json:"legal_entity_name,omitempty" db:"legal_entity_name"`
	Category        *string `json:"category,omitempty" db:"category"`
	StreetAddress   *string `json:"street_address,omitempty" db:"street_address"`
	City            *string `json:"city,omitempty" db:"city"`
	State           *string `json:"state,omitempty" db:"state"`
	PropertyCount   int64   `json:"property_count" db:"property_count"`
	TotalUnits      int64   `json:"total_units" db:"total_units"`
	OccupiedUnits   int64   `json:"occupied_units" db:"occupied_units"`
	RentableSqFt    int64   `json:"rentable_sq_ft" db:"rentable_sq_ft"`

	// Capital stack and valuation
	PurchasePrice      float64  `json:"purchase_price" db:"purchase_price"`
	AllInCostBasis     float64  `json:"all_in_cost_basis" db:"all_in_cost_basis"`
	CurrentMarketValue float64  `json:"current_market_value" db:"current_market_value"`
	TotalDebt          float64  `json:"total_debt" db:"total_debt"`
	TotalEquity        float64  `json:"total_equity" db:"total_equity"`
	TotalCashInvested  float64  `json:"total_cash_invested" db:"total_cash_invested"`
	PricePerDoor       *float64 `json:"price_per_door" db:"price_per_door"`
	PricePerSqFt       *float64 `json:"price_per_sq_ft" db:"price_per_sq_ft"`

	// Annual cash flow waterfall
	GPRMarket               float64 `json:"gpr_market" db:"gpr_market"`
	GPRInPlace              float64 `json:"gpr_in_place" db:"gpr_in_place"`
	LossToLease             float64 `json:"loss_to_lease" db:"loss_to_lease"`
	VacancyLoss             float64 `json:"vacancy_loss" db:"vacancy_loss"`
	CreditLoss              float64 `json:"credit_loss" db:"credit_loss"`
	NonRevenueLoss          float64 `json:"non_revenue_loss" db:"non_revenue_loss"`
	VacancyAndCreditLoss    float64 `json:"vacancy_and_credit_loss" db:"vacancy_and_credit_loss"`
	OtherIncome             float64 `json:"other_income" db:"other_income"`
	EffectiveGrossIncome    float64 `json:"effective_gross_income" db:"effective_gross_income"`
	AnnualPropertyTaxes     float64 `json:"annual_property_taxes" db:"annual_property_taxes"`
	AnnualInsurance         float64 `json:"annual_insurance" db:"annual_insurance"`
	AnnualHOA               float64 `json:"annual_hoa" db:"annual_hoa"`
	AnnualUtilities         float64 `json:"annual_utilities" db:"annual_utilities"`
	AnnualCAM               float64 `json:"annual_cam" db:"annual_cam"`
	AnnualAdminLegalCPA     float64 `json:"annual_admin_legal_cpa" db:"annual_admin_legal_cpa"`
	AnnualManagementFee     float64 `json:"annual_management_fee" db:"annual_management_fee"`
	AnnualRepairsMaint      float64 `json:"annual_repairs_maint" db:"annual_repairs_maint"`
	TotalOperatingExpenses  float64 `json:"total_operating_expenses" db:"total_operating_expenses"`
	NOI                     float64 `json:"noi" db:"noi"`
	AnnualCapexReserve      float64 `json:"annual_capex_reserve" db:"annual_capex_reserve"`
	UnleveredCashFlow       float64 `json:"unlevered_cash_flow" db:"unlevered_cash_flow"`
	AnnualDebtService       float64 `json:"annual_debt_service" db:"annual_debt_service"`
	LeveredCashFlowPreTax   float64 `json:"levered_cash_flow_pre_tax" db:"levered_cash_flow_pre_tax"`
	MonthlyCashFlow         float64 `json:"monthly_cash_flow" db:"monthly_cash_flow"`
	AnnualInterestExpense   float64 `json:"annual_interest_expense" db:"annual_interest_expense"`
	AnnualDepreciation      float64 `json:"annual_depreciation" db:"annual_depreciation"`
	TaxableNetIncome        float64 `json:"taxable_net_income" db:"taxable_net_income"`
	LeveredCashFlowAfterTax float64 `json:"levered_cash_flow_after_tax" db:"levered_cash_flow_after_tax"`

	// Return and risk ratios
	CapRatePct            *float64 `json:"cap_rate_pct" db:"cap_rate_pct"`
	YieldOnCostPct        *float64 `json:"yield_on_cost_pct" db:"yield_on_cost_pct"`
	CashOnCashPct         *float64 `json:"cash_on_cash_pct" db:"cash_on_cash_pct"`
	DSCR                  *float64 `json:"dscr" db:"dscr"`
	DebtYieldPct          *float64 `json:"debt_yield_pct" db:"debt_yield_pct"`
	LTVPct                *float64 `json:"ltv_pct" db:"ltv_pct"`
	OpexRatioPct          *float64 `json:"opex_ratio_pct" db:"opex_ratio_pct"`
	BreakEvenOccupancyPct *float64 `json:"break_even_occupancy_pct" db:"break_even_occupancy_pct"`
	GrossRentMultiplier   *float64 `json:"gross_rent_multiplier" db:"gross_rent_multiplier"`
}

type LegalEntity struct {
	ID               int64   `json:"id" db:"id"`
	Name             string  `json:"name" db:"name"`
	EntityType       string  `json:"entity_type" db:"entity_type"`
	BankAccountLast4 *string `json:"bank_account_last4" db:"bank_account_last4"`
	CreditCardLast4  *string `json:"credit_card_last4" db:"credit_card_last4"`
}

type FinancialDashboard struct {
	Summary       FinancialMetrics   `json:"summary"`
	Properties    []FinancialMetrics `json:"properties"`
	LegalEntities []LegalEntity      `json:"legal_entities"`
}

// financialFilter narrows the dashboard to a subset of the target user's properties; empty fields match everything.
type financialFilter struct {
	Categories    []string
	LegalEntityID *int64
	PropertyIDs   []int64
}

// registerFinancialRoutes registers endpoints on the financial controller.
func (s *Server) registerFinancialRoutes(mux *http.ServeMux, authMW func(http.Handler) http.Handler) {
	mux.Handle("GET /api/financials/dashboard", authMW(http.HandlerFunc(s.financialDashboard)))
}

func parsePositiveInt(v string) (int64, bool) {
	n, err := strconv.ParseInt(strings.TrimSpace(v), 10, 64)
	return n, err == nil && n > 0
}

// parseFinancialFilter reads ?category=A,B&legal_entity_id=N&property_id=1,2 and returns an error message on bad input.
func parseFinancialFilter(r *http.Request) (financialFilter, string) {
	var f financialFilter
	q := r.URL.Query()
	if v := q.Get("category"); v != "" {
		for _, c := range strings.Split(v, ",") {
			c = strings.ToUpper(strings.TrimSpace(c))
			if !slices.Contains(propertyCategories, c) {
				return f, "category must be one of " + strings.Join(propertyCategories, ", ")
			}
			f.Categories = append(f.Categories, c)
		}
	}
	if v := q.Get("legal_entity_id"); v != "" {
		id, ok := parsePositiveInt(v)
		if !ok {
			return f, "invalid legal_entity_id"
		}
		f.LegalEntityID = &id
	}
	if v := q.Get("property_id"); v != "" {
		for _, p := range strings.Split(v, ",") {
			id, ok := parsePositiveInt(p)
			if !ok {
				return f, "invalid property_id"
			}
			f.PropertyIDs = append(f.PropertyIDs, id)
		}
	}
	return f, ""
}

// financialDashboard returns the weighted summary plus per-property metrics for the caller's
// properties (or ?user_id's, for ADMINs), optionally filtered by category, legal entity or property.
func (s *Server) financialDashboard(w http.ResponseWriter, r *http.Request) {
	callerID := auth.UserID(r.Context())
	callerRole := auth.UserRole(r.Context())
	targetUserID := callerID
	if uidStr := r.URL.Query().Get("user_id"); uidStr != "" {
		uid, ok := parsePositiveInt(uidStr)
		if !ok {
			writeErr(w, http.StatusBadRequest, "invalid user id")
			return
		}
		if callerID != uid && callerRole != "ADMIN" {
			log.Printf("financialDashboard: forbidden - caller %d (role %s) cannot access financials for user %d", callerID, callerRole, uid)
			writeErr(w, http.StatusForbidden, "forbidden")
			return
		}
		targetUserID = uid
	}
	f, msg := parseFinancialFilter(r)
	if msg != "" {
		writeErr(w, http.StatusBadRequest, msg)
		return
	}
	if s.DB == nil {
		writeErr(w, http.StatusInternalServerError, "database not configured")
		return
	}
	ctx := r.Context()

	rows, err := s.DB.Query(ctx,
		`SELECT * FROM v_property_financial_analysis
		 WHERE user_id = $1
		   AND ($2::text[] IS NULL OR category::text = ANY($2::text[]))
		   AND ($3::bigint IS NULL OR legal_entity_id = $3)
		   AND ($4::bigint[] IS NULL OR property_id = ANY($4))
		 ORDER BY property_name, property_id`,
		targetUserID, f.Categories, f.LegalEntityID, f.PropertyIDs)
	if err != nil {
		log.Printf("financialDashboard: property query error: %v", err)
		writeErr(w, http.StatusInternalServerError, "internal error")
		return
	}
	props, err := pgx.CollectRows(rows, pgx.RowToStructByNameLax[FinancialMetrics])
	if err != nil {
		log.Printf("financialDashboard: property scan error: %v", err)
		writeErr(w, http.StatusInternalServerError, "internal error")
		return
	}
	for i := range props {
		props[i].ScopeID = strconv.FormatInt(*props[i].PropertyID, 10)
		props[i].ScopeName = *props[i].PropertyName
		props[i].PropertyCount = 1
	}

	rows, err = s.DB.Query(ctx,
		`SELECT * FROM fn_portfolio_summary($1, $2::text[]::property_category[], $3, $4)`,
		targetUserID, f.Categories, f.LegalEntityID, f.PropertyIDs)
	if err != nil {
		log.Printf("financialDashboard: summary query error: %v", err)
		writeErr(w, http.StatusInternalServerError, "internal error")
		return
	}
	summary, err := pgx.CollectExactlyOneRow(rows, pgx.RowToStructByNameLax[FinancialMetrics])
	if err != nil {
		log.Printf("financialDashboard: summary scan error: %v", err)
		writeErr(w, http.StatusInternalServerError, "internal error")
		return
	}

	rows, err = s.DB.Query(ctx,
		`SELECT id, name, entity_type, bank_account_last4, credit_card_last4 FROM legal_entities WHERE user_id = $1 ORDER BY name, id`,
		targetUserID)
	if err != nil {
		log.Printf("financialDashboard: legal entity query error: %v", err)
		writeErr(w, http.StatusInternalServerError, "internal error")
		return
	}
	entities, err := pgx.CollectRows(rows, pgx.RowToStructByName[LegalEntity])
	if err != nil {
		log.Printf("financialDashboard: legal entity scan error: %v", err)
		writeErr(w, http.StatusInternalServerError, "internal error")
		return
	}

	summary.ScopeID, summary.ScopeName = "PORTFOLIO", "Entire Portfolio"
	switch {
	case len(f.PropertyIDs) == 1 && len(props) == 1:
		summary.ScopeID, summary.ScopeName = props[0].ScopeID, props[0].ScopeName
	case f.LegalEntityID != nil:
		summary.ScopeID = strconv.FormatInt(*f.LegalEntityID, 10)
		summary.ScopeName = "Legal Entity #" + summary.ScopeID
		for _, e := range entities {
			if e.ID == *f.LegalEntityID {
				summary.ScopeName = e.Name
			}
		}
	case len(f.Categories) > 0:
		summary.ScopeID, summary.ScopeName = "CATEGORY", strings.Join(f.Categories, ", ")
	}

	writeJSON(w, http.StatusOK, FinancialDashboard{Summary: summary, Properties: props, LegalEntities: entities})
}
