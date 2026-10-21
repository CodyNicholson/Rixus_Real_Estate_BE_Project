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

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

// maxMonthlyAmount is the largest value NUMERIC(10,2) can hold.
const maxMonthlyAmount = 99999999.99

var (
	unitStatuses     = []string{"OCCUPIED", "VACANT_READY", "VACANT_RENOVATION", "NON_REVENUE_MODEL"}
	ledgerCategories = []string{
		"GROSS_RENT", "RUBS_REIMBURSEMENT", "PET_RENT", "PARKING_STORAGE", "LAUNDRY_VENDING", "OTHER_INCOME", "CONCESSIONS_BAD_DEBT",
		"PROPERTY_TAX", "INSURANCE", "HOA_DUES", "PROPERTY_MANAGEMENT", "LEASING_FEES",
		"UTILITIES_WATER_SEWER", "UTILITIES_ELECTRIC_GAS", "UTILITIES_TRASH",
		"CAM_LAWN_SNOW_PEST", "REPAIRS_MAINTENANCE", "TURNOVER_MAKE_READY", "ADMIN_LEGAL_CPA",
		"CAPEX_REPLACEMENT", "INITIAL_REHAB_BASIS", "DEBT_PRINCIPAL", "DEBT_INTEREST",
	}
)

// Unit is one row of a property's rent roll.
type Unit struct {
	ID                    int64      `json:"id" db:"id"`
	PropertyID            int64      `json:"property_id" db:"property_id"`
	UnitNumber            string     `json:"unit_number" db:"unit_number"`
	FloorPlanName         *string    `json:"floor_plan_name" db:"floor_plan_name"`
	Bedrooms              float64    `json:"bedrooms" db:"bedrooms"`
	Bathrooms             float64    `json:"bathrooms" db:"bathrooms"`
	RentableSqFt          int        `json:"rentable_sq_ft" db:"rentable_sq_ft"`
	Status                string     `json:"status" db:"status"`
	IsRenovated           bool       `json:"is_renovated" db:"is_renovated"`
	MarketRent            float64    `json:"market_rent" db:"market_rent"`
	InPlaceRent           float64    `json:"in_place_rent" db:"in_place_rent"`
	MonthlyRUBSCharge     float64    `json:"monthly_rubs_charge" db:"monthly_rubs_charge"`
	MonthlyPetRent        float64    `json:"monthly_pet_rent" db:"monthly_pet_rent"`
	MonthlyParkingStorage float64    `json:"monthly_parking_storage" db:"monthly_parking_storage"`
	SecurityDeposit       float64    `json:"security_deposit" db:"security_deposit"`
	LeaseStartDate        *time.Time `json:"lease_start_date" db:"lease_start_date"`
	LeaseEndDate          *time.Time `json:"lease_end_date" db:"lease_end_date"`
}

// unitInput is the request body for creating or replacing a unit.
type unitInput struct {
	UnitNumber            string     `json:"unit_number"`
	FloorPlanName         *string    `json:"floor_plan_name"`
	Bedrooms              *float64   `json:"bedrooms"`
	Bathrooms             *float64   `json:"bathrooms"`
	RentableSqFt          *int       `json:"rentable_sq_ft"`
	Status                string     `json:"status"`
	IsRenovated           bool       `json:"is_renovated"`
	MarketRent            *float64   `json:"market_rent"`
	InPlaceRent           *float64   `json:"in_place_rent"`
	MonthlyRUBSCharge     float64    `json:"monthly_rubs_charge"`
	MonthlyPetRent        float64    `json:"monthly_pet_rent"`
	MonthlyParkingStorage float64    `json:"monthly_parking_storage"`
	SecurityDeposit       float64    `json:"security_deposit"`
	LeaseStartDate        *time.Time `json:"lease_start_date"`
	LeaseEndDate          *time.Time `json:"lease_end_date"`
}

// Loan is a debt facility on a property. MonthlyPayment is the scheduled payment the analysis uses
// (principal and interest, or interest only during an IO period) plus PMI.
type Loan struct {
	ID                      int64   `json:"id" db:"id"`
	PropertyID              int64   `json:"property_id" db:"property_id"`
	LenderName              *string `json:"lender_name" db:"lender_name"`
	LoanType                string  `json:"loan_type" db:"loan_type"`
	OriginalLoanAmount      float64 `json:"original_loan_amount" db:"original_loan_amount"`
	CurrentPrincipalBalance float64 `json:"current_principal_balance" db:"current_principal_balance"`
	InterestRate            float64 `json:"interest_rate" db:"interest_rate"`
	AmortizationMonths      int     `json:"amortization_months" db:"amortization_months"`
	LoanTermMonths          int     `json:"loan_term_months" db:"loan_term_months"`
	InterestOnlyMonths      int     `json:"interest_only_months" db:"interest_only_months"`
	LoanOriginationFees     float64 `json:"loan_origination_fees" db:"loan_origination_fees"`
	MonthlyPMI              float64 `json:"monthly_pmi" db:"monthly_pmi"`
	IsActive                bool    `json:"is_active" db:"is_active"`
	MonthlyPayment          float64 `json:"monthly_payment" db:"monthly_payment"`
}

// loanInput is the request body for creating or replacing a loan.
type loanInput struct {
	LenderName              *string  `json:"lender_name"`
	LoanType                string   `json:"loan_type"`
	OriginalLoanAmount      *float64 `json:"original_loan_amount"`
	CurrentPrincipalBalance *float64 `json:"current_principal_balance"`
	InterestRate            *float64 `json:"interest_rate"`
	AmortizationMonths      *int     `json:"amortization_months"`
	LoanTermMonths          *int     `json:"loan_term_months"`
	InterestOnlyMonths      int      `json:"interest_only_months"`
	LoanOriginationFees     float64  `json:"loan_origination_fees"`
	MonthlyPMI              float64  `json:"monthly_pmi"`
	IsActive                *bool    `json:"is_active"`
}

// UnderwritingModel holds a property's pro forma operating assumptions. *_pct fields are fractions
// (0.05 = 5%). ID is null when the property has no saved model and the defaults apply.
type UnderwritingModel struct {
	ID                         *int64  `json:"id" db:"id"`
	PropertyID                 int64   `json:"property_id" db:"property_id"`
	ScenarioName               string  `json:"scenario_name" db:"scenario_name"`
	PhysicalVacancyPct         float64 `json:"physical_vacancy_pct" db:"physical_vacancy_pct"`
	CreditLossConcessionsPct   float64 `json:"credit_loss_concessions_pct" db:"credit_loss_concessions_pct"`
	MonthlyOtherPropertyIncome float64 `json:"monthly_other_property_income" db:"monthly_other_property_income"`
	AnnualPropertyTaxes        float64 `json:"annual_property_taxes" db:"annual_property_taxes"`
	AnnualInsurance            float64 `json:"annual_insurance" db:"annual_insurance"`
	MonthlyHOADues             float64 `json:"monthly_hoa_dues" db:"monthly_hoa_dues"`
	MonthlyLandlordUtilities   float64 `json:"monthly_landlord_utilities" db:"monthly_landlord_utilities"`
	MonthlyCAMLawnSnowPest     float64 `json:"monthly_cam_lawn_snow_pest" db:"monthly_cam_lawn_snow_pest"`
	AnnualAdminLegalCPA        float64 `json:"annual_admin_legal_cpa" db:"annual_admin_legal_cpa"`
	ManagementFeePctOfEGI      float64 `json:"management_fee_pct_of_egi" db:"management_fee_pct_of_egi"`
	RepairsMaintPctOfEGI       float64 `json:"repairs_maint_pct_of_egi" db:"repairs_maint_pct_of_egi"`
	AnnualRepairsPerUnit       float64 `json:"annual_repairs_per_unit" db:"annual_repairs_per_unit"`
	CapexReservePctOfEGI       float64 `json:"capex_reserve_pct_of_egi" db:"capex_reserve_pct_of_egi"`
	AnnualCapexPerUnit         float64 `json:"annual_capex_per_unit" db:"annual_capex_per_unit"`
	AnnualRentGrowthPct        float64 `json:"annual_rent_growth_pct" db:"annual_rent_growth_pct"`
	AnnualExpenseGrowthPct     float64 `json:"annual_expense_growth_pct" db:"annual_expense_growth_pct"`
	ExitCapRatePct             float64 `json:"exit_cap_rate_pct" db:"exit_cap_rate_pct"`
	SellingCostPct             float64 `json:"selling_cost_pct" db:"selling_cost_pct"`
	MarginalIncomeTaxPct       float64 `json:"marginal_income_tax_pct" db:"marginal_income_tax_pct"`
}

// LedgerTransaction is one actual income or expense entry. Amounts are positive; the category gives the direction.
type LedgerTransaction struct {
	ID              int64     `json:"id" db:"id"`
	PropertyID      int64     `json:"property_id" db:"property_id"`
	UnitID          *int64    `json:"unit_id" db:"unit_id"`
	TransactionDate time.Time `json:"transaction_date" db:"transaction_date"`
	Category        string    `json:"category" db:"category"`
	Amount          float64   `json:"amount" db:"amount"`
	VendorOrPayor   *string   `json:"vendor_or_payor" db:"vendor_or_payor"`
	Description     *string   `json:"description" db:"description"`
	CreatedAt       time.Time `json:"created_at" db:"created_at"`
}

// ledgerInput is the request body for creating or replacing a ledger transaction.
type ledgerInput struct {
	UnitID          *int64     `json:"unit_id"`
	TransactionDate *time.Time `json:"transaction_date"`
	Category        string     `json:"category"`
	Amount          *float64   `json:"amount"`
	VendorOrPayor   *string    `json:"vendor_or_payor"`
	Description     *string    `json:"description"`
}

const (
	unitColumns = `id, property_id, unit_number, floor_plan_name, bedrooms, bathrooms, rentable_sq_ft, status::text AS status,
		is_renovated, market_rent, in_place_rent, monthly_rubs_charge, monthly_pet_rent, monthly_parking_storage,
		security_deposit, lease_start_date, lease_end_date`
	loanColumns = `id, property_id, lender_name, loan_type, original_loan_amount, current_principal_balance, interest_rate,
		amortization_months, loan_term_months, interest_only_months, loan_origination_fees, monthly_pmi, is_active,
		fn_monthly_debt_service(original_loan_amount, interest_rate, amortization_months, interest_only_months) + monthly_pmi AS monthly_payment`
	ledgerColumns = `id, property_id, unit_id, transaction_date, category::text AS category, amount, vendor_or_payor, description, created_at`
)

// underwritingFields lists the editable underwriting_models columns in UnderwritingModel.values order.
var underwritingFields = []string{
	"scenario_name", "physical_vacancy_pct", "credit_loss_concessions_pct", "monthly_other_property_income",
	"annual_property_taxes", "annual_insurance", "monthly_hoa_dues", "monthly_landlord_utilities",
	"monthly_cam_lawn_snow_pest", "annual_admin_legal_cpa", "management_fee_pct_of_egi", "repairs_maint_pct_of_egi",
	"annual_repairs_per_unit", "capex_reserve_pct_of_egi", "annual_capex_per_unit", "annual_rent_growth_pct",
	"annual_expense_growth_pct", "exit_cap_rate_pct", "selling_cost_pct", "marginal_income_tax_pct",
}

// registerPropertyFinancialRoutes registers the per-property rent roll, debt, underwriting and ledger endpoints.
func (s *Server) registerPropertyFinancialRoutes(mux *http.ServeMux, authMW func(http.Handler) http.Handler) {
	mux.Handle("GET /api/properties/{id}/units", authMW(http.HandlerFunc(s.listUnits)))
	mux.Handle("POST /api/properties/{id}/units", authMW(http.HandlerFunc(s.createUnit)))
	mux.Handle("PUT /api/properties/{id}/units/{unitId}", authMW(http.HandlerFunc(s.updateUnit)))
	mux.Handle("DELETE /api/properties/{id}/units/{unitId}", authMW(http.HandlerFunc(s.deleteUnit)))
	mux.Handle("GET /api/properties/{id}/loans", authMW(http.HandlerFunc(s.listLoans)))
	mux.Handle("POST /api/properties/{id}/loans", authMW(http.HandlerFunc(s.createLoan)))
	mux.Handle("PUT /api/properties/{id}/loans/{loanId}", authMW(http.HandlerFunc(s.updateLoan)))
	mux.Handle("DELETE /api/properties/{id}/loans/{loanId}", authMW(http.HandlerFunc(s.deleteLoan)))
	mux.Handle("GET /api/properties/{id}/underwriting", authMW(http.HandlerFunc(s.getUnderwriting)))
	mux.Handle("PUT /api/properties/{id}/underwriting", authMW(http.HandlerFunc(s.putUnderwriting)))
	mux.Handle("GET /api/properties/{id}/ledger", authMW(http.HandlerFunc(s.listLedger)))
	mux.Handle("POST /api/properties/{id}/ledger", authMW(http.HandlerFunc(s.createLedgerTransaction)))
	mux.Handle("PUT /api/properties/{id}/ledger/{transactionId}", authMW(http.HandlerFunc(s.updateLedgerTransaction)))
	mux.Handle("DELETE /api/properties/{id}/ledger/{transactionId}", authMW(http.HandlerFunc(s.deleteLedgerTransaction)))
}

// defaultUnderwriting returns the column defaults of underwriting_models, which the analysis view also assumes.
func defaultUnderwriting(propID int64) UnderwritingModel {
	return UnderwritingModel{
		PropertyID:               propID,
		ScenarioName:             "Baseline Pro Forma",
		PhysicalVacancyPct:       0.05,
		CreditLossConcessionsPct: 0.01,
		ManagementFeePctOfEGI:    0.08,
		RepairsMaintPctOfEGI:     0.05,
		CapexReservePctOfEGI:     0.05,
		AnnualRentGrowthPct:      0.03,
		AnnualExpenseGrowthPct:   0.025,
		ExitCapRatePct:           0.065,
		SellingCostPct:           0.06,
		MarginalIncomeTaxPct:     0.32,
	}
}

func (m *UnderwritingModel) values() []any {
	return []any{m.ScenarioName, m.PhysicalVacancyPct, m.CreditLossConcessionsPct, m.MonthlyOtherPropertyIncome,
		m.AnnualPropertyTaxes, m.AnnualInsurance, m.MonthlyHOADues, m.MonthlyLandlordUtilities,
		m.MonthlyCAMLawnSnowPest, m.AnnualAdminLegalCPA, m.ManagementFeePctOfEGI, m.RepairsMaintPctOfEGI,
		m.AnnualRepairsPerUnit, m.CapexReservePctOfEGI, m.AnnualCapexPerUnit, m.AnnualRentGrowthPct,
		m.AnnualExpenseGrowthPct, m.ExitCapRatePct, m.SellingCostPct, m.MarginalIncomeTaxPct}
}

func (in *unitInput) validate() string {
	in.UnitNumber = strings.TrimSpace(in.UnitNumber)
	if n := utf8.RuneCountInString(in.UnitNumber); n < 1 || n > 30 {
		return "unit_number is required and must be at most 30 characters"
	}
	var ok bool
	if in.FloorPlanName, ok = optText(in.FloorPlanName, 50); !ok {
		return "floor_plan_name must be at most 50 characters"
	}
	one := 1.0
	if in.Bedrooms == nil {
		in.Bedrooms = &one
	}
	if in.Bathrooms == nil {
		in.Bathrooms = &one
	}
	if !validAmount(*in.Bedrooms, 99) || !validAmount(*in.Bathrooms, 99) {
		return "bedrooms and bathrooms must be between 0 and 99"
	}
	if in.RentableSqFt == nil || *in.RentableSqFt < 0 || *in.RentableSqFt > math.MaxInt32 {
		return "rentable_sq_ft is required and cannot be negative"
	}
	in.Status = strings.ToUpper(strings.TrimSpace(in.Status))
	if in.Status == "" {
		in.Status = "OCCUPIED"
	}
	if !slices.Contains(unitStatuses, in.Status) {
		return "status must be one of " + strings.Join(unitStatuses, ", ")
	}
	if in.MarketRent == nil || in.InPlaceRent == nil || !validAmount(*in.MarketRent, maxMonthlyAmount) || !validAmount(*in.InPlaceRent, maxMonthlyAmount) {
		return "market_rent and in_place_rent are required and must be between 0 and 99999999.99"
	}
	for _, v := range []float64{in.MonthlyRUBSCharge, in.MonthlyPetRent, in.MonthlyParkingStorage, in.SecurityDeposit} {
		if !validAmount(v, maxMonthlyAmount) {
			return "monthly charges and security_deposit must be between 0 and 99999999.99"
		}
	}
	if in.LeaseStartDate != nil && in.LeaseEndDate != nil && in.LeaseEndDate.Before(*in.LeaseStartDate) {
		return "lease_end_date cannot be before lease_start_date"
	}
	return ""
}

func (in *unitInput) args() []any {
	return []any{in.UnitNumber, in.FloorPlanName, *in.Bedrooms, *in.Bathrooms, *in.RentableSqFt, in.Status, in.IsRenovated,
		*in.MarketRent, *in.InPlaceRent, in.MonthlyRUBSCharge, in.MonthlyPetRent, in.MonthlyParkingStorage,
		in.SecurityDeposit, in.LeaseStartDate, in.LeaseEndDate}
}

func (in *loanInput) validate() string {
	var ok bool
	if in.LenderName, ok = optText(in.LenderName, 100); !ok {
		return "lender_name must be at most 100 characters"
	}
	in.LoanType = strings.ToUpper(strings.TrimSpace(in.LoanType))
	if in.LoanType == "" {
		in.LoanType = "CONVENTIONAL"
	}
	if utf8.RuneCountInString(in.LoanType) > 50 {
		return "loan_type must be at most 50 characters"
	}
	if in.OriginalLoanAmount == nil || in.CurrentPrincipalBalance == nil ||
		!validAmount(*in.OriginalLoanAmount, maxBasisAmount) || !validAmount(*in.CurrentPrincipalBalance, maxBasisAmount) {
		return "original_loan_amount and current_principal_balance are required and must be between 0 and 999999999999.99"
	}
	if in.InterestRate == nil || !validAmount(*in.InterestRate, 1) {
		return "interest_rate is required and must be a fraction between 0 and 1 (0.065 = 6.5%)"
	}
	months := 360
	if in.AmortizationMonths == nil {
		in.AmortizationMonths = &months
	}
	if in.LoanTermMonths == nil {
		in.LoanTermMonths = &months
	}
	if *in.AmortizationMonths < 1 || *in.AmortizationMonths > math.MaxInt16 || *in.LoanTermMonths < 1 || *in.LoanTermMonths > math.MaxInt16 {
		return "amortization_months and loan_term_months must be between 1 and 32767"
	}
	if in.InterestOnlyMonths < 0 || in.InterestOnlyMonths > math.MaxInt16 {
		return "interest_only_months must be between 0 and 32767"
	}
	if !validAmount(in.LoanOriginationFees, maxMoneyAmount) || !validAmount(in.MonthlyPMI, maxMonthlyAmount) {
		return "loan_origination_fees and monthly_pmi cannot be negative"
	}
	if in.IsActive == nil {
		active := true
		in.IsActive = &active
	}
	return ""
}

func (in *loanInput) args() []any {
	return []any{in.LenderName, in.LoanType, *in.OriginalLoanAmount, *in.CurrentPrincipalBalance, *in.InterestRate,
		*in.AmortizationMonths, *in.LoanTermMonths, in.InterestOnlyMonths, in.LoanOriginationFees, in.MonthlyPMI, *in.IsActive}
}

func (m *UnderwritingModel) validate() string {
	m.ScenarioName = strings.TrimSpace(m.ScenarioName)
	if m.ScenarioName == "" {
		m.ScenarioName = "Baseline Pro Forma"
	}
	if utf8.RuneCountInString(m.ScenarioName) > 100 {
		return "scenario_name must be at most 100 characters"
	}
	for _, v := range []float64{m.PhysicalVacancyPct, m.CreditLossConcessionsPct, m.ManagementFeePctOfEGI,
		m.RepairsMaintPctOfEGI, m.CapexReservePctOfEGI, m.ExitCapRatePct, m.SellingCostPct, m.MarginalIncomeTaxPct} {
		if !validAmount(v, 1) {
			return "percentage fields must be fractions between 0 and 1 (0.05 = 5%)"
		}
	}
	for _, v := range []float64{m.AnnualRentGrowthPct, m.AnnualExpenseGrowthPct} {
		if math.IsNaN(v) || v < -1 || v > 1 {
			return "growth rates must be fractions between -1 and 1"
		}
	}
	for _, v := range []float64{m.MonthlyOtherPropertyIncome, m.MonthlyHOADues, m.MonthlyLandlordUtilities,
		m.MonthlyCAMLawnSnowPest, m.AnnualAdminLegalCPA, m.AnnualRepairsPerUnit, m.AnnualCapexPerUnit} {
		if !validAmount(v, maxMonthlyAmount) {
			return "dollar amounts must be between 0 and 99999999.99"
		}
	}
	if !validAmount(m.AnnualPropertyTaxes, maxMoneyAmount) || !validAmount(m.AnnualInsurance, maxMoneyAmount) {
		return "annual_property_taxes and annual_insurance must be between 0 and 9999999999.99"
	}
	return ""
}

func (in *ledgerInput) validate() string {
	if in.TransactionDate == nil {
		return "transaction_date is required"
	}
	in.Category = strings.ToUpper(strings.TrimSpace(in.Category))
	if !slices.Contains(ledgerCategories, in.Category) {
		return "category must be one of " + strings.Join(ledgerCategories, ", ")
	}
	if in.Amount == nil || math.IsNaN(*in.Amount) || math.Abs(*in.Amount) > maxMoneyAmount {
		return "amount is required and must be between -9999999999.99 and 9999999999.99"
	}
	if in.UnitID != nil && *in.UnitID <= 0 {
		return "invalid unit_id"
	}
	var ok bool
	if in.VendorOrPayor, ok = optText(in.VendorOrPayor, 150); !ok {
		return "vendor_or_payor must be at most 150 characters"
	}
	if in.Description, ok = optText(in.Description, 2000); !ok {
		return "description must be at most 2000 characters"
	}
	return ""
}

func (in *ledgerInput) args() []any {
	return []any{in.UnitID, *in.TransactionDate, in.Category, *in.Amount, in.VendorOrPayor, in.Description}
}

// authorizedProperty parses the {id} path value and checks the caller may access that property.
func (s *Server) authorizedProperty(w http.ResponseWriter, r *http.Request, fn string) (int64, bool) {
	propID, ok := parsePathID(r, "id")
	if !ok {
		writeErr(w, http.StatusBadRequest, "invalid property id")
		return 0, false
	}
	if s.DB == nil {
		writeErr(w, http.StatusInternalServerError, "database not configured")
		return 0, false
	}
	if _, ok := s.authorizeProperty(w, r, fn, propID); !ok {
		return 0, false
	}
	return propID, true
}

// pathChildID parses a child record's id from the path, writing a 400 when it is invalid.
func pathChildID(w http.ResponseWriter, r *http.Request, name, label string) (int64, bool) {
	id, ok := parsePathID(r, name)
	if !ok {
		writeErr(w, http.StatusBadRequest, "invalid "+label+" id")
	}
	return id, ok
}

// decodeValid decodes the request body into in and validates it, writing a 400 on failure.
func decodeValid(w http.ResponseWriter, r *http.Request, in interface{ validate() string }) bool {
	if !decode(w, r, in) {
		return false
	}
	if msg := in.validate(); msg != "" {
		writeErr(w, http.StatusBadRequest, msg)
		return false
	}
	return true
}

// respondRows writes every row a query returns as a JSON array of T.
func respondRows[T any](w http.ResponseWriter, fn string, rows pgx.Rows, err error) {
	var out []T
	if err == nil {
		out, err = pgx.CollectRows(rows, pgx.RowToStructByName[T])
	}
	if err != nil {
		log.Printf("%s: query error: %v", fn, err)
		writeErr(w, http.StatusInternalServerError, "internal error")
		return
	}
	if out == nil {
		out = []T{}
	}
	writeJSON(w, http.StatusOK, out)
}

// respondRow writes the single row a query returns as T, or a 404 with notFound when it returns none.
func respondRow[T any](w http.ResponseWriter, fn string, status int, notFound string, rows pgx.Rows, err error) {
	var out T
	if err == nil {
		out, err = pgx.CollectExactlyOneRow(rows, pgx.RowToStructByName[T])
	}
	var pgErr *pgconn.PgError
	switch {
	case err == nil:
		writeJSON(w, status, out)
	case errors.Is(err, pgx.ErrNoRows):
		writeErr(w, http.StatusNotFound, notFound)
	case errors.As(err, &pgErr) && pgErr.Code == "23505" && pgErr.ConstraintName == "units_property_id_unit_number_key":
		writeErr(w, http.StatusConflict, "unit_number already exists on this property")
	default:
		log.Printf("%s: database error: %v", fn, err)
		writeErr(w, http.StatusInternalServerError, "internal error")
	}
}

// deleteChild deletes the path's record from a property child table, 404 when it is not on the property.
func (s *Server) deleteChild(w http.ResponseWriter, r *http.Request, fn, table, pathName, label string) {
	propID, ok := s.authorizedProperty(w, r, fn)
	if !ok {
		return
	}
	id, ok := pathChildID(w, r, pathName, label)
	if !ok {
		return
	}
	tag, err := s.DB.Exec(r.Context(), `DELETE FROM `+table+` WHERE id = $1 AND property_id = $2`, id, propID)
	if err != nil {
		log.Printf("%s: database delete error: %v", fn, err)
		writeErr(w, http.StatusInternalServerError, "internal error")
		return
	}
	if tag.RowsAffected() == 0 {
		writeErr(w, http.StatusNotFound, label+" not found")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) listUnits(w http.ResponseWriter, r *http.Request) {
	propID, ok := s.authorizedProperty(w, r, "listUnits")
	if !ok {
		return
	}
	rows, err := s.DB.Query(r.Context(),
		`SELECT `+unitColumns+` FROM units WHERE property_id = $1 ORDER BY unit_number, id`, propID)
	respondRows[Unit](w, "listUnits", rows, err)
}

func (s *Server) createUnit(w http.ResponseWriter, r *http.Request) {
	var in unitInput
	if !decodeValid(w, r, &in) {
		return
	}
	propID, ok := s.authorizedProperty(w, r, "createUnit")
	if !ok {
		return
	}
	rows, err := s.DB.Query(r.Context(),
		`INSERT INTO units (property_id, unit_number, floor_plan_name, bedrooms, bathrooms, rentable_sq_ft, status, is_renovated,
			market_rent, in_place_rent, monthly_rubs_charge, monthly_pet_rent, monthly_parking_storage, security_deposit,
			lease_start_date, lease_end_date)
		 VALUES ($1,$2,$3,$4,$5,$6,$7::text::unit_status,$8,$9,$10,$11,$12,$13,$14,$15,$16) RETURNING `+unitColumns,
		append([]any{propID}, in.args()...)...)
	respondRow[Unit](w, "createUnit", http.StatusCreated, "property not found", rows, err)
}

// updateUnit replaces a unit's fields; omitted optional fields reset to their defaults.
func (s *Server) updateUnit(w http.ResponseWriter, r *http.Request) {
	var in unitInput
	if !decodeValid(w, r, &in) {
		return
	}
	propID, ok := s.authorizedProperty(w, r, "updateUnit")
	if !ok {
		return
	}
	unitID, ok := pathChildID(w, r, "unitId", "unit")
	if !ok {
		return
	}
	rows, err := s.DB.Query(r.Context(),
		`UPDATE units SET unit_number = $3, floor_plan_name = $4, bedrooms = $5, bathrooms = $6, rentable_sq_ft = $7,
			status = $8::text::unit_status, is_renovated = $9, market_rent = $10, in_place_rent = $11,
			monthly_rubs_charge = $12, monthly_pet_rent = $13, monthly_parking_storage = $14, security_deposit = $15,
			lease_start_date = $16, lease_end_date = $17
		 WHERE id = $1 AND property_id = $2 RETURNING `+unitColumns,
		append([]any{unitID, propID}, in.args()...)...)
	respondRow[Unit](w, "updateUnit", http.StatusOK, "unit not found", rows, err)
}

// deleteUnit removes a unit; ledger transactions tagged with it keep their property and lose the unit tag.
func (s *Server) deleteUnit(w http.ResponseWriter, r *http.Request) {
	s.deleteChild(w, r, "deleteUnit", "units", "unitId", "unit")
}

func (s *Server) listLoans(w http.ResponseWriter, r *http.Request) {
	propID, ok := s.authorizedProperty(w, r, "listLoans")
	if !ok {
		return
	}
	rows, err := s.DB.Query(r.Context(),
		`SELECT `+loanColumns+` FROM property_loans WHERE property_id = $1 ORDER BY is_active DESC, id`, propID)
	respondRows[Loan](w, "listLoans", rows, err)
}

func (s *Server) createLoan(w http.ResponseWriter, r *http.Request) {
	var in loanInput
	if !decodeValid(w, r, &in) {
		return
	}
	propID, ok := s.authorizedProperty(w, r, "createLoan")
	if !ok {
		return
	}
	rows, err := s.DB.Query(r.Context(),
		`INSERT INTO property_loans (property_id, lender_name, loan_type, original_loan_amount, current_principal_balance,
			interest_rate, amortization_months, loan_term_months, interest_only_months, loan_origination_fees, monthly_pmi, is_active)
		 VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12) RETURNING `+loanColumns,
		append([]any{propID}, in.args()...)...)
	respondRow[Loan](w, "createLoan", http.StatusCreated, "property not found", rows, err)
}

// updateLoan replaces a loan's fields; omitted optional fields reset to their defaults.
func (s *Server) updateLoan(w http.ResponseWriter, r *http.Request) {
	var in loanInput
	if !decodeValid(w, r, &in) {
		return
	}
	propID, ok := s.authorizedProperty(w, r, "updateLoan")
	if !ok {
		return
	}
	loanID, ok := pathChildID(w, r, "loanId", "loan")
	if !ok {
		return
	}
	rows, err := s.DB.Query(r.Context(),
		`UPDATE property_loans SET lender_name = $3, loan_type = $4, original_loan_amount = $5, current_principal_balance = $6,
			interest_rate = $7, amortization_months = $8, loan_term_months = $9, interest_only_months = $10,
			loan_origination_fees = $11, monthly_pmi = $12, is_active = $13
		 WHERE id = $1 AND property_id = $2 RETURNING `+loanColumns,
		append([]any{loanID, propID}, in.args()...)...)
	respondRow[Loan](w, "updateLoan", http.StatusOK, "loan not found", rows, err)
}

func (s *Server) deleteLoan(w http.ResponseWriter, r *http.Request) {
	s.deleteChild(w, r, "deleteLoan", "property_loans", "loanId", "loan")
}

// getUnderwriting returns the property's primary underwriting model, or the defaults (id null) when none is saved.
func (s *Server) getUnderwriting(w http.ResponseWriter, r *http.Request) {
	propID, ok := s.authorizedProperty(w, r, "getUnderwriting")
	if !ok {
		return
	}
	rows, err := s.DB.Query(r.Context(),
		`SELECT id, property_id, `+strings.Join(underwritingFields, ", ")+`
		 FROM underwriting_models WHERE property_id = $1 AND is_primary_model`, propID)
	if err == nil {
		var m UnderwritingModel
		m, err = pgx.CollectExactlyOneRow(rows, pgx.RowToStructByName[UnderwritingModel])
		if errors.Is(err, pgx.ErrNoRows) {
			m, err = defaultUnderwriting(propID), nil
		}
		if err == nil {
			writeJSON(w, http.StatusOK, m)
			return
		}
	}
	log.Printf("getUnderwriting: query error: %v", err)
	writeErr(w, http.StatusInternalServerError, "internal error")
}

// putUnderwriting creates or replaces the property's primary underwriting model; omitted fields take their defaults.
func (s *Server) putUnderwriting(w http.ResponseWriter, r *http.Request) {
	m := defaultUnderwriting(0)
	if !decodeValid(w, r, &m) {
		return
	}
	propID, ok := s.authorizedProperty(w, r, "putUnderwriting")
	if !ok {
		return
	}
	placeholders := make([]string, len(underwritingFields))
	updates := make([]string, len(underwritingFields))
	for i, f := range underwritingFields {
		placeholders[i] = "$" + strconv.Itoa(i+2)
		updates[i] = f + " = EXCLUDED." + f
	}
	rows, err := s.DB.Query(r.Context(),
		`INSERT INTO underwriting_models (property_id, is_primary_model, `+strings.Join(underwritingFields, ", ")+`)
		 VALUES ($1, TRUE, `+strings.Join(placeholders, ", ")+`)
		 ON CONFLICT (property_id) WHERE is_primary_model DO UPDATE SET `+strings.Join(updates, ", ")+`
		 RETURNING id, property_id, `+strings.Join(underwritingFields, ", "),
		append([]any{propID}, m.values()...)...)
	respondRow[UnderwritingModel](w, "putUnderwriting", http.StatusOK, "property not found", rows, err)
}

// listLedger returns the property's transactions, newest first, optionally limited to ?from=YYYY-MM-DD&to=YYYY-MM-DD.
func (s *Server) listLedger(w http.ResponseWriter, r *http.Request) {
	var from, to *time.Time
	for _, p := range []struct {
		name string
		dest **time.Time
	}{{"from", &from}, {"to", &to}} {
		if v := r.URL.Query().Get(p.name); v != "" {
			d, err := time.Parse(time.DateOnly, v)
			if err != nil {
				writeErr(w, http.StatusBadRequest, p.name+" must be a date in YYYY-MM-DD format")
				return
			}
			*p.dest = &d
		}
	}
	propID, ok := s.authorizedProperty(w, r, "listLedger")
	if !ok {
		return
	}
	rows, err := s.DB.Query(r.Context(),
		`SELECT `+ledgerColumns+` FROM ledger_transactions
		 WHERE property_id = $1 AND ($2::date IS NULL OR transaction_date >= $2) AND ($3::date IS NULL OR transaction_date <= $3)
		 ORDER BY transaction_date DESC, id DESC`, propID, from, to)
	respondRows[LedgerTransaction](w, "listLedger", rows, err)
}

// checkLedgerUnit writes a 400 and returns false unless the transaction's unit is unset or on the property.
func (s *Server) checkLedgerUnit(w http.ResponseWriter, r *http.Request, fn string, unitID *int64, propID int64) bool {
	if unitID == nil {
		return true
	}
	var exists bool
	err := s.DB.QueryRow(r.Context(),
		`SELECT EXISTS (SELECT 1 FROM units WHERE id = $1 AND property_id = $2)`, *unitID, propID).Scan(&exists)
	if err != nil {
		log.Printf("%s: unit query error: %v", fn, err)
		writeErr(w, http.StatusInternalServerError, "internal error")
		return false
	}
	if !exists {
		writeErr(w, http.StatusBadRequest, "unit_id is not a unit of this property")
	}
	return exists
}

func (s *Server) createLedgerTransaction(w http.ResponseWriter, r *http.Request) {
	var in ledgerInput
	if !decodeValid(w, r, &in) {
		return
	}
	propID, ok := s.authorizedProperty(w, r, "createLedgerTransaction")
	if !ok || !s.checkLedgerUnit(w, r, "createLedgerTransaction", in.UnitID, propID) {
		return
	}
	rows, err := s.DB.Query(r.Context(),
		`INSERT INTO ledger_transactions (property_id, unit_id, transaction_date, category, amount, vendor_or_payor, description)
		 VALUES ($1,$2,$3,$4::text::ledger_category,$5,$6,$7) RETURNING `+ledgerColumns,
		append([]any{propID}, in.args()...)...)
	respondRow[LedgerTransaction](w, "createLedgerTransaction", http.StatusCreated, "property not found", rows, err)
}

func (s *Server) updateLedgerTransaction(w http.ResponseWriter, r *http.Request) {
	var in ledgerInput
	if !decodeValid(w, r, &in) {
		return
	}
	propID, ok := s.authorizedProperty(w, r, "updateLedgerTransaction")
	if !ok {
		return
	}
	txID, ok := pathChildID(w, r, "transactionId", "transaction")
	if !ok || !s.checkLedgerUnit(w, r, "updateLedgerTransaction", in.UnitID, propID) {
		return
	}
	rows, err := s.DB.Query(r.Context(),
		`UPDATE ledger_transactions SET unit_id = $3, transaction_date = $4, category = $5::text::ledger_category,
			amount = $6, vendor_or_payor = $7, description = $8
		 WHERE id = $1 AND property_id = $2 RETURNING `+ledgerColumns,
		append([]any{txID, propID}, in.args()...)...)
	respondRow[LedgerTransaction](w, "updateLedgerTransaction", http.StatusOK, "transaction not found", rows, err)
}

func (s *Server) deleteLedgerTransaction(w http.ResponseWriter, r *http.Request) {
	s.deleteChild(w, r, "deleteLedgerTransaction", "ledger_transactions", "transactionId", "transaction")
}
