// backend/internal/hrm/exits/service.go
package exits

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/shopspring/decimal"

	"github.com/mridha/businesssaas/internal/authz"
	"github.com/mridha/businesssaas/internal/hrm/payslips"
	"github.com/mridha/businesssaas/internal/hrm/scope"
	"github.com/mridha/businesssaas/internal/platform/checklists"
)

// Caller carries the caller's identity and resolved scope tier. The handler
// resolves the tier once and hands it over, the hrm/performance precedent —
// this service holds no authz.Service of its own.
type Caller struct {
	UserID string
	Scope  authz.Scope
	// CanManage gates the write paths the route gate cannot express.
	CanManage bool
}

// Service is exit management's business layer.
type Service interface {
	Create(ctx context.Context, orgID string, caller Caller, req CreateExitRequest) (*Exit, error)
	List(ctx context.Context, orgID string, caller Caller, f ListFilter) (*ExitListResponse, error)
	Get(ctx context.Context, orgID string, caller Caller, ref string) (*Exit, error)
	Update(ctx context.Context, orgID string, caller Caller, ref string, req UpdateExitRequest) (*Exit, error)
	Cancel(ctx context.Context, orgID string, caller Caller, ref string) (*Exit, error)

	// StartClearance instantiates the org's default offboarding checklist for
	// this exit. Separate from Create because an org with no offboarding
	// template still needs exits to work, and because HR may want to record
	// the exit before clearance begins.
	StartClearance(ctx context.Context, orgID string, caller Caller, ref string) (*Exit, error)

	// Clearance items
	AddClearanceItem(ctx context.Context, orgID string, caller Caller, ref string, req CreateClearanceItemRequest) (*ClearanceItem, error)
	ListClearanceItems(ctx context.Context, orgID string, caller Caller, ref string) ([]*ClearanceItem, error)
	ResolveClearanceItem(ctx context.Context, orgID string, caller Caller, ref, itemRef string, req ResolveClearanceItemRequest) (*ClearanceItem, error)

	// Rehire
	DecideRehire(ctx context.Context, orgID string, caller Caller, employeeRef string, req DecideRehireRequest) (*RehireEligibility, error)
	GetRehire(ctx context.Context, orgID string, caller Caller, employeeRef string) (*RehireEligibility, error)

	// AttachFnFRun links a payroll run to the exit it settles.
	AttachFnFRun(ctx context.Context, orgID string, caller Caller, ref, runID string) (*Exit, error)
	// ListSettlementLines returns the audit trail explaining a settlement.
	ListSettlementLines(ctx context.Context, orgID string, caller Caller, ref string) ([]*SettlementLineRow, error)

	// Gratuity rules
	ListGratuityRules(ctx context.Context, orgID string, caller Caller) ([]*GratuityRuleRow, error)
	CreateGratuityRule(ctx context.Context, orgID string, caller Caller, req CreateGratuityRuleRequest) (*GratuityRuleRow, error)

	// SettlementForRun and MarkSettled satisfy payslips.FnFSource, and
	// ClearanceComplete backs the approval gate. Declared on the interface so
	// main.go can pass this service where an FnFSource is wanted —
	// satisfaction is structural, so the methods must be visible HERE and not
	// only on serviceImpl. The terminations.CreateDraftFromPIP precedent.
	SettlementForRun(ctx context.Context, orgID, runID string) (*payslips.FnFSettlement, error)
	MarkSettled(ctx context.Context, runID string, applied []payslips.AppliedSettlementLine) error
	ClearanceComplete(ctx context.Context, orgID, runID string) (bool, error)

	// CheckRehireEligibility satisfies recruitment.RehireChecker. Declared on
	// the interface so main.go can pass this service where a RehireChecker is
	// wanted — satisfaction is structural, so the method must be visible here
	// and not only on serviceImpl.
	CheckRehireEligibility(ctx context.Context, orgID, email string) (*RehireEligibility, error)
}

type serviceImpl struct {
	repo       Repository
	checklists checklists.Service
	resolver   *scope.Resolver
	// The three cross-module settlement sources. All nil-safe: a deployment
	// without one produces no line from it rather than failing the whole
	// settlement. See their interface doc comments in settlement.go.
	leaveSource   LeaveEncashmentSource
	loanSource    LoanSettlementSource
	advanceSource AdvanceSettlementSource
}

func NewService(
	repo Repository, checklistsSvc checklists.Service, resolver *scope.Resolver,
	leaveSource LeaveEncashmentSource, loanSource LoanSettlementSource,
	advanceSource AdvanceSettlementSource,
) Service {
	return &serviceImpl{
		repo: repo, checklists: checklistsSvc, resolver: resolver,
		leaveSource: leaveSource, loanSource: loanSource, advanceSource: advanceSource,
	}
}

// ── Access ───────────────────────────────────────────────────────────────────

// authorize checks the caller may see this specific exit at their tier. The
// same rule ListFilter applies in SQL, restated for single-row reads — a
// filtered list that hides an exit is worthless if fetching it by id returns
// it anyway.
func (s *serviceImpl) authorize(ctx context.Context, orgID string, caller Caller, e *Exit) error {
	ok, err := s.resolver.AuthorizeRecordAccess(ctx, caller.Scope, orgID, caller.UserID, e.EmployeeID)
	if err != nil {
		return fmt.Errorf("exits: authorize: %w", err)
	}
	if !ok {
		// Not-found rather than denied: "you may not see exit X" still
		// confirms X exists, and an exit record discloses that a named
		// colleague is leaving before it has been announced.
		return ErrExitNotFound
	}
	return nil
}

func (s *serviceImpl) loadExit(ctx context.Context, orgID string, caller Caller, ref string) (*Exit, error) {
	e, err := s.repo.FindExitByRef(ctx, orgID, ref)
	if err != nil {
		return nil, fmt.Errorf("exits: load %s: %w", ref, err)
	}
	if e == nil {
		return nil, ErrExitNotFound
	}
	if err := s.authorize(ctx, orgID, caller, e); err != nil {
		return nil, err
	}
	return e, nil
}

// ── Exits ────────────────────────────────────────────────────────────────────

func (s *serviceImpl) Create(ctx context.Context, orgID string, caller Caller, req CreateExitRequest) (*Exit, error) {
	if !caller.CanManage {
		return nil, ErrAccessDenied
	}
	employeeID := strings.TrimSpace(req.EmployeeID)
	if employeeID == "" {
		return nil, ErrEmployeeRequired
	}
	sourceType := SourceType(strings.TrimSpace(req.SourceType))
	if !sourceType.IsValid() {
		return nil, ErrInvalidSourceType
	}
	sourceID := strings.TrimSpace(req.SourceID)
	if sourceID == "" {
		return nil, ErrSourceNotFound
	}

	// source_id carries no FK by design (migration 00114), so existence and
	// ownership are checked here. Without this an exit could point at another
	// employee's resignation and settle the wrong person.
	ok, err := s.repo.SourceExists(ctx, orgID, sourceType, sourceID, employeeID)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, ErrSourceMismatch
	}

	// One live exit per employee — matches uq_hrm_exit_active. Checked here
	// so the caller gets ErrExitAlreadyOpen rather than a raw 23505.
	existing, err := s.repo.FindOpenExitForEmployee(ctx, orgID, employeeID)
	if err != nil {
		return nil, err
	}
	if existing != nil {
		return nil, ErrExitAlreadyOpen
	}

	notice, err := s.repo.FindSourceNotice(ctx, orgID, sourceType, sourceID)
	if err != nil {
		return nil, err
	}
	if notice == nil {
		return nil, ErrSourceNotFound
	}

	e := &Exit{
		OrgID: orgID, EmployeeID: employeeID,
		SourceType: sourceType, SourceID: sourceID,
		Remarks: req.Remarks, CreatedBy: caller.UserID,
	}
	applyNotice(e, notice)

	if err := s.repo.CreateExit(ctx, e); err != nil {
		return nil, err
	}

	// A termination already carries a rehire decision (is_rehire_eligible,
	// migration 00034) that nothing has ever read. Seeding it here is what
	// finally gives that column a consumer.
	if sourceType == SourceTermination {
		s.seedRehireFromTermination(ctx, orgID, caller, e)
	}
	return e, nil
}

// applyNotice snapshots the source's notice dates onto the exit and derives
// the shortfall. Both live here rather than in the repository because the
// derivation is a business rule, and because the exit must keep its own copy:
// correcting the resignation later must not silently restate a settlement
// already computed against the old dates.
func applyNotice(e *Exit, notice *SourceNotice) {
	if !notice.HasNoticeTracking {
		// A termination. The employer set the leaving date, so there is no
		// entitlement to fall short of.
		e.LastWorkingDate = time.Now()
		e.ExpectedLastWorkingDate = nil
		e.NoticeShortfallDays = 0
		return
	}
	e.LastWorkingDate = notice.LastWorkingDate
	if notice.IsNoticeWaived {
		// A waiver is not a shortfall. Leaving ExpectedLastWorkingDate nil is
		// what makes NoticeShortfallDays return zero — see notice.go.
		e.ExpectedLastWorkingDate = nil
		e.NoticeShortfallDays = 0
		return
	}
	expected := notice.LastWorkingDate
	e.ExpectedLastWorkingDate = &expected
	e.NoticeShortfallDays = NoticeShortfallDays(&expected, notice.LastWorkingDate)
}

func (s *serviceImpl) seedRehireFromTermination(ctx context.Context, orgID string, caller Caller, e *Exit) {
	flag, err := s.repo.FindTerminationRehireFlag(ctx, orgID, e.SourceID)
	if err != nil || flag == nil {
		// Best-effort: a missing rehire seed must not fail exit creation.
		// The decision can always be recorded explicitly afterwards.
		if err != nil {
			slog.Warn("exits: could not seed rehire eligibility from termination",
				slog.String("org_id", orgID), slog.String("exit_id", e.ID), slog.Any("error", err))
		}
		return
	}
	status := RehireEligible
	var reason *string
	if !*flag {
		status = RehireNotEligible
		r := "Seeded from the termination record, which was marked not rehire-eligible."
		reason = &r
	}
	re := &RehireEligibility{
		OrgID: orgID, EmployeeID: e.EmployeeID, ExitID: &e.ID,
		Status: status, Reason: reason, DecidedBy: &caller.UserID,
	}
	if err := s.repo.UpsertRehireEligibility(ctx, re); err != nil {
		slog.Warn("exits: could not seed rehire eligibility",
			slog.String("org_id", orgID), slog.String("exit_id", e.ID), slog.Any("error", err))
	}
}

func (s *serviceImpl) List(ctx context.Context, orgID string, caller Caller, f ListFilter) (*ExitListResponse, error) {
	// Never read off the request — a caller who could set these would hand
	// themselves org-wide visibility.
	f.Scope = caller.Scope
	f.CallerUserID = caller.UserID
	f.Normalise()

	list, err := s.repo.FindExits(ctx, orgID, f)
	if err != nil {
		return nil, err
	}
	total, err := s.repo.CountExits(ctx, orgID, f)
	if err != nil {
		return nil, err
	}
	for _, e := range list {
		if err := s.attachClearance(ctx, orgID, e, false); err != nil {
			return nil, err
		}
	}
	return &ExitListResponse{Exits: list, Total: total, Limit: f.Limit, Offset: f.Offset}, nil
}

func (s *serviceImpl) Get(ctx context.Context, orgID string, caller Caller, ref string) (*Exit, error) {
	e, err := s.loadExit(ctx, orgID, caller, ref)
	if err != nil {
		return nil, err
	}
	if err := s.attachClearance(ctx, orgID, e, true); err != nil {
		return nil, err
	}
	return e, nil
}

// attachClearance computes the clearance summary from the items. Nothing is
// stored — see migration 00114's header. withItems is false on the list path
// so a page of exits does not carry every item body.
func (s *serviceImpl) attachClearance(ctx context.Context, orgID string, e *Exit, withItems bool) error {
	items, err := s.repo.FindClearanceItems(ctx, orgID, e.ID)
	if err != nil {
		return fmt.Errorf("exits: attachClearance: %w", err)
	}
	e.Clearance = summariseClearance(items)
	if withItems {
		e.Items = items
	}
	return nil
}

func (s *serviceImpl) Update(ctx context.Context, orgID string, caller Caller, ref string, req UpdateExitRequest) (*Exit, error) {
	if !caller.CanManage {
		return nil, ErrAccessDenied
	}
	e, err := s.loadExit(ctx, orgID, caller, ref)
	if err != nil {
		return nil, err
	}
	if e.Status.IsTerminal() {
		return nil, ErrWrongStatus
	}
	if req.Remarks != nil {
		e.Remarks = req.Remarks
	}
	if req.LastWorkingDate != nil {
		d, err := time.Parse("2006-01-02", strings.TrimSpace(*req.LastWorkingDate))
		if err != nil {
			return nil, fmt.Errorf("exits: Update: last_working_date must be YYYY-MM-DD: %w", err)
		}
		// Once settled the date is frozen: the settlement was computed
		// against it, and moving it afterwards would leave the payslip
		// explaining a period that no longer exists.
		if e.Status == StatusSettled || e.Status == StatusCompleted {
			return nil, ErrWrongStatus
		}
		e.LastWorkingDate = d
		// Recomputed, never separately editable, so the date and the
		// shortfall can never disagree.
		e.NoticeShortfallDays = NoticeShortfallDays(e.ExpectedLastWorkingDate, d)
	}
	if err := s.repo.UpdateExit(ctx, e); err != nil {
		return nil, err
	}
	return e, nil
}

func (s *serviceImpl) Cancel(ctx context.Context, orgID string, caller Caller, ref string) (*Exit, error) {
	if !caller.CanManage {
		return nil, ErrAccessDenied
	}
	e, err := s.loadExit(ctx, orgID, caller, ref)
	if err != nil {
		return nil, err
	}
	// A settled exit has already moved money. Cancelling it would leave a
	// payslip settling an exit that claims never to have happened.
	if e.Status == StatusSettled || e.Status == StatusCompleted || e.Status == StatusCancelled {
		return nil, ErrWrongStatus
	}
	e.Status = StatusCancelled
	if err := s.repo.UpdateExit(ctx, e); err != nil {
		return nil, err
	}
	return e, nil
}

// ── Clearance ────────────────────────────────────────────────────────────────

func (s *serviceImpl) StartClearance(ctx context.Context, orgID string, caller Caller, ref string) (*Exit, error) {
	if !caller.CanManage {
		return nil, ErrAccessDenied
	}
	e, err := s.loadExit(ctx, orgID, caller, ref)
	if err != nil {
		return nil, err
	}
	if e.Status != StatusInitiated {
		return nil, ErrWrongStatus
	}

	subject, err := s.repo.FindSubject(ctx, orgID, e.EmployeeID)
	if err != nil {
		return nil, fmt.Errorf("exits: StartClearance: %w", err)
	}
	if subject == nil {
		return nil, ErrExitNotFound
	}

	result, err := s.checklists.InstantiateDefault(ctx, orgID, checklists.ChecklistTypeOffboarding,
		checklists.SubjectContext{
			SubjectType:   checklists.SubjectTypeEmployee,
			SubjectID:     subject.EmployeeID,
			SubjectLabel:  subject.DisplayName,
			SubjectUserID: subject.UserID,
			ManagerUserID: subject.ManagerUserID,
			// The last working date, not the hire date: offboarding due
			// dates are offsets from the day somebody leaves.
			AnchorDate: e.LastWorkingDate,
			CreatedBy:  caller.UserID,
		})
	if err != nil {
		return nil, fmt.Errorf("exits: StartClearance: instantiate: %w", err)
	}
	if result != nil {
		e.ChecklistInstanceID = &result.Instance.ID
		if result.UnresolvedCount > 0 {
			slog.Warn("exits: offboarding checklist instantiated with unresolved item owners",
				slog.String("org_id", orgID), slog.String("exit_id", e.ID),
				slog.Int("unresolved_count", result.UnresolvedCount))
		}
	}
	// An org with no offboarding template still moves to in_clearance —
	// clearance items can be added by hand, and refusing here would make the
	// whole exit flow depend on a template nobody has configured yet.
	e.Status = StatusInClearance
	if err := s.repo.UpdateExit(ctx, e); err != nil {
		return nil, err
	}
	return e, nil
}

func (s *serviceImpl) AddClearanceItem(ctx context.Context, orgID string, caller Caller, ref string, req CreateClearanceItemRequest) (*ClearanceItem, error) {
	e, err := s.loadExit(ctx, orgID, caller, ref)
	if err != nil {
		return nil, err
	}
	if e.Status.IsTerminal() || e.Status == StatusSettled {
		return nil, ErrWrongStatus
	}
	dept := strings.TrimSpace(req.Department)
	desc := strings.TrimSpace(req.Description)
	if dept == "" || desc == "" {
		return nil, fmt.Errorf("exits: AddClearanceItem: department and description are required")
	}
	amount := decimal.Zero
	if req.BlockingAmount != nil && strings.TrimSpace(*req.BlockingAmount) != "" {
		amount, err = decimal.NewFromString(strings.TrimSpace(*req.BlockingAmount))
		if err != nil || amount.IsNegative() {
			return nil, ErrInvalidAmount
		}
	}
	currency := "BDT"
	if req.Currency != nil && strings.TrimSpace(*req.Currency) != "" {
		currency = strings.ToUpper(strings.TrimSpace(*req.Currency))
	}

	item := &ClearanceItem{
		ExitID: e.ID, ChecklistItemID: req.ChecklistItemID,
		Department: dept, Description: desc,
		BlockingAmount: amount, Currency: currency,
		CreatedBy: caller.UserID,
	}
	if err := s.repo.CreateClearanceItem(ctx, orgID, item); err != nil {
		return nil, err
	}
	return item, nil
}

func (s *serviceImpl) ListClearanceItems(ctx context.Context, orgID string, caller Caller, ref string) ([]*ClearanceItem, error) {
	e, err := s.loadExit(ctx, orgID, caller, ref)
	if err != nil {
		return nil, err
	}
	return s.repo.FindClearanceItems(ctx, orgID, e.ID)
}

func (s *serviceImpl) ResolveClearanceItem(ctx context.Context, orgID string, caller Caller, ref, itemRef string, req ResolveClearanceItemRequest) (*ClearanceItem, error) {
	e, err := s.loadExit(ctx, orgID, caller, ref)
	if err != nil {
		return nil, err
	}
	if e.Status.IsTerminal() {
		return nil, ErrWrongStatus
	}
	item, err := s.repo.FindClearanceItemByRef(ctx, orgID, e.ID, itemRef)
	if err != nil {
		return nil, fmt.Errorf("exits: ResolveClearanceItem: %w", err)
	}
	if item == nil {
		return nil, ErrClearanceItemNotFound
	}
	if item.IsResolved {
		return nil, ErrAlreadyResolved
	}

	now := time.Now()
	item.IsResolved = true
	item.ResolvedBy = &caller.UserID
	item.ResolvedAt = &now
	item.ResolutionNote = req.ResolutionNote
	// blocking_amount is deliberately NOT zeroed on a waiver. A forgiven debt
	// should still show what was forgiven — rewriting it to zero destroys the
	// only record that there was ever anything to forgive.
	_ = req.WaiveAmount

	if err := s.repo.UpdateClearanceItem(ctx, orgID, item); err != nil {
		return nil, err
	}
	return item, nil
}

// ── Rehire ───────────────────────────────────────────────────────────────────

func (s *serviceImpl) DecideRehire(ctx context.Context, orgID string, caller Caller, employeeRef string, req DecideRehireRequest) (*RehireEligibility, error) {
	status := RehireStatus(strings.TrimSpace(req.Status))
	if !status.IsValid() {
		return nil, ErrInvalidRehireStatus
	}
	subject, err := s.repo.FindSubject(ctx, orgID, employeeRef)
	if err != nil {
		return nil, fmt.Errorf("exits: DecideRehire: %w", err)
	}
	if subject == nil {
		return nil, ErrExitNotFound
	}
	re := &RehireEligibility{
		OrgID: orgID, EmployeeID: subject.EmployeeID,
		Status: status, Reason: req.Reason, DecidedBy: &caller.UserID,
	}
	if err := s.repo.UpsertRehireEligibility(ctx, re); err != nil {
		return nil, err
	}
	return re, nil
}

func (s *serviceImpl) GetRehire(ctx context.Context, orgID string, caller Caller, employeeRef string) (*RehireEligibility, error) {
	subject, err := s.repo.FindSubject(ctx, orgID, employeeRef)
	if err != nil {
		return nil, fmt.Errorf("exits: GetRehire: %w", err)
	}
	if subject == nil {
		return nil, ErrExitNotFound
	}
	ok, err := s.resolver.AuthorizeRecordAccess(ctx, caller.Scope, orgID, caller.UserID, subject.EmployeeID)
	if err != nil {
		return nil, fmt.Errorf("exits: GetRehire: %w", err)
	}
	if !ok {
		return nil, ErrExitNotFound
	}
	return s.repo.FindRehireEligibility(ctx, orgID, subject.EmployeeID)
}

// CheckRehireEligibility answers recruitment's question: "has this person
// left us before, and did we decide not to take them back?"
//
// Matching is by EMAIL, because a candidate is not yet an employee and there
// is no id to join on. It returns nil for the ordinary case — most candidates
// are strangers — and recruitment records a WARNING rather than blocking, so
// a wrongly-flagged ex-employee is not made unhireable with no override.
func (s *serviceImpl) CheckRehireEligibility(ctx context.Context, orgID, email string) (*RehireEligibility, error) {
	email = strings.TrimSpace(email)
	if email == "" {
		return nil, nil
	}
	re, err := s.repo.FindRehireEligibilityByEmail(ctx, orgID, email)
	if err != nil {
		return nil, fmt.Errorf("exits: CheckRehireEligibility: %w", err)
	}
	return re, nil
}
