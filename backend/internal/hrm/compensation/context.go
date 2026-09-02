// backend/internal/hrm/compensation/context.go
package compensation

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/shopspring/decimal"
)

// CompensationContext is what the build plan calls the shared builder "feeding
// both salary and bonus formulas". It is the one place that resolves an
// employee's current pay, their matched band and their compa-ratio, so a
// merit-matrix lookup (revisions) and a pct-of-basic bonus calculation read
// identical numbers computed the identical way.
//
// It intentionally does NOT reach for expr-lang/expr the way
// hrm/payslips.evalFormula does — payslips already owns a formula language for
// salary COMPONENTS, and duplicating that DSL here for two calc methods
// (fixed, pct_of_basic) would be exactly the speculative-primitive rule 1
// forbids. CalcMethod stays a closed, small enum until a real need for more
// widens it.
type CompensationContext struct {
	EmployeeID      string
	CurrentBasicPay decimal.Decimal
	GradeLabel      *string
	Band            *Band // nil if no band matches the employee's grade
	CompaRatio      *decimal.Decimal
	RatingLevelID   *string
	RatingLabel     *string
	RatingValue     *decimal.Decimal
	ReferenceDate   time.Time
}

// ComputeCompaRatio computes basic pay / band mid-point. Returns nil when
// there is no band to compare against — an undefined ratio, not a zero one.
// Exported and pure (no DB access) so it is unit-testable directly — the
// payslips.ComputeSlab / ReferencesGross precedent (r25).
func (c *CompensationContext) ComputeCompaRatio() *decimal.Decimal {
	if c.Band == nil || c.Band.MidAmount.IsZero() {
		return nil
	}
	r := c.CurrentBasicPay.Div(c.Band.MidAmount)
	return &r
}

// Snapshot renders the context (plus whatever the caller adds) into the
// mandatory calculation_snapshot JSONB. Marshalling failure is treated as
// impossible for this shape (decimal.Decimal and strings only) and falls back
// to a minimal-but-non-empty snapshot rather than propagating an error into a
// money-computing path for a formatting concern.
func (c *CompensationContext) Snapshot(extra map[string]any) []byte {
	m := map[string]any{
		"employee_id":       c.EmployeeID,
		"current_basic_pay": c.CurrentBasicPay.String(),
		"reference_date":    c.ReferenceDate.Format("2006-01-02"),
	}
	if c.GradeLabel != nil {
		m["grade_label"] = *c.GradeLabel
	}
	if c.Band != nil {
		m["band"] = map[string]any{
			"id":         c.Band.ID,
			"min_amount": c.Band.MinAmount.String(),
			"mid_amount": c.Band.MidAmount.String(),
			"max_amount": c.Band.MaxAmount.String(),
		}
	}
	if c.CompaRatio != nil {
		m["compa_ratio"] = c.CompaRatio.String()
	}
	if c.RatingLabel != nil {
		m["rating_label"] = *c.RatingLabel
	}
	if c.RatingValue != nil {
		m["rating_value"] = c.RatingValue.String()
	}
	for k, v := range extra {
		m[k] = v
	}
	b, err := json.Marshal(m)
	if err != nil {
		return []byte(fmt.Sprintf(`{"employee_id":%q,"snapshot_error":"marshal failed"}`, c.EmployeeID))
	}
	return b
}

// buildContext resolves one employee's CompensationContext as of
// referenceDate: current basic pay + salary structure grade, the band whose
// grade_label matches (case-insensitively) and whose effective_date is the
// latest not after referenceDate, and the compa-ratio computed against it.
//
// Rating is deliberately NOT resolved here — the merit engine (revisions) and
// a performance bonus both need "most recent PUBLISHED appraisal", but for
// different reference windows (a cycle's own date vs. a bonus period), so
// each caller resolves and attaches rating itself.
func (s *serviceImpl) buildContext(ctx context.Context, orgID, employeeID string, referenceDate time.Time) (*CompensationContext, error) {
	var basicPay decimal.Decimal
	var gradeLabel *string
	if err := s.db.QueryRow(ctx,
		`SELECT COALESCE(es.basic_pay,0), ss.grade_label
		   FROM hrm_employees e
		   LEFT JOIN LATERAL (
		       SELECT structure_id, basic_pay FROM hrm_employee_salary_records
		        WHERE employee_id = e.id AND effective_date <= $3
		        ORDER BY effective_date DESC LIMIT 1
		   ) es ON TRUE
		   LEFT JOIN hrm_salary_structures ss ON ss.id = es.structure_id
		  WHERE e.id = $1 AND e.org_id = $2`,
		employeeID, orgID, referenceDate,
	).Scan(&basicPay, &gradeLabel); err != nil {
		return nil, fmt.Errorf("compensation: buildContext: resolve salary: %w", err)
	}

	cc := &CompensationContext{
		EmployeeID:      employeeID,
		CurrentBasicPay: basicPay,
		GradeLabel:      gradeLabel,
		ReferenceDate:   referenceDate,
	}

	if gradeLabel != nil && *gradeLabel != "" {
		band, err := s.repo.FindActiveBand(ctx, orgID, *gradeLabel, referenceDate)
		if err != nil {
			return nil, fmt.Errorf("compensation: buildContext: resolve band: %w", err)
		}
		cc.Band = band
		cc.CompaRatio = cc.ComputeCompaRatio()
	}

	return cc, nil
}

// attachLatestRating resolves the employee's most recent PUBLISHED (or later —
// 'acknowledged') appraisal rating as of referenceDate and attaches it to cc.
// A missing rating is not an error — the caller decides what that means
// (skip the employee, or fall back to a default matrix behaviour).
func (s *serviceImpl) attachLatestRating(ctx context.Context, orgID string, cc *CompensationContext, referenceDate time.Time) error {
	var levelID, label string
	var value decimal.Decimal
	err := s.db.QueryRow(ctx,
		`SELECT l.id::text, l.label, l.value
		   FROM hrm_appraisals a
		   JOIN hrm_rating_scale_levels l ON l.id = a.final_rating_level_id
		  WHERE a.org_id = $1 AND a.employee_id = $2
		    AND a.phase IN ('published','acknowledged')
		    AND a.published_at <= $3
		  ORDER BY a.published_at DESC LIMIT 1`,
		orgID, cc.EmployeeID, referenceDate,
	).Scan(&levelID, &label, &value)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil // no rating found — not an error, see doc comment
	}
	if err != nil {
		return fmt.Errorf("compensation: attachLatestRating: %w", err)
	}
	cc.RatingLevelID = &levelID
	cc.RatingLabel = &label
	cc.RatingValue = &value
	return nil
}

func roundMoney(d decimal.Decimal) decimal.Decimal {
	return d.Round(2)
}

// ApplyIncrease computes currentBasicPay * (1 + increasePct/100), rounded to
// 2 decimal places. The whole of what a matched merit matrix cell does to an
// employee's pay. Exported and pure — no DB, no service receiver — so it is
// unit-testable directly without a stub repository, the payslips.ComputeSlab
// / ReferencesGross precedent (r25): the arithmetic that decides real money
// gets tested before anything that calls it.
func ApplyIncrease(currentBasicPay, increasePct decimal.Decimal) decimal.Decimal {
	increase := currentBasicPay.Mul(increasePct).Div(decimal.NewFromInt(100))
	return roundMoney(currentBasicPay.Add(increase))
}
