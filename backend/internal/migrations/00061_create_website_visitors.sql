-- +goose Up
-- +goose StatementBegin

CREATE TABLE website_visitors (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    session_id TEXT NOT NULL,
    ip_address TEXT,
    user_agent TEXT,
    company_name TEXT,
    company_domain TEXT,
    enrichment_data JSONB,
    linked_lead_id UUID REFERENCES crm_leads(id) ON DELETE SET NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_website_visitors_org_id ON website_visitors(org_id);
CREATE INDEX idx_website_visitors_session_id ON website_visitors(session_id);

CREATE TABLE visitor_pageviews (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    visitor_id UUID NOT NULL REFERENCES website_visitors(id) ON DELETE CASCADE,
    url TEXT NOT NULL,
    title TEXT,
    referrer TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_visitor_pageviews_visitor_id ON visitor_pageviews(visitor_id);

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

DROP TABLE visitor_pageviews;
DROP TABLE website_visitors;

-- +goose StatementEnd
