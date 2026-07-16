package visitors

import (
	"context"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Repository interface {
	FindVisitorBySession(ctx context.Context, orgID, sessionID string) (*WebsiteVisitor, error)
	CreateVisitor(ctx context.Context, v *WebsiteVisitor) error
	UpdateVisitor(ctx context.Context, v *WebsiteVisitor) error
	CreatePageview(ctx context.Context, pv *VisitorPageview) error
	ListVisitors(ctx context.Context, orgID string) ([]*WebsiteVisitor, error)
}

type repoImpl struct {
	db *pgxpool.Pool
}

func NewRepository(db *pgxpool.Pool) Repository {
	return &repoImpl{db: db}
}

func (r *repoImpl) FindVisitorBySession(ctx context.Context, orgID, sessionID string) (*WebsiteVisitor, error) {
	const q = `
		SELECT id, org_id, session_id, ip_address, user_agent, company_name, company_domain, enrichment_data, linked_lead_id, created_at, updated_at
		FROM website_visitors
		WHERE org_id = $1 AND session_id = $2
	`
	v := &WebsiteVisitor{}
	err := r.db.QueryRow(ctx, q, orgID, sessionID).Scan(
		&v.ID, &v.OrgID, &v.SessionID, &v.IPAddress, &v.UserAgent, &v.CompanyName, &v.CompanyDomain, &v.EnrichmentData, &v.LinkedLeadID, &v.CreatedAt, &v.UpdatedAt,
	)
	if err == pgx.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return v, nil
}

func (r *repoImpl) CreateVisitor(ctx context.Context, v *WebsiteVisitor) error {
	const q = `
		INSERT INTO website_visitors
		(org_id, session_id, ip_address, user_agent, company_name, company_domain, enrichment_data, linked_lead_id)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		RETURNING id, created_at, updated_at
	`
	return r.db.QueryRow(ctx, q,
		v.OrgID, v.SessionID, v.IPAddress, v.UserAgent, v.CompanyName, v.CompanyDomain, v.EnrichmentData, v.LinkedLeadID,
	).Scan(&v.ID, &v.CreatedAt, &v.UpdatedAt)
}

func (r *repoImpl) UpdateVisitor(ctx context.Context, v *WebsiteVisitor) error {
	const q = `
		UPDATE website_visitors
		SET ip_address = $1, user_agent = $2, company_name = $3, company_domain = $4, enrichment_data = $5, linked_lead_id = $6, updated_at = NOW()
		WHERE id = $7 AND org_id = $8
		RETURNING updated_at
	`
	return r.db.QueryRow(ctx, q,
		v.IPAddress, v.UserAgent, v.CompanyName, v.CompanyDomain, v.EnrichmentData, v.LinkedLeadID, v.ID, v.OrgID,
	).Scan(&v.UpdatedAt)
}

func (r *repoImpl) CreatePageview(ctx context.Context, pv *VisitorPageview) error {
	const q = `
		INSERT INTO visitor_pageviews
		(visitor_id, url, title, referrer)
		VALUES ($1, $2, $3, $4)
		RETURNING id, created_at
	`
	return r.db.QueryRow(ctx, q,
		pv.VisitorID, pv.URL, pv.Title, pv.Referrer,
	).Scan(&pv.ID, &pv.CreatedAt)
}

func (r *repoImpl) ListVisitors(ctx context.Context, orgID string) ([]*WebsiteVisitor, error) {
	const q = `
		SELECT id, org_id, session_id, ip_address, user_agent, company_name, company_domain, enrichment_data, linked_lead_id, created_at, updated_at
		FROM website_visitors
		WHERE org_id = $1
		ORDER BY updated_at DESC
		LIMIT 100
	`
	rows, err := r.db.Query(ctx, q, orgID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var visitors []*WebsiteVisitor
	for rows.Next() {
		var v WebsiteVisitor
		if err := rows.Scan(
			&v.ID, &v.OrgID, &v.SessionID, &v.IPAddress, &v.UserAgent, &v.CompanyName, &v.CompanyDomain, &v.EnrichmentData, &v.LinkedLeadID, &v.CreatedAt, &v.UpdatedAt,
		); err != nil {
			return nil, err
		}
		visitors = append(visitors, &v)
	}
	return visitors, rows.Err()
}
