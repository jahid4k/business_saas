package dashboard

import (
	"context"
	"strconv"

	"github.com/jackc/pgx/v5/pgxpool"
)

type Repository struct {
	db *pgxpool.Pool
}

func NewRepository(db *pgxpool.Pool) *Repository {
	return &Repository{db: db}
}

func (r *Repository) GetActivePipelineValue(ctx context.Context, orgID string) (float64, error) {
	query := `
		SELECT COALESCE(SUM(value), 0)
		FROM crm_deals
		WHERE org_id = $1 AND status = 'open'
	`
	var total float64
	err := r.db.QueryRow(ctx, query, orgID).Scan(&total)
	return total, err
}

func (r *Repository) GetTotalHeadcount(ctx context.Context, orgID string) (int, error) {
	query := `
		SELECT COUNT(*)
		FROM hrm_employees
		WHERE org_id = $1 AND status = 'active'
	`
	var count int
	err := r.db.QueryRow(ctx, query, orgID).Scan(&count)
	return count, err
}

func (r *Repository) GetPendingApprovalsCount(ctx context.Context, orgID string) (int, error) {
	query := `
		SELECT COUNT(*)
		FROM hrm_approval_instances
		WHERE org_id = $1 AND overall_status = 'pending'
	`
	var count int
	err := r.db.QueryRow(ctx, query, orgID).Scan(&count)
	return count, err
}

func (r *Repository) GetStagnantDeals(ctx context.Context, orgID string, days int, limit int) ([]*ActionItem, error) {
	query := `
		SELECT
			id,
			title,
			'Deal value: $' || value::text || ' (untouched for ' || $2::text || '+ days)' as description,
			updated_at as timestamp,
			'stagnant_deal' as type,
			'crm/pipeline' as action_url
		FROM crm_deals
		WHERE org_id = $1 
		  AND status = 'open' 
		  AND updated_at < NOW() - ($2::text || ' days')::interval
		ORDER BY updated_at ASC
		LIMIT $3
	`
	items := make([]*ActionItem, 0)
	daysStr := strconv.Itoa(days)
	rows, err := r.db.Query(ctx, query, orgID, daysStr, limit)
	if err != nil {
		return items, err
	}
	defer rows.Close()

	for rows.Next() {
		var item ActionItem
		if err := rows.Scan(
			&item.ID,
			&item.Title,
			&item.Description,
			&item.Timestamp,
			&item.Type,
			&item.ActionURL,
		); err != nil {
			return nil, err
		}
		items = append(items, &item)
	}

	return items, rows.Err()
}

func (r *Repository) GetPendingApprovalItems(ctx context.Context, orgID string, limit int) ([]*ActionItem, error) {
	query := `
		SELECT
			id,
			'Review ' || INITCAP(REPLACE(entity_type, '_', ' ')) as title,
			'Pending approval request' as description,
			created_at as timestamp,
			'pending_approval' as type,
			'hrm/approvals' as action_url
		FROM hrm_approval_instances
		WHERE org_id = $1 AND overall_status = 'pending'
		ORDER BY created_at ASC
		LIMIT $2
	`
	items := make([]*ActionItem, 0)
	rows, err := r.db.Query(ctx, query, orgID, limit)
	if err != nil {
		return items, err
	}
	defer rows.Close()

	for rows.Next() {
		var item ActionItem
		if err := rows.Scan(
			&item.ID,
			&item.Title,
			&item.Description,
			&item.Timestamp,
			&item.Type,
			&item.ActionURL,
		); err != nil {
			return nil, err
		}
		items = append(items, &item)
	}

	return items, rows.Err()
}
