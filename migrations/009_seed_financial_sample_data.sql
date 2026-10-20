-- Migration 009: Sample financial portfolio for testing and display (optional, run manually)
--
-- Seeds 3 legal entities and 6 Chicago-area properties (SFR, condo, 2-flat, 4-flat, 6-unit and
-- mixed-use) with rent rolls, loans, pro forma models and 12 months of ledger actuals for each
-- test user: codynicholson96@gmail.com and developer@rixus.com by default, or only the user given
-- with -v owner_email. Emails with no user yet are skipped with a notice (register/log in first).
-- Requires migration 008 (the app applies it at startup).
-- Re-running replaces the previous sample rows for those users; other data is untouched.
-- Sample properties are tagged with a parcel_pin starting with 'SAMPLE-' and sample entities
-- with a name ending in '(Sample)'.
--
-- Usage:
--   docker exec -i rixus-be-db psql -U postgres -d rixus -v ON_ERROR_STOP=1 \
--     < migrations/009_seed_financial_sample_data.sql
--   # or for a single user:
--   docker exec -i rixus-be-db psql -U postgres -d rixus -v ON_ERROR_STOP=1 \
--     -v owner_email=you@example.com < migrations/009_seed_financial_sample_data.sql

\if :{?owner_email}
\else
\set owner_email 'codynicholson96@gmail.com,developer@rixus.com'
\endif

BEGIN;

SELECT set_config('seed.owner_emails', :'owner_email', true) \g /dev/null

DO $$
DECLARE
    v_email TEXT;
    v_seeded INT := 0;
    v_user_id BIGINT;
    v_holdco BIGINT;
    v_series BIGINT;
    v_personal BIGINT;
    v_prop BIGINT;
BEGIN
    FOREACH v_email IN ARRAY string_to_array(current_setting('seed.owner_emails'), ',') LOOP
        v_email := btrim(v_email);
        SELECT id INTO v_user_id FROM users WHERE email = v_email;
        IF v_user_id IS NULL THEN
            RAISE NOTICE 'skipping %: no user with that email', v_email;
            CONTINUE;
        END IF;
        v_seeded := v_seeded + 1;

        -- Remove a previous run (units, loans, models and ledger rows cascade)
        DELETE FROM properties WHERE user_id = v_user_id AND parcel_pin LIKE 'SAMPLE-%';
        DELETE FROM legal_entities WHERE user_id = v_user_id AND name LIKE '%(Sample)';

        INSERT INTO legal_entities (user_id, name, entity_type, bank_account_last4, credit_card_last4)
        VALUES (v_user_id, 'Bronzeville Holdings LLC (Sample)', 'HOLDING_LLC', '4821', '0937')
        RETURNING id INTO v_holdco;
        INSERT INTO legal_entities (user_id, name, entity_type, bank_account_last4, credit_card_last4)
        VALUES (v_user_id, 'Rixus Series LLC - Series A (Sample)', 'SERIES_LLC_CELL', '7714', NULL)
        RETURNING id INTO v_series;
        INSERT INTO legal_entities (user_id, name, entity_type, bank_account_last4, credit_card_last4)
        VALUES (v_user_id, 'Personal Ownership (Sample)', 'INDIVIDUAL', '3390', '5512')
        RETURNING id INTO v_personal;

        -- 1. Single-family home: classic SFR with an 80% LTV conventional loan
        INSERT INTO properties (user_id, legal_entity_id, name, category, street_address, city, state, zip, parcel_pin,
            year_built, units, gross_sq_ft, acquisition_date, purchase_price, closing_costs, initial_rehab_budget,
            initial_operating_reserve, after_repair_value, land_value_allocation_pct, depreciation_schedule_years, utility_config)
        VALUES (v_user_id, v_personal, 'Oak Park Bungalow', 'SINGLE_FAMILY', '1147 S Elmwood Ave', 'Oak Park', 'IL', '60304',
            'SAMPLE-16-17-215-008', 1924, 1, 1420, '2023-06-15', 315000, 7800, 22000, 6000, 365000, 0.2500, 27.5,
            '{"water": "TENANT", "electric": "TENANT", "gas": "TENANT", "trash": "LANDLORD"}')
        RETURNING id INTO v_prop;
        INSERT INTO units (property_id, unit_number, floor_plan_name, bedrooms, bathrooms, rentable_sq_ft, status, is_renovated,
            market_rent, in_place_rent, monthly_pet_rent, security_deposit, lease_start_date, lease_end_date)
        VALUES (v_prop, 'MAIN', '3BR/1.5BA Bungalow', 3, 1.5, 1420, 'OCCUPIED', TRUE, 2850, 2725, 35, 2725, '2026-07-01', '2027-06-30');
        INSERT INTO property_loans (property_id, lender_name, loan_type, original_loan_amount, current_principal_balance,
            interest_rate, amortization_months, loan_term_months, loan_origination_fees)
        VALUES (v_prop, 'Wintrust Mortgage', 'CONVENTIONAL', 252000, 245310.44, 0.06375, 360, 360, 2520);
        INSERT INTO underwriting_models (property_id, physical_vacancy_pct, credit_loss_concessions_pct, annual_property_taxes,
            annual_insurance, monthly_landlord_utilities, monthly_cam_lawn_snow_pest, annual_admin_legal_cpa,
            management_fee_pct_of_egi, repairs_maint_pct_of_egi, capex_reserve_pct_of_egi)
        VALUES (v_prop, 0.0400, 0.0050, 6900, 1450, 35, 40, 350, 0.0000, 0.0500, 0.0500);

        -- 2. Condo: HOA-heavy single unit with a slightly higher rate
        INSERT INTO properties (user_id, legal_entity_id, name, category, street_address, city, state, zip, parcel_pin,
            year_built, units, gross_sq_ft, acquisition_date, purchase_price, closing_costs, initial_rehab_budget,
            initial_operating_reserve, after_repair_value, land_value_allocation_pct, depreciation_schedule_years, utility_config)
        VALUES (v_user_id, v_series, 'South Loop Condo #1204', 'CONDO_TOWNHOME', '1400 S Michigan Ave Unit 1204', 'Chicago', 'IL', '60605',
            'SAMPLE-17-22-110-122-1204', 2007, 1, 1150, '2024-03-01', 268000, 6400, 4500, 5000, 280000, 0.1000, 27.5,
            '{"water": "LANDLORD", "electric": "TENANT", "gas": "LANDLORD", "trash": "LANDLORD"}')
        RETURNING id INTO v_prop;
        INSERT INTO units (property_id, unit_number, floor_plan_name, bedrooms, bathrooms, rentable_sq_ft, status, is_renovated,
            market_rent, in_place_rent, monthly_parking_storage, security_deposit, lease_start_date, lease_end_date)
        VALUES (v_prop, 'MAIN', '2BR/2BA Corner', 2, 2, 1150, 'OCCUPIED', FALSE, 2950, 2900, 225, 2900, '2026-04-01', '2027-03-31');
        INSERT INTO property_loans (property_id, lender_name, loan_type, original_loan_amount, current_principal_balance,
            interest_rate, amortization_months, loan_term_months, loan_origination_fees)
        VALUES (v_prop, 'Chase Home Lending', 'CONVENTIONAL', 201000, 198102.67, 0.06875, 360, 360, 2010);
        INSERT INTO underwriting_models (property_id, physical_vacancy_pct, credit_loss_concessions_pct, annual_property_taxes,
            annual_insurance, monthly_hoa_dues, annual_admin_legal_cpa, management_fee_pct_of_egi, repairs_maint_pct_of_egi,
            capex_reserve_pct_of_egi)
        VALUES (v_prop, 0.0400, 0.0050, 5200, 650, 640, 350, 0.0800, 0.0300, 0.0200);

        -- 3. Logan Square 2-flat: owner-style 2-unit with RUBS and one unit below market
        INSERT INTO properties (user_id, legal_entity_id, name, category, street_address, city, state, zip, parcel_pin,
            year_built, units, gross_sq_ft, acquisition_date, purchase_price, closing_costs, initial_rehab_budget,
            initial_operating_reserve, after_repair_value, land_value_allocation_pct, depreciation_schedule_years, utility_config)
        VALUES (v_user_id, v_holdco, 'Logan Square 2-Flat', 'MULTI_FAMILY_2_4', '2618 N Spaulding Ave', 'Chicago', 'IL', '60647',
            'SAMPLE-13-26-311-019', 1912, 2, 2400, '2022-09-30', 520000, 11200, 38000, 10000, 615000, 0.3000, 27.5,
            '{"water": "LANDLORD", "electric": "TENANT", "gas": "TENANT", "trash": "LANDLORD"}')
        RETURNING id INTO v_prop;
        INSERT INTO units (property_id, unit_number, floor_plan_name, bedrooms, bathrooms, rentable_sq_ft, status, is_renovated,
            market_rent, in_place_rent, monthly_rubs_charge, monthly_pet_rent, security_deposit, lease_start_date, lease_end_date)
        VALUES
            (v_prop, '1', '3BR/1BA Garden', 3, 1, 1150, 'OCCUPIED', FALSE, 2650, 2350, 85, 0, 2350, '2025-11-01', '2026-10-31'),
            (v_prop, '2', '3BR/2BA Renovated', 3, 2, 1250, 'OCCUPIED', TRUE, 2900, 2875, 85, 40, 2875, '2026-06-01', '2027-05-31');
        INSERT INTO property_loans (property_id, lender_name, loan_type, original_loan_amount, current_principal_balance,
            interest_rate, amortization_months, loan_term_months, loan_origination_fees)
        VALUES (v_prop, 'Old Second National Bank', 'CONVENTIONAL', 416000, 398540.12, 0.05250, 360, 360, 4160);
        INSERT INTO underwriting_models (property_id, physical_vacancy_pct, credit_loss_concessions_pct, annual_property_taxes,
            annual_insurance, monthly_landlord_utilities, monthly_cam_lawn_snow_pest, annual_admin_legal_cpa,
            management_fee_pct_of_egi, repairs_maint_pct_of_egi, annual_repairs_per_unit, capex_reserve_pct_of_egi, annual_capex_per_unit)
        VALUES (v_prop, 0.0500, 0.0100, 9800, 2600, 190, 85, 600, 0.0800, 0.0400, 250, 0.0300, 300);

        -- 4. Bronzeville greystone 4-flat: value-add with one unit down for renovation and coin laundry
        INSERT INTO properties (user_id, legal_entity_id, name, category, street_address, city, state, zip, parcel_pin,
            year_built, units, gross_sq_ft, acquisition_date, purchase_price, closing_costs, initial_rehab_budget,
            initial_operating_reserve, after_repair_value, land_value_allocation_pct, depreciation_schedule_years, utility_config)
        VALUES (v_user_id, v_holdco, 'Bronzeville Greystone 4-Flat', 'MULTI_FAMILY_2_4', '4419 S King Dr', 'Chicago', 'IL', '60653',
            'SAMPLE-20-03-208-031', 1898, 4, 4600, '2025-02-14', 585000, 14600, 95000, 20000, 760000, 0.2000, 27.5,
            '{"water": "LANDLORD", "electric": "TENANT", "gas": "TENANT", "trash": "LANDLORD"}')
        RETURNING id INTO v_prop;
        INSERT INTO units (property_id, unit_number, floor_plan_name, bedrooms, bathrooms, rentable_sq_ft, status, is_renovated,
            market_rent, in_place_rent, monthly_rubs_charge, monthly_pet_rent, monthly_parking_storage, security_deposit,
            lease_start_date, lease_end_date)
        VALUES
            (v_prop, '1F', '3BR/1BA Classic', 3, 1, 1150, 'OCCUPIED', FALSE, 2200, 1850, 75, 0, 0, 1850, '2025-03-01', '2027-02-28'),
            (v_prop, '1R', '3BR/1BA Classic', 3, 1, 1150, 'OCCUPIED', FALSE, 2200, 1925, 75, 30, 0, 1925, '2025-08-01', '2026-07-31'),
            (v_prop, '2F', '3BR/2BA Renovated', 3, 2, 1150, 'OCCUPIED', TRUE, 2450, 2450, 75, 0, 75, 2450, '2026-05-01', '2027-04-30'),
            (v_prop, '2R', '3BR/2BA Renovated', 3, 2, 1150, 'VACANT_RENOVATION', TRUE, 2450, 0, 0, 0, 0, 0, NULL, NULL);
        INSERT INTO property_loans (property_id, lender_name, loan_type, original_loan_amount, current_principal_balance,
            interest_rate, amortization_months, loan_term_months, loan_origination_fees)
        VALUES (v_prop, 'Byline Bank', 'COMMERCIAL', 438750, 433112.58, 0.06750, 300, 60, 6581);
        INSERT INTO underwriting_models (property_id, physical_vacancy_pct, credit_loss_concessions_pct, monthly_other_property_income,
            annual_property_taxes, annual_insurance, monthly_landlord_utilities, monthly_cam_lawn_snow_pest, annual_admin_legal_cpa,
            management_fee_pct_of_egi, repairs_maint_pct_of_egi, annual_repairs_per_unit, capex_reserve_pct_of_egi, annual_capex_per_unit)
        VALUES (v_prop, 0.0700, 0.0150, 140, 11200, 4800, 420, 150, 900, 0.0800, 0.0300, 400, 0.0200, 350);

        -- 5. Pilsen 6-unit: 5+ unit commercial loan stacked with a 0% seller carry note
        INSERT INTO properties (user_id, legal_entity_id, name, category, street_address, city, state, zip, parcel_pin,
            year_built, units, gross_sq_ft, acquisition_date, purchase_price, closing_costs, initial_rehab_budget,
            initial_operating_reserve, after_repair_value, land_value_allocation_pct, depreciation_schedule_years, utility_config)
        VALUES (v_user_id, v_series, 'Pilsen 6-Unit', 'MULTI_FAMILY_5_PLUS', '1837 S Throop St', 'Chicago', 'IL', '60608',
            'SAMPLE-17-20-427-014', 1905, 6, 5700, '2024-08-20', 840000, 18900, 60000, 30000, 965000, 0.2500, 27.5,
            '{"water": "LANDLORD", "electric": "TENANT", "gas": "LANDLORD", "trash": "LANDLORD"}')
        RETURNING id INTO v_prop;
        INSERT INTO units (property_id, unit_number, floor_plan_name, bedrooms, bathrooms, rentable_sq_ft, status, is_renovated,
            market_rent, in_place_rent, monthly_rubs_charge, monthly_pet_rent, monthly_parking_storage, security_deposit,
            lease_start_date, lease_end_date)
        VALUES
            (v_prop, 'G', '1BR/1BA Garden', 1, 1, 650, 'OCCUPIED', FALSE, 1350, 1275, 60, 0, 0, 1275, '2026-01-01', '2026-12-31'),
            (v_prop, '1F', '2BR/1BA Classic', 2, 1, 950, 'OCCUPIED', TRUE, 1850, 1825, 70, 25, 0, 1825, '2026-03-01', '2027-02-28'),
            (v_prop, '1R', '2BR/1BA Classic', 2, 1, 950, 'OCCUPIED', FALSE, 1850, 1650, 70, 0, 0, 1650, '2025-09-01', '2026-11-30'),
            (v_prop, '2F', '3BR/1BA Classic', 3, 1, 1050, 'OCCUPIED', TRUE, 2150, 2150, 80, 0, 100, 2150, '2026-08-01', '2027-07-31'),
            (v_prop, '2R', '3BR/1BA Classic', 3, 1, 1050, 'OCCUPIED', FALSE, 2150, 1900, 80, 35, 0, 1900, '2025-06-01', '2027-05-31'),
            (v_prop, 'COACH', '1BR/1BA Coach House', 1, 1, 1050, 'VACANT_READY', TRUE, 1500, 0, 0, 0, 0, 0, NULL, NULL);
        INSERT INTO property_loans (property_id, lender_name, loan_type, original_loan_amount, current_principal_balance,
            interest_rate, amortization_months, loan_term_months, loan_origination_fees)
        VALUES
            (v_prop, 'First Midwest Bank', 'COMMERCIAL', 588000, 579944.31, 0.06875, 300, 60, 8820),
            (v_prop, 'Seller: M. Alvarez Trust', 'SELLER', 84000, 77000.00, 0.00000, 120, 60, 0);
        INSERT INTO underwriting_models (property_id, physical_vacancy_pct, credit_loss_concessions_pct, monthly_other_property_income,
            annual_property_taxes, annual_insurance, monthly_landlord_utilities, monthly_cam_lawn_snow_pest, annual_admin_legal_cpa,
            management_fee_pct_of_egi, repairs_maint_pct_of_egi, annual_repairs_per_unit, capex_reserve_pct_of_egi, annual_capex_per_unit)
        VALUES (v_prop, 0.0600, 0.0150, 95, 14800, 6900, 780, 210, 1500, 0.0700, 0.0300, 450, 0.0200, 350);

        -- 6. Humboldt Park mixed-use: storefront + 2 apartments on interest-only bridge debt, 39-year schedule
        INSERT INTO properties (user_id, legal_entity_id, name, category, street_address, city, state, zip, parcel_pin,
            year_built, units, gross_sq_ft, acquisition_date, purchase_price, closing_costs, initial_rehab_budget,
            initial_operating_reserve, after_repair_value, land_value_allocation_pct, depreciation_schedule_years, utility_config)
        VALUES (v_user_id, v_holdco, 'Humboldt Park Mixed-Use', 'MIXED_USE', '2734 W Division St', 'Chicago', 'IL', '60622',
            'SAMPLE-16-01-421-027', 1915, 3, 3900, '2026-01-10', 690000, 17250, 70000, 25000, 790000, 0.2500, 39.0,
            '{"water": "LANDLORD", "electric": "TENANT", "gas": "TENANT", "trash": "TENANT"}')
        RETURNING id INTO v_prop;
        INSERT INTO units (property_id, unit_number, floor_plan_name, bedrooms, bathrooms, rentable_sq_ft, status, is_renovated,
            market_rent, in_place_rent, monthly_rubs_charge, monthly_pet_rent, security_deposit, lease_start_date, lease_end_date)
        VALUES
            (v_prop, 'RETAIL', 'Ground-Floor Storefront', 0, 1, 1500, 'OCCUPIED', FALSE, 3400, 3100, 0, 0, 6200, '2024-05-01', '2029-04-30'),
            (v_prop, '2', '3BR/2BA Renovated', 3, 2, 1200, 'OCCUPIED', TRUE, 2500, 2500, 70, 0, 2500, '2026-04-01', '2027-03-31'),
            (v_prop, '3', '3BR/2BA Renovated', 3, 2, 1200, 'NON_REVENUE_MODEL', TRUE, 2500, 0, 0, 0, 0, NULL, NULL);
        INSERT INTO property_loans (property_id, lender_name, loan_type, original_loan_amount, current_principal_balance,
            interest_rate, amortization_months, loan_term_months, interest_only_months, loan_origination_fees)
        VALUES (v_prop, 'Lima One Capital', 'BRIDGE', 552000, 552000.00, 0.08250, 360, 24, 24, 11040);
        INSERT INTO underwriting_models (property_id, physical_vacancy_pct, credit_loss_concessions_pct, annual_property_taxes,
            annual_insurance, monthly_landlord_utilities, monthly_cam_lawn_snow_pest, annual_admin_legal_cpa,
            management_fee_pct_of_egi, repairs_maint_pct_of_egi, annual_repairs_per_unit, capex_reserve_pct_of_egi,
            annual_capex_per_unit, exit_cap_rate_pct)
        VALUES (v_prop, 0.0800, 0.0150, 13900, 5400, 160, 120, 1200, 0.0800, 0.0300, 300, 0.0300, 300, 0.0725);

        -- 12 months of actuals (Oct 2025 - Sep 2026), derived from each property's rent roll, loans and model.
        -- Variation is deterministic so re-runs produce the same ledger.

        -- Unit-level rent and ancillary income for occupied units, skipping months before acquisition
        INSERT INTO ledger_transactions (property_id, unit_id, transaction_date, category, amount, vendor_or_payor, description)
        SELECT u.property_id, u.id, m.d + 1, c.category, c.amount, 'Tenant ' || u.unit_number, c.description
        FROM units u
        JOIN properties p ON p.id = u.property_id
        CROSS JOIN generate_series(DATE '2025-10-01', DATE '2026-09-01', INTERVAL '1 month') AS g(ts)
        CROSS JOIN LATERAL (SELECT g.ts::date AS d) m
        CROSS JOIN LATERAL (VALUES
            ('GROSS_RENT'::ledger_category, u.in_place_rent, 'Monthly rent'),
            ('RUBS_REIMBURSEMENT'::ledger_category, u.monthly_rubs_charge, 'Utility bill-back (RUBS)'),
            ('PET_RENT'::ledger_category, u.monthly_pet_rent, 'Pet rent'),
            ('PARKING_STORAGE'::ledger_category, u.monthly_parking_storage, 'Parking / storage')
        ) AS c(category, amount, description)
        WHERE p.user_id = v_user_id AND p.parcel_pin LIKE 'SAMPLE-%'
          AND u.status = 'OCCUPIED' AND c.amount > 0 AND m.d >= p.acquisition_date;

        -- Property-level income and expenses
        INSERT INTO ledger_transactions (property_id, transaction_date, category, amount, vendor_or_payor, description)
        SELECT x.property_id, x.d, x.category, ROUND(x.amount, 2), x.vendor, x.description
        FROM (
            SELECT p.id AS property_id, m.d, m.n, uw.monthly_other_property_income, uw.management_fee_pct_of_egi,
                   uw.monthly_hoa_dues, uw.monthly_landlord_utilities, uw.monthly_cam_lawn_snow_pest, uw.repairs_maint_pct_of_egi,
                   uw.annual_repairs_per_unit, uw.annual_property_taxes, uw.annual_insurance, uw.annual_admin_legal_cpa,
                   uw.credit_loss_concessions_pct,
                   (SELECT COALESCE(SUM(in_place_rent), 0) FROM units WHERE property_id = p.id AND status = 'OCCUPIED') AS collected_rent
            FROM properties p
            JOIN underwriting_models uw ON uw.property_id = p.id AND uw.is_primary_model
            CROSS JOIN LATERAL (
                SELECT g.ts::date AS d, (row_number() OVER ())::int AS n
                FROM generate_series(DATE '2025-10-01', DATE '2026-09-01', INTERVAL '1 month') AS g(ts)
            ) m
            WHERE p.user_id = v_user_id AND p.parcel_pin LIKE 'SAMPLE-%' AND m.d >= p.acquisition_date
        ) s
        CROSS JOIN LATERAL (VALUES
            (s.property_id, s.d + 4, 'LAUNDRY_VENDING'::ledger_category, s.monthly_other_property_income * (0.85 + (s.n % 4) * 0.1), 'CSC ServiceWorks', 'Coin laundry / vending'),
            (s.property_id, s.d + 9, 'PROPERTY_MANAGEMENT'::ledger_category, s.collected_rent * s.management_fee_pct_of_egi, 'Rixus OpCo Management', 'Management fee'),
            (s.property_id, s.d + 14, 'HOA_DUES'::ledger_category, s.monthly_hoa_dues, 'Association', 'HOA dues'),
            (s.property_id, s.d + 17, 'UTILITIES_WATER_SEWER'::ledger_category, s.monthly_landlord_utilities * 0.7 * (0.9 + (s.n % 3) * 0.1), 'City of Chicago Water', 'Water / sewer'),
            (s.property_id, s.d + 17, 'UTILITIES_TRASH'::ledger_category, s.monthly_landlord_utilities * 0.3, 'Waste Management', 'Trash service'),
            (s.property_id, s.d + 20, 'CAM_LAWN_SNOW_PEST'::ledger_category,
                s.monthly_cam_lawn_snow_pest * CASE WHEN EXTRACT(MONTH FROM s.d) IN (12, 1, 2) THEN 1.6 ELSE 0.8 END, 'Northside Grounds', 'Lawn / snow / pest'),
            (s.property_id, s.d + 22, 'REPAIRS_MAINTENANCE'::ledger_category,
                CASE WHEN (s.n + s.property_id) % 3 = 0 THEN s.collected_rent * 12 * s.repairs_maint_pct_of_egi / 4 + s.annual_repairs_per_unit ELSE 0 END,
                'Handyman Pros', 'Repairs and maintenance'),
            (s.property_id, s.d + 2, 'PROPERTY_TAX'::ledger_category,
                CASE WHEN EXTRACT(MONTH FROM s.d) = 3 THEN s.annual_property_taxes * 0.55
                     WHEN EXTRACT(MONTH FROM s.d) = 8 THEN s.annual_property_taxes * 0.45 ELSE 0 END, 'Cook County Treasurer', 'Property tax installment'),
            (s.property_id, s.d + 5, 'INSURANCE'::ledger_category,
                CASE WHEN s.n = 1 + (s.property_id % 12) THEN s.annual_insurance ELSE 0 END, 'Erie Insurance', 'Annual premium'),
            (s.property_id, s.d + 11, 'ADMIN_LEGAL_CPA'::ledger_category,
                CASE WHEN EXTRACT(MONTH FROM s.d) = 4 THEN s.annual_admin_legal_cpa * 0.7 WHEN EXTRACT(MONTH FROM s.d) = 1 THEN s.annual_admin_legal_cpa * 0.3 ELSE 0 END,
                'Loop CPA Group', 'Tax prep / entity filings'),
            (s.property_id, s.d + 26, 'TURNOVER_MAKE_READY'::ledger_category,
                CASE WHEN s.n = 1 + ((s.property_id * 5) % 12) THEN 650 + (s.property_id % 4) * 300 ELSE 0 END, 'Spotless Turns', 'Unit turnover'),
            (s.property_id, s.d + 26, 'CAPEX_REPLACEMENT'::ledger_category,
                CASE WHEN s.n = 1 + ((s.property_id * 7) % 12) THEN 1800 + (s.property_id % 3) * 1400 ELSE 0 END, 'ABC Plumbing & HVAC', 'Water heater / HVAC replacement'),
            (s.property_id, s.d + 27, 'CONCESSIONS_BAD_DEBT'::ledger_category,
                CASE WHEN s.n = 1 + ((s.property_id * 3) % 12) THEN s.collected_rent * s.credit_loss_concessions_pct * 6 ELSE 0 END, NULL, 'Concession / write-off')
        ) AS x(property_id, d, category, amount, vendor, description)
        WHERE x.amount > 0;

        -- Monthly debt service split into interest and principal on the current balance
        INSERT INTO ledger_transactions (property_id, transaction_date, category, amount, vendor_or_payor, description)
        SELECT l.property_id, m.d, c.category, c.amount, l.lender_name, c.description
        FROM property_loans l
        JOIN properties p ON p.id = l.property_id
        CROSS JOIN generate_series(DATE '2025-10-01', DATE '2026-09-01', INTERVAL '1 month') AS g(ts)
        CROSS JOIN LATERAL (SELECT g.ts::date AS d) m
        CROSS JOIN LATERAL (SELECT
            fn_monthly_debt_service(l.original_loan_amount, l.interest_rate, l.amortization_months, l.interest_only_months) AS payment,
            ROUND(l.current_principal_balance * l.interest_rate / 12, 2) AS interest
        ) pay
        CROSS JOIN LATERAL (VALUES
            ('DEBT_INTEREST'::ledger_category, pay.interest, 'Mortgage interest'),
            ('DEBT_PRINCIPAL'::ledger_category, pay.payment - pay.interest, 'Mortgage principal')
        ) AS c(category, amount, description)
        WHERE p.user_id = v_user_id AND p.parcel_pin LIKE 'SAMPLE-%' AND l.is_active
          AND c.amount > 0 AND m.d >= p.acquisition_date;
    END LOOP;

    IF v_seeded = 0 THEN
        RAISE EXCEPTION 'no matching users for %', current_setting('seed.owner_emails');
    END IF;
END $$;

COMMIT;
