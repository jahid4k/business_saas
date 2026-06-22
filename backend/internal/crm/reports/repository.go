// backend/internal/crm/reports/repository.go
package reports

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Repository handles the one aggregate query that belongs to the reports
// domain: the CRM summary. All other report data is fetched via service
// dependencies (deals, leads, engagement) to avoid duplicating queries.
type Repository interface {
	GetSummary(ctx context.Context, orgID string) (*CRMSummary, error)
}

type repoImpl struct {
	db *pgxpool.Pool
}

// NewRepository creates a new reports repository.
func NewRepository(db *pgxpool.Pool) Repository {
	return &repoImpl{db: db}
}

func (r *repoImpl) GetSummary(ctx context.Context, orgID string) (*CRMSummary, error) {
	// Single correlated-subquery row gives us all nine counts/sums in one
	// round-trip. PostgreSQL executes each subquery independently and folds
	// the results into a single output row — equivalent to nine separate COUNT
	// calls but without the latency of nine network round-trips.
	const q = `
		SELECT
		    (SELECT COUNT(*) FROM platform_contacts  WHERE org_id = $1 AND deleted_at IS NULL),
		    (SELECT COUNT(*) FROM platform_companies WHERE org_id = $1 AND deleted_at IS NULL),
		    (SELECT COUNT(*) FROM crm_leads          WHERE org_id = $1 AND deleted_at IS NULL),
		    (SELECT COUNT(*) FROM crm_deals          WHERE org_id = $1 AND deleted_at IS NULL),
		    (SELECT COUNT(*) FROM crm_deals          WHERE org_id = $1 AND status = 'open'  AND deleted_at IS NULL),
		    (SELECT COUNT(*) FROM crm_deals          WHERE org_id = $1 AND status = 'won'   AND deleted_at IS NULL),
		    (SELECT COUNT(*) FROM crm_deals          WHERE org_id = $1 AND status = 'lost'  AND deleted_at IS NULL),
		    (SELECT COALESCE(SUM(value), 0) FROM crm_deals WHERE org_id = $1 AND deleted_at IS NULL),
		    (SELECT COALESCE(SUM(value), 0) FROM crm_deals WHERE org_id = $1 AND status = 'won' AND deleted_at IS NULL)`

	summary := &CRMSummary{}
	err := r.db.QueryRow(ctx, q, orgID).Scan(
		&summary.TotalContacts, &summary.TotalCompanies, &summary.TotalLeads,
		&summary.TotalDeals, &summary.OpenDeals, &summary.WonDeals, &summary.LostDeals,
		&summary.TotalDealValue, &summary.WonDealValue,
	)
	if err != nil {
		return nil, fmt.Errorf("reports: GetSummary: %w", err)
	}
	return summary, nil
}
