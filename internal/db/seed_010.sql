-- Embedded Seed 010: Populate comprehensive financial and tenant testing data for rixusrealestate@gmail.com
DO $$
DECLARE
    v_user_id BIGINT;
    v_holdco BIGINT;
    v_series BIGINT;
    v_personal BIGINT;
    v_prop BIGINT;
BEGIN
    -- 1. Ensure user rixusrealestate@gmail.com exists with ADMIN permissions
    SELECT id INTO v_user_id FROM users WHERE email = 'rixusrealestate@gmail.com';
    IF v_user_id IS NULL THEN
        INSERT INTO users (email, name, user_role)
        VALUES ('rixusrealestate@gmail.com', 'Rixus Real Estate', 'ADMIN')
        RETURNING id INTO v_user_id;
    ELSE
        UPDATE users SET user_role = 'ADMIN' WHERE id = v_user_id;
    END IF;

    -- 2. Clean up previous sample data for this user to make migration 100% idempotent
    DELETE FROM tenants WHERE property_id IN (SELECT id FROM properties WHERE user_id = v_user_id);
    DELETE FROM properties WHERE user_id = v_user_id AND (parcel_pin LIKE 'RIXUS-%' OR parcel_pin LIKE 'SAMPLE-%');
    DELETE FROM legal_entities WHERE user_id = v_user_id AND (name LIKE '%(Sample)' OR name LIKE 'Rixus %');

    -- 3. Legal Entities
    INSERT INTO legal_entities (user_id, name, entity_type, bank_account_last4, credit_card_last4)
    VALUES (v_user_id, 'Rixus Capital Holdings LLC (Sample)', 'HOLDING_LLC', '4821', '0937')
    RETURNING id INTO v_holdco;

    INSERT INTO legal_entities (user_id, name, entity_type, bank_account_last4, credit_card_last4)
    VALUES (v_user_id, 'Rixus Series LLC - Chicago Core Cell (Sample)', 'SERIES_LLC_CELL', '7714', NULL)
    RETURNING id INTO v_series;

    INSERT INTO legal_entities (user_id, name, entity_type, bank_account_last4, credit_card_last4)
    VALUES (v_user_id, 'Rixus Direct Ownership (Sample)', 'INDIVIDUAL', '3390', '5512')
    RETURNING id INTO v_personal;

    -- =========================================================================
    -- PROPERTY 1: Lincoln Park Luxury 3-Flat (Multi-Family 2-4)
    -- =========================================================================
    INSERT INTO properties (user_id, legal_entity_id, name, category, street_address, city, state, zip, parcel_pin,
        year_built, units, gross_sq_ft, acquisition_date, purchase_price, closing_costs, initial_rehab_budget,
        initial_operating_reserve, after_repair_value, land_value_allocation_pct, depreciation_schedule_years, utility_config)
    VALUES (v_user_id, v_holdco, 'Lincoln Park Luxury 3-Flat', 'MULTI_FAMILY_2_4', '2124 N Kenmore Ave', 'Chicago', 'IL', '60614',
        'RIXUS-14-32-108-015', 1910, 3, 3850, '2023-04-15', 1250000, 22000, 65000, 25000, 1450000, 0.2500, 27.5,
        '{"water": "LANDLORD", "electric": "TENANT", "gas": "TENANT", "trash": "LANDLORD"}')
    RETURNING id INTO v_prop;

    -- Rent Roll Units
    INSERT INTO units (property_id, unit_number, floor_plan_name, bedrooms, bathrooms, rentable_sq_ft, status, is_renovated,
        market_rent, in_place_rent, monthly_rubs_charge, monthly_pet_rent, monthly_parking_storage, security_deposit, lease_start_date, lease_end_date)
    VALUES
        (v_prop, '1', '3BR/2BA Garden Suite', 3, 2, 1200, 'OCCUPIED', TRUE, 2800, 2750, 75, 35, 0, 2750, '2026-06-01', '2027-05-31'),
        (v_prop, '2', '3BR/2BA Duplex Down', 3, 2, 1350, 'OCCUPIED', TRUE, 3200, 3200, 85, 0, 150, 3200, '2026-04-01', '2027-03-31'),
        (v_prop, '3', '3BR/2BA Penthouse', 3, 2, 1300, 'OCCUPIED', TRUE, 3400, 3350, 85, 40, 150, 3350, '2025-08-01', '2026-07-31');

    -- Tenant Profiles
    INSERT INTO tenants (tenant_initials, property_id, unit_number, monthly_rent, pet_count, move_in_date, move_out_date, lease_start_date, lease_end_date, security_deposit)
    VALUES
        ('MK', v_prop, 1, 2750, 1, '2025-06-01 00:00:00Z', NULL, '2026-06-01 00:00:00Z', '2027-05-31 00:00:00Z', 2750),
        ('CR', v_prop, 2, 3200, 0, '2024-04-01 00:00:00Z', NULL, '2026-04-01 00:00:00Z', '2027-03-31 00:00:00Z', 3200),
        ('DL', v_prop, 3, 3350, 1, '2023-08-01 00:00:00Z', NULL, '2025-08-01 00:00:00Z', '2026-07-31 00:00:00Z', 3350);

    -- Financing
    INSERT INTO property_loans (property_id, lender_name, loan_type, original_loan_amount, current_principal_balance,
        interest_rate, amortization_months, loan_term_months, loan_origination_fees)
    VALUES (v_prop, 'First American Bank', 'CONVENTIONAL', 1000000, 974250.00, 0.06250, 360, 360, 10000);

    -- Underwriting Assumptions
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
        'RIXUS-17-06-214-022', 1920, 4, 4600, '2024-01-10', 980000, 18500, 45000, 18000, 1150000, 0.2200, 27.5,
        '{"water": "LANDLORD", "electric": "TENANT", "gas": "TENANT", "trash": "LANDLORD"}')
    RETURNING id INTO v_prop;

    -- Rent Roll Units
    INSERT INTO units (property_id, unit_number, floor_plan_name, bedrooms, bathrooms, rentable_sq_ft, status, is_renovated,
        market_rent, in_place_rent, monthly_rubs_charge, monthly_pet_rent, monthly_parking_storage, security_deposit, lease_start_date, lease_end_date)
    VALUES
        (v_prop, '1', '2BR/1BA Classic Front', 2, 1, 1100, 'OCCUPIED', FALSE, 2250, 2100, 70, 0, 0, 2100, '2025-09-01', '2026-08-31'),
        (v_prop, '2', '2BR/1BA Classic Rear', 2, 1, 1100, 'OCCUPIED', FALSE, 2250, 2150, 70, 30, 0, 2150, '2026-02-01', '2027-01-31'),
        (v_prop, '3', '2BR/1.5BA Renovated', 2, 1.5, 1150, 'OCCUPIED', TRUE, 2500, 2500, 75, 0, 100, 2500, '2026-05-01', '2027-04-30'),
        (v_prop, '4', '2BR/1.5BA Renovated Top', 2, 1.5, 1150, 'VACANT_READY', TRUE, 2500, 0, 0, 0, 0, 0, NULL, NULL);

    -- Tenants
    INSERT INTO tenants (tenant_initials, property_id, unit_number, monthly_rent, pet_count, move_in_date, move_out_date, lease_start_date, lease_end_date, security_deposit)
    VALUES
        ('TB', v_prop, 1, 2100, 0, '2024-09-01 00:00:00Z', NULL, '2025-09-01 00:00:00Z', '2026-08-31 00:00:00Z', 2100),
        ('AJ', v_prop, 2, 2150, 1, '2025-02-01 00:00:00Z', NULL, '2026-02-01 00:00:00Z', '2027-01-31 00:00:00Z', 2150),
        ('NP', v_prop, 3, 2500, 0, '2025-05-01 00:00:00Z', NULL, '2026-05-01 00:00:00Z', '2027-04-30 00:00:00Z', 2500),
        ('EW', v_prop, 4, 2200, 0, '2024-03-01 00:00:00Z', '2026-04-30 00:00:00Z', '2025-04-01 00:00:00Z', '2026-04-30 00:00:00Z', 2200);

    -- Financing
    INSERT INTO property_loans (property_id, lender_name, loan_type, original_loan_amount, current_principal_balance,
        interest_rate, amortization_months, loan_term_months, loan_origination_fees)
    VALUES (v_prop, 'Wintrust Commercial Banking', 'COMMERCIAL', 784000, 768400.00, 0.06500, 300, 60, 7840);

    -- Underwriting Assumptions
    INSERT INTO underwriting_models (property_id, physical_vacancy_pct, credit_loss_concessions_pct, monthly_other_property_income,
        annual_property_taxes, annual_insurance, monthly_landlord_utilities, monthly_cam_lawn_snow_pest, annual_admin_legal_cpa,
        management_fee_pct_of_egi, repairs_maint_pct_of_egi, capex_reserve_pct_of_egi)
    VALUES (v_prop, 0.0500, 0.0100, 120, 11800, 3200, 310, 120, 750, 0.0700, 0.0400, 0.0300);

    -- =========================================================================
    -- PROPERTY 3: West Loop High-Rise Condo #502 (Condo / Townhome)
    -- =========================================================================
    INSERT INTO properties (user_id, legal_entity_id, name, category, street_address, city, state, zip, parcel_pin,
        year_built, units, gross_sq_ft, acquisition_date, purchase_price, closing_costs, initial_rehab_budget,
        initial_operating_reserve, after_repair_value, land_value_allocation_pct, depreciation_schedule_years, utility_config)
    VALUES (v_user_id, v_series, 'West Loop High-Rise Condo #502', 'CONDO_TOWNHOME', '111 S Peoria St Unit 502', 'Chicago', 'IL', '60607',
        'RIXUS-17-17-302-045-1052', 2018, 1, 1280, '2024-06-01', 520000, 9800, 8000, 10000, 565000, 0.1200, 27.5,
        '{"water": "LANDLORD", "electric": "TENANT", "gas": "LANDLORD", "trash": "LANDLORD"}')
    RETURNING id INTO v_prop;

    -- Rent Roll
    INSERT INTO units (property_id, unit_number, floor_plan_name, bedrooms, bathrooms, rentable_sq_ft, status, is_renovated,
        market_rent, in_place_rent, monthly_parking_storage, security_deposit, lease_start_date, lease_end_date)
    VALUES (v_prop, '1', '2BR/2BA Luxury Corner', 2, 2, 1280, 'OCCUPIED', FALSE, 3400, 3300, 250, 3300, '2026-03-01', '2027-02-28');

    -- Tenant Profile
    INSERT INTO tenants (tenant_initials, property_id, unit_number, monthly_rent, pet_count, move_in_date, move_out_date, lease_start_date, lease_end_date, security_deposit)
    VALUES ('SG', v_prop, 1, 3300, 0, '2024-06-15 00:00:00Z', NULL, '2026-03-01 00:00:00Z', '2027-02-28 00:00:00Z', 3300);

    -- Financing
    INSERT INTO property_loans (property_id, lender_name, loan_type, original_loan_amount, current_principal_balance,
        interest_rate, amortization_months, loan_term_months, loan_origination_fees)
    VALUES (v_prop, 'Chase Home Lending', 'CONVENTIONAL', 416000, 408500.00, 0.06750, 360, 360, 4160);

    -- Underwriting Assumptions
    INSERT INTO underwriting_models (property_id, physical_vacancy_pct, credit_loss_concessions_pct, monthly_hoa_dues,
        annual_property_taxes, annual_insurance, annual_admin_legal_cpa, management_fee_pct_of_egi, repairs_maint_pct_of_egi,
        capex_reserve_pct_of_egi)
    VALUES (v_prop, 0.0300, 0.0050, 580, 8400, 850, 400, 0.0600, 0.0300, 0.0200);

    -- =========================================================================
    -- PROPERTY 4: Fulton Market Mixed-Use Commercial & Lofts (Mixed-Use)
    -- =========================================================================
    INSERT INTO properties (user_id, legal_entity_id, name, category, street_address, city, state, zip, parcel_pin,
        year_built, units, gross_sq_ft, acquisition_date, purchase_price, closing_costs, initial_rehab_budget,
        initial_operating_reserve, after_repair_value, land_value_allocation_pct, depreciation_schedule_years, utility_config)
    VALUES (v_user_id, v_holdco, 'Fulton Market Mixed-Use', 'MIXED_USE', '935 W Randolph St', 'Chicago', 'IL', '60607',
        'RIXUS-17-08-410-019', 1915, 3, 4500, '2025-03-15', 1650000, 32000, 120000, 35000, 1900000, 0.2500, 39.0,
        '{"water": "LANDLORD", "electric": "TENANT", "gas": "TENANT", "trash": "TENANT"}')
    RETURNING id INTO v_prop;

    -- Rent Roll
    INSERT INTO units (property_id, unit_number, floor_plan_name, bedrooms, bathrooms, rentable_sq_ft, status, is_renovated,
        market_rent, in_place_rent, monthly_rubs_charge, monthly_pet_rent, security_deposit, lease_start_date, lease_end_date)
    VALUES
        (v_prop, '1', 'Ground Floor Retail Storefront', 0, 1, 1800, 'OCCUPIED', FALSE, 5500, 5200, 0, 0, 10400, '2024-05-01', '2029-04-30'),
        (v_prop, '2', '2BR/2BA Timber Loft', 2, 2, 1350, 'OCCUPIED', TRUE, 3300, 3250, 80, 0, 3250, '2026-01-01', '2026-12-31'),
        (v_prop, '3', '2BR/2BA Penthouse Loft', 2, 2, 1350, 'OCCUPIED', TRUE, 3500, 3500, 80, 50, 3500, '2026-06-01', '2027-05-31');

    -- Tenants
    INSERT INTO tenants (tenant_initials, property_id, unit_number, monthly_rent, pet_count, move_in_date, move_out_date, lease_start_date, lease_end_date, security_deposit)
    VALUES
        ('RF', v_prop, 1, 5200, 0, '2024-05-01 00:00:00Z', NULL, '2024-05-01 00:00:00Z', '2029-04-30 00:00:00Z', 10400),
        ('JH', v_prop, 2, 3250, 0, '2025-01-01 00:00:00Z', NULL, '2026-01-01 00:00:00Z', '2026-12-31 00:00:00Z', 3250),
        ('BM', v_prop, 3, 3500, 1, '2025-06-01 00:00:00Z', NULL, '2026-06-01 00:00:00Z', '2027-05-31 00:00:00Z', 3500);

    -- Financing
    INSERT INTO property_loans (property_id, lender_name, loan_type, original_loan_amount, current_principal_balance,
        interest_rate, amortization_months, loan_term_months, interest_only_months, loan_origination_fees)
    VALUES (v_prop, 'Huntington National Bank', 'BRIDGE', 1320000, 1320000.00, 0.07850, 360, 24, 24, 26400);

    -- Underwriting Assumptions
    INSERT INTO underwriting_models (property_id, physical_vacancy_pct, credit_loss_concessions_pct, annual_property_taxes,
        annual_insurance, monthly_landlord_utilities, monthly_cam_lawn_snow_pest, annual_admin_legal_cpa,
        management_fee_pct_of_egi, repairs_maint_pct_of_egi, capex_reserve_pct_of_egi, exit_cap_rate_pct)
    VALUES (v_prop, 0.0600, 0.0100, 22400, 6500, 240, 160, 1400, 0.0600, 0.0350, 0.0300, 0.0675);

    -- =========================================================================
    -- PROPERTY 5: Oak Park Prairie Single-Family Home (Single Family)
    -- =========================================================================
    INSERT INTO properties (user_id, legal_entity_id, name, category, street_address, city, state, zip, parcel_pin,
        year_built, units, gross_sq_ft, acquisition_date, purchase_price, closing_costs, initial_rehab_budget,
        initial_operating_reserve, after_repair_value, land_value_allocation_pct, depreciation_schedule_years, utility_config)
    VALUES (v_user_id, v_personal, 'Oak Park Prairie Home', 'SINGLE_FAMILY', '318 S Kenilworth Ave', 'Oak Park', 'IL', '60302',
        'RIXUS-16-07-312-008', 1922, 1, 2400, '2023-08-20', 460000, 10500, 35000, 12000, 530000, 0.2500, 27.5,
        '{"water": "TENANT", "electric": "TENANT", "gas": "TENANT", "trash": "LANDLORD"}')
    RETURNING id INTO v_prop;

    -- Rent Roll
    INSERT INTO units (property_id, unit_number, floor_plan_name, bedrooms, bathrooms, rentable_sq_ft, status, is_renovated,
        market_rent, in_place_rent, monthly_pet_rent, security_deposit, lease_start_date, lease_end_date)
    VALUES (v_prop, '1', '4BR/2.5BA Prairie Home', 4, 2.5, 2400, 'OCCUPIED', TRUE, 3400, 3350, 50, 3350, '2025-09-01', '2027-08-31');

    -- Tenant Profile
    INSERT INTO tenants (tenant_initials, property_id, unit_number, monthly_rent, pet_count, move_in_date, move_out_date, lease_start_date, lease_end_date, security_deposit)
    VALUES ('WH', v_prop, 1, 3350, 1, '2023-09-01 00:00:00Z', NULL, '2025-09-01 00:00:00Z', '2027-08-31 00:00:00Z', 3350);

    -- Financing
    INSERT INTO property_loans (property_id, lender_name, loan_type, original_loan_amount, current_principal_balance,
        interest_rate, amortization_months, loan_term_months, loan_origination_fees)
    VALUES (v_prop, 'Byline Bank', 'CONVENTIONAL', 368000, 358200.00, 0.05875, 360, 360, 3680);

    -- Underwriting Assumptions
    INSERT INTO underwriting_models (property_id, physical_vacancy_pct, credit_loss_concessions_pct, annual_property_taxes,
        annual_insurance, monthly_landlord_utilities, monthly_cam_lawn_snow_pest, annual_admin_legal_cpa,
        management_fee_pct_of_egi, repairs_maint_pct_of_egi, capex_reserve_pct_of_egi)
    VALUES (v_prop, 0.0300, 0.0050, 9600, 1800, 45, 55, 450, 0.0000, 0.0400, 0.0400);

    -- =========================================================================
    -- PROPERTY 6: Logan Square 6-Unit Apartment Building (Multi-Family 5+)
    -- =========================================================================
    INSERT INTO properties (user_id, legal_entity_id, name, category, street_address, city, state, zip, parcel_pin,
        year_built, units, gross_sq_ft, acquisition_date, purchase_price, closing_costs, initial_rehab_budget,
        initial_operating_reserve, after_repair_value, land_value_allocation_pct, depreciation_schedule_years, utility_config)
    VALUES (v_user_id, v_series, 'Logan Square 6-Unit', 'MULTI_FAMILY_5_PLUS', '2844 N Sawyer Ave', 'Chicago', 'IL', '60618',
        'RIXUS-13-25-419-011', 1916, 6, 5900, '2024-09-15', 1380000, 28000, 85000, 30000, 1580000, 0.2500, 27.5,
        '{"water": "LANDLORD", "electric": "TENANT", "gas": "LANDLORD", "trash": "LANDLORD"}')
    RETURNING id INTO v_prop;

    -- Rent Roll
    INSERT INTO units (property_id, unit_number, floor_plan_name, bedrooms, bathrooms, rentable_sq_ft, status, is_renovated,
        market_rent, in_place_rent, monthly_rubs_charge, monthly_pet_rent, monthly_parking_storage, security_deposit, lease_start_date, lease_end_date)
    VALUES
        (v_prop, '1', '2BR/1BA Garden', 2, 1, 850, 'OCCUPIED', FALSE, 1650, 1600, 65, 0, 0, 1600, '2025-11-01', '2026-10-31'),
        (v_prop, '2', '2BR/1BA 1st Fl Front', 2, 1, 950, 'OCCUPIED', TRUE, 1900, 1850, 70, 25, 0, 1850, '2026-03-01', '2027-02-28'),
        (v_prop, '3', '2BR/1BA 1st Fl Rear', 2, 1, 950, 'OCCUPIED', FALSE, 1900, 1850, 70, 0, 0, 1850, '2026-01-01', '2026-12-31'),
        (v_prop, '4', '3BR/1BA 2nd Fl Front', 3, 1, 1050, 'OCCUPIED', TRUE, 2200, 2150, 75, 35, 85, 2150, '2026-07-01', '2027-06-30'),
        (v_prop, '5', '3BR/1BA 2nd Fl Rear', 3, 1, 1050, 'OCCUPIED', FALSE, 2200, 2100, 75, 0, 0, 2100, '2025-10-01', '2026-09-30'),
        (v_prop, '6', '2BR/1BA Coach House', 2, 1, 1050, 'VACANT_RENOVATION', TRUE, 2000, 0, 0, 0, 0, 0, NULL, NULL);

    -- Tenants
    INSERT INTO tenants (tenant_initials, property_id, unit_number, monthly_rent, pet_count, move_in_date, move_out_date, lease_start_date, lease_end_date, security_deposit)
    VALUES
        ('PL', v_prop, 1, 1600, 0, '2024-11-01 00:00:00Z', NULL, '2025-11-01 00:00:00Z', '2026-10-31 00:00:00Z', 1600),
        ('GK', v_prop, 2, 1850, 1, '2025-03-01 00:00:00Z', NULL, '2026-03-01 00:00:00Z', '2027-02-28 00:00:00Z', 1850),
        ('ER', v_prop, 3, 1850, 0, '2025-01-01 00:00:00Z', NULL, '2026-01-01 00:00:00Z', '2026-12-31 00:00:00Z', 1850),
        ('MS', v_prop, 4, 2150, 1, '2025-07-01 00:00:00Z', NULL, '2026-07-01 00:00:00Z', '2027-06-30 00:00:00Z', 2150),
        ('LC', v_prop, 5, 2100, 0, '2024-10-01 00:00:00Z', NULL, '2025-10-01 00:00:00Z', '2026-09-30 00:00:00Z', 2100);

    -- Financing
    INSERT INTO property_loans (property_id, lender_name, loan_type, original_loan_amount, current_principal_balance,
        interest_rate, amortization_months, loan_term_months, loan_origination_fees)
    VALUES
        (v_prop, 'Old Second National Bank', 'COMMERCIAL', 1035000, 1012800.00, 0.06625, 300, 60, 10350),
        (v_prop, 'Seller Carry - N. Kowalski', 'SELLER', 100000, 92000.00, 0.04000, 120, 60, 0);

    -- Underwriting Assumptions
    INSERT INTO underwriting_models (property_id, physical_vacancy_pct, credit_loss_concessions_pct, monthly_other_property_income,
        annual_property_taxes, annual_insurance, monthly_landlord_utilities, monthly_cam_lawn_snow_pest, annual_admin_legal_cpa,
        management_fee_pct_of_egi, repairs_maint_pct_of_egi, capex_reserve_pct_of_egi)
    VALUES (v_prop, 0.0500, 0.0100, 160, 18200, 5800, 820, 220, 1200, 0.0700, 0.0350, 0.0250);

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
    WHERE p.user_id = v_user_id AND (p.parcel_pin LIKE 'RIXUS-%' OR p.parcel_pin LIKE 'SAMPLE-%')
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
        WHERE p.user_id = v_user_id AND (p.parcel_pin LIKE 'RIXUS-%' OR p.parcel_pin LIKE 'SAMPLE-%') AND m.d >= p.acquisition_date
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
    WHERE p.user_id = v_user_id AND (p.parcel_pin LIKE 'RIXUS-%' OR p.parcel_pin LIKE 'SAMPLE-%') AND l.is_active
      AND c.amount > 0 AND m.d >= p.acquisition_date;

    RAISE NOTICE 'Migration 010 complete: populated 3 legal entities, 6 properties, 18 units, 15 active tenants, debt facilities, underwriting models, and 12-month ledger for rixusrealestate@gmail.com';
END $$;
