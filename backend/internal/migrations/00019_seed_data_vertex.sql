-- +goose Up
-- +goose StatementBegin
-- ============================================================
-- Migration: 00019_seed_data_vertex
--
-- Seeds a completely different set of realistic testing data.
-- Scenario: "Vertex Logistics" — a Supply Chain Tech & Freight 
-- Forwarding B2B SaaS company with an active global sales pipeline.
--
-- ─── What gets created ───────────────────────────────────────
--   6   Users          (owner, admin, manager, 2× member, viewer)
--   1   Organization   (vertex-logistics)
--   1   Subscription   (enterprise / active)
--   8   Companies      (platform_companies)
--  12   Contacts       (platform_contacts)
--   1   Pipeline  +  5 Stages
--  17   Deals          (12 open, 3 won, 2 lost)
--  15   CRM Leads      (4 new, 4 contacted, 3 qualified, 2 unqualified, 2 converted)
--  15   Tasks          (general org tasks — todo / in_progress / done)
--  10   Platform tasks (linked to deals & contacts)
--  10   Platform notes
--  12   Platform activities
--   8   Platform email logs
--   3   Sessions
--  10   Login events   
--   2   Pending invitations
--   8   Audit logs
-- ─────────────────────────────────────────────────────────────
--
-- Password for ALL users:  Password@123
--
--   Owner   →  tariq@vertexlogistics.io
--   Admin   →  elena@vertexlogistics.io
--   Manager →  marcus@vertexlogistics.io
--   Member  →  ananya@vertexlogistics.io
--   Member  →  david@vertexlogistics.io
--   Viewer  →  yuki@vertexlogistics.io
-- ============================================================

DO $$
DECLARE
    -- Users
    v_tariq_id    UUID;
    v_elena_id    UUID;
    v_marcus_id   UUID;
    v_ananya_id   UUID;
    v_david_id    UUID;
    v_yuki_id     UUID;

    -- Organization
    v_org_id      UUID;

    -- System roles
    v_role_owner_id   UUID;
    v_role_admin_id   UUID;
    v_role_manager_id UUID;
    v_role_member_id  UUID;
    v_role_viewer_id  UUID;

    -- Companies
    v_cmp_atlas_id    UUID;
    v_cmp_beacon_id   UUID;
    v_cmp_cargo_id    UUID;
    v_cmp_horizon_id  UUID;
    v_cmp_infinity_id UUID;
    v_cmp_matrix_id   UUID;
    v_cmp_vanguard_id UUID;
    v_cmp_zenith_id   UUID;

    -- Contacts
    v_con1_id  UUID;  v_con2_id  UUID;  v_con3_id  UUID;
    v_con4_id  UUID;  v_con5_id  UUID;  v_con6_id  UUID;
    v_con7_id  UUID;  v_con8_id  UUID;  v_con9_id  UUID;
    v_con10_id UUID;  v_con11_id UUID;  v_con12_id UUID;

    -- Pipeline & stages
    v_pipeline_id UUID;
    v_stage1_id   UUID;
    v_stage2_id   UUID;
    v_stage3_id   UUID;
    v_stage4_id   UUID;
    v_stage5_id   UUID;

    -- Open deals
    v_deal1_id  UUID;  v_deal2_id  UUID;  v_deal3_id  UUID;
    v_deal4_id  UUID;  v_deal5_id  UUID;  v_deal6_id  UUID;
    v_deal7_id  UUID;  v_deal8_id  UUID;  v_deal9_id  UUID;
    v_deal10_id UUID;  v_deal11_id UUID;  v_deal12_id UUID;

    -- Won deals referenced by converted leads
    v_won_matrix_id UUID;
    v_won_atlas_id  UUID;

BEGIN
    -- ──────────────────────────────────────────────────────────
    -- GUARD: skip if seed org already exists
    -- ──────────────────────────────────────────────────────────
    IF EXISTS (SELECT 1 FROM organizations WHERE LOWER(slug) = 'vertex-logistics') THEN
        RAISE NOTICE 'Seed data (vertex-logistics) already present — skipping 00019.';
        RETURN;
    END IF;

    -- ==========================================================
    -- 1. USERS (Password: Password@123)
    -- ==========================================================
    INSERT INTO users (
        email, password_hash,
        first_name, last_name, display_name, full_name,
        email_verified, email_verified_at, status,
        timezone, locale, language, currency, phone,
        last_login_at, last_activity_at
    ) VALUES
        ('tariq@vertexlogistics.io', crypt('Password@123', gen_salt('bf', 10)),
         'Tariq', 'Ahmed', 'Tariq Ahmed', 'Tariq Ahmed', TRUE, NOW()-INTERVAL '120 days', 'active',
         'Europe/London', 'en', 'en', 'USD', '+44-20-7946-0958', NOW()-INTERVAL '1 hour', NOW()-INTERVAL '1 hour'),

        ('elena@vertexlogistics.io', crypt('Password@123', gen_salt('bf', 10)),
         'Elena', 'Rostova', 'Elena Rostova', 'Elena Rostova', TRUE, NOW()-INTERVAL '110 days', 'active',
         'Europe/Paris', 'en', 'en', 'EUR', '+33-1-5555-0143', NOW()-INTERVAL '1 day', NOW()-INTERVAL '1 day'),

        ('marcus@vertexlogistics.io', crypt('Password@123', gen_salt('bf', 10)),
         'Marcus', 'Vance', 'Marcus Vance', 'Marcus Vance', TRUE, NOW()-INTERVAL '100 days', 'active',
         'America/New_York', 'en', 'en', 'USD', '+1-212-555-0199', NOW()-INTERVAL '4 hours', NOW()-INTERVAL '4 hours'),

        ('ananya@vertexlogistics.io', crypt('Password@123', gen_salt('bf', 10)),
         'Ananya', 'Iyer', 'Ananya Iyer', 'Ananya Iyer', TRUE, NOW()-INTERVAL '90 days', 'active',
         'Asia/Kolkata', 'en', 'en', 'INR', '+91-22-5555-0182', NOW()-INTERVAL '3 hours', NOW()-INTERVAL '3 hours'),

        ('david@vertexlogistics.io', crypt('Password@123', gen_salt('bf', 10)),
         'David', 'Beck', 'David Beck', 'David Beck', TRUE, NOW()-INTERVAL '80 days', 'active',
         'America/Los_Angeles', 'en', 'en', 'USD', '+1-415-555-0147', NOW()-INTERVAL '3 days', NOW()-INTERVAL '3 days'),

        ('yuki@vertexlogistics.io', crypt('Password@123', gen_salt('bf', 10)),
         'Yuki', 'Sato', 'Yuki Sato', 'Yuki Sato', TRUE, NOW()-INTERVAL '60 days', 'active',
         'Asia/Tokyo', 'en', 'ja', 'JPY', '+81-3-5555-0122', NOW()-INTERVAL '5 days', NOW()-INTERVAL '5 days')
    ON CONFLICT DO NOTHING;

    SELECT id INTO v_tariq_id  FROM users WHERE LOWER(email) = 'tariq@vertexlogistics.io';
    SELECT id INTO v_elena_id  FROM users WHERE LOWER(email) = 'elena@vertexlogistics.io';
    SELECT id INTO v_marcus_id FROM users WHERE LOWER(email) = 'marcus@vertexlogistics.io';
    SELECT id INTO v_ananya_id FROM users WHERE LOWER(email) = 'ananya@vertexlogistics.io';
    SELECT id INTO v_david_id  FROM users WHERE LOWER(email) = 'david@vertexlogistics.io';
    SELECT id INTO v_yuki_id   FROM users WHERE LOWER(email) = 'yuki@vertexlogistics.io';

    -- ==========================================================
    -- 2. ORGANIZATION
    -- ==========================================================
    INSERT INTO organizations (
        name, slug, legal_name, type, industry, website,
        country, timezone, currency, status, created_at, updated_at
    ) VALUES (
        'Vertex Logistics', 'vertex-logistics', 'Vertex Freight Global Ltd',
        'enterprise', 'Transportation', 'https://vertexlogistics.io',
        'GB', 'Europe/London', 'USD', 'active',
        NOW()-INTERVAL '120 days', NOW()-INTERVAL '1 hour'
    ) ON CONFLICT DO NOTHING;

    SELECT id INTO v_org_id FROM organizations WHERE LOWER(slug) = 'vertex-logistics';

    -- ==========================================================
    -- 3. SUBSCRIPTION (Enterprise)
    -- ==========================================================
    INSERT INTO subscriptions (
        org_id, plan, plan_name, status, billing_cycle, currency, amount,
        current_period_start, current_period_end, created_at, updated_at
    ) VALUES (
        v_org_id, 'enterprise', 'Enterprise Scale', 'active', 'yearly', 'USD', 4999.00,
        DATE_TRUNC('month', NOW()),
        DATE_TRUNC('month', NOW()) + INTERVAL '1 year',
        NOW()-INTERVAL '120 days', NOW()
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
        (v_org_id, v_tariq_id,  v_role_owner_id,   'owner',   'Managing Director', 'Executive',  'active', 'accepted', NOW()-INTERVAL '120 days', NOW()-INTERVAL '120 days', NOW()-INTERVAL '120 days', NOW()-INTERVAL '120 days'),
        (v_org_id, v_elena_id,  v_role_admin_id,   'admin',   'Operations Director','Operations','active', 'accepted', NOW()-INTERVAL '110 days', NOW()-INTERVAL '110 days', NOW()-INTERVAL '110 days', NOW()-INTERVAL '110 days'),
        (v_org_id, v_marcus_id, v_role_manager_id, 'manager', 'Global Sales Head', 'Sales',      'active', 'accepted', NOW()-INTERVAL '100 days', NOW()-INTERVAL '100 days', NOW()-INTERVAL '100 days', NOW()-INTERVAL '100 days'),
        (v_org_id, v_ananya_id, v_role_member_id,  'member',  'Senior Enterprise AE','Sales',     'active', 'accepted', NOW()-INTERVAL '90 days',  NOW()-INTERVAL '90 days',  NOW()-INTERVAL '90 days',  NOW()-INTERVAL '90 days'),
        (v_org_id, v_david_id,  v_role_member_id,  'member',  'Logistics AE',       'Sales',      'active', 'accepted', NOW()-INTERVAL '80 days',  NOW()-INTERVAL '80 days',  NOW()-INTERVAL '80 days',  NOW()-INTERVAL '80 days'),
        (v_org_id, v_yuki_id,   v_role_viewer_id,  'viewer',  'Supply Chain Analyst','Strategy',  'active', 'accepted', NOW()-INTERVAL '60 days',  NOW()-INTERVAL '60 days',  NOW()-INTERVAL '60 days',  NOW()-INTERVAL '60 days')
    ON CONFLICT (org_id, user_id) DO NOTHING;

    -- ==========================================================
    -- 6. PLATFORM COMPANIES (8 New Companies)
    -- ==========================================================
    INSERT INTO platform_companies (
        org_id, name, domain, industry, website, phone,
        address, country, status, owner_id, created_by, created_at, updated_at
    ) VALUES
        (v_org_id,'Atlas Maritime',    'atlasmaritime.com', 'Shipping & Marine', 'https://atlasmaritime.com', '+1-206-555-8801','Pipeline Pier 4, Seattle, WA',      'US','active',v_ananya_id,v_ananya_id,NOW()-INTERVAL '115 days',NOW()-INTERVAL '10 days'),
        (v_org_id,'Beacon Cold Storage','beaconcold.com',   'Food & Beverages',  'https://beaconcold.com',   '+1-612-555-8802','500 Frost Ave, Minneapolis, MN',    'US','active',v_ananya_id,v_ananya_id,NOW()-INTERVAL '110 days',NOW()-INTERVAL '12 days'),
        (v_org_id,'CargoLink Europe',  'cargolink.nl',      'Logistics & Freight','https://cargolink.nl',      '+31-20-746-0031','Schiphol Cargo Hub, Amsterdam',     'NL','active',v_david_id, v_david_id, NOW()-INTERVAL '105 days',NOW()-INTERVAL '5 days'),
        (v_org_id,'Horizon Aviation',  'horizonair.com',    'Aviation',          'https://horizonair.com',    '+44-161-555-0922','Hangar 12, Manchester Airport',     'GB','active',v_marcus_id,v_marcus_id,NOW()-INTERVAL '95 days', NOW()-INTERVAL '8 days'),
        (v_org_id,'Infinity Supply',   'infinitysupply.sg', 'Wholesale',         'https://infinitysupply.sg', '+65-6733-4012',  '10 Changi South St, Singapore',     'SG','active',v_david_id, v_david_id, NOW()-INTERVAL '80 days', NOW()-INTERVAL '2 days'),
        (v_org_id,'Matrix Pharma Tech','matrixpharma.de',   'Pharmaceuticals',   'https://matrixpharma.de',   '+49-30-8891-042', 'Biotech Park, Berlin',              'DE','active',v_ananya_id,v_ananya_id,NOW()-INTERVAL '75 days', NOW()-INTERVAL '4 days'),
        (v_org_id,'Vanguard Freight',  'vanguardfreight.au','Transportation',    'https://vanguardfreight.au','+61-2-9284-0192', 'Port Botany Yards, Sydney, NSW',    'AU','active',v_marcus_id,v_marcus_id,NOW()-INTERVAL '60 days', NOW()-INTERVAL '1 day'),
        (v_org_id,'Zenith Manufacturing','zenithparts.com',  'Automotive',        'https://zenithparts.com',  '+1-313-555-9921','88 Assembly Way, Detroit, MI',      'US','active',v_david_id, v_david_id, NOW()-INTERVAL '50 days', NOW()-INTERVAL '7 days')
    ON CONFLICT DO NOTHING;

    SELECT id INTO v_cmp_atlas_id    FROM platform_companies WHERE org_id = v_org_id AND name = 'Atlas Maritime';
    SELECT id INTO v_cmp_beacon_id   FROM platform_companies WHERE org_id = v_org_id AND name = 'Beacon Cold Storage';
    SELECT id INTO v_cmp_cargo_id    FROM platform_companies WHERE org_id = v_org_id AND name = 'CargoLink Europe';
    SELECT id INTO v_cmp_horizon_id  FROM platform_companies WHERE org_id = v_org_id AND name = 'Horizon Aviation';
    SELECT id INTO v_cmp_infinity_id FROM platform_companies WHERE org_id = v_org_id AND name = 'Infinity Supply';
    SELECT id INTO v_cmp_matrix_id   FROM platform_companies WHERE org_id = v_org_id AND name = 'Matrix Pharma Tech';
    SELECT id INTO v_cmp_vanguard_id FROM platform_companies WHERE org_id = v_org_id AND name = 'Vanguard Freight';
    SELECT id INTO v_cmp_zenith_id       FROM platform_companies WHERE org_id = v_org_id AND name = 'Zenith Manufacturing';

    -- ==========================================================
    -- 7. PLATFORM CONTACTS (12 New Contacts)
    -- ==========================================================
    INSERT INTO platform_contacts (
        org_id, first_name, last_name, email, phone, title,
        company_id, source, status, owner_id, created_by, created_at, updated_at
    ) VALUES
        (v_org_id,'Captain Erik','Nielsen', 'e.nielsen@atlasmaritime.com','+1-206-555-0911','Fleet Operations Director', v_cmp_atlas_id,   'partner',  'active',v_ananya_id,v_ananya_id,NOW()-INTERVAL '112 days',NOW()-INTERVAL '10 days'),
        (v_org_id,'Martha',      'Stewart', 'm.stewart@beaconcold.com',  '+1-612-555-4321','Chief Supply Chain Officer', v_cmp_beacon_id,  'web',      'active',v_ananya_id,v_ananya_id,NOW()-INTERVAL '108 days',NOW()-INTERVAL '12 days'),
        (v_org_id,'Hans',        'Vermeer', 'h.vermeer@cargolink.nl',    '+31-20-746-1122','Rotterdam Terminal Mgr',    v_cmp_cargo_id,   'cold_call','active',v_david_id, v_david_id, NOW()-INTERVAL '102 days',NOW()-INTERVAL '5 days'),
        (v_org_id,'Sir Alan',    'Cross',   'a.cross@horizonair.com',    '+44-161-555-1100','VP Procurement',             v_cmp_horizon_id, 'referral', 'active',v_marcus_id,v_marcus_id,NOW()-INTERVAL '92 days', NOW()-INTERVAL '8 days'),
        (v_org_id,'Lin',         'Wei',     'l.wei@infinitysupply.sg',   '+65-6733-9988',  'Global Sourcing Lead',       v_cmp_infinity_id,'event',    'active',v_david_id, v_david_id, NOW()-INTERVAL '78 days', NOW()-INTERVAL '2 days'),
        (v_org_id,'Dr. Stefan',  'Müller',  's.mueller@matrixpharma.de', '+49-30-8891-991', 'Cold Chain Quality Lead',    v_cmp_matrix_id,  'web',      'active',v_ananya_id,v_ananya_id,NOW()-INTERVAL '72 days', NOW()-INTERVAL '4 days'),
        (v_org_id,'Bruce',       'Wayne',   'b.wayne@vanguardfreight.au','+61-2-9284-8831', 'Intermodal Coordinator',     v_cmp_vanguard_id,'partner',  'active',v_marcus_id,v_marcus_id,NOW()-INTERVAL '58 days', NOW()-INTERVAL '1 day'),
        (v_org_id,'Sarah',       'Connor',  's.connor@zenithparts.com',  '+1-313-555-7761','JIT Inventory Supervisor',   v_cmp_zenith_id,      'cold_call','active',v_david_id, v_david_id, NOW()-INTERVAL '48 days', NOW()-INTERVAL '7 days'),
        (v_org_id,'Nils',        'Sjoberg', 'n.sjoberg@atlasmaritime.com','+1-206-555-0912','Port Captain',               v_cmp_atlas_id,   'partner',  'active',v_ananya_id,v_ananya_id,NOW()-INTERVAL '40 days', NOW()-INTERVAL '5 days'),
        (v_org_id,'Francois',    'Dubois',  'f.dubois@horizonair.com',   '+33-1-5555-9871', 'Customs Compliance Mgr',      v_cmp_horizon_id, 'event',    'active',v_marcus_id,v_marcus_id,NOW()-INTERVAL '35 days', NOW()-INTERVAL '2 days'),
        (v_org_id,'Arjun',       'Mehta',   'a.mehta@infinitysupply.sg', '+65-6733-9989',  'Warehouse Management Exec',  v_cmp_infinity_id,'web',      'active',v_david_id, v_david_id, NOW()-INTERVAL '25 days', NOW()-INTERVAL '1 day'),
        (v_org_id,'Klaus',       'Fischer', 'k.fischer@matrixpharma.de', '+49-30-8891-992', 'Logistics Procurement Director',v_cmp_matrix_id,  'referral', 'active',v_ananya_id,v_ananya_id,NOW()-INTERVAL '15 days', NOW()-INTERVAL '4 hours')
    ON CONFLICT DO NOTHING;

    SELECT id INTO v_con1_id  FROM platform_contacts WHERE org_id = v_org_id AND email = 'e.nielsen@atlasmaritime.com';
    SELECT id INTO v_con2_id  FROM platform_contacts WHERE org_id = v_org_id AND email = 'm.stewart@beaconcold.com';
    SELECT id INTO v_con3_id  FROM platform_contacts WHERE org_id = v_org_id AND email = 'h.vermeer@cargolink.nl';
    SELECT id INTO v_con4_id  FROM platform_contacts WHERE org_id = v_org_id AND email = 'a.cross@horizonair.com';
    SELECT id INTO v_con5_id  FROM platform_contacts WHERE org_id = v_org_id AND email = 'l.wei@infinitysupply.sg';
    SELECT id INTO v_con6_id  FROM platform_contacts WHERE org_id = v_org_id AND email = 's.mueller@matrixpharma.de';
    SELECT id INTO v_con7_id  FROM platform_contacts WHERE org_id = v_org_id AND email = 'b.wayne@vanguardfreight.au';
    SELECT id INTO v_con8_id  FROM platform_contacts WHERE org_id = v_org_id AND email = 's.connor@zenithparts.com';
    SELECT id INTO v_con9_id  FROM platform_contacts WHERE org_id = v_org_id AND email = 'n.sjoberg@atlasmaritime.com';
    SELECT id INTO v_con10_id FROM platform_contacts WHERE org_id = v_org_id AND email = 'f.dubois@horizonair.com';
    SELECT id INTO v_con11_id FROM platform_contacts WHERE org_id = v_org_id AND email = 'a.mehta@infinitysupply.sg';
    SELECT id INTO v_con12_id FROM platform_contacts WHERE org_id = v_org_id AND email = 'k.fischer@matrixpharma.de';

    -- ==========================================================
    -- 8. CRM PIPELINE + STAGES
    -- ==========================================================
    INSERT INTO crm_pipelines (org_id, name, description, is_default, created_by, created_at, updated_at)
    VALUES (v_org_id, 'Global Freight Pipeline', 'Pipeline for international logistics routing contracts', TRUE,
            v_tariq_id, NOW()-INTERVAL '118 days', NOW()-INTERVAL '15 days')
    ON CONFLICT DO NOTHING;

    SELECT id INTO v_pipeline_id FROM crm_pipelines WHERE org_id = v_org_id AND name = 'Global Freight Pipeline';

    INSERT INTO crm_pipeline_stages (org_id, pipeline_id, name, position, probability, created_at, updated_at)
    VALUES
        (v_org_id, v_pipeline_id, 'Lead Identified',   1, 15,  NOW()-INTERVAL '118 days', NOW()),
        (v_org_id, v_pipeline_id, 'Route Verification',2, 35,  NOW()-INTERVAL '118 days', NOW()),
        (v_org_id, v_pipeline_id, 'Tariff Quotation',  3, 60,  NOW()-INTERVAL '118 days', NOW()),
        (v_org_id, v_pipeline_id, 'Customs Review',    4, 80,  NOW()-INTERVAL '118 days', NOW()),
        (v_org_id, v_pipeline_id, 'Final Allocation',  5, 95,  NOW()-INTERVAL '118 days', NOW())
    ON CONFLICT DO NOTHING;

    SELECT id INTO v_stage1_id FROM crm_pipeline_stages WHERE pipeline_id = v_pipeline_id AND name = 'Lead Identified';
    SELECT id INTO v_stage2_id FROM crm_pipeline_stages WHERE pipeline_id = v_pipeline_id AND name = 'Route Verification';
    SELECT id INTO v_stage3_id FROM crm_pipeline_stages WHERE pipeline_id = v_pipeline_id AND name = 'Tariff Quotation';
    SELECT id INTO v_stage4_id FROM crm_pipeline_stages WHERE pipeline_id = v_pipeline_id AND name = 'Customs Review';
    SELECT id INTO v_stage5_id FROM crm_pipeline_stages WHERE pipeline_id = v_pipeline_id AND name = 'Final Allocation';

    -- ==========================================================
    -- 9. CRM DEALS (17 total: 12 open, 3 won, 2 lost)
    -- ==========================================================

    -- Open — Lead Identified (3)
    INSERT INTO crm_deals (org_id,pipeline_id,stage_id,title,value,currency,contact_id,company_id,status,close_date,owner_id,created_by,created_at,updated_at) VALUES
        (v_org_id,v_pipeline_id,v_stage1_id,'Atlas — Asia Route Expansion',      125000,'USD',v_con1_id, v_cmp_atlas_id,   'open','2026-11-01',v_ananya_id,v_ananya_id,NOW()-INTERVAL '30 days', NOW()-INTERVAL '2 days'),
        (v_org_id,v_pipeline_id,v_stage1_id,'Vanguard — Intermodal Linkup',       62000,'USD',v_con7_id, v_cmp_vanguard_id, 'open','2026-10-15',v_marcus_id,v_marcus_id,NOW()-INTERVAL '25 days', NOW()-INTERVAL '5 days'),
        (v_org_id,v_pipeline_id,v_stage1_id,'Infinity — Changi Distribution',     45000,'USD',v_con11_id,v_cmp_infinity_id, 'open','2026-10-30',v_david_id, v_david_id, NOW()-INTERVAL '15 days', NOW()-INTERVAL '1 day');

    -- Open — Route Verification (3)
    INSERT INTO crm_deals (org_id,pipeline_id,stage_id,title,value,currency,contact_id,company_id,status,close_date,owner_id,created_by,created_at,updated_at) VALUES
        (v_org_id,v_pipeline_id,v_stage2_id,'Beacon Cold Storage Hubbing',       195000,'USD',v_con2_id, v_cmp_beacon_id,  'open','2026-09-25',v_ananya_id,v_ananya_id,NOW()-INTERVAL '45 days', NOW()-INTERVAL '4 days'),
        (v_org_id,v_pipeline_id,v_stage2_id,'CargoLink Rotterdam Charter',       340000,'USD',v_con3_id, v_cmp_cargo_id,   'open','2026-09-10',v_david_id, v_david_id, NOW()-INTERVAL '40 days', NOW()-INTERVAL '3 days'),
        (v_org_id,v_pipeline_id,v_stage2_id,'Horizon Air Transatlantic Freight',  95000,'USD',v_con10_id,v_cmp_horizon_id,  'open','2026-09-01',v_marcus_id,v_marcus_id,NOW()-INTERVAL '35 days', NOW()-INTERVAL '2 days');

    -- Open — Tariff Quotation (3)
    INSERT INTO crm_deals (org_id,pipeline_id,stage_id,title,value,currency,contact_id,company_id,status,close_date,owner_id,created_by,created_at,updated_at) VALUES
        (v_org_id,v_pipeline_id,v_stage3_id,'Matrix Pharma Cool Chain Q4',       280000,'USD',v_con6_id, v_cmp_matrix_id,  'open','2026-08-15',v_ananya_id,v_ananya_id,NOW()-INTERVAL '60 days', NOW()-INTERVAL '6 days'),
        (v_org_id,v_pipeline_id,v_stage3_id,'Infinity Supply Bulk Contract',      85000,'USD',v_con5_id, v_cmp_infinity_id, 'open','2026-08-20',v_david_id, v_david_id, NOW()-INTERVAL '50 days', NOW()-INTERVAL '7 days'),
        (v_org_id,v_pipeline_id,v_stage3_id,'Atlas Maritime Transpacific Fleet', 520000,'USD',v_con9_id, v_cmp_atlas_id,   'open','2026-08-30',v_ananya_id,v_ananya_id,NOW()-INTERVAL '55 days', NOW()-INTERVAL '8 days');

    -- Open — Customs Review (3)
    INSERT INTO crm_deals (org_id,pipeline_id,stage_id,title,value,currency,contact_id,company_id,status,close_date,owner_id,created_by,created_at,updated_at) VALUES
        (v_org_id,v_pipeline_id,v_stage4_id,'Horizon — UK Customs Clearance Plan',140000,'USD',v_con4_id, v_cmp_horizon_id, 'open','2026-07-25',v_marcus_id,v_marcus_id,NOW()-INTERVAL '80 days', NOW()-INTERVAL '10 days'),
        (v_org_id,v_pipeline_id,v_stage4_id,'Zenith Parts JIT Supply Line',      460000,'USD',v_con8_id, v_cmp_zenith_id,       'open','2026-07-20',v_david_id, v_david_id, NOW()-INTERVAL '75 days', NOW()-INTERVAL '9 days'),
        (v_org_id,v_pipeline_id,v_stage4_id,'Matrix GxP Compliance Distribution',175000,'USD',v_con12_id,v_cmp_matrix_id,  'open','2026-07-18',v_ananya_id,v_ananya_id,NOW()-INTERVAL '70 days', NOW()-INTERVAL '5 days');

    -- Open — Final Allocation (2)
    INSERT INTO crm_deals (org_id,pipeline_id,stage_id,title,value,currency,contact_id,company_id,status,close_date,owner_id,created_by,created_at,updated_at) VALUES
        (v_org_id,v_pipeline_id,v_stage5_id,'CargoLink North Sea Master Agreement',680000,'USD',v_con3_id,v_cmp_cargo_id,   'open','2026-06-30',v_david_id, v_david_id, NOW()-INTERVAL '100 days',NOW()-INTERVAL '1 day'),
        (v_org_id,v_pipeline_id,v_stage5_id,'Beacon Midwest Freight Corridor',   230000,'USD',v_con2_id, v_cmp_beacon_id,  'open','2026-07-02',v_ananya_id,v_ananya_id,NOW()-INTERVAL '90 days', NOW()-INTERVAL '2 days');

    -- Won (3)
    INSERT INTO crm_deals (org_id,pipeline_id,stage_id,title,value,currency,contact_id,company_id,status,close_date,won_at,owner_id,created_by,created_at,updated_at) VALUES
        (v_org_id,v_pipeline_id,v_stage5_id,'Matrix Pharma Berlin Logistics Pilot', 45000,'USD',v_con6_id, v_cmp_matrix_id,  'won','2026-03-15',NOW()-INTERVAL '103 days',v_ananya_id,v_ananya_id,NOW()-INTERVAL '115 days',NOW()-INTERVAL '103 days'),
        (v_org_id,v_pipeline_id,v_stage5_id,'Atlas Maritime Core Cross-Docking',   115000,'USD',v_con1_id, v_cmp_atlas_id,   'won','2026-04-10',NOW()-INTERVAL '77 days', v_ananya_id,v_ananya_id,NOW()-INTERVAL '110 days',NOW()-INTERVAL '77 days'),
        (v_org_id,v_pipeline_id,v_stage5_id,'Vanguard Freight Queensland Route',   88000,'USD',v_con7_id, v_cmp_vanguard_id, 'won','2026-05-01',NOW()-INTERVAL '56 days', v_marcus_id,v_marcus_id,NOW()-INTERVAL '95 days', NOW()-INTERVAL '56 days');

    -- Lost (2)
    INSERT INTO crm_deals (org_id,pipeline_id,stage_id,title,value,currency,contact_id,company_id,status,close_date,lost_at,lost_reason,owner_id,created_by,created_at,updated_at) VALUES
        (v_org_id,v_pipeline_id,v_stage4_id,'Zenith Assembly Sub-Contract',       35000,'USD',v_con8_id, v_cmp_zenith_id,       'lost','2026-02-20',NOW()-INTERVAL '126 days','Competitor had pre-existing integration with local rail provider', v_david_id, v_david_id, NOW()-INTERVAL '112 days',NOW()-INTERVAL '126 days'),
        (v_org_id,v_pipeline_id,v_stage3_id,'Horizon Cargo Hangar Setup',         75000,'USD',v_con4_id, v_cmp_horizon_id,  'lost','2026-04-05',NOW()-INTERVAL '82 days', 'Project cancelled by global headquarters due to budget cuts',         v_marcus_id,v_marcus_id,NOW()-INTERVAL '100 days',NOW()-INTERVAL '82 days');

    -- Fetch deal IDs
    SELECT id INTO v_deal1_id  FROM crm_deals WHERE org_id = v_org_id AND title = 'Atlas — Asia Route Expansion';
    SELECT id INTO v_deal2_id  FROM crm_deals WHERE org_id = v_org_id AND title = 'Vanguard — Intermodal Linkup';
    SELECT id INTO v_deal3_id  FROM crm_deals WHERE org_id = v_org_id AND title = 'Infinity — Changi Distribution';
    SELECT id INTO v_deal4_id  FROM crm_deals WHERE org_id = v_org_id AND title = 'Beacon Cold Storage Hubbing';
    SELECT id INTO v_deal5_id  FROM crm_deals WHERE org_id = v_org_id AND title = 'CargoLink Rotterdam Charter';
    SELECT id INTO v_deal6_id  FROM crm_deals WHERE org_id = v_org_id AND title = 'Horizon Air Transatlantic Freight';
    SELECT id INTO v_deal7_id  FROM crm_deals WHERE org_id = v_org_id AND title = 'Matrix Pharma Cool Chain Q4';
    SELECT id INTO v_deal8_id  FROM crm_deals WHERE org_id = v_org_id AND title = 'Infinity Supply Bulk Contract';
    SELECT id INTO v_deal9_id  FROM crm_deals WHERE org_id = v_org_id AND title = 'Atlas Maritime Transpacific Fleet';
    SELECT id INTO v_deal10_id FROM crm_deals WHERE org_id = v_org_id AND title = 'Horizon — UK Customs Clearance Plan';
    SELECT id INTO v_deal11_id FROM crm_deals WHERE org_id = v_org_id AND title = 'CargoLink North Sea Master Agreement';
    SELECT id INTO v_deal12_id FROM crm_deals WHERE org_id = v_org_id AND title = 'Beacon Midwest Freight Corridor';
    
    SELECT id INTO v_won_matrix_id FROM crm_deals WHERE org_id = v_org_id AND title = 'Matrix Pharma Berlin Logistics Pilot';
    SELECT id INTO v_won_atlas_id  FROM crm_deals WHERE org_id = v_org_id AND title = 'Atlas Maritime Core Cross-Docking';

    -- ==========================================================
    -- 10. CRM LEADS (15 Completely New Leads)
    -- ==========================================================

    -- New (4)
    INSERT INTO crm_leads (org_id,first_name,last_name,email,phone,company_name,title,source,status,owner_id,created_by,created_at,updated_at) VALUES
        (v_org_id,'Christian','Lindner', 'c.lindner@rhinefreight.de',  '+49-221-555-901','RhineFreight GmbH', 'Managing Partner',     'web',      'new',v_ananya_id,v_ananya_id,NOW()-INTERVAL '10 days',NOW()-INTERVAL '10 days'),
        (v_org_id,'Saskia',   'Van-Dam', 's.vandam@nordicship.no',     '+47-22-555-0192', 'Nordic Shipping',  'Chief Operations Exec','social',   'new',v_david_id, v_david_id, NOW()-INTERVAL '7 days', NOW()-INTERVAL '7 days'),
        (v_org_id,'Kenji',    'Tanaka',  'k.tanaka@tokyolog.jp',       '+81-3-5555-7643', 'Tokyo Logistical', 'Global Routing Planner','web',      'new',v_ananya_id,v_ananya_id,NOW()-INTERVAL '4 days', NOW()-INTERVAL '4 days'),
        (v_org_id,'Ahmed',    'Mansoor', 'a.mansoor@gulfcharter.ae',   '+971-4-555-9831', 'Gulf Charter Hub', 'VP Supply Chain',      'referral', 'new',v_marcus_id,v_marcus_id,NOW()-INTERVAL '2 days', NOW()-INTERVAL '2 days');

    -- Contacted (4)
    INSERT INTO crm_leads (org_id,first_name,last_name,email,phone,company_name,title,source,status,owner_id,created_by,created_at,updated_at) VALUES
        (v_org_id,'Fiona',   'Gallagher','f.gallagher@dublinport.ie',  '+353-1-555-8821', 'Dublin Port Storage','Harbour Master',     'cold_call','contacted',v_david_id, v_david_id, NOW()-INTERVAL '24 days',NOW()-INTERVAL '15 days'),
        (v_org_id,'Matteo',  'Rossi',    'm.rossi@milanocargo.it',     '+39-02-555-1209', 'Milano Cargo SRL', 'Customs Director',     'event',    'contacted',v_marcus_id,v_marcus_id,NOW()-INTERVAL '20 days',NOW()-INTERVAL '12 days'),
        (v_org_id,'Zoe',     'Kravitz',  'z.kravitz@austinwholesalers.com','+1-512-555-3211','Austin Wholesalers','Distribution Manager','partner', 'contacted',v_ananya_id,v_ananya_id,NOW()-INTERVAL '16 days',NOW()-INTERVAL '10 days'),
        (v_org_id,'Oliver',  'Twist',    'o.twist@londonwharf.co.uk',  '+44-20-7946-8812','London Wharfages', 'Logistics Analyst',    'email',    'contacted',v_david_id, v_david_id, NOW()-INTERVAL '12 days',NOW()-INTERVAL '9 days');

    -- Qualified (3)
    INSERT INTO crm_leads (org_id,first_name,last_name,email,phone,company_name,title,source,status,owner_id,created_by,created_at,updated_at) VALUES
        (v_org_id,'Santiago','Gomez',    's.gomez@madridrail.es',      '+34-91-555-7721', 'Madrid Rail Express','Head of Infrastructure','referral','qualified',v_marcus_id,v_marcus_id,NOW()-INTERVAL '48 days',NOW()-INTERVAL '6 days'),
        (v_org_id,'Yasmine', 'El-Amin',  'y.elamin@cairologistics.eg', '+20-2-555-4311',  'Cairo Logistics',  'Regional Director',    'event',    'qualified',v_ananya_id,v_ananya_id,NOW()-INTERVAL '42 days',NOW()-INTERVAL '5 days'),
        (v_org_id,'Ravi',    'Shankar',  'r.shankar@mumbaicustoms.in', '+91-22-5555-9012', 'Mumbai Intermodal','Director of Operations','web',     'qualified',v_david_id, v_david_id, NOW()-INTERVAL '36 days',NOW()-INTERVAL '4 days');

    -- Unqualified (2)
    INSERT INTO crm_leads (org_id,first_name,last_name,email,phone,company_name,title,source,status,owner_id,created_by,created_at,updated_at) VALUES
        (v_org_id,'Peter',   'Parker',   'p.parker@scooterfreight.com','+1-718-555-0912', 'Scooter Delivery Co','Owner',               'web',   'unqualified',v_ananya_id,v_ananya_id,NOW()-INTERVAL '55 days',NOW()-INTERVAL '50 days'),
        (v_org_id,'Emily',   'Watson',   'e.watson@collegemove.org',   '+1-617-555-1143', 'Dorm Mover System', 'Student Organiser',    'email', 'unqualified',v_david_id, v_david_id, NOW()-INTERVAL '40 days',NOW()-INTERVAL '38 days');

    -- Converted (2)
    INSERT INTO crm_leads (org_id,first_name,last_name,email,phone,company_name,title,source,status,
        converted_at,converted_contact_id,converted_deal_id,owner_id,created_by,created_at,updated_at) VALUES
        (v_org_id,'Dr. Stefan','Müller', 's.mueller@matrixpharma.de', '+49-30-8891-991', 'Matrix Pharma Tech','Cold Chain Quality Lead','web',    'converted',
         NOW()-INTERVAL '103 days',v_con6_id,v_won_matrix_id,v_ananya_id,v_ananya_id,NOW()-INTERVAL '115 days',NOW()-INTERVAL '103 days'),
        (v_org_id,'Captain Erik','Nielsen','e.nielsen@atlasmaritime.com','+1-206-555-0911','Atlas Maritime', 'Fleet Operations Director','partner','converted',
         NOW()-INTERVAL '77 days',v_con1_id,v_won_atlas_id, v_ananya_id,v_ananya_id,NOW()-INTERVAL '110 days',NOW()-INTERVAL '77 days');

    -- ==========================================================
    -- 11. GENERAL TASKS (15)
    -- ==========================================================
    INSERT INTO tasks (org_id, title, description, status, due_date, created_by, assigned_to, created_at, updated_at)
    VALUES
        -- todo (5)
        (v_org_id,'Review IMO 2026 Regulations', 'Analyze the new International Maritime Organization limits on fuel compliance for upcoming quotes', 'todo', NOW()+INTERVAL '6 days',  v_marcus_id,v_marcus_id,NOW()-INTERVAL '1 day',   NOW()-INTERVAL '1 day'),
        (v_org_id,'Rotterdam Port Audit Prep',   'Collate compliance data certificates for terminal operation visibility testing',         'todo', NOW()+INTERVAL '12 days', v_elena_id, v_david_id, NOW()-INTERVAL '2 days',  NOW()-INTERVAL '2 days'),
        (v_org_id,'Carrier Insurance Check',     'Verify global carrier insurance validity certificates for the active sub-vendors network','todo', NOW()+INTERVAL '8 days',  v_elena_id, v_elena_id, NOW()-INTERVAL '1 day',   NOW()-INTERVAL '1 day'),
        (v_org_id,'Q4 Tariff Matrix Planning',   'Draft updated maritime spot tariff matrix boundaries based on current shipping lanes fuel price','todo', NOW()+INTERVAL '4 days',  v_marcus_id,v_david_id, NOW()-INTERVAL '3 days',  NOW()-INTERVAL '3 days'),
        (v_org_id,'SaaS Terminal Mapping Sync',  'Coordinate dashboard mapping rules for API integration with terminal operating structures', 'todo', NOW()+INTERVAL '18 days', v_tariq_id, v_yuki_id,  NOW()-INTERVAL '4 days',  NOW()-INTERVAL '4 days'),
        -- in_progress (5)
        (v_org_id,'Enterprise Freight RFP Draft','Draft tailored complex bulk response layout framework for top-tier supply tenders',        'in_progress',NOW()+INTERVAL '2 days',  v_marcus_id,v_ananya_id,NOW()-INTERVAL '8 days',  NOW()-INTERVAL '12 hours'),
        (v_org_id,'Global KPIs Spreadsheet Update','Analyze lane transit times metrics vs targets for the June regional director brief',    'in_progress',NOW()+INTERVAL '5 days',  v_marcus_id,v_marcus_id,NOW()-INTERVAL '6 days',  NOW()-INTERVAL '1 hour'),
        (v_org_id,'Customs Broker API Check',    'Fix missing tracking event webhook failures with EuroCustoms digital platform',         'in_progress',NOW()+INTERVAL '20 days', v_elena_id, v_elena_id, NOW()-INTERVAL '30 days', NOW()-INTERVAL '2 days'),
        (v_org_id,'Freight Hub Marketing Update','Review collateral presentation for London logistics tradeshow panel sessions',          'in_progress',NOW()+INTERVAL '10 days', v_tariq_id, v_yuki_id,  NOW()-INTERVAL '10 days', NOW()-INTERVAL '1 day'),
        (v_org_id,'EDI Integration Pipeline Fix','Address database mapping mismatches on incoming ASN manifests tracking payload data',     'in_progress',NOW()+INTERVAL '5 days',  v_elena_id, v_ananya_id,NOW()-INTERVAL '9 days',  NOW()-INTERVAL '4 hours'),
        -- done (5)
        (v_org_id,'H1 Operational Planning Presentation','Delivered operational scale strategy framework report to executive board members', 'done', NOW()-INTERVAL '60 days', v_tariq_id, v_tariq_id, NOW()-INTERVAL '80 days', NOW()-INTERVAL '61 days'),
        (v_org_id,'Global Sales Summit 2026',    'Concluded EMEA/APAC strategic planning sessions for freight forwarding account reps',    'done', NOW()-INTERVAL '120 days',v_marcus_id,v_elena_id, NOW()-INTERVAL '140 days',NOW()-INTERVAL '121 days'),
        (v_org_id,'Vertex SaaS Platform Launch', 'Successfully activated the main tenant framework and multi-currency billing pipeline',  'done', NOW()-INTERVAL '100 days',v_tariq_id, v_marcus_id,NOW()-INTERVAL '120 days',NOW()-INTERVAL '100 days'),
        (v_org_id,'Carrier Onboarding Manual',   'Completed comprehensive procedures guide booklet for network shipping providers',       'done', NOW()-INTERVAL '70 days', v_marcus_id,v_marcus_id,NOW()-INTERVAL '85 days', NOW()-INTERVAL '70 days'),
        (v_org_id,'Fuel Surcharge System Launch','Deployed unified pricing mechanism calculating fluctuating bunker adjustment factor rules','done', NOW()-INTERVAL '45 days', v_elena_id, v_elena_id, NOW()-INTERVAL '50 days', NOW()-INTERVAL '45 days');

    -- ==========================================================
    -- 12. PLATFORM TASKS (10 — linked to deals / contacts)
    -- ==========================================================
    INSERT INTO platform_tasks (
        org_id, module, title, description, due_date, status, priority,
        related_type, related_id, assigned_to, created_by, created_at, updated_at
    ) VALUES
        (v_org_id,'crm','Verify Bill of Lading with Atlas',   'Review container payload description layout mapping data verification clauses', NOW()+INTERVAL '3 days', 'open',     'high',  'crm.deal',         v_deal9_id, v_ananya_id,v_ananya_id,NOW()-INTERVAL '4 days', NOW()-INTERVAL '4 days'),
        (v_org_id,'crm','Verify Cold Chain layout profiles',  'Request exact temperature data logger specification blueprints from Dr. Stefan',NOW()+INTERVAL '2 days', 'open',     'high',  'crm.deal',         v_deal7_id, v_ananya_id,v_ananya_id,NOW()-INTERVAL '5 days', NOW()-INTERVAL '1 day'),
        (v_org_id,'crm','Send quote adjustment to CargoLink', 'Incorporate the new Euro-hub port tax exceptions inside the Master Charter doc',NOW()+INTERVAL '4 days', 'open',     'medium','crm.deal',         v_deal11_id,v_david_id, v_david_id, NOW()-INTERVAL '2 days', NOW()-INTERVAL '2 days'),
        (v_org_id,'crm','Review Heathrow slot allocations',  'Verify transit schedules compliance metrics ahead of airline contract review',  NOW()+INTERVAL '6 days', 'open',     'medium','crm.deal',         v_deal6_id, v_marcus_id,v_marcus_id,NOW()-INTERVAL '3 days', NOW()-INTERVAL '3 days'),
        (v_org_id,'crm','Calculate customs matrix boundaries','Send tailored tariff structure breakdown for Zenith Parts imports',            NOW()-INTERVAL '1 day',  'open',     'high',  'crm.deal',         v_deal10_id,v_david_id, v_david_id, NOW()-INTERVAL '3 days', NOW()-INTERVAL '1 day'),
        (v_org_id,'crm','Call Martha Stewart re storage expansion','Confirm frozen pallet allocations capacity guarantees timeline',           NOW()-INTERVAL '2 days', 'open',     'medium','platform.contact',  v_con2_id,  v_ananya_id,v_ananya_id,NOW()-INTERVAL '5 days', NOW()-INTERVAL '5 days'),
        (v_org_id,'crm','Collect intermodal certificates',   'Request updated rail hazardous cargo authorization proofs from Bruce Wayne',   NOW()+INTERVAL '5 days', 'open',     'low',   'platform.contact',  v_con7_id,  v_marcus_id,v_marcus_id,NOW()-INTERVAL '6 days', NOW()-INTERVAL '6 days'),
        (v_org_id,'crm','Log Rotterdam terminal minutes',     'Update pricing comments feedback following Vermeer terminals phone call sessions',NOW()-INTERVAL '3 days', 'completed','medium','crm.deal',         v_deal5_id, v_david_id, v_david_id, NOW()-INTERVAL '7 days', NOW()-INTERVAL '3 days'),
        (v_org_id,'crm','Send GxP validation blueprints',     'Confirm secure handling checklist details match regulatory pharmaceutical rules',NOW()-INTERVAL '4 days', 'completed','high',  'crm.deal',         v_deal7_id, v_ananya_id,v_ananya_id,NOW()-INTERVAL '10 days',NOW()-INTERVAL '4 days'),
        (v_org_id,'crm','Welcome intro call — Captain Erik',  'Kickoff meeting logistics regarding current ongoing active routes management',   NOW()-INTERVAL '8 days', 'completed','medium','platform.contact',  v_con1_id,  v_ananya_id,v_ananya_id,NOW()-INTERVAL '12 days',NOW()-INTERVAL '8 days');

    -- ==========================================================
    -- 13. PLATFORM NOTES (10)
    -- ==========================================================
    INSERT INTO platform_notes (org_id, module, content, related_type, related_id, created_by, created_at, updated_at)
    VALUES
        (v_org_id,'crm',
         'Initial consultation call with Captain Erik finished. Client wants to secure slot availability routes ahead of peak winter congestion patterns. High intent profile. Moving this contract layout proposal forward quickly.',
         'crm.deal',v_deal9_id,v_ananya_id,NOW()-INTERVAL '22 days',NOW()-INTERVAL '22 days'),

        (v_org_id,'crm',
         'Martha Stewart flagged capacity constraints inside their Minneapolis warehouse structure. Our hubbing SaaS product allows tracking virtual allocations to offload overflow lanes. Demo set up for next week.',
         'crm.deal',v_deal4_id,v_ananya_id,NOW()-INTERVAL '25 days',NOW()-INTERVAL '25 days'),

        (v_org_id,'crm',
         'Hans Vermeer discussed multi-year terminal leasing framework clauses. Rotterdam volume throughput targets are highly dependent on global shipping alliances stability patterns. Keep internal operations team in loop.',
         'crm.deal',v_deal11_id,v_david_id, v_david_id, NOW()-INTERVAL '18 days',NOW()-INTERVAL '18 days'),

        (v_org_id,'crm',
         'Sir Alan was direct regarding tariff margins limits. Horizon Aviation expects fixed pricing security on core transatlantic lanes for 18 months minimum. Legal tracking required on fuel surcharge threshold variables.',
         'crm.deal',v_deal6_id,v_marcus_id,NOW()-INTERVAL '15 days',NOW()-INTERVAL '15 days'),

        (v_org_id,'crm',
         'Zenith Parts deal requires deep API mapping alignment validation. Sarah Connor mentioned their inventory setup updates automatically every 15 mins via obsolete legacy structures. Custom middleware engine required.',
         'crm.deal',v_deal10_id,v_david_id, v_david_id, NOW()-INTERVAL '12 days',NOW()-INTERVAL '12 days'),

        (v_org_id,'crm',
         'Dr. Stefan Müller signed off on the Berlin logistics framework review summary report. Validation tests passed compliance standards. This provides strong enterprise leverage credibility for larger upcoming pharma deals.',
         'crm.deal',v_deal7_id,v_ananya_id,NOW()-INTERVAL '8 days', NOW()-INTERVAL '8 days'),

        (v_org_id,'crm',
         'Captain Erik Nielsen is a traditional marine operator. Prefers absolute structured detail specifications over fast general feature summaries. High relationship equity holder.',
         'platform.contact',v_con1_id,v_ananya_id,NOW()-INTERVAL '85 days', NOW()-INTERVAL '85 days'),

        (v_org_id,'crm',
         'Lin Wei is our primary champion inside the Singapore procurement ecosystem. Highly technical on warehouse metric layout analytics tracking requirements.',
         'platform.contact',v_con5_id,v_david_id, v_david_id, NOW()-INTERVAL '60 days', NOW()-INTERVAL '60 days'),

        (v_org_id,'crm',
         'Bruce Wayne handles heavy freight routing scheduling. Emphasized rail siding availability rules are the key bottleneck constraints across the Australian network lines.',
         'platform.contact',v_con7_id,v_marcus_id,NOW()-INTERVAL '45 days', NOW()-INTERVAL '45 days'),

        (v_org_id,'crm',
         'Klaus Fischer highlighted strict GxP handling guidelines compliance is non-negotiable. Cold chain asset failures mean automatic termination clauses inside master contracts.',
         'platform.contact',v_con12_id,v_ananya_id,NOW()-INTERVAL '10 days', NOW()-INTERVAL '10 days');

    -- ==========================================================
    -- 14. PLATFORM ACTIVITIES (12)
    -- ==========================================================
    INSERT INTO platform_activities (
        org_id, module, type, subject, description, outcome,
        related_type, related_id, occurred_at, duration_mins,
        created_by, created_at, updated_at
    ) VALUES
        (v_org_id,'crm','call',   'Discovery — Atlas Route Options',
         'Explored volume metrics requirements patterns for cross-docking lane configurations inside Pacific Northwest terminals.',
         'Positive — client requested formal tariff rate breakdown layout framework',
         'crm.deal',v_deal9_id,NOW()-INTERVAL '20 days',35,v_ananya_id,NOW()-INTERVAL '20 days',NOW()-INTERVAL '20 days'),

        (v_org_id,'crm','email',  'Tariff Quotation Framework Data Pack',
         'Sent updated matrix parameters structures to Dr. Stefan Müller detailing cold chain logistics failsafe validation safeguards.',
         'Email confirmed opened — validation tests mapping initiated',
         'crm.deal',v_deal7_id,NOW()-INTERVAL '15 days',NULL,v_ananya_id,NOW()-INTERVAL '15 days',NOW()-INTERVAL '15 days'),

        (v_org_id,'crm','meeting','Rotterdam Operations Sync Session',
         'On-site review session at port office facility. Addressed pipeline payload mapping rules and integration boundaries configuration details.',
         'Agreed on timeline constraints; moving to next assessment phase',
         'crm.deal',v_deal11_id,NOW()-INTERVAL '12 days',110,v_david_id, v_david_id, NOW()-INTERVAL '12 days',NOW()-INTERVAL '12 days'),

        (v_org_id,'crm','call',   'Customs Compliance Scope Review',
         'Detailed walkthrough session with Sir Alan Cross reviewing import declaration bottlenecks on international flight legs.',
         'Horizon legal team reviewing custom tariff exception clauses documents',
         'crm.deal',v_deal6_id,NOW()-INTERVAL '10 days',45,v_marcus_id,NOW()-INTERVAL '10 days',NOW()-INTERVAL '10 days'),

        (v_org_id,'crm','meeting','Detroit JIT Supply Technical Review',
         'Met with Sarah Connor and engineering group to plan middleware setup specifications mapping inventory APIs into database structures.',
         'Architecture layout blueprint draft confirmed approved in principle',
         'crm.deal',v_deal10_id,NOW()-INTERVAL '8 days',90,v_david_id, v_david_id, NOW()-INTERVAL '8 days',NOW()-INTERVAL '8 days'),

        (v_org_id,'crm','call',   'Bulk Allocation Pricing Final Review',
         'Final sync meeting with Lin Wei defining annual freight spending visibility targets across Changi terminal corridors.',
         'Contract final draft requested matching the volume price adjustments',
         'crm.deal',v_deal8_id,NOW()-INTERVAL '5 days',40,v_marcus_id,NOW()-INTERVAL '5 days',NOW()-INTERVAL '5 days'),

        (v_org_id,'crm','email',  'Proposal Presentation — Hubbing Framework',
         'Dispatched custom virtual storage routing framework parameters summary deck to Martha Stewart.',
         'Awaiting operational capacity response feedback profiles',
         'crm.deal',v_deal4_id,NOW()-INTERVAL '14 days',NULL,v_ananya_id,NOW()-INTERVAL '14 days',NOW()-INTERVAL '14 days'),

        (v_org_id,'crm','call',   'Qualification Call — CargoLink Route Prep',
         'Assessed terminal capability profiles with Hans Vermeer to verify multi-modal cargo payload weight constraint profiles compliance.',
         'Lane capabilities verified confirmed compliant; shifting to quote preparation',
         'crm.deal',v_deal5_id,NOW()-INTERVAL '25 days',30,v_david_id, v_david_id, NOW()-INTERVAL '25 days',NOW()-INTERVAL '25 days'),

        (v_org_id,'crm','meeting','Sydney Port Yards Evaluation',
         'Comprehensive facilities mapping walkthrough review defining freight optimization targets for local distribution lines network.',
         'Strong performance score rating achieved; customized presentation requested',
         'crm.deal',v_deal2_id,NOW()-INTERVAL '18 days',240,v_marcus_id,NOW()-INTERVAL '18 days',NOW()-INTERVAL '18 days'),

        (v_org_id,'crm','call',   'Quarterly Relationship Sync — Nielsen',
         'Client relationship management tracking session. Expressed massive satisfaction regarding current ongoing container throughput analytics values tracking dashboards.',
         'Maintained high relationship health equity status score',
         'platform.contact',v_con1_id,NOW()-INTERVAL '32 days',25,v_ananya_id,NOW()-INTERVAL '32 days',NOW()-INTERVAL '32 days'),

        (v_org_id,'crm','email',  'Outbound Outreach — Hans Vermeer Intro',
         'Introduced our global software visibility suite capability profiles to Hans via referral connection pipelines.',
         'Response received within same working day; discovery call confirmed booked',
         'platform.contact',v_con3_id,NOW()-INTERVAL '50 days',NULL,v_david_id, v_david_id, NOW()-INTERVAL '50 days',NOW()-INTERVAL '50 days'),

        (v_org_id,'crm','meeting','Project Alignment Workshop — Cold Storage',
         'Kickoff meeting finalizing hardware tracking beacons installation timelines inside client fleet trailers.',
         'Deployment parameters confirmed finalized; installation starting next group phase',
         'crm.deal',v_deal4_id,NOW()-INTERVAL '8 days',75,v_ananya_id,NOW()-INTERVAL '8 days',NOW()-INTERVAL '8 days');

    -- ==========================================================
    -- 15. PLATFORM EMAIL LOGS (8)
    -- ==========================================================
    INSERT INTO platform_email_logs (
        org_id, module, subject, body, from_email, to_email,
        direction, status, related_type, related_id, sent_at, created_by, created_at
    ) VALUES
        (v_org_id,'crm',
         'Global Maritime Route Quote Framework — Atlas',
         'Hi Erik, as outlined inside our planning sync session, here is the updated tariff calculation breakdown sheet for your transpacific container allocations. This pricing guarantees priority lane processing metrics.',
         'ananya@vertexlogistics.io','e.nielsen@atlasmaritime.com',
         'outbound','sent','crm.deal',v_deal9_id,NOW()-INTERVAL '18 days',v_ananya_id,NOW()-INTERVAL '18 days'),

        (v_org_id,'crm',
         'Cold Chain Validation Protocol Blueprint Specs',
         'Hi Stefan, please find attached our comprehensive compliance mapping rules layout defining security standard operations across regional biotech storage infrastructure checkpoints.',
         'ananya@vertexlogistics.io','s.mueller@matrixpharma.de',
         'outbound','sent','crm.deal',v_deal7_id,NOW()-INTERVAL '14 days',v_ananya_id,NOW()-INTERVAL '14 days'),

        (v_org_id,'crm',
         'Terminal Charter Framework Documentation Package',
         'Hi Hans, wonderful speaking today. Attached is our formal contract proposal layout structure defining operational volume price boundaries for Rotterdam terminal facilities management routing structures.',
         'david@vertexlogistics.io','h.vermeer@cargolink.nl',
         'outbound','sent','crm.deal',v_deal11_id,NOW()-INTERVAL '10 days',v_david_id, v_david_id, NOW()-INTERVAL '10 days'),

        (v_org_id,'crm',
         'Customs Integration API Spec Documentation Review',
         'Hi Sarah, following our architecture overview workshop sessions, here are the secure endpoint schemas mapping real-time inventory adjustments directly into the container distribution platforms.',
         'david@vertexlogistics.io','s.connor@zenithparts.com',
         'outbound','sent','crm.deal',v_deal10_id,NOW()-INTERVAL '6 days', v_david_id, v_david_id, NOW()-INTERVAL '6 days'),

        (v_org_id,'crm',
         'Revised Freight Surcharge Threshold Language Update',
         'Hi Alan, we modified the contract clause terminology to mirror standard market variables indicators accurately as requested. Please see revised framework version documentation attached.',
         'marcus@vertexlogistics.io','a.cross@horizonair.com',
         'outbound','sent','crm.deal',v_deal6_id,NOW()-INTERVAL '12 days',v_marcus_id,NOW()-INTERVAL '12 days'),

        (v_org_id,'crm',
         'Re: Surcharge Terms Agreement Clarifications Needed',
         'Hi Marcus, thanks for resolving the language changes so quickly. Does this specific index recalculation step trigger at the beginning of each calendar quarter? Let us run a quick team review session.',
         'a.cross@horizonair.com','marcus@vertexlogistics.io',
         'inbound','received','crm.deal',v_deal6_id,NOW()-INTERVAL '11 days',v_marcus_id,NOW()-INTERVAL '11 days'),

        (v_org_id,'crm',
         'Virtual Hub Allocation Framework Draft v1.2',
         'Hi Martha, please find attached the initial master operations draft configuration detailing virtual space provisioning controls for the overflow depot system lines.',
         'ananya@vertexlogistics.io','m.stewart@beaconcold.com',
         'outbound','sent','crm.deal',v_deal4_id,NOW()-INTERVAL '5 days', v_ananya_id,v_ananya_id,NOW()-INTERVAL '5 days'),

        (v_org_id,'crm',
         'Introduction — Vertex Logistics Visibility Software',
         'Hi Hans, reaching out on a direct introduction recommendation from Captain Erik at Atlas. We recently streamlined their freight scheduling operations tracking layouts. Let us schedule 20 minutes.',
         'david@vertexlogistics.io','h.vermeer@cargolink.nl',
         'outbound','sent','platform.contact',v_con3_id,NOW()-INTERVAL '49 days',v_david_id, v_david_id, NOW()-INTERVAL '49 days');

    -- ==========================================================
    -- 16. SESSIONS (3)
    -- ==========================================================
    INSERT INTO sessions (
        user_id, org_id, token_hash, device_name, device_type, browser, os,
        user_agent, ip_address, country, city,
        last_activity_at, created_at, expires_at
    ) VALUES
        (v_tariq_id, v_org_id,
         encode(digest('vertex-seed-sess-tariq-workstation-2026',  'sha256'), 'hex'),
         'Lenovo ThinkPad X1', 'desktop','Chrome','Ubuntu 24.04 LTS',
         'Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/122.0.0.0 Safari/537.36',
         '84.22.103.45','GB','London',
         NOW()-INTERVAL '1 hour',  NOW()-INTERVAL '1 hour',  NOW()+INTERVAL '7 days'),

        (v_tariq_id, v_org_id,
         encode(digest('vertex-seed-sess-tariq-ipad-2026',         'sha256'), 'hex'),
         'iPad Pro 11"',       'tablet', 'Safari','iPadOS 17',
         'Mozilla/5.0 (iPad; CPU OS 17_3 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/17.3 Mobile/15E148 Safari/604.1',
         '84.22.103.46','GB','London',
         NOW()-INTERVAL '2 days',  NOW()-INTERVAL '4 days',   NOW()+INTERVAL '3 days'),

        (v_ananya_id, v_org_id,
         encode(digest('vertex-seed-sess-ananya-macbook-2026',     'sha256'), 'hex'),
         'MacBook Air 13"',    'desktop','Safari','macOS 14 Sonoma',
         'Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/17.2 Safari/605.1.15',
         '115.110.201.88','IN','Mumbai',
         NOW()-INTERVAL '3 hours', NOW()-INTERVAL '3 hours',  NOW()+INTERVAL '7 days')
    ON CONFLICT DO NOTHING;

    -- ==========================================================
    -- 17. LOGIN EVENTS (10)
    -- ==========================================================
    INSERT INTO login_events (user_id, email, provider, status, failure_reason, ip_address, user_agent, country, city, created_at)
    VALUES
        (v_tariq_id, 'tariq@vertexlogistics.io', 'credentials','success',NULL,           '84.22.103.45',  'Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36',                           'GB','London',      NOW()-INTERVAL '1 hour'),
        (v_tariq_id, 'tariq@vertexlogistics.io', 'credentials','success',NULL,           '84.22.103.45',  'Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36',                           'GB','London',      NOW()-INTERVAL '2 days'),
        (v_marcus_id,'marcus@vertexlogistics.io','credentials','success',NULL,           '64.233.160.12', 'Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36',                  'US','New York',    NOW()-INTERVAL '4 hours'),
        (v_ananya_id,'ananya@vertexlogistics.io','credentials','success',NULL,           '115.110.201.88','Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/605.1.15',          'IN','Mumbai',      NOW()-INTERVAL '3 hours'),
        (v_david_id, 'david@vertexlogistics.io', 'credentials','success',NULL,           '204.14.233.91', 'Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36',                  'US','San Francisco',NOW()-INTERVAL '3 days'),
        (v_elena_id, 'elena@vertexlogistics.io', 'credentials','success',NULL,           '193.51.201.14', 'Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/605.1.15',          'FR','Paris',       NOW()-INTERVAL '1 day'),
        (v_yuki_id,  'yuki@vertexlogistics.io',  'credentials','success',NULL,           '210.143.44.12', 'Mozilla/5.0 (iPhone; CPU iPhone OS 17_2 like Mac OS X) AppleWebKit/605.1.15',   'JP','Tokyo',       NOW()-INTERVAL '5 days'),
        (NULL,       'tariq@vertexlogistics.io', 'credentials','failure','invalid_credentials','185.220.101.5','python-requests/2.31.0','DE','Frankfurt', NOW()-INTERVAL '4 days'),
        (NULL,       'root@vertexlogistics.io',  'credentials','failure','user_not_found',     '185.220.101.6','python-requests/2.31.0','DE','Frankfurt', NOW()-INTERVAL '4 days'),
        (NULL,       'tariq@vertexlogistics.io', 'credentials','failure','invalid_credentials','93.174.93.12', 'curl/8.4.0',            'NL','Amsterdam', NOW()-INTERVAL '6 days');

    -- ==========================================================
    -- 18. PENDING INVITATIONS (2)
    -- ==========================================================
    INSERT INTO organization_invitations (
        org_id, email, role_id, role_key, title, department,
        token_hash, status, invited_by, expires_at, last_sent_at, created_at, updated_at
    ) VALUES
        (v_org_id, 'simon.peg@vertexlogistics.io',
         v_role_member_id, 'member', 'Customs Operations Agent', 'Operations',
         encode(digest('vertex-seed-invite-simon-peg-2026', 'sha256'), 'hex'),
         'pending', v_elena_id,
         NOW()+INTERVAL '5 days', NOW()-INTERVAL '1 day', NOW()-INTERVAL '1 day', NOW()-INTERVAL '1 day'),

        (v_org_id, 'mei.ling@vertexlogistics.io',
         v_role_member_id, 'member', 'APAC Logistics Coordinator', 'Sales',
         encode(digest('vertex-seed-invite-mei-ling-2026', 'sha256'), 'hex'),
         'pending', v_marcus_id,
         NOW()+INTERVAL '6 days', NOW()-INTERVAL '2 days', NOW()-INTERVAL '2 days', NOW()-INTERVAL '2 days')
    ON CONFLICT DO NOTHING;

    -- ==========================================================
    -- 19. AUDIT LOGS (8)
    -- ==========================================================
    INSERT INTO audit_logs (org_id, user_id, event_type, description, resource_type, resource_id, status, ip_address, created_at)
    VALUES
        (v_org_id,v_tariq_id, 'organization.created',    'Organization ''Vertex Logistics'' tenant structure established', 'organization', v_org_id::TEXT,        'success','84.22.103.45',  NOW()-INTERVAL '120 days'),
        (v_org_id,v_tariq_id, 'auth.sign_in',             'Managing Director signed in from London desktop terminal',       'user',         v_tariq_id::TEXT,    'success','84.22.103.45',  NOW()-INTERVAL '1 hour'),
        (v_org_id,v_elena_id, 'member.invited',           'Invited simon.peg@vertexlogistics.io as operations member',      'organization', v_org_id::TEXT,        'success','193.51.201.14', NOW()-INTERVAL '1 day'),
        (v_org_id,v_marcus_id,'member.invited',           'Invited mei.ling@vertexlogistics.io as tracking sales associate','organization', v_org_id::TEXT,        'success','64.233.160.12', NOW()-INTERVAL '2 days'),
        (v_org_id,v_tariq_id, 'role.permissions.updated', 'Adjusted tariff visibility write bounds for system role ''manager''','role',         v_role_manager_id::TEXT,'success','84.22.103.45',  NOW()-INTERVAL '95 days'),
        (v_org_id,v_elena_id, 'member.role.changed',      'Promoted David Beck from junior trainee track to logistics AE',  'organization', v_org_id::TEXT,        'success','193.51.201.14', NOW()-INTERVAL '79 days'),
        (v_org_id,v_ananya_id,'crm.lead.converted',       'Lead Stefan Müller converted into valid account + active deal Pipeline','crm_lead', v_con6_id::TEXT,        'success','115.110.201.88',NOW()-INTERVAL '103 days'),
        (v_org_id,v_ananya_id,'crm.lead.converted',       'Lead Erik Nielsen converted into core maritime account profile','crm_lead',     v_con1_id::TEXT,        'success','115.110.201.88',NOW()-INTERVAL '77 days');

END $$;
-- +goose StatementEnd


-- +goose Down
-- +goose StatementBegin
-- ============================================================
-- Removes all seed data for the 'vertex-logistics' organization.
-- ============================================================

DO $$
DECLARE
    v_org_id UUID;
BEGIN
    SELECT id INTO v_org_id FROM organizations WHERE LOWER(slug) = 'vertex-logistics';
    IF v_org_id IS NULL THEN
        RAISE NOTICE 'Seed org ''vertex-logistics'' not found — nothing to remove.';
        RETURN;
    END IF;

    -- Cleanup platform interactions
    DELETE FROM platform_email_logs WHERE org_id = v_org_id;
    DELETE FROM platform_activities  WHERE org_id = v_org_id;
    DELETE FROM platform_notes       WHERE org_id = v_org_id;
    DELETE FROM platform_tasks       WHERE org_id = v_org_id;

    -- Cleanup CRM architecture
    DELETE FROM crm_deals            WHERE org_id = v_org_id;
    DELETE FROM crm_leads            WHERE org_id = v_org_id;
    DELETE FROM crm_pipeline_stages  WHERE org_id = v_org_id;
    DELETE FROM crm_pipelines        WHERE org_id = v_org_id;

    -- Cleanup base master data entities
    DELETE FROM platform_contacts    WHERE org_id = v_org_id;
    DELETE FROM platform_companies   WHERE org_id = v_org_id;

    -- Cleanup internal engine metrics tasks
    DELETE FROM tasks                WHERE org_id = v_org_id;

    -- Cleanup infrastructure bounds configurations
    DELETE FROM sessions               WHERE org_id = v_org_id;
    DELETE FROM audit_logs             WHERE org_id = v_org_id;
    DELETE FROM organization_invitations WHERE org_id = v_org_id;
    DELETE FROM organization_members   WHERE org_id = v_org_id;
    DELETE FROM subscriptions          WHERE org_id = v_org_id;

    -- Remove top-level organization framework
    DELETE FROM organizations          WHERE id = v_org_id;

    -- Remove targeted audit logs traces matched specifically by known emails
    DELETE FROM login_events WHERE email IN (
        'tariq@vertexlogistics.io', 'elena@vertexlogistics.io',
        'marcus@vertexlogistics.io','ananya@vertexlogistics.io',
        'david@vertexlogistics.io', 'yuki@vertexlogistics.io',
        'root@vertexlogistics.io'
    );

    -- Remove core users instances last
    DELETE FROM users WHERE LOWER(email) IN (
        'tariq@vertexlogistics.io', 'elena@vertexlogistics.io',
        'marcus@vertexlogistics.io','ananya@vertexlogistics.io',
        'david@vertexlogistics.io', 'yuki@vertexlogistics.io'
    );

    RAISE NOTICE 'Seed data for ''vertex-logistics'' removed successfully.';
END $$;
-- +goose StatementEnd