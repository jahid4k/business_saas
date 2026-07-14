-- +goose Up
-- +goose StatementBegin

CREATE TABLE hrm_employee_statuses (
    id          UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id      UUID        NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    name        TEXT        NOT NULL,
    category    TEXT        NOT NULL CHECK (category IN ('active', 'inactive', 'on_leave', 'terminated')),
    color       TEXT        NOT NULL DEFAULT 'bg-zinc-500/10 text-zinc-400 border-zinc-500/20',
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (org_id, name)
);

COMMENT ON TABLE hrm_employee_statuses IS 'Dynamic employee statuses per organization.';

ALTER TABLE hrm_employees ADD COLUMN status_id UUID REFERENCES hrm_employee_statuses(id) ON DELETE RESTRICT;

-- Insert default statuses for all existing organizations and map existing employees
DO $$
DECLARE
    org RECORD;
    v_active UUID;
    v_inactive UUID;
    v_on_leave UUID;
    v_resigned UUID;
    v_terminated UUID;
BEGIN
    FOR org IN SELECT id FROM organizations LOOP
        INSERT INTO hrm_employee_statuses (org_id, name, category, color)
        VALUES 
            (org.id, 'Active', 'active', 'bg-emerald-500/10 text-emerald-400 border-emerald-500/20') RETURNING id INTO v_active;
        
        INSERT INTO hrm_employee_statuses (org_id, name, category, color)
        VALUES 
            (org.id, 'Inactive', 'inactive', 'bg-zinc-500/10 text-zinc-400 border-zinc-500/20') RETURNING id INTO v_inactive;

        INSERT INTO hrm_employee_statuses (org_id, name, category, color)
        VALUES 
            (org.id, 'On Leave', 'on_leave', 'bg-amber-500/10 text-amber-400 border-amber-500/20') RETURNING id INTO v_on_leave;

        INSERT INTO hrm_employee_statuses (org_id, name, category, color)
        VALUES 
            (org.id, 'Resigned', 'terminated', 'bg-orange-500/10 text-orange-400 border-orange-500/20') RETURNING id INTO v_resigned;

        INSERT INTO hrm_employee_statuses (org_id, name, category, color)
        VALUES 
            (org.id, 'Terminated', 'terminated', 'bg-red-500/10 text-red-400 border-red-500/20') RETURNING id INTO v_terminated;

        -- Update existing employees for this org
        UPDATE hrm_employees SET status_id = v_active WHERE org_id = org.id AND status = 'active';
        UPDATE hrm_employees SET status_id = v_inactive WHERE org_id = org.id AND status = 'inactive';
        UPDATE hrm_employees SET status_id = v_on_leave WHERE org_id = org.id AND status = 'on_leave';
        UPDATE hrm_employees SET status_id = v_resigned WHERE org_id = org.id AND status = 'resigned';
        UPDATE hrm_employees SET status_id = v_terminated WHERE org_id = org.id AND status = 'terminated';
    END LOOP;
END $$;

ALTER TABLE hrm_employees ALTER COLUMN status_id SET NOT NULL;
ALTER TABLE hrm_employees DROP COLUMN status;

-- +goose StatementEnd


-- +goose Down
-- +goose StatementBegin

ALTER TABLE hrm_employees ADD COLUMN status TEXT DEFAULT 'active';

DO $$
DECLARE
    emp RECORD;
    st RECORD;
BEGIN
    FOR emp IN SELECT id, status_id FROM hrm_employees LOOP
        SELECT name INTO st FROM hrm_employee_statuses WHERE id = emp.status_id;
        IF st.name = 'Active' THEN
            UPDATE hrm_employees SET status = 'active' WHERE id = emp.id;
        ELSIF st.name = 'Inactive' THEN
            UPDATE hrm_employees SET status = 'inactive' WHERE id = emp.id;
        ELSIF st.name = 'On Leave' THEN
            UPDATE hrm_employees SET status = 'on_leave' WHERE id = emp.id;
        ELSIF st.name = 'Resigned' THEN
            UPDATE hrm_employees SET status = 'resigned' WHERE id = emp.id;
        ELSIF st.name = 'Terminated' THEN
            UPDATE hrm_employees SET status = 'terminated' WHERE id = emp.id;
        ELSE
            UPDATE hrm_employees SET status = 'active' WHERE id = emp.id;
        END IF;
    END LOOP;
END $$;

ALTER TABLE hrm_employees
    ADD CONSTRAINT hrm_employees_status_check
        CHECK (status IN ('active', 'inactive', 'on_leave', 'terminated', 'resigned'));
ALTER TABLE hrm_employees ALTER COLUMN status SET NOT NULL;
ALTER TABLE hrm_employees DROP COLUMN status_id;

DROP TABLE hrm_employee_statuses;

-- +goose StatementEnd
