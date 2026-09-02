// backend/internal/hrm/fx/repository.go
package fx

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Repository is the FX data layer.
type Repository interface {
	CreateRate(ctx context.Context, r *ExchangeRate) error
	FindRateByRef(ctx context.Context, orgID, ref string) (*ExchangeRate, error)
	ListRates(ctx context.Context, orgID string, from, to *string, limit int) ([]*ExchangeRate, error)

	// FindDirectAsOf returns the latest rate for the pair not after asOf.
	// Returns nil when none exists, which is a normal outcome.
	FindDirectAsOf(ctx context.Context, orgID, from, to string, asOf time.Time) (*ExchangeRate, error)
}

type repoImpl struct{ db *pgxpool.Pool }

func NewRepository(db *pgxpool.Pool) Repository { return &repoImpl{db: db} }

const rateSel = `id::text, public_id, org_id::text, from_currency, to_currency, rate,
	rate_date, source, note, created_by::text, created_at`

func scanRate(row pgx.Row) (*ExchangeRate, error) {
	r := &ExchangeRate{}
	err := row.Scan(&r.ID, &r.PublicID, &r.OrgID, &r.FromCurrency, &r.ToCurrency, &r.Rate,
		&r.RateDate, &r.Source, &r.Note, &r.CreatedBy, &r.CreatedAt)
	if err != nil {
		return nil, err
	}
	return r, nil
}

func (r *repoImpl) CreateRate(ctx context.Context, e *ExchangeRate) error {
	err := r.db.QueryRow(ctx,
		`INSERT INTO hrm_exchange_rates
		   (org_id, from_currency, to_currency, rate, rate_date, source, note, created_by)
		 VALUES ($1,$2,$3,$4,$5,$6,$7,$8)
		 RETURNING id, public_id, created_at`,
		e.OrgID, e.FromCurrency, e.ToCurrency, e.Rate, e.RateDate, e.Source, e.Note, e.CreatedBy,
	).Scan(&e.ID, &e.PublicID, &e.CreatedAt)
	if err != nil {
		if strings.Contains(err.Error(), "uq_hrm_fx_pair_date") {
			return ErrDuplicateRate
		}
		if strings.Contains(err.Error(), "chk_hrm_fx_distinct") {
			return ErrSameCurrency
		}
		if strings.Contains(err.Error(), "chk_hrm_fx_positive") {
			return ErrInvalidRate
		}
		return fmt.Errorf("fx: CreateRate: %w", err)
	}
	return nil
}

func (r *repoImpl) FindRateByRef(ctx context.Context, orgID, ref string) (*ExchangeRate, error) {
	e, err := scanRate(r.db.QueryRow(ctx,
		`SELECT `+rateSel+` FROM hrm_exchange_rates
		  WHERE org_id=$1 AND (id::text=$2 OR public_id=$2)`, orgID, ref))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("fx: FindRateByRef: %w", err)
	}
	return e, nil
}

func (r *repoImpl) ListRates(ctx context.Context, orgID string, from, to *string, limit int) ([]*ExchangeRate, error) {
	q := `SELECT ` + rateSel + ` FROM hrm_exchange_rates WHERE org_id=$1`
	args := []any{orgID}
	if from != nil && *from != "" {
		args = append(args, NormaliseCurrency(*from))
		q += fmt.Sprintf(` AND from_currency=$%d`, len(args))
	}
	if to != nil && *to != "" {
		args = append(args, NormaliseCurrency(*to))
		q += fmt.Sprintf(` AND to_currency=$%d`, len(args))
	}
	if limit <= 0 || limit > 500 {
		limit = 200
	}
	args = append(args, limit)
	q += fmt.Sprintf(` ORDER BY rate_date DESC, from_currency, to_currency LIMIT $%d`, len(args))

	rows, err := r.db.Query(ctx, q, args...)
	if err != nil {
		return nil, fmt.Errorf("fx: ListRates: %w", err)
	}
	defer rows.Close()
	out := []*ExchangeRate{}
	for rows.Next() {
		e, err := scanRate(rows)
		if err != nil {
			return nil, fmt.Errorf("fx: ListRates: scan: %w", err)
		}
		out = append(out, e)
	}
	return out, rows.Err()
}

// FindDirectAsOf is the effective-dated lookup: the latest rate for the pair
// whose rate_date is not AFTER asOf.
//
// ⚠ MAX(rate_date) <= asOf, the SlabsAsOf shape
// (internal/hrm/statutory/repository.go:154). A rate recorded tomorrow must
// never price a claim submitted today — that would let a rate correction
// silently rewrite a settled figure, which is exactly what recording the rate
// alongside every converted amount exists to prevent.
func (r *repoImpl) FindDirectAsOf(ctx context.Context, orgID, from, to string, asOf time.Time) (*ExchangeRate, error) {
	e, err := scanRate(r.db.QueryRow(ctx,
		`SELECT `+rateSel+` FROM hrm_exchange_rates
		  WHERE org_id=$1 AND from_currency=$2 AND to_currency=$3 AND rate_date <= $4
		  ORDER BY rate_date DESC LIMIT 1`,
		orgID, NormaliseCurrency(from), NormaliseCurrency(to), asOf))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("fx: FindDirectAsOf: %w", err)
	}
	return e, nil
}
