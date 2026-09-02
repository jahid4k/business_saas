// backend/internal/hrm/statutory/repository.go
package statutory

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Repository interface {
	ListRules(ctx context.Context, orgID string) ([]*Rule, error)
	ListActiveRules(ctx context.Context, orgID string) ([]*Rule, error)
	// ListActiveRulesForCountry narrows to one country's rules.
	//
	// ⚠ hrm_statutory_rules.country_code is NOT NULL, so unlike
	// hrm_per_diem_rates there is no "applies everywhere" wildcard row to
	// fall back to. That is exactly why ComputeForEmployee only calls this
	// when a legal entity has actually declared a country.
	ListActiveRulesForCountry(ctx context.Context, orgID, countryCode string) ([]*Rule, error)
	FindRuleByRef(ctx context.Context, orgID, ref string) (*Rule, error)
	CreateRule(ctx context.Context, r *Rule) error
	UpdateRule(ctx context.Context, r *Rule) error

	CreateSlab(ctx context.Context, ruleID string, s *Slab) error
	ListSlabsByRule(ctx context.Context, ruleID string) ([]*Slab, error)
	// SlabsAsOf returns the bracket table effective as of asOf: every slab
	// row sharing the LATEST effective_date not after asOf. A rule's
	// bracket table is revised by inserting a whole new set of rows dated
	// the same day — not by editing individual brackets in place — so this
	// grouping is what makes "the current table" well-defined rather than a
	// mix of brackets from different revisions.
	SlabsAsOf(ctx context.Context, ruleID string, asOf time.Time) ([]*Slab, error)
}

type repoImpl struct{ db *pgxpool.Pool }

func NewRepository(db *pgxpool.Pool) Repository { return &repoImpl{db: db} }

const ruleSel = `id, public_id, org_id, name, country_code, rule_type, base_variable,
	is_employer_contribution, is_active, created_by, created_at, updated_at`

func scanRule(row pgx.Row) (*Rule, error) {
	r := &Rule{}
	err := row.Scan(&r.ID, &r.PublicID, &r.OrgID, &r.Name, &r.CountryCode, &r.RuleType, &r.BaseVariable,
		&r.IsEmployerContribution, &r.IsActive, &r.CreatedBy, &r.CreatedAt, &r.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return r, nil
}

func (r *repoImpl) ListRules(ctx context.Context, orgID string) ([]*Rule, error) {
	rows, err := r.db.Query(ctx, `SELECT `+ruleSel+` FROM hrm_statutory_rules WHERE org_id=$1 ORDER BY name`, orgID)
	if err != nil {
		return nil, fmt.Errorf("statutory: ListRules: %w", err)
	}
	defer rows.Close()
	list := make([]*Rule, 0)
	for rows.Next() {
		rr, err := scanRule(rows)
		if err != nil {
			return nil, err
		}
		list = append(list, rr)
	}
	return list, rows.Err()
}

func (r *repoImpl) ListActiveRules(ctx context.Context, orgID string) ([]*Rule, error) {
	rows, err := r.db.Query(ctx,
		`SELECT `+ruleSel+` FROM hrm_statutory_rules WHERE org_id=$1 AND is_active=TRUE ORDER BY name`, orgID)
	if err != nil {
		return nil, fmt.Errorf("statutory: ListActiveRules: %w", err)
	}
	defer rows.Close()
	list := make([]*Rule, 0)
	for rows.Next() {
		rr, err := scanRule(rows)
		if err != nil {
			return nil, err
		}
		list = append(list, rr)
	}
	return list, rows.Err()
}

func (r *repoImpl) FindRuleByRef(ctx context.Context, orgID, ref string) (*Rule, error) {
	return scanRule(r.db.QueryRow(ctx,
		`SELECT `+ruleSel+` FROM hrm_statutory_rules WHERE org_id=$1 AND (id::text=$2 OR public_id=$2)`,
		orgID, ref))
}

func (r *repoImpl) CreateRule(ctx context.Context, rl *Rule) error {
	return r.db.QueryRow(ctx,
		`INSERT INTO hrm_statutory_rules (org_id, name, country_code, rule_type, base_variable, is_employer_contribution, created_by)
		 VALUES ($1,$2,$3,$4,$5,$6,$7) RETURNING id, public_id, created_at, updated_at`,
		rl.OrgID, rl.Name, rl.CountryCode, rl.RuleType, rl.BaseVariable, rl.IsEmployerContribution, rl.CreatedBy,
	).Scan(&rl.ID, &rl.PublicID, &rl.CreatedAt, &rl.UpdatedAt)
}

func (r *repoImpl) UpdateRule(ctx context.Context, rl *Rule) error {
	ct, err := r.db.Exec(ctx,
		`UPDATE hrm_statutory_rules SET is_active=$3, updated_at=NOW() WHERE org_id=$1 AND id=$2::uuid`,
		rl.OrgID, rl.ID, rl.IsActive)
	if err != nil {
		return fmt.Errorf("statutory: UpdateRule: %w", err)
	}
	if ct.RowsAffected() == 0 {
		return ErrRuleNotFound
	}
	return nil
}

const slabSel = `id, public_id, rule_id, up_to, rate_pct, effective_date, created_by, created_at`

func scanSlab(row pgx.Row) (*Slab, error) {
	s := &Slab{}
	err := row.Scan(&s.ID, &s.PublicID, &s.RuleID, &s.UpTo, &s.RatePct, &s.EffectiveDate, &s.CreatedBy, &s.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return s, nil
}

func (r *repoImpl) CreateSlab(ctx context.Context, ruleID string, s *Slab) error {
	return r.db.QueryRow(ctx,
		`INSERT INTO hrm_statutory_slabs (rule_id, up_to, rate_pct, effective_date, created_by)
		 VALUES ($1,$2,$3,$4,$5) RETURNING id, public_id, created_at`,
		ruleID, s.UpTo, s.RatePct, s.EffectiveDate, s.CreatedBy,
	).Scan(&s.ID, &s.PublicID, &s.CreatedAt)
}

func (r *repoImpl) ListSlabsByRule(ctx context.Context, ruleID string) ([]*Slab, error) {
	rows, err := r.db.Query(ctx,
		`SELECT `+slabSel+` FROM hrm_statutory_slabs WHERE rule_id=$1::uuid ORDER BY effective_date DESC, up_to NULLS LAST`, ruleID)
	if err != nil {
		return nil, fmt.Errorf("statutory: ListSlabsByRule: %w", err)
	}
	defer rows.Close()
	list := make([]*Slab, 0)
	for rows.Next() {
		s, err := scanSlab(rows)
		if err != nil {
			return nil, err
		}
		list = append(list, s)
	}
	return list, rows.Err()
}

func (r *repoImpl) SlabsAsOf(ctx context.Context, ruleID string, asOf time.Time) ([]*Slab, error) {
	rows, err := r.db.Query(ctx,
		`SELECT `+slabSel+` FROM hrm_statutory_slabs
		  WHERE rule_id=$1::uuid AND effective_date = (
		      SELECT MAX(effective_date) FROM hrm_statutory_slabs
		       WHERE rule_id=$1::uuid AND effective_date <= $2
		  )
		  ORDER BY up_to NULLS LAST`,
		ruleID, asOf)
	if err != nil {
		return nil, fmt.Errorf("statutory: SlabsAsOf: %w", err)
	}
	defer rows.Close()
	list := make([]*Slab, 0)
	for rows.Next() {
		s, err := scanSlab(rows)
		if err != nil {
			return nil, err
		}
		list = append(list, s)
	}
	return list, rows.Err()
}

func (r *repoImpl) ListActiveRulesForCountry(ctx context.Context, orgID, countryCode string) ([]*Rule, error) {
	rows, err := r.db.Query(ctx,
		`SELECT `+ruleSel+` FROM hrm_statutory_rules
		  WHERE org_id=$1 AND is_active=TRUE AND country_code=$2 ORDER BY name`,
		orgID, strings.ToUpper(strings.TrimSpace(countryCode)))
	if err != nil {
		return nil, fmt.Errorf("statutory: ListActiveRulesForCountry: %w", err)
	}
	defer rows.Close()
	list := make([]*Rule, 0)
	for rows.Next() {
		rr, err := scanRule(rows)
		if err != nil {
			return nil, err
		}
		list = append(list, rr)
	}
	return list, rows.Err()
}
