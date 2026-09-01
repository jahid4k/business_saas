// backend/internal/hrm/succession/service.go
package succession

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/shopspring/decimal"
)

// Caller carries the authorities the route gate has already established.
//
// CanViewConfidential is separate from CanManage on purpose: administering
// succession and reading the judgements it holds about named people are
// different authorities, the 9C hrm.exits.interview_view precedent.
type Caller struct {
	UserID              string
	CanManage           bool
	CanViewConfidential bool
	CanManagePlans      bool
}

// Service is succession's business layer.
type Service interface {
	// Critical positions — org design, the non-confidential half.
	CreateCriticalPosition(ctx context.Context, orgID string, caller Caller, req CreateCriticalPositionRequest) (*CriticalPosition, error)
	UpdateCriticalPosition(ctx context.Context, orgID string, caller Caller, ref string, req UpdateCriticalPositionRequest) (*CriticalPosition, error)
	ListCriticalPositions(ctx context.Context, orgID string, caller Caller, activeOnly bool) ([]*CriticalPosition, error)

	// Confidential
	RecordAssessment(ctx context.Context, orgID string, caller Caller, req RecordAssessmentRequest) (*TalentAssessment, error)
	NineBoxGrid(ctx context.Context, orgID string, caller Caller, asOf *time.Time) ([]*GridCell, error)
	Nominate(ctx context.Context, orgID string, caller Caller, criticalRef string, req NominateRequest) (*Candidate, error)
	WithdrawNomination(ctx context.Context, orgID string, caller Caller, candidateRef string, req WithdrawRequest) (*Candidate, error)
	ListCandidates(ctx context.Context, orgID string, caller Caller, criticalRef string, activeOnly bool) ([]*Candidate, error)
	ReviewEmployee(ctx context.Context, orgID string, caller Caller, employeeID string) (*ReviewerView, error)

	// Subject-visible
	MyDevelopment(ctx context.Context, orgID string, caller Caller) (*SubjectView, error)
	CreatePlan(ctx context.Context, orgID string, caller Caller, req CreatePlanRequest) (*DevelopmentPlan, error)
	UpdatePlan(ctx context.Context, orgID string, caller Caller, ref string, req UpdatePlanRequest) (*DevelopmentPlan, error)
	GetPlan(ctx context.Context, orgID string, caller Caller, ref string) (*DevelopmentPlan, error)
	ListPlans(ctx context.Context, orgID string, caller Caller, employeeID string) ([]*DevelopmentPlan, error)
	AddPlanItem(ctx context.Context, orgID string, caller Caller, planRef string, req CreateItemRequest) (*PlanItem, error)
	UpdatePlanItem(ctx context.Context, orgID string, caller Caller, itemRef string, req UpdateItemRequest) (*PlanItem, error)
}

type serviceImpl struct{ repo Repository }

func NewService(repo Repository) Service { return &serviceImpl{repo: repo} }

// ── critical positions ───────────────────────────────────────────────────────

func (s *serviceImpl) CreateCriticalPosition(ctx context.Context, orgID string, caller Caller, req CreateCriticalPositionRequest) (*CriticalPosition, error) {
	if !caller.CanManage {
		return nil, ErrAccessDenied
	}
	positionID := strings.TrimSpace(req.PositionID)
	if positionID == "" {
		return nil, ErrPositionNotFound
	}
	ok, err := s.repo.PositionExists(ctx, orgID, positionID)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, ErrPositionNotFound
	}

	level := Criticality(orDefault(req.CriticalityLevel, string(HighCriticality)))
	risk := VacancyRisk(orDefault(req.VacancyRisk, string(RiskMedium)))
	if !level.IsValid() || !risk.IsValid() {
		return nil, ErrInvalidCriticality
	}
	due, err := parseDate(req.ReviewDueDate)
	if err != nil {
		return nil, fmt.Errorf("succession: CreateCriticalPosition: review_due_date must be YYYY-MM-DD: %w", err)
	}

	cp := &CriticalPosition{
		OrgID: orgID, PositionID: positionID, CriticalityLevel: level, VacancyRisk: risk,
		ImpactOfVacancy: req.ImpactOfVacancy, IdentifiedBy: caller.UserID, ReviewDueDate: due,
	}
	if err := s.repo.CreateCritical(ctx, cp); err != nil {
		return nil, err
	}
	return s.repo.FindCriticalByRef(ctx, orgID, cp.ID)
}

func (s *serviceImpl) UpdateCriticalPosition(ctx context.Context, orgID string, caller Caller, ref string, req UpdateCriticalPositionRequest) (*CriticalPosition, error) {
	if !caller.CanManage {
		return nil, ErrAccessDenied
	}
	cp, err := s.repo.FindCriticalByRef(ctx, orgID, ref)
	if err != nil {
		return nil, err
	}
	if cp == nil {
		return nil, ErrCriticalNotFound
	}
	if req.CriticalityLevel != nil {
		cp.CriticalityLevel = Criticality(strings.TrimSpace(*req.CriticalityLevel))
	}
	if req.VacancyRisk != nil {
		cp.VacancyRisk = VacancyRisk(strings.TrimSpace(*req.VacancyRisk))
	}
	if !cp.CriticalityLevel.IsValid() || !cp.VacancyRisk.IsValid() {
		return nil, ErrInvalidCriticality
	}
	if req.ImpactOfVacancy != nil {
		cp.ImpactOfVacancy = req.ImpactOfVacancy
	}
	if req.ReviewDueDate != nil {
		d, err := parseDate(req.ReviewDueDate)
		if err != nil {
			return nil, fmt.Errorf("succession: UpdateCriticalPosition: review_due_date must be YYYY-MM-DD: %w", err)
		}
		cp.ReviewDueDate = d
	}
	if req.IsActive != nil && *req.IsActive != cp.IsActive {
		cp.IsActive = *req.IsActive
		// A designation is RETIRED, not deleted: its nominations and the
		// record of why the role once mattered stay readable.
		if !cp.IsActive {
			now := time.Now()
			cp.DeactivatedAt = &now
		} else {
			cp.DeactivatedAt = nil
		}
	}
	if err := s.repo.UpdateCritical(ctx, cp); err != nil {
		return nil, err
	}
	return s.repo.FindCriticalByRef(ctx, orgID, cp.ID)
}

func (s *serviceImpl) ListCriticalPositions(ctx context.Context, orgID string, caller Caller, activeOnly bool) ([]*CriticalPosition, error) {
	return s.repo.ListCritical(ctx, orgID, activeOnly)
}

// ── talent assessments (CONFIDENTIAL) ────────────────────────────────────────

// RecordAssessment writes one 9-box placement.
//
// ⚠ PERFORMANCE MAY BE DERIVED FROM THE APPRAISAL. POTENTIAL MAY NOT BE
// DERIVED FROM ANYTHING. If the caller omits the performance band it is read
// from their most recent published appraisal, because the appraisal IS the
// performance record. There is deliberately no equivalent fallback for
// potential: an omitted potential band is a refusal, not a copy of the
// performance band, because the moment potential mirrors performance the
// grid collapses to its diagonal and stops carrying information.
func (s *serviceImpl) RecordAssessment(ctx context.Context, orgID string, caller Caller, req RecordAssessmentRequest) (*TalentAssessment, error) {
	if !caller.CanManage {
		return nil, ErrAccessDenied
	}
	employeeID := strings.TrimSpace(req.EmployeeID)
	if employeeID == "" {
		return nil, ErrEmployeeNotFound
	}
	ok, err := s.repo.EmployeeExists(ctx, orgID, employeeID)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, ErrEmployeeNotFound
	}

	potential, valid := ParseBand(req.PotentialBand)
	if !valid {
		return nil, ErrInvalidBand
	}
	rationale := strings.TrimSpace(req.PotentialRationale)
	if rationale == "" {
		return nil, ErrRationaleRequired
	}

	asOf := time.Now()
	if strings.TrimSpace(req.AsOfDate) != "" {
		d, err := time.Parse("2006-01-02", strings.TrimSpace(req.AsOfDate))
		if err != nil {
			return nil, fmt.Errorf("succession: RecordAssessment: as_of_date must be YYYY-MM-DD: %w", err)
		}
		asOf = d
	}

	a := &TalentAssessment{
		OrgID: orgID, EmployeeID: employeeID, AsOfDate: asOf,
		PotentialBand: potential, PotentialRationale: rationale, AssessedBy: caller.UserID,
	}

	if perf, valid := ParseBand(req.PerformanceBand); valid {
		a.PerformanceBand = perf
	} else if strings.TrimSpace(req.PerformanceBand) != "" {
		return nil, ErrInvalidBand
	} else {
		derived, rating, err := s.derivePerformanceBand(ctx, orgID, employeeID)
		if err != nil {
			return nil, err
		}
		if derived == "" {
			return nil, ErrInvalidBand
		}
		a.PerformanceBand = derived
		a.PerformanceRatingSnapshot = rating
	}

	if err := s.repo.UpsertAssessment(ctx, a); err != nil {
		return nil, err
	}
	return a, nil
}

// derivePerformanceBand reads the most recent PUBLISHED appraisal rating and
// bands it against the org's rating scale. Returns an empty band rather than
// a guess when there is no rating or no scale — an unrated employee is
// unplaced, not a low performer.
func (s *serviceImpl) derivePerformanceBand(ctx context.Context, orgID, employeeID string) (Band, *decimal.Decimal, error) {
	ratings, err := s.repo.LatestPublishedRatings(ctx, orgID, employeeID, 1)
	if err != nil {
		return "", nil, err
	}
	if len(ratings) == 0 {
		return "", nil, nil
	}
	max, ok, err := s.repo.RatingScaleMax(ctx, orgID)
	if err != nil {
		return "", nil, err
	}
	if !ok {
		return "", nil, nil
	}
	band, ok := PerformanceBandFromRating(ratings[0], max)
	if !ok {
		return "", nil, nil
	}
	r := ratings[0]
	return band, &r, nil
}

func (s *serviceImpl) NineBoxGrid(ctx context.Context, orgID string, caller Caller, asOf *time.Time) ([]*GridCell, error) {
	if !caller.CanViewConfidential {
		return nil, ErrAccessDenied
	}
	assessments, err := s.repo.ListAssessments(ctx, orgID, asOf)
	if err != nil {
		return nil, err
	}
	byBox := map[int]*GridCell{}
	for _, a := range assessments {
		box, ok := Box(a.PerformanceBand, a.PotentialBand)
		if !ok {
			continue
		}
		cell := byBox[box.Box]
		if cell == nil {
			cell = &GridCell{
				Box: box.Box, Label: box.Label,
				Performance: box.Performance, Potential: box.Potential,
				EmployeeIDs: []string{},
			}
			byBox[box.Box] = cell
		}
		cell.EmployeeIDs = append(cell.EmployeeIDs, a.EmployeeID)
	}
	// All nine cells are returned, empty ones included: an empty box 9 is
	// the finding, and omitting it would hide it.
	out := make([]*GridCell, 0, 9)
	for _, pot := range []Band{BandHigh, BandMedium, BandLow} {
		for _, perf := range []Band{BandLow, BandMedium, BandHigh} {
			b, _ := Box(perf, pot)
			if cell := byBox[b.Box]; cell != nil {
				out = append(out, cell)
				continue
			}
			out = append(out, &GridCell{
				Box: b.Box, Label: b.Label, Performance: perf, Potential: pot,
				EmployeeIDs: []string{},
			})
		}
	}
	return out, nil
}

// ── candidates (CONFIDENTIAL) ────────────────────────────────────────────────

func (s *serviceImpl) Nominate(ctx context.Context, orgID string, caller Caller, criticalRef string, req NominateRequest) (*Candidate, error) {
	if !caller.CanManage {
		return nil, ErrAccessDenied
	}
	cp, err := s.repo.FindCriticalByRef(ctx, orgID, criticalRef)
	if err != nil {
		return nil, err
	}
	if cp == nil {
		return nil, ErrCriticalNotFound
	}
	employeeID := strings.TrimSpace(req.EmployeeID)
	ok, err := s.repo.EmployeeExists(ctx, orgID, employeeID)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, ErrEmployeeNotFound
	}
	readiness := Readiness(orDefault(req.Readiness, string(Ready3To5Years)))
	if !readiness.IsValid() {
		return nil, ErrInvalidReadiness
	}
	// A linked plan must belong to the nominee. Pointing a nomination at
	// somebody else's plan would put one person's development behind
	// another person's succession record.
	var planID *string
	if req.DevelopmentPlanID != nil && strings.TrimSpace(*req.DevelopmentPlanID) != "" {
		plan, err := s.repo.FindPlanByRef(ctx, orgID, strings.TrimSpace(*req.DevelopmentPlanID))
		if err != nil {
			return nil, err
		}
		if plan == nil {
			return nil, ErrPlanNotFound
		}
		if plan.EmployeeID != employeeID {
			return nil, ErrPlanNotFound
		}
		planID = &plan.ID
	}

	c := &Candidate{
		OrgID: orgID, CriticalPositionID: cp.ID, EmployeeID: employeeID,
		Readiness: readiness, NominationRationale: req.NominationRationale,
		DevelopmentPlanID: planID, NominatedBy: caller.UserID,
	}
	if err := s.repo.CreateCandidate(ctx, c); err != nil {
		return nil, err
	}
	return s.repo.FindCandidateByRef(ctx, orgID, c.ID)
}

func (s *serviceImpl) WithdrawNomination(ctx context.Context, orgID string, caller Caller, candidateRef string, req WithdrawRequest) (*Candidate, error) {
	if !caller.CanManage {
		return nil, ErrAccessDenied
	}
	c, err := s.repo.FindCandidateByRef(ctx, orgID, candidateRef)
	if err != nil {
		return nil, err
	}
	if c == nil {
		return nil, ErrCandidateNotFound
	}
	if err := s.repo.WithdrawCandidate(ctx, orgID, c.ID, req.Reason, time.Now()); err != nil {
		return nil, err
	}
	return s.repo.FindCandidateByRef(ctx, orgID, c.ID)
}

func (s *serviceImpl) ListCandidates(ctx context.Context, orgID string, caller Caller, criticalRef string, activeOnly bool) ([]*Candidate, error) {
	if !caller.CanViewConfidential {
		return nil, ErrAccessDenied
	}
	cp, err := s.repo.FindCriticalByRef(ctx, orgID, criticalRef)
	if err != nil {
		return nil, err
	}
	if cp == nil {
		return nil, ErrCriticalNotFound
	}
	return s.repo.ListCandidatesForPosition(ctx, orgID, cp.ID, activeOnly)
}

// ReviewEmployee is the CONFIDENTIAL read path.
//
// The service re-checks the same authority the route gates on, so a future
// non-HTTP caller — a scheduler, a report generator, another package —
// cannot reach this material by skipping the middleware.
func (s *serviceImpl) ReviewEmployee(ctx context.Context, orgID string, caller Caller, employeeID string) (*ReviewerView, error) {
	if !caller.CanViewConfidential {
		return nil, ErrAccessDenied
	}
	employeeID = strings.TrimSpace(employeeID)
	hire, lastPromo, name, err := s.repo.EmployeeTimeline(ctx, orgID, employeeID)
	if err != nil {
		return nil, err
	}

	view := &ReviewerView{
		EmployeeID: employeeID, DisplayName: name,
		FlightRisk: []Signal{}, Nominations: []*Candidate{},
	}

	if a, err := s.repo.LatestAssessment(ctx, orgID, employeeID); err != nil {
		return nil, err
	} else if a != nil {
		view.Assessment = a
		if box, ok := Box(a.PerformanceBand, a.PotentialBand); ok {
			view.NineBox = &box
		}
	}

	noms, err := s.repo.ListCandidatesForEmployee(ctx, orgID, employeeID)
	if err != nil {
		return nil, err
	}
	view.Nominations = noms

	signals, err := s.flightRisk(ctx, orgID, employeeID, hire, lastPromo, time.Now())
	if err != nil {
		return nil, err
	}
	view.FlightRisk = signals
	return view, nil
}

// flightRisk evaluates every indicator and returns those that fired.
//
// ⚠ It returns a LIST OF EXPLAINED FACTS AND NOTHING ELSE — no score, no
// level, no total. The build plan excludes predictive scoring, and this is
// the function where that exclusion has to hold: the moment a number exists,
// it gets acted on by people who cannot say what it means.
func (s *serviceImpl) flightRisk(ctx context.Context, orgID, employeeID string, hire time.Time, lastPromo *time.Time, asOf time.Time) ([]Signal, error) {
	out := []Signal{}

	if sig, ok := EvalNoPromotion(hire, lastPromo, asOf); ok {
		out = append(out, sig)
	}

	basic, bandMin, grade, err := s.repo.CurrentPayAndBand(ctx, orgID, employeeID, asOf)
	if err != nil {
		return nil, err
	}
	if sig, ok := EvalBelowBand(basic, bandMin, grade); ok {
		out = append(out, sig)
	}

	since := asOf.AddDate(0, -ManagerChurnWindowMonths, 0)
	changes, err := s.repo.ManagerChangesSince(ctx, orgID, employeeID, since)
	if err != nil {
		return nil, err
	}
	if sig, ok := EvalManagerChurn(changes); ok {
		out = append(out, sig)
	}

	ratings, err := s.repo.LatestPublishedRatings(ctx, orgID, employeeID, 2)
	if err != nil {
		return nil, err
	}
	if len(ratings) == 2 {
		// ratings[0] is the newest, so the previous one is ratings[1].
		if sig, ok := EvalAppraisalDecline(ratings[1], ratings[0]); ok {
			out = append(out, sig)
		}
	}
	return out, nil
}

// ── development plans (SUBJECT-VISIBLE) ──────────────────────────────────────

// MyDevelopment is the SUBJECT read path.
//
// ⚠ It calls repo.SubjectPlans and nothing else. It does not look up an
// assessment, a nomination or a flight-risk signal, and SubjectView has
// nowhere to put one. That is the confidentiality guarantee: there is no
// confidential value in memory for a later change to leak, so no filtering
// step exists that somebody could forget to apply.
func (s *serviceImpl) MyDevelopment(ctx context.Context, orgID string, caller Caller) (*SubjectView, error) {
	employeeID, err := s.repo.ResolveOwnEmployeeID(ctx, orgID, caller.UserID)
	if err != nil {
		return nil, err
	}
	plans, err := s.repo.SubjectPlans(ctx, orgID, employeeID)
	if err != nil {
		return nil, err
	}
	return &SubjectView{EmployeeID: employeeID, Plans: plans}, nil
}

func (s *serviceImpl) CreatePlan(ctx context.Context, orgID string, caller Caller, req CreatePlanRequest) (*DevelopmentPlan, error) {
	if !caller.CanManagePlans {
		return nil, ErrAccessDenied
	}
	employeeID := strings.TrimSpace(req.EmployeeID)
	ok, err := s.repo.EmployeeExists(ctx, orgID, employeeID)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, ErrEmployeeNotFound
	}
	title := strings.TrimSpace(req.Title)
	if title == "" {
		return nil, ErrTitleRequired
	}
	status := "draft"
	if req.Status != nil && strings.TrimSpace(*req.Status) != "" {
		status = strings.TrimSpace(*req.Status)
		if !planStatuses[status] {
			return nil, ErrInvalidStatus
		}
	}
	target, err := parseDate(req.TargetDate)
	if err != nil {
		return nil, fmt.Errorf("succession: CreatePlan: target_date must be YYYY-MM-DD: %w", err)
	}
	p := &DevelopmentPlan{
		OrgID: orgID, EmployeeID: employeeID, Title: title, Objective: req.Objective,
		TargetDate: target, Status: status, CreatedBy: caller.UserID, Items: []*PlanItem{},
	}
	if err := s.repo.CreatePlan(ctx, p); err != nil {
		return nil, err
	}
	return p, nil
}

func (s *serviceImpl) UpdatePlan(ctx context.Context, orgID string, caller Caller, ref string, req UpdatePlanRequest) (*DevelopmentPlan, error) {
	if !caller.CanManagePlans {
		return nil, ErrAccessDenied
	}
	p, err := s.repo.FindPlanByRef(ctx, orgID, ref)
	if err != nil {
		return nil, err
	}
	if p == nil {
		return nil, ErrPlanNotFound
	}
	if req.Title != nil {
		t := strings.TrimSpace(*req.Title)
		if t == "" {
			return nil, ErrTitleRequired
		}
		p.Title = t
	}
	if req.Objective != nil {
		p.Objective = req.Objective
	}
	if req.TargetDate != nil {
		d, err := parseDate(req.TargetDate)
		if err != nil {
			return nil, fmt.Errorf("succession: UpdatePlan: target_date must be YYYY-MM-DD: %w", err)
		}
		p.TargetDate = d
	}
	if req.Status != nil {
		st := strings.TrimSpace(*req.Status)
		if !planStatuses[st] {
			return nil, ErrInvalidStatus
		}
		p.Status = st
		if st == "completed" && p.CompletedAt == nil {
			now := time.Now()
			p.CompletedAt = &now
		}
		if st != "completed" {
			p.CompletedAt = nil
		}
	}
	if err := s.repo.UpdatePlan(ctx, p); err != nil {
		return nil, err
	}
	return s.repo.FindPlanByRef(ctx, orgID, p.ID)
}

// GetPlan lets the subject read their own plan without holding the manage
// authority. Anybody else needs it.
func (s *serviceImpl) GetPlan(ctx context.Context, orgID string, caller Caller, ref string) (*DevelopmentPlan, error) {
	p, err := s.repo.FindPlanByRef(ctx, orgID, ref)
	if err != nil {
		return nil, err
	}
	if p == nil {
		return nil, ErrPlanNotFound
	}
	if !caller.CanManagePlans {
		own, err := s.repo.ResolveOwnEmployeeID(ctx, orgID, caller.UserID)
		if err != nil {
			return nil, err
		}
		if own != p.EmployeeID {
			// Not-found rather than denied: confirming the plan exists
			// would tell the caller that a plan was written about that
			// person, which is itself something they were not shown.
			return nil, ErrPlanNotFound
		}
		if p.Status == "draft" {
			return nil, ErrPlanNotFound
		}
	}
	plans, err := s.repo.ListPlans(ctx, orgID, p.EmployeeID)
	if err != nil {
		return nil, err
	}
	for _, cand := range plans {
		if cand.ID == p.ID {
			return cand, nil
		}
	}
	return p, nil
}

func (s *serviceImpl) ListPlans(ctx context.Context, orgID string, caller Caller, employeeID string) ([]*DevelopmentPlan, error) {
	if !caller.CanManagePlans {
		return nil, ErrAccessDenied
	}
	return s.repo.ListPlans(ctx, orgID, strings.TrimSpace(employeeID))
}

func (s *serviceImpl) AddPlanItem(ctx context.Context, orgID string, caller Caller, planRef string, req CreateItemRequest) (*PlanItem, error) {
	if !caller.CanManagePlans {
		return nil, ErrAccessDenied
	}
	p, err := s.repo.FindPlanByRef(ctx, orgID, planRef)
	if err != nil {
		return nil, err
	}
	if p == nil {
		return nil, ErrPlanNotFound
	}
	desc := strings.TrimSpace(req.Description)
	if desc == "" {
		return nil, ErrDescriptionRequired
	}
	target, err := parseDate(req.TargetDate)
	if err != nil {
		return nil, fmt.Errorf("succession: AddPlanItem: target_date must be YYYY-MM-DD: %w", err)
	}
	it := &PlanItem{
		OrgID: orgID, PlanID: p.ID, Description: desc, TargetDate: target,
		CreatedBy: caller.UserID,
	}
	if req.SortOrder != nil {
		it.SortOrder = *req.SortOrder
	}
	if err := s.repo.CreateItem(ctx, it); err != nil {
		return nil, err
	}
	return it, nil
}

// UpdatePlanItem lets the SUBJECT mark their own actions done.
//
// An employee who cannot record their own progress has a plan written about
// them rather than one they are working through, so the own-plan path is
// deliberately open here — but only for status, never for what the action
// says or when it is due.
func (s *serviceImpl) UpdatePlanItem(ctx context.Context, orgID string, caller Caller, itemRef string, req UpdateItemRequest) (*PlanItem, error) {
	it, err := s.repo.FindItemByRef(ctx, orgID, itemRef)
	if err != nil {
		return nil, err
	}
	if it == nil {
		return nil, ErrItemNotFound
	}
	plan, err := s.repo.FindPlanByRef(ctx, orgID, it.PlanID)
	if err != nil {
		return nil, err
	}
	if plan == nil {
		return nil, ErrPlanNotFound
	}

	isOwner := false
	if !caller.CanManagePlans {
		own, err := s.repo.ResolveOwnEmployeeID(ctx, orgID, caller.UserID)
		if err != nil {
			return nil, err
		}
		if own != plan.EmployeeID || plan.Status == "draft" {
			return nil, ErrItemNotFound
		}
		isOwner = true
	}

	if req.Status != nil {
		st := strings.TrimSpace(*req.Status)
		if !itemStatuses[st] {
			return nil, ErrInvalidStatus
		}
		it.Status = st
		if st == "completed" && it.CompletedAt == nil {
			now := time.Now()
			it.CompletedAt = &now
		}
		if st != "completed" {
			it.CompletedAt = nil
		}
	}
	if !isOwner {
		if req.Description != nil {
			d := strings.TrimSpace(*req.Description)
			if d == "" {
				return nil, ErrDescriptionRequired
			}
			it.Description = d
		}
		if req.TargetDate != nil {
			d, err := parseDate(req.TargetDate)
			if err != nil {
				return nil, fmt.Errorf("succession: UpdatePlanItem: target_date must be YYYY-MM-DD: %w", err)
			}
			it.TargetDate = d
		}
		if req.SortOrder != nil {
			it.SortOrder = *req.SortOrder
		}
	}
	if err := s.repo.UpdateItem(ctx, it); err != nil {
		return nil, err
	}
	return s.repo.FindItemByRef(ctx, orgID, it.ID)
}

func orDefault(v, fallback string) string {
	if strings.TrimSpace(v) == "" {
		return fallback
	}
	return strings.TrimSpace(v)
}
