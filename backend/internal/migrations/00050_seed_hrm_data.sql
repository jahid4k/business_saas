-- +goose Up
-- +goose StatementBegin

-- ============================================================
-- Migration: 00050_seed_hrm_data
--
-- Seeds realistic HRM test data across every HRM module, on top
-- of the existing 'nexus-solutions' organization (see 00018).
-- Requires 00049 (adds 'award' to the approval action_type /
-- entity_type CHECK constraints) to run first.
--
-- ─── What gets created ───────────────────────────────────────
--   4   Users            (David, Rebecca, Tom, Hassan — dept heads,
--                          added as platform users so they can act
--                          as real "reporting_manager" approvers)
--   7   Departments       (incl. one nested: QA under Engineering)
--  15   Positions
--  20   Employees         (10 linked to platform users, 10 standalone —
--                          exercises hrm_employees.user_id nullability)
--   5   Leave types
--  12   Leave requests     (pending / approved / rejected / cancelled)
--   9   Salary components  (all 6 calc_methods incl. slab — exercises
--                          the progressive-bracket payroll fix)
--   3   Salary structures + structure-component junctions
--  20   Employee salary records (21 incl. Fatima's promotion revision)
--   6   Approval templates (all 4 approver_type variants; one 2-level
--                          chain; one non-default conditional template)
--   7   Approval instances (3 pending — termination/promotion/warning —
--                          1 pending transfer, 1 pending award,
--                          1 approved historical, 1 rejected historical)
--   2   Approval decisions (on the two completed historical instances)
--   4   Warning types      (2 requiring HR approval, one with its own
--                          approval_template_id)
--   2   Warning escalation rules
--   5   Employee warnings  (draft/issued/acknowledged/pending_approval/closed)
--   3   Complaints
--   4   Acknowledgements
--   5   Document templates
--   5   Employee documents (incl. one direct upload, one bulk-sent)
--   1   Document bulk send
--   2   Shifts + 3 work schedule assignments
--   1   Holiday calendar + 7 holidays + 1 calendar assignment
--  20   Employee contracts
--   4   Promotions   (draft / pending_approval / approved+applied / rejected)
--   3   Transfers    (draft / approved / pending_approval)
--   2   Resignations (accepted / withdrawn)
--   3   Terminations (pending_approval / applied / cancelled)
-- ~120  Attendance records (last 2 working weeks, generated via loop)
--   2   Attendance periods (finalized + open)
--   1   Payslip run + 5 payslips + ~31 payslip lines (hand-verified
--                          arithmetic, including the slab tax bracket)
--   3   Awards       (pending_approval / issued x2)
--   3   Announcements
--   4   Calendar events
--   4   Employee milestones
--
-- Design notes:
--   • Reuses the existing 'nexus-solutions' org and its 6 users
--     rather than creating a new org — HRM sits alongside CRM data
--     in the same tenant, which is how the product is actually used.
--   • Guard: skips entirely if HRM seed data already exists for
--     this org (checks for the 'Engineering' department).
--   • All currency fields explicitly set to 'USD' — the column
--     DEFAULTs to 'BDT' in several HRM tables, but nexus-solutions
--     itself is a USD/US-based org per 00018; leaving the default
--     would silently mix currencies within one org.
--   • Employee hierarchy and department heads are inserted in
--     dependency order (managers before reports) so manager_id /
--     head_employee_id can be set inline without a deferred update
--     pass, the same pattern the schema comments describe.
-- ============================================================

DO $$
DECLARE
    -- Existing org + users (from 00018)
    v_org_id       UUID;
    v_ayesha_id    UUID;  -- CEO (owner)
    v_sarah_id     UUID;  -- HR Manager (admin)
    v_mike_id      UUID;  -- Sales Manager (manager)
    v_priya_id     UUID;  -- Account Executive (member)
    v_james_id     UUID;  -- Account Executive (member)
    v_diana_id     UUID;  -- Marketing Specialist (viewer)
    v_role_manager_id UUID;

    -- New platform users (dept heads — need real logins to act as approvers)
    v_david_user_id    UUID;
    v_rebecca_user_id  UUID;
    v_tom_user_id      UUID;
    v_hassan_user_id   UUID;

    -- Departments
    v_dept_exec    UUID;
    v_dept_eng     UUID;
    v_dept_qa      UUID;
    v_dept_sales   UUID;
    v_dept_mktg    UUID;
    v_dept_hr      UUID;
    v_dept_support UUID;

    -- Positions
    v_pos_ceo          UUID;
    v_pos_vp_eng        UUID;
    v_pos_sr_swe        UUID;
    v_pos_swe           UUID;
    v_pos_qa_lead       UUID;
    v_pos_qa_eng        UUID;
    v_pos_sales_mgr     UUID;
    v_pos_ae            UUID;
    v_pos_sdr           UUID;
    v_pos_mktg_mgr      UUID;
    v_pos_mktg_spec     UUID;
    v_pos_hr_mgr        UUID;
    v_pos_hr_gen        UUID;
    v_pos_support_lead  UUID;
    v_pos_support_spec  UUID;

    -- Employees
    v_emp_ayesha   UUID;
    v_emp_sarah    UUID;
    v_emp_mike     UUID;
    v_emp_david    UUID;
    v_emp_rebecca  UUID;
    v_emp_tom      UUID;
    v_emp_hassan   UUID;
    v_emp_priya    UUID;
    v_emp_james    UUID;
    v_emp_sofia    UUID;
    v_emp_omar     UUID;
    v_emp_diana    UUID;
    v_emp_nadia    UUID;
    v_emp_zara     UUID;
    v_emp_fatima   UUID;
    v_emp_arjun    UUID;
    v_emp_kevin    UUID;
    v_emp_lisa     UUID;
    v_emp_daniel   UUID;
    v_emp_grace    UUID;

    -- Leave types
    v_lvt_annual   UUID;
    v_lvt_sick     UUID;
    v_lvt_casual   UUID;
    v_lvt_unpaid   UUID;
    v_lvt_parental UUID;

    -- Salary components
    v_comp_hra        UUID;
    v_comp_medical     UUID;
    v_comp_transport   UUID;
    v_comp_perfbonus   UUID;
    v_comp_commission  UUID;
    v_comp_healthins   UUID;
    v_comp_pf          UUID;
    v_comp_incometax   UUID;
    v_comp_employerpf  UUID;

    -- Salary structures
    v_struct_standard UUID;
    v_struct_senior   UUID;
    v_struct_sales    UUID;

    -- Approval templates
    v_tmpl_promotion   UUID;
    v_tmpl_transfer    UUID;
    v_tmpl_termination UUID;
    v_tmpl_warning     UUID;
    v_tmpl_award       UUID;
    v_tmpl_leave       UUID;

    -- Approval instances
    v_inst_sofia_term    UUID;
    v_inst_kevin_promo   UUID;
    v_inst_nadia_warn    UUID;
    v_inst_zara_transfer UUID;
    v_inst_arjun_award   UUID;
    v_inst_fatima_promo  UUID;  -- historical, approved
    v_inst_zara_promo    UUID;  -- historical, rejected

    -- Warning types
    v_wt_verbal             UUID;
    v_wt_written_attendance UUID;
    v_wt_written_conduct    UUID;
    v_wt_final               UUID;

    -- Warnings
    v_warn_nadia UUID;

    -- Document templates
    v_doctmpl_offer       UUID;
    v_doctmpl_promo       UUID;
    v_doctmpl_warning     UUID;
    v_doctmpl_termination UUID;
    v_doctmpl_policy      UUID;

    -- Employee documents
    v_doc_fatima_promo UUID;
    v_doc_daniel_term  UUID;
    v_doc_nadia_warn   UUID;
    v_doc_policy_diana UUID;
    v_doc_policy_arjun UUID;

    v_bulk_batch_id UUID := gen_random_uuid();

    -- Shifts
    v_shift_general UUID;
    v_shift_flex    UUID;

    -- Holiday calendar
    v_holcal_2026 UUID;

    -- Lifecycle records
    v_promo_fatima UUID;
    v_promo_kevin  UUID;
    v_promo_zara   UUID;
    v_promo_arjun  UUID;
    v_trf_omar     UUID;
    v_trf_grace    UUID;
    v_trf_zara     UUID;
    v_res_lisa     UUID;
    v_res_priya    UUID;
    v_term_sofia   UUID;
    v_term_daniel  UUID;
    v_term_arjun   UUID;

    -- Attendance
    v_attp_june UUID;
    v_attp_july UUID;
    v_att_date  DATE;
    v_att_emp   UUID;
    v_dow       INTEGER;

    -- Payroll
    v_payrun_june UUID;
    v_payslip_id  UUID;

    -- Awards / announcements / calendar / milestones
    v_award_arjun  UUID;
    v_award_priya  UUID;
    v_award_fatima UUID;
    v_ann_welcome  UUID;
    v_ann_policy   UUID;
    v_ann_awards   UUID;
    v_cal_anniv    UUID;
    v_cal_holiday  UUID;
    v_mil_fatima_anniv UUID;

BEGIN
    -- ──────────────────────────────────────────────────────────
    -- GUARD: skip if HRM seed data already exists (idempotent)
    -- ──────────────────────────────────────────────────────────
    SELECT id INTO v_org_id FROM organizations WHERE LOWER(slug) = 'nexus-solutions';
    IF v_org_id IS NULL THEN
        RAISE NOTICE 'nexus-solutions org not found — run 00018 first. Skipping 00050.';
        RETURN;
    END IF;
    IF EXISTS (SELECT 1 FROM hrm_departments WHERE org_id = v_org_id AND name = 'Engineering') THEN
        RAISE NOTICE 'HRM seed data already present for nexus-solutions — skipping 00050.';
        RETURN;
    END IF;

    SELECT id INTO v_ayesha_id FROM users WHERE LOWER(email) = 'ayesha@nexussolutions.io';
    SELECT id INTO v_sarah_id  FROM users WHERE LOWER(email) = 'sarah@nexussolutions.io';
    SELECT id INTO v_mike_id   FROM users WHERE LOWER(email) = 'mike@nexussolutions.io';
    SELECT id INTO v_priya_id  FROM users WHERE LOWER(email) = 'priya@nexussolutions.io';
    SELECT id INTO v_james_id  FROM users WHERE LOWER(email) = 'james@nexussolutions.io';
    SELECT id INTO v_diana_id  FROM users WHERE LOWER(email) = 'diana@nexussolutions.io';
    SELECT id INTO v_role_manager_id FROM roles WHERE org_id IS NULL AND name = 'manager';

    IF v_ayesha_id IS NULL OR v_role_manager_id IS NULL THEN
        RAISE NOTICE 'Expected users/roles from 00018 not found — skipping 00050.';
        RETURN;
    END IF;

    -- ==========================================================
    -- 0. NEW PLATFORM USERS — department heads (Password@123)
    -- ==========================================================
    INSERT INTO users (email, password_hash, first_name, last_name, display_name, full_name,
        email_verified, email_verified_at, status, timezone, locale, language, currency, phone,
        last_login_at, last_activity_at)
    VALUES
        ('david.chen@nexussolutions.io', crypt('Password@123', gen_salt('bf', 10)),
         'David','Chen','David Chen','David Chen', TRUE, NOW()-INTERVAL '700 days','active',
         'America/New_York','en','en','USD','+1-212-555-0201', NOW()-INTERVAL '1 day', NOW()-INTERVAL '1 day'),
        ('rebecca.stone@nexussolutions.io', crypt('Password@123', gen_salt('bf', 10)),
         'Rebecca','Stone','Rebecca Stone','Rebecca Stone', TRUE, NOW()-INTERVAL '690 days','active',
         'America/New_York','en','en','USD','+1-212-555-0202', NOW()-INTERVAL '2 days', NOW()-INTERVAL '2 days'),
        ('tom.bennett@nexussolutions.io', crypt('Password@123', gen_salt('bf', 10)),
         'Tom','Bennett','Tom Bennett','Tom Bennett', TRUE, NOW()-INTERVAL '660 days','active',
         'America/Chicago','en','en','USD','+1-312-555-0203', NOW()-INTERVAL '3 days', NOW()-INTERVAL '3 days'),
        ('hassan.ali@nexussolutions.io', crypt('Password@123', gen_salt('bf', 10)),
         'Hassan','Ali','Hassan Ali','Hassan Ali', TRUE, NOW()-INTERVAL '640 days','active',
         'America/New_York','en','en','USD','+1-212-555-0204', NOW()-INTERVAL '1 day', NOW()-INTERVAL '1 day')
    ON CONFLICT DO NOTHING;

    SELECT id INTO v_david_user_id   FROM users WHERE LOWER(email) = 'david.chen@nexussolutions.io';
    SELECT id INTO v_rebecca_user_id FROM users WHERE LOWER(email) = 'rebecca.stone@nexussolutions.io';
    SELECT id INTO v_tom_user_id     FROM users WHERE LOWER(email) = 'tom.bennett@nexussolutions.io';
    SELECT id INTO v_hassan_user_id  FROM users WHERE LOWER(email) = 'hassan.ali@nexussolutions.io';

    INSERT INTO organization_members (org_id, user_id, role_id, role_key, title, department, status,
        invitation_status, invitation_accepted_at, joined_at, created_at, updated_at)
    VALUES
        (v_org_id, v_david_user_id,   v_role_manager_id, 'manager', 'VP of Engineering',        'Engineering',       'active',
         'accepted', NOW()-INTERVAL '700 days', NOW()-INTERVAL '700 days', NOW()-INTERVAL '700 days', NOW()-INTERVAL '700 days'),
        (v_org_id, v_rebecca_user_id, v_role_manager_id, 'manager', 'Marketing Manager',         'Marketing',         'active',
         'accepted', NOW()-INTERVAL '690 days', NOW()-INTERVAL '690 days', NOW()-INTERVAL '690 days', NOW()-INTERVAL '690 days'),
        (v_org_id, v_tom_user_id,     v_role_manager_id, 'manager', 'Customer Support Lead',     'Customer Support',  'active',
         'accepted', NOW()-INTERVAL '660 days', NOW()-INTERVAL '660 days', NOW()-INTERVAL '660 days', NOW()-INTERVAL '660 days'),
        (v_org_id, v_hassan_user_id,  v_role_manager_id, 'manager', 'QA Lead',                   'Engineering',       'active',
         'accepted', NOW()-INTERVAL '640 days', NOW()-INTERVAL '640 days', NOW()-INTERVAL '640 days', NOW()-INTERVAL '640 days')
    ON CONFLICT (org_id, user_id) DO NOTHING;

    -- ==========================================================
    -- 1. DEPARTMENTS (7 — QA nested under Engineering)
    -- ==========================================================
    INSERT INTO hrm_departments (org_id, name, description, is_active, created_by) VALUES
        (v_org_id, 'Executive',        'Company leadership and strategy',                 TRUE, v_ayesha_id) RETURNING id INTO v_dept_exec;
    INSERT INTO hrm_departments (org_id, name, description, is_active, created_by) VALUES
        (v_org_id, 'Engineering',      'Product engineering and platform development',    TRUE, v_ayesha_id) RETURNING id INTO v_dept_eng;
    INSERT INTO hrm_departments (org_id, name, description, parent_department_id, is_active, created_by) VALUES
        (v_org_id, 'QA', 'Quality assurance and release testing', v_dept_eng, TRUE, v_ayesha_id) RETURNING id INTO v_dept_qa;
    INSERT INTO hrm_departments (org_id, name, description, is_active, created_by) VALUES
        (v_org_id, 'Sales',            'Revenue and account management',                  TRUE, v_ayesha_id) RETURNING id INTO v_dept_sales;
    INSERT INTO hrm_departments (org_id, name, description, is_active, created_by) VALUES
        (v_org_id, 'Marketing',        'Brand, demand generation, and content',           TRUE, v_ayesha_id) RETURNING id INTO v_dept_mktg;
    INSERT INTO hrm_departments (org_id, name, description, is_active, created_by) VALUES
        (v_org_id, 'Human Resources',  'People operations and talent',                    TRUE, v_ayesha_id) RETURNING id INTO v_dept_hr;
    INSERT INTO hrm_departments (org_id, name, description, is_active, created_by) VALUES
        (v_org_id, 'Customer Support', 'Customer success and technical support',          TRUE, v_ayesha_id) RETURNING id INTO v_dept_support;

    -- ==========================================================
    -- 2. POSITIONS (15)
    -- ==========================================================
    INSERT INTO hrm_positions (org_id, department_id, title, description, created_by) VALUES
        (v_org_id, v_dept_exec,    'Chief Executive Officer',        'Company leadership',                     v_ayesha_id) RETURNING id INTO v_pos_ceo;
    INSERT INTO hrm_positions (org_id, department_id, title, description, created_by) VALUES
        (v_org_id, v_dept_eng,     'VP of Engineering',              'Leads the engineering organization',     v_ayesha_id) RETURNING id INTO v_pos_vp_eng;
    INSERT INTO hrm_positions (org_id, department_id, title, description, created_by) VALUES
        (v_org_id, v_dept_eng,     'Senior Software Engineer',       'Senior individual contributor',          v_ayesha_id) RETURNING id INTO v_pos_sr_swe;
    INSERT INTO hrm_positions (org_id, department_id, title, description, created_by) VALUES
        (v_org_id, v_dept_eng,     'Software Engineer',              'Individual contributor',                 v_ayesha_id) RETURNING id INTO v_pos_swe;
    INSERT INTO hrm_positions (org_id, department_id, title, description, created_by) VALUES
        (v_org_id, v_dept_qa,      'QA Lead',                        'Leads quality assurance',                v_ayesha_id) RETURNING id INTO v_pos_qa_lead;
    INSERT INTO hrm_positions (org_id, department_id, title, description, created_by) VALUES
        (v_org_id, v_dept_qa,      'QA Engineer',                    'Manual and automated testing',           v_ayesha_id) RETURNING id INTO v_pos_qa_eng;
    INSERT INTO hrm_positions (org_id, department_id, title, description, created_by) VALUES
        (v_org_id, v_dept_sales,   'Sales Manager',                  'Leads the sales team',                   v_ayesha_id) RETURNING id INTO v_pos_sales_mgr;
    INSERT INTO hrm_positions (org_id, department_id, title, description, created_by) VALUES
        (v_org_id, v_dept_sales,   'Account Executive',              'Closes new business',                    v_ayesha_id) RETURNING id INTO v_pos_ae;
    INSERT INTO hrm_positions (org_id, department_id, title, description, created_by) VALUES
        (v_org_id, v_dept_sales,   'Sales Development Representative','Outbound pipeline generation',          v_ayesha_id) RETURNING id INTO v_pos_sdr;
    INSERT INTO hrm_positions (org_id, department_id, title, description, created_by) VALUES
        (v_org_id, v_dept_mktg,    'Marketing Manager',              'Leads marketing strategy',                v_ayesha_id) RETURNING id INTO v_pos_mktg_mgr;
    INSERT INTO hrm_positions (org_id, department_id, title, description, created_by) VALUES
        (v_org_id, v_dept_mktg,    'Marketing Specialist',           'Content and campaigns',                   v_ayesha_id) RETURNING id INTO v_pos_mktg_spec;
    INSERT INTO hrm_positions (org_id, department_id, title, description, created_by) VALUES
        (v_org_id, v_dept_hr,      'HR Manager',                     'Leads people operations',                 v_ayesha_id) RETURNING id INTO v_pos_hr_mgr;
    INSERT INTO hrm_positions (org_id, department_id, title, description, created_by) VALUES
        (v_org_id, v_dept_hr,      'HR Generalist',                  'Day-to-day HR operations',                v_ayesha_id) RETURNING id INTO v_pos_hr_gen;
    INSERT INTO hrm_positions (org_id, department_id, title, description, created_by) VALUES
        (v_org_id, v_dept_support, 'Customer Support Lead',          'Leads the support team',                  v_ayesha_id) RETURNING id INTO v_pos_support_lead;
    INSERT INTO hrm_positions (org_id, department_id, title, description, created_by) VALUES
        (v_org_id, v_dept_support, 'Support Specialist',             'Front-line customer support',             v_ayesha_id) RETURNING id INTO v_pos_support_spec;

    -- ==========================================================
    -- 3. EMPLOYEES (20 — inserted in manager-dependency order)
    -- ==========================================================

    -- Level 0
    INSERT INTO hrm_employees (org_id, user_id, employee_number, first_name, last_name, email, work_email,
        phone, gender, hire_date, employment_type, status, department_id, position_id, city, country, created_by)
    VALUES (v_org_id, v_ayesha_id, 'EMP-001', 'Ayesha','Rahman','ayesha@nexussolutions.io','ayesha@nexussolutions.io',
        '+1-212-555-0101','female','2024-01-15','full_time','active', v_dept_exec, v_pos_ceo, 'New York','US', v_ayesha_id)
    RETURNING id INTO v_emp_ayesha;

    -- Level 1 — report to Ayesha
    INSERT INTO hrm_employees (org_id, user_id, employee_number, first_name, last_name, email, work_email,
        phone, gender, hire_date, employment_type, status, department_id, position_id, manager_id, city, country, created_by)
    VALUES (v_org_id, v_sarah_id, 'EMP-002', 'Sarah','Thompson','sarah@nexussolutions.io','sarah@nexussolutions.io',
        '+1-415-555-0102','female','2024-02-01','full_time','active', v_dept_hr, v_pos_hr_mgr, v_emp_ayesha, 'San Francisco','US', v_ayesha_id)
    RETURNING id INTO v_emp_sarah;

    INSERT INTO hrm_employees (org_id, user_id, employee_number, first_name, last_name, email, work_email,
        phone, gender, hire_date, employment_type, status, department_id, position_id, manager_id, city, country, created_by)
    VALUES (v_org_id, v_mike_id, 'EMP-003', 'Mike','Karim','mike@nexussolutions.io','mike@nexussolutions.io',
        '+1-312-555-0103','male','2024-03-01','full_time','active', v_dept_sales, v_pos_sales_mgr, v_emp_ayesha, 'Chicago','US', v_ayesha_id)
    RETURNING id INTO v_emp_mike;

    INSERT INTO hrm_employees (org_id, user_id, employee_number, first_name, last_name, email, work_email,
        phone, gender, hire_date, employment_type, status, department_id, position_id, manager_id, city, country, created_by)
    VALUES (v_org_id, v_david_user_id, 'EMP-004', 'David','Chen','david.chen@nexussolutions.io','david.chen@nexussolutions.io',
        '+1-212-555-0201','male','2024-04-01','full_time','active', v_dept_eng, v_pos_vp_eng, v_emp_ayesha, 'New York','US', v_ayesha_id)
    RETURNING id INTO v_emp_david;

    INSERT INTO hrm_employees (org_id, user_id, employee_number, first_name, last_name, email, work_email,
        phone, gender, hire_date, employment_type, status, department_id, position_id, manager_id, city, country, created_by)
    VALUES (v_org_id, v_rebecca_user_id, 'EMP-005', 'Rebecca','Stone','rebecca.stone@nexussolutions.io','rebecca.stone@nexussolutions.io',
        '+1-212-555-0202','female','2024-08-15','full_time','active', v_dept_mktg, v_pos_mktg_mgr, v_emp_ayesha, 'New York','US', v_ayesha_id)
    RETURNING id INTO v_emp_rebecca;

    INSERT INTO hrm_employees (org_id, user_id, employee_number, first_name, last_name, email, work_email,
        phone, gender, hire_date, employment_type, status, department_id, position_id, manager_id, city, country, created_by)
    VALUES (v_org_id, v_tom_user_id, 'EMP-006', 'Tom','Bennett','tom.bennett@nexussolutions.io','tom.bennett@nexussolutions.io',
        '+1-312-555-0203','male','2024-09-01','full_time','active', v_dept_support, v_pos_support_lead, v_emp_ayesha, 'Chicago','US', v_ayesha_id)
    RETURNING id INTO v_emp_tom;

    -- Level 2 — report to level-1 managers
    INSERT INTO hrm_employees (org_id, user_id, employee_number, first_name, last_name, email, work_email,
        phone, gender, hire_date, employment_type, status, department_id, position_id, manager_id, city, country, created_by)
    VALUES (v_org_id, v_hassan_user_id, 'EMP-007', 'Hassan','Ali','hassan.ali@nexussolutions.io','hassan.ali@nexussolutions.io',
        '+1-212-555-0204','male','2024-10-01','full_time','active', v_dept_qa, v_pos_qa_lead, v_emp_david, 'New York','US', v_ayesha_id)
    RETURNING id INTO v_emp_hassan;

    INSERT INTO hrm_employees (org_id, user_id, employee_number, first_name, last_name, email, work_email,
        phone, gender, hire_date, employment_type, status, department_id, position_id, manager_id, city, country, created_by)
    VALUES (v_org_id, v_priya_id, 'EMP-008', 'Priya','Patel','priya@nexussolutions.io','priya@nexussolutions.io',
        '+1-646-555-0104','female','2024-05-01','full_time','active', v_dept_sales, v_pos_ae, v_emp_mike, 'New York','US', v_ayesha_id)
    RETURNING id INTO v_emp_priya;

    INSERT INTO hrm_employees (org_id, user_id, employee_number, first_name, last_name, email, work_email,
        phone, gender, hire_date, employment_type, status, department_id, position_id, manager_id, city, country, created_by)
    VALUES (v_org_id, v_james_id, 'EMP-009', 'James','Wilson','james@nexussolutions.io','james@nexussolutions.io',
        '+1-720-555-0105','male','2024-11-01','full_time','active', v_dept_sales, v_pos_ae, v_emp_mike, 'Denver','US', v_ayesha_id)
    RETURNING id INTO v_emp_james;

    -- Sofia Reyes — Account Executive, subject of a pending termination
    INSERT INTO hrm_employees (org_id, employee_number, first_name, last_name, email, work_email,
        phone, gender, hire_date, employment_type, status, department_id, position_id, manager_id, city, country, created_by)
    VALUES (v_org_id, 'EMP-010', 'Sofia','Reyes','s.reyes@nexussolutions.io','s.reyes@nexussolutions.io',
        '+1-305-555-0110','female','2024-06-01','full_time','active', v_dept_sales, v_pos_ae, v_emp_mike, 'Miami','US', v_ayesha_id)
    RETURNING id INTO v_emp_sofia;

    -- Omar Farouk — SDR, new hire on probation
    INSERT INTO hrm_employees (org_id, employee_number, first_name, last_name, email, work_email,
        phone, gender, hire_date, employment_type, status, department_id, position_id, manager_id, city, country, created_by)
    VALUES (v_org_id, 'EMP-011', 'Omar','Farouk','o.farouk@nexussolutions.io','o.farouk@nexussolutions.io',
        '+1-646-555-0111','male','2026-06-01','full_time','active', v_dept_sales, v_pos_sdr, v_emp_mike, 'New York','US', v_ayesha_id)
    RETURNING id INTO v_emp_omar;

    INSERT INTO hrm_employees (org_id, user_id, employee_number, first_name, last_name, email, work_email,
        phone, gender, hire_date, employment_type, status, department_id, position_id, manager_id, city, country, created_by)
    VALUES (v_org_id, v_diana_id, 'EMP-012', 'Diana','Lee','diana@nexussolutions.io','diana@nexussolutions.io',
        '+1-323-555-0106','female','2025-01-15','full_time','active', v_dept_mktg, v_pos_mktg_spec, v_emp_rebecca, 'Los Angeles','US', v_ayesha_id)
    RETURNING id INTO v_emp_diana;

    -- Nadia Islam — HR Generalist, subject of a pending (approval-gated) warning
    INSERT INTO hrm_employees (org_id, employee_number, first_name, last_name, email, work_email,
        phone, gender, hire_date, employment_type, status, department_id, position_id, manager_id, city, country, created_by)
    VALUES (v_org_id, 'EMP-013', 'Nadia','Islam','n.islam@nexussolutions.io','n.islam@nexussolutions.io',
        '+1-415-555-0113','female','2025-04-01','full_time','active', v_dept_hr, v_pos_hr_gen, v_emp_sarah, 'San Francisco','US', v_ayesha_id)
    RETURNING id INTO v_emp_nadia;

    -- Zara Ahmed — Support Specialist, subject of a pending dept transfer
    INSERT INTO hrm_employees (org_id, employee_number, first_name, last_name, email, work_email,
        phone, gender, hire_date, employment_type, status, department_id, position_id, manager_id, city, country, created_by)
    VALUES (v_org_id, 'EMP-014', 'Zara','Ahmed','z.ahmed@nexussolutions.io','z.ahmed@nexussolutions.io',
        '+1-312-555-0114','female','2025-03-01','full_time','active', v_dept_support, v_pos_support_spec, v_emp_tom, 'Chicago','US', v_ayesha_id)
    RETURNING id INTO v_emp_zara;

    -- Fatima Noor — Senior SWE, already promoted (historical, approved)
    INSERT INTO hrm_employees (org_id, employee_number, first_name, last_name, email, work_email,
        phone, gender, hire_date, employment_type, status, department_id, position_id, manager_id, city, country, created_by)
    VALUES (v_org_id, 'EMP-015', 'Fatima','Noor','f.noor@nexussolutions.io','f.noor@nexussolutions.io',
        '+1-212-555-0115','female','2023-03-10','full_time','active', v_dept_eng, v_pos_sr_swe, v_emp_david, 'New York','US', v_ayesha_id)
    RETURNING id INTO v_emp_fatima;

    INSERT INTO hrm_employees (org_id, employee_number, first_name, last_name, email, work_email,
        phone, gender, hire_date, employment_type, status, department_id, position_id, manager_id, city, country, created_by)
    VALUES (v_org_id, 'EMP-016', 'Arjun','Mehta','a.mehta@nexussolutions.io','a.mehta@nexussolutions.io',
        '+1-212-555-0116','male','2024-09-01','full_time','active', v_dept_eng, v_pos_swe, v_emp_david, 'New York','US', v_ayesha_id)
    RETURNING id INTO v_emp_arjun;

    -- Kevin Brooks — Software Engineer, subject of a pending promotion
    INSERT INTO hrm_employees (org_id, employee_number, first_name, last_name, email, work_email,
        phone, gender, hire_date, employment_type, status, department_id, position_id, manager_id, city, country, created_by)
    VALUES (v_org_id, 'EMP-017', 'Kevin','Brooks','k.brooks@nexussolutions.io','k.brooks@nexussolutions.io',
        '+1-212-555-0117','male','2024-07-01','full_time','active', v_dept_eng, v_pos_swe, v_emp_david, 'New York','US', v_ayesha_id)
    RETURNING id INTO v_emp_kevin;

    -- Lisa Wong — Software Engineer, resignation accepted, notice period in progress
    INSERT INTO hrm_employees (org_id, employee_number, first_name, last_name, email, work_email,
        phone, gender, hire_date, employment_type, status, department_id, position_id, manager_id, city, country, created_by)
    VALUES (v_org_id, 'EMP-018', 'Lisa','Wong','l.wong@nexussolutions.io','l.wong@nexussolutions.io',
        '+1-212-555-0118','female','2023-08-01','full_time','resigned', v_dept_eng, v_pos_swe, v_emp_david, 'New York','US', v_ayesha_id)
    RETURNING id INTO v_emp_lisa;

    -- Daniel Osei — Software Engineer, already terminated (probation_fail, historical)
    INSERT INTO hrm_employees (org_id, employee_number, first_name, last_name, email, work_email,
        phone, gender, hire_date, termination_date, employment_type, status, department_id, position_id, manager_id, city, country, created_by)
    VALUES (v_org_id, 'EMP-019', 'Daniel','Osei','d.osei@nexussolutions.io','d.osei@nexussolutions.io',
        '+1-212-555-0119','male','2025-05-01','2026-06-15','full_time','terminated', v_dept_eng, v_pos_swe, v_emp_david, 'New York','US', v_ayesha_id)
    RETURNING id INTO v_emp_daniel;

    -- Level 3 — Grace reports to Hassan (QA)
    INSERT INTO hrm_employees (org_id, employee_number, first_name, last_name, email, work_email,
        phone, gender, hire_date, employment_type, status, department_id, position_id, manager_id, city, country, created_by)
    VALUES (v_org_id, 'EMP-020', 'Grace','Kim','g.kim@nexussolutions.io','g.kim@nexussolutions.io',
        '+1-212-555-0120','female','2025-02-01','full_time','active', v_dept_qa, v_pos_qa_eng, v_emp_hassan, 'New York','US', v_ayesha_id)
    RETURNING id INTO v_emp_grace;

    -- Back-fill department heads now that employees exist
    UPDATE hrm_departments SET head_employee_id = v_emp_ayesha  WHERE id = v_dept_exec;
    UPDATE hrm_departments SET head_employee_id = v_emp_david   WHERE id = v_dept_eng;
    UPDATE hrm_departments SET head_employee_id = v_emp_hassan  WHERE id = v_dept_qa;
    UPDATE hrm_departments SET head_employee_id = v_emp_mike    WHERE id = v_dept_sales;
    UPDATE hrm_departments SET head_employee_id = v_emp_rebecca WHERE id = v_dept_mktg;
    UPDATE hrm_departments SET head_employee_id = v_emp_sarah   WHERE id = v_dept_hr;
    UPDATE hrm_departments SET head_employee_id = v_emp_tom     WHERE id = v_dept_support;

    -- ==========================================================
    -- 4. LEAVE TYPES (5)
    -- ==========================================================
    INSERT INTO hrm_leave_types (org_id, name, description, max_days_per_year, is_paid, requires_approval, created_by) VALUES
        (v_org_id, 'Annual Leave',   'Planned vacation time',            20, TRUE,  TRUE, v_ayesha_id) RETURNING id INTO v_lvt_annual;
    INSERT INTO hrm_leave_types (org_id, name, description, max_days_per_year, is_paid, requires_approval, created_by) VALUES
        (v_org_id, 'Sick Leave',     'Illness or medical appointments',  10, TRUE,  TRUE, v_ayesha_id) RETURNING id INTO v_lvt_sick;
    INSERT INTO hrm_leave_types (org_id, name, description, max_days_per_year, is_paid, requires_approval, created_by) VALUES
        (v_org_id, 'Casual Leave',   'Short-notice personal time off',    7, TRUE,  TRUE, v_ayesha_id) RETURNING id INTO v_lvt_casual;
    INSERT INTO hrm_leave_types (org_id, name, description, max_days_per_year, is_paid, requires_approval, created_by) VALUES
        (v_org_id, 'Unpaid Leave',   'Extended leave without pay',        0, FALSE, TRUE, v_ayesha_id) RETURNING id INTO v_lvt_unpaid;
    INSERT INTO hrm_leave_types (org_id, name, description, max_days_per_year, is_paid, requires_approval, created_by) VALUES
        (v_org_id, 'Parental Leave', 'Maternity, paternity, and adoption leave', 90, TRUE, TRUE, v_ayesha_id) RETURNING id INTO v_lvt_parental;

    -- ==========================================================
    -- 5. LEAVE REQUESTS (12)
    -- ==========================================================
    -- Note: created_by references users(id), not hrm_employees(id). Standalone
    -- employees (no platform login) can't be their own created_by, so those rows
    -- use whichever manager/HR user would realistically have entered it for them.
    INSERT INTO hrm_leave_requests (org_id, employee_id, leave_type_id, start_date, end_date, total_days, reason, status, reviewed_by, reviewed_at, review_note, created_by) VALUES
        (v_org_id, v_emp_arjun,  v_lvt_annual, CURRENT_DATE+14, CURRENT_DATE+18, 5, 'Family trip',            'approved', v_david_user_id,   NOW()-INTERVAL '5 days',  'Approved — coverage arranged with Kevin', v_david_user_id),
        (v_org_id, v_emp_grace,  v_lvt_sick,   CURRENT_DATE-3,  CURRENT_DATE-2,  2, 'Flu',                     'approved', v_hassan_user_id,  NOW()-INTERVAL '3 days',  'Get well soon',                            v_hassan_user_id),
        (v_org_id, v_emp_priya,  v_lvt_annual, CURRENT_DATE+30, CURRENT_DATE+32, 3, 'Long weekend',            'pending',  NULL,              NULL,                     NULL,                                        v_priya_id),
        (v_org_id, v_emp_james,  v_lvt_casual, CURRENT_DATE+5,  CURRENT_DATE+5,  1, 'Personal errand',         'approved', v_mike_id,         NOW()-INTERVAL '1 day',   'Approved',                                  v_james_id),
        (v_org_id, v_emp_diana,  v_lvt_annual, CURRENT_DATE+7,  CURRENT_DATE+10, 4, 'Conference conflict week','rejected', v_rebecca_user_id, NOW()-INTERVAL '2 days',  'Overlaps product launch week — please pick another week', v_diana_id),
        (v_org_id, v_emp_nadia,  v_lvt_sick,   CURRENT_DATE-10, CURRENT_DATE-10, 1, 'Doctor appointment',      'approved', v_sarah_id,        NOW()-INTERVAL '10 days', 'Approved',                                  v_sarah_id),
        (v_org_id, v_emp_zara,   v_lvt_unpaid, CURRENT_DATE+45, CURRENT_DATE+54, 10,'Family emergency abroad', 'pending',  NULL,              NULL,                     NULL,                                        v_tom_user_id),
        (v_org_id, v_emp_omar,   v_lvt_casual, CURRENT_DATE+3,  CURRENT_DATE+3,  1, 'Apartment move',          'approved', v_mike_id,         NOW()-INTERVAL '1 day',   'Approved',                                  v_mike_id),
        (v_org_id, v_emp_fatima, v_lvt_annual, CURRENT_DATE+20, CURRENT_DATE+26, 7, 'Summer vacation',         'approved', v_david_user_id,   NOW()-INTERVAL '4 days',  'Enjoy!',                                    v_david_user_id),
        (v_org_id, v_emp_kevin,  v_lvt_sick,   CURRENT_DATE-20, CURRENT_DATE-18, 3, 'Recovering from surgery', 'approved', v_david_user_id,   NOW()-INTERVAL '20 days', 'Approved — take the time you need',        v_david_user_id),
        (v_org_id, v_emp_hassan, v_lvt_annual, CURRENT_DATE+60, CURRENT_DATE+64, 5, 'Wedding',                 'cancelled',v_david_user_id,   NOW()-INTERVAL '6 days',  'Cancelled by employee — date moved',       v_hassan_user_id),
        (v_org_id, v_emp_grace,  v_lvt_casual, CURRENT_DATE+2,  CURRENT_DATE+2,  1, 'Dentist',                 'pending',  NULL,              NULL,                     NULL,                                        v_hassan_user_id);

    -- ==========================================================
    -- 6. SALARY COMPONENTS (9 — every calc_method, incl. slab)
    -- ==========================================================
    INSERT INTO hrm_salary_components (org_id, name, description, component_type, calc_method, fixed_value, is_taxable, display_order, created_by) VALUES
        (v_org_id, 'House Rent Allowance', 'Housing allowance, % of basic pay', 'earning', 'pct_of_basic', 40, TRUE, 1, v_ayesha_id) RETURNING id INTO v_comp_hra;
    INSERT INTO hrm_salary_components (org_id, name, description, component_type, calc_method, fixed_value, is_taxable, display_order, created_by) VALUES
        (v_org_id, 'Medical Allowance', 'Fixed monthly medical allowance', 'earning', 'fixed', 300, FALSE, 2, v_ayesha_id) RETURNING id INTO v_comp_medical;
    INSERT INTO hrm_salary_components (org_id, name, description, component_type, calc_method, fixed_value, is_taxable, display_order, created_by) VALUES
        (v_org_id, 'Transport Allowance', 'Fixed monthly transport allowance', 'earning', 'fixed', 150, FALSE, 3, v_ayesha_id) RETURNING id INTO v_comp_transport;
    INSERT INTO hrm_salary_components (org_id, name, description, component_type, calc_method, formula_expression, formula_variables, is_taxable, display_order, created_by) VALUES
        (v_org_id, 'Performance Bonus', 'Tenure-gated performance bonus, 10% of basic once past 1 year',
         'earning', 'formula', 'TENURE_YEARS >= 1 ? BASIC * 0.10 : 0', ARRAY['BASIC','TENURE_YEARS'], TRUE, 4, v_ayesha_id) RETURNING id INTO v_comp_perfbonus;
    INSERT INTO hrm_salary_components (org_id, name, description, component_type, calc_method, fixed_value, is_taxable, display_order, created_by) VALUES
        (v_org_id, 'Sales Commission', 'Entered manually per payroll run by HR/finance', 'earning', 'manual', 0, TRUE, 4, v_ayesha_id) RETURNING id INTO v_comp_commission;
    INSERT INTO hrm_salary_components (org_id, name, description, component_type, calc_method, fixed_value, is_taxable, display_order, created_by) VALUES
        (v_org_id, 'Health Insurance Premium', 'Employee-paid share of group health premium, % of gross',
         'deduction', 'pct_of_gross', 2, FALSE, 10, v_ayesha_id) RETURNING id INTO v_comp_healthins;
    INSERT INTO hrm_salary_components (org_id, name, description, component_type, calc_method, fixed_value, is_taxable, display_order, created_by) VALUES
        (v_org_id, 'Provident Fund', 'Employee retirement contribution, % of basic', 'deduction', 'pct_of_basic', 8, FALSE, 11, v_ayesha_id) RETURNING id INTO v_comp_pf;
    INSERT INTO hrm_salary_components (org_id, name, description, component_type, calc_method, slab_config, is_taxable, display_order, created_by) VALUES
        (v_org_id, 'Income Tax', 'Progressive income tax withholding on gross pay', 'deduction', 'slab',
         '{"base_variable":"GROSS","slabs":[{"up_to":3000,"rate":0.0},{"up_to":6000,"rate":0.10},{"up_to":12000,"rate":0.20},{"up_to":null,"rate":0.30}]}'::jsonb,
         FALSE, 12, v_ayesha_id) RETURNING id INTO v_comp_incometax;
    INSERT INTO hrm_salary_components (org_id, name, description, component_type, calc_method, fixed_value, is_taxable, display_order, created_by) VALUES
        (v_org_id, 'Employer PF Contribution', 'Employer-matched retirement contribution, % of basic — informational, not deducted from employee',
         'employer_contribution', 'pct_of_basic', 8, FALSE, 20, v_ayesha_id) RETURNING id INTO v_comp_employerpf;

    -- ==========================================================
    -- 7. SALARY STRUCTURES (3) + STRUCTURE COMPONENTS
    -- ==========================================================
    INSERT INTO hrm_salary_structures (org_id, name, description, grade_label, created_by) VALUES
        (v_org_id, 'Standard Employee', 'Default structure for individual contributors', 'Standard', v_ayesha_id) RETURNING id INTO v_struct_standard;
    INSERT INTO hrm_salary_structures (org_id, name, description, grade_label, created_by) VALUES
        (v_org_id, 'Senior & Management', 'For senior ICs, leads, and managers', 'Senior', v_ayesha_id) RETURNING id INTO v_struct_senior;
    INSERT INTO hrm_salary_structures (org_id, name, description, grade_label, created_by) VALUES
        (v_org_id, 'Sales Team', 'Includes commission component', 'Sales', v_ayesha_id) RETURNING id INTO v_struct_sales;

    -- Standard Employee: HRA, Medical, Transport, PF, Income Tax
    INSERT INTO hrm_salary_structure_components (structure_id, component_id, display_order) VALUES
        (v_struct_standard, v_comp_hra, 1), (v_struct_standard, v_comp_medical, 2), (v_struct_standard, v_comp_transport, 3),
        (v_struct_standard, v_comp_pf, 11), (v_struct_standard, v_comp_incometax, 12);

    -- Senior & Management: HRA, Medical(+override), Transport(+override), Perf Bonus, Health Ins, PF, Income Tax, Employer PF
    INSERT INTO hrm_salary_structure_components (structure_id, component_id, override_value, display_order) VALUES
        (v_struct_senior, v_comp_hra, NULL, 1),
        (v_struct_senior, v_comp_medical, 400, 2),
        (v_struct_senior, v_comp_transport, 200, 3),
        (v_struct_senior, v_comp_perfbonus, NULL, 4),
        (v_struct_senior, v_comp_healthins, NULL, 10),
        (v_struct_senior, v_comp_pf, NULL, 11),
        (v_struct_senior, v_comp_incometax, NULL, 12),
        (v_struct_senior, v_comp_employerpf, NULL, 20);

    -- Sales Team: HRA, Medical, Transport, Commission, PF, Income Tax
    INSERT INTO hrm_salary_structure_components (structure_id, component_id, display_order) VALUES
        (v_struct_sales, v_comp_hra, 1), (v_struct_sales, v_comp_medical, 2), (v_struct_sales, v_comp_transport, 3),
        (v_struct_sales, v_comp_commission, 4), (v_struct_sales, v_comp_pf, 11), (v_struct_sales, v_comp_incometax, 12);

    -- ==========================================================
    -- 8. EMPLOYEE SALARY RECORDS (21 — Fatima has 2: joining + promotion revision)
    -- ==========================================================
    INSERT INTO hrm_employee_salary_records (org_id, employee_id, structure_id, basic_pay, effective_date, change_reason, created_by) VALUES
        (v_org_id, v_emp_ayesha,  v_struct_senior,   15000, '2024-01-15', 'joining', v_ayesha_id),
        (v_org_id, v_emp_sarah,   v_struct_senior,   10500, '2024-02-01', 'joining', v_ayesha_id),
        (v_org_id, v_emp_mike,    v_struct_senior,   11000, '2024-03-01', 'joining', v_ayesha_id),
        (v_org_id, v_emp_david,   v_struct_senior,   12000, '2024-04-01', 'joining', v_ayesha_id),
        (v_org_id, v_emp_rebecca, v_struct_senior,    9800, '2024-08-15', 'joining', v_ayesha_id),
        (v_org_id, v_emp_tom,     v_struct_senior,    8500, '2024-09-01', 'joining', v_ayesha_id),
        (v_org_id, v_emp_hassan,  v_struct_senior,    9000, '2024-10-01', 'joining', v_ayesha_id),
        (v_org_id, v_emp_priya,   v_struct_sales,     5500, '2024-05-01', 'joining', v_ayesha_id),
        (v_org_id, v_emp_james,   v_struct_sales,     5200, '2024-11-01', 'joining', v_ayesha_id),
        (v_org_id, v_emp_sofia,   v_struct_sales,     5000, '2024-06-01', 'joining', v_ayesha_id),
        (v_org_id, v_emp_omar,    v_struct_sales,     3500, '2026-06-01', 'joining', v_ayesha_id),
        (v_org_id, v_emp_diana,   v_struct_standard,  4800, '2025-01-15', 'joining', v_ayesha_id),
        (v_org_id, v_emp_nadia,   v_struct_standard,  4500, '2025-04-01', 'joining', v_ayesha_id),
        (v_org_id, v_emp_zara,    v_struct_standard,  3800, '2025-03-01', 'joining', v_ayesha_id),
        (v_org_id, v_emp_fatima,  v_struct_standard,  7500, '2023-03-10', 'joining', v_ayesha_id),
        (v_org_id, v_emp_arjun,   v_struct_standard,  6500, '2024-09-01', 'joining', v_ayesha_id),
        (v_org_id, v_emp_kevin,   v_struct_standard,  6800, '2024-07-01', 'joining', v_ayesha_id),
        (v_org_id, v_emp_lisa,    v_struct_standard,  6900, '2023-08-01', 'joining', v_ayesha_id),
        (v_org_id, v_emp_daniel,  v_struct_standard,  6200, '2025-05-01', 'joining', v_ayesha_id),
        (v_org_id, v_emp_grace,   v_struct_standard,  4200, '2025-02-01', 'joining', v_ayesha_id),
        -- Fatima's promotion revision — becomes her active record going forward
        (v_org_id, v_emp_fatima,  v_struct_senior,    9500, CURRENT_DATE-INTERVAL '150 days', 'promotion', v_ayesha_id);

    -- ==========================================================
    -- 9. APPROVAL TEMPLATES (6) + LEVELS
    -- ==========================================================
    INSERT INTO hrm_approval_templates (org_id, name, description, action_type, is_default, created_by) VALUES
        (v_org_id, 'Standard Promotion Approval', 'Reporting manager signs off on all promotions', 'promotion', TRUE, v_ayesha_id) RETURNING id INTO v_tmpl_promotion;
    INSERT INTO hrm_approval_template_levels (template_id, level, approver_type, sla_hours, on_sla_breach) VALUES
        (v_tmpl_promotion, 1, 'reporting_manager', 48, 'escalate_next');

    INSERT INTO hrm_approval_templates (org_id, name, description, action_type, is_default, created_by) VALUES
        (v_org_id, 'Standard Transfer Approval', 'Destination department head approves incoming transfers', 'transfer', TRUE, v_ayesha_id) RETURNING id INTO v_tmpl_transfer;
    INSERT INTO hrm_approval_template_levels (template_id, level, approver_type, sla_hours, on_sla_breach) VALUES
        (v_tmpl_transfer, 1, 'dept_head', 48, 'escalate_next');

    INSERT INTO hrm_approval_templates (org_id, name, description, action_type, is_default, created_by) VALUES
        (v_org_id, 'Termination Approval Chain', 'Manager review, then HR admin sign-off', 'termination', TRUE, v_ayesha_id) RETURNING id INTO v_tmpl_termination;
    INSERT INTO hrm_approval_template_levels (template_id, level, approver_type, sla_hours, on_sla_breach) VALUES
        (v_tmpl_termination, 1, 'reporting_manager', 48, 'escalate_next');
    INSERT INTO hrm_approval_template_levels (template_id, level, approver_type, approver_role, sla_hours, on_sla_breach) VALUES
        (v_tmpl_termination, 2, 'role', 'admin', 72, 'auto_approve');

    INSERT INTO hrm_approval_templates (org_id, name, description, action_type, is_default, created_by) VALUES
        (v_org_id, 'Warning HR Review', 'HR admin reviews before a serious warning is issued', 'warning', TRUE, v_ayesha_id) RETURNING id INTO v_tmpl_warning;
    INSERT INTO hrm_approval_template_levels (template_id, level, approver_type, approver_role, sla_hours, on_sla_breach) VALUES
        (v_tmpl_warning, 1, 'role', 'admin', 24, 'escalate_next');

    INSERT INTO hrm_approval_templates (org_id, name, description, action_type, is_default, created_by) VALUES
        (v_org_id, 'Recognition Award Sign-off', 'Founder signs off on recognition awards', 'award', TRUE, v_ayesha_id) RETURNING id INTO v_tmpl_award;
    INSERT INTO hrm_approval_template_levels (template_id, level, approver_type, approver_user_id, sla_hours, on_sla_breach) VALUES
        (v_tmpl_award, 1, 'specific_user', v_ayesha_id, 72, 'auto_approve');

    -- Non-default, condition-gated template — demonstrates condition_expression + is_default=false.
    -- Not currently consumed by the leave module (hrm_leave_requests uses its own
    -- reviewed_by/status flow, not hrm_approval_instances) — included here for
    -- configuration/CRUD testing of the Approval Chains UI, not an active code path.
    INSERT INTO hrm_approval_templates (org_id, name, description, action_type, condition_expression, is_default, created_by) VALUES
        (v_org_id, 'Extended Leave Approval', 'For leave requests longer than 10 days', 'leave', 'total_days > 10', FALSE, v_ayesha_id) RETURNING id INTO v_tmpl_leave;
    INSERT INTO hrm_approval_template_levels (template_id, level, approver_type, approver_role, sla_hours, on_sla_breach) VALUES
        (v_tmpl_leave, 1, 'role', 'admin', 24, 'escalate_next');

    -- ==========================================================
    -- 10. APPROVAL INSTANCES (7) + DECISIONS (2)
    -- ==========================================================
    -- Pending: Sofia's termination, level 1 of 2 (reporting_manager = Mike)
    INSERT INTO hrm_approval_instances (org_id, template_id, entity_type, entity_id, instance_snapshot, current_level, overall_status, requested_by) VALUES
        (v_org_id, v_tmpl_termination, 'termination', gen_random_uuid(), -- placeholder, corrected below
         '[{"level":1,"approver_type":"reporting_manager","sla_hours":48,"on_sla_breach":"escalate_next"},{"level":2,"approver_type":"role","approver_role":"admin","sla_hours":72,"on_sla_breach":"auto_approve"}]'::jsonb,
         1, 'pending', v_mike_id)
        RETURNING id INTO v_inst_sofia_term;

    -- Pending: Kevin's promotion, level 1 of 1 (reporting_manager = David)
    INSERT INTO hrm_approval_instances (org_id, template_id, entity_type, entity_id, instance_snapshot, current_level, overall_status, requested_by) VALUES
        (v_org_id, v_tmpl_promotion, 'promotion', gen_random_uuid(),
         '[{"level":1,"approver_type":"reporting_manager","sla_hours":48,"on_sla_breach":"escalate_next"}]'::jsonb,
         1, 'pending', v_david_user_id)
        RETURNING id INTO v_inst_kevin_promo;

    -- Pending: Nadia's warning, level 1 of 1 (role=admin = Sarah)
    INSERT INTO hrm_approval_instances (org_id, template_id, entity_type, entity_id, instance_snapshot, current_level, overall_status, requested_by) VALUES
        (v_org_id, v_tmpl_warning, 'warning', gen_random_uuid(),
         '[{"level":1,"approver_type":"role","approver_role":"admin","sla_hours":24,"on_sla_breach":"escalate_next"}]'::jsonb,
         1, 'pending', v_sarah_id)
        RETURNING id INTO v_inst_nadia_warn;

    -- Pending: Zara's transfer, level 1 of 1 (dept_head of destination dept = Mike, Sales)
    INSERT INTO hrm_approval_instances (org_id, template_id, entity_type, entity_id, instance_snapshot, current_level, overall_status, requested_by) VALUES
        (v_org_id, v_tmpl_transfer, 'transfer', gen_random_uuid(),
         '[{"level":1,"approver_type":"dept_head","sla_hours":48,"on_sla_breach":"escalate_next"}]'::jsonb,
         1, 'pending', v_tom_user_id)
        RETURNING id INTO v_inst_zara_transfer;

    -- Pending: Arjun's award, level 1 of 1 (specific_user = Ayesha)
    INSERT INTO hrm_approval_instances (org_id, template_id, entity_type, entity_id, instance_snapshot, current_level, overall_status, requested_by) VALUES
        (v_org_id, v_tmpl_award, 'award', gen_random_uuid(),
         ('[{"level":1,"approver_type":"specific_user","approver_user_id":"' || v_ayesha_id || '","sla_hours":72,"on_sla_breach":"auto_approve"}]')::jsonb,
         1, 'pending', v_david_user_id)
        RETURNING id INTO v_inst_arjun_award;

    -- Historical, APPROVED: Fatima's promotion (completed 150 days ago)
    INSERT INTO hrm_approval_instances (org_id, template_id, entity_type, entity_id, instance_snapshot, current_level, overall_status, requested_by, created_at, updated_at, completed_at) VALUES
        (v_org_id, v_tmpl_promotion, 'promotion', gen_random_uuid(),
         '[{"level":1,"approver_type":"reporting_manager","sla_hours":48,"on_sla_breach":"escalate_next"}]'::jsonb,
         2, 'approved', v_david_user_id, NOW()-INTERVAL '152 days', NOW()-INTERVAL '150 days', NOW()-INTERVAL '150 days')
        RETURNING id INTO v_inst_fatima_promo;
    INSERT INTO hrm_approval_decisions (instance_id, level, approver_id, action, note, decided_at) VALUES
        (v_inst_fatima_promo, 1, v_david_user_id, 'approved', 'Fatima has been an outstanding senior contributor — approved.', NOW()-INTERVAL '150 days');

    -- Historical, REJECTED: Zara's promotion attempt (completed 40 days ago)
    INSERT INTO hrm_approval_instances (org_id, template_id, entity_type, entity_id, instance_snapshot, current_level, overall_status, requested_by, created_at, updated_at, completed_at) VALUES
        (v_org_id, v_tmpl_promotion, 'promotion', gen_random_uuid(),
         '[{"level":1,"approver_type":"reporting_manager","sla_hours":48,"on_sla_breach":"escalate_next"}]'::jsonb,
         1, 'rejected', v_tom_user_id, NOW()-INTERVAL '42 days', NOW()-INTERVAL '40 days', NOW()-INTERVAL '40 days')
        RETURNING id INTO v_inst_zara_promo;
    INSERT INTO hrm_approval_decisions (instance_id, level, approver_id, action, note, decided_at) VALUES
        (v_inst_zara_promo, 1, v_tom_user_id, 'rejected', 'Not yet — revisit in the next review cycle after the CSAT project wraps up.', NOW()-INTERVAL '40 days');

    -- ==========================================================
    -- 11. WARNING TYPES (4) + ESCALATION RULES (2)
    -- ==========================================================
    INSERT INTO hrm_warning_types (org_id, name, description, severity_level, can_be_issued_by, requires_hr_approval,
        employee_can_respond, response_window_days, valid_duration_days, created_by) VALUES
        (v_org_id, 'Verbal Warning', 'Informal counselling, logged for the record', 2, '{admin,manager}', FALSE, TRUE, 3, 180, v_ayesha_id)
        RETURNING id INTO v_wt_verbal;
    INSERT INTO hrm_warning_types (org_id, name, description, severity_level, can_be_issued_by, requires_hr_approval,
        employee_can_respond, response_window_days, valid_duration_days, created_by) VALUES
        (v_org_id, 'Written Warning — Attendance', 'Formal warning for repeated lateness or absence', 5, '{admin,manager}', FALSE, TRUE, 5, 365, v_ayesha_id)
        RETURNING id INTO v_wt_written_attendance;
    INSERT INTO hrm_warning_types (org_id, name, description, severity_level, can_be_issued_by, requires_hr_approval,
        approval_template_id, employee_can_respond, response_window_days, valid_duration_days, created_by) VALUES
        (v_org_id, 'Written Warning — Conduct', 'Formal warning for a conduct or policy violation', 7, '{admin}', TRUE,
         v_tmpl_warning, TRUE, 5, 365, v_ayesha_id)
        RETURNING id INTO v_wt_written_conduct;
    INSERT INTO hrm_warning_types (org_id, name, description, severity_level, can_be_issued_by, requires_hr_approval,
        approval_template_id, employee_can_respond, response_window_days, valid_duration_days, created_by) VALUES
        (v_org_id, 'Final Warning', 'Last warning before termination is considered', 9, '{admin}', TRUE,
         v_tmpl_warning, TRUE, 3, 0, v_ayesha_id)
        RETURNING id INTO v_wt_final;

    INSERT INTO hrm_warning_escalation_rules (org_id, trigger_warning_type_id, trigger_count, within_days, action, notification_roles, created_by) VALUES
        (v_org_id, v_wt_verbal, 3, 180, 'notify_hr', '{admin}', v_ayesha_id),
        (v_org_id, v_wt_written_conduct, 2, 365, 'flag_termination_review', '{admin,owner}', v_ayesha_id);

    -- ==========================================================
    -- 12. DOCUMENT TEMPLATES (5)
    -- ==========================================================
    INSERT INTO hrm_document_templates (org_id, name, document_type, description, body_markdown, available_variables, requires_acknowledgement, created_by) VALUES
        (v_org_id, 'Offer Letter — Standard', 'offer_letter', 'Standard full-time offer letter',
         E'# Offer of Employment\n\nDear {{employee.first_name}},\n\nWe are pleased to offer you the position of **{{employee.position}}** at Nexus Solutions, reporting to {{employee.manager_name}}, starting {{employee.hire_date}}.\n\nWelcome to the team!',
         ARRAY['employee.first_name','employee.position','employee.manager_name','employee.hire_date'], FALSE, v_ayesha_id)
        RETURNING id INTO v_doctmpl_offer;
    INSERT INTO hrm_document_templates (org_id, name, document_type, description, body_markdown, available_variables, requires_acknowledgement, created_by) VALUES
        (v_org_id, 'Promotion Letter', 'promotion_letter', 'Confirms a promotion and new compensation',
         E'# Promotion Confirmation\n\nDear {{employee.first_name}},\n\nCongratulations! Effective {{promotion.effective_date}}, you are promoted to **{{promotion.to_position}}**.\n\nYour new basic pay is {{promotion.new_basic_pay}}.',
         ARRAY['employee.first_name','promotion.effective_date','promotion.to_position','promotion.new_basic_pay'], TRUE, v_ayesha_id)
        RETURNING id INTO v_doctmpl_promo;
    INSERT INTO hrm_document_templates (org_id, name, document_type, description, body_markdown, available_variables, requires_acknowledgement, created_by) VALUES
        (v_org_id, 'Warning Letter — General', 'warning_letter', 'Formal written warning letter',
         E'# Formal Warning\n\nDear {{employee.first_name}},\n\nThis letter formally documents a **{{warning.type_name}}** issued on {{warning.incident_date}}.\n\n{{warning.description}}\n\nPlease respond within {{warning.response_window_days}} days if you wish to contest this warning.',
         ARRAY['employee.first_name','warning.type_name','warning.incident_date','warning.description','warning.response_window_days'], TRUE, v_ayesha_id)
        RETURNING id INTO v_doctmpl_warning;
    INSERT INTO hrm_document_templates (org_id, name, document_type, description, body_markdown, available_variables, requires_acknowledgement, created_by) VALUES
        (v_org_id, 'Termination Letter', 'termination_letter', 'Formal termination confirmation',
         E'# Termination of Employment\n\nDear {{employee.first_name}},\n\nThis letter confirms the termination of your employment effective {{termination.termination_date}}. Your last working day is {{termination.last_working_date}}.',
         ARRAY['employee.first_name','termination.termination_date','termination.last_working_date'], FALSE, v_ayesha_id)
        RETURNING id INTO v_doctmpl_termination;
    INSERT INTO hrm_document_templates (org_id, name, document_type, description, body_markdown, available_variables, requires_acknowledgement, created_by) VALUES
        (v_org_id, 'Code of Conduct Policy 2026', 'policy', 'Annual code of conduct acknowledgement',
         E'# Code of Conduct — 2026\n\nAll employees are expected to review and acknowledge the updated Code of Conduct, effective January 2026.',
         ARRAY[]::text[], TRUE, v_ayesha_id)
        RETURNING id INTO v_doctmpl_policy;

    -- ==========================================================
    -- 13. EMPLOYEE DOCUMENTS (5) + BULK SEND (1)
    -- ==========================================================
    INSERT INTO hrm_employee_documents (org_id, employee_id, template_id, title, document_type, file_url, file_name, mime_type,
        generated_content, related_type, related_id, status, issued_by, sent_at, acknowledged_at, acknowledgement_note, created_by) VALUES
        (v_org_id, v_emp_fatima, v_doctmpl_promo, 'Promotion Confirmation — Fatima Noor', 'promotion_letter',
         'https://files.nexussolutions.io/hrm/documents/promo-fatima-noor.pdf', 'promo-fatima-noor.pdf', 'application/pdf',
         '# Promotion Confirmation\n\nDear Fatima,\n\nCongratulations! Effective ' || (CURRENT_DATE-INTERVAL '150 days')::date || ', you are promoted to Senior Software Engineer.',
         'promotion', NULL, 'acknowledged', v_ayesha_id, NOW()-INTERVAL '149 days', NOW()-INTERVAL '148 days', 'Thank you!', v_ayesha_id)
        RETURNING id INTO v_doc_fatima_promo;

    INSERT INTO hrm_employee_documents (org_id, employee_id, template_id, title, document_type, file_url, file_name, mime_type,
        generated_content, related_type, related_id, status, issued_by, sent_at, created_by) VALUES
        (v_org_id, v_emp_daniel, v_doctmpl_termination, 'Termination Confirmation — Daniel Osei', 'termination_letter',
         'https://files.nexussolutions.io/hrm/documents/term-daniel-osei.pdf', 'term-daniel-osei.pdf', 'application/pdf',
         '# Termination of Employment\n\nDear Daniel,\n\nThis letter confirms the termination of your employment effective 2026-06-15.',
         'termination', NULL, 'sent', v_sarah_id, NOW()-INTERVAL '27 days', v_sarah_id)
        RETURNING id INTO v_doc_daniel_term;

    INSERT INTO hrm_employee_documents (org_id, employee_id, template_id, title, document_type, file_url, file_name, mime_type,
        generated_content, related_type, related_id, status, created_by) VALUES
        (v_org_id, v_emp_nadia, v_doctmpl_warning, 'Warning Letter (Draft) — Nadia Islam', 'warning_letter',
         'https://files.nexussolutions.io/hrm/documents/warn-nadia-islam-draft.pdf', 'warn-nadia-islam-draft.pdf', 'application/pdf',
         '# Formal Warning\n\nDraft — pending HR approval before this warning is issued.',
         'warning', NULL, 'draft', v_sarah_id)
        RETURNING id INTO v_doc_nadia_warn;

    -- Direct upload (no template) — employee ID proof
    INSERT INTO hrm_employee_documents (org_id, employee_id, title, document_type, file_url, file_name, file_size_bytes, mime_type, status, created_by) VALUES
        (v_org_id, v_emp_grace, 'Passport Copy — Grace Kim', 'id_proof',
         'https://files.nexussolutions.io/hrm/uploads/grace-kim-passport.pdf', 'grace-kim-passport.pdf', 482331, 'application/pdf', 'acknowledged', v_sarah_id);

    -- Bulk-sent policy acknowledgement (2 sample recipients out of a larger batch)
    INSERT INTO hrm_document_bulk_sends (org_id, template_id, sender_id, recipient_type, recipient_ids, batch_id, total_count, pending_count, completed_count, sent_at) VALUES
        (v_org_id, v_doctmpl_policy, v_sarah_id, 'all', '[]'::jsonb, v_bulk_batch_id, 20, 5, 15, NOW()-INTERVAL '20 days');

    INSERT INTO hrm_employee_documents (org_id, employee_id, template_id, title, document_type, file_url, file_name, mime_type,
        related_type, status, issued_by, sent_at, acknowledged_at, bulk_send_batch_id, created_by) VALUES
        (v_org_id, v_emp_diana, v_doctmpl_policy, 'Code of Conduct Policy 2026', 'policy',
         'https://files.nexussolutions.io/hrm/documents/policy-2026-diana-lee.pdf', 'policy-2026-diana-lee.pdf', 'application/pdf',
         NULL, 'acknowledged', v_sarah_id, NOW()-INTERVAL '20 days', NOW()-INTERVAL '18 days', v_bulk_batch_id, v_sarah_id)
        RETURNING id INTO v_doc_policy_diana;
    INSERT INTO hrm_employee_documents (org_id, employee_id, template_id, title, document_type, file_url, file_name, mime_type,
        related_type, status, issued_by, sent_at, bulk_send_batch_id, created_by) VALUES
        (v_org_id, v_emp_arjun, v_doctmpl_policy, 'Code of Conduct Policy 2026', 'policy',
         'https://files.nexussolutions.io/hrm/documents/policy-2026-arjun-mehta.pdf', 'policy-2026-arjun-mehta.pdf', 'application/pdf',
         NULL, 'sent', v_sarah_id, NOW()-INTERVAL '20 days', v_bulk_batch_id, v_sarah_id)
        RETURNING id INTO v_doc_policy_arjun;

    -- ==========================================================
    -- 14. SHIFTS (2) + WORK SCHEDULE ASSIGNMENTS (3)
    -- ==========================================================
    INSERT INTO hrm_shifts (org_id, name, description, shift_type, start_time, end_time, break_minutes, working_days,
        track_overtime, overtime_threshold_hours, is_default, created_by) VALUES
        (v_org_id, 'General Shift', 'Standard 9-to-6 office hours', 'fixed', '09:00', '18:00', 60,
         '{mon,tue,wed,thu,fri}', TRUE, 9, TRUE, v_ayesha_id)
        RETURNING id INTO v_shift_general;
    INSERT INTO hrm_shifts (org_id, name, description, shift_type, core_start_time, core_end_time, weekly_hours_target, break_minutes, working_days, created_by) VALUES
        (v_org_id, 'Flexible Hours', 'Core collaboration window with flexible start/end', 'flexible', '11:00', '16:00', 40, 60,
         '{mon,tue,wed,thu,fri}', v_ayesha_id)
        RETURNING id INTO v_shift_flex;

    INSERT INTO hrm_work_schedule_assignments (org_id, shift_id, assignee_type, assignee_id, effective_date, created_by) VALUES
        (v_org_id, v_shift_general, 'organization', v_org_id, '2024-01-01', v_ayesha_id),
        (v_org_id, v_shift_flex,    'department',   v_dept_eng, '2024-04-01', v_ayesha_id),
        (v_org_id, v_shift_general, 'employee',     v_emp_omar, '2026-06-01', v_mike_id);

    -- ==========================================================
    -- 15. HOLIDAY CALENDAR (1) + HOLIDAYS (7) + ASSIGNMENT (1)
    -- ==========================================================
    INSERT INTO hrm_holiday_calendars (org_id, name, description, country_code, year, created_by) VALUES
        (v_org_id, 'US Holidays 2026', 'Company-observed public holidays', 'US', 2026, v_ayesha_id)
        RETURNING id INTO v_holcal_2026;

    INSERT INTO hrm_holidays (calendar_id, name, date, holiday_type, is_paid, repeat_yearly) VALUES
        (v_holcal_2026, 'New Year''s Day',    '2026-01-01', 'public',  TRUE, TRUE),
        (v_holcal_2026, 'Memorial Day',       '2026-05-25', 'public',  TRUE, TRUE),
        (v_holcal_2026, 'Juneteenth',         '2026-06-19', 'public',  TRUE, TRUE),
        (v_holcal_2026, 'Independence Day',   '2026-07-04', 'public',  TRUE, TRUE),
        (v_holcal_2026, 'Labor Day',          '2026-09-07', 'public',  TRUE, TRUE),
        (v_holcal_2026, 'Thanksgiving Day',   '2026-11-26', 'public',  TRUE, TRUE),
        (v_holcal_2026, 'Christmas Day',      '2026-12-25', 'public',  TRUE, TRUE);

    INSERT INTO hrm_calendar_assignments (org_id, calendar_id, assignee_type, assignee_id, effective_date, created_by) VALUES
        (v_org_id, v_holcal_2026, 'organization', v_org_id, '2026-01-01', v_ayesha_id);

    -- ==========================================================
    -- 16. EMPLOYEE CONTRACTS (20 — one active contract per employee)
    -- ==========================================================
    INSERT INTO hrm_employee_contracts (org_id, employee_id, contract_type, start_date, probation_end_date, notice_period_days, salary_structure_id, work_hours_per_week, is_active, created_by) VALUES
        (v_org_id, v_emp_ayesha,  'permanent', '2024-01-15', NULL, 60, v_struct_senior,   40, TRUE, v_ayesha_id),
        (v_org_id, v_emp_sarah,   'permanent', '2024-02-01', NULL, 30, v_struct_senior,   40, TRUE, v_ayesha_id),
        (v_org_id, v_emp_mike,    'permanent', '2024-03-01', NULL, 30, v_struct_senior,   40, TRUE, v_ayesha_id),
        (v_org_id, v_emp_david,   'permanent', '2024-04-01', NULL, 30, v_struct_senior,   40, TRUE, v_ayesha_id),
        (v_org_id, v_emp_rebecca, 'permanent', '2024-08-15', NULL, 30, v_struct_senior,   40, TRUE, v_ayesha_id),
        (v_org_id, v_emp_tom,     'permanent', '2024-09-01', NULL, 30, v_struct_senior,   40, TRUE, v_ayesha_id),
        (v_org_id, v_emp_hassan,  'permanent', '2024-10-01', NULL, 30, v_struct_senior,   40, TRUE, v_ayesha_id),
        (v_org_id, v_emp_priya,   'permanent', '2024-05-01', NULL, 30, v_struct_sales,    40, TRUE, v_ayesha_id),
        (v_org_id, v_emp_james,   'permanent', '2024-11-01', NULL, 30, v_struct_sales,    40, TRUE, v_ayesha_id),
        (v_org_id, v_emp_sofia,   'permanent', '2024-06-01', NULL, 30, v_struct_sales,    40, TRUE, v_ayesha_id),
        (v_org_id, v_emp_omar,    'probation', '2026-06-01', '2026-09-01', 7, v_struct_sales, 40, TRUE, v_ayesha_id),
        (v_org_id, v_emp_diana,   'permanent', '2025-01-15', NULL, 30, v_struct_standard, 40, TRUE, v_ayesha_id),
        (v_org_id, v_emp_nadia,   'permanent', '2025-04-01', NULL, 30, v_struct_standard, 40, TRUE, v_ayesha_id),
        (v_org_id, v_emp_zara,    'fixed_term','2025-03-01', NULL, 14, v_struct_standard, 40, TRUE, v_ayesha_id),
        (v_org_id, v_emp_fatima,  'permanent', '2023-03-10', NULL, 30, v_struct_senior,   40, TRUE, v_ayesha_id),
        (v_org_id, v_emp_arjun,   'permanent', '2024-09-01', NULL, 30, v_struct_standard, 40, TRUE, v_ayesha_id),
        (v_org_id, v_emp_kevin,   'permanent', '2024-07-01', NULL, 30, v_struct_standard, 40, TRUE, v_ayesha_id),
        (v_org_id, v_emp_lisa,    'permanent', '2023-08-01', NULL, 30, v_struct_standard, 40, TRUE, v_ayesha_id),
        (v_org_id, v_emp_daniel,  'probation', '2025-05-01', '2025-08-01', 7, v_struct_standard, 40, FALSE, v_ayesha_id),
        (v_org_id, v_emp_grace,   'permanent', '2025-02-01', NULL, 30, v_struct_standard, 40, TRUE, v_ayesha_id);

    -- ==========================================================
    -- 17. PROMOTIONS (4)
    -- ==========================================================
    -- Fatima — applied, historical (linked to the approved instance + document above)
    INSERT INTO hrm_promotions (org_id, employee_id, from_position_id, from_department_id, from_salary_structure_id, from_basic_pay,
        to_position_id, to_department_id, to_salary_structure_id, new_basic_pay, effective_date, reason,
        approval_instance_id, document_id, status, applied_at, applied_by, created_by, created_at) VALUES
        (v_org_id, v_emp_fatima, v_pos_swe, v_dept_eng, v_struct_standard, 7500,
         v_pos_sr_swe, v_dept_eng, v_struct_senior, 9500, (CURRENT_DATE-INTERVAL '150 days')::date, 'Consistently strong technical leadership and mentoring',
         v_inst_fatima_promo, v_doc_fatima_promo, 'applied', NOW()-INTERVAL '150 days', v_david_user_id, v_david_user_id, NOW()-INTERVAL '152 days')
        RETURNING id INTO v_promo_fatima;

    -- Kevin — pending_approval (the live test case for our approval-chain wiring)
    INSERT INTO hrm_promotions (org_id, employee_id, from_position_id, from_department_id, from_basic_pay,
        to_position_id, to_department_id, to_salary_structure_id, new_basic_pay, effective_date, reason,
        approval_instance_id, status, created_by, created_at) VALUES
        (v_org_id, v_emp_kevin, v_pos_swe, v_dept_eng, 6800,
         v_pos_sr_swe, v_dept_eng, v_struct_senior, 8800, (CURRENT_DATE+INTERVAL '14 days')::date, 'Led the Q2 platform migration end-to-end',
         v_inst_kevin_promo, 'pending_approval', v_david_user_id, NOW()-INTERVAL '2 days')
        RETURNING id INTO v_promo_kevin;

    -- Zara — rejected, historical
    INSERT INTO hrm_promotions (org_id, employee_id, from_position_id, from_department_id, from_basic_pay,
        to_position_id, to_department_id, new_basic_pay, effective_date, reason,
        approval_instance_id, status, created_by, created_at) VALUES
        (v_org_id, v_emp_zara, v_pos_support_spec, v_dept_support, 3800,
         v_pos_support_lead, v_dept_support, 4400, (CURRENT_DATE-INTERVAL '40 days')::date, 'Requested consideration for team lead opening',
         v_inst_zara_promo, 'rejected', v_tom_user_id, NOW()-INTERVAL '42 days')
        RETURNING id INTO v_promo_zara;

    -- Arjun — draft, not yet submitted
    INSERT INTO hrm_promotions (org_id, employee_id, from_position_id, from_department_id, from_basic_pay,
        to_position_id, to_department_id, new_basic_pay, effective_date, reason, status, created_by, created_at) VALUES
        (v_org_id, v_emp_arjun, v_pos_swe, v_dept_eng, 6500,
         v_pos_sr_swe, v_dept_eng, 8200, (CURRENT_DATE+INTERVAL '30 days')::date, 'Drafted ahead of Arjun''s upcoming 2-year review',
         'draft', v_david_user_id, NOW()-INTERVAL '1 day')
        RETURNING id INTO v_promo_arjun;

    -- ==========================================================
    -- 18. TRANSFERS (3)
    -- ==========================================================
    -- Omar — approved (location change), not yet applied
    INSERT INTO hrm_transfers (org_id, employee_id, transfer_type, from_location, to_location, effective_date, reason, status, created_by, created_at) VALUES
        (v_org_id, v_emp_omar, 'location', 'New York, NY (office)', 'Remote', (CURRENT_DATE+INTERVAL '7 days')::date,
         'Relocating; approved for full-remote work', 'approved', v_mike_id, NOW()-INTERVAL '5 days')
        RETURNING id INTO v_trf_omar;

    -- Grace — draft
    INSERT INTO hrm_transfers (org_id, employee_id, transfer_type, from_department_id, to_department_id, effective_date, reason, status, created_by, created_at) VALUES
        (v_org_id, v_emp_grace, 'department', v_dept_qa, v_dept_eng, (CURRENT_DATE+INTERVAL '21 days')::date,
         'Cross-training into a full engineering role', 'draft', v_hassan_user_id, NOW()-INTERVAL '1 day')
        RETURNING id INTO v_trf_grace;

    -- Zara — pending_approval (dept_head approver test case)
    INSERT INTO hrm_transfers (org_id, employee_id, transfer_type, from_department_id, to_department_id, effective_date, reason,
        approval_instance_id, status, created_by, created_at) VALUES
        (v_org_id, v_emp_zara, 'department', v_dept_support, v_dept_sales, (CURRENT_DATE+INTERVAL '30 days')::date,
         'Zara has expressed strong interest in moving into sales', v_inst_zara_transfer, 'pending_approval', v_tom_user_id, NOW()-INTERVAL '3 days')
        RETURNING id INTO v_trf_zara;

    -- ==========================================================
    -- 19. RESIGNATIONS (2)
    -- ==========================================================
    -- Lisa — accepted, notice period in progress
    INSERT INTO hrm_resignations (org_id, employee_id, resignation_date, notice_period_days, is_notice_waived, last_working_date,
        reason_category, reason_remarks, exit_interview_completed, exit_clearance_completed, status, accepted_at, accepted_by, created_by, created_at) VALUES
        (v_org_id, v_emp_lisa, (CURRENT_DATE-INTERVAL '20 days')::date, 30, FALSE, (CURRENT_DATE+INTERVAL '10 days')::date,
         'better_opportunity', 'Accepted an offer at another company; grateful for the time here', FALSE, FALSE,
         'accepted', NOW()-INTERVAL '18 days', v_sarah_id, v_david_user_id, NOW()-INTERVAL '20 days')
        RETURNING id INTO v_res_lisa;

    -- Priya — withdrawn (considered leaving, changed her mind)
    INSERT INTO hrm_resignations (org_id, employee_id, resignation_date, notice_period_days, is_notice_waived, last_working_date,
        reason_category, reason_remarks, status, created_by, created_at, updated_at) VALUES
        (v_org_id, v_emp_priya, (CURRENT_DATE-INTERVAL '60 days')::date, 30, FALSE, (CURRENT_DATE-INTERVAL '30 days')::date,
         'career_growth', 'Reconsidered after a compensation review and a new account portfolio', 'withdrawn',
         v_priya_id, NOW()-INTERVAL '60 days', NOW()-INTERVAL '55 days')
        RETURNING id INTO v_res_priya;

    -- ==========================================================
    -- 20. TERMINATIONS (3)
    -- ==========================================================
    -- Sofia — pending_approval (the live test case)
    INSERT INTO hrm_terminations (org_id, employee_id, termination_type, termination_date, last_working_date, reason, internal_notes,
        approval_instance_id, status, created_by, created_at) VALUES
        (v_org_id, v_emp_sofia, 'involuntary', (CURRENT_DATE+INTERVAL '14 days')::date, (CURRENT_DATE+INTERVAL '14 days')::date,
         'Sustained underperformance against quota over two consecutive quarters',
         'PIP was issued on ' || (CURRENT_DATE-INTERVAL '60 days')::date || '; no material improvement observed.',
         v_inst_sofia_term, 'pending_approval', v_mike_id, NOW()-INTERVAL '4 days')
        RETURNING id INTO v_term_sofia;

    -- Daniel — applied, historical
    INSERT INTO hrm_terminations (org_id, employee_id, termination_type, termination_date, last_working_date, reason,
        severance_amount, severance_currency, is_rehire_eligible, exit_clearance_completed, status, applied_at, applied_by, created_by, created_at) VALUES
        (v_org_id, v_emp_daniel, 'probation_fail', '2026-06-15', '2026-06-15', 'Did not meet probation performance criteria',
         0, 'USD', TRUE, TRUE, 'applied', NOW()-INTERVAL '27 days', v_sarah_id, v_sarah_id, NOW()-INTERVAL '30 days')
        RETURNING id INTO v_term_daniel;

    -- Arjun — cancelled (situation was resolved before it went further)
    INSERT INTO hrm_terminations (org_id, employee_id, termination_type, termination_date, last_working_date, reason, status, created_by, created_at) VALUES
        (v_org_id, v_emp_arjun, 'involuntary', (CURRENT_DATE-INTERVAL '35 days')::date, (CURRENT_DATE-INTERVAL '35 days')::date,
         'Initial performance concern — draft only', 'cancelled', v_david_user_id, NOW()-INTERVAL '38 days')
        RETURNING id INTO v_term_arjun;

    -- ==========================================================
    -- 21. EMPLOYEE WARNINGS (5)
    -- ==========================================================
    INSERT INTO hrm_employee_warnings (org_id, employee_id, warning_type_id, warning_type_name, severity_level, title, description,
        incident_date, issued_by, can_employee_respond, response_window_days, response_deadline, expires_at, is_active, issued_at, status, created_by) VALUES
        (v_org_id, v_emp_arjun, v_wt_verbal, 'Verbal Warning', 2, 'Repeated lateness', 'Arrived more than 30 minutes late on three occasions this month.',
         (CURRENT_DATE-INTERVAL '10 days')::date, v_david_user_id, TRUE, 3, (CURRENT_DATE-INTERVAL '7 days')::date, (CURRENT_DATE+INTERVAL '170 days')::date,
         TRUE, NOW()-INTERVAL '10 days', 'issued', v_david_user_id);

    INSERT INTO hrm_employee_warnings (org_id, employee_id, warning_type_id, warning_type_name, severity_level, title, description,
        incident_date, issued_by, can_employee_respond, response_window_days, response_deadline, employee_response, employee_responded_at,
        expires_at, is_active, issued_at, status, created_by) VALUES
        (v_org_id, v_emp_omar, v_wt_verbal, 'Verbal Warning', 2, 'Missed onboarding checklist deadline', 'Did not complete the required security training by the assigned deadline.',
         (CURRENT_DATE-INTERVAL '5 days')::date, v_mike_id, TRUE, 3, (CURRENT_DATE-INTERVAL '2 days')::date,
         'Apologies — completed the training the same day I was notified.', NOW()-INTERVAL '4 days',
         (CURRENT_DATE+INTERVAL '175 days')::date, TRUE, NOW()-INTERVAL '5 days', 'issued', v_mike_id);

    INSERT INTO hrm_employee_warnings (org_id, employee_id, warning_type_id, warning_type_name, severity_level, title, description,
        incident_date, issued_by, can_employee_respond, response_window_days, response_deadline, employee_response, employee_responded_at,
        expires_at, is_active, issued_at, status, created_by) VALUES
        (v_org_id, v_emp_grace, v_wt_written_attendance, 'Written Warning — Attendance', 5, 'Attendance pattern concern',
         'Three unapproved absences in the past 60 days without prior notice.',
         (CURRENT_DATE-INTERVAL '30 days')::date, v_hassan_user_id, TRUE, 5, (CURRENT_DATE-INTERVAL '25 days')::date,
         'Understood — I will notify the team proactively going forward.', NOW()-INTERVAL '27 days',
         (CURRENT_DATE+INTERVAL '335 days')::date, TRUE, NOW()-INTERVAL '30 days', 'acknowledged', v_hassan_user_id);

    -- Nadia — pending_approval (the live test case for the warnings approval gate)
    INSERT INTO hrm_employee_warnings (org_id, employee_id, warning_type_id, warning_type_name, severity_level, title, description,
        incident_date, issued_by, approval_instance_id, can_employee_respond, response_window_days,
        is_active, status, created_by) VALUES
        (v_org_id, v_emp_nadia, v_wt_written_conduct, 'Written Warning — Conduct', 7, 'Confidentiality policy violation',
         'Shared candidate compensation details outside the hiring committee, in violation of the confidentiality policy.',
         (CURRENT_DATE-INTERVAL '2 days')::date, v_sarah_id, v_inst_nadia_warn, TRUE, 5,
         TRUE, 'pending_approval', v_sarah_id)
        RETURNING id INTO v_warn_nadia;

    -- Daniel — closed, historical (preceded his termination)
    INSERT INTO hrm_employee_warnings (org_id, employee_id, warning_type_id, warning_type_name, severity_level, title, description,
        incident_date, issued_by, can_employee_respond, response_window_days, response_deadline,
        expires_at, is_active, issued_at, status, created_by) VALUES
        (v_org_id, v_emp_daniel, v_wt_final, 'Final Warning', 9, 'Missed critical release deadline',
         'Failed to deliver committed work for the June release after two prior check-ins; directly contributed to a customer-facing delay.',
         '2026-06-05', v_david_user_id, TRUE, 3, '2026-06-08', NULL, FALSE, NOW()-INTERVAL '37 days', 'closed', v_david_user_id);

    -- ==========================================================
    -- 22. COMPLAINTS (3)
    -- ==========================================================
    -- created_by references users(id); Grace and Zara are standalone employees
    -- (no login), so these are recorded as entered by their manager / HR intake.
    INSERT INTO hrm_complaints (org_id, employee_id, is_anonymous, complaint_type, title, description, incident_date,
        against_employee_id, investigator_id, investigation_notes, investigation_started_at, status, created_by) VALUES
        (v_org_id, v_emp_grace, FALSE, 'workplace_safety', 'Loose cabling near QA lab entrance',
         'Exposed cabling near the QA lab door is a trip hazard, flagged twice already in team standups.',
         (CURRENT_DATE-INTERVAL '15 days')::date, NULL, v_sarah_id, 'Facilities ticket opened; awaiting building management response.',
         NOW()-INTERVAL '14 days', 'under_review', v_hassan_user_id);

    INSERT INTO hrm_complaints (org_id, employee_id, is_anonymous, complaint_type, title, description, incident_date,
        against_employee_id, investigator_id, investigation_notes, investigation_started_at, status, created_by) VALUES
        (v_org_id, v_emp_grace, TRUE, 'manager_conduct', 'Concerns about sprint planning conduct',
         'Submitted anonymously — dismissive tone toward junior team members during sprint planning sessions.',
         (CURRENT_DATE-INTERVAL '8 days')::date, v_emp_hassan, v_sarah_id, 'Gathering feedback from the wider QA team confidentially.',
         NOW()-INTERVAL '6 days', 'investigating', v_sarah_id);

    INSERT INTO hrm_complaints (org_id, employee_id, is_anonymous, complaint_type, title, description, incident_date,
        against_details, resolution, resolution_action, resolved_at, resolved_by, status, created_by) VALUES
        (v_org_id, v_emp_zara, FALSE, 'wage_dispute', 'Overtime hours not reflected in last payslip',
         'Approved overtime from the product launch weekend does not appear on the latest payslip.',
         (CURRENT_DATE-INTERVAL '35 days')::date, 'Payroll processing — not against a specific individual',
         'Confirmed a payroll processing gap; overtime hours corrected and back-paid in the following cycle.',
         'mediation', NOW()-INTERVAL '25 days', v_sarah_id, 'resolved', v_tom_user_id);

    -- ==========================================================
    -- 23. ACKNOWLEDGEMENTS (4)
    -- ==========================================================
    INSERT INTO hrm_acknowledgements (org_id, employee_id, acknowledgeable_type, acknowledgeable_id, entity_title, notes,
        signature_required, signed_at, status, acknowledged_at, requested_by, requested_at) VALUES
        (v_org_id, v_emp_grace, 'warning', (SELECT id FROM hrm_employee_warnings WHERE employee_id = v_emp_grace AND warning_type_id = v_wt_written_attendance),
         'Written Warning — Attendance', 'Employee acknowledged receipt and response window',
         FALSE, NULL, 'acknowledged', NOW()-INTERVAL '27 days', v_hassan_user_id, NOW()-INTERVAL '30 days');

    INSERT INTO hrm_acknowledgements (org_id, employee_id, acknowledgeable_type, acknowledgeable_id, entity_title, notes,
        signature_required, signed_at, status, acknowledged_at, requested_by, requested_at) VALUES
        (v_org_id, v_emp_diana, 'document', v_doc_policy_diana, 'Code of Conduct Policy 2026', 'Bulk policy send',
         TRUE, NOW()-INTERVAL '18 days', 'acknowledged', NOW()-INTERVAL '18 days', v_sarah_id, NOW()-INTERVAL '20 days');

    INSERT INTO hrm_acknowledgements (org_id, employee_id, acknowledgeable_type, acknowledgeable_id, entity_title, notes,
        signature_required, status, requested_by, requested_at, expires_at) VALUES
        (v_org_id, v_emp_arjun, 'document', v_doc_policy_arjun, 'Code of Conduct Policy 2026', 'Bulk policy send — awaiting employee action',
         TRUE, 'pending', v_sarah_id, NOW()-INTERVAL '20 days', (CURRENT_DATE+INTERVAL '10 days')::date);

    INSERT INTO hrm_acknowledgements (org_id, employee_id, acknowledgeable_type, acknowledgeable_id, entity_title, notes,
        signature_required, status, requested_by, requested_at) VALUES
        (v_org_id, v_emp_fatima, 'document', v_doc_fatima_promo, 'Promotion Confirmation — Fatima Noor', 'Promotion letter acknowledgement',
         TRUE, 'acknowledged', v_ayesha_id, NOW()-INTERVAL '149 days');

    -- ==========================================================
    -- 24. ATTENDANCE — last 2 working weeks for active employees (loop-generated)
    -- ==========================================================
    FOR v_att_emp IN
        SELECT id FROM hrm_employees WHERE org_id = v_org_id AND status = 'active'
    LOOP
        v_att_date := CURRENT_DATE - INTERVAL '13 days';
        WHILE v_att_date <= CURRENT_DATE - INTERVAL '1 day' LOOP
            v_dow := EXTRACT(ISODOW FROM v_att_date); -- 1=Mon .. 7=Sun
            IF v_dow < 6 THEN -- weekdays only
                IF v_att_emp = v_emp_arjun AND v_att_date = CURRENT_DATE - INTERVAL '9 days' THEN
                    -- One late day for Arjun, matching his verbal warning narrative
                    INSERT INTO hrm_attendance_records (org_id, employee_id, attendance_date, shift_id, shift_name, expected_in, expected_out,
                        check_in_time, check_out_time, break_minutes, regular_hours, day_type, source, status, created_by)
                    VALUES (v_org_id, v_att_emp, v_att_date, v_shift_general, 'General Shift', '09:00', '18:00',
                        '09:41', '18:05', 60, 7.4, 'late', 'device', 'approved', v_sarah_id);
                ELSIF v_att_emp = v_emp_kevin AND v_att_date BETWEEN CURRENT_DATE - INTERVAL '9 days' AND CURRENT_DATE - INTERVAL '8 days' THEN
                    -- Two sick days for Kevin (unrelated historical leave request window is earlier; this is a separate short absence)
                    INSERT INTO hrm_attendance_records (org_id, employee_id, attendance_date, shift_id, shift_name, expected_in, expected_out,
                        break_minutes, regular_hours, day_type, source, status, created_by)
                    VALUES (v_org_id, v_att_emp, v_att_date, v_shift_general, 'General Shift', '09:00', '18:00',
                        0, 0, 'absent', 'manual', 'approved', v_sarah_id);
                ELSIF v_att_emp = v_emp_zara AND v_att_date = CURRENT_DATE - INTERVAL '6 days' THEN
                    -- One work-from-home day
                    INSERT INTO hrm_attendance_records (org_id, employee_id, attendance_date, shift_id, shift_name, expected_in, expected_out,
                        check_in_time, check_out_time, break_minutes, regular_hours, day_type, source, status, created_by)
                    VALUES (v_org_id, v_att_emp, v_att_date, v_shift_general, 'General Shift', '09:00', '18:00',
                        '09:02', '17:58', 60, 8.0, 'work_from_home', 'manual', 'approved', v_sarah_id);
                ELSE
                    -- Normal present day
                    INSERT INTO hrm_attendance_records (org_id, employee_id, attendance_date, shift_id, shift_name, expected_in, expected_out,
                        check_in_time, check_out_time, break_minutes, regular_hours, day_type, source, status, created_by)
                    VALUES (v_org_id, v_att_emp, v_att_date, v_shift_general, 'General Shift', '09:00', '18:00',
                        '08:57', '18:02', 60, 8.0, 'present', 'device', 'approved', v_sarah_id);
                END IF;
            END IF;
            v_att_date := v_att_date + INTERVAL '1 day';
        END LOOP;
    END LOOP;

    -- One pending regularization request (Arjun forgot to check out)
    INSERT INTO hrm_attendance_records (org_id, employee_id, attendance_date, shift_id, shift_name, expected_in, expected_out,
        check_in_time, break_minutes, regular_hours, day_type, source, regularization_reason, status, created_by)
    VALUES (v_org_id, v_emp_arjun, CURRENT_DATE - INTERVAL '2 days', v_shift_general, 'General Shift', '09:00', '18:00',
        '08:55', 60, 0, 'present', 'device', 'Forgot to badge out — left at approximately 18:15 for an in-person client call.',
        'pending', v_david_user_id)
    ON CONFLICT (employee_id, attendance_date) DO NOTHING;

    -- ==========================================================
    -- 25. ATTENDANCE PERIODS (2)
    -- ==========================================================
    INSERT INTO hrm_attendance_periods (org_id, period_year, period_month, status, total_employees, total_work_days,
        total_present, total_absent, total_holidays, total_leaves, total_overtime_hours, finalized_at, finalized_by, created_by) VALUES
        (v_org_id, EXTRACT(YEAR FROM CURRENT_DATE - INTERVAL '1 month')::int, EXTRACT(MONTH FROM CURRENT_DATE - INTERVAL '1 month')::int,
         'finalized', 18, 21, 356, 12, 1, 8, 24.5, NOW()-INTERVAL '10 days', v_sarah_id, v_sarah_id)
        RETURNING id INTO v_attp_june;

    INSERT INTO hrm_attendance_periods (org_id, period_year, period_month, status, total_employees, created_by) VALUES
        (v_org_id, EXTRACT(YEAR FROM CURRENT_DATE)::int, EXTRACT(MONTH FROM CURRENT_DATE)::int, 'open', 18, v_sarah_id)
        RETURNING id INTO v_attp_july;

    -- ==========================================================
    -- 26. PAYROLL — 1 run + 5 hand-computed payslips (verifies the
    --     slab tax engine end-to-end; see migration header for the
    --     worked arithmetic on each of these five employees)
    -- ==========================================================
    INSERT INTO hrm_payslip_runs (org_id, period_year, period_month, description, currency, attendance_period_id,
        total_employees, total_gross_pay, total_deductions, total_net_pay, status, computed_at, computed_by, created_by) VALUES
        (v_org_id, EXTRACT(YEAR FROM CURRENT_DATE - INTERVAL '1 month')::int, EXTRACT(MONTH FROM CURRENT_DATE - INTERVAL '1 month')::int,
         'June payroll run', 'USD', v_attp_june,
         5, 24950.00, 5501.00, 19449.00, 'computed', NOW()-INTERVAL '8 days', v_sarah_id, v_sarah_id)
        RETURNING id INTO v_payrun_june;

    -- Ayesha (CEO) — gross 8100, deductions 2082, net 6018
    INSERT INTO hrm_payslips (org_id, employee_id, payslip_run_id, period_year, period_month, salary_structure_id, salary_structure_name,
        gross_pay, total_deductions, net_pay, basic_pay, work_days, present_days, currency, status)
    VALUES (v_org_id, v_emp_ayesha, v_payrun_june, EXTRACT(YEAR FROM CURRENT_DATE - INTERVAL '1 month')::int, EXTRACT(MONTH FROM CURRENT_DATE - INTERVAL '1 month')::int,
        v_struct_senior, 'Senior & Management', 8100.00, 2082.00, 6018.00, 15000.00, 21, 21, 'USD', 'computed')
    RETURNING id INTO v_payslip_id;
    INSERT INTO hrm_payslip_lines (payslip_id, org_id, component_id, component_name, component_type, calc_method, computed_amount, display_order) VALUES
        (v_payslip_id, v_org_id, v_comp_hra,       'House Rent Allowance',     'earning',              'pct_of_basic', 6000.00, 1),
        (v_payslip_id, v_org_id, v_comp_medical,   'Medical Allowance',        'earning',              'fixed',         400.00, 2),
        (v_payslip_id, v_org_id, v_comp_transport, 'Transport Allowance',      'earning',              'fixed',         200.00, 3),
        (v_payslip_id, v_org_id, v_comp_perfbonus, 'Performance Bonus',        'earning',              'formula',      1500.00, 4),
        (v_payslip_id, v_org_id, v_comp_healthins, 'Health Insurance Premium', 'deduction',            'pct_of_gross',  162.00, 10),
        (v_payslip_id, v_org_id, v_comp_pf,        'Provident Fund',           'deduction',            'pct_of_basic', 1200.00, 11),
        (v_payslip_id, v_org_id, v_comp_incometax, 'Income Tax',               'deduction',            'slab',          720.00, 12),
        (v_payslip_id, v_org_id, v_comp_employerpf,'Employer PF Contribution', 'employer_contribution','pct_of_basic', 1200.00, 20);

    -- David (VP Eng) — gross 6600, deductions 1512, net 5088
    INSERT INTO hrm_payslips (org_id, employee_id, payslip_run_id, period_year, period_month, salary_structure_id, salary_structure_name,
        gross_pay, total_deductions, net_pay, basic_pay, work_days, present_days, currency, status)
    VALUES (v_org_id, v_emp_david, v_payrun_june, EXTRACT(YEAR FROM CURRENT_DATE - INTERVAL '1 month')::int, EXTRACT(MONTH FROM CURRENT_DATE - INTERVAL '1 month')::int,
        v_struct_senior, 'Senior & Management', 6600.00, 1512.00, 5088.00, 12000.00, 21, 20, 'USD', 'computed')
    RETURNING id INTO v_payslip_id;
    INSERT INTO hrm_payslip_lines (payslip_id, org_id, component_id, component_name, component_type, calc_method, computed_amount, display_order) VALUES
        (v_payslip_id, v_org_id, v_comp_hra,       'House Rent Allowance',     'earning',              'pct_of_basic', 4800.00, 1),
        (v_payslip_id, v_org_id, v_comp_medical,   'Medical Allowance',        'earning',              'fixed',         400.00, 2),
        (v_payslip_id, v_org_id, v_comp_transport, 'Transport Allowance',      'earning',              'fixed',         200.00, 3),
        (v_payslip_id, v_org_id, v_comp_perfbonus, 'Performance Bonus',        'earning',              'formula',      1200.00, 4),
        (v_payslip_id, v_org_id, v_comp_healthins, 'Health Insurance Premium', 'deduction',            'pct_of_gross',  132.00, 10),
        (v_payslip_id, v_org_id, v_comp_pf,        'Provident Fund',           'deduction',            'pct_of_basic',  960.00, 11),
        (v_payslip_id, v_org_id, v_comp_incometax, 'Income Tax',               'deduction',            'slab',          420.00, 12),
        (v_payslip_id, v_org_id, v_comp_employerpf,'Employer PF Contribution', 'employer_contribution','pct_of_basic',  960.00, 20);

    -- Fatima (Senior SWE, post-promotion) — gross 5350, deductions 1102, net 4248
    INSERT INTO hrm_payslips (org_id, employee_id, payslip_run_id, period_year, period_month, salary_structure_id, salary_structure_name,
        gross_pay, total_deductions, net_pay, basic_pay, work_days, present_days, currency, status)
    VALUES (v_org_id, v_emp_fatima, v_payrun_june, EXTRACT(YEAR FROM CURRENT_DATE - INTERVAL '1 month')::int, EXTRACT(MONTH FROM CURRENT_DATE - INTERVAL '1 month')::int,
        v_struct_senior, 'Senior & Management', 5350.00, 1102.00, 4248.00, 9500.00, 21, 21, 'USD', 'computed')
    RETURNING id INTO v_payslip_id;
    INSERT INTO hrm_payslip_lines (payslip_id, org_id, component_id, component_name, component_type, calc_method, computed_amount, display_order) VALUES
        (v_payslip_id, v_org_id, v_comp_hra,       'House Rent Allowance',     'earning',              'pct_of_basic', 3800.00, 1),
        (v_payslip_id, v_org_id, v_comp_medical,   'Medical Allowance',        'earning',              'fixed',         400.00, 2),
        (v_payslip_id, v_org_id, v_comp_transport, 'Transport Allowance',      'earning',              'fixed',         200.00, 3),
        (v_payslip_id, v_org_id, v_comp_perfbonus, 'Performance Bonus',        'earning',              'formula',       950.00, 4),
        (v_payslip_id, v_org_id, v_comp_healthins, 'Health Insurance Premium', 'deduction',            'pct_of_gross',  107.00, 10),
        (v_payslip_id, v_org_id, v_comp_pf,        'Provident Fund',           'deduction',            'pct_of_basic',  760.00, 11),
        (v_payslip_id, v_org_id, v_comp_incometax, 'Income Tax',               'deduction',            'slab',          235.00, 12),
        (v_payslip_id, v_org_id, v_comp_employerpf,'Employer PF Contribution', 'employer_contribution','pct_of_basic',  760.00, 20);

    -- Arjun (SWE, Standard structure — no Perf Bonus/Health Ins/Employer PF) — gross 3050, deductions 525, net 2525
    INSERT INTO hrm_payslips (org_id, employee_id, payslip_run_id, period_year, period_month, salary_structure_id, salary_structure_name,
        gross_pay, total_deductions, net_pay, basic_pay, work_days, present_days, absent_days, currency, status)
    VALUES (v_org_id, v_emp_arjun, v_payrun_june, EXTRACT(YEAR FROM CURRENT_DATE - INTERVAL '1 month')::int, EXTRACT(MONTH FROM CURRENT_DATE - INTERVAL '1 month')::int,
        v_struct_standard, 'Standard Employee', 3050.00, 525.00, 2525.00, 6500.00, 21, 20, 1, 'USD', 'computed')
    RETURNING id INTO v_payslip_id;
    INSERT INTO hrm_payslip_lines (payslip_id, org_id, component_id, component_name, component_type, calc_method, computed_amount, display_order) VALUES
        (v_payslip_id, v_org_id, v_comp_hra,       'House Rent Allowance', 'earning',   'pct_of_basic', 2600.00, 1),
        (v_payslip_id, v_org_id, v_comp_medical,   'Medical Allowance',    'earning',   'fixed',         300.00, 2),
        (v_payslip_id, v_org_id, v_comp_transport, 'Transport Allowance',  'earning',   'fixed',         150.00, 3),
        (v_payslip_id, v_org_id, v_comp_pf,        'Provident Fund',       'deduction', 'pct_of_basic',  520.00, 11),
        (v_payslip_id, v_org_id, v_comp_incometax, 'Income Tax',           'deduction', 'slab',            5.00, 12);

    -- Omar (SDR, Sales structure, manual commission not yet entered) — gross 1850, deductions 280, net 1570
    INSERT INTO hrm_payslips (org_id, employee_id, payslip_run_id, period_year, period_month, salary_structure_id, salary_structure_name,
        gross_pay, total_deductions, net_pay, basic_pay, work_days, present_days, currency, status)
    VALUES (v_org_id, v_emp_omar, v_payrun_june, EXTRACT(YEAR FROM CURRENT_DATE - INTERVAL '1 month')::int, EXTRACT(MONTH FROM CURRENT_DATE - INTERVAL '1 month')::int,
        v_struct_sales, 'Sales Team', 1850.00, 280.00, 1570.00, 3500.00, 21, 21, 'USD', 'computed')
    RETURNING id INTO v_payslip_id;
    INSERT INTO hrm_payslip_lines (payslip_id, org_id, component_id, component_name, component_type, calc_method, computed_amount, display_order) VALUES
        (v_payslip_id, v_org_id, v_comp_hra,        'House Rent Allowance', 'earning',   'pct_of_basic', 1400.00, 1),
        (v_payslip_id, v_org_id, v_comp_medical,    'Medical Allowance',    'earning',   'fixed',         300.00, 2),
        (v_payslip_id, v_org_id, v_comp_transport,  'Transport Allowance',  'earning',   'fixed',         150.00, 3),
        (v_payslip_id, v_org_id, v_comp_commission, 'Sales Commission',     'earning',   'manual',          0.00, 4),
        (v_payslip_id, v_org_id, v_comp_pf,         'Provident Fund',       'deduction', 'pct_of_basic',  280.00, 11),
        (v_payslip_id, v_org_id, v_comp_incometax,  'Income Tax',           'deduction', 'slab',            0.00, 12);

    -- ==========================================================
    -- 27. AWARDS (3)
    -- ==========================================================
    -- Arjun — pending_approval (the live test case for award approval wiring)
    INSERT INTO hrm_awards (org_id, employee_id, award_type, title, description, points, monetary_value, currency,
        award_date, issued_by, approval_instance_id, status, created_by) VALUES
        (v_org_id, v_emp_arjun, 'innovation', 'Innovation Spotlight — Q2 2026',
         'Built an internal tool that cut migration script runtime by 70%, nominated by the whole engineering team.',
         300, 250.00, 'USD', CURRENT_DATE - INTERVAL '3 days', v_david_user_id, v_inst_arjun_award, 'pending_approval', v_david_user_id)
        RETURNING id INTO v_award_arjun;

    -- Priya — issued (historical)
    INSERT INTO hrm_awards (org_id, employee_id, award_type, title, description, points, monetary_value, currency,
        award_date, issued_by, status, issued_at, created_by) VALUES
        (v_org_id, v_emp_priya, 'performance', 'Q2 Top Performer',
         'Exceeded quota by 140% in Q2, the highest attainment on the sales team this quarter.',
         500, 1000.00, 'USD', CURRENT_DATE - INTERVAL '25 days', v_mike_id, 'issued', NOW()-INTERVAL '24 days', v_mike_id)
        RETURNING id INTO v_award_priya;

    -- Fatima — issued tenure award, linked to her milestone below
    INSERT INTO hrm_awards (org_id, employee_id, award_type, title, description, points, currency,
        award_date, issued_by, status, issued_at, created_by) VALUES
        (v_org_id, v_emp_fatima, 'tenure', '3-Year Work Anniversary',
         'Celebrating three years of outstanding contribution to the engineering team.',
         150, 'USD', CURRENT_DATE - INTERVAL '150 days', v_ayesha_id, 'issued', NOW()-INTERVAL '150 days', v_ayesha_id)
        RETURNING id INTO v_award_fatima;

    -- ==========================================================
    -- 28. ANNOUNCEMENTS (3)
    -- ==========================================================
    INSERT INTO hrm_announcements (org_id, title, content, category, scope_type, published_at, is_pinned, pin_order, author_id, status, created_by) VALUES
        (v_org_id, 'Welcome to the Team!', 'Please join us in welcoming Omar Farouk to the Sales team as our newest Sales Development Representative.',
         'general', 'organization', NOW()-INTERVAL '30 days', FALSE, 0, v_sarah_id, 'published', v_sarah_id)
        RETURNING id INTO v_ann_welcome;

    INSERT INTO hrm_announcements (org_id, title, content, category, scope_type, published_at, requires_acknowledgement, acknowledgement_deadline,
        is_pinned, pin_order, author_id, status, created_by) VALUES
        (v_org_id, 'Updated Code of Conduct — Please Acknowledge', 'We have refreshed our Code of Conduct for 2026. All employees must review and acknowledge by the deadline below.',
         'policy', 'organization', NOW()-INTERVAL '20 days', TRUE, (CURRENT_DATE+INTERVAL '10 days')::date,
         TRUE, 1, v_sarah_id, 'published', v_sarah_id)
        RETURNING id INTO v_ann_policy;

    INSERT INTO hrm_announcements (org_id, title, content, category, scope_type, published_at, is_pinned, pin_order, author_id, status, created_by) VALUES
        (v_org_id, 'Congratulations to Our Q2 Award Winners', 'A huge congratulations to Priya Patel (Q2 Top Performer) and Fatima Noor (3-Year Work Anniversary) — thank you both for your outstanding contributions!',
         'award', 'organization', NOW()-INTERVAL '23 days', FALSE, 0, v_ayesha_id, 'published', v_ayesha_id)
        RETURNING id INTO v_ann_awards;

    -- ==========================================================
    -- 29. CALENDAR EVENTS (4)
    -- ==========================================================
    INSERT INTO hrm_calendar_events (org_id, title, description, event_type, start_date, end_date, is_all_day,
        scope_type, organizer_id, status, created_by) VALUES
        (v_org_id, 'New Employee Orientation — Q3', 'Onboarding session covering benefits, tools, and company culture.',
         'training', (CURRENT_DATE+INTERVAL '10 days')::date, (CURRENT_DATE+INTERVAL '10 days')::date, TRUE,
         'organization', v_sarah_id, 'upcoming', v_sarah_id);

    INSERT INTO hrm_calendar_events (org_id, title, description, event_type, start_date, end_date, is_all_day,
        scope_type, requires_rsvp, rsvp_deadline, organizer_id, status, created_by) VALUES
        (v_org_id, 'Q3 All-Hands Meeting', 'Company-wide update on Q2 results and Q3 priorities.',
         'company_event', (CURRENT_DATE+INTERVAL '15 days')::date, (CURRENT_DATE+INTERVAL '15 days')::date, TRUE,
         'organization', TRUE, (CURRENT_DATE+INTERVAL '13 days')::date, v_ayesha_id, 'upcoming', v_ayesha_id);

    INSERT INTO hrm_calendar_events (org_id, title, description, event_type, start_date, end_date, is_all_day,
        scope_type, scope_ids, organizer_id, is_auto_generated, source, status, created_by) VALUES
        (v_org_id, 'Fatima Noor''s 3-Year Work Anniversary', 'Celebrating Fatima''s 3rd work anniversary with Nexus Solutions.',
         'work_anniversary', (CURRENT_DATE-INTERVAL '150 days')::date, (CURRENT_DATE-INTERVAL '150 days')::date, TRUE,
         'individual', ARRAY[v_emp_fatima], v_ayesha_id, TRUE, 'milestone', 'completed', v_ayesha_id)
        RETURNING id INTO v_cal_anniv;

    INSERT INTO hrm_calendar_events (org_id, title, description, event_type, start_date, end_date, is_all_day,
        scope_type, organizer_id, is_auto_generated, source, status, created_by) VALUES
        (v_org_id, 'Independence Day Holiday', 'Company holiday — office closed.',
         'holiday', '2026-07-04', '2026-07-04', TRUE, 'organization', v_sarah_id, TRUE, 'holiday_calendar', 'completed', v_sarah_id)
        RETURNING id INTO v_cal_holiday;

    -- ==========================================================
    -- 30. EMPLOYEE MILESTONES (4)
    -- ==========================================================
    INSERT INTO hrm_employee_milestones (org_id, employee_id, milestone_type, title, description, milestone_date, years_count,
        is_auto_generated, auto_award_id, auto_calendar_event_id, is_acknowledged, acknowledged_at, created_by) VALUES
        (v_org_id, v_emp_fatima, 'work_anniversary', '3-Year Work Anniversary', 'Three years at Nexus Solutions',
         (CURRENT_DATE-INTERVAL '150 days')::date, 3, TRUE, v_award_fatima, v_cal_anniv, TRUE, NOW()-INTERVAL '148 days', v_ayesha_id)
        RETURNING id INTO v_mil_fatima_anniv;

    INSERT INTO hrm_employee_milestones (org_id, employee_id, milestone_type, title, description, milestone_date,
        is_auto_generated, is_acknowledged, created_by) VALUES
        (v_org_id, v_emp_fatima, 'promotion', 'Promoted to Senior Software Engineer', 'Career milestone tied to promotion record',
         (CURRENT_DATE-INTERVAL '150 days')::date, FALSE, TRUE, v_ayesha_id);

    INSERT INTO hrm_employee_milestones (org_id, employee_id, milestone_type, title, description, milestone_date,
        is_auto_generated, is_acknowledged, created_by) VALUES
        (v_org_id, v_emp_omar, 'probation_complete', 'Probation Period Ends', 'End of Omar''s 3-month probation period',
         '2026-09-01', TRUE, FALSE, v_sarah_id);

    INSERT INTO hrm_employee_milestones (org_id, employee_id, milestone_type, title, description, milestone_date,
        is_auto_generated, is_acknowledged, created_by) VALUES
        (v_org_id, v_emp_david, 'birthday', 'David Chen''s Birthday', 'Annual birthday reminder',
         (CURRENT_DATE+INTERVAL '25 days')::date, TRUE, FALSE, v_ayesha_id);

    RAISE NOTICE 'HRM seed data created for nexus-solutions: 20 employees across 7 departments, full lifecycle + payroll + approval-chain test fixtures.';
END $$;

-- +goose StatementEnd


-- +goose Down
-- +goose StatementBegin

DO $$
DECLARE
    v_org_id UUID;
BEGIN
    SELECT id INTO v_org_id FROM organizations WHERE LOWER(slug) = 'nexus-solutions';
    IF v_org_id IS NULL THEN RETURN; END IF;

    DELETE FROM hrm_employee_milestones   WHERE org_id = v_org_id;
    DELETE FROM hrm_calendar_events       WHERE org_id = v_org_id;
    DELETE FROM hrm_announcements         WHERE org_id = v_org_id;
    DELETE FROM hrm_awards                WHERE org_id = v_org_id;
    DELETE FROM hrm_payslip_lines         WHERE org_id = v_org_id;
    DELETE FROM hrm_payslips              WHERE org_id = v_org_id;
    DELETE FROM hrm_payslip_runs          WHERE org_id = v_org_id;
    DELETE FROM hrm_attendance_periods    WHERE org_id = v_org_id;
    DELETE FROM hrm_attendance_records    WHERE org_id = v_org_id;
    DELETE FROM hrm_acknowledgements      WHERE org_id = v_org_id;
    DELETE FROM hrm_complaints            WHERE org_id = v_org_id;
    DELETE FROM hrm_employee_warnings     WHERE org_id = v_org_id;
    DELETE FROM hrm_terminations          WHERE org_id = v_org_id;
    DELETE FROM hrm_resignations          WHERE org_id = v_org_id;
    DELETE FROM hrm_transfers             WHERE org_id = v_org_id;
    DELETE FROM hrm_promotions            WHERE org_id = v_org_id;
    DELETE FROM hrm_employee_contracts    WHERE org_id = v_org_id;
    DELETE FROM hrm_calendar_assignments  WHERE org_id = v_org_id;
    DELETE FROM hrm_holidays              WHERE calendar_id IN (SELECT id FROM hrm_holiday_calendars WHERE org_id = v_org_id);
    DELETE FROM hrm_holiday_calendars     WHERE org_id = v_org_id;
    DELETE FROM hrm_work_schedule_assignments WHERE org_id = v_org_id;
    DELETE FROM hrm_shifts                WHERE org_id = v_org_id;
    DELETE FROM hrm_document_bulk_sends   WHERE org_id = v_org_id;
    DELETE FROM hrm_employee_documents    WHERE org_id = v_org_id;
    DELETE FROM hrm_document_templates    WHERE org_id = v_org_id;
    DELETE FROM hrm_warning_escalation_rules WHERE org_id = v_org_id;
    DELETE FROM hrm_employee_warnings     WHERE org_id = v_org_id; -- FK to warning_types
    DELETE FROM hrm_warning_types         WHERE org_id = v_org_id;
    DELETE FROM hrm_approval_decisions    WHERE instance_id IN (SELECT id FROM hrm_approval_instances WHERE org_id = v_org_id);
    DELETE FROM hrm_approval_instances    WHERE org_id = v_org_id;
    DELETE FROM hrm_approval_template_levels WHERE template_id IN (SELECT id FROM hrm_approval_templates WHERE org_id = v_org_id);
    DELETE FROM hrm_approval_templates    WHERE org_id = v_org_id;
    DELETE FROM hrm_employee_salary_records WHERE org_id = v_org_id;
    DELETE FROM hrm_salary_structure_components WHERE structure_id IN (SELECT id FROM hrm_salary_structures WHERE org_id = v_org_id);
    DELETE FROM hrm_salary_structures     WHERE org_id = v_org_id;
    DELETE FROM hrm_salary_components     WHERE org_id = v_org_id;
    DELETE FROM hrm_leave_requests        WHERE org_id = v_org_id;
    DELETE FROM hrm_leave_types           WHERE org_id = v_org_id;

    UPDATE hrm_departments SET head_employee_id = NULL WHERE org_id = v_org_id;
    DELETE FROM hrm_employees             WHERE org_id = v_org_id;
    DELETE FROM hrm_positions             WHERE org_id = v_org_id;
    DELETE FROM hrm_departments            WHERE org_id = v_org_id;

    DELETE FROM organization_members WHERE org_id = v_org_id AND user_id IN (
        SELECT id FROM users WHERE email IN ('david.chen@nexussolutions.io','rebecca.stone@nexussolutions.io',
                                              'tom.bennett@nexussolutions.io','hassan.ali@nexussolutions.io'));
    DELETE FROM users WHERE email IN ('david.chen@nexussolutions.io','rebecca.stone@nexussolutions.io',
                                       'tom.bennett@nexussolutions.io','hassan.ali@nexussolutions.io');
END $$;

-- +goose StatementEnd