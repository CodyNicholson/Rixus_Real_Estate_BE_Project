CREATE TABLE IF NOT EXISTS users (
    id BIGSERIAL PRIMARY KEY,
    email TEXT NOT NULL UNIQUE,
    name TEXT NOT NULL,
    user_role TEXT NOT NULL DEFAULT 'VIEWER' CHECK (user_role IN ('ADMIN', 'VIEWER')),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- For existing databases, drop password_hash as authentication relies on Google OAuth
ALTER TABLE users DROP COLUMN IF EXISTS password_hash;
ALTER TABLE users ALTER COLUMN user_role SET DEFAULT 'VIEWER';

CREATE TABLE IF NOT EXISTS properties (
    id BIGSERIAL PRIMARY KEY,
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    street_address TEXT NOT NULL,
    city TEXT NOT NULL DEFAULT '',
    state TEXT NOT NULL DEFAULT '',
    zip TEXT NOT NULL DEFAULT '',
    units SMALLINT NOT NULL DEFAULT 1 CHECK (units > 0),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- For existing databases, replace monthly_rent with units
ALTER TABLE properties DROP COLUMN IF EXISTS monthly_rent;
ALTER TABLE properties ADD COLUMN IF NOT EXISTS units SMALLINT NOT NULL DEFAULT 1 CHECK (units > 0);

CREATE INDEX IF NOT EXISTS properties_user_id_idx ON properties(user_id);

CREATE TABLE IF NOT EXISTS tenants (
    tenant_id BIGSERIAL PRIMARY KEY,
    tenant_initials VARCHAR(4) NOT NULL,
    property_id BIGINT NOT NULL REFERENCES properties(id) ON DELETE CASCADE,
    unit_number SMALLINT NOT NULL DEFAULT 1 CHECK (unit_number > 0),
    monthly_rent NUMERIC(12,2) NOT NULL CHECK (monthly_rent >= 0),
    pet_count SMALLINT NOT NULL DEFAULT 0 CHECK (pet_count >= 0),
    move_in_date TIMESTAMPTZ,
    move_out_date TIMESTAMPTZ,
    lease_start_date TIMESTAMPTZ,
    lease_end_date TIMESTAMPTZ,
    security_deposit NUMERIC(12,2) NOT NULL DEFAULT 0 CHECK (security_deposit >= 0),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- For existing databases, add lease dates
ALTER TABLE tenants ADD COLUMN IF NOT EXISTS lease_start_date TIMESTAMPTZ;
ALTER TABLE tenants ADD COLUMN IF NOT EXISTS lease_end_date TIMESTAMPTZ;

-- For existing databases, add security deposit
ALTER TABLE tenants ADD COLUMN IF NOT EXISTS security_deposit NUMERIC(12,2) NOT NULL DEFAULT 0 CHECK (security_deposit >= 0);

CREATE INDEX IF NOT EXISTS tenants_property_id_idx ON tenants(property_id);

-- Unified property-unit financial model (see migrations/008_financials.sql)

DO $$ BEGIN
    CREATE TYPE property_category AS ENUM ('SINGLE_FAMILY', 'CONDO_TOWNHOME', 'MULTI_FAMILY_2_4', 'MULTI_FAMILY_5_PLUS', 'MIXED_USE');
EXCEPTION WHEN duplicate_object THEN NULL; END $$;

DO $$ BEGIN
    CREATE TYPE unit_status AS ENUM ('OCCUPIED', 'VACANT_READY', 'VACANT_RENOVATION', 'NON_REVENUE_MODEL');
EXCEPTION WHEN duplicate_object THEN NULL; END $$;

DO $$ BEGIN
    CREATE TYPE ledger_category AS ENUM (
        -- Revenue
        'GROSS_RENT', 'RUBS_REIMBURSEMENT', 'PET_RENT', 'PARKING_STORAGE', 'LAUNDRY_VENDING', 'OTHER_INCOME', 'CONCESSIONS_BAD_DEBT',
        -- Operating expenses
        'PROPERTY_TAX', 'INSURANCE', 'HOA_DUES', 'PROPERTY_MANAGEMENT', 'LEASING_FEES',
        'UTILITIES_WATER_SEWER', 'UTILITIES_ELECTRIC_GAS', 'UTILITIES_TRASH',
        'CAM_LAWN_SNOW_PEST', 'REPAIRS_MAINTENANCE', 'TURNOVER_MAKE_READY', 'ADMIN_LEGAL_CPA',
        -- Below the line
        'CAPEX_REPLACEMENT', 'INITIAL_REHAB_BASIS', 'DEBT_PRINCIPAL', 'DEBT_INTEREST'
    );
EXCEPTION WHEN duplicate_object THEN NULL; END $$;

-- Holding LLCs, Series LLC cells or personal ownership, owned by a user
CREATE TABLE IF NOT EXISTS legal_entities (
    id BIGSERIAL PRIMARY KEY,
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    name VARCHAR(150) NOT NULL,
    entity_type VARCHAR(50) NOT NULL,
    bank_account_last4 VARCHAR(4),
    credit_card_last4 VARCHAR(4),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS legal_entities_user_id_idx ON legal_entities(user_id);

-- Acquisition basis, physical metadata and depreciation allocation on properties
ALTER TABLE properties ADD COLUMN IF NOT EXISTS legal_entity_id BIGINT REFERENCES legal_entities(id) ON DELETE SET NULL;
ALTER TABLE properties ADD COLUMN IF NOT EXISTS name VARCHAR(150) NOT NULL DEFAULT '';
ALTER TABLE properties ADD COLUMN IF NOT EXISTS category property_category NOT NULL DEFAULT 'SINGLE_FAMILY';
ALTER TABLE properties ADD COLUMN IF NOT EXISTS parcel_pin VARCHAR(50);
ALTER TABLE properties ADD COLUMN IF NOT EXISTS year_built SMALLINT;
ALTER TABLE properties ADD COLUMN IF NOT EXISTS gross_sq_ft INTEGER;
ALTER TABLE properties ADD COLUMN IF NOT EXISTS acquisition_date DATE;
ALTER TABLE properties ADD COLUMN IF NOT EXISTS purchase_price NUMERIC(14,2) NOT NULL DEFAULT 0 CHECK (purchase_price >= 0);
ALTER TABLE properties ADD COLUMN IF NOT EXISTS closing_costs NUMERIC(14,2) NOT NULL DEFAULT 0;
ALTER TABLE properties ADD COLUMN IF NOT EXISTS initial_rehab_budget NUMERIC(14,2) NOT NULL DEFAULT 0;
ALTER TABLE properties ADD COLUMN IF NOT EXISTS initial_operating_reserve NUMERIC(14,2) NOT NULL DEFAULT 0;
ALTER TABLE properties ADD COLUMN IF NOT EXISTS after_repair_value NUMERIC(14,2);
ALTER TABLE properties ADD COLUMN IF NOT EXISTS land_value_allocation_pct NUMERIC(5,4) NOT NULL DEFAULT 0.2000 CHECK (land_value_allocation_pct BETWEEN 0 AND 1);
ALTER TABLE properties ADD COLUMN IF NOT EXISTS depreciation_schedule_years NUMERIC(4,1) NOT NULL DEFAULT 27.5 CHECK (depreciation_schedule_years > 0);
ALTER TABLE properties ADD COLUMN IF NOT EXISTS utility_config JSONB NOT NULL DEFAULT '{"water": "LANDLORD", "electric": "TENANT", "gas": "TENANT", "trash": "LANDLORD"}'::jsonb;
ALTER TABLE properties ADD COLUMN IF NOT EXISTS is_active BOOLEAN NOT NULL DEFAULT TRUE;

CREATE INDEX IF NOT EXISTS properties_legal_entity_id_idx ON properties(legal_entity_id);

-- Rent roll: 1 row for a single-unit property, N rows for multi-unit
CREATE TABLE IF NOT EXISTS units (
    id BIGSERIAL PRIMARY KEY,
    property_id BIGINT NOT NULL REFERENCES properties(id) ON DELETE CASCADE,
    unit_number VARCHAR(30) NOT NULL,
    floor_plan_name VARCHAR(50),
    bedrooms NUMERIC(3,1) NOT NULL DEFAULT 1,
    bathrooms NUMERIC(3,1) NOT NULL DEFAULT 1,
    rentable_sq_ft INTEGER NOT NULL,
    status unit_status NOT NULL DEFAULT 'OCCUPIED',
    is_renovated BOOLEAN NOT NULL DEFAULT FALSE,
    market_rent NUMERIC(10,2) NOT NULL,
    in_place_rent NUMERIC(10,2) NOT NULL,
    monthly_rubs_charge NUMERIC(10,2) NOT NULL DEFAULT 0,
    monthly_pet_rent NUMERIC(10,2) NOT NULL DEFAULT 0,
    monthly_parking_storage NUMERIC(10,2) NOT NULL DEFAULT 0,
    security_deposit NUMERIC(10,2) NOT NULL DEFAULT 0,
    lease_start_date DATE,
    lease_end_date DATE,
    UNIQUE (property_id, unit_number)
);

-- Multimedia and documents storage for PDFs, images, videos, audio, and contracts
CREATE TABLE IF NOT EXISTS multimedia (
    id BIGSERIAL PRIMARY KEY,
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    property_id BIGINT REFERENCES properties(id) ON DELETE SET NULL,
    title VARCHAR(255) NOT NULL,
    description TEXT,
    category VARCHAR(50) NOT NULL DEFAULT 'document',
    media_type VARCHAR(50) NOT NULL DEFAULT 'pdf',
    file_name VARCHAR(255) NOT NULL,
    file_size BIGINT NOT NULL,
    mime_type VARCHAR(120) NOT NULL,
    file_path TEXT NOT NULL DEFAULT '',
    file_data BYTEA,
    is_template BOOLEAN NOT NULL DEFAULT false,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS multimedia_user_id_idx ON multimedia(user_id);
CREATE INDEX IF NOT EXISTS multimedia_property_id_idx ON multimedia(property_id);
CREATE INDEX IF NOT EXISTS multimedia_media_type_idx ON multimedia(media_type);
CREATE INDEX IF NOT EXISTS multimedia_is_template_idx ON multimedia(is_template);

-- Debt facilities: primary mortgages, interest-only bridge debt and seller financing
CREATE TABLE IF NOT EXISTS property_loans (
    id BIGSERIAL PRIMARY KEY,
    property_id BIGINT NOT NULL REFERENCES properties(id) ON DELETE CASCADE,
    lender_name VARCHAR(100),
    loan_type VARCHAR(50) NOT NULL DEFAULT 'CONVENTIONAL',
    original_loan_amount NUMERIC(14,2) NOT NULL,
    current_principal_balance NUMERIC(14,2) NOT NULL,
    interest_rate NUMERIC(7,5) NOT NULL,
    amortization_months SMALLINT NOT NULL DEFAULT 360,
    loan_term_months SMALLINT NOT NULL DEFAULT 360,
    interest_only_months SMALLINT NOT NULL DEFAULT 0,
    loan_origination_fees NUMERIC(12,2) NOT NULL DEFAULT 0,
    monthly_pmi NUMERIC(10,2) NOT NULL DEFAULT 0,
    is_active BOOLEAN NOT NULL DEFAULT TRUE
);

-- Pro forma operating assumptions per property
CREATE TABLE IF NOT EXISTS underwriting_models (
    id BIGSERIAL PRIMARY KEY,
    property_id BIGINT NOT NULL REFERENCES properties(id) ON DELETE CASCADE,
    scenario_name VARCHAR(100) NOT NULL DEFAULT 'Baseline Pro Forma',
    is_primary_model BOOLEAN NOT NULL DEFAULT TRUE,
    physical_vacancy_pct NUMERIC(6,4) NOT NULL DEFAULT 0.0500,
    credit_loss_concessions_pct NUMERIC(6,4) NOT NULL DEFAULT 0.0100,
    monthly_other_property_income NUMERIC(10,2) NOT NULL DEFAULT 0,
    annual_property_taxes NUMERIC(12,2) NOT NULL DEFAULT 0,
    annual_insurance NUMERIC(12,2) NOT NULL DEFAULT 0,
    monthly_hoa_dues NUMERIC(10,2) NOT NULL DEFAULT 0,
    monthly_landlord_utilities NUMERIC(10,2) NOT NULL DEFAULT 0,
    monthly_cam_lawn_snow_pest NUMERIC(10,2) NOT NULL DEFAULT 0,
    annual_admin_legal_cpa NUMERIC(10,2) NOT NULL DEFAULT 0,
    management_fee_pct_of_egi NUMERIC(6,4) NOT NULL DEFAULT 0.0800,
    repairs_maint_pct_of_egi NUMERIC(6,4) NOT NULL DEFAULT 0.0500,
    annual_repairs_per_unit NUMERIC(10,2) NOT NULL DEFAULT 0,
    capex_reserve_pct_of_egi NUMERIC(6,4) NOT NULL DEFAULT 0.0500,
    annual_capex_per_unit NUMERIC(10,2) NOT NULL DEFAULT 0,
    annual_rent_growth_pct NUMERIC(6,4) NOT NULL DEFAULT 0.0300,
    annual_expense_growth_pct NUMERIC(6,4) NOT NULL DEFAULT 0.0250,
    exit_cap_rate_pct NUMERIC(6,4) NOT NULL DEFAULT 0.0650,
    selling_cost_pct NUMERIC(6,4) NOT NULL DEFAULT 0.0600,
    marginal_income_tax_pct NUMERIC(6,4) NOT NULL DEFAULT 0.3200
);

-- At most one primary model per property, so the analysis view returns one row per property
CREATE UNIQUE INDEX IF NOT EXISTS underwriting_models_primary_idx ON underwriting_models(property_id) WHERE is_primary_model;

-- Actual income and expense entries for trailing-12-month analysis
CREATE TABLE IF NOT EXISTS ledger_transactions (
    id BIGSERIAL PRIMARY KEY,
    property_id BIGINT NOT NULL REFERENCES properties(id) ON DELETE CASCADE,
    unit_id BIGINT REFERENCES units(id) ON DELETE SET NULL,
    transaction_date DATE NOT NULL,
    category ledger_category NOT NULL,
    amount NUMERIC(12,2) NOT NULL,
    vendor_or_payor VARCHAR(150),
    description TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS units_property_id_idx ON units(property_id);
CREATE INDEX IF NOT EXISTS property_loans_property_active_idx ON property_loans(property_id) WHERE is_active;
CREATE INDEX IF NOT EXISTS ledger_transactions_property_date_idx ON ledger_transactions(property_id, transaction_date);

-- Fixed monthly payment for a loan; interest-only while p_io_months > 0, straight-line for 0% seller notes
CREATE OR REPLACE FUNCTION fn_monthly_debt_service(
    p_principal NUMERIC,
    p_annual_rate NUMERIC,
    p_amort_months SMALLINT,
    p_io_months SMALLINT DEFAULT 0
) RETURNS NUMERIC AS $$
DECLARE
    v_monthly_rate NUMERIC;
BEGIN
    IF p_principal IS NULL OR p_principal <= 0 THEN
        RETURN 0.00;
    END IF;
    IF p_annual_rate <= 0 THEN
        RETURN ROUND(p_principal / NULLIF(p_amort_months, 0), 2);
    END IF;
    v_monthly_rate := p_annual_rate / 12.0;
    IF p_io_months > 0 THEN
        RETURN ROUND(p_principal * v_monthly_rate, 2);
    END IF;
    -- P * [r(1+r)^n] / [(1+r)^n - 1]
    RETURN ROUND(
        p_principal * (v_monthly_rate * POWER(1.0 + v_monthly_rate, p_amort_months))
        / (POWER(1.0 + v_monthly_rate, p_amort_months) - 1.0),
        2
    );
END;
$$ LANGUAGE plpgsql IMMUTABLE;

-- Views are rebuilt on every run so column changes apply cleanly
DROP VIEW IF EXISTS v_portfolio_summary;
DROP FUNCTION IF EXISTS fn_portfolio_summary(BIGINT, property_category[], BIGINT, BIGINT[]);
DROP VIEW IF EXISTS v_property_financial_analysis;

-- One row per active property: income waterfall, operating expenses, debt service, tax and return ratios.
-- Pro forma income is underwritten on market rent; loss to lease is reported but not deducted.
CREATE VIEW v_property_financial_analysis AS
WITH unit_rollup AS (
    SELECT
        property_id,
        COUNT(*) FILTER (WHERE status = 'OCCUPIED') AS occupied_units,
        SUM(rentable_sq_ft) AS total_rentable_sq_ft,
        SUM(market_rent * 12) AS annual_gross_potential_rent,
        SUM(in_place_rent * 12) AS annual_in_place_scheduled_rent,
        SUM(CASE WHEN status = 'NON_REVENUE_MODEL' THEN market_rent * 12 ELSE 0 END) AS non_revenue_unit_loss,
        SUM((monthly_rubs_charge + monthly_pet_rent + monthly_parking_storage) * 12) AS annual_unit_ancillary_income
    FROM units
    GROUP BY property_id
),
debt_rollup AS (
    SELECT
        property_id,
        SUM(original_loan_amount) AS total_original_debt,
        SUM(current_principal_balance) AS total_current_debt,
        SUM(loan_origination_fees) AS total_loan_fees,
        SUM((fn_monthly_debt_service(original_loan_amount, interest_rate, amortization_months, interest_only_months) + monthly_pmi) * 12) AS annual_debt_service,
        SUM(ROUND(current_principal_balance * interest_rate, 2)) AS approx_year1_interest_expense
    FROM property_loans
    WHERE is_active
    GROUP BY property_id
),
base AS (
    SELECT
        p.id AS property_id,
        p.user_id,
        p.legal_entity_id,
        le.name AS legal_entity_name,
        COALESCE(NULLIF(p.name, ''), p.street_address) AS property_name,
        p.category,
        p.street_address,
        p.city,
        p.state,
        p.units::bigint AS total_units,
        COALESCE(u.occupied_units, 0)::bigint AS occupied_units,
        COALESCE(u.total_rentable_sq_ft, p.gross_sq_ft, 0)::bigint AS rentable_sq_ft,
        p.purchase_price,
        COALESCE(p.after_repair_value, p.purchase_price) AS current_market_value,
        p.purchase_price + p.closing_costs + p.initial_rehab_budget AS all_in_cost_basis,
        (p.purchase_price - COALESCE(d.total_original_debt, 0))
            + p.closing_costs + p.initial_rehab_budget + p.initial_operating_reserve
            + COALESCE(d.total_loan_fees, 0) AS total_cash_invested,
        COALESCE(d.total_current_debt, 0) AS total_debt,
        COALESCE(d.annual_debt_service, 0) AS annual_debt_service,
        COALESCE(d.approx_year1_interest_expense, 0) AS annual_interest_expense,
        COALESCE(u.annual_gross_potential_rent, 0) AS gpr_market,
        COALESCE(u.annual_in_place_scheduled_rent, 0) AS gpr_in_place,
        COALESCE(u.non_revenue_unit_loss, 0) AS non_revenue_loss,
        COALESCE(u.annual_unit_ancillary_income, 0) + COALESCE(uw.monthly_other_property_income, 0) * 12 AS other_income,
        COALESCE(uw.physical_vacancy_pct, 0.05) AS vacancy_pct,
        COALESCE(uw.credit_loss_concessions_pct, 0.01) AS credit_loss_pct,
        COALESCE(uw.annual_property_taxes, 0) AS annual_property_taxes,
        COALESCE(uw.annual_insurance, 0) AS annual_insurance,
        COALESCE(uw.monthly_hoa_dues, 0) * 12 AS annual_hoa,
        COALESCE(uw.monthly_landlord_utilities, 0) * 12 AS annual_utilities,
        COALESCE(uw.monthly_cam_lawn_snow_pest, 0) * 12 AS annual_cam,
        COALESCE(uw.annual_admin_legal_cpa, 0) AS annual_admin_legal_cpa,
        COALESCE(uw.management_fee_pct_of_egi, 0.08) AS management_fee_pct,
        COALESCE(uw.repairs_maint_pct_of_egi, 0.05) AS repairs_maint_pct,
        COALESCE(uw.annual_repairs_per_unit, 0) * p.units AS fixed_repairs_reserve,
        COALESCE(uw.capex_reserve_pct_of_egi, 0.05) AS capex_reserve_pct,
        COALESCE(uw.annual_capex_per_unit, 0) * p.units AS fixed_capex_reserve,
        ROUND(((p.purchase_price * (1.0 - p.land_value_allocation_pct)) + p.closing_costs + p.initial_rehab_budget)
            / p.depreciation_schedule_years, 2) AS annual_depreciation,
        COALESCE(uw.marginal_income_tax_pct, 0.32) AS marginal_income_tax_pct
    FROM properties p
    LEFT JOIN legal_entities le ON le.id = p.legal_entity_id
    LEFT JOIN unit_rollup u ON u.property_id = p.id
    LEFT JOIN debt_rollup d ON d.property_id = p.id
    LEFT JOIN underwriting_models uw ON uw.property_id = p.id AND uw.is_primary_model
    WHERE p.is_active
),
income AS (
    SELECT
        b.*,
        GREATEST(b.gpr_market - b.gpr_in_place, 0) AS loss_to_lease,
        ROUND(b.gpr_market * b.vacancy_pct, 2) AS vacancy_loss,
        ROUND(b.gpr_market * b.credit_loss_pct, 2) AS credit_loss
    FROM base b
),
egi AS (
    SELECT
        i.*,
        i.gpr_market - i.vacancy_loss - i.credit_loss - i.non_revenue_loss + i.other_income AS effective_gross_income
    FROM income i
),
opex AS (
    SELECT
        e.*,
        ROUND(e.effective_gross_income * e.management_fee_pct, 2) AS annual_management_fee,
        ROUND(e.effective_gross_income * e.repairs_maint_pct + e.fixed_repairs_reserve, 2) AS annual_repairs_maint,
        ROUND(e.effective_gross_income * e.capex_reserve_pct + e.fixed_capex_reserve, 2) AS annual_capex_reserve
    FROM egi e
),
noi AS (
    SELECT
        o.*,
        o.annual_property_taxes + o.annual_insurance + o.annual_hoa + o.annual_utilities + o.annual_cam
            + o.annual_admin_legal_cpa + o.annual_management_fee + o.annual_repairs_maint AS total_operating_expenses
    FROM opex o
),
cash_flow AS (
    SELECT
        n.*,
        n.effective_gross_income - n.total_operating_expenses AS noi,
        n.effective_gross_income - n.total_operating_expenses - n.annual_capex_reserve - n.annual_debt_service AS levered_cash_flow_pre_tax,
        n.effective_gross_income - n.total_operating_expenses - n.annual_interest_expense - n.annual_depreciation AS taxable_net_income
    FROM noi n
)
SELECT
    property_id,
    user_id,
    legal_entity_id,
    legal_entity_name,
    property_name,
    category,
    street_address,
    city,
    state,
    total_units,
    occupied_units,
    rentable_sq_ft,

    -- Capital stack and valuation
    purchase_price,
    all_in_cost_basis,
    current_market_value,
    total_debt,
    current_market_value - total_debt AS total_equity,
    total_cash_invested,
    ROUND(purchase_price / NULLIF(total_units, 0), 2) AS price_per_door,
    ROUND(purchase_price / NULLIF(rentable_sq_ft, 0), 2) AS price_per_sq_ft,

    -- Annual cash flow waterfall
    gpr_market,
    gpr_in_place,
    loss_to_lease,
    vacancy_loss,
    credit_loss,
    non_revenue_loss,
    vacancy_loss + credit_loss + non_revenue_loss AS vacancy_and_credit_loss,
    other_income,
    effective_gross_income,
    annual_property_taxes,
    annual_insurance,
    annual_hoa,
    annual_utilities,
    annual_cam,
    annual_admin_legal_cpa,
    annual_management_fee,
    annual_repairs_maint,
    total_operating_expenses,
    noi,
    annual_capex_reserve,
    noi - annual_capex_reserve AS unlevered_cash_flow,
    annual_debt_service,
    levered_cash_flow_pre_tax,
    ROUND(levered_cash_flow_pre_tax / 12.0, 2) AS monthly_cash_flow,
    annual_interest_expense,
    annual_depreciation,
    taxable_net_income,
    ROUND(levered_cash_flow_pre_tax - taxable_net_income * marginal_income_tax_pct, 2) AS levered_cash_flow_after_tax,

    -- Return and risk ratios
    ROUND(noi / NULLIF(purchase_price, 0) * 100, 2) AS cap_rate_pct,
    ROUND(noi / NULLIF(all_in_cost_basis, 0) * 100, 2) AS yield_on_cost_pct,
    ROUND(levered_cash_flow_pre_tax / NULLIF(total_cash_invested, 0) * 100, 2) AS cash_on_cash_pct,
    ROUND(noi / NULLIF(annual_debt_service, 0), 2) AS dscr,
    ROUND(noi / NULLIF(total_debt, 0) * 100, 2) AS debt_yield_pct,
    ROUND(total_debt / NULLIF(current_market_value, 0) * 100, 2) AS ltv_pct,
    ROUND(total_operating_expenses / NULLIF(effective_gross_income, 0) * 100, 2) AS opex_ratio_pct,
    ROUND((total_operating_expenses + annual_debt_service) / NULLIF(effective_gross_income, 0) * 100, 2) AS break_even_occupancy_pct,
    ROUND(purchase_price / NULLIF(gpr_market, 0), 2) AS gross_rent_multiplier
FROM cash_flow;

-- Portfolio rollup over any subset of properties. Ratios are weighted: dollar numerators and
-- denominators are summed across properties first, never averaged. NULL filters match everything.
CREATE FUNCTION fn_portfolio_summary(
    p_user_id BIGINT DEFAULT NULL,
    p_categories property_category[] DEFAULT NULL,
    p_legal_entity_id BIGINT DEFAULT NULL,
    p_property_ids BIGINT[] DEFAULT NULL
) RETURNS TABLE (
    property_count BIGINT,
    total_units BIGINT,
    occupied_units BIGINT,
    rentable_sq_ft BIGINT,
    purchase_price NUMERIC,
    all_in_cost_basis NUMERIC,
    current_market_value NUMERIC,
    total_debt NUMERIC,
    total_equity NUMERIC,
    total_cash_invested NUMERIC,
    price_per_door NUMERIC,
    price_per_sq_ft NUMERIC,
    gpr_market NUMERIC,
    gpr_in_place NUMERIC,
    loss_to_lease NUMERIC,
    vacancy_loss NUMERIC,
    credit_loss NUMERIC,
    non_revenue_loss NUMERIC,
    vacancy_and_credit_loss NUMERIC,
    other_income NUMERIC,
    effective_gross_income NUMERIC,
    annual_property_taxes NUMERIC,
    annual_insurance NUMERIC,
    annual_hoa NUMERIC,
    annual_utilities NUMERIC,
    annual_cam NUMERIC,
    annual_admin_legal_cpa NUMERIC,
    annual_management_fee NUMERIC,
    annual_repairs_maint NUMERIC,
    total_operating_expenses NUMERIC,
    noi NUMERIC,
    annual_capex_reserve NUMERIC,
    unlevered_cash_flow NUMERIC,
    annual_debt_service NUMERIC,
    levered_cash_flow_pre_tax NUMERIC,
    monthly_cash_flow NUMERIC,
    annual_interest_expense NUMERIC,
    annual_depreciation NUMERIC,
    taxable_net_income NUMERIC,
    levered_cash_flow_after_tax NUMERIC,
    cap_rate_pct NUMERIC,
    yield_on_cost_pct NUMERIC,
    cash_on_cash_pct NUMERIC,
    dscr NUMERIC,
    debt_yield_pct NUMERIC,
    ltv_pct NUMERIC,
    opex_ratio_pct NUMERIC,
    break_even_occupancy_pct NUMERIC,
    gross_rent_multiplier NUMERIC
) AS $$
    SELECT
        COUNT(*),
        COALESCE(SUM(f.total_units), 0)::bigint,
        COALESCE(SUM(f.occupied_units), 0)::bigint,
        COALESCE(SUM(f.rentable_sq_ft), 0)::bigint,
        COALESCE(SUM(f.purchase_price), 0),
        COALESCE(SUM(f.all_in_cost_basis), 0),
        COALESCE(SUM(f.current_market_value), 0),
        COALESCE(SUM(f.total_debt), 0),
        COALESCE(SUM(f.total_equity), 0),
        COALESCE(SUM(f.total_cash_invested), 0),
        ROUND(SUM(f.purchase_price) / NULLIF(SUM(f.total_units), 0), 2),
        ROUND(SUM(f.purchase_price) / NULLIF(SUM(f.rentable_sq_ft), 0), 2),
        COALESCE(SUM(f.gpr_market), 0),
        COALESCE(SUM(f.gpr_in_place), 0),
        COALESCE(SUM(f.loss_to_lease), 0),
        COALESCE(SUM(f.vacancy_loss), 0),
        COALESCE(SUM(f.credit_loss), 0),
        COALESCE(SUM(f.non_revenue_loss), 0),
        COALESCE(SUM(f.vacancy_and_credit_loss), 0),
        COALESCE(SUM(f.other_income), 0),
        COALESCE(SUM(f.effective_gross_income), 0),
        COALESCE(SUM(f.annual_property_taxes), 0),
        COALESCE(SUM(f.annual_insurance), 0),
        COALESCE(SUM(f.annual_hoa), 0),
        COALESCE(SUM(f.annual_utilities), 0),
        COALESCE(SUM(f.annual_cam), 0),
        COALESCE(SUM(f.annual_admin_legal_cpa), 0),
        COALESCE(SUM(f.annual_management_fee), 0),
        COALESCE(SUM(f.annual_repairs_maint), 0),
        COALESCE(SUM(f.total_operating_expenses), 0),
        COALESCE(SUM(f.noi), 0),
        COALESCE(SUM(f.annual_capex_reserve), 0),
        COALESCE(SUM(f.unlevered_cash_flow), 0),
        COALESCE(SUM(f.annual_debt_service), 0),
        COALESCE(SUM(f.levered_cash_flow_pre_tax), 0),
        COALESCE(SUM(f.monthly_cash_flow), 0),
        COALESCE(SUM(f.annual_interest_expense), 0),
        COALESCE(SUM(f.annual_depreciation), 0),
        COALESCE(SUM(f.taxable_net_income), 0),
        COALESCE(SUM(f.levered_cash_flow_after_tax), 0),
        ROUND(SUM(f.noi) / NULLIF(SUM(f.purchase_price), 0) * 100, 2),
        ROUND(SUM(f.noi) / NULLIF(SUM(f.all_in_cost_basis), 0) * 100, 2),
        ROUND(SUM(f.levered_cash_flow_pre_tax) / NULLIF(SUM(f.total_cash_invested), 0) * 100, 2),
        ROUND(SUM(f.noi) / NULLIF(SUM(f.annual_debt_service), 0), 2),
        ROUND(SUM(f.noi) / NULLIF(SUM(f.total_debt), 0) * 100, 2),
        ROUND(SUM(f.total_debt) / NULLIF(SUM(f.current_market_value), 0) * 100, 2),
        ROUND(SUM(f.total_operating_expenses) / NULLIF(SUM(f.effective_gross_income), 0) * 100, 2),
        ROUND((SUM(f.total_operating_expenses) + SUM(f.annual_debt_service)) / NULLIF(SUM(f.effective_gross_income), 0) * 100, 2),
        ROUND(SUM(f.purchase_price) / NULLIF(SUM(f.gpr_market), 0), 2)
    FROM v_property_financial_analysis f
    WHERE (p_user_id IS NULL OR f.user_id = p_user_id)
      AND (p_categories IS NULL OR f.category = ANY(p_categories))
      AND (p_legal_entity_id IS NULL OR f.legal_entity_id = p_legal_entity_id)
      AND (p_property_ids IS NULL OR f.property_id = ANY(p_property_ids));
$$ LANGUAGE sql STABLE;

-- Whole-database rollup across every active property
CREATE VIEW v_portfolio_summary AS
SELECT * FROM fn_portfolio_summary();
