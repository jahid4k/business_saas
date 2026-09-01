// backend/internal/hrm/analytics/repository.go
package analytics

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Repository is analytics' data layer.
//
// ⚠ THE READ METHODS AND THE JOB METHODS TOUCH DIFFERENT TABLES, AND THAT
// SEPARATION IS THE SLICE'S CENTRAL RULE.
//
// Everything under "read path" queries hrm_headcount_snapshots and
// hrm_attrition_facts ONLY. Everything under "job path" is the only code in
// this package permitted to read OLTP (hrm_employees, hrm_exits,
// hrm_terminations, hrm_rehire_eligibility, salary records). A read method
// that reached into hrm_employees would make the metric change under a
// reader's feet, and would silently rewrite history whenever somebody
// corrected an old record. An integration test mutates OLTP and asserts the
// metric does not move until the job has run.
type Repository interface {
	// ── Metric definitions ──
	CreateMetric(ctx context.Context, m *MetricDefinition) error
	UpdateMetric(ctx context.Context, m *MetricDefinition) error
	FindMetricByRef(ctx context.Context, orgID, ref string) (*MetricDefinition, error)
	FindMetricByKey(ctx context.Context, orgID, key string) (*MetricDefinition, error)
	ListMetrics(ctx context.Context, orgID string, activeOnly bool) ([]*MetricDefinition, error)

	// ── Read path: FACT TABLES ONLY ──
	ListSnapshots(ctx context.Context, orgID string, from, to time.Time, dimension Grain) ([]*HeadcountSnapshot, error)
	LatestSnapshotOnOrBefore(ctx context.Context, orgID string, on time.Time, dimension Grain, dimensionID *string) (*HeadcountSnapshot, error)
	AttritionFactsBetween(ctx context.Context, orgID string, from, to time.Time) ([]*AttritionFact, error)
	GenderDistributionFromFacts(ctx context.Context, orgID string, from, to time.Time) ([]Group, error)
	CohortRows(ctx context.Context, orgID string, from, to time.Time) ([]*CohortRow, error)

	// ── Job path: the ONLY OLTP readers in this package ──
	OrgIDsWithEmployees(ctx context.Context) ([]string, error)
	BuildAttritionFacts(ctx context.Context, orgID string, on time.Time) (int, error)
	BuildHeadcountSnapshot(ctx context.Context, orgID string, on time.Time, threshold int) (int, error)
	// GenderDistributionLive backs the DEI headcount view. It reads
	// hrm_employees because current composition is a present-tense question
	// with no useful snapshot equivalent; it returns COUNTS ONLY, never rows,
	// and its result always goes through Suppress.
	GenderDistributionLive(ctx context.Context, orgID string) ([]Group, error)
}

type repoImpl struct{ db *pgxpool.Pool }

func NewRepository(db *pgxpool.Pool) Repository { return &repoImpl{db: db} }

// ── metric definitions ───────────────────────────────────────────────────────

const metricSel = `id::text, public_id, org_id::text, metric_key, name, description,
	computation, formula_statement, grain, attrition_types, include_probation_exits,
	suppression_threshold, is_active, created_by::text, created_at, updated_at`

func scanMetric(row pgx.Row) (*MetricDefinition, error) {
	m := &MetricDefinition{}
	err := row.Scan(&m.ID, &m.PublicID, &m.OrgID, &m.MetricKey, &m.Name, &m.Description,
		&m.Computation, &m.FormulaStatement, &m.Grain, &m.AttritionTypes,
		&m.IncludeProbationExits, &m.SuppressionThreshold, &m.IsActive,
		&m.CreatedBy, &m.CreatedAt, &m.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return m, nil
}

func (r *repoImpl) CreateMetric(ctx context.Context, m *MetricDefinition) error {
	err := r.db.QueryRow(ctx,
		`INSERT INTO hrm_metric_definitions
		   (org_id, metric_key, name, description, computation, formula_statement, grain,
		    attrition_types, include_probation_exits, suppression_threshold, created_by)
		 VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11)
		 RETURNING id, public_id, is_active, created_at, updated_at`,
		m.OrgID, m.MetricKey, m.Name, m.Description, m.Computation, m.FormulaStatement,
		m.Grain, m.AttritionTypes, m.IncludeProbationExits, m.SuppressionThreshold, m.CreatedBy,
	).Scan(&m.ID, &m.PublicID, &m.IsActive, &m.CreatedAt, &m.UpdatedAt)
	if err != nil {
		if strings.Contains(err.Error(), "uq_hrm_metric_org_key") {
			return ErrDuplicateMetric
		}
		return fmt.Errorf("analytics: CreateMetric: %w", err)
	}
	return nil
}

func (r *repoImpl) UpdateMetric(ctx context.Context, m *MetricDefinition) error {
	_, err := r.db.Exec(ctx,
		`UPDATE hrm_metric_definitions
		    SET name=$3, description=$4, formula_statement=$5, grain=$6, attrition_types=$7,
		        include_probation_exits=$8, suppression_threshold=$9, is_active=$10, updated_at=NOW()
		  WHERE org_id=$1 AND id=$2::uuid`,
		m.OrgID, m.ID, m.Name, m.Description, m.FormulaStatement, m.Grain, m.AttritionTypes,
		m.IncludeProbationExits, m.SuppressionThreshold, m.IsActive)
	if err != nil {
		return fmt.Errorf("analytics: UpdateMetric: %w", err)
	}
	return nil
}

func (r *repoImpl) FindMetricByRef(ctx context.Context, orgID, ref string) (*MetricDefinition, error) {
	m, err := scanMetric(r.db.QueryRow(ctx,
		`SELECT `+metricSel+` FROM hrm_metric_definitions
		  WHERE org_id=$1 AND (id::text=$2 OR public_id=$2)`, orgID, ref))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("analytics: FindMetricByRef: %w", err)
	}
	return m, nil
}

func (r *repoImpl) FindMetricByKey(ctx context.Context, orgID, key string) (*MetricDefinition, error) {
	m, err := scanMetric(r.db.QueryRow(ctx,
		`SELECT `+metricSel+` FROM hrm_metric_definitions
		  WHERE org_id=$1 AND metric_key=$2`, orgID, key))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("analytics: FindMetricByKey: %w", err)
	}
	return m, nil
}

func (r *repoImpl) ListMetrics(ctx context.Context, orgID string, activeOnly bool) ([]*MetricDefinition, error) {
	q := `SELECT ` + metricSel + ` FROM hrm_metric_definitions WHERE org_id=$1`
	if activeOnly {
		q += ` AND is_active`
	}
	q += ` ORDER BY metric_key`
	rows, err := r.db.Query(ctx, q, orgID)
	if err != nil {
		return nil, fmt.Errorf("analytics: ListMetrics: %w", err)
	}
	defer rows.Close()
	out := []*MetricDefinition{}
	for rows.Next() {
		m, err := scanMetric(rows)
		if err != nil {
			return nil, fmt.Errorf("analytics: ListMetrics: scan: %w", err)
		}
		out = append(out, m)
	}
	return out, rows.Err()
}

// ── read path — FACT TABLES ONLY ─────────────────────────────────────────────

const snapshotSel = `s.id::text, s.public_id, s.org_id::text, s.snapshot_date, s.dimension,
	s.dimension_id::text, s.headcount, s.joiners, s.leavers, s.voluntary_leavers,
	s.involuntary_leavers, s.regretted_leavers, s.avg_tenure_days,
	s.comp_p25, s.comp_median, s.comp_p75, s.comp_currency, s.computed_at,
	COALESCE(d.name, '')`

func scanSnapshot(row pgx.Row) (*HeadcountSnapshot, error) {
	s := &HeadcountSnapshot{}
	err := row.Scan(&s.ID, &s.PublicID, &s.OrgID, &s.SnapshotDate, &s.Dimension, &s.DimensionID,
		&s.Headcount, &s.Joiners, &s.Leavers, &s.VoluntaryLeavers, &s.InvoluntaryLeavers,
		&s.RegrettedLeavers, &s.AvgTenureDays, &s.CompP25, &s.CompMedian, &s.CompP75,
		&s.CompCurrency, &s.ComputedAt, &s.DimensionLabel)
	if err != nil {
		return nil, err
	}
	return s, nil
}

// The department join is a LABEL lookup, not an aggregation. It reads a name
// for display and nothing that could change a number.
const snapshotFrom = ` FROM hrm_headcount_snapshots s
	LEFT JOIN hrm_departments d ON d.id = s.dimension_id`

func (r *repoImpl) ListSnapshots(ctx context.Context, orgID string, from, to time.Time, dimension Grain) ([]*HeadcountSnapshot, error) {
	rows, err := r.db.Query(ctx,
		`SELECT `+snapshotSel+snapshotFrom+`
		  WHERE s.org_id=$1 AND s.snapshot_date BETWEEN $2 AND $3 AND s.dimension=$4
		  ORDER BY s.snapshot_date, s.dimension_id NULLS FIRST`,
		orgID, from, to, dimension)
	if err != nil {
		return nil, fmt.Errorf("analytics: ListSnapshots: %w", err)
	}
	defer rows.Close()
	out := []*HeadcountSnapshot{}
	for rows.Next() {
		s, err := scanSnapshot(rows)
		if err != nil {
			return nil, fmt.Errorf("analytics: ListSnapshots: scan: %w", err)
		}
		out = append(out, s)
	}
	return out, rows.Err()
}

// LatestSnapshotOnOrBefore is the effective-dated MAX(date) <= asOf shape
// (the SlabsAsOf pattern), so an opening headcount is the last snapshot taken
// before the period rather than an interpolation.
func (r *repoImpl) LatestSnapshotOnOrBefore(ctx context.Context, orgID string, on time.Time, dimension Grain, dimensionID *string) (*HeadcountSnapshot, error) {
	q := `SELECT ` + snapshotSel + snapshotFrom + `
	       WHERE s.org_id=$1 AND s.snapshot_date <= $2 AND s.dimension=$3`
	args := []any{orgID, on, dimension}
	if dimensionID == nil {
		q += ` AND s.dimension_id IS NULL`
	} else {
		q += ` AND s.dimension_id=$4::uuid`
		args = append(args, *dimensionID)
	}
	q += ` ORDER BY s.snapshot_date DESC LIMIT 1`

	s, err := scanSnapshot(r.db.QueryRow(ctx, q, args...))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("analytics: LatestSnapshotOnOrBefore: %w", err)
	}
	return s, nil
}

const factSel = `id::text, public_id, org_id::text, employee_id::text, exit_id::text,
	exit_date, hire_date, cohort_month, tenure_days, is_first_year, source_type,
	termination_type, is_voluntary, is_regretted, department_id::text, position_id::text,
	legal_entity_id::text, gender, computed_at`

func scanFact(row pgx.Row) (*AttritionFact, error) {
	f := &AttritionFact{}
	err := row.Scan(&f.ID, &f.PublicID, &f.OrgID, &f.EmployeeID, &f.ExitID, &f.ExitDate,
		&f.HireDate, &f.CohortMonth, &f.TenureDays, &f.IsFirstYear, &f.SourceType,
		&f.TerminationType, &f.IsVoluntary, &f.IsRegretted, &f.DepartmentID, &f.PositionID,
		&f.LegalEntityID, &f.Gender, &f.ComputedAt)
	if err != nil {
		return nil, err
	}
	return f, nil
}

func (r *repoImpl) AttritionFactsBetween(ctx context.Context, orgID string, from, to time.Time) ([]*AttritionFact, error) {
	rows, err := r.db.Query(ctx,
		`SELECT `+factSel+` FROM hrm_attrition_facts
		  WHERE org_id=$1 AND exit_date BETWEEN $2 AND $3
		  ORDER BY exit_date`, orgID, from, to)
	if err != nil {
		return nil, fmt.Errorf("analytics: AttritionFactsBetween: %w", err)
	}
	defer rows.Close()
	out := []*AttritionFact{}
	for rows.Next() {
		f, err := scanFact(rows)
		if err != nil {
			return nil, fmt.Errorf("analytics: AttritionFactsBetween: scan: %w", err)
		}
		out = append(out, f)
	}
	return out, rows.Err()
}

// GenderDistributionFromFacts is the DEI view of LEAVERS, which is the
// question that actually matters — whether the organization loses one group
// faster than another. Counts only.
func (r *repoImpl) GenderDistributionFromFacts(ctx context.Context, orgID string, from, to time.Time) ([]Group, error) {
	rows, err := r.db.Query(ctx,
		`SELECT COALESCE(gender,'unspecified'), count(*)::int
		   FROM hrm_attrition_facts
		  WHERE org_id=$1 AND exit_date BETWEEN $2 AND $3
		  GROUP BY 1 ORDER BY 1`, orgID, from, to)
	if err != nil {
		return nil, fmt.Errorf("analytics: GenderDistributionFromFacts: %w", err)
	}
	defer rows.Close()
	return scanGroups(rows)
}

func scanGroups(rows pgx.Rows) ([]Group, error) {
	out := []Group{}
	for rows.Next() {
		var g Group
		if err := rows.Scan(&g.Key, &g.Count); err != nil {
			return nil, fmt.Errorf("analytics: scan group: %w", err)
		}
		out = append(out, g)
	}
	return out, rows.Err()
}

// CohortRows computes retention per hire cohort.
//
// The cohort SIZE comes from the facts plus the employees still present, but
// only the exits are read from the fact table; "still active" is a
// present-tense count and has no historical meaning. Retention is computed in
// Go by CohortRetention so the arithmetic has one implementation.
func (r *repoImpl) CohortRows(ctx context.Context, orgID string, from, to time.Time) ([]*CohortRow, error) {
	rows, err := r.db.Query(ctx,
		`WITH cohorts AS (
		     SELECT date_trunc('month', e.hire_date)::date AS cohort_month,
		            count(*)::int AS cohort_size,
		            count(*) FILTER (
		                WHERE NOT EXISTS (
		                    SELECT 1 FROM hrm_attrition_facts f
		                     WHERE f.org_id = e.org_id AND f.employee_id = e.id
		                )
		            )::int AS still_active
		       FROM hrm_employees e
		      WHERE e.org_id = $1 AND e.hire_date BETWEEN $2 AND $3
		      GROUP BY 1
		 )
		 SELECT cohort_month, cohort_size, still_active FROM cohorts ORDER BY cohort_month`,
		orgID, from, to)
	if err != nil {
		return nil, fmt.Errorf("analytics: CohortRows: %w", err)
	}
	defer rows.Close()
	out := []*CohortRow{}
	for rows.Next() {
		c := &CohortRow{}
		if err := rows.Scan(&c.CohortMonth, &c.CohortSize, &c.StillActive); err != nil {
			return nil, fmt.Errorf("analytics: CohortRows: scan: %w", err)
		}
		if pct, ok := CohortRetention(c.CohortSize, c.StillActive); ok {
			c.Retention = &pct
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

// ── job path — the ONLY OLTP readers ─────────────────────────────────────────

func (r *repoImpl) OrgIDsWithEmployees(ctx context.Context) ([]string, error) {
	rows, err := r.db.Query(ctx, `SELECT DISTINCT org_id::text FROM hrm_employees`)
	if err != nil {
		return nil, fmt.Errorf("analytics: OrgIDsWithEmployees: %w", err)
	}
	defer rows.Close()
	out := []string{}
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("analytics: OrgIDsWithEmployees: scan: %w", err)
		}
		out = append(out, id)
	}
	return out, rows.Err()
}

// BuildAttritionFacts materialises one row per completed exit.
//
// The four-way classification is done HERE, once, at build time:
//
//   - voluntary/involuntary from source_type and termination_type
//     (analytics.IsVoluntaryExit is the Go statement of the same rule and is
//     what the unit tests pin; this SQL must agree with it).
//   - regretted/non-regretted from Phase 9's hrm_rehire_eligibility.status,
//     which 9A gave its first reader. ⚠ 'conditional' and a missing row both
//     yield NULL — unknown — rather than false.
//
// ON CONFLICT DO UPDATE rather than DO NOTHING: a rehire decision recorded
// after the exit must be able to fill in a NULL is_regretted on the next run.
// The row is otherwise immutable in practice because its inputs do not move.
func (r *repoImpl) BuildAttritionFacts(ctx context.Context, orgID string, on time.Time) (int, error) {
	ct, err := r.db.Exec(ctx,
		`INSERT INTO hrm_attrition_facts
		   (org_id, employee_id, exit_id, exit_date, hire_date, cohort_month, tenure_days,
		    is_first_year, source_type, termination_type, is_voluntary, is_regretted,
		    department_id, position_id, legal_entity_id, gender)
		 SELECT x.org_id, x.employee_id, x.id, x.last_working_date, e.hire_date,
		        date_trunc('month', e.hire_date)::date,
		        GREATEST((x.last_working_date - e.hire_date), 0),
		        GREATEST((x.last_working_date - e.hire_date), 0) < $3,
		        x.source_type,
		        t.termination_type,
		        CASE
		          WHEN x.source_type = 'resignation' THEN TRUE
		          WHEN t.termination_type IN ('voluntary', 'retirement') THEN TRUE
		          ELSE FALSE
		        END,
		        CASE re.status
		          WHEN 'eligible'     THEN TRUE
		          WHEN 'not_eligible' THEN FALSE
		          ELSE NULL
		        END,
		        e.department_id, e.position_id, e.legal_entity_id, e.gender
		   FROM hrm_exits x
		   JOIN hrm_employees e ON e.id = x.employee_id
		   LEFT JOIN hrm_terminations t
		          ON x.source_type = 'termination' AND t.id = x.source_id
		   LEFT JOIN hrm_rehire_eligibility re ON re.exit_id = x.id
		  WHERE x.org_id = $1
		    AND x.status <> 'cancelled'
		    AND x.last_working_date IS NOT NULL
		    AND x.last_working_date <= $2
		 ON CONFLICT (org_id, employee_id, exit_date) DO UPDATE SET
		    is_regretted = EXCLUDED.is_regretted,
		    termination_type = EXCLUDED.termination_type,
		    is_voluntary = EXCLUDED.is_voluntary,
		    computed_at = NOW()`,
		orgID, on, FirstYearDays)
	if err != nil {
		return 0, fmt.Errorf("analytics: BuildAttritionFacts: %w", err)
	}
	return int(ct.RowsAffected()), nil
}

// BuildHeadcountSnapshot writes the org total plus one row per department.
//
// ⚠ The compensation percentiles are written as NULL whenever the group is
// below the suppression threshold. Withholding them at WRITE time rather than
// at read time means a small team's pay distribution is never recorded, so no
// future query, export or backup can expose it.
func (r *repoImpl) BuildHeadcountSnapshot(ctx context.Context, orgID string, on time.Time, threshold int) (int, error) {
	if threshold < MinThreshold {
		threshold = MinThreshold
	}
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return 0, fmt.Errorf("analytics: BuildHeadcountSnapshot: begin: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	total := 0
	for _, spec := range []struct {
		dimension string
		groupBy   string
	}{
		{"org", `NULL::uuid`},
		{"department", `e.department_id`},
		// ⚠ 11B-2: the legal_entity dimension has always been permitted by
		// chk_hrm_hcsnap_dimension, but the job never populated it. An org
		// with no entities produces no rows here at all, because the non-org
		// branch excludes NULL members — so this adds nothing for the
		// organizations that have none, which is all of them today.
		{"legal_entity", `e.legal_entity_id`},
	} {
		ct, err := tx.Exec(ctx, snapshotUpsertSQL(spec.dimension, spec.groupBy),
			orgID, on, threshold)
		if err != nil {
			return 0, fmt.Errorf("analytics: BuildHeadcountSnapshot(%s): %w", spec.dimension, err)
		}
		total += int(ct.RowsAffected())
	}
	if err := tx.Commit(ctx); err != nil {
		return 0, fmt.Errorf("analytics: BuildHeadcountSnapshot: commit: %w", err)
	}
	return total, nil
}

// snapshotUpsertSQL builds the aggregation for one dimension.
//
// The dimension expression is interpolated from a CLOSED SET defined directly
// above in BuildHeadcountSnapshot — it never comes from a request — while
// org, date and threshold stay bound parameters.
func snapshotUpsertSQL(dimension, groupExpr string) string {
	where := ``
	if dimension != "org" {
		// A row for employees with no department — or no legal entity —
		// would be keyed on NULL, which the unique index treats as its own
		// value and chk_hrm_hcsnap_dim_id forbids outright.
		//
		// ⚠ The guard must name the DIMENSION'S OWN column. Leaving it
		// hardcoded to department_id once legal_entity was added would have
		// admitted every employee with no entity into the legal_entity
		// snapshot and then failed the CHECK.
		where = ` AND ` + groupExpr + ` IS NOT NULL`
	}
	return `
WITH pop AS (
    SELECT ` + groupExpr + ` AS dim_id, e.id, e.hire_date, e.termination_date,
           (SELECT es.basic_pay FROM hrm_employee_salary_records es
             WHERE es.employee_id = e.id AND es.effective_date <= $2
             ORDER BY es.effective_date DESC LIMIT 1) AS basic_pay,
           (SELECT es.currency FROM hrm_employee_salary_records es
             WHERE es.employee_id = e.id AND es.effective_date <= $2
             ORDER BY es.effective_date DESC LIMIT 1) AS currency
      FROM hrm_employees e
     WHERE e.org_id = $1
       AND e.hire_date <= $2
       AND (e.termination_date IS NULL OR e.termination_date > $2)` + where + `
),
leavers AS (
    SELECT ` + strings.ReplaceAll(groupExpr, "e.", "f.") + ` AS dim_id,
           count(*)::int AS leavers,
           count(*) FILTER (WHERE f.is_voluntary)::int AS voluntary,
           count(*) FILTER (WHERE NOT f.is_voluntary)::int AS involuntary,
           count(*) FILTER (WHERE f.is_regretted)::int AS regretted
      FROM hrm_attrition_facts f
     WHERE f.org_id = $1 AND f.exit_date > ($2::date - INTERVAL '1 month') AND f.exit_date <= $2
     GROUP BY 1
),
joiners AS (
    SELECT ` + groupExpr + ` AS dim_id, count(*)::int AS joiners
      FROM hrm_employees e
     WHERE e.org_id = $1 AND e.hire_date > ($2::date - INTERVAL '1 month') AND e.hire_date <= $2` + where + `
     GROUP BY 1
),
agg AS (
    SELECT p.dim_id,
           count(*)::int AS headcount,
           COALESCE(avg($2::date - p.hire_date), 0)::int AS avg_tenure_days,
           percentile_cont(0.25) WITHIN GROUP (ORDER BY p.basic_pay) AS p25,
           percentile_cont(0.50) WITHIN GROUP (ORDER BY p.basic_pay) AS median,
           percentile_cont(0.75) WITHIN GROUP (ORDER BY p.basic_pay) AS p75,
           min(p.currency) AS currency
      FROM pop p GROUP BY p.dim_id
)
INSERT INTO hrm_headcount_snapshots
    (org_id, snapshot_date, dimension, dimension_id, headcount, joiners, leavers,
     voluntary_leavers, involuntary_leavers, regretted_leavers, avg_tenure_days,
     comp_p25, comp_median, comp_p75, comp_currency, computed_at)
SELECT $1, $2, '` + dimension + `', a.dim_id, a.headcount,
       COALESCE(j.joiners, 0), COALESCE(l.leavers, 0),
       COALESCE(l.voluntary, 0), COALESCE(l.involuntary, 0), COALESCE(l.regretted, 0),
       a.avg_tenure_days,
       CASE WHEN a.headcount >= $3 THEN ROUND(a.p25::numeric, 2)    END,
       CASE WHEN a.headcount >= $3 THEN ROUND(a.median::numeric, 2) END,
       CASE WHEN a.headcount >= $3 THEN ROUND(a.p75::numeric, 2)    END,
       CASE WHEN a.headcount >= $3 THEN a.currency END,
       NOW()
  FROM agg a
  LEFT JOIN leavers l ON l.dim_id IS NOT DISTINCT FROM a.dim_id
  LEFT JOIN joiners j ON j.dim_id IS NOT DISTINCT FROM a.dim_id
ON CONFLICT (org_id, snapshot_date, dimension,
             COALESCE(dimension_id, '00000000-0000-0000-0000-000000000000'::uuid))
DO UPDATE SET
    headcount = EXCLUDED.headcount,
    joiners = EXCLUDED.joiners,
    leavers = EXCLUDED.leavers,
    voluntary_leavers = EXCLUDED.voluntary_leavers,
    involuntary_leavers = EXCLUDED.involuntary_leavers,
    regretted_leavers = EXCLUDED.regretted_leavers,
    avg_tenure_days = EXCLUDED.avg_tenure_days,
    comp_p25 = EXCLUDED.comp_p25,
    comp_median = EXCLUDED.comp_median,
    comp_p75 = EXCLUDED.comp_p75,
    comp_currency = EXCLUDED.comp_currency,
    computed_at = NOW()`
}

// GenderDistributionLive counts the CURRENT population by gender.
//
// ⚠ This is the one OLTP read the DEI view makes, and it is deliberate:
// "what is the composition of this organization today" is a present-tense
// question, and answering it from last night's snapshot would be a worse
// answer, not a purer one. It returns COUNTS, never rows, and every caller
// puts the result through Suppress before it reaches a response.
func (r *repoImpl) GenderDistributionLive(ctx context.Context, orgID string) ([]Group, error) {
	rows, err := r.db.Query(ctx,
		`SELECT COALESCE(gender,'unspecified'), count(*)::int
		   FROM hrm_employees
		  WHERE org_id=$1 AND termination_date IS NULL
		  GROUP BY 1 ORDER BY 1`, orgID)
	if err != nil {
		return nil, fmt.Errorf("analytics: GenderDistributionLive: %w", err)
	}
	defer rows.Close()
	return scanGroups(rows)
}
