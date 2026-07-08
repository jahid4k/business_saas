-- +goose Up
-- +goose StatementBegin

-- ============================================================
-- Migration: 00019_seed_hrm_permissions
-- Seeds all HRM module permissions into the permissions table.
-- Depends on: 00003_create_permissions (permissions table exists)
-- ============================================================

INSERT INTO permissions (key, resource, action, description) VALUES
    -- Employee management
    ('hrm.employees.view',      'hrm.employees', 'view',      'View HRM employees'),
    ('hrm.employees.create',    'hrm.employees', 'create',    'Create HRM employees'),
    ('hrm.employees.update',    'hrm.employees', 'update',    'Update HRM employee records'),
    ('hrm.employees.delete',    'hrm.employees', 'delete',    'Delete HRM employee records'),
    ('hrm.employees.terminate', 'hrm.employees', 'terminate', 'Terminate an employee'),

    -- Department management
    ('hrm.departments.view',   'hrm.departments', 'view',   'View HRM departments'),
    ('hrm.departments.create', 'hrm.departments', 'create', 'Create HRM departments'),
    ('hrm.departments.update', 'hrm.departments', 'update', 'Update HRM departments'),
    ('hrm.departments.delete', 'hrm.departments', 'delete', 'Delete HRM departments'),

    -- Position management
    ('hrm.positions.view',   'hrm.positions', 'view',   'View HRM job positions'),
    ('hrm.positions.create', 'hrm.positions', 'create', 'Create HRM job positions'),
    ('hrm.positions.update', 'hrm.positions', 'update', 'Update HRM job positions'),
    ('hrm.positions.delete', 'hrm.positions', 'delete', 'Delete HRM job positions'),

    -- Leave management
    -- hrm.leave.view    — see all leave types and all leave requests
    -- hrm.leave.create  — create/update/delete leave types (HR admin)
    -- hrm.leave.update  — update leave types
    -- hrm.leave.delete  — delete leave types
    -- hrm.leave.request — submit a leave request (employee self-service)
    -- hrm.leave.approve — approve or reject any leave request (manager/HR)
    ('hrm.leave.view',    'hrm.leave', 'view',    'View all leave types and requests'),
    ('hrm.leave.create',  'hrm.leave', 'create',  'Create HRM leave types'),
    ('hrm.leave.update',  'hrm.leave', 'update',  'Update HRM leave types'),
    ('hrm.leave.delete',  'hrm.leave', 'delete',  'Delete HRM leave types'),
    ('hrm.leave.request', 'hrm.leave', 'request', 'Submit a leave request'),
    ('hrm.leave.approve', 'hrm.leave', 'approve', 'Approve or reject leave requests'),

    -- Reports
    ('hrm.reports.view', 'hrm.reports', 'view', 'View HRM reports and analytics')

ON CONFLICT (key) DO NOTHING;

-- +goose StatementEnd


-- +goose Down
-- +goose StatementBegin

DELETE FROM permissions WHERE key LIKE 'hrm.%';

-- +goose StatementEnd
