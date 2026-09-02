// backend/internal/hrm/analytics/service.go
package analytics

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"
)

// Caller carries the four read authorities the route gate established.
//
// They are separate because the DATA IS DIFFERENT IN KIND, not because it
// sits on one sensitivity scale: headcount trends are management
// information, pay distribution can often be resolved to an individual on a
// small team, demographic composition is aggregate-only always, and bulk
// extract is a different act from reading a chart.
type Caller struct {
	UserID              string
	CanView             bool
	CanViewCompensation bool
	CanViewDEI          bool
	CanExport           bool
	CanManage           bool
}

// Service is analytics' business layer.
type Service interface {
	// Definitions
	CreateMetric(ctx context.Context, orgID string, caller Caller, req CreateMetricRequest) (*MetricDefinition, error)
	UpdateMetric(ctx context.Context, orgID string, caller Caller, ref string, req UpdateMetricRequest) (*MetricDefinition, error)
	ListMetrics(ctx context.Context, orgID string, caller Caller, activeOnly bool) ([]*MetricDefinition, error)

	// Read path — fact tables only
	Headcount(ctx context.Context, orgID string, caller Caller, from, to time.Time, grain Grain) ([]*HeadcountSnapshot, error)
	Attrition(ctx context.Context, orgID string, caller Caller, from, to time.Time) (*AttritionSummary, error)
	Cohorts(ctx context.Context, orgID string, caller Caller, from, to time.Time) ([]*CohortRow, error)
	Diversity(ctx context.Context, orgID string, caller Caller, from, to time.Time) (map[string]Distribution, error)
	Compensation(ctx context.Context, orgID string, caller Caller, on time.Time, grain Grain) ([]*CompensationBand, error)
	ExportAttrition(ctx context.Context, orgID string, caller Caller, from, to time.Time) (string, error)

	// Job
	RunSnapshot(ctx context.Context, on time.Time) (*SnapshotResult, error)
	RunSnapshotForOrg(ctx context.Context, orgID string, caller Caller, on time.Time) (*SnapshotResult, error)
}

type serviceImpl struct {
	repo Repository
	log  *slog.Logger
}

func NewService(repo Repository) Service {
	return &serviceImpl{repo: repo, log: slog.Default()}
}

// DefaultSuppressionThreshold applies when an org has recorded no DEI metric
// definition of its own. Five is the common statutory floor; an org may raise
// it, and chk_hrm_metric_threshold stops anyone lowering it below 2.
const DefaultSuppressionThreshold = 5

// ── definitions ──────────────────────────────────────────────────────────────

func (s *serviceImpl) CreateMetric(ctx context.Context, orgID string, caller Caller, req CreateMetricRequest) (*MetricDefinition, error) {
	if !caller.CanManage {
		return nil, ErrAccessDenied
	}
	key := strings.TrimSpace(req.MetricKey)
	name := strings.TrimSpace(req.Name)
	if key == "" || name == "" {
		return nil, ErrKeyRequired
	}
	comp := Computation(strings.TrimSpace(req.Computation))
	if !comp.IsValid() {
		return nil, ErrInvalidComputation
	}
	// ⚠ The statement is mandatory because it is the only thing a reader can
	// check the named computation against. It is never parsed.
	statement := strings.TrimSpace(req.FormulaStatement)
	if statement == "" {
		return nil, ErrStatementRequired
	}
	grain := Grain(strings.TrimSpace(req.Grain))
	if grain == "" {
		grain = GrainOrg
	}
	if !grain.IsValid() {
		return nil, ErrInvalidGrain
	}
	threshold := DefaultSuppressionThreshold
	if req.SuppressionThreshold != nil {
		threshold = *req.SuppressionThreshold
		if threshold < MinThreshold {
			return nil, ErrThresholdTooLow
		}
	}
	includeProbation := true
	if req.IncludeProbationExits != nil {
		includeProbation = *req.IncludeProbationExits
	}

	m := &MetricDefinition{
		OrgID: orgID, MetricKey: key, Name: name, Description: req.Description,
		Computation: comp, FormulaStatement: statement, Grain: grain,
		AttritionTypes: req.AttritionTypes, IncludeProbationExits: includeProbation,
		SuppressionThreshold: threshold, CreatedBy: caller.UserID,
	}
	if err := s.repo.CreateMetric(ctx, m); err != nil {
		return nil, err
	}
	return m, nil
}

func (s *serviceImpl) UpdateMetric(ctx context.Context, orgID string, caller Caller, ref string, req UpdateMetricRequest) (*MetricDefinition, error) {
	if !caller.CanManage {
		return nil, ErrAccessDenied
	}
	m, err := s.repo.FindMetricByRef(ctx, orgID, ref)
	if err != nil {
		return nil, err
	}
	if m == nil {
		return nil, ErrMetricNotFound
	}
	if req.Name != nil {
		if strings.TrimSpace(*req.Name) == "" {
			return nil, ErrKeyRequired
		}
		m.Name = strings.TrimSpace(*req.Name)
	}
	if req.Description != nil {
		m.Description = req.Description
	}
	if req.FormulaStatement != nil {
		if strings.TrimSpace(*req.FormulaStatement) == "" {
			return nil, ErrStatementRequired
		}
		m.FormulaStatement = strings.TrimSpace(*req.FormulaStatement)
	}
	if req.Grain != nil {
		g := Grain(strings.TrimSpace(*req.Grain))
		if !g.IsValid() {
			return nil, ErrInvalidGrain
		}
		m.Grain = g
	}
	if req.AttritionTypes != nil {
		m.AttritionTypes = req.AttritionTypes
	}
	if req.IncludeProbationExits != nil {
		m.IncludeProbationExits = *req.IncludeProbationExits
	}
	if req.SuppressionThreshold != nil {
		// ⚠ Refused rather than clamped. Somebody lowering a disclosure
		// threshold has to be told they cannot, not quietly overruled and
		// left believing the setting took.
		if *req.SuppressionThreshold < MinThreshold {
			return nil, ErrThresholdTooLow
		}
		m.SuppressionThreshold = *req.SuppressionThreshold
	}
	if req.IsActive != nil {
		m.IsActive = *req.IsActive
	}
	if err := s.repo.UpdateMetric(ctx, m); err != nil {
		return nil, err
	}
	return s.repo.FindMetricByRef(ctx, orgID, m.ID)
}

func (s *serviceImpl) ListMetrics(ctx context.Context, orgID string, caller Caller, activeOnly bool) ([]*MetricDefinition, error) {
	if !caller.CanView {
		return nil, ErrAccessDenied
	}
	return s.repo.ListMetrics(ctx, orgID, activeOnly)
}

// resolveThreshold reads the org's own DEI suppression threshold, falling
// back to the statutory-ish default. A missing definition never means "no
// suppression".
func (s *serviceImpl) resolveThreshold(ctx context.Context, orgID string) int {
	m, err := s.repo.FindMetricByKey(ctx, orgID, string(CompDEIDistribution))
	if err != nil || m == nil {
		return DefaultSuppressionThreshold
	}
	if m.SuppressionThreshold < MinThreshold {
		return MinThreshold
	}
	return m.SuppressionThreshold
}

// ── read path ────────────────────────────────────────────────────────────────

func validPeriod(from, to time.Time) error {
	if from.After(to) {
		return ErrInvalidPeriod
	}
	return nil
}

func (s *serviceImpl) Headcount(ctx context.Context, orgID string, caller Caller, from, to time.Time, grain Grain) ([]*HeadcountSnapshot, error) {
	if !caller.CanView {
		return nil, ErrAccessDenied
	}
	if err := validPeriod(from, to); err != nil {
		return nil, err
	}
	if !grain.IsValid() {
		return nil, ErrInvalidGrain
	}
	snaps, err := s.repo.ListSnapshots(ctx, orgID, from, to, grain)
	if err != nil {
		return nil, err
	}
	// ⚠ The compensation columns are stripped for a caller without
	// view_compensation. They are ALSO absent from the row entirely when the
	// group was below threshold at write time — this strip is the permission
	// half, not the disclosure half, and neither substitutes for the other.
	if !caller.CanViewCompensation {
		for _, sn := range snaps {
			sn.CompP25, sn.CompMedian, sn.CompP75, sn.CompCurrency = nil, nil, nil, nil
		}
	}
	return snaps, nil
}

// Attrition reads ONLY hrm_attrition_facts and hrm_headcount_snapshots.
//
// ⚠ It does not count terminations, resignations or employees. A live count
// would move under the reader between two refreshes of the same page and
// would rewrite history whenever an old record was corrected.
func (s *serviceImpl) Attrition(ctx context.Context, orgID string, caller Caller, from, to time.Time) (*AttritionSummary, error) {
	if !caller.CanView {
		return nil, ErrAccessDenied
	}
	if err := validPeriod(from, to); err != nil {
		return nil, err
	}
	facts, err := s.repo.AttritionFactsBetween(ctx, orgID, from, to)
	if err != nil {
		return nil, err
	}

	sum := &AttritionSummary{From: from, To: to, ByTerminationType: []Group{}}
	byType := map[string]int{}
	for _, f := range facts {
		sum.Leavers++
		if f.IsVoluntary {
			sum.Voluntary++
		} else {
			sum.Involuntary++
		}
		switch {
		case f.IsRegretted == nil:
			// ⚠ Reported as its own figure, never folded into non-regretted.
			// A regretted rate computed over a population where half the
			// exits were never reviewed is a different number from one where
			// all were, and the reader must be able to tell which they have.
			sum.RegrettedUnknown++
		case *f.IsRegretted:
			sum.Regretted++
		default:
			sum.NonRegretted++
		}
		if f.IsFirstYear {
			sum.FirstYearExits++
		}
		key := f.SourceType
		if f.TerminationType != nil && *f.TerminationType != "" {
			key = *f.TerminationType
		}
		byType[key]++
	}
	for k, v := range byType {
		sum.ByTerminationType = append(sum.ByTerminationType, Group{Key: k, Count: v})
	}

	opening, err := s.repo.LatestSnapshotOnOrBefore(ctx, orgID, from, GrainOrg, nil)
	if err != nil {
		return nil, err
	}
	closing, err := s.repo.LatestSnapshotOnOrBefore(ctx, orgID, to, GrainOrg, nil)
	if err != nil {
		return nil, err
	}
	if opening != nil {
		sum.OpeningHeadcount = opening.Headcount
	}
	if closing != nil {
		sum.ClosingHeadcount = closing.Headcount
	}
	sum.AverageHeadcount = AverageHeadcount(sum.OpeningHeadcount, sum.ClosingHeadcount)

	// ⚠ Left nil rather than defaulted to 0% when there is no denominator.
	// An organization with no snapshot did not achieve perfect retention.
	if rate, ok := AttritionRate(sum.Leavers, sum.AverageHeadcount); ok {
		sum.AttritionRate = &rate
	}
	if rate, ok := AttritionRate(sum.FirstYearExits, sum.AverageHeadcount); ok {
		sum.FirstYearRate = &rate
	}
	return sum, nil
}

func (s *serviceImpl) Cohorts(ctx context.Context, orgID string, caller Caller, from, to time.Time) ([]*CohortRow, error) {
	if !caller.CanView {
		return nil, ErrAccessDenied
	}
	if err := validPeriod(from, to); err != nil {
		return nil, err
	}
	return s.repo.CohortRows(ctx, orgID, from, to)
}

// Diversity returns aggregate composition and aggregate leaver composition,
// both suppressed.
//
// ⚠ EVERY RETURN PATH GOES THROUGH Suppress, AND view_dei DOES NOT LIFT IT.
// The permission decides whether a caller sees the breakdown at all; it never
// unlocks a cell. There is no branch here that returns raw groups, and no
// parameter that could make one.
func (s *serviceImpl) Diversity(ctx context.Context, orgID string, caller Caller, from, to time.Time) (map[string]Distribution, error) {
	if !caller.CanViewDEI {
		return nil, ErrAccessDenied
	}
	if err := validPeriod(from, to); err != nil {
		return nil, err
	}
	threshold := s.resolveThreshold(ctx, orgID)

	current, err := s.repo.GenderDistributionLive(ctx, orgID)
	if err != nil {
		return nil, err
	}
	leavers, err := s.repo.GenderDistributionFromFacts(ctx, orgID, from, to)
	if err != nil {
		return nil, err
	}
	return map[string]Distribution{
		"headcount_by_gender": Suppress(current, threshold),
		"leavers_by_gender":   Suppress(leavers, threshold),
	}, nil
}

// Compensation reads the pay distribution from the snapshot.
//
// The percentiles are already NULL for any group that was below the
// threshold when the job ran, so a suppressed band has no stored value here
// to expose. Suppressed is set from that absence rather than recomputed.
func (s *serviceImpl) Compensation(ctx context.Context, orgID string, caller Caller, on time.Time, grain Grain) ([]*CompensationBand, error) {
	if !caller.CanViewCompensation {
		return nil, ErrAccessDenied
	}
	if !grain.IsValid() {
		return nil, ErrInvalidGrain
	}
	snaps, err := s.repo.ListSnapshots(ctx, orgID, on, on, grain)
	if err != nil {
		return nil, err
	}
	out := make([]*CompensationBand, 0, len(snaps))
	for _, sn := range snaps {
		out = append(out, &CompensationBand{
			SnapshotDate: sn.SnapshotDate, Dimension: sn.Dimension, DimensionID: sn.DimensionID,
			DimensionLabel: sn.DimensionLabel, Headcount: sn.Headcount,
			P25: sn.CompP25, Median: sn.CompMedian, P75: sn.CompP75, Currency: sn.CompCurrency,
			Suppressed: sn.CompMedian == nil,
		})
	}
	return out, nil
}

// ExportAttrition renders the fact rows as CSV.
//
// ⚠ Gender is deliberately NOT a column. Export is row-level by nature, and a
// row-level extract carrying a demographic attribute is the exact thing
// "DEI is aggregate-only" forbids — it would hand a spreadsheet the
// suppression rule cannot reach. Demographic analysis stays on /diversity,
// where it is aggregated and suppressed.
func (s *serviceImpl) ExportAttrition(ctx context.Context, orgID string, caller Caller, from, to time.Time) (string, error) {
	if !caller.CanExport {
		return "", ErrAccessDenied
	}
	if err := validPeriod(from, to); err != nil {
		return "", err
	}
	facts, err := s.repo.AttritionFactsBetween(ctx, orgID, from, to)
	if err != nil {
		return "", err
	}
	var b strings.Builder
	b.WriteString("employee_id,exit_date,hire_date,cohort_month,tenure_days,is_first_year," +
		"source_type,termination_type,is_voluntary,is_regretted,department_id,legal_entity_id\n")
	for _, f := range facts {
		b.WriteString(fmt.Sprintf("%s,%s,%s,%s,%d,%t,%s,%s,%t,%s,%s,%s\n",
			f.EmployeeID,
			f.ExitDate.Format("2006-01-02"),
			f.HireDate.Format("2006-01-02"),
			f.CohortMonth.Format("2006-01-02"),
			f.TenureDays, f.IsFirstYear, f.SourceType,
			derefOr(f.TerminationType, ""), f.IsVoluntary,
			// "unknown" rather than an empty cell, so a spreadsheet does not
			// read a blank as false.
			boolOrUnknown(f.IsRegretted),
			derefOr(f.DepartmentID, ""), derefOr(f.LegalEntityID, "")))
	}
	return b.String(), nil
}

// ── job ──────────────────────────────────────────────────────────────────────

// RunSnapshot is the nightly job: build facts first, then snapshots.
//
// ⚠ THE ORDER MATTERS. The snapshot's leaver counts are read FROM the
// attrition facts, so building snapshots first would report a month with no
// leavers and then never correct it — the snapshot row for that date already
// exists and the next run writes a different date.
//
// Instance-wide, the benefits.activate_pending_enrollments shape: one run
// covers every org, and a failure in one org is logged and skipped rather
// than abandoning the rest.
func (s *serviceImpl) RunSnapshot(ctx context.Context, on time.Time) (*SnapshotResult, error) {
	orgIDs, err := s.repo.OrgIDsWithEmployees(ctx)
	if err != nil {
		return nil, err
	}
	res := &SnapshotResult{SnapshotDate: on}
	for _, orgID := range orgIDs {
		facts, err := s.repo.BuildAttritionFacts(ctx, orgID, on)
		if err != nil {
			s.log.Error("analytics: attrition fact build failed", "orgID", orgID, "error", err)
			continue
		}
		rows, err := s.repo.BuildHeadcountSnapshot(ctx, orgID, on, s.resolveThreshold(ctx, orgID))
		if err != nil {
			s.log.Error("analytics: headcount snapshot failed", "orgID", orgID, "error", err)
			continue
		}
		res.OrgsProcessed++
		res.FactsWritten += facts
		res.RowsWritten += rows
	}
	return res, nil
}

// RunSnapshotForOrg is the on-demand path behind hrm.analytics.manage, for
// backfilling a date or seeing a change reflected without waiting a day.
func (s *serviceImpl) RunSnapshotForOrg(ctx context.Context, orgID string, caller Caller, on time.Time) (*SnapshotResult, error) {
	if !caller.CanManage {
		return nil, ErrAccessDenied
	}
	facts, err := s.repo.BuildAttritionFacts(ctx, orgID, on)
	if err != nil {
		return nil, err
	}
	rows, err := s.repo.BuildHeadcountSnapshot(ctx, orgID, on, s.resolveThreshold(ctx, orgID))
	if err != nil {
		return nil, err
	}
	return &SnapshotResult{
		SnapshotDate: on, OrgsProcessed: 1, FactsWritten: facts, RowsWritten: rows,
	}, nil
}

func derefOr(p *string, fallback string) string {
	if p == nil {
		return fallback
	}
	return *p
}

func boolOrUnknown(p *bool) string {
	if p == nil {
		return "unknown"
	}
	if *p {
		return "true"
	}
	return "false"
}
