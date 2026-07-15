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
	GetRepPerformance(ctx context.Context, orgID string) ([]*RepPerformance, error)
	GetForecast(ctx context.Context, orgID string) (*Forecast, error)
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

func (r *repoImpl) GetRepPerformance(ctx context.Context, orgID string) ([]*RepPerformance, error) {
	const q = `
		SELECT
			u.id,
			u.full_name,
			(SELECT COUNT(*) FROM platform_activities a WHERE a.org_id = $1 AND a.created_by = u.id AND a.type = 'call') as calls,
			(SELECT COUNT(*) FROM platform_activities a WHERE a.org_id = $1 AND a.created_by = u.id AND a.type = 'meeting') as meetings,
			(SELECT COUNT(*) FROM crm_deals d WHERE d.org_id = $1 AND d.owner_id = u.id AND d.status = 'won' AND d.deleted_at IS NULL) as deals_closed,
			(SELECT COALESCE(SUM(d.value), 0) FROM crm_deals d WHERE d.org_id = $1 AND d.owner_id = u.id AND d.status = 'won' AND d.deleted_at IS NULL) as revenue_won
		FROM org_members om
		JOIN users u ON u.id = om.user_id
		WHERE om.org_id = $1
		ORDER BY revenue_won DESC, deals_closed DESC
	`

	rows, err := r.db.Query(ctx, q, orgID)
	if err != nil {
		return nil, fmt.Errorf("reports: GetRepPerformance: query: %w", err)
	}
	defer rows.Close()

	var result []*RepPerformance
	for rows.Next() {
		rp := &RepPerformance{}
		if err := rows.Scan(
			&rp.RepID,
			&rp.RepName,
			&rp.Calls,
			&rp.Meetings,
			&rp.DealsClosed,
			&rp.RevenueWon,
		); err != nil {
			return nil, fmt.Errorf("reports: GetRepPerformance: scan: %w", err)
		}

		result = append(result, rp)
	}
	return result, rows.Err()
}

func (r *repoImpl) GetForecast(ctx context.Context, orgID string) (*Forecast, error) {
	const q = `
		SELECT
			COALESCE(SUM(d.value), 0) as total_pipeline_value,
			COALESCE(SUM(d.value * (s.probability::numeric / 100)), 0) as weighted_forecast
		FROM crm_deals d
		JOIN crm_pipeline_stages s ON d.stage_id = s.id
		WHERE d.org_id = $1 AND d.status = 'open' AND d.deleted_at IS NULL
	`

	f := &Forecast{}
	err := r.db.QueryRow(ctx, q, orgID).Scan(&f.TotalPipelineValue, &f.WeightedForecast)
	if err != nil {
		return nil, fmt.Errorf("reports: GetForecast: %w", err)
	}
	return f, nil
}
