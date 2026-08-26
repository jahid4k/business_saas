// backend/internal/hrm/benefits/service.go
package benefits

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/shopspring/decimal"

	"github.com/mridha/businesssaas/internal/hrm/payslips"
)

// Service covers benefit plans, tiers, enrollments and dependents, plus the
// two methods (ActivatePendingEnrollments, PendingDeductionsForEmployee)
// that make this package a scheduler consumer and structurally satisfy
// payslips.BenefitsSource.
type Service interface {
	ListPlans(ctx context.Context, orgID string) ([]*Plan, error)
	GetPlan(ctx context.Context, orgID, ref string) (*Plan, error)
	CreatePlan(ctx context.Context, orgID, createdBy string, req CreatePlanRequest) (*Plan, error)
	CreateTier(ctx context.Context, orgID, planRef, createdBy string, req CreateTierRequest) (*Tier, error)
	ListTiers(ctx context.Context, orgID, planRef string) ([]*Tier, error)

	ListEnrollments(ctx context.Context, orgID string, filter ListFilter) (*EnrollmentListResponse, error)
	GetEnrollment(ctx context.Context, orgID, ref string) (*Enrollment, error)
	// EnrollSelf is the self-service enrollment path — the route gates on
	// hrm.benefit_enrollments.enroll_self, which cannot express "for
	// yourself only" (the hrm.goals.set_own precedent), so the SERVICE
	// resolves the caller's own employeeID from userID, exactly as
	// compensation.Repository.FindEmployeeIDByUserID does.
	EnrollSelf(ctx context.Context, orgID, userID string, req CreateEnrollmentRequest) (*Enrollment, error)
	WaiveEnrollment(ctx context.Context, orgID, ref string) (*Enrollment, error)

	CreateDependent(ctx context.Context, orgID, createdBy string, req CreateDependentRequest) (*Dependent, error)
	ListDependents(ctx context.Context, orgID, employeeID string) ([]*Dependent, error)
	VerifyDependent(ctx context.Context, orgID, ref, verifiedBy string) (*Dependent, error)

	// ActivatePendingEnrollments is the scheduler consumer — flips every
	// 'pending' enrollment whose effective_date has arrived to 'active',
	// instance-wide, the leave/absence/certification-sweep shape.
	ActivatePendingEnrollments(ctx context.Context) (int, error)

	// PendingDeductionsForEmployee satisfies payslips.BenefitsSource
	// structurally. See that interface's doc comment in
	// hrm/payslips/model.go for why benefits imports payslips, not the
	// reverse.
	PendingDeductionsForEmployee(ctx context.Context, orgID, employeeID string, year, month int) ([]payslips.PendingBenefitDeduction, error)
}

type serviceImpl struct{ repo Repository }

func NewService(repo Repository) Service { return &serviceImpl{repo: repo} }

func (s *serviceImpl) ListPlans(ctx context.Context, orgID string) ([]*Plan, error) {
	return s.repo.ListPlans(ctx, orgID)
}

func (s *serviceImpl) GetPlan(ctx context.Context, orgID, ref string) (*Plan, error) {
	p, err := s.repo.FindPlanByRef(ctx, orgID, ref)
	if err != nil {
		return nil, fmt.Errorf("benefits: GetPlan: %w", err)
	}
	if p == nil {
		return nil, ErrPlanNotFound
	}
	return p, nil
}

func (s *serviceImpl) CreatePlan(ctx context.Context, orgID, createdBy string, req CreatePlanRequest) (*Plan, error) {
	pt := PlanType(strings.TrimSpace(req.PlanType))
	if !pt.IsValid() {
		return nil, ErrInvalidPlanType
	}
	name := strings.TrimSpace(req.Name)
	if name == "" {
		return nil, fmt.Errorf("benefits: CreatePlan: name is required")
	}
	p := &Plan{OrgID: orgID, Name: name, PlanType: pt, Description: nilIfBlank(req.Description), IsActive: true, CreatedBy: createdBy}
	if err := s.repo.CreatePlan(ctx, p); err != nil {
		return nil, fmt.Errorf("benefits: CreatePlan: %w", err)
	}
	return p, nil
}

func (s *serviceImpl) CreateTier(ctx context.Context, orgID, planRef, createdBy string, req CreateTierRequest) (*Tier, error) {
	plan, err := s.repo.FindPlanByRef(ctx, orgID, planRef)
	if err != nil {
		return nil, fmt.Errorf("benefits: CreateTier: %w", err)
	}
	if plan == nil {
		return nil, ErrPlanNotFound
	}
	empCost, err := parseNonNegative(req.EmployeeCost)
	if err != nil {
		return nil, err
	}
	erCost, err := parseNonNegative(req.EmployerCost)
	if err != nil {
		return nil, err
	}
	name := strings.TrimSpace(req.TierName)
	if name == "" {
		return nil, fmt.Errorf("benefits: CreateTier: tier_name is required")
	}
	t := &Tier{PlanID: plan.ID, TierName: name, EmployeeCost: empCost, EmployerCost: erCost, IsActive: true, CreatedBy: createdBy}
	if err := s.repo.CreateTier(ctx, plan.ID, t); err != nil {
		return nil, fmt.Errorf("benefits: CreateTier: %w", err)
	}
	return t, nil
}

func (s *serviceImpl) ListTiers(ctx context.Context, orgID, planRef string) ([]*Tier, error) {
	plan, err := s.repo.FindPlanByRef(ctx, orgID, planRef)
	if err != nil {
		return nil, fmt.Errorf("benefits: ListTiers: %w", err)
	}
	if plan == nil {
		return nil, ErrPlanNotFound
	}
	return s.repo.ListTiersByPlan(ctx, plan.ID)
}

func (s *serviceImpl) ListEnrollments(ctx context.Context, orgID string, filter ListFilter) (*EnrollmentListResponse, error) {
	filter.Normalise()
	list, total, err := s.repo.ListEnrollments(ctx, orgID, filter)
	if err != nil {
		return nil, fmt.Errorf("benefits: ListEnrollments: %w", err)
	}
	return &EnrollmentListResponse{Enrollments: list, Total: total, Limit: filter.Limit, Offset: filter.Offset}, nil
}

func (s *serviceImpl) GetEnrollment(ctx context.Context, orgID, ref string) (*Enrollment, error) {
	e, err := s.repo.FindEnrollmentByRef(ctx, orgID, ref)
	if err != nil {
		return nil, fmt.Errorf("benefits: GetEnrollment: %w", err)
	}
	if e == nil {
		return nil, ErrEnrollmentNotFound
	}
	return e, nil
}

// EnrollSelf snapshots the tier's CURRENT cost onto the enrollment — see
// migration 00104's header on why a later re-price must not change it.
func (s *serviceImpl) EnrollSelf(ctx context.Context, orgID, userID string, req CreateEnrollmentRequest) (*Enrollment, error) {
	employeeID, err := s.repo.FindEmployeeIDByUserID(ctx, orgID, userID)
	if err != nil {
		return nil, fmt.Errorf("benefits: EnrollSelf: resolve caller employee: %w", err)
	}
	if employeeID == "" {
		return nil, fmt.Errorf("benefits: EnrollSelf: caller has no employee record")
	}
	window := WindowType(strings.TrimSpace(req.EnrollmentWindowType))
	if !window.IsValid() {
		return nil, ErrInvalidWindowType
	}
	tier, err := s.repo.FindTierByRef(ctx, req.TierID)
	if err != nil {
		return nil, fmt.Errorf("benefits: EnrollSelf: %w", err)
	}
	if tier == nil {
		return nil, ErrTierNotFound
	}
	plan, err := s.repo.FindPlanByRef(ctx, orgID, req.PlanID)
	if err != nil {
		return nil, fmt.Errorf("benefits: EnrollSelf: %w", err)
	}
	if plan == nil || tier.PlanID != plan.ID {
		return nil, ErrPlanNotFound
	}
	eff, err := parseDate(req.EffectiveDate)
	if err != nil {
		return nil, err
	}

	e := &Enrollment{
		OrgID: orgID, EmployeeID: employeeID, PlanID: plan.ID, TierID: tier.ID,
		EnrollmentWindowType: window, Status: EnrollmentPending, EffectiveDate: *eff,
		EmployeeCostSnapshot: tier.EmployeeCost, EmployerCostSnapshot: tier.EmployerCost,
		CreatedBy: userID,
	}
	if err := s.repo.CreateEnrollment(ctx, e); err != nil {
		return nil, err // ErrAlreadyEnrolled surfaces from the repository unwrapped
	}
	return e, nil
}

func (s *serviceImpl) WaiveEnrollment(ctx context.Context, orgID, ref string) (*Enrollment, error) {
	e, err := s.repo.FindEnrollmentByRef(ctx, orgID, ref)
	if err != nil {
		return nil, fmt.Errorf("benefits: WaiveEnrollment: %w", err)
	}
	if e == nil {
		return nil, ErrEnrollmentNotFound
	}
	if e.Status != EnrollmentPending && e.Status != EnrollmentActive {
		return nil, ErrWrongStatus
	}
	if err := s.repo.UpdateEnrollmentStatus(ctx, e.ID, EnrollmentWaived); err != nil {
		return nil, fmt.Errorf("benefits: WaiveEnrollment: %w", err)
	}
	e.Status = EnrollmentWaived
	return e, nil
}

func (s *serviceImpl) CreateDependent(ctx context.Context, orgID, createdBy string, req CreateDependentRequest) (*Dependent, error) {
	rel := Relationship(strings.TrimSpace(req.Relationship))
	if !rel.IsValid() {
		return nil, ErrInvalidRelationship
	}
	name := strings.TrimSpace(req.FullName)
	if name == "" {
		return nil, fmt.Errorf("benefits: CreateDependent: full_name is required")
	}
	employeeID := strings.TrimSpace(req.EmployeeID)
	if employeeID == "" {
		return nil, fmt.Errorf("benefits: CreateDependent: employee_id is required")
	}
	var dob *time.Time
	if req.DateOfBirth != nil && strings.TrimSpace(*req.DateOfBirth) != "" {
		d, err := parseDate(*req.DateOfBirth)
		if err != nil {
			return nil, err
		}
		dob = d
	}
	d := &Dependent{
		OrgID: orgID, EmployeeID: employeeID, EnrollmentID: req.EnrollmentID,
		FullName: name, Relationship: rel, DateOfBirth: dob, CreatedBy: createdBy,
	}
	if err := s.repo.CreateDependent(ctx, d); err != nil {
		return nil, fmt.Errorf("benefits: CreateDependent: %w", err)
	}
	return d, nil
}

func (s *serviceImpl) ListDependents(ctx context.Context, orgID, employeeID string) ([]*Dependent, error) {
	return s.repo.ListDependentsByEmployee(ctx, orgID, employeeID)
}

func (s *serviceImpl) VerifyDependent(ctx context.Context, orgID, ref, verifiedBy string) (*Dependent, error) {
	d, err := s.repo.FindDependentByRef(ctx, orgID, ref)
	if err != nil {
		return nil, fmt.Errorf("benefits: VerifyDependent: %w", err)
	}
	if d == nil {
		return nil, ErrDependentNotFound
	}
	if err := s.repo.VerifyDependent(ctx, d.ID, verifiedBy); err != nil {
		return nil, fmt.Errorf("benefits: VerifyDependent: %w", err)
	}
	d.IsVerified = true
	return d, nil
}

func (s *serviceImpl) ActivatePendingEnrollments(ctx context.Context) (int, error) {
	n, err := s.repo.ActivatePendingDue(ctx)
	if err != nil {
		return n, fmt.Errorf("benefits: ActivatePendingEnrollments: %w", err)
	}
	return n, nil
}

func (s *serviceImpl) PendingDeductionsForEmployee(ctx context.Context, orgID, employeeID string, _, _ int) ([]payslips.PendingBenefitDeduction, error) {
	enrollments, err := s.repo.ActiveEnrollmentsForEmployee(ctx, orgID, employeeID)
	if err != nil {
		return nil, fmt.Errorf("benefits: PendingDeductionsForEmployee: %w", err)
	}
	out := make([]payslips.PendingBenefitDeduction, 0, len(enrollments))
	for _, e := range enrollments {
		if e.EmployeeCostSnapshot.IsZero() {
			continue
		}
		out = append(out, payslips.PendingBenefitDeduction{
			EnrollmentID: e.ID,
			Description:  "Benefits Premium",
			Amount:       e.EmployeeCostSnapshot,
		})
	}
	return out, nil
}

func parseNonNegative(s string) (decimal.Decimal, error) {
	d, err := decimal.NewFromString(strings.TrimSpace(s))
	if err != nil || d.IsNegative() {
		return decimal.Zero, ErrInvalidAmount
	}
	return d, nil
}

const dateLayout = "2006-01-02"

func parseDate(v string) (*time.Time, error) {
	v = strings.TrimSpace(v)
	if v == "" {
		return nil, fmt.Errorf("benefits: date is required")
	}
	d, err := time.Parse(dateLayout, v)
	if err != nil {
		return nil, fmt.Errorf("benefits: invalid date %q: %w", v, err)
	}
	return &d, nil
}

func nilIfBlank(s *string) *string {
	if s == nil || strings.TrimSpace(*s) == "" {
		return nil
	}
	trimmed := strings.TrimSpace(*s)
	return &trimmed
}
