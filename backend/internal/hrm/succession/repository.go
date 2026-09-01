// backend/internal/hrm/succession/repository.go
package succession

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/shopspring/decimal"
)

// Repository is the succession data layer.
//
// ⚠ THE SUBJECT AND REVIEWER READ PATHS ARE SEPARATE METHODS BY DESIGN, and
// no method serves both. SubjectPlans' SQL never names hrm_talent_assessments
// or hrm_succession_candidates; that is the confidentiality guarantee, and it
// is a property of the queries rather than of any filtering applied
// afterwards. A future "just add a join, it's convenient" is the failure mode
// this shape exists to make visible.
type Repository interface {
	EmployeeExists(ctx context.Context, orgID, employeeID string) (bool, error)
	PositionExists(ctx context.Context, orgID, positionID string) (bool, error)
	// ResolveOwnEmployeeID maps a caller's user id to their employee row.
	// Lives here rather than on employees.Service so this package owns its
	// own resolution, the 7C/7D pattern.
	ResolveOwnEmployeeID(ctx context.Context, orgID, userID string) (string, error)

	// Critical positions
	CreateCritical(ctx context.Context, cp *CriticalPosition) error
	UpdateCritical(ctx context.Context, cp *CriticalPosition) error
	FindCriticalByRef(ctx context.Context, orgID, ref string) (*CriticalPosition, error)
	ListCritical(ctx context.Context, orgID string, activeOnly bool) ([]*CriticalPosition, error)

	// Talent assessments — CONFIDENTIAL
	UpsertAssessment(ctx context.Context, a *TalentAssessment) error
	LatestAssessment(ctx context.Context, orgID, employeeID string) (*TalentAssessment, error)
	ListAssessments(ctx context.Context, orgID string, asOf *time.Time) ([]*TalentAssessment, error)
	// LatestPublishedRating returns the two most recent published appraisal
	// ratings, newest first, for both the performance axis and the decline
	// signal.
	LatestPublishedRatings(ctx context.Context, orgID, employeeID string, limit int) ([]decimal.Decimal, error)
	RatingScaleMax(ctx context.Context, orgID string) (decimal.Decimal, bool, error)

	// Candidates — CONFIDENTIAL
	CreateCandidate(ctx context.Context, c *Candidate) error
	FindCandidateByRef(ctx context.Context, orgID, ref string) (*Candidate, error)
	WithdrawCandidate(ctx context.Context, orgID, id string, reason *string, at time.Time) error
	ListCandidatesForPosition(ctx context.Context, orgID, criticalPositionID string, activeOnly bool) ([]*Candidate, error)
	ListCandidatesForEmployee(ctx context.Context, orgID, employeeID string) ([]*Candidate, error)

	// Development plans — SUBJECT-VISIBLE
	CreatePlan(ctx context.Context, p *DevelopmentPlan) error
	UpdatePlan(ctx context.Context, p *DevelopmentPlan) error
	FindPlanByRef(ctx context.Context, orgID, ref string) (*DevelopmentPlan, error)
	// SubjectPlans is the subject's ONLY read path. Its SQL touches
	// hrm_development_plans and hrm_development_plan_items and nothing else.
	SubjectPlans(ctx context.Context, orgID, employeeID string) ([]*DevelopmentPlan, error)
	ListPlans(ctx context.Context, orgID, employeeID string) ([]*DevelopmentPlan, error)
	CreateItem(ctx context.Context, it *PlanItem) error
	UpdateItem(ctx context.Context, it *PlanItem) error
	FindItemByRef(ctx context.Context, orgID, ref string) (*PlanItem, error)

	// Flight-risk inputs, each a plain fact read from an existing table.
	EmployeeTimeline(ctx context.Context, orgID, employeeID string) (hire time.Time, lastPromotion *time.Time, name string, err error)
	CurrentPayAndBand(ctx context.Context, orgID, employeeID string, asOf time.Time) (basic, bandMin decimal.Decimal, grade string, err error)
	ManagerChangesSince(ctx context.Context, orgID, employeeID string, since time.Time) (int, error)
}

type repoImpl struct{ db *pgxpool.Pool }

func NewRepository(db *pgxpool.Pool) Repository { return &repoImpl{db: db} }

// ── existence and identity ───────────────────────────────────────────────────

func (r *repoImpl) EmployeeExists(ctx context.Context, orgID, employeeID string) (bool, error) {
	var ok bool
	err := r.db.QueryRow(ctx,
		`SELECT EXISTS(SELECT 1 FROM hrm_employees WHERE org_id=$1 AND id=$2::uuid)`,
		orgID, employeeID).Scan(&ok)
	if err != nil {
		return false, fmt.Errorf("succession: EmployeeExists: %w", err)
	}
	return ok, nil
}

func (r *repoImpl) PositionExists(ctx context.Context, orgID, positionID string) (bool, error) {
	var ok bool
	err := r.db.QueryRow(ctx,
		`SELECT EXISTS(SELECT 1 FROM hrm_positions WHERE org_id=$1 AND id=$2::uuid)`,
		orgID, positionID).Scan(&ok)
	if err != nil {
		return false, fmt.Errorf("succession: PositionExists: %w", err)
	}
	return ok, nil
}

func (r *repoImpl) ResolveOwnEmployeeID(ctx context.Context, orgID, userID string) (string, error) {
	var id string
	err := r.db.QueryRow(ctx,
		`SELECT id::text FROM hrm_employees WHERE org_id=$1 AND user_id=$2::uuid LIMIT 1`,
		orgID, userID).Scan(&id)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", ErrNoEmployeeRecord
	}
	if err != nil {
		return "", fmt.Errorf("succession: ResolveOwnEmployeeID: %w", err)
	}
	return id, nil
}

// ── critical positions ───────────────────────────────────────────────────────

const criticalSel = `cp.id::text, cp.public_id, cp.org_id::text, cp.position_id::text,
	COALESCE(p.title,''), cp.criticality_level, cp.vacancy_risk, cp.impact_of_vacancy,
	cp.identified_by::text, cp.review_due_date, cp.is_active, cp.deactivated_at,
	cp.created_at, cp.updated_at,
	(SELECT count(*) FROM hrm_employees e
	  WHERE e.org_id = cp.org_id AND e.position_id = cp.position_id
	    AND e.termination_date IS NULL),
	(SELECT count(*) FROM hrm_succession_candidates sc
	  WHERE sc.critical_position_id = cp.id AND sc.status = 'active')`

func scanCritical(row pgx.Row) (*CriticalPosition, error) {
	cp := &CriticalPosition{}
	err := row.Scan(&cp.ID, &cp.PublicID, &cp.OrgID, &cp.PositionID, &cp.PositionTitle,
		&cp.CriticalityLevel, &cp.VacancyRisk, &cp.ImpactOfVacancy, &cp.IdentifiedBy,
		&cp.ReviewDueDate, &cp.IsActive, &cp.DeactivatedAt, &cp.CreatedAt, &cp.UpdatedAt,
		&cp.IncumbentCount, &cp.ActiveCandidates)
	if err != nil {
		return nil, err
	}
	return cp, nil
}

func (r *repoImpl) CreateCritical(ctx context.Context, cp *CriticalPosition) error {
	err := r.db.QueryRow(ctx,
		`INSERT INTO hrm_critical_positions
		   (org_id, position_id, criticality_level, vacancy_risk, impact_of_vacancy,
		    identified_by, review_due_date)
		 VALUES ($1,$2::uuid,$3,$4,$5,$6,$7)
		 RETURNING id, public_id, is_active, created_at, updated_at`,
		cp.OrgID, cp.PositionID, cp.CriticalityLevel, cp.VacancyRisk, cp.ImpactOfVacancy,
		cp.IdentifiedBy, cp.ReviewDueDate,
	).Scan(&cp.ID, &cp.PublicID, &cp.IsActive, &cp.CreatedAt, &cp.UpdatedAt)
	if err != nil {
		if strings.Contains(err.Error(), "uq_hrm_critpos_active_position") {
			return ErrAlreadyDesignated
		}
		return fmt.Errorf("succession: CreateCritical: %w", err)
	}
	return nil
}

func (r *repoImpl) UpdateCritical(ctx context.Context, cp *CriticalPosition) error {
	_, err := r.db.Exec(ctx,
		`UPDATE hrm_critical_positions
		    SET criticality_level=$3, vacancy_risk=$4, impact_of_vacancy=$5,
		        review_due_date=$6, is_active=$7, deactivated_at=$8, updated_at=NOW()
		  WHERE org_id=$1 AND id=$2::uuid`,
		cp.OrgID, cp.ID, cp.CriticalityLevel, cp.VacancyRisk, cp.ImpactOfVacancy,
		cp.ReviewDueDate, cp.IsActive, cp.DeactivatedAt)
	if err != nil {
		if strings.Contains(err.Error(), "uq_hrm_critpos_active_position") {
			return ErrAlreadyDesignated
		}
		return fmt.Errorf("succession: UpdateCritical: %w", err)
	}
	return nil
}

func (r *repoImpl) FindCriticalByRef(ctx context.Context, orgID, ref string) (*CriticalPosition, error) {
	cp, err := scanCritical(r.db.QueryRow(ctx,
		`SELECT `+criticalSel+` FROM hrm_critical_positions cp
		   LEFT JOIN hrm_positions p ON p.id = cp.position_id
		  WHERE cp.org_id=$1 AND (cp.id::text=$2 OR cp.public_id=$2)`, orgID, ref))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("succession: FindCriticalByRef: %w", err)
	}
	return cp, nil
}

func (r *repoImpl) ListCritical(ctx context.Context, orgID string, activeOnly bool) ([]*CriticalPosition, error) {
	q := `SELECT ` + criticalSel + ` FROM hrm_critical_positions cp
	        LEFT JOIN hrm_positions p ON p.id = cp.position_id
	       WHERE cp.org_id=$1`
	if activeOnly {
		q += ` AND cp.is_active`
	}
	// Mission-critical roles with an empty bench are what the list is for,
	// so they sort to the top without the caller having to ask.
	q += ` ORDER BY CASE cp.criticality_level WHEN 'mission_critical' THEN 0 WHEN 'high' THEN 1 ELSE 2 END,
	              CASE cp.vacancy_risk WHEN 'high' THEN 0 WHEN 'medium' THEN 1 ELSE 2 END,
	              p.title NULLS LAST`
	rows, err := r.db.Query(ctx, q, orgID)
	if err != nil {
		return nil, fmt.Errorf("succession: ListCritical: %w", err)
	}
	defer rows.Close()
	out := []*CriticalPosition{}
	for rows.Next() {
		cp, err := scanCritical(rows)
		if err != nil {
			return nil, fmt.Errorf("succession: ListCritical: scan: %w", err)
		}
		out = append(out, cp)
	}
	return out, rows.Err()
}

// ── talent assessments (CONFIDENTIAL) ────────────────────────────────────────

const assessmentSel = `id::text, public_id, org_id::text, employee_id::text, as_of_date,
	performance_band, performance_appraisal_id::text, performance_rating_snapshot,
	potential_band, potential_rationale, assessed_by::text, created_at, updated_at`

// assessmentSelQualified is written out rather than derived from assessmentSel
// by string surgery. r33 mangled public_id, org_id and employee_id doing
// exactly that with a ReplaceAll on "id,".
const assessmentSelQualified = `t.id::text, t.public_id, t.org_id::text, t.employee_id::text, t.as_of_date,
	t.performance_band, t.performance_appraisal_id::text, t.performance_rating_snapshot,
	t.potential_band, t.potential_rationale, t.assessed_by::text, t.created_at, t.updated_at`

func scanAssessment(row pgx.Row) (*TalentAssessment, error) {
	a := &TalentAssessment{}
	err := row.Scan(&a.ID, &a.PublicID, &a.OrgID, &a.EmployeeID, &a.AsOfDate,
		&a.PerformanceBand, &a.PerformanceAppraisalID, &a.PerformanceRatingSnapshot,
		&a.PotentialBand, &a.PotentialRationale, &a.AssessedBy, &a.CreatedAt, &a.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return a, nil
}

// UpsertAssessment treats a re-assessment on the same as-of date as an edit.
// Two rows for one employee on one date would be two opinions with no way to
// say which the grid should draw.
func (r *repoImpl) UpsertAssessment(ctx context.Context, a *TalentAssessment) error {
	err := r.db.QueryRow(ctx,
		`INSERT INTO hrm_talent_assessments
		   (org_id, employee_id, as_of_date, performance_band, performance_appraisal_id,
		    performance_rating_snapshot, potential_band, potential_rationale, assessed_by)
		 VALUES ($1,$2::uuid,$3,$4,$5,$6,$7,$8,$9)
		 ON CONFLICT (org_id, employee_id, as_of_date) DO UPDATE SET
		    performance_band=EXCLUDED.performance_band,
		    performance_appraisal_id=EXCLUDED.performance_appraisal_id,
		    performance_rating_snapshot=EXCLUDED.performance_rating_snapshot,
		    potential_band=EXCLUDED.potential_band,
		    potential_rationale=EXCLUDED.potential_rationale,
		    assessed_by=EXCLUDED.assessed_by,
		    updated_at=NOW()
		 RETURNING id, public_id, created_at, updated_at`,
		a.OrgID, a.EmployeeID, a.AsOfDate, a.PerformanceBand, a.PerformanceAppraisalID,
		a.PerformanceRatingSnapshot, a.PotentialBand, a.PotentialRationale, a.AssessedBy,
	).Scan(&a.ID, &a.PublicID, &a.CreatedAt, &a.UpdatedAt)
	if err != nil {
		return fmt.Errorf("succession: UpsertAssessment: %w", err)
	}
	return nil
}

func (r *repoImpl) LatestAssessment(ctx context.Context, orgID, employeeID string) (*TalentAssessment, error) {
	a, err := scanAssessment(r.db.QueryRow(ctx,
		`SELECT `+assessmentSel+` FROM hrm_talent_assessments
		  WHERE org_id=$1 AND employee_id=$2::uuid
		  ORDER BY as_of_date DESC LIMIT 1`, orgID, employeeID))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("succession: LatestAssessment: %w", err)
	}
	return a, nil
}

// ListAssessments returns the most recent assessment per employee not after
// asOf — the effective-dated MAX(...) <= asOf shape used for salary slabs, so
// the grid can be redrawn as it stood at any past review.
func (r *repoImpl) ListAssessments(ctx context.Context, orgID string, asOf *time.Time) ([]*TalentAssessment, error) {
	args := []any{orgID}
	// The cutoff has to appear in BOTH the outer filter and the correlated
	// MAX, or the outer row would be matched against a maximum computed from
	// assessments that had not happened yet as of the reporting date.
	outer, inner := ``, ``
	if asOf != nil {
		outer = ` AND t.as_of_date <= $2`
		inner = ` AND t2.as_of_date <= $2`
		args = append(args, *asOf)
	}
	rows, err := r.db.Query(ctx,
		`SELECT `+assessmentSelQualified+` FROM hrm_talent_assessments t
		  WHERE t.org_id=$1`+outer+`
		    AND t.as_of_date = (
		        SELECT max(t2.as_of_date) FROM hrm_talent_assessments t2
		         WHERE t2.org_id = t.org_id AND t2.employee_id = t.employee_id`+inner+`
		    )
		  ORDER BY t.employee_id`, args...)
	if err != nil {
		return nil, fmt.Errorf("succession: ListAssessments: %w", err)
	}
	defer rows.Close()
	out := []*TalentAssessment{}
	for rows.Next() {
		a, err := scanAssessment(rows)
		if err != nil {
			return nil, fmt.Errorf("succession: ListAssessments: scan: %w", err)
		}
		out = append(out, a)
	}
	return out, rows.Err()
}

// LatestPublishedRatings returns published, uncancelled final ratings newest
// first. Only PUBLISHED appraisals count: a draft rating is a work in
// progress and placing somebody on a grid from one would be judging them on
// an unfinished sentence.
func (r *repoImpl) LatestPublishedRatings(ctx context.Context, orgID, employeeID string, limit int) ([]decimal.Decimal, error) {
	rows, err := r.db.Query(ctx,
		`SELECT final_rating_value FROM hrm_appraisals
		  WHERE org_id=$1 AND employee_id=$2::uuid
		    AND published_at IS NOT NULL AND cancelled_at IS NULL
		    AND final_rating_value IS NOT NULL
		  ORDER BY published_at DESC LIMIT $3`, orgID, employeeID, limit)
	if err != nil {
		return nil, fmt.Errorf("succession: LatestPublishedRatings: %w", err)
	}
	defer rows.Close()
	out := []decimal.Decimal{}
	for rows.Next() {
		var v decimal.Decimal
		if err := rows.Scan(&v); err != nil {
			return nil, fmt.Errorf("succession: LatestPublishedRatings: scan: %w", err)
		}
		out = append(out, v)
	}
	return out, rows.Err()
}

// RatingScaleMax reads the highest point value on the org's rating scale, so
// the performance thresholds stay fractions rather than assuming 5 points.
func (r *repoImpl) RatingScaleMax(ctx context.Context, orgID string) (decimal.Decimal, bool, error) {
	// max() over zero rows returns one NULL row rather than ErrNoRows, so the
	// destination must be nullable or an org with no scale would error.
	var max *decimal.Decimal
	err := r.db.QueryRow(ctx,
		`SELECT max(l.value) FROM hrm_rating_scale_levels l
		  WHERE l.scale_id = (
		      SELECT s.id FROM hrm_rating_scales s
		       WHERE s.org_id=$1 AND s.is_active
		       ORDER BY s.is_default DESC, s.created_at LIMIT 1
		  )`, orgID).Scan(&max)
	if err != nil {
		return decimal.Zero, false, fmt.Errorf("succession: RatingScaleMax: %w", err)
	}
	// One scale, chosen deterministically. Taking max() across every active
	// scale would measure an employee rated on a 5-point scale against a
	// 10-point maximum and band the whole company low.
	if max == nil || max.LessThanOrEqual(decimal.Zero) {
		return decimal.Zero, false, nil
	}
	return *max, true, nil
}

// ── candidates (CONFIDENTIAL) ────────────────────────────────────────────────

const candidateSel = `sc.id::text, sc.public_id, sc.org_id::text, sc.critical_position_id::text,
	sc.employee_id::text, COALESCE(btrim(e.first_name || ' ' || COALESCE(e.last_name,'')),''),
	sc.readiness, sc.nomination_rationale, sc.development_plan_id::text, sc.status,
	sc.withdrawn_at, sc.withdrawn_reason, sc.nominated_by::text, sc.created_at, sc.updated_at`

func scanCandidate(row pgx.Row) (*Candidate, error) {
	c := &Candidate{}
	err := row.Scan(&c.ID, &c.PublicID, &c.OrgID, &c.CriticalPositionID, &c.EmployeeID,
		&c.EmployeeName, &c.Readiness, &c.NominationRationale, &c.DevelopmentPlanID,
		&c.Status, &c.WithdrawnAt, &c.WithdrawnReason, &c.NominatedBy, &c.CreatedAt, &c.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return c, nil
}

func (r *repoImpl) CreateCandidate(ctx context.Context, c *Candidate) error {
	err := r.db.QueryRow(ctx,
		`INSERT INTO hrm_succession_candidates
		   (org_id, critical_position_id, employee_id, readiness, nomination_rationale,
		    development_plan_id, nominated_by)
		 VALUES ($1,$2::uuid,$3::uuid,$4,$5,$6::uuid,$7)
		 RETURNING id, public_id, status, created_at, updated_at`,
		c.OrgID, c.CriticalPositionID, c.EmployeeID, c.Readiness, c.NominationRationale,
		c.DevelopmentPlanID, c.NominatedBy,
	).Scan(&c.ID, &c.PublicID, &c.Status, &c.CreatedAt, &c.UpdatedAt)
	if err != nil {
		if strings.Contains(err.Error(), "uq_hrm_succand_active") {
			return ErrAlreadyNominated
		}
		return fmt.Errorf("succession: CreateCandidate: %w", err)
	}
	return nil
}

func (r *repoImpl) FindCandidateByRef(ctx context.Context, orgID, ref string) (*Candidate, error) {
	c, err := scanCandidate(r.db.QueryRow(ctx,
		`SELECT `+candidateSel+` FROM hrm_succession_candidates sc
		   LEFT JOIN hrm_employees e ON e.id = sc.employee_id
		  WHERE sc.org_id=$1 AND (sc.id::text=$2 OR sc.public_id=$2)`, orgID, ref))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("succession: FindCandidateByRef: %w", err)
	}
	return c, nil
}

func (r *repoImpl) WithdrawCandidate(ctx context.Context, orgID, id string, reason *string, at time.Time) error {
	ct, err := r.db.Exec(ctx,
		`UPDATE hrm_succession_candidates
		    SET status='withdrawn', withdrawn_at=$3, withdrawn_reason=$4, updated_at=NOW()
		  WHERE org_id=$1 AND id=$2::uuid AND status='active'`, orgID, id, at, reason)
	if err != nil {
		return fmt.Errorf("succession: WithdrawCandidate: %w", err)
	}
	if ct.RowsAffected() == 0 {
		return ErrAlreadyWithdrawn
	}
	return nil
}

func (r *repoImpl) listCandidates(ctx context.Context, where string, args ...any) ([]*Candidate, error) {
	rows, err := r.db.Query(ctx,
		`SELECT `+candidateSel+` FROM hrm_succession_candidates sc
		   LEFT JOIN hrm_employees e ON e.id = sc.employee_id
		  WHERE `+where+`
		  ORDER BY CASE sc.readiness
		             WHEN 'ready_now' THEN 0 WHEN 'ready_1_2_years' THEN 1
		             WHEN 'ready_3_5_years' THEN 2 ELSE 3 END, sc.created_at`, args...)
	if err != nil {
		return nil, fmt.Errorf("succession: listCandidates: %w", err)
	}
	defer rows.Close()
	out := []*Candidate{}
	for rows.Next() {
		c, err := scanCandidate(rows)
		if err != nil {
			return nil, fmt.Errorf("succession: listCandidates: scan: %w", err)
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

func (r *repoImpl) ListCandidatesForPosition(ctx context.Context, orgID, criticalPositionID string, activeOnly bool) ([]*Candidate, error) {
	w := `sc.org_id=$1 AND sc.critical_position_id=$2::uuid`
	if activeOnly {
		w += ` AND sc.status='active'`
	}
	return r.listCandidates(ctx, w, orgID, criticalPositionID)
}

func (r *repoImpl) ListCandidatesForEmployee(ctx context.Context, orgID, employeeID string) ([]*Candidate, error) {
	return r.listCandidates(ctx, `sc.org_id=$1 AND sc.employee_id=$2::uuid`, orgID, employeeID)
}

// ── development plans (SUBJECT-VISIBLE) ──────────────────────────────────────

const planSel = `id::text, public_id, org_id::text, employee_id::text, title, objective,
	target_date, status, completed_at, created_by::text, created_at, updated_at`

func scanPlan(row pgx.Row) (*DevelopmentPlan, error) {
	p := &DevelopmentPlan{}
	err := row.Scan(&p.ID, &p.PublicID, &p.OrgID, &p.EmployeeID, &p.Title, &p.Objective,
		&p.TargetDate, &p.Status, &p.CompletedAt, &p.CreatedBy, &p.CreatedAt, &p.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return p, nil
}

const itemSel = `id::text, public_id, org_id::text, plan_id::text, description, target_date,
	status, completed_at, sort_order, created_by::text, created_at, updated_at`

func scanItem(row pgx.Row) (*PlanItem, error) {
	it := &PlanItem{}
	err := row.Scan(&it.ID, &it.PublicID, &it.OrgID, &it.PlanID, &it.Description, &it.TargetDate,
		&it.Status, &it.CompletedAt, &it.SortOrder, &it.CreatedBy, &it.CreatedAt, &it.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return it, nil
}

func (r *repoImpl) CreatePlan(ctx context.Context, p *DevelopmentPlan) error {
	err := r.db.QueryRow(ctx,
		`INSERT INTO hrm_development_plans
		   (org_id, employee_id, title, objective, target_date, status, created_by)
		 VALUES ($1,$2::uuid,$3,$4,$5,$6,$7)
		 RETURNING id, public_id, created_at, updated_at`,
		p.OrgID, p.EmployeeID, p.Title, p.Objective, p.TargetDate, p.Status, p.CreatedBy,
	).Scan(&p.ID, &p.PublicID, &p.CreatedAt, &p.UpdatedAt)
	if err != nil {
		return fmt.Errorf("succession: CreatePlan: %w", err)
	}
	return nil
}

func (r *repoImpl) UpdatePlan(ctx context.Context, p *DevelopmentPlan) error {
	_, err := r.db.Exec(ctx,
		`UPDATE hrm_development_plans
		    SET title=$3, objective=$4, target_date=$5, status=$6, completed_at=$7, updated_at=NOW()
		  WHERE org_id=$1 AND id=$2::uuid`,
		p.OrgID, p.ID, p.Title, p.Objective, p.TargetDate, p.Status, p.CompletedAt)
	if err != nil {
		return fmt.Errorf("succession: UpdatePlan: %w", err)
	}
	return nil
}

func (r *repoImpl) FindPlanByRef(ctx context.Context, orgID, ref string) (*DevelopmentPlan, error) {
	p, err := scanPlan(r.db.QueryRow(ctx,
		`SELECT `+planSel+` FROM hrm_development_plans
		  WHERE org_id=$1 AND (id::text=$2 OR public_id=$2)`, orgID, ref))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("succession: FindPlanByRef: %w", err)
	}
	return p, nil
}

// SubjectPlans is the employee's own read path.
//
// ⚠ THIS QUERY MUST NEVER JOIN hrm_talent_assessments OR
// hrm_succession_candidates. The subject's confidentiality is the fact that
// their query cannot reach those tables — not a filter applied to a wider
// result. Adding a join here would leak a 9-box placement or a nomination
// through a path that no permission check covers, because the caller is
// legitimately allowed to read everything this method returns.
func (r *repoImpl) SubjectPlans(ctx context.Context, orgID, employeeID string) ([]*DevelopmentPlan, error) {
	return r.loadPlansWithItems(ctx,
		`SELECT `+planSel+` FROM hrm_development_plans
		  WHERE org_id=$1 AND employee_id=$2::uuid
		    AND status <> 'draft'
		  ORDER BY created_at DESC`, orgID, employeeID)
}

// ListPlans is the manager/HR path. Same tables, but drafts included: an
// author needs to see a plan before it is shared with its subject.
func (r *repoImpl) ListPlans(ctx context.Context, orgID, employeeID string) ([]*DevelopmentPlan, error) {
	if strings.TrimSpace(employeeID) == "" {
		return r.loadPlansWithItems(ctx,
			`SELECT `+planSel+` FROM hrm_development_plans
			  WHERE org_id=$1 ORDER BY created_at DESC`, orgID)
	}
	return r.loadPlansWithItems(ctx,
		`SELECT `+planSel+` FROM hrm_development_plans
		  WHERE org_id=$1 AND employee_id=$2::uuid ORDER BY created_at DESC`, orgID, employeeID)
}

func (r *repoImpl) loadPlansWithItems(ctx context.Context, q string, args ...any) ([]*DevelopmentPlan, error) {
	rows, err := r.db.Query(ctx, q, args...)
	if err != nil {
		return nil, fmt.Errorf("succession: loadPlans: %w", err)
	}
	defer rows.Close()
	plans := []*DevelopmentPlan{}
	byID := map[string]*DevelopmentPlan{}
	ids := []string{}
	for rows.Next() {
		p, err := scanPlan(rows)
		if err != nil {
			return nil, fmt.Errorf("succession: loadPlans: scan: %w", err)
		}
		p.Items = []*PlanItem{}
		plans = append(plans, p)
		byID[p.ID] = p
		ids = append(ids, p.ID)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if len(ids) == 0 {
		return plans, nil
	}

	irows, err := r.db.Query(ctx,
		`SELECT `+itemSel+` FROM hrm_development_plan_items
		  WHERE plan_id = ANY($1::uuid[]) ORDER BY sort_order, created_at`, ids)
	if err != nil {
		return nil, fmt.Errorf("succession: loadPlans: items: %w", err)
	}
	defer irows.Close()
	for irows.Next() {
		it, err := scanItem(irows)
		if err != nil {
			return nil, fmt.Errorf("succession: loadPlans: item scan: %w", err)
		}
		if p := byID[it.PlanID]; p != nil {
			p.Items = append(p.Items, it)
		}
	}
	return plans, irows.Err()
}

func (r *repoImpl) CreateItem(ctx context.Context, it *PlanItem) error {
	err := r.db.QueryRow(ctx,
		`INSERT INTO hrm_development_plan_items
		   (org_id, plan_id, description, target_date, sort_order, created_by)
		 VALUES ($1,$2::uuid,$3,$4,$5,$6)
		 RETURNING id, public_id, status, created_at, updated_at`,
		it.OrgID, it.PlanID, it.Description, it.TargetDate, it.SortOrder, it.CreatedBy,
	).Scan(&it.ID, &it.PublicID, &it.Status, &it.CreatedAt, &it.UpdatedAt)
	if err != nil {
		return fmt.Errorf("succession: CreateItem: %w", err)
	}
	return nil
}

func (r *repoImpl) UpdateItem(ctx context.Context, it *PlanItem) error {
	_, err := r.db.Exec(ctx,
		`UPDATE hrm_development_plan_items
		    SET description=$3, target_date=$4, status=$5, completed_at=$6,
		        sort_order=$7, updated_at=NOW()
		  WHERE org_id=$1 AND id=$2::uuid`,
		it.OrgID, it.ID, it.Description, it.TargetDate, it.Status, it.CompletedAt, it.SortOrder)
	if err != nil {
		return fmt.Errorf("succession: UpdateItem: %w", err)
	}
	return nil
}

func (r *repoImpl) FindItemByRef(ctx context.Context, orgID, ref string) (*PlanItem, error) {
	it, err := scanItem(r.db.QueryRow(ctx,
		`SELECT `+itemSel+` FROM hrm_development_plan_items
		  WHERE org_id=$1 AND (id::text=$2 OR public_id=$2)`, orgID, ref))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("succession: FindItemByRef: %w", err)
	}
	return it, nil
}

// ── flight-risk inputs ───────────────────────────────────────────────────────
//
// Each method below reads a plain fact from a table that already exists. No
// signal is stored: a stored signal goes stale the moment a promotion, pay
// revision, reporting change or appraisal lands, and nothing would detect it.

func (r *repoImpl) EmployeeTimeline(ctx context.Context, orgID, employeeID string) (time.Time, *time.Time, string, error) {
	var hire time.Time
	var lastPromo *time.Time
	var name string
	err := r.db.QueryRow(ctx,
		`SELECT e.hire_date,
		        (SELECT max(p.effective_date) FROM hrm_promotions p
		          WHERE p.org_id = e.org_id AND p.employee_id = e.id
		            AND p.applied_at IS NOT NULL),
		        btrim(e.first_name || ' ' || COALESCE(e.last_name,''))
		   FROM hrm_employees e WHERE e.org_id=$1 AND e.id=$2::uuid`,
		orgID, employeeID).Scan(&hire, &lastPromo, &name)
	if errors.Is(err, pgx.ErrNoRows) {
		return time.Time{}, nil, "", ErrEmployeeNotFound
	}
	if err != nil {
		return time.Time{}, nil, "", fmt.Errorf("succession: EmployeeTimeline: %w", err)
	}
	return hire, lastPromo, name, nil
}

// CurrentPayAndBand walks employee -> latest salary record -> salary
// structure's grade -> the compensation band effective as of the date. Same
// chain compensation.buildContext uses, so a below-band signal and a merit
// calculation cannot disagree about which band somebody is in.
func (r *repoImpl) CurrentPayAndBand(ctx context.Context, orgID, employeeID string, asOf time.Time) (decimal.Decimal, decimal.Decimal, string, error) {
	var basic, bandMin decimal.Decimal
	var grade *string
	err := r.db.QueryRow(ctx,
		`SELECT COALESCE(es.basic_pay,0), COALESCE(b.min_amount,0), ss.grade_label
		   FROM hrm_employees e
		   LEFT JOIN LATERAL (
		       SELECT structure_id, basic_pay FROM hrm_employee_salary_records
		        WHERE employee_id = e.id AND effective_date <= $3
		        ORDER BY effective_date DESC LIMIT 1
		   ) es ON TRUE
		   LEFT JOIN hrm_salary_structures ss ON ss.id = es.structure_id
		   LEFT JOIN LATERAL (
		       SELECT min_amount FROM hrm_compensation_bands
		        WHERE org_id = e.org_id AND grade_label = ss.grade_label
		          AND effective_date <= $3
		        ORDER BY effective_date DESC LIMIT 1
		   ) b ON TRUE
		  WHERE e.org_id=$1 AND e.id=$2::uuid`,
		orgID, employeeID, asOf).Scan(&basic, &bandMin, &grade)
	if errors.Is(err, pgx.ErrNoRows) {
		return decimal.Zero, decimal.Zero, "", ErrEmployeeNotFound
	}
	if err != nil {
		return decimal.Zero, decimal.Zero, "", fmt.Errorf("succession: CurrentPayAndBand: %w", err)
	}
	g := ""
	if grade != nil {
		g = *grade
	}
	return basic, bandMin, g, nil
}

// ManagerChangesSince counts SOLID-line relationships started in the window.
// Matrix lines are excluded: a project lead changing is not the relationship
// churn this signal is about, and counting them would make the signal fire on
// ordinary project rotation.
func (r *repoImpl) ManagerChangesSince(ctx context.Context, orgID, employeeID string, since time.Time) (int, error) {
	var n int
	err := r.db.QueryRow(ctx,
		`SELECT count(*) FROM hrm_reporting_relationships
		  WHERE org_id=$1 AND employee_id=$2::uuid
		    AND relationship_type='solid' AND effective_from >= $3`,
		orgID, employeeID, since).Scan(&n)
	if err != nil {
		return 0, fmt.Errorf("succession: ManagerChangesSince: %w", err)
	}
	return n, nil
}
