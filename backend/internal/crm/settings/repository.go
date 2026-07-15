package settings

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Repository interface {
	Get(ctx context.Context, orgID string) (*CRMSettings, error)
	Upsert(ctx context.Context, settings *CRMSettings) error
}

type repoImpl struct {
	db *pgxpool.Pool
}

func NewRepository(db *pgxpool.Pool) Repository {
	return &repoImpl{db: db}
}

func (r *repoImpl) Get(ctx context.Context, orgID string) (*CRMSettings, error) {
	const q = `
		SELECT org_id, lead_routing_enabled, round_robin_assignees, created_at, updated_at
		FROM crm_settings
		WHERE org_id = $1
	`
	s := &CRMSettings{}
	var assignees []byte
	err := r.db.QueryRow(ctx, q, orgID).Scan(
		&s.OrgID, &s.LeadRoutingEnabled, &assignees, &s.CreatedAt, &s.UpdatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		// Return defaults if not found
		return &CRMSettings{
			OrgID:               orgID,
			LeadRoutingEnabled:  false,
			RoundRobinAssignees: []string{},
		}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("settings: get: %w", err)
	}

	if len(assignees) > 0 {
		_ = json.Unmarshal(assignees, &s.RoundRobinAssignees)
	}
	if s.RoundRobinAssignees == nil {
		s.RoundRobinAssignees = []string{}
	}

	return s, nil
}

func (r *repoImpl) Upsert(ctx context.Context, s *CRMSettings) error {
	const q = `
		INSERT INTO crm_settings (org_id, lead_routing_enabled, round_robin_assignees)
		VALUES ($1, $2, $3)
		ON CONFLICT (org_id) DO UPDATE SET
			lead_routing_enabled = EXCLUDED.lead_routing_enabled,
			round_robin_assignees = EXCLUDED.round_robin_assignees,
			updated_at = NOW()
		RETURNING created_at, updated_at
	`
	assignees, _ := json.Marshal(s.RoundRobinAssignees)
	err := r.db.QueryRow(ctx, q, s.OrgID, s.LeadRoutingEnabled, assignees).Scan(&s.CreatedAt, &s.UpdatedAt)
	if err != nil {
		return fmt.Errorf("settings: upsert: %w", err)
	}
	return nil
}
