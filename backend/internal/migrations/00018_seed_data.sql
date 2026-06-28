-- +goose Up
-- +goose StatementBegin
-- ============================================================
-- Migration: 00018_seed_data
--
-- Seeds realistic development / testing data for BusinessSAAS.
--
-- Scenario: "Nexus Solutions" — a B2B SaaS startup with an
-- active sales team using the CRM module.
--
-- ─── What gets created ───────────────────────────────────────
--   6   Users          (owner, admin, manager, 2× member, viewer)
--   1   Organization   (nexus-solutions)
--   1   Subscription   (pro / active)
--   8   Companies      (platform_companies)
--  12   Contacts       (platform_contacts)
--   1   Pipeline  +  5 Stages
--  17   Deals          (12 open across all stages, 3 won, 2 lost)
--  15   CRM Leads      (4 new, 4 contacted, 3 qualified, 2 unqualified, 2 converted)
--  15   Tasks          (general org tasks — todo / in_progress / done / cancelled)
--  10   Platform tasks (linked to deals & contacts)
--  10   Platform notes
--  12   Platform activities (calls, emails, meetings)
--   8   Platform email logs
--   3   Sessions
--  10   Login events   (7 success, 3 failure)
--   2   Pending invitations
--   8   Audit logs
-- ─────────────────────────────────────────────────────────────
--
-- Password for ALL users:  Password@123
--
--   Owner   →  ayesha@nexussolutions.io
--   Admin   →  sarah@nexussolutions.io
--   Manager →  mike@nexussolutions.io
--   Member  →  priya@nexussolutions.io
--   Member  →  james@nexussolutions.io
--   Viewer  →  diana@nexussolutions.io
-- ============================================================

DO $$
DECLARE
    -- Users
    v_ayesha_id   UUID;
    v_sarah_id    UUID;
    v_mike_id     UUID;
    v_priya_id    UUID;
    v_james_id    UUID;
    v_diana_id    UUID;

    -- Organization
    v_org_id      UUID;

    -- System roles (fetched, not created)
    v_role_owner_id   UUID;
    v_role_admin_id   UUID;
    v_role_manager_id UUID;
    v_role_member_id  UUID;
    v_role_viewer_id  UUID;

    -- Companies
    v_cmp_acme_id     UUID;
    v_cmp_tech_id     UUID;
    v_cmp_global_id   UUID;
    v_cmp_blue_id     UUID;
    v_cmp_meridian_id UUID;
    v_cmp_apex_id     UUID;
    v_cmp_nova_id     UUID;
    v_cmp_cascade_id  UUID;

    -- Contacts
    v_con1_id  UUID;  v_con2_id  UUID;  v_con3_id  UUID;
    v_con4_id  UUID;  v_con5_id  UUID;  v_con6_id  UUID;
    v_con7_id  UUID;  v_con8_id  UUID;  v_con9_id  UUID;
    v_con10_id UUID;  v_con11_id UUID;  v_con12_id UUID;

    -- Pipeline & stages
    v_pipeline_id UUID;
    v_stage1_id   UUID;  -- Prospecting
    v_stage2_id   UUID;  -- Qualification
    v_stage3_id   UUID;  -- Proposal
    v_stage4_id   UUID;  -- Negotiation
    v_stage5_id   UUID;  -- Closing

    -- Open deals
    v_deal1_id  UUID;  v_deal2_id  UUID;  v_deal3_id  UUID;
    v_deal4_id  UUID;  v_deal5_id  UUID;  v_deal6_id  UUID;
    v_deal7_id  UUID;  v_deal8_id  UUID;  v_deal9_id  UUID;
    v_deal10_id UUID;  v_deal11_id UUID;  v_deal12_id UUID;

    -- Won deals referenced by converted leads
    v_won_apex_id  UUID;
    v_won_tvntr_id UUID;

BEGIN
    -- ──────────────────────────────────────────────────────────
    -- GUARD: skip if seed org already exists (idempotent)
    -- ──────────────────────────────────────────────────────────
    IF EXISTS (SELECT 1 FROM organizations WHERE LOWER(slug) = 'nexus-solutions') THEN
        RAISE NOTICE 'Seed data (nexus-solutions) already present — skipping 00018.';
        RETURN;
    END IF;

    -- ==========================================================
    -- 1. USERS   password: Password@123
    -- ==========================================================
    INSERT INTO users (
        email, password_hash,
        first_name, last_name, display_name, full_name,
        email_verified, email_verified_at, status,
        timezone, locale, language, currency, phone,
        last_login_at, last_activity_at
    ) VALUES
        ('ayesha@nexussolutions.io',
         crypt('Password@123', gen_salt('bf', 10)),
         'Ayesha','Rahman','Ayesha Rahman','Ayesha Rahman',
         TRUE, NOW()-INTERVAL '180 days', 'active',
         'America/New_York','en','en','USD','+1-212-555-0101',
         NOW()-INTERVAL '2 hours', NOW()-INTERVAL '2 hours'),

        ('sarah@nexussolutions.io',
         crypt('Password@123', gen_salt('bf', 10)),
         'Sarah','Thompson','Sarah Thompson','Sarah Thompson',
         TRUE, NOW()-INTERVAL '160 days', 'active',
         'America/Los_Angeles','en','en','USD','+1-415-555-0102',
         NOW()-INTERVAL '1 day', NOW()-INTERVAL '1 day'),

        ('mike@nexussolutions.io',
         crypt('Password@123', gen_salt('bf', 10)),
         'Mike','Karim','Mike Karim','Mike Karim',
         TRUE, NOW()-INTERVAL '150 days', 'active',
         'America/Chicago','en','en','USD','+1-312-555-0103',
         NOW()-INTERVAL '3 hours', NOW()-INTERVAL '3 hours'),

        ('priya@nexussolutions.io',
         crypt('Password@123', gen_salt('bf', 10)),
         'Priya','Patel','Priya Patel','Priya Patel',
         TRUE, NOW()-INTERVAL '120 days', 'active',
         'America/New_York','en','en','USD','+1-646-555-0104',
         NOW()-INTERVAL '5 hours', NOW()-INTERVAL '5 hours'),

        ('james@nexussolutions.io',
         crypt('Password@123', gen_salt('bf', 10)),
         'James','Wilson','James Wilson','James Wilson',
         TRUE, NOW()-INTERVAL '90 days', 'active',
         'America/Denver','en','en','USD','+1-720-555-0105',
         NOW()-INTERVAL '2 days', NOW()-INTERVAL '2 days'),

        ('diana@nexussolutions.io',
         crypt('Password@123', gen_salt('bf', 10)),
         'Diana','Lee','Diana Lee','Diana Lee',
         TRUE, NOW()-INTERVAL '60 days', 'active',
         'America/Los_Angeles','en','en','USD','+1-323-555-0106',
         NOW()-INTERVAL '4 days', NOW()-INTERVAL '4 days')
    ON CONFLICT DO NOTHING;

    SELECT id INTO v_ayesha_id FROM users WHERE LOWER(email) = 'ayesha@nexussolutions.io';
    SELECT id INTO v_sarah_id  FROM users WHERE LOWER(email) = 'sarah@nexussolutions.io';
    SELECT id INTO v_mike_id   FROM users WHERE LOWER(email) = 'mike@nexussolutions.io';
    SELECT id INTO v_priya_id  FROM users WHERE LOWER(email) = 'priya@nexussolutions.io';
    SELECT id INTO v_james_id  FROM users WHERE LOWER(email) = 'james@nexussolutions.io';
    SELECT id INTO v_diana_id  FROM users WHERE LOWER(email) = 'diana@nexussolutions.io';

    -- ==========================================================
    -- 2. ORGANIZATION
    -- ==========================================================
    INSERT INTO organizations (
        name, slug, legal_name, type, industry, website,
        country, timezone, currency, status, created_at, updated_at
    ) VALUES (
        'Nexus Solutions', 'nexus-solutions', 'Nexus Solutions LLC',
        'startup', 'Technology', 'https://nexussolutions.io',
        'US', 'America/New_York', 'USD', 'active',
        NOW()-INTERVAL '180 days', NOW()-INTERVAL '2 hours'
    ) ON CONFLICT DO NOTHING;

    SELECT id INTO v_org_id FROM organizations WHERE LOWER(slug) = 'nexus-solutions';

    -- ==========================================================
    -- 3. SUBSCRIPTION  (Pro / monthly)
    -- ==========================================================
    INSERT INTO subscriptions (
        org_id, plan, plan_name, status, billing_cycle, currency, amount,
        current_period_start, current_period_end, created_at, updated_at
    ) VALUES (
        v_org_id, 'pro', 'Pro', 'active', 'monthly', 'USD', 149.00,
        DATE_TRUNC('month', NOW()),
        DATE_TRUNC('month', NOW()) + INTERVAL '1 month',
        NOW()-INTERVAL '180 days', NOW()
    );

    -- ==========================================================
    -- 4. SYSTEM ROLE IDs
    -- ==========================================================
    SELECT id INTO v_role_owner_id   FROM roles WHERE org_id IS NULL AND name = 'owner';
    SELECT id INTO v_role_admin_id   FROM roles WHERE org_id IS NULL AND name = 'admin';
    SELECT id INTO v_role_manager_id FROM roles WHERE org_id IS NULL AND name = 'manager';
    SELECT id INTO v_role_member_id  FROM roles WHERE org_id IS NULL AND name = 'member';
    SELECT id INTO v_role_viewer_id  FROM roles WHERE org_id IS NULL AND name = 'viewer';

    -- ==========================================================
    -- 5. ORGANIZATION MEMBERS
    -- ==========================================================
    INSERT INTO organization_members (
        org_id, user_id, role_id, role_key, title, department, status,
        invitation_status, invitation_accepted_at, joined_at, created_at, updated_at
    ) VALUES
        (v_org_id, v_ayesha_id, v_role_owner_id,   'owner',
         'CEO & Co-Founder',  'Executive',  'active',
         'accepted', NOW()-INTERVAL '180 days', NOW()-INTERVAL '180 days',
         NOW()-INTERVAL '180 days', NOW()-INTERVAL '180 days'),

        (v_org_id, v_sarah_id,  v_role_admin_id,   'admin',
         'Chief of Staff',    'Operations', 'active',
         'accepted', NOW()-INTERVAL '160 days', NOW()-INTERVAL '160 days',
         NOW()-INTERVAL '160 days', NOW()-INTERVAL '160 days'),

        (v_org_id, v_mike_id,   v_role_manager_id, 'manager',
         'Sales Manager',     'Sales',      'active',
         'accepted', NOW()-INTERVAL '150 days', NOW()-INTERVAL '150 days',
         NOW()-INTERVAL '150 days', NOW()-INTERVAL '150 days'),

        (v_org_id, v_priya_id,  v_role_member_id,  'member',
         'Account Executive', 'Sales',      'active',
         'accepted', NOW()-INTERVAL '120 days', NOW()-INTERVAL '120 days',
         NOW()-INTERVAL '120 days', NOW()-INTERVAL '120 days'),

        (v_org_id, v_james_id,  v_role_member_id,  'member',
         'Account Executive', 'Sales',      'active',
         'accepted', NOW()-INTERVAL '90 days', NOW()-INTERVAL '90 days',
         NOW()-INTERVAL '90 days', NOW()-INTERVAL '90 days'),

        (v_org_id, v_diana_id,  v_role_viewer_id,  'viewer',
         'Marketing Analyst', 'Marketing',  'active',
         'accepted', NOW()-INTERVAL '60 days', NOW()-INTERVAL '60 days',
         NOW()-INTERVAL '60 days', NOW()-INTERVAL '60 days')
    ON CONFLICT (org_id, user_id) DO NOTHING;

    -- ==========================================================
    -- 6. PLATFORM COMPANIES  (8)
    -- ==========================================================
    INSERT INTO platform_companies (
        org_id, name, domain, industry, website, phone,
        address, country, status, owner_id, created_by, created_at, updated_at
    ) VALUES
        (v_org_id,'Acme Corporation',   'acme.com',           'Manufacturing',      'https://acme.com',           '+1-800-555-2001','123 Industrial Blvd, Detroit, MI',    'US','active',v_priya_id,v_priya_id,NOW()-INTERVAL '170 days',NOW()-INTERVAL '30 days'),
        (v_org_id,'TechVentures Inc',   'techventures.io',    'Technology',         'https://techventures.io',    '+1-415-555-2002','456 Silicon Ave, San Francisco, CA',   'US','active',v_priya_id,v_priya_id,NOW()-INTERVAL '165 days',NOW()-INTERVAL '15 days'),
        (v_org_id,'GlobalMart Retail',  'globalmart.com',     'Retail',             'https://globalmart.com',     '+1-212-555-2003','789 Commerce St, New York, NY',        'US','active',v_james_id,v_james_id,NOW()-INTERVAL '155 days',NOW()-INTERVAL '10 days'),
        (v_org_id,'BlueSky Dynamics',   'blueskydynamics.com','Aerospace & Defense','https://blueskydynamics.com','+1-202-555-2004','1000 Aerospace Dr, Arlington, VA',     'US','active',v_mike_id, v_mike_id, NOW()-INTERVAL '145 days',NOW()-INTERVAL '20 days'),
        (v_org_id,'Meridian Software',  'meridiansoftware.ca','Software',           'https://meridiansoftware.ca','+1-416-555-2005','200 Tech Park, Toronto, ON',           'CA','active',v_james_id,v_james_id,NOW()-INTERVAL '130 days',NOW()-INTERVAL '5 days'),
        (v_org_id,'Apex Analytics',     'apexanalytics.com',  'Data Analytics',     'https://apexanalytics.com',  '+1-512-555-2006','300 Data Center Rd, Austin, TX',       'US','active',v_priya_id,v_priya_id,NOW()-INTERVAL '110 days',NOW()-INTERVAL '8 days'),
        (v_org_id,'Nova Retail Group',  'novalretail.co.uk',  'Retail',             'https://novalretail.co.uk',  '+44-20-7946-2007','50 Kingsway, London, WC2B 6EX',      'GB','active',v_mike_id, v_mike_id, NOW()-INTERVAL '85 days', NOW()-INTERVAL '12 days'),
        (v_org_id,'Cascade Systems',    'cascadesystems.net', 'IT Services',        'https://cascadesystems.net', '+1-503-555-2008','800 Tech Way, Portland, OR',           'US','active',v_james_id,v_james_id,NOW()-INTERVAL '70 days', NOW()-INTERVAL '3 days')
    ON CONFLICT DO NOTHING;

    SELECT id INTO v_cmp_acme_id     FROM platform_companies WHERE org_id = v_org_id AND name = 'Acme Corporation';
    SELECT id INTO v_cmp_tech_id     FROM platform_companies WHERE org_id = v_org_id AND name = 'TechVentures Inc';
    SELECT id INTO v_cmp_global_id   FROM platform_companies WHERE org_id = v_org_id AND name = 'GlobalMart Retail';
    SELECT id INTO v_cmp_blue_id     FROM platform_companies WHERE org_id = v_org_id AND name = 'BlueSky Dynamics';
    SELECT id INTO v_cmp_meridian_id FROM platform_companies WHERE org_id = v_org_id AND name = 'Meridian Software';
    SELECT id INTO v_cmp_apex_id     FROM platform_companies WHERE org_id = v_org_id AND name = 'Apex Analytics';
    SELECT id INTO v_cmp_nova_id     FROM platform_companies WHERE org_id = v_org_id AND name = 'Nova Retail Group';
    SELECT id INTO v_cmp_cascade_id  FROM platform_companies WHERE org_id = v_org_id AND name = 'Cascade Systems';

    -- ==========================================================
    -- 7. PLATFORM CONTACTS  (12)
    -- ==========================================================
    INSERT INTO platform_contacts (
        org_id, first_name, last_name, email, phone, title,
        company_id, source, status, owner_id, created_by, created_at, updated_at
    ) VALUES
        (v_org_id,'John',    'Mitchell',  'j.mitchell@acme.com',            '+1-313-555-3001','CEO',                  v_cmp_acme_id,    'referral', 'active',v_priya_id,v_priya_id,NOW()-INTERVAL '168 days',NOW()-INTERVAL '25 days'),
        (v_org_id,'Laura',   'Chen',      'l.chen@acme.com',                '+1-313-555-3002','VP of Operations',     v_cmp_acme_id,    'referral', 'active',v_priya_id,v_priya_id,NOW()-INTERVAL '168 days',NOW()-INTERVAL '20 days'),
        (v_org_id,'Marcus',  'Rodriguez', 'm.rodriguez@techventures.io',    '+1-415-555-3003','CTO',                  v_cmp_tech_id,    'referral', 'active',v_priya_id,v_priya_id,NOW()-INTERVAL '163 days',NOW()-INTERVAL '14 days'),
        (v_org_id,'Jennifer','Walsh',     'j.walsh@globalmart.com',         '+1-212-555-3004','Head of Procurement',  v_cmp_global_id,  'cold_call','active',v_james_id,v_james_id,NOW()-INTERVAL '153 days',NOW()-INTERVAL '9 days'),
        (v_org_id,'Tom',     'Bradley',   't.bradley@globalmart.com',       '+1-212-555-3005','Director of IT',       v_cmp_global_id,  'cold_call','active',v_james_id,v_james_id,NOW()-INTERVAL '153 days',NOW()-INTERVAL '7 days'),
        (v_org_id,'Sophia',  'Nakamura',  's.nakamura@blueskydynamics.com', '+1-703-555-3006','VP Engineering',       v_cmp_blue_id,    'event',    'active',v_mike_id, v_mike_id, NOW()-INTERVAL '143 days',NOW()-INTERVAL '18 days'),
        (v_org_id,'Robert',  'Okafor',    'r.okafor@meridiansoftware.ca',   '+1-416-555-3007','Head of Finance',      v_cmp_meridian_id,'partner',  'active',v_james_id,v_james_id,NOW()-INTERVAL '128 days',NOW()-INTERVAL '4 days'),
        (v_org_id,'Emma',    'Richardson','e.richardson@apexanalytics.com', '+1-512-555-3008','Head of Sales',        v_cmp_apex_id,    'web',      'active',v_priya_id,v_priya_id,NOW()-INTERVAL '108 days',NOW()-INTERVAL '6 days'),
        (v_org_id,'David',   'Kim',       'd.kim@novalretail.co.uk',        '+44-7700-900009','CEO',                  v_cmp_nova_id,    'referral', 'active',v_mike_id, v_mike_id, NOW()-INTERVAL '83 days', NOW()-INTERVAL '11 days'),
        (v_org_id,'Caroline','Foster',    'c.foster@cascadesystems.net',    '+1-503-555-3010','CTO',                  v_cmp_cascade_id, 'web',      'active',v_james_id,v_james_id,NOW()-INTERVAL '68 days', NOW()-INTERVAL '2 days'),
        (v_org_id,'Alex',    'Huang',     'a.huang@techventures.io',        '+1-415-555-3011','Director of Strategy', v_cmp_tech_id,    'web',      'active',v_priya_id,v_priya_id,NOW()-INTERVAL '55 days', NOW()-INTERVAL '15 days'),
        (v_org_id,'Maya',    'Santos',    'm.santos@blueskydynamics.com',   '+1-703-555-3012','Operations Director',  v_cmp_blue_id,    'event',    'active',v_mike_id, v_mike_id, NOW()-INTERVAL '40 days', NOW()-INTERVAL '8 days')
    ON CONFLICT DO NOTHING;

    SELECT id INTO v_con1_id  FROM platform_contacts WHERE org_id = v_org_id AND email = 'j.mitchell@acme.com';
    SELECT id INTO v_con2_id  FROM platform_contacts WHERE org_id = v_org_id AND email = 'l.chen@acme.com';
    SELECT id INTO v_con3_id  FROM platform_contacts WHERE org_id = v_org_id AND email = 'm.rodriguez@techventures.io';
    SELECT id INTO v_con4_id  FROM platform_contacts WHERE org_id = v_org_id AND email = 'j.walsh@globalmart.com';
    SELECT id INTO v_con5_id  FROM platform_contacts WHERE org_id = v_org_id AND email = 't.bradley@globalmart.com';
    SELECT id INTO v_con6_id  FROM platform_contacts WHERE org_id = v_org_id AND email = 's.nakamura@blueskydynamics.com';
    SELECT id INTO v_con7_id  FROM platform_contacts WHERE org_id = v_org_id AND email = 'r.okafor@meridiansoftware.ca';
    SELECT id INTO v_con8_id  FROM platform_contacts WHERE org_id = v_org_id AND email = 'e.richardson@apexanalytics.com';
    SELECT id INTO v_con9_id  FROM platform_contacts WHERE org_id = v_org_id AND email = 'd.kim@novalretail.co.uk';
    SELECT id INTO v_con10_id FROM platform_contacts WHERE org_id = v_org_id AND email = 'c.foster@cascadesystems.net';
    SELECT id INTO v_con11_id FROM platform_contacts WHERE org_id = v_org_id AND email = 'a.huang@techventures.io';
    SELECT id INTO v_con12_id FROM platform_contacts WHERE org_id = v_org_id AND email = 'm.santos@blueskydynamics.com';

    -- ==========================================================
    -- 8. CRM PIPELINE  +  STAGES
    -- ==========================================================
    INSERT INTO crm_pipelines (org_id, name, description, is_default, created_by, created_at, updated_at)
    VALUES (v_org_id, 'Sales Pipeline', 'Primary enterprise sales pipeline', TRUE,
            v_ayesha_id, NOW()-INTERVAL '178 days', NOW()-INTERVAL '30 days')
    ON CONFLICT DO NOTHING;

    SELECT id INTO v_pipeline_id FROM crm_pipelines WHERE org_id = v_org_id AND name = 'Sales Pipeline';

    INSERT INTO crm_pipeline_stages (org_id, pipeline_id, name, position, probability, created_at, updated_at)
    VALUES
        (v_org_id, v_pipeline_id, 'Prospecting',   1, 10,  NOW()-INTERVAL '178 days', NOW()),
        (v_org_id, v_pipeline_id, 'Qualification', 2, 25,  NOW()-INTERVAL '178 days', NOW()),
        (v_org_id, v_pipeline_id, 'Proposal',      3, 50,  NOW()-INTERVAL '178 days', NOW()),
        (v_org_id, v_pipeline_id, 'Negotiation',   4, 75,  NOW()-INTERVAL '178 days', NOW()),
        (v_org_id, v_pipeline_id, 'Closing',       5, 90,  NOW()-INTERVAL '178 days', NOW())
    ON CONFLICT DO NOTHING;

    SELECT id INTO v_stage1_id FROM crm_pipeline_stages WHERE pipeline_id = v_pipeline_id AND name = 'Prospecting';
    SELECT id INTO v_stage2_id FROM crm_pipeline_stages WHERE pipeline_id = v_pipeline_id AND name = 'Qualification';
    SELECT id INTO v_stage3_id FROM crm_pipeline_stages WHERE pipeline_id = v_pipeline_id AND name = 'Proposal';
    SELECT id INTO v_stage4_id FROM crm_pipeline_stages WHERE pipeline_id = v_pipeline_id AND name = 'Negotiation';
    SELECT id INTO v_stage5_id FROM crm_pipeline_stages WHERE pipeline_id = v_pipeline_id AND name = 'Closing';

    -- ==========================================================
    -- 9. CRM DEALS  (17 total: 12 open, 3 won, 2 lost)
    -- ==========================================================

    -- Open — Prospecting (3)
    INSERT INTO crm_deals (org_id,pipeline_id,stage_id,title,value,currency,contact_id,company_id,status,close_date,owner_id,created_by,created_at,updated_at) VALUES
        (v_org_id,v_pipeline_id,v_stage1_id,'Acme — Platform Upgrade',          45000,'USD',v_con1_id, v_cmp_acme_id,    'open','2026-09-15',v_priya_id,v_priya_id,NOW()-INTERVAL '45 days', NOW()-INTERVAL '3 days'),
        (v_org_id,v_pipeline_id,v_stage1_id,'Nova Retail Analytics Suite',       78500,'USD',v_con9_id, v_cmp_nova_id,    'open','2026-10-01',v_mike_id, v_mike_id, NOW()-INTERVAL '30 days', NOW()-INTERVAL '2 days'),
        (v_org_id,v_pipeline_id,v_stage1_id,'Cascade Systems — IT Audit',        22000,'USD',v_con10_id,v_cmp_cascade_id, 'open','2026-09-30',v_james_id,v_james_id,NOW()-INTERVAL '20 days', NOW()-INTERVAL '1 day');

    -- Open — Qualification (3)
    INSERT INTO crm_deals (org_id,pipeline_id,stage_id,title,value,currency,contact_id,company_id,status,close_date,owner_id,created_by,created_at,updated_at) VALUES
        (v_org_id,v_pipeline_id,v_stage2_id,'TechVentures — DevOps Bundle',     112000,'USD',v_con3_id, v_cmp_tech_id,    'open','2026-08-20',v_priya_id,v_priya_id,NOW()-INTERVAL '60 days', NOW()-INTERVAL '5 days'),
        (v_org_id,v_pipeline_id,v_stage2_id,'GlobalMart Logistics Platform',    235000,'USD',v_con4_id, v_cmp_global_id,  'open','2026-09-01',v_james_id,v_james_id,NOW()-INTERVAL '55 days', NOW()-INTERVAL '4 days'),
        (v_org_id,v_pipeline_id,v_stage2_id,'Apex Analytics Data Pipeline',      68000,'USD',v_con8_id, v_cmp_apex_id,    'open','2026-08-15',v_priya_id,v_priya_id,NOW()-INTERVAL '40 days', NOW()-INTERVAL '6 days');

    -- Open — Proposal (3)
    INSERT INTO crm_deals (org_id,pipeline_id,stage_id,title,value,currency,contact_id,company_id,status,close_date,owner_id,created_by,created_at,updated_at) VALUES
        (v_org_id,v_pipeline_id,v_stage3_id,'BlueSky Dynamics — Security Suite',185000,'USD',v_con6_id, v_cmp_blue_id,    'open','2026-07-31',v_mike_id, v_mike_id, NOW()-INTERVAL '80 days', NOW()-INTERVAL '10 days'),
        (v_org_id,v_pipeline_id,v_stage3_id,'Meridian Software Consulting',      47500,'USD',v_con7_id, v_cmp_meridian_id,'open','2026-08-10',v_james_id,v_james_id,NOW()-INTERVAL '70 days', NOW()-INTERVAL '8 days'),
        (v_org_id,v_pipeline_id,v_stage3_id,'TechVentures Infrastructure',      165000,'USD',v_con11_id,v_cmp_tech_id,    'open','2026-07-20',v_priya_id,v_priya_id,NOW()-INTERVAL '65 days', NOW()-INTERVAL '7 days');

    -- Open — Negotiation (3)
    INSERT INTO crm_deals (org_id,pipeline_id,stage_id,title,value,currency,contact_id,company_id,status,close_date,owner_id,created_by,created_at,updated_at) VALUES
        (v_org_id,v_pipeline_id,v_stage4_id,'Acme Corp ERP Integration',        320000,'USD',v_con2_id, v_cmp_acme_id,    'open','2026-07-15',v_priya_id,v_priya_id,NOW()-INTERVAL '110 days',NOW()-INTERVAL '12 days'),
        (v_org_id,v_pipeline_id,v_stage4_id,'GlobalMart POS Modernisation',     192000,'USD',v_con5_id, v_cmp_global_id,  'open','2026-07-10',v_james_id,v_james_id,NOW()-INTERVAL '100 days',NOW()-INTERVAL '9 days'),
        (v_org_id,v_pipeline_id,v_stage4_id,'BlueSky Compliance Module',         88500,'USD',v_con12_id,v_cmp_blue_id,    'open','2026-07-05',v_mike_id, v_mike_id, NOW()-INTERVAL '90 days', NOW()-INTERVAL '11 days');

    -- Open — Closing (2)
    INSERT INTO crm_deals (org_id,pipeline_id,stage_id,title,value,currency,contact_id,company_id,status,close_date,owner_id,created_by,created_at,updated_at) VALUES
        (v_org_id,v_pipeline_id,v_stage5_id,'Nova Retail — Full Suite Contract', 410000,'USD',v_con9_id, v_cmp_nova_id,    'open','2026-06-30',v_mike_id, v_mike_id, NOW()-INTERVAL '140 days',NOW()-INTERVAL '1 day'),
        (v_org_id,v_pipeline_id,v_stage5_id,'Cascade Multi-year License',        155000,'USD',v_con10_id,v_cmp_cascade_id, 'open','2026-07-01',v_james_id,v_james_id,NOW()-INTERVAL '130 days',NOW()-INTERVAL '2 days');

    -- Won (3)
    INSERT INTO crm_deals (org_id,pipeline_id,stage_id,title,value,currency,contact_id,company_id,status,close_date,won_at,owner_id,created_by,created_at,updated_at) VALUES
        (v_org_id,v_pipeline_id,v_stage5_id,'Apex Analytics — Starter Package',  25000,'USD',v_con8_id, v_cmp_apex_id,    'won','2026-02-28',NOW()-INTERVAL '118 days',v_priya_id,v_priya_id,NOW()-INTERVAL '160 days',NOW()-INTERVAL '118 days'),
        (v_org_id,v_pipeline_id,v_stage5_id,'TechVentures Seed Deal',            55000,'USD',v_con3_id, v_cmp_tech_id,    'won','2026-03-31',NOW()-INTERVAL '87 days', v_priya_id,v_priya_id,NOW()-INTERVAL '150 days',NOW()-INTERVAL '87 days'),
        (v_org_id,v_pipeline_id,v_stage5_id,'Meridian Annual Subscription',      96000,'USD',v_con7_id, v_cmp_meridian_id,'won','2026-04-30',NOW()-INTERVAL '57 days', v_james_id,v_james_id,NOW()-INTERVAL '140 days',NOW()-INTERVAL '57 days');

    -- Lost (2)
    INSERT INTO crm_deals (org_id,pipeline_id,stage_id,title,value,currency,contact_id,company_id,status,close_date,lost_at,lost_reason,owner_id,created_by,created_at,updated_at) VALUES
        (v_org_id,v_pipeline_id,v_stage4_id,'Acme Corp — Basic Module',          18000,'USD',v_con1_id, v_cmp_acme_id,    'lost','2026-01-31',NOW()-INTERVAL '146 days','Chose competitor solution — lower price point',           v_priya_id,v_priya_id,NOW()-INTERVAL '175 days',NOW()-INTERVAL '146 days'),
        (v_org_id,v_pipeline_id,v_stage3_id,'BlueSky Pilot Project',             32000,'USD',v_con6_id, v_cmp_blue_id,    'lost','2026-03-15',NOW()-INTERVAL '103 days','Budget freeze Q1 — flagged for Q3 re-engagement follow-up',v_mike_id, v_mike_id, NOW()-INTERVAL '155 days',NOW()-INTERVAL '103 days');

    -- Fetch deal IDs needed for engagement records
    SELECT id INTO v_deal1_id  FROM crm_deals WHERE org_id = v_org_id AND title = 'Acme — Platform Upgrade';
    SELECT id INTO v_deal2_id  FROM crm_deals WHERE org_id = v_org_id AND title = 'Nova Retail Analytics Suite';
    SELECT id INTO v_deal3_id  FROM crm_deals WHERE org_id = v_org_id AND title = 'Cascade Systems — IT Audit';
    SELECT id INTO v_deal4_id  FROM crm_deals WHERE org_id = v_org_id AND title = 'TechVentures — DevOps Bundle';
    SELECT id INTO v_deal5_id  FROM crm_deals WHERE org_id = v_org_id AND title = 'GlobalMart Logistics Platform';
    SELECT id INTO v_deal6_id  FROM crm_deals WHERE org_id = v_org_id AND title = 'Apex Analytics Data Pipeline';
    SELECT id INTO v_deal7_id  FROM crm_deals WHERE org_id = v_org_id AND title = 'BlueSky Dynamics — Security Suite';
    SELECT id INTO v_deal8_id  FROM crm_deals WHERE org_id = v_org_id AND title = 'Meridian Software Consulting';
    SELECT id INTO v_deal9_id  FROM crm_deals WHERE org_id = v_org_id AND title = 'TechVentures Infrastructure';
    SELECT id INTO v_deal10_id FROM crm_deals WHERE org_id = v_org_id AND title = 'Acme Corp ERP Integration';
    SELECT id INTO v_deal11_id FROM crm_deals WHERE org_id = v_org_id AND title = 'Nova Retail — Full Suite Contract';
    SELECT id INTO v_deal12_id FROM crm_deals WHERE org_id = v_org_id AND title = 'Cascade Multi-year License';
    SELECT id INTO v_won_apex_id  FROM crm_deals WHERE org_id = v_org_id AND title = 'Apex Analytics — Starter Package';
    SELECT id INTO v_won_tvntr_id FROM crm_deals WHERE org_id = v_org_id AND title = 'TechVentures Seed Deal';

    -- ==========================================================
    -- 10. CRM LEADS  (15)
    -- ==========================================================

    -- New (4)
    INSERT INTO crm_leads (org_id,first_name,last_name,email,phone,company_name,title,source,status,owner_id,created_by,created_at,updated_at) VALUES
        (v_org_id,'Nathan', 'Brooks',  'n.brooks@prospectcorp.com',   '+1-617-555-4001','ProspectCorp',      'Head of Engineering',   'web',      'new',v_priya_id,v_priya_id,NOW()-INTERVAL '8 days', NOW()-INTERVAL '8 days'),
        (v_org_id,'Elena',  'Vasquez', 'e.vasquez@deltatech.io',      '+1-214-555-4002','DeltaTech',         'VP Product',            'social',   'new',v_james_id,v_james_id,NOW()-INTERVAL '6 days', NOW()-INTERVAL '6 days'),
        (v_org_id,'Kevin',  'Park',    'k.park@orbitsystems.com',     '+1-408-555-4003','Orbit Systems',     'Director of IT',        'web',      'new',v_priya_id,v_priya_id,NOW()-INTERVAL '3 days', NOW()-INTERVAL '3 days'),
        (v_org_id,'Fatima', 'Al-Said', 'f.alsaid@crescentgroup.ae',   '+971-4-555-4004','Crescent Group',    'CEO',                   'referral', 'new',v_mike_id, v_mike_id, NOW()-INTERVAL '1 day',  NOW()-INTERVAL '1 day');

    -- Contacted (4)
    INSERT INTO crm_leads (org_id,first_name,last_name,email,phone,company_name,title,source,status,owner_id,created_by,created_at,updated_at) VALUES
        (v_org_id,'Lucas', 'Martin',  'l.martin@primelogix.com',     '+1-404-555-4005','PrimeLogix',        'CTO',                   'cold_call','contacted',v_james_id,v_james_id,NOW()-INTERVAL '25 days',NOW()-INTERVAL '18 days'),
        (v_org_id,'Amara', 'Osei',    'a.osei@panafrictech.co',      '+233-302-555-006','PanAfriTech',       'Head of Digital',       'event',    'contacted',v_mike_id, v_mike_id, NOW()-INTERVAL '22 days',NOW()-INTERVAL '15 days'),
        (v_org_id,'Liam',  'OBrien',  'l.obrien@irelandfintech.ie',  '+353-1-555-4007','Ireland FinTech',   'Partnerships Director', 'partner',  'contacted',v_priya_id,v_priya_id,NOW()-INTERVAL '18 days',NOW()-INTERVAL '12 days'),
        (v_org_id,'Yuki',  'Tanaka',  'y.tanaka@sakuramedical.jp',   '+81-3-555-4008', 'Sakura Medical',    'Director of Operations','email',    'contacted',v_james_id,v_james_id,NOW()-INTERVAL '15 days',NOW()-INTERVAL '10 days');

    -- Qualified (3)
    INSERT INTO crm_leads (org_id,first_name,last_name,email,phone,company_name,title,source,status,owner_id,created_by,created_at,updated_at) VALUES
        (v_org_id,'Samuel', 'Adeyemi', 's.adeyemi@lagosenergy.ng',   '+234-1-555-4009','Lagos Energy',       'COO',                  'referral', 'qualified',v_mike_id, v_mike_id, NOW()-INTERVAL '50 days',NOW()-INTERVAL '8 days'),
        (v_org_id,'Chloe',  'Dupont',  'c.dupont@parissolutions.fr', '+33-1-5555-4010','Paris Solutions',    'VP Operations',         'event',    'qualified',v_priya_id,v_priya_id,NOW()-INTERVAL '45 days',NOW()-INTERVAL '6 days'),
        (v_org_id,'Raj',    'Krishnan','r.krishnan@bangalorehub.in', '+91-80-555-4011','Bangalore Hub',      'Head of Technology',    'web',      'qualified',v_james_id,v_james_id,NOW()-INTERVAL '38 days',NOW()-INTERVAL '5 days');

    -- Unqualified (2)
    INSERT INTO crm_leads (org_id,first_name,last_name,email,phone,company_name,title,source,status,owner_id,created_by,created_at,updated_at) VALUES
        (v_org_id,'Tyler',  'Greene', 't.greene@freelancestudio.com','+1-512-555-4012','Freelance Studio',  'Owner',                 'web',   'unqualified',v_priya_id,v_priya_id,NOW()-INTERVAL '60 days',NOW()-INTERVAL '55 days'),
        (v_org_id,'Monica', 'Singh',  'm.singh@studentproject.edu',  '+1-617-555-4013','Student Project Co','Student',               'email', 'unqualified',v_james_id,v_james_id,NOW()-INTERVAL '42 days',NOW()-INTERVAL '40 days');

    -- Converted (2) — reference existing contacts + won deals
    INSERT INTO crm_leads (org_id,first_name,last_name,email,phone,company_name,title,source,status,
        converted_at,converted_contact_id,converted_deal_id,owner_id,created_by,created_at,updated_at) VALUES
        (v_org_id,'Emma',   'Richardson','e.richardson@apexanalytics.com','+1-512-555-3008','Apex Analytics',  'Head of Sales','web',     'converted',
         NOW()-INTERVAL '128 days',v_con8_id,v_won_apex_id, v_priya_id,v_priya_id,NOW()-INTERVAL '160 days',NOW()-INTERVAL '128 days'),
        (v_org_id,'Marcus', 'Rodriguez', 'm.rodriguez@techventures.io',  '+1-415-555-3003','TechVentures Inc','CTO',         'referral','converted',
         NOW()-INTERVAL '138 days',v_con3_id,v_won_tvntr_id,v_priya_id,v_priya_id,NOW()-INTERVAL '165 days',NOW()-INTERVAL '138 days');

    -- ==========================================================
    -- 11. GENERAL TASKS  (15)
    -- ==========================================================
    INSERT INTO tasks (org_id, title, description, status, due_date, created_by, assigned_to, created_at, updated_at)
    VALUES
        -- todo (5)
        (v_org_id,'Q3 Sales Strategy Review',     'Review Q3 sales targets and revise territory strategy based on H1 performance',          'todo',       NOW()+INTERVAL '7 days',  v_mike_id,  v_mike_id,  NOW()-INTERVAL '2 days',  NOW()-INTERVAL '2 days'),
        (v_org_id,'CRM Data Quality Audit',       'Identify and merge duplicate contacts; archive stale leads older than 90 days',          'todo',       NOW()+INTERVAL '14 days', v_sarah_id, v_priya_id, NOW()-INTERVAL '3 days',  NOW()-INTERVAL '3 days'),
        (v_org_id,'New Hire Onboarding — SDR',    'Prepare onboarding pack, system access and 30-day ramp plan for incoming SDR',           'todo',       NOW()+INTERVAL '10 days', v_sarah_id, v_sarah_id, NOW()-INTERVAL '1 day',   NOW()-INTERVAL '1 day'),
        (v_org_id,'Renewal Pipeline Review',      'Review all accounts with renewals due in Q3 2026 and flag churn risks',                  'todo',       NOW()+INTERVAL '5 days',  v_mike_id,  v_james_id, NOW()-INTERVAL '4 days',  NOW()-INTERVAL '4 days'),
        (v_org_id,'Competitor Analysis Update',   'Update competitive landscape deck to include two new entrants identified in Q2',         'todo',       NOW()+INTERVAL '21 days', v_ayesha_id,v_diana_id, NOW()-INTERVAL '5 days',  NOW()-INTERVAL '5 days'),
        -- in_progress (5)
        (v_org_id,'Enterprise Pricing Deck',      'Build updated pricing presentation for 100+ seat enterprise tiers with ROI calculator',  'in_progress',NOW()+INTERVAL '3 days',  v_mike_id,  v_priya_id, NOW()-INTERVAL '10 days', NOW()-INTERVAL '1 day'),
        (v_org_id,'Monthly Report — June 2026',   'Compile June sales KPIs: pipeline velocity, conversion rates, and revenue vs. target',   'in_progress',NOW()+INTERVAL '4 days',  v_mike_id,  v_mike_id,  NOW()-INTERVAL '8 days',  NOW()-INTERVAL '2 hours'),
        (v_org_id,'SOC2 Type II Renewal',         'Coordinate with security team and external auditor for annual SOC2 Type II renewal',     'in_progress',NOW()+INTERVAL '30 days', v_sarah_id, v_sarah_id, NOW()-INTERVAL '45 days', NOW()-INTERVAL '3 days'),
        (v_org_id,'Website Content Refresh',      'Update all product pages, case studies, and G2 review links for Q3 campaign launch',    'in_progress',NOW()+INTERVAL '14 days', v_ayesha_id,v_diana_id, NOW()-INTERVAL '15 days', NOW()-INTERVAL '1 day'),
        (v_org_id,'API Docs v2.5 Update',         'Update all API endpoint documentation to reflect v2.5 schema and response changes',     'in_progress',NOW()+INTERVAL '7 days',  v_sarah_id, v_priya_id, NOW()-INTERVAL '12 days', NOW()-INTERVAL '6 hours'),
        -- done (4)
        (v_org_id,'Q1 2026 Board Deck',           'Prepared Q1 financial results and KPIs for board of directors presentation',            'done',       NOW()-INTERVAL '90 days', v_ayesha_id,v_ayesha_id,NOW()-INTERVAL '110 days',NOW()-INTERVAL '92 days'),
        (v_org_id,'Sales Kickoff 2026',           'Planned and executed annual sales team kickoff with guest speakers',                    'done',       NOW()-INTERVAL '150 days',v_mike_id,  v_sarah_id, NOW()-INTERVAL '170 days',NOW()-INTERVAL '152 days'),
        (v_org_id,'CRM Module Internal Launch',   'Ran internal CRM launch event including demo, Q&A, and training schedule',              'done',       NOW()-INTERVAL '120 days',v_ayesha_id,v_mike_id,  NOW()-INTERVAL '145 days',NOW()-INTERVAL '122 days'),
        (v_org_id,'CRM Training — All AEs',       'Conducted hands-on CRM training session for all account executives',                   'done',       NOW()-INTERVAL '100 days',v_mike_id,  v_mike_id,  NOW()-INTERVAL '115 days',NOW()-INTERVAL '102 days'),
        -- cancelled (1)
        (v_org_id,'Legacy CRM Data Migration',    'Migrate 5 years of data from old CRM — deprioritised after data audit revealed poor quality', 'cancelled', NOW()-INTERVAL '30 days',v_sarah_id,v_sarah_id,NOW()-INTERVAL '80 days', NOW()-INTERVAL '35 days');

    -- ==========================================================
    -- 12. PLATFORM TASKS  (10 — linked to deals / contacts)
    -- ==========================================================
    INSERT INTO platform_tasks (
        org_id, module, title, description, due_date, status, priority,
        related_type, related_id, assigned_to, created_by, created_at, updated_at
    ) VALUES
        (v_org_id,'crm','Send ERP proposal to Acme',          'Custom pricing for 500-seat 3-year license; include implementation timeline',    NOW()+INTERVAL '2 days', 'open',     'high',  'crm.deal',        v_deal10_id,v_priya_id,v_priya_id,NOW()-INTERVAL '5 days', NOW()-INTERVAL '5 days'),
        (v_org_id,'crm','Schedule BlueSky CISO intro call',   'Security deep-dive post-whitepaper; invite CISO + Sophia Nakamura',              NOW()+INTERVAL '3 days', 'open',     'high',  'crm.deal',        v_deal7_id, v_mike_id, v_mike_id, NOW()-INTERVAL '7 days', NOW()-INTERVAL '2 days'),
        (v_org_id,'crm','Chase Nova Retail contract sign-off','Legal review expected by June 30 — daily follow-up with David Kim''s office',    NOW()+INTERVAL '4 days', 'open',     'high',  'crm.deal',        v_deal11_id,v_mike_id, v_mike_id, NOW()-INTERVAL '3 days', NOW()-INTERVAL '3 days'),
        (v_org_id,'crm','ROI model for GlobalMart',           'Build logistics cost-savings model based on 800 stores — include 3 scenarios',   NOW()+INTERVAL '5 days', 'open',     'medium','crm.deal',        v_deal5_id, v_james_id,v_james_id,NOW()-INTERVAL '4 days', NOW()-INTERVAL '4 days'),
        (v_org_id,'crm','Send DevOps case studies to Marcus', 'Share 3 DevOps scale success stories — focus on Kubernetes migration wins',      NOW()+INTERVAL '1 day',  'open',     'medium','crm.deal',        v_deal4_id, v_priya_id,v_priya_id,NOW()-INTERVAL '2 days', NOW()-INTERVAL '1 day'),
        (v_org_id,'crm','Follow up with Jennifer Walsh',      'Verify PO approval timeline — procurement committee meets July 15',              NOW()-INTERVAL '2 days', 'open',     'high',  'platform.contact', v_con4_id,  v_james_id,v_james_id,NOW()-INTERVAL '6 days', NOW()-INTERVAL '6 days'),
        (v_org_id,'crm','Confirm NDA signing — Cascade',      'Follow up on outstanding NDA; work cannot begin until signed',                   NOW()-INTERVAL '1 day',  'open',     'medium','crm.deal',        v_deal3_id, v_james_id,v_james_id,NOW()-INTERVAL '8 days', NOW()-INTERVAL '8 days'),
        (v_org_id,'crm','Update Meridian deal notes post-call','Add pricing feedback from Robert Okafor call; note NET60 agreement',            NOW()-INTERVAL '3 days', 'completed','medium','crm.deal',        v_deal8_id, v_james_id,v_james_id,NOW()-INTERVAL '10 days',NOW()-INTERVAL '3 days'),
        (v_org_id,'crm','Review Apex Q2 requirements doc',    'New procurement policy — verify our proposal still complies',                    NOW()-INTERVAL '5 days', 'completed','low',   'crm.deal',        v_deal6_id, v_priya_id,v_priya_id,NOW()-INTERVAL '15 days',NOW()-INTERVAL '5 days'),
        (v_org_id,'crm','Welcome call — John Mitchell',       'Introductory relationship call; set expectations and agree on communication cadence', NOW()-INTERVAL '10 days','completed','medium','platform.contact',v_con1_id,v_priya_id,v_priya_id,NOW()-INTERVAL '20 days',NOW()-INTERVAL '10 days');

    -- ==========================================================
    -- 13. PLATFORM NOTES  (10)
    -- ==========================================================
    INSERT INTO platform_notes (org_id, module, content, related_type, related_id, created_by, created_at, updated_at)
    VALUES
        (v_org_id,'crm',
         'Discovery call with John Mitchell went very well. He is specifically interested in the reporting module and wants to upgrade from 200 to 500 seats. Budget pre-approved for H2 2026. Moving this to proposal stage next week.',
         'crm.deal',v_deal1_id,v_priya_id,NOW()-INTERVAL '42 days',NOW()-INTERVAL '42 days'),

        (v_org_id,'crm',
         'Marcus confirmed TechVentures is evaluating 3 vendors. Our DevOps integration story is the clearest differentiator — their engineering team specifically asked about the Kubernetes operator. Sandbox access set up; evaluation starts Monday.',
         'crm.deal',v_deal4_id,v_priya_id,NOW()-INTERVAL '35 days',NOW()-INTERVAL '35 days'),

        (v_org_id,'crm',
         'Jennifer Walsh confirmed the GlobalMart procurement committee meets on the 15th of each month. We must submit final pricing by July 10 to make the August committee date. Tom Bradley (Director of IT) is our internal champion — keep him close.',
         'crm.deal',v_deal5_id,v_james_id,NOW()-INTERVAL '28 days',NOW()-INTERVAL '28 days'),

        (v_org_id,'crm',
         'Sophia Nakamura was very technical — asked detailed questions on encryption at rest/in transit, audit logging completeness, and data residency options. Their CISO must review before any decision. Sent security whitepaper + SOC2 report same evening.',
         'crm.deal',v_deal7_id,v_mike_id, NOW()-INTERVAL '20 days',NOW()-INTERVAL '20 days'),

        (v_org_id,'crm',
         'Acme ERP deal escalated to board level. Laura Chen confirmed they are considering a 3-year contract worth $320K. Legal review has begun on our standard MSA. This is our largest active deal — Ayesha to join the next executive call for sponsorship.',
         'crm.deal',v_deal10_id,v_priya_id,NOW()-INTERVAL '15 days',NOW()-INTERVAL '15 days'),

        (v_org_id,'crm',
         'David Kim approved the Full Suite deal in principle after our executive sync. Legal team reviewing our standard contract. 12% volume discount agreed and approved by Ayesha. Expect signature by June 30 — flagship win for the UK market.',
         'crm.deal',v_deal11_id,v_mike_id, NOW()-INTERVAL '10 days',NOW()-INTERVAL '10 days'),

        (v_org_id,'crm',
         'John Mitchell is extremely relationship-oriented. Prefers short calls over long emails. Has been promoting Nexus at industry roundtables unprompted. Strong candidate for our October customer speaker programme — flag for marketing.',
         'platform.contact',v_con1_id,v_priya_id,NOW()-INTERVAL '100 days',NOW()-INTERVAL '100 days'),

        (v_org_id,'crm',
         'Marcus Rodriguez is highly influential in the SF DevOps community. His recent blog post on container orchestration got 15K reads. Excellent case study candidate once TechVentures deal closes — get NDA-compliant approval upfront.',
         'platform.contact',v_con3_id,v_priya_id,NOW()-INTERVAL '80 days',NOW()-INTERVAL '80 days'),

        (v_org_id,'crm',
         'GlobalMart procurement requires 3 signatures: Jennifer Walsh, Tom Bradley, and VP of Finance. Tom is our champion. Jennifer is cautious about off-shore data storage — address this explicitly with our US-East data residency option in the proposal.',
         'platform.contact',v_con4_id,v_james_id,NOW()-INTERVAL '60 days',NOW()-INTERVAL '60 days'),

        (v_org_id,'crm',
         'Robert Okafor is detail-oriented on contract language. The 90-day net payment clause is a sticking point. Proposed NET60 for year 1 and discussed with Ayesha — she approved. James is sending revised proposal with updated terms today.',
         'platform.contact',v_con7_id,v_james_id,NOW()-INTERVAL '25 days',NOW()-INTERVAL '25 days');

    -- ==========================================================
    -- 14. PLATFORM ACTIVITIES  (12)
    -- ==========================================================
    INSERT INTO platform_activities (
        org_id, module, type, subject, description, outcome,
        related_type, related_id, occurred_at, duration_mins,
        created_by, created_at, updated_at
    ) VALUES
        (v_org_id,'crm','call',   'Discovery — Acme Platform Upgrade',
         'Introductory discovery call with John Mitchell. Covered pain points with legacy reporting and identified 3 key use cases for the platform upgrade.',
         'Positive — proceeding to proposal stage',
         'crm.deal',v_deal1_id,NOW()-INTERVAL '42 days',45,v_priya_id,NOW()-INTERVAL '42 days',NOW()-INTERVAL '42 days'),

        (v_org_id,'crm','email',  'Follow-up — TechVentures DevOps Bundle',
         'Sent comprehensive feature overview to Marcus Rodriguez including API documentation, sandbox credentials, and a customised DevOps use-case one-pager.',
         'Email opened — sandbox activated — reply pending',
         'crm.deal',v_deal4_id,NOW()-INTERVAL '32 days',NULL,v_priya_id,NOW()-INTERVAL '32 days',NOW()-INTERVAL '32 days'),

        (v_org_id,'crm','meeting','In-person demo — GlobalMart',
         'On-site demo at GlobalMart HQ in New York. Presented the logistics module to Jennifer Walsh and Tom Bradley. Strong engagement; Jennifer asked about data residency options.',
         'Requested formal pricing proposal by end of month',
         'crm.deal',v_deal5_id,NOW()-INTERVAL '26 days',90,v_james_id,NOW()-INTERVAL '26 days',NOW()-INTERVAL '26 days'),

        (v_org_id,'crm','call',   'Security review — BlueSky Security Suite',
         'Technical deep-dive with Sophia Nakamura and their CISO. Walked through SOC2 Type II report, pen test results, and encryption standards.',
         'CISO wants formal security spec and data processing agreement',
         'crm.deal',v_deal7_id,NOW()-INTERVAL '18 days',60,v_mike_id,NOW()-INTERVAL '18 days',NOW()-INTERVAL '18 days'),

        (v_org_id,'crm','meeting','Executive meeting — Acme ERP Integration',
         'Met with Laura Chen and Acme General Counsel to discuss contract framework. 3-year commitment structure agreed in principle. Legal review begins next week.',
         '3-year deal agreed in principle — legal review started',
         'crm.deal',v_deal10_id,NOW()-INTERVAL '12 days',120,v_priya_id,NOW()-INTERVAL '12 days',NOW()-INTERVAL '12 days'),

        (v_org_id,'crm','call',   'Contract negotiation — Nova Retail Full Suite',
         'Weekly sync with David Kim to finalise contract terms. Volume discount of 12% agreed. 3-year SLA structure confirmed. Ayesha approved discount level.',
         'Contract being finalised — signature expected by June 30',
         'crm.deal',v_deal11_id,NOW()-INTERVAL '5 days',30,v_mike_id,NOW()-INTERVAL '5 days',NOW()-INTERVAL '5 days'),

        (v_org_id,'crm','email',  'Proposal — Meridian Software Consulting',
         'Sent custom consulting proposal to Robert Okafor with revised payment terms: NET60 year 1, NET30 thereafter. Added 5% volume discount for teams over 50 users.',
         'Awaiting response — follow up in 3 business days',
         'crm.deal',v_deal8_id,NOW()-INTERVAL '22 days',NULL,v_james_id,NOW()-INTERVAL '22 days',NOW()-INTERVAL '22 days'),

        (v_org_id,'crm','call',   'Qualification — Apex Analytics Data Pipeline',
         'Qualification call with Emma Richardson. Clear fit — data engineering team of 12 are daily pipeline tool users. Budget range confirmed at $65K–75K.',
         'Qualified — moving to proposal stage',
         'crm.deal',v_deal6_id,NOW()-INTERVAL '35 days',30,v_priya_id,NOW()-INTERVAL '35 days',NOW()-INTERVAL '35 days'),

        (v_org_id,'crm','meeting','Technical workshop — TechVentures Infrastructure',
         'Full-day workshop with Alex Huang and their engineering team. Covered API integration patterns, data migration approach, and customisation roadmap.',
         'Very positive — team impressed; proposal requested by end of week',
         'crm.deal',v_deal9_id,NOW()-INTERVAL '8 days',360,v_priya_id,NOW()-INTERVAL '8 days',NOW()-INTERVAL '8 days'),

        (v_org_id,'crm','call',   'Monthly check-in — John Mitchell',
         'Scheduled relationship call with John Mitchell. He mentioned Acme is exploring API integrations for a new product line — potential upsell opportunity worth flagging.',
         'Upsell opportunity identified — create separate deal after ERP closes',
         'platform.contact',v_con1_id,NOW()-INTERVAL '30 days',20,v_priya_id,NOW()-INTERVAL '30 days',NOW()-INTERVAL '30 days'),

        (v_org_id,'crm','email',  'Introduction — Caroline Foster at Cascade',
         'Cold outreach to Caroline Foster at Cascade Systems on referral from John Mitchell at Acme. Included product overview brochure and two IT audit case studies.',
         'Reply received within 24h — interested in an initial call',
         'platform.contact',v_con10_id,NOW()-INTERVAL '65 days',NULL,v_james_id,NOW()-INTERVAL '65 days',NOW()-INTERVAL '65 days'),

        (v_org_id,'crm','meeting','Engagement kickoff — Cascade IT Audit',
         'Project kickoff with Caroline Foster and Cascade IT Director. Agreed scope: 6-week audit covering network infrastructure, access controls, and DR readiness.',
         'Kickoff successful — audit work begins next Monday',
         'crm.deal',v_deal3_id,NOW()-INTERVAL '15 days',60,v_james_id,NOW()-INTERVAL '15 days',NOW()-INTERVAL '15 days');

    -- ==========================================================
    -- 15. PLATFORM EMAIL LOGS  (8)
    -- ==========================================================
    INSERT INTO platform_email_logs (
        org_id, module, subject, body, from_email, to_email,
        direction, status, related_type, related_id, sent_at, created_by, created_at
    ) VALUES
        (v_org_id,'crm',
         'Platform Upgrade Proposal — Acme Corporation',
         'Hi John, as discussed in our call, please find attached our proposal for the platform upgrade. We have tailored the pricing for 500 seats with annual billing and included the enhanced reporting module you highlighted. Looking forward to your feedback.',
         'priya@nexussolutions.io','j.mitchell@acme.com',
         'outbound','sent','crm.deal',v_deal1_id,NOW()-INTERVAL '38 days',v_priya_id,NOW()-INTERVAL '38 days'),

        (v_org_id,'crm',
         'Sandbox Access — TechVentures DevOps Evaluation',
         'Hi Marcus, your sandbox environment is ready. Login credentials are in the attached document and the DevOps integration guide is included. Please do not hesitate to reach out during the evaluation period.',
         'priya@nexussolutions.io','m.rodriguez@techventures.io',
         'outbound','sent','crm.deal',v_deal4_id,NOW()-INTERVAL '30 days',v_priya_id,NOW()-INTERVAL '30 days'),

        (v_org_id,'crm',
         'Pricing Proposal — GlobalMart Logistics Platform',
         'Hi Jennifer, it was great meeting you and Tom in New York last week. Please find our formal pricing proposal attached. We have incorporated the logistics optimisation module and addressed your data residency requirements with our US-East data centre.',
         'james@nexussolutions.io','j.walsh@globalmart.com',
         'outbound','sent','crm.deal',v_deal5_id,NOW()-INTERVAL '23 days',v_james_id,NOW()-INTERVAL '23 days'),

        (v_org_id,'crm',
         'Security Whitepaper + SOC2 Report — BlueSky Dynamics',
         'Hi Sophia, following our security review call I am sending over our SOC2 Type II report and security architecture whitepaper for your CISO. Please let me know if any additional documentation would be helpful.',
         'mike@nexussolutions.io','s.nakamura@blueskydynamics.com',
         'outbound','sent','crm.deal',v_deal7_id,NOW()-INTERVAL '16 days',v_mike_id,NOW()-INTERVAL '16 days'),

        (v_org_id,'crm',
         'Revised Consulting Proposal — Meridian Software',
         'Hi Robert, as requested I have revised the payment terms to NET60 for Year 1 and NET30 from Year 2 onwards. I have also added a 5% volume discount clause for teams over 50 users. Please find the updated proposal attached.',
         'james@nexussolutions.io','r.okafor@meridiansoftware.ca',
         'outbound','sent','crm.deal',v_deal8_id,NOW()-INTERVAL '20 days',v_james_id,NOW()-INTERVAL '20 days'),

        (v_org_id,'crm',
         'Re: Revised Payment Terms — Clarification Needed',
         'Hi James, thank you for the updated proposal. Quick question: does the NET60 term apply to implementation fees as well as the software licence? We are also open to a 4-year term in exchange for a better annual rate. Can we schedule a call this week?',
         'r.okafor@meridiansoftware.ca','james@nexussolutions.io',
         'inbound','received','crm.deal',v_deal8_id,NOW()-INTERVAL '17 days',v_james_id,NOW()-INTERVAL '17 days'),

        (v_org_id,'crm',
         'Nova Retail Group — Contract Draft v1.0',
         'Hi David, please find attached the initial contract draft for the 3-year Full Suite agreement. Our legal team has cleared the standard terms. Please review and let us know if any modifications are required on your end.',
         'mike@nexussolutions.io','d.kim@novalretail.co.uk',
         'outbound','sent','crm.deal',v_deal11_id,NOW()-INTERVAL '8 days',v_mike_id,NOW()-INTERVAL '8 days'),

        (v_org_id,'crm',
         'Introduction — Nexus Solutions + Cascade Systems',
         'Hi Caroline, I am reaching out on a warm referral from John Mitchell at Acme Corporation. We recently helped Acme modernise their IT infrastructure, and John thought Cascade could benefit from a similar engagement. I would love 30 minutes to introduce ourselves.',
         'james@nexussolutions.io','c.foster@cascadesystems.net',
         'outbound','sent','platform.contact',v_con10_id,NOW()-INTERVAL '67 days',v_james_id,NOW()-INTERVAL '67 days');

    -- ==========================================================
    -- 16. SESSIONS  (3)
    -- ==========================================================
    INSERT INTO sessions (
        user_id, org_id, token_hash, device_name, device_type, browser, os,
        user_agent, ip_address, country, city,
        last_activity_at, created_at, expires_at
    ) VALUES
        (v_ayesha_id, v_org_id,
         encode(digest('nexus-seed-sess-ayesha-macbook-2026',      'sha256'), 'hex'),
         'MacBook Pro 16"',  'desktop','Safari','macOS 14 Sonoma',
         'Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/17.0 Safari/605.1.15',
         '98.206.1.100','US','New York',
         NOW()-INTERVAL '2 hours',  NOW()-INTERVAL '2 hours',  NOW()+INTERVAL '7 days'),

        (v_ayesha_id, v_org_id,
         encode(digest('nexus-seed-sess-ayesha-iphone15-2026',     'sha256'), 'hex'),
         'iPhone 15 Pro',    'mobile', 'Safari','iOS 17',
         'Mozilla/5.0 (iPhone; CPU iPhone OS 17_0 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/17.0 Mobile/15E148 Safari/604.1',
         '98.206.1.101','US','New York',
         NOW()-INTERVAL '1 day',    NOW()-INTERVAL '5 days',   NOW()+INTERVAL '2 days'),

        (v_mike_id,   v_org_id,
         encode(digest('nexus-seed-sess-mike-chrome-windows-2026', 'sha256'), 'hex'),
         'Dell XPS 15',      'desktop','Chrome','Windows 11',
         'Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36',
         '173.52.144.55','US','Chicago',
         NOW()-INTERVAL '3 hours',  NOW()-INTERVAL '3 hours',  NOW()+INTERVAL '7 days')
    ON CONFLICT DO NOTHING;

    -- ==========================================================
    -- 17. LOGIN EVENTS  (10: 7 success, 3 failure)
    -- ==========================================================
    INSERT INTO login_events (user_id, email, provider, status, failure_reason, ip_address, user_agent, country, city, created_at)
    VALUES
        (v_ayesha_id,'ayesha@nexussolutions.io','credentials','success',NULL,           '98.206.1.100',  'Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/605.1.15',          'US','New York',    NOW()-INTERVAL '2 hours'),
        (v_ayesha_id,'ayesha@nexussolutions.io','credentials','success',NULL,           '98.206.1.100',  'Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/605.1.15',          'US','New York',    NOW()-INTERVAL '1 day'),
        (v_mike_id,  'mike@nexussolutions.io',  'credentials','success',NULL,           '173.52.144.55', 'Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36',                  'US','Chicago',     NOW()-INTERVAL '3 hours'),
        (v_priya_id, 'priya@nexussolutions.io', 'credentials','success',NULL,           '72.161.54.22',  'Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/605.1.15',          'US','New York',    NOW()-INTERVAL '5 hours'),
        (v_james_id, 'james@nexussolutions.io', 'credentials','success',NULL,           '67.180.113.14', 'Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36',                  'US','Denver',      NOW()-INTERVAL '2 days'),
        (v_sarah_id, 'sarah@nexussolutions.io', 'credentials','success',NULL,           '76.174.203.40', 'Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/605.1.15',          'US','Los Angeles', NOW()-INTERVAL '1 day'),
        (v_diana_id, 'diana@nexussolutions.io', 'credentials','success',NULL,           '76.174.203.41', 'Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome', 'US','Los Angeles', NOW()-INTERVAL '4 days'),
        (NULL,       'ayesha@nexussolutions.io','credentials','failure','invalid_credentials','45.142.212.100','python-requests/2.31.0','RU','Moscow',   NOW()-INTERVAL '3 days'),
        (NULL,       'admin@nexussolutions.io', 'credentials','failure','user_not_found',     '45.142.212.101','python-requests/2.31.0','RU','Moscow',   NOW()-INTERVAL '3 days'),
        (NULL,       'ayesha@nexussolutions.io','credentials','failure','invalid_credentials','103.55.37.15',  'curl/7.88.1',           'CN','Shanghai', NOW()-INTERVAL '5 days');

    -- ==========================================================
    -- 18. PENDING INVITATIONS  (2)
    -- ==========================================================
    INSERT INTO organization_invitations (
        org_id, email, role_id, role_key, title, department,
        token_hash, status, invited_by, expires_at, last_sent_at, created_at, updated_at
    ) VALUES
        (v_org_id, 'alex.ngo@nexussolutions.io',
         v_role_member_id, 'member', 'Sales Development Rep', 'Sales',
         encode(digest('nexus-seed-invite-alex-ngo-2026', 'sha256'), 'hex'),
         'pending', v_mike_id,
         NOW()+INTERVAL '6 days', NOW()-INTERVAL '1 day', NOW()-INTERVAL '1 day', NOW()-INTERVAL '1 day'),

        (v_org_id, 'zara.hussain@nexussolutions.io',
         v_role_viewer_id, 'viewer', 'Data Analyst Intern', 'Marketing',
         encode(digest('nexus-seed-invite-zara-hussain-2026', 'sha256'), 'hex'),
         'pending', v_sarah_id,
         NOW()+INTERVAL '5 days', NOW()-INTERVAL '2 days', NOW()-INTERVAL '2 days', NOW()-INTERVAL '2 days')
    ON CONFLICT DO NOTHING;

    -- ==========================================================
    -- 19. AUDIT LOGS  (8)
    -- ==========================================================
    INSERT INTO audit_logs (org_id, user_id, event_type, description, resource_type, resource_id, status, ip_address, created_at)
    VALUES
        (v_org_id,v_ayesha_id,'organization.created',    'Organization ''Nexus Solutions'' created',             'organization',     v_org_id::TEXT,         'success','98.206.1.100', NOW()-INTERVAL '180 days'),
        (v_org_id,v_ayesha_id,'auth.sign_in',             'Owner signed in from New York (MacBook)',              'user',             v_ayesha_id::TEXT,      'success','98.206.1.100', NOW()-INTERVAL '2 hours'),
        (v_org_id,v_mike_id,  'member.invited',           'Invited alex.ngo@nexussolutions.io as member',        'organization',     v_org_id::TEXT,         'success','173.52.144.55',NOW()-INTERVAL '1 day'),
        (v_org_id,v_sarah_id, 'member.invited',           'Invited zara.hussain@nexussolutions.io as viewer',    'organization',     v_org_id::TEXT,         'success','76.174.203.40',NOW()-INTERVAL '2 days'),
        (v_org_id,v_ayesha_id,'role.permissions.updated', 'Updated permissions for system role ''manager''',     'role',             v_role_manager_id::TEXT,'success','98.206.1.100', NOW()-INTERVAL '120 days'),
        (v_org_id,v_sarah_id, 'member.role.changed',      'Changed James Wilson''s role from viewer to member',  'organization',     v_org_id::TEXT,         'success','76.174.203.40',NOW()-INTERVAL '89 days'),
        (v_org_id,v_priya_id, 'crm.lead.converted',       'Lead Emma Richardson converted → contact + deal',     'crm_lead',         v_con8_id::TEXT,        'success','72.161.54.22', NOW()-INTERVAL '128 days'),
        (v_org_id,v_priya_id, 'crm.lead.converted',       'Lead Marcus Rodriguez converted → contact + deal',    'crm_lead',         v_con3_id::TEXT,        'success','72.161.54.22', NOW()-INTERVAL '138 days');

END $$;
-- +goose StatementEnd


-- +goose Down
-- +goose StatementBegin
-- ============================================================
-- Removes all seed data for the 'nexus-solutions' organization.
-- Safe to run multiple times (exits early if org not found).
-- ============================================================

DO $$
DECLARE
    v_org_id UUID;
BEGIN
    SELECT id INTO v_org_id FROM organizations WHERE LOWER(slug) = 'nexus-solutions';
    IF v_org_id IS NULL THEN
        RAISE NOTICE 'Seed org ''nexus-solutions'' not found — nothing to remove.';
        RETURN;
    END IF;

    -- Platform engagement (no FK issues; delete first)
    DELETE FROM platform_email_logs WHERE org_id = v_org_id;
    DELETE FROM platform_activities  WHERE org_id = v_org_id;
    DELETE FROM platform_notes       WHERE org_id = v_org_id;
    DELETE FROM platform_tasks       WHERE org_id = v_org_id;

    -- CRM: delete deals first (pipeline/stage have ON DELETE RESTRICT from deals)
    DELETE FROM crm_deals            WHERE org_id = v_org_id;
    DELETE FROM crm_leads            WHERE org_id = v_org_id;
    DELETE FROM crm_pipeline_stages  WHERE org_id = v_org_id;
    DELETE FROM crm_pipelines        WHERE org_id = v_org_id;

    -- Platform entities
    DELETE FROM platform_contacts    WHERE org_id = v_org_id;
    DELETE FROM platform_companies   WHERE org_id = v_org_id;

    -- General tasks
    DELETE FROM tasks                WHERE org_id = v_org_id;

    -- Security / membership / billing
    DELETE FROM sessions               WHERE org_id = v_org_id;
    DELETE FROM audit_logs             WHERE org_id = v_org_id;
    DELETE FROM organization_invitations WHERE org_id = v_org_id;
    DELETE FROM organization_members   WHERE org_id = v_org_id;
    DELETE FROM subscriptions          WHERE org_id = v_org_id;

    -- Organization itself
    DELETE FROM organizations          WHERE id = v_org_id;

    -- Login events (not org-scoped — match by known seed emails)
    DELETE FROM login_events WHERE email IN (
        'ayesha@nexussolutions.io', 'sarah@nexussolutions.io',
        'mike@nexussolutions.io',   'priya@nexussolutions.io',
        'james@nexussolutions.io',  'diana@nexussolutions.io',
        'admin@nexussolutions.io'
    );

    -- Users last (other tables reference them with SET NULL cascades)
    DELETE FROM users WHERE LOWER(email) IN (
        'ayesha@nexussolutions.io', 'sarah@nexussolutions.io',
        'mike@nexussolutions.io',   'priya@nexussolutions.io',
        'james@nexussolutions.io',  'diana@nexussolutions.io'
    );

    RAISE NOTICE 'Seed data for ''nexus-solutions'' removed successfully.';
END $$;
-- +goose StatementEnd