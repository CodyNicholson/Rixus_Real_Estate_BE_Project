-- Migration 011: Seed comprehensive testing data for user developer@rixus.com
--
-- This script seeds a complete, production-grade real estate testing portfolio:
--   1. User record for developer@rixus.com with ADMIN role
--   2. 4 Legal Entities (Holding LLC, Series LLC cell, Equity Fund, Direct Ownership)
--   3. 10 Properties across all 5 asset classes (Multi-Family 2-4, Multi-Family 5+,
--      Mixed-Use, Single Family, Condo/Townhome) including 1 active excluded property
--      and 1 property with Unassigned legal entity.
--   4. 32 Rent roll units spanning all unit statuses (OCCUPIED, VACANT_READY,
--      VACANT_RENOVATION, NON_REVENUE_MODEL) with market vs in-place rent, RUBS,
--      pet rent, parking/storage, security deposits, and staggered lease terms.
--   5. 27 Tenant records (26 active + 1 historical move-out) with initials, unit numbers,
--      rent, pet counts, move-in/out dates, lease dates, and security deposits.
--   6. Diverse debt facilities: 30-yr conventional, commercial amortizing, commercial
--      interest-only bridge, seller-financing 2nd note, PMI loan, hard money IO,
--      and a free & clear property with $0 debt.
--   7. Full Pro Forma Underwriting models with property taxes, insurance, management fees,
--      utilities, CAM/snow, repairs, CapEx reserves, and cap rates.
--   8. 12 Months of actual ledger transactions (Oct 2025 - Sep 2026) for gross rents,
--      RUBS, pet rent, parking, laundry/vending, operating expenses, taxes, and debt service.
--   9. 8 Multimedia and document records (PDF leases, templates, inspection reports,
--      architectural floorplans, and property photos) with valid binary BYTEA content.

DO $$
DECLARE
    v_user_id BIGINT;
    v_holdco BIGINT;
    v_series BIGINT;
    v_fund BIGINT;
    v_personal BIGINT;
    v_prop BIGINT;
    v_u1 BIGINT;
    v_u2 BIGINT;
    v_u3 BIGINT;
    v_u4 BIGINT;
    v_u5 BIGINT;
    v_u6 BIGINT;
    v_u7 BIGINT;
    v_u8 BIGINT;
    v_doc_pdf BYTEA;
    v_doc_jpg BYTEA;
BEGIN
    -- 1. Ensure user developer@rixus.com exists with ADMIN permissions
    SELECT id INTO v_user_id FROM users WHERE email = 'developer@rixus.com';
    IF v_user_id IS NULL THEN
        INSERT INTO users (email, name, user_role)
        VALUES ('developer@rixus.com', 'Cody (Developer)', 'ADMIN')
        RETURNING id INTO v_user_id;
        RAISE NOTICE 'Created new user developer@rixus.com (id: %)', v_user_id;
    ELSE
        UPDATE users SET user_role = 'ADMIN', name = 'Cody (Developer)' WHERE id = v_user_id;
        RAISE NOTICE 'Found existing user developer@rixus.com (id: %)', v_user_id;
    END IF;

    -- 2. Clean up previous test data for this user to make script 100% idempotent
    DELETE FROM ledger_transactions WHERE property_id IN (SELECT id FROM properties WHERE user_id = v_user_id);
    DELETE FROM underwriting_models WHERE property_id IN (SELECT id FROM properties WHERE user_id = v_user_id);
    DELETE FROM property_loans WHERE property_id IN (SELECT id FROM properties WHERE user_id = v_user_id);
    DELETE FROM tenants WHERE property_id IN (SELECT id FROM properties WHERE user_id = v_user_id);
    DELETE FROM units WHERE property_id IN (SELECT id FROM properties WHERE user_id = v_user_id);
    DELETE FROM multimedia WHERE user_id = v_user_id;
    DELETE FROM properties WHERE user_id = v_user_id;
    DELETE FROM legal_entities WHERE user_id = v_user_id;

    -- 3. Legal Entities
    INSERT INTO legal_entities (user_id, name, entity_type, bank_account_last4, credit_card_last4)
    VALUES (v_user_id, 'Rixus Apex Holdings LLC', 'HOLDING_LLC', '4821', '0937')
    RETURNING id INTO v_holdco;

    INSERT INTO legal_entities (user_id, name, entity_type, bank_account_last4, credit_card_last4)
    VALUES (v_user_id, 'Rixus Series LLC - Fulton Market Cell', 'SERIES_LLC_CELL', '7714', NULL)
    RETURNING id INTO v_series;

    INSERT INTO legal_entities (user_id, name, entity_type, bank_account_last4, credit_card_last4)
    VALUES (v_user_id, 'Rixus Northside Equity Fund LLC', 'HOLDING_LLC', '6109', '2241')
    RETURNING id INTO v_fund;

    INSERT INTO legal_entities (user_id, name, entity_type, bank_account_last4, credit_card_last4)
    VALUES (v_user_id, 'Developer Direct Ownership', 'INDIVIDUAL', '3390', '5512')
    RETURNING id INTO v_personal;

    -- =========================================================================
    -- PROPERTY 1: Lincoln Park Luxury 3-Flat (Multi-Family 2-4)
    -- =========================================================================
    INSERT INTO properties (user_id, legal_entity_id, name, category, street_address, city, state, zip, parcel_pin,
        year_built, units, gross_sq_ft, acquisition_date, purchase_price, closing_costs, initial_rehab_budget,
        initial_operating_reserve, after_repair_value, land_value_allocation_pct, depreciation_schedule_years, utility_config)
    VALUES (v_user_id, v_holdco, 'Lincoln Park Luxury 3-Flat', 'MULTI_FAMILY_2_4', '2124 N Kenmore Ave', 'Chicago', 'IL', '60614',
        'DEV-14-32-108-015', 1910, 3, 3850, '2023-04-15', 1250000, 22000, 65000, 25000, 1450000, 0.2500, 27.5,
        '{"water": "LANDLORD", "electric": "TENANT", "gas": "TENANT", "trash": "LANDLORD"}')
    RETURNING id INTO v_prop;

    INSERT INTO units (property_id, unit_number, floor_plan_name, bedrooms, bathrooms, rentable_sq_ft, status, is_renovated,
        market_rent, in_place_rent, monthly_rubs_charge, monthly_pet_rent, monthly_parking_storage, security_deposit, lease_start_date, lease_end_date)
    VALUES
        (v_prop, '1', '3BR/2BA Garden Suite', 3, 2, 1200, 'OCCUPIED', TRUE, 2800, 2750, 75, 35, 0, 2750, '2026-06-01', '2027-05-31'),
        (v_prop, '2', '3BR/2BA Duplex Down', 3, 2, 1350, 'OCCUPIED', TRUE, 3200, 3200, 85, 0, 150, 3200, '2026-04-01', '2027-03-31'),
        (v_prop, '3', '3BR/2BA Penthouse', 3, 2, 1300, 'OCCUPIED', TRUE, 3400, 3350, 85, 40, 150, 3350, '2025-08-01', '2026-07-31');

    INSERT INTO tenants (tenant_initials, property_id, unit_number, monthly_rent, pet_count, move_in_date, move_out_date, lease_start_date, lease_end_date, security_deposit)
    VALUES
        ('MK', v_prop, 1, 2750, 1, '2025-06-01 00:00:00Z', NULL, '2026-06-01 00:00:00Z', '2027-05-31 00:00:00Z', 2750),
        ('CR', v_prop, 2, 3200, 0, '2024-04-01 00:00:00Z', NULL, '2026-04-01 00:00:00Z', '2027-03-31 00:00:00Z', 3200),
        ('DL', v_prop, 3, 3350, 1, '2023-08-01 00:00:00Z', NULL, '2025-08-01 00:00:00Z', '2026-07-31 00:00:00Z', 3350);

    INSERT INTO property_loans (property_id, lender_name, loan_type, original_loan_amount, current_principal_balance,
        interest_rate, amortization_months, loan_term_months, loan_origination_fees)
    VALUES (v_prop, 'First American Bank', 'CONVENTIONAL', 1000000, 974250.00, 0.06250, 360, 360, 10000);

    INSERT INTO underwriting_models (property_id, physical_vacancy_pct, credit_loss_concessions_pct, annual_property_taxes,
        annual_insurance, monthly_landlord_utilities, monthly_cam_lawn_snow_pest, annual_admin_legal_cpa,
        management_fee_pct_of_egi, repairs_maint_pct_of_egi, capex_reserve_pct_of_egi)
    VALUES (v_prop, 0.0400, 0.0050, 14200, 3600, 180, 90, 650, 0.0600, 0.0400, 0.0300);

    -- =========================================================================
    -- PROPERTY 2: Wicker Park Modern 4-Flat (Multi-Family 2-4)
    -- =========================================================================
    INSERT INTO properties (user_id, legal_entity_id, name, category, street_address, city, state, zip, parcel_pin,
        year_built, units, gross_sq_ft, acquisition_date, purchase_price, closing_costs, initial_rehab_budget,
        initial_operating_reserve, after_repair_value, land_value_allocation_pct, depreciation_schedule_years, utility_config)
    VALUES (v_user_id, v_holdco, 'Wicker Park Modern 4-Flat', 'MULTI_FAMILY_2_4', '1432 N Milwaukee Ave', 'Chicago', 'IL', '60622',
        'DEV-17-06-214-022', 1920, 4, 4600, '2024-01-10', 980000, 18500, 45000, 18000, 1150000, 0.2200, 27.5,
        '{"water": "LANDLORD", "electric": "TENANT", "gas": "TENANT", "trash": "LANDLORD"}')
    RETURNING id INTO v_prop;

    INSERT INTO units (property_id, unit_number, floor_plan_name, bedrooms, bathrooms, rentable_sq_ft, status, is_renovated,
        market_rent, in_place_rent, monthly_rubs_charge, monthly_pet_rent, monthly_parking_storage, security_deposit, lease_start_date, lease_end_date)
    VALUES
        (v_prop, '1', '2BR/1BA Classic Front', 2, 1, 1100, 'OCCUPIED', FALSE, 2250, 2100, 70, 0, 0, 2100, '2025-09-01', '2026-08-31'),
        (v_prop, '2', '2BR/1BA Classic Rear', 2, 1, 1100, 'OCCUPIED', FALSE, 2250, 2150, 70, 30, 0, 2150, '2026-02-01', '2027-01-31'),
        (v_prop, '3', '2BR/1.5BA Renovated', 2, 1.5, 1150, 'OCCUPIED', TRUE, 2500, 2500, 75, 0, 100, 2500, '2026-05-01', '2027-04-30'),
        (v_prop, '4', '2BR/1.5BA Renovated Top', 2, 1.5, 1150, 'VACANT_READY', TRUE, 2500, 0, 0, 0, 0, 0, NULL, NULL);

    INSERT INTO tenants (tenant_initials, property_id, unit_number, monthly_rent, pet_count, move_in_date, move_out_date, lease_start_date, lease_end_date, security_deposit)
    VALUES
        ('TB', v_prop, 1, 2100, 0, '2024-09-01 00:00:00Z', NULL, '2025-09-01 00:00:00Z', '2026-08-31 00:00:00Z', 2100),
        ('AJ', v_prop, 2, 2150, 1, '2025-02-01 00:00:00Z', NULL, '2026-02-01 00:00:00Z', '2027-01-31 00:00:00Z', 2150),
        ('NP', v_prop, 3, 2500, 0, '2025-05-01 00:00:00Z', NULL, '2026-05-01 00:00:00Z', '2027-04-30 00:00:00Z', 2500),
        ('EW', v_prop, 4, 2200, 0, '2024-03-01 00:00:00Z', '2026-04-30 00:00:00Z', '2025-04-01 00:00:00Z', '2026-04-30 00:00:00Z', 2200);

    INSERT INTO property_loans (property_id, lender_name, loan_type, original_loan_amount, current_principal_balance,
        interest_rate, amortization_months, loan_term_months, loan_origination_fees)
    VALUES (v_prop, 'Wintrust Commercial Banking', 'COMMERCIAL', 784000, 768400.00, 0.06500, 300, 60, 7840);

    INSERT INTO underwriting_models (property_id, physical_vacancy_pct, credit_loss_concessions_pct, monthly_other_property_income,
        annual_property_taxes, annual_insurance, monthly_landlord_utilities, monthly_cam_lawn_snow_pest, annual_admin_legal_cpa,
        management_fee_pct_of_egi, repairs_maint_pct_of_egi, capex_reserve_pct_of_egi)
    VALUES (v_prop, 0.0500, 0.0100, 120, 11800, 3200, 310, 120, 750, 0.0700, 0.0400, 0.0300);

    -- =========================================================================
    -- PROPERTY 3: Fulton Market Mixed-Use Commercial & Lofts (Mixed-Use)
    -- =========================================================================
    INSERT INTO properties (user_id, legal_entity_id, name, category, street_address, city, state, zip, parcel_pin,
        year_built, units, gross_sq_ft, acquisition_date, purchase_price, closing_costs, initial_rehab_budget,
        initial_operating_reserve, after_repair_value, land_value_allocation_pct, depreciation_schedule_years, utility_config)
    VALUES (v_user_id, v_series, 'Fulton Market Mixed-Use', 'MIXED_USE', '935 W Randolph St', 'Chicago', 'IL', '60607',
        'DEV-17-08-410-019', 1915, 3, 4500, '2025-03-15', 1650000, 32000, 120000, 35000, 1900000, 0.2500, 39.0,
        '{"water": "LANDLORD", "electric": "TENANT", "gas": "TENANT", "trash": "TENANT"}')
    RETURNING id INTO v_prop;

    INSERT INTO units (property_id, unit_number, floor_plan_name, bedrooms, bathrooms, rentable_sq_ft, status, is_renovated,
        market_rent, in_place_rent, monthly_rubs_charge, monthly_pet_rent, security_deposit, lease_start_date, lease_end_date)
    VALUES
        (v_prop, '1', 'Ground Floor Retail Storefront', 0, 1, 1800, 'OCCUPIED', FALSE, 5500, 5200, 0, 0, 10400, '2024-05-01', '2029-04-30'),
        (v_prop, '2', '2BR/2BA Timber Loft', 2, 2, 1350, 'OCCUPIED', TRUE, 3300, 3250, 80, 0, 3250, '2026-01-01', '2026-12-31'),
        (v_prop, '3', '2BR/2BA Penthouse Loft', 2, 2, 1350, 'OCCUPIED', TRUE, 3500, 3500, 80, 50, 3500, '2026-06-01', '2027-05-31');

    INSERT INTO tenants (tenant_initials, property_id, unit_number, monthly_rent, pet_count, move_in_date, move_out_date, lease_start_date, lease_end_date, security_deposit)
    VALUES
        ('RF', v_prop, 1, 5200, 0, '2024-05-01 00:00:00Z', NULL, '2024-05-01 00:00:00Z', '2029-04-30 00:00:00Z', 10400),
        ('JH', v_prop, 2, 3250, 0, '2025-01-01 00:00:00Z', NULL, '2026-01-01 00:00:00Z', '2026-12-31 00:00:00Z', 3250),
        ('BM', v_prop, 3, 3500, 1, '2025-06-01 00:00:00Z', NULL, '2026-06-01 00:00:00Z', '2027-05-31 00:00:00Z', 3500);

    INSERT INTO property_loans (property_id, lender_name, loan_type, original_loan_amount, current_principal_balance,
        interest_rate, amortization_months, loan_term_months, interest_only_months, loan_origination_fees)
    VALUES (v_prop, 'Huntington National Bank', 'BRIDGE', 1320000, 1320000.00, 0.07850, 360, 24, 24, 26400);

    INSERT INTO underwriting_models (property_id, physical_vacancy_pct, credit_loss_concessions_pct, annual_property_taxes,
        annual_insurance, monthly_landlord_utilities, monthly_cam_lawn_snow_pest, annual_admin_legal_cpa,
        management_fee_pct_of_egi, repairs_maint_pct_of_egi, capex_reserve_pct_of_egi, exit_cap_rate_pct)
    VALUES (v_prop, 0.0600, 0.0100, 22400, 6500, 240, 160, 1400, 0.0600, 0.0350, 0.0300, 0.0675);

    -- =========================================================================
    -- PROPERTY 4: Logan Square 6-Unit Apartment Building (Multi-Family 5+)
    -- =========================================================================
    INSERT INTO properties (user_id, legal_entity_id, name, category, street_address, city, state, zip, parcel_pin,
        year_built, units, gross_sq_ft, acquisition_date, purchase_price, closing_costs, initial_rehab_budget,
        initial_operating_reserve, after_repair_value, land_value_allocation_pct, depreciation_schedule_years, utility_config)
    VALUES (v_user_id, v_fund, 'Logan Square 6-Unit', 'MULTI_FAMILY_5_PLUS', '2844 N Sawyer Ave', 'Chicago', 'IL', '60618',
        'DEV-13-25-419-011', 1916, 6, 5900, '2024-09-15', 1380000, 28000, 85000, 30000, 1580000, 0.2500, 27.5,
        '{"water": "LANDLORD", "electric": "TENANT", "gas": "LANDLORD", "trash": "LANDLORD"}')
    RETURNING id INTO v_prop;

    INSERT INTO units (property_id, unit_number, floor_plan_name, bedrooms, bathrooms, rentable_sq_ft, status, is_renovated,
        market_rent, in_place_rent, monthly_rubs_charge, monthly_pet_rent, monthly_parking_storage, security_deposit, lease_start_date, lease_end_date)
    VALUES
        (v_prop, '1', '2BR/1BA Garden', 2, 1, 850, 'OCCUPIED', FALSE, 1650, 1600, 65, 0, 0, 1600, '2025-11-01', '2026-10-31'),
        (v_prop, '2', '2BR/1BA 1st Fl Front', 2, 1, 950, 'OCCUPIED', TRUE, 1900, 1850, 70, 25, 0, 1850, '2026-03-01', '2027-02-28'),
        (v_prop, '3', '2BR/1BA 1st Fl Rear', 2, 1, 950, 'OCCUPIED', FALSE, 1900, 1850, 70, 0, 0, 1850, '2026-01-01', '2026-12-31'),
        (v_prop, '4', '3BR/1BA 2nd Fl Front', 3, 1, 1050, 'OCCUPIED', TRUE, 2200, 2150, 75, 35, 85, 2150, '2026-07-01', '2027-06-30'),
        (v_prop, '5', '3BR/1BA 2nd Fl Rear', 3, 1, 1050, 'OCCUPIED', FALSE, 2200, 2100, 75, 0, 0, 2100, '2025-10-01', '2026-09-30'),
        (v_prop, '6', '2BR/1BA Coach House', 2, 1, 1050, 'VACANT_RENOVATION', TRUE, 2000, 0, 0, 0, 0, 0, NULL, NULL);

    INSERT INTO tenants (tenant_initials, property_id, unit_number, monthly_rent, pet_count, move_in_date, move_out_date, lease_start_date, lease_end_date, security_deposit)
    VALUES
        ('PL', v_prop, 1, 1600, 0, '2024-11-01 00:00:00Z', NULL, '2025-11-01 00:00:00Z', '2026-10-31 00:00:00Z', 1600),
        ('GK', v_prop, 2, 1850, 1, '2025-03-01 00:00:00Z', NULL, '2026-03-01 00:00:00Z', '2027-02-28 00:00:00Z', 1850),
        ('ER', v_prop, 3, 1850, 0, '2025-01-01 00:00:00Z', NULL, '2026-01-01 00:00:00Z', '2026-12-31 00:00:00Z', 1850),
        ('MS', v_prop, 4, 2150, 1, '2025-07-01 00:00:00Z', NULL, '2026-07-01 00:00:00Z', '2027-06-30 00:00:00Z', 2150),
        ('LC', v_prop, 5, 2100, 0, '2024-10-01 00:00:00Z', NULL, '2025-10-01 00:00:00Z', '2026-09-30 00:00:00Z', 2100);

    INSERT INTO property_loans (property_id, lender_name, loan_type, original_loan_amount, current_principal_balance,
        interest_rate, amortization_months, loan_term_months, loan_origination_fees)
    VALUES
        (v_prop, 'Old Second National Bank', 'COMMERCIAL', 1035000, 1012800.00, 0.06625, 300, 60, 10350),
        (v_prop, 'Seller Carry - N. Kowalski', 'SELLER', 100000, 92000.00, 0.04000, 120, 60, 0);

    INSERT INTO underwriting_models (property_id, physical_vacancy_pct, credit_loss_concessions_pct, monthly_other_property_income,
        annual_property_taxes, annual_insurance, monthly_landlord_utilities, monthly_cam_lawn_snow_pest, annual_admin_legal_cpa,
        management_fee_pct_of_egi, repairs_maint_pct_of_egi, capex_reserve_pct_of_egi)
    VALUES (v_prop, 0.0500, 0.0100, 160, 18200, 5800, 820, 220, 1200, 0.0700, 0.0350, 0.0250);

    -- =========================================================================
    -- PROPERTY 5: South Loop 8-Unit Boutique Mid-Rise (Multi-Family 5+)
    -- =========================================================================
    INSERT INTO properties (user_id, legal_entity_id, name, category, street_address, city, state, zip, parcel_pin,
        year_built, units, gross_sq_ft, acquisition_date, purchase_price, closing_costs, initial_rehab_budget,
        initial_operating_reserve, after_repair_value, land_value_allocation_pct, depreciation_schedule_years, utility_config)
    VALUES (v_user_id, v_fund, 'South Loop 8-Unit Boutique Mid-Rise', 'MULTI_FAMILY_5_PLUS', '1248 S Michigan Ave', 'Chicago', 'IL', '60605',
        'DEV-17-22-104-033', 1928, 8, 7800, '2024-04-01', 2100000, 42000, 110000, 45000, 2450000, 0.2000, 27.5,
        '{"water": "LANDLORD", "electric": "TENANT", "gas": "TENANT", "trash": "LANDLORD"}')
    RETURNING id INTO v_prop;

    INSERT INTO units (property_id, unit_number, floor_plan_name, bedrooms, bathrooms, rentable_sq_ft, status, is_renovated,
        market_rent, in_place_rent, monthly_rubs_charge, monthly_pet_rent, monthly_parking_storage, security_deposit, lease_start_date, lease_end_date)
    VALUES
        (v_prop, '1', '1BR/1BA Garden', 1, 1, 720, 'OCCUPIED', FALSE, 1750, 1700, 60, 0, 0, 1700, '2025-08-01', '2026-07-31'),
        (v_prop, '2', '1BR/1BA 1st Fl', 1, 1, 750, 'OCCUPIED', TRUE, 1850, 1800, 60, 30, 0, 1800, '2026-02-01', '2027-01-31'),
        (v_prop, '3', '2BR/1.5BA 2nd Fl Front', 2, 1.5, 1050, 'OCCUPIED', FALSE, 2400, 2350, 75, 0, 120, 2350, '2025-10-01', '2026-09-30'),
        (v_prop, '4', '2BR/1.5BA 2nd Fl Rear', 2, 1.5, 1050, 'OCCUPIED', TRUE, 2500, 2500, 75, 35, 0, 2500, '2026-05-01', '2027-04-30'),
        (v_prop, '5', '2BR/2BA 3rd Fl Front', 2, 2, 1150, 'OCCUPIED', TRUE, 2700, 2650, 80, 0, 120, 2650, '2026-03-01', '2027-02-28'),
        (v_prop, '6', '2BR/2BA 3rd Fl Rear', 2, 2, 1150, 'OCCUPIED', FALSE, 2700, 2600, 80, 0, 0, 2600, '2025-11-01', '2026-10-31'),
        (v_prop, '7', '3BR/2BA Top Fl Penthouse', 3, 2, 1400, 'OCCUPIED', TRUE, 3400, 3350, 90, 0, 150, 3350, '2026-04-01', '2027-03-31'),
        (v_prop, '8', '1BR/1BA Super Suite', 1, 1, 650, 'NON_REVENUE_MODEL', FALSE, 1600, 0, 0, 0, 0, 0, NULL, NULL);

    INSERT INTO tenants (tenant_initials, property_id, unit_number, monthly_rent, pet_count, move_in_date, move_out_date, lease_start_date, lease_end_date, security_deposit)
    VALUES
        ('DA', v_prop, 1, 1700, 0, '2024-08-01 00:00:00Z', NULL, '2025-08-01 00:00:00Z', '2026-07-31 00:00:00Z', 1700),
        ('RK', v_prop, 2, 1800, 1, '2025-02-01 00:00:00Z', NULL, '2026-02-01 00:00:00Z', '2027-01-31 00:00:00Z', 1800),
        ('TJ', v_prop, 3, 2350, 0, '2024-10-01 00:00:00Z', NULL, '2025-10-01 00:00:00Z', '2026-09-30 00:00:00Z', 2350),
        ('NB', v_prop, 4, 2500, 1, '2025-05-01 00:00:00Z', NULL, '2026-05-01 00:00:00Z', '2027-04-30 00:00:00Z', 2500),
        ('HW', v_prop, 5, 2650, 0, '2025-03-01 00:00:00Z', NULL, '2026-03-01 00:00:00Z', '2027-02-28 00:00:00Z', 2650),
        ('SO', v_prop, 6, 2600, 0, '2024-11-01 00:00:00Z', NULL, '2025-11-01 00:00:00Z', '2026-10-31 00:00:00Z', 2600),
        ('VT', v_prop, 7, 3350, 0, '2025-04-01 00:00:00Z', NULL, '2026-04-01 00:00:00Z', '2027-03-31 00:00:00Z', 3350);

    INSERT INTO property_loans (property_id, lender_name, loan_type, original_loan_amount, current_principal_balance,
        interest_rate, amortization_months, loan_term_months, monthly_pmi, loan_origination_fees)
    VALUES (v_prop, 'CIBC Bank USA', 'COMMERCIAL', 1680000, 1642000.00, 0.06375, 300, 60, 140.00, 16800);

    INSERT INTO underwriting_models (property_id, physical_vacancy_pct, credit_loss_concessions_pct, monthly_other_property_income,
        annual_property_taxes, annual_insurance, monthly_landlord_utilities, monthly_cam_lawn_snow_pest, annual_admin_legal_cpa,
        management_fee_pct_of_egi, repairs_maint_pct_of_egi, capex_reserve_pct_of_egi)
    VALUES (v_prop, 0.0400, 0.0100, 240, 28500, 8400, 950, 340, 1600, 0.0600, 0.0350, 0.0300);

    -- =========================================================================
    -- PROPERTY 6: Oak Park Prairie Single-Family Home (Single Family)
    -- =========================================================================
    INSERT INTO properties (user_id, legal_entity_id, name, category, street_address, city, state, zip, parcel_pin,
        year_built, units, gross_sq_ft, acquisition_date, purchase_price, closing_costs, initial_rehab_budget,
        initial_operating_reserve, after_repair_value, land_value_allocation_pct, depreciation_schedule_years, utility_config)
    VALUES (v_user_id, v_personal, 'Oak Park Prairie Home', 'SINGLE_FAMILY', '318 S Kenilworth Ave', 'Oak Park', 'IL', '60302',
        'DEV-16-07-312-008', 1922, 1, 2400, '2023-08-20', 460000, 10500, 35000, 12000, 530000, 0.2500, 27.5,
        '{"water": "TENANT", "electric": "TENANT", "gas": "TENANT", "trash": "LANDLORD"}')
    RETURNING id INTO v_prop;

    INSERT INTO units (property_id, unit_number, floor_plan_name, bedrooms, bathrooms, rentable_sq_ft, status, is_renovated,
        market_rent, in_place_rent, monthly_pet_rent, security_deposit, lease_start_date, lease_end_date)
    VALUES (v_prop, '1', '4BR/2.5BA Prairie Home', 4, 2.5, 2400, 'OCCUPIED', TRUE, 3400, 3350, 50, 3350, '2025-09-01', '2027-08-31');

    INSERT INTO tenants (tenant_initials, property_id, unit_number, monthly_rent, pet_count, move_in_date, move_out_date, lease_start_date, lease_end_date, security_deposit)
    VALUES ('WH', v_prop, 1, 3350, 1, '2023-09-01 00:00:00Z', NULL, '2025-09-01 00:00:00Z', '2027-08-31 00:00:00Z', 3350);

    INSERT INTO property_loans (property_id, lender_name, loan_type, original_loan_amount, current_principal_balance,
        interest_rate, amortization_months, loan_term_months, loan_origination_fees)
    VALUES (v_prop, 'Byline Bank', 'CONVENTIONAL', 368000, 358200.00, 0.05875, 360, 360, 3680);

    INSERT INTO underwriting_models (property_id, physical_vacancy_pct, credit_loss_concessions_pct, annual_property_taxes,
        annual_insurance, monthly_landlord_utilities, monthly_cam_lawn_snow_pest, annual_admin_legal_cpa,
        management_fee_pct_of_egi, repairs_maint_pct_of_egi, capex_reserve_pct_of_egi)
    VALUES (v_prop, 0.0300, 0.0050, 9600, 1800, 45, 55, 450, 0.0000, 0.0400, 0.0400);

    -- =========================================================================
    -- PROPERTY 7: West Loop High-Rise Luxury Condo #502 (Condo / Townhome)
    -- =========================================================================
    INSERT INTO properties (user_id, legal_entity_id, name, category, street_address, city, state, zip, parcel_pin,
        year_built, units, gross_sq_ft, acquisition_date, purchase_price, closing_costs, initial_rehab_budget,
        initial_operating_reserve, after_repair_value, land_value_allocation_pct, depreciation_schedule_years, utility_config)
    VALUES (v_user_id, v_series, 'West Loop High-Rise Condo #502', 'CONDO_TOWNHOME', '111 S Peoria St Unit 502', 'Chicago', 'IL', '60607',
        'DEV-17-17-302-045-1052', 2018, 1, 1280, '2024-06-01', 520000, 9800, 8000, 10000, 565000, 0.1200, 27.5,
        '{"water": "LANDLORD", "electric": "TENANT", "gas": "LANDLORD", "trash": "LANDLORD"}')
    RETURNING id INTO v_prop;

    INSERT INTO units (property_id, unit_number, floor_plan_name, bedrooms, bathrooms, rentable_sq_ft, status, is_renovated,
        market_rent, in_place_rent, monthly_parking_storage, security_deposit, lease_start_date, lease_end_date)
    VALUES (v_prop, '1', '2BR/2BA Luxury Corner', 2, 2, 1280, 'OCCUPIED', FALSE, 3400, 3300, 250, 3300, '2026-03-01', '2027-02-28');

    INSERT INTO tenants (tenant_initials, property_id, unit_number, monthly_rent, pet_count, move_in_date, move_out_date, lease_start_date, lease_end_date, security_deposit)
    VALUES ('SG', v_prop, 1, 3300, 0, '2024-06-15 00:00:00Z', NULL, '2026-03-01 00:00:00Z', '2027-02-28 00:00:00Z', 3300);

    INSERT INTO property_loans (property_id, lender_name, loan_type, original_loan_amount, current_principal_balance,
        interest_rate, amortization_months, loan_term_months, loan_origination_fees)
    VALUES (v_prop, 'Chase Home Lending', 'CONVENTIONAL', 416000, 408500.00, 0.06750, 360, 360, 4160);

    INSERT INTO underwriting_models (property_id, physical_vacancy_pct, credit_loss_concessions_pct, monthly_hoa_dues,
        annual_property_taxes, annual_insurance, annual_admin_legal_cpa, management_fee_pct_of_egi, repairs_maint_pct_of_egi,
        capex_reserve_pct_of_egi)
    VALUES (v_prop, 0.0300, 0.0050, 580, 8400, 850, 400, 0.0600, 0.0300, 0.0200);

    -- =========================================================================
    -- PROPERTY 8: Gold Coast Penthouse (Condo / Townhome - Free & Clear Debt-Free Asset)
    -- =========================================================================
    INSERT INTO properties (user_id, legal_entity_id, name, category, street_address, city, state, zip, parcel_pin,
        year_built, units, gross_sq_ft, acquisition_date, purchase_price, closing_costs, initial_rehab_budget,
        initial_operating_reserve, after_repair_value, land_value_allocation_pct, depreciation_schedule_years, utility_config)
    VALUES (v_user_id, v_personal, 'Gold Coast Penthouse (Debt-Free)', 'CONDO_TOWNHOME', '1210 N Astor St Apt 14A', 'Chicago', 'IL', '60610',
        'DEV-17-03-201-088-1401', 1968, 1, 2100, '2023-11-01', 890000, 16000, 25000, 20000, 975000, 0.1000, 27.5,
        '{"water": "LANDLORD", "electric": "TENANT", "gas": "LANDLORD", "trash": "LANDLORD"}')
    RETURNING id INTO v_prop;

    INSERT INTO units (property_id, unit_number, floor_plan_name, bedrooms, bathrooms, rentable_sq_ft, status, is_renovated,
        market_rent, in_place_rent, monthly_parking_storage, security_deposit, lease_start_date, lease_end_date)
    VALUES (v_prop, '1', '3BR/3BA Penthouse Terrace', 3, 3, 2100, 'OCCUPIED', TRUE, 5200, 5000, 300, 5000, '2025-11-01', '2026-10-31');

    INSERT INTO tenants (tenant_initials, property_id, unit_number, monthly_rent, pet_count, move_in_date, move_out_date, lease_start_date, lease_end_date, security_deposit)
    VALUES ('AL', v_prop, 1, 5000, 0, '2023-11-15 00:00:00Z', NULL, '2025-11-01 00:00:00Z', '2026-10-31 00:00:00Z', 5000);

    -- Property 8 has NO loans (Free & Clear: $0 debt)

    INSERT INTO underwriting_models (property_id, physical_vacancy_pct, credit_loss_concessions_pct, monthly_hoa_dues,
        annual_property_taxes, annual_insurance, annual_admin_legal_cpa, management_fee_pct_of_egi, repairs_maint_pct_of_egi,
        capex_reserve_pct_of_egi)
    VALUES (v_prop, 0.0200, 0.0050, 1150, 14200, 1200, 600, 0.0500, 0.0250, 0.0200);

    -- =========================================================================
    -- PROPERTY 9: Pilsen Live/Work Arts District Mixed-Use (Mixed-Use, Unassigned Entity)
    -- =========================================================================
    INSERT INTO properties (user_id, legal_entity_id, name, category, street_address, city, state, zip, parcel_pin,
        year_built, units, gross_sq_ft, acquisition_date, purchase_price, closing_costs, initial_rehab_budget,
        initial_operating_reserve, after_repair_value, land_value_allocation_pct, depreciation_schedule_years, utility_config)
    VALUES (v_user_id, NULL, 'Pilsen Arts District Mixed-Use', 'MIXED_USE', '1821 S Halsted St', 'Chicago', 'IL', '60608',
        'DEV-17-20-418-026', 1924, 3, 3600, '2024-05-15', 740000, 14000, 50000, 15000, 860000, 0.2200, 39.0,
        '{"water": "LANDLORD", "electric": "TENANT", "gas": "TENANT", "trash": "LANDLORD"}')
    RETURNING id INTO v_prop;

    INSERT INTO units (property_id, unit_number, floor_plan_name, bedrooms, bathrooms, rentable_sq_ft, status, is_renovated,
        market_rent, in_place_rent, monthly_rubs_charge, monthly_pet_rent, security_deposit, lease_start_date, lease_end_date)
    VALUES
        (v_prop, '1', 'Commercial Art Studio & Gallery', 0, 1, 1200, 'OCCUPIED', FALSE, 2800, 2700, 0, 0, 5400, '2024-06-01', '2027-05-31'),
        (v_prop, '2', '2BR/1BA Loft', 2, 1, 1200, 'OCCUPIED', FALSE, 2200, 2150, 65, 0, 2150, '2026-05-01', '2027-04-30'),
        (v_prop, '3', '2BR/1BA Top Loft', 2, 1, 1200, 'OCCUPIED', TRUE, 2350, 2300, 65, 30, 2300, '2025-08-01', '2026-07-31');

    INSERT INTO tenants (tenant_initials, property_id, unit_number, monthly_rent, pet_count, move_in_date, move_out_date, lease_start_date, lease_end_date, security_deposit)
    VALUES
        ('EZ', v_prop, 1, 2700, 0, '2024-06-01 00:00:00Z', NULL, '2024-06-01 00:00:00Z', '2027-05-31 00:00:00Z', 5400),
        ('KL', v_prop, 2, 2150, 0, '2025-05-01 00:00:00Z', NULL, '2026-05-01 00:00:00Z', '2027-04-30 00:00:00Z', 2150),
        ('TY', v_prop, 3, 2300, 1, '2024-08-01 00:00:00Z', NULL, '2025-08-01 00:00:00Z', '2026-07-31 00:00:00Z', 2300);

    INSERT INTO property_loans (property_id, lender_name, loan_type, original_loan_amount, current_principal_balance,
        interest_rate, amortization_months, loan_term_months, loan_origination_fees)
    VALUES (v_prop, 'Fifth Third Bank', 'COMMERCIAL', 555000, 538000.00, 0.06850, 240, 60, 5550);

    INSERT INTO underwriting_models (property_id, physical_vacancy_pct, credit_loss_concessions_pct, annual_property_taxes,
        annual_insurance, monthly_landlord_utilities, monthly_cam_lawn_snow_pest, annual_admin_legal_cpa,
        management_fee_pct_of_egi, repairs_maint_pct_of_egi, capex_reserve_pct_of_egi)
    VALUES (v_prop, 0.0500, 0.0100, 11200, 3100, 210, 110, 700, 0.0600, 0.0350, 0.0300);

    -- =========================================================================
    -- PROPERTY 10: Evanston Value-Add Duplex (Multi-Family 2-4, Pending Rehab, Excluded)
    -- =========================================================================
    INSERT INTO properties (user_id, legal_entity_id, name, category, street_address, city, state, zip, parcel_pin,
        year_built, units, gross_sq_ft, acquisition_date, purchase_price, closing_costs, initial_rehab_budget,
        initial_operating_reserve, after_repair_value, land_value_allocation_pct, depreciation_schedule_years, utility_config, is_active)
    VALUES (v_user_id, v_holdco, 'Evanston Value-Add Duplex (Pending Rehab)', 'MULTI_FAMILY_2_4', '1715 Ridge Ave', 'Evanston', 'IL', '60201',
        'DEV-10-18-305-014', 1925, 2, 2800, '2026-08-01', 410000, 8500, 95000, 15000, 620000, 0.2000, 27.5,
        '{"water": "LANDLORD", "electric": "TENANT", "gas": "TENANT", "trash": "LANDLORD"}', FALSE)
    RETURNING id INTO v_prop;

    INSERT INTO units (property_id, unit_number, floor_plan_name, bedrooms, bathrooms, rentable_sq_ft, status, is_renovated,
        market_rent, in_place_rent, monthly_rubs_charge, monthly_pet_rent, monthly_parking_storage, security_deposit, lease_start_date, lease_end_date)
    VALUES
        (v_prop, '1', '2BR/1BA Gut Rehab Unit A', 2, 1, 1400, 'VACANT_RENOVATION', FALSE, 2300, 0, 0, 0, 0, 0, NULL, NULL),
        (v_prop, '2', '2BR/1BA Gut Rehab Unit B', 2, 1, 1400, 'VACANT_RENOVATION', FALSE, 2300, 0, 0, 0, 0, 0, NULL, NULL);

    INSERT INTO property_loans (property_id, lender_name, loan_type, original_loan_amount, current_principal_balance,
        interest_rate, amortization_months, loan_term_months, interest_only_months, loan_origination_fees)
    VALUES (v_prop, 'Lakeshore Private Lending', 'BRIDGE', 328000, 328000.00, 0.09500, 12, 12, 12, 6560);

    INSERT INTO underwriting_models (property_id, physical_vacancy_pct, credit_loss_concessions_pct, annual_property_taxes,
        annual_insurance, monthly_landlord_utilities, monthly_cam_lawn_snow_pest, annual_admin_legal_cpa,
        management_fee_pct_of_egi, repairs_maint_pct_of_egi, capex_reserve_pct_of_egi)
    VALUES (v_prop, 0.0500, 0.0100, 7800, 2200, 150, 80, 500, 0.0700, 0.0400, 0.0300);

    -- =========================================================================
    -- HISTORICAL LEDGER ACTUALS (12 Months: Oct 2025 - Sep 2026)
    -- =========================================================================

    -- 1. Unit-level rent, RUBS, pet fees, and parking
    INSERT INTO ledger_transactions (property_id, unit_id, transaction_date, category, amount, vendor_or_payor, description)
    SELECT u.property_id, u.id, m.d + 1, c.category, c.amount, 'Tenant ' || u.unit_number, c.description
    FROM units u
    JOIN properties p ON p.id = u.property_id
    CROSS JOIN generate_series(DATE '2025-10-01', DATE '2026-09-01', INTERVAL '1 month') AS g(ts)
    CROSS JOIN LATERAL (SELECT g.ts::date AS d) m
    CROSS JOIN LATERAL (VALUES
        ('GROSS_RENT'::ledger_category, u.in_place_rent, 'Monthly rent collection'),
        ('RUBS_REIMBURSEMENT'::ledger_category, u.monthly_rubs_charge, 'Utility bill-back reimbursement (RUBS)'),
        ('PET_RENT'::ledger_category, u.monthly_pet_rent, 'Monthly pet rent fee'),
        ('PARKING_STORAGE'::ledger_category, u.monthly_parking_storage, 'Assigned parking / storage locker fee')
    ) AS c(category, amount, description)
    WHERE p.user_id = v_user_id AND p.parcel_pin LIKE 'DEV-%'
      AND u.status = 'OCCUPIED' AND c.amount > 0 AND m.d >= p.acquisition_date;

    -- 2. Property-level operating expenses and ancillary income
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
        WHERE p.user_id = v_user_id AND p.parcel_pin LIKE 'DEV-%' AND m.d >= p.acquisition_date
    ) s
    CROSS JOIN LATERAL (VALUES
        (s.property_id, s.d + 4, 'LAUNDRY_VENDING'::ledger_category, s.monthly_other_property_income * (0.85 + (s.n % 4) * 0.1), 'CSC ServiceWorks', 'Coin laundry / smart vending revenue'),
        (s.property_id, s.d + 9, 'PROPERTY_MANAGEMENT'::ledger_category, s.collected_rent * s.management_fee_pct_of_egi, 'Rixus Property Management', 'Monthly professional property management fee'),
        (s.property_id, s.d + 14, 'HOA_DUES'::ledger_category, s.monthly_hoa_dues, 'Condo Association Management', 'Monthly HOA master dues assessment'),
        (s.property_id, s.d + 17, 'UTILITIES_WATER_SEWER'::ledger_category, s.monthly_landlord_utilities * 0.7 * (0.9 + (s.n % 3) * 0.1), 'City of Chicago Water Dept', 'Municipal water / sewer service'),
        (s.property_id, s.d + 17, 'UTILITIES_TRASH'::ledger_category, s.monthly_landlord_utilities * 0.3, 'Waste Management', 'Commercial container waste & recycling pickup'),
        (s.property_id, s.d + 20, 'CAM_LAWN_SNOW_PEST'::ledger_category,
            s.monthly_cam_lawn_snow_pest * CASE WHEN EXTRACT(MONTH FROM s.d) IN (12, 1, 2) THEN 1.6 ELSE 0.8 END, 'Northside Grounds Care', 'Common area maintenance, snow removal & landscaping'),
        (s.property_id, s.d + 22, 'REPAIRS_MAINTENANCE'::ledger_category,
            CASE WHEN (s.n + s.property_id) % 3 = 0 THEN s.collected_rent * 12 * s.repairs_maint_pct_of_egi / 4 + s.annual_repairs_per_unit ELSE 0 END,
            'Midwest Property Repairs', 'General maintenance, plumbing & electrical service'),
        (s.property_id, s.d + 2, 'PROPERTY_TAX'::ledger_category,
            CASE WHEN EXTRACT(MONTH FROM s.d) = 3 THEN s.annual_property_taxes * 0.55
                 WHEN EXTRACT(MONTH FROM s.d) = 8 THEN s.annual_property_taxes * 0.45 ELSE 0 END, 'Cook County Treasurer', 'Semi-annual real estate property tax installment'),
        (s.property_id, s.d + 5, 'INSURANCE'::ledger_category,
            CASE WHEN s.n = 1 + (s.property_id % 12) THEN s.annual_insurance ELSE 0 END, 'Travelers Insurance', 'Commercial property & casualty insurance premium'),
        (s.property_id, s.d + 11, 'ADMIN_LEGAL_CPA'::ledger_category,
            CASE WHEN EXTRACT(MONTH FROM s.d) = 4 THEN s.annual_admin_legal_cpa * 0.7 WHEN EXTRACT(MONTH FROM s.d) = 1 THEN s.annual_admin_legal_cpa * 0.3 ELSE 0 END,
            'Loop CPA & Real Estate Advisory', 'Annual partnership tax return prep & legal filings'),
        (s.property_id, s.d + 26, 'TURNOVER_MAKE_READY'::ledger_category,
            CASE WHEN s.n = 1 + ((s.property_id * 5) % 12) THEN 650 + (s.property_id % 4) * 300 ELSE 0 END, 'Apex Turnover Solutions', 'Unit refresh, deep clean & paint'),
        (s.property_id, s.d + 26, 'CAPEX_REPLACEMENT'::ledger_category,
            CASE WHEN s.n = 1 + ((s.property_id * 7) % 12) THEN 1800 + (s.property_id % 3) * 1400 ELSE 0 END, 'Reliable HVAC & Mechanical', 'High-efficiency mechanical equipment replacement'),
        (s.property_id, s.d + 27, 'CONCESSIONS_BAD_DEBT'::ledger_category,
            CASE WHEN s.n = 1 + ((s.property_id * 3) % 12) THEN s.collected_rent * s.credit_loss_concessions_pct * 6 ELSE 0 END, NULL, 'Move-in rent concession & bad debt allowance')
    ) AS x(property_id, d, category, amount, vendor, description)
    WHERE x.amount > 0;

    -- 3. Debt service payments (Principal & Interest split)
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
        ('DEBT_INTEREST'::ledger_category, pay.interest, 'Monthly mortgage interest payment'),
        ('DEBT_PRINCIPAL'::ledger_category, pay.payment - pay.interest, 'Monthly loan principal amortization')
    ) AS c(category, amount, description)
    WHERE p.user_id = v_user_id AND p.parcel_pin LIKE 'DEV-%' AND l.is_active
      AND c.amount > 0 AND m.d >= p.acquisition_date;

    -- =========================================================================
    -- MULTIMEDIA & DOCUMENTS
    -- =========================================================================

    -- Standard PDF byte payload
    v_doc_pdf := convert_to(
'%PDF-1.4
1 0 obj
<< /Type /Catalog /Pages 2 0 R >>
endobj
2 0 obj
<< /Type /Pages /Kids [3 0 R] /Count 1 >>
endobj
3 0 obj
<< /Type /Page /Parent 2 0 R /MediaBox [0 0 612 792] /Contents 4 0 R /Resources << /Font << /F1 5 0 R >> >> >>
endobj
4 0 obj
<< /Length 120 >>
stream
BT
/F1 18 Tf
50 720 Td
(Rixus Real Estate Platform - Verified Document) Tj
/F1 12 Tf
50 690 Td
(Account: developer@rixus.com | Portfolio Compliance Certified) Tj
ET
endstream
endobj
5 0 obj
<< /Type /Font /Subtype /Type1 /BaseFont /Helvetica >>
endobj
xref
0 6
0000000000 65535 f 
0000000009 00000 n 
0000000058 00000 n 
0000000115 00000 n 
0000000229 00000 n 
0000000399 00000 n 
trailer
<< /Size 6 /Root 1 0 R >>
startxref
478
%%EOF', 'UTF8');

    -- Standard 1x1 JPEG byte payload
    v_doc_jpg := decode('ffd8ffe000104a46494600010101004800480000ffdb004300080606070605080707070909080a0c140d0c0b0b0c1912130f141d1a1f1e1d1a1c1c20242e2720222c231c1c2837292c30313434341f27393d38323c2e333431ffc0000b080001000101011100ffda0008010100003f00bf0037ffd9', 'hex');

    -- Document 1: Illinois Lease Template
    INSERT INTO multimedia (user_id, property_id, title, description, category, media_type, file_name, file_size, mime_type, file_data, is_template)
    VALUES (v_user_id, NULL, 'Standard Illinois Residential Lease Agreement (2026 Edition)',
        'Official 2026 standard statutory residential lease agreement for Illinois Cook County jurisdictions.',
        'legal', 'pdf', 'illinois_standard_lease_2026.pdf', octet_length(v_doc_pdf), 'application/pdf', v_doc_pdf, TRUE);

    -- Document 2: Form I-9
    INSERT INTO multimedia (user_id, property_id, title, description, category, media_type, file_name, file_size, mime_type, file_data, is_template)
    VALUES (v_user_id, NULL, 'USCIS Form I-9 Employment & Tenant Eligibility Verification (2026)',
        'Federal Form I-9 identity verification document for employment and screening records.',
        'compliance', 'pdf', 'form_i9_verification_2026.pdf', octet_length(v_doc_pdf), 'application/pdf', v_doc_pdf, TRUE);

    -- Document 3: EPA Lead Paint Disclosure
    INSERT INTO multimedia (user_id, property_id, title, description, category, media_type, file_name, file_size, mime_type, file_data, is_template)
    VALUES (v_user_id, NULL, 'EPA Protect Your Family From Lead In Your Home Disclosure',
        'Required federal lead-based paint disclosure pamphlet for pre-1978 residential buildings.',
        'compliance', 'pdf', 'epa_lead_paint_disclosure.pdf', octet_length(v_doc_pdf), 'application/pdf', v_doc_pdf, TRUE);

    -- Document 4: Lincoln Park Executed Lease (Property 1)
    SELECT id INTO v_prop FROM properties WHERE user_id = v_user_id AND parcel_pin = 'DEV-14-32-108-015';
    INSERT INTO multimedia (user_id, property_id, title, description, category, media_type, file_name, file_size, mime_type, file_data, is_template)
    VALUES (v_user_id, v_prop, 'Executed Lease Agreement - Unit 1 Garden Suite',
        'Fully signed 12-month residential lease for tenant M.K. covering 2026-2027.',
        'lease', 'pdf', 'lease_2124_kenmore_unit1.pdf', octet_length(v_doc_pdf), 'application/pdf', v_doc_pdf, FALSE);

    -- Document 5: Logan Square Inspection Report (Property 4)
    SELECT id INTO v_prop FROM properties WHERE user_id = v_user_id AND parcel_pin = 'DEV-13-25-419-011';
    INSERT INTO multimedia (user_id, property_id, title, description, category, media_type, file_name, file_size, mime_type, file_data, is_template)
    VALUES (v_user_id, v_prop, 'Professional Property Inspection & Structural Condition Report',
        'Full 45-page engineer physical inspection and roof/foundation assessment report.',
        'inspection', 'pdf', 'inspection_report_sawyer_6unit.pdf', octet_length(v_doc_pdf), 'application/pdf', v_doc_pdf, FALSE);

    -- Document 6: West Loop Photo (Property 7)
    SELECT id INTO v_prop FROM properties WHERE user_id = v_user_id AND parcel_pin = 'DEV-17-17-302-045-1052';
    INSERT INTO multimedia (user_id, property_id, title, description, category, media_type, file_name, file_size, mime_type, file_data, is_template)
    VALUES (v_user_id, v_prop, 'West Loop High-Rise Architecture & Skyline Exterior Photo',
        'High-resolution architectural exterior elevation photograph of luxury condo asset.',
        'photo', 'image', 'west_loop_condo_exterior.jpeg', octet_length(v_doc_jpg), 'image/jpeg', v_doc_jpg, FALSE);

    -- Document 7: Fulton Market Commercial Blueprint (Property 3)
    SELECT id INTO v_prop FROM properties WHERE user_id = v_user_id AND parcel_pin = 'DEV-17-08-410-019';
    INSERT INTO multimedia (user_id, property_id, title, description, category, media_type, file_name, file_size, mime_type, file_data, is_template)
    VALUES (v_user_id, v_prop, 'Commercial Retail Storefront Floor Plan & Architectural Schematic',
        'Architectural schematic drawing and mechanical MEP plans for ground floor commercial retail.',
        'blueprint', 'pdf', 'fulton_market_storefront_floorplan.pdf', octet_length(v_doc_pdf), 'application/pdf', v_doc_pdf, FALSE);

    -- Document 8: General Series LLC Operating Agreement
    INSERT INTO multimedia (user_id, property_id, title, description, category, media_type, file_name, file_size, mime_type, file_data, is_template)
    VALUES (v_user_id, NULL, 'Illinois Certificate of Good Standing & Series LLC Operating Agreement',
        'Master operating agreement establishing statutory separation of liabilities across property series cells.',
        'legal', 'pdf', 'rixus_series_llc_operating_agreement.pdf', octet_length(v_doc_pdf), 'application/pdf', v_doc_pdf, FALSE);

    RAISE NOTICE 'Migration 011 complete: seeded 4 legal entities, 10 properties, 32 units, 27 tenants, 10 loans, 10 underwriting models, 12-month ledger, and 8 multimedia records for developer@rixus.com';
END $$;
