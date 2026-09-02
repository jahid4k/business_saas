// backend/internal/hrm/pip/service.go
package pip

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"
)

type Service interface {
	List(ctx context.Context, orgID string, caller Caller, f ListFilter) (*ListResponse, error)
	Get(ctx context.Context, orgID, ref string, caller Caller) (*Detail, error)
	Create(ctx context.Context, orgID, createdBy string, caller Caller, req CreateRequest) (*Detail, error)
	Update(ctx context.Context, orgID, ref string, caller Caller, req UpdateRequest) (*Detail, error)
	Activate(ctx context.Context, orgID, ref string, caller Caller) (*Detail, error)
	Cancel(ctx context.Context, orgID, ref string, caller Caller) (*Detail, error)

	AddCheckin(ctx context.Context, orgID, ref string, caller Caller, req CheckinRequest) (*Detail, error)
	Extend(ctx context.Context, orgID, ref string, caller Caller, req ExtendRequest) (*Detail, error)
	// Close records the outcome. A 'failed' outcome additionally creates a
	// DRAFT termination through TerminationCreator and stops.
	Close(ctx context.Context, orgID, ref string, caller Caller, req CloseRequest) (*Detail, error)
}

type serviceImpl struct {
	repo         Repository
	records      RecordAuthorizer
	terminations TerminationCreator
}

// NewService takes TerminationCreator as an interface, nil-tolerant. A
// deployment that has not wired terminations still runs; a failed PIP simply
// closes without producing a draft, and says so through ErrTerminationHandoff
// rather than silently doing nothing.
func NewService(repo Repository, records RecordAuthorizer, terminations TerminationCreator) Service {
	return &serviceImpl{repo: repo, records: records, terminations: terminations}
}

// ── Reads ────────────────────────────────────────────────────────────────────

func (s *serviceImpl) List(ctx context.Context, orgID string, caller Caller, f ListFilter) (*ListResponse, error) {
	f.Normalise()
	f.Scope = caller.Tier
	f.CallerUserID = caller.UserID

	plans, err := s.repo.Find(ctx, orgID, f)
	if err != nil {
		return nil, err
	}
	total, err := s.repo.Count(ctx, orgID, f)
	if err != nil {
		return nil, err
	}
	return &ListResponse{PIPs: plans, Total: total, Limit: f.Limit, Offset: f.Offset}, nil
}

func (s *serviceImpl) Get(ctx context.Context, orgID, ref string, caller Caller) (*Detail, error) {
	p, err := s.load(ctx, orgID, ref, caller)
	if err != nil {
		return nil, err
	}
	return s.hydrate(ctx, p)
}

// load fetches org-scoped, then narrows by the caller's scope tier against
// the plan's own employee. The list path filters in SQL; this is the
// fetch-by-id half of the same control.
func (s *serviceImpl) load(ctx context.Context, orgID, ref string, caller Caller) (*PIP, error) {
	p, err := s.repo.FindByRef(ctx, orgID, ref)
	if err != nil {
		return nil, err
	}
	if p == nil {
		return nil, ErrNotFound
	}
	ok, err := s.records.AuthorizeRecordAccess(ctx, caller.Tier, orgID, caller.UserID, p.EmployeeID)
	if err != nil {
		return nil, fmt.Errorf("pip: authorize: %w", err)
	}
	if !ok {
		return nil, ErrAccessDenied
	}
	return p, nil
}

func (s *serviceImpl) hydrate(ctx context.Context, p *PIP) (*Detail, error) {
	checkins, err := s.repo.FindCheckins(ctx, p.ID)
	if err != nil {
		return nil, err
	}
	if checkins == nil {
		checkins = []*Checkin{}
	}
	return &Detail{
		PIP:           p,
		Checkins:      checkins,
		WasExtended:   p.WasExtended(),
		DaysRemaining: daysRemaining(p.EndDate),
	}, nil
}

// ── Writes ───────────────────────────────────────────────────────────────────

// authorizeWrite narrows a write. hrm.pips.manage is unscoped at the route,
// so this is the only thing stopping a view_team manager opening a plan on
// someone outside their reporting line.
func (s *serviceImpl) authorizeWrite(ctx context.Context, orgID, employeeID string, caller Caller) error {
	if !caller.CanManage {
		return ErrAccessDenied
	}
	ok, err := s.records.AuthorizeRecordAccess(ctx, caller.Tier, orgID, caller.UserID, employeeID)
	if err != nil {
		return fmt.Errorf("pip: authorize write: %w", err)
	}
	if !ok {
		return ErrAccessDenied
	}
	return nil
}

func (s *serviceImpl) Create(ctx context.Context, orgID, createdBy string, caller Caller, req CreateRequest) (*Detail, error) {
	title := strings.TrimSpace(req.Title)
	if title == "" {
		return nil, ErrTitleRequired
	}
	concerns := strings.TrimSpace(req.Concerns)
	if concerns == "" {
		return nil, ErrConcernsRequired
	}
	criteria := strings.TrimSpace(req.SuccessCriteria)
	if criteria == "" {
		return nil, ErrCriteriaRequired
	}
	start, end, err := parsePeriod(req.StartDate, req.EndDate)
	if err != nil {
		return nil, err
	}

	emp, err := s.repo.FindEmployeeRef(ctx, orgID, strings.TrimSpace(req.EmployeeID))
	if err != nil {
		return nil, err
	}
	if emp == nil {
		return nil, ErrEmployeeNotFound
	}
	if err := s.authorizeWrite(ctx, orgID, emp.EmployeeID, caller); err != nil {
		return nil, err
	}

	// The partial unique index is the guarantee; this is the friendly
	// message. Both read the same definition of "open" — Status.IsOpen.
	open, err := s.repo.HasOpenPlan(ctx, orgID, emp.EmployeeID)
	if err != nil {
		return nil, err
	}
	if open {
		return nil, ErrAlreadyOpen
	}

	p := &PIP{
		OrgID: orgID, EmployeeID: emp.EmployeeID,
		// Frozen here: a reorg must not silently reassign responsibility for
		// someone's dismissal process.
		ManagerEmployeeID: emp.ManagerEmployeeID,
		Title:             title, Concerns: concerns, SuccessCriteria: criteria,
		SupportProvided: nilIfBlank(req.SupportProvided),
		StartDate:       start, EndDate: end,
		CreatedBy: createdBy,
	}
	if err := s.repo.Create(ctx, p); err != nil {
		return nil, err
	}
	return s.hydrate(ctx, p)
}

func (s *serviceImpl) Update(ctx context.Context, orgID, ref string, caller Caller, req UpdateRequest) (*Detail, error) {
	p, err := s.load(ctx, orgID, ref, caller)
	if err != nil {
		return nil, err
	}
	if err := s.authorizeWrite(ctx, orgID, p.EmployeeID, caller); err != nil {
		return nil, err
	}
	if !p.Status.IsOpen() {
		return nil, ErrNotOpen
	}

	if req.Title != nil {
		t := strings.TrimSpace(*req.Title)
		if t == "" {
			return nil, ErrTitleRequired
		}
		p.Title = t
	}
	if req.Concerns != nil {
		t := strings.TrimSpace(*req.Concerns)
		if t == "" {
			return nil, ErrConcernsRequired
		}
		p.Concerns = t
	}
	if req.SuccessCriteria != nil {
		t := strings.TrimSpace(*req.SuccessCriteria)
		if t == "" {
			return nil, ErrCriteriaRequired
		}
		p.SuccessCriteria = t
	}
	if req.SupportProvided != nil {
		p.SupportProvided = nilIfBlank(req.SupportProvided)
	}
	if req.StartDate != nil {
		start, err := time.Parse(dateLayout, strings.TrimSpace(*req.StartDate))
		if err != nil {
			return nil, ErrInvalidDate
		}
		if p.EndDate.Before(start) {
			return nil, ErrInvalidPeriod
		}
		p.StartDate = start
	}

	// Note what is absent: end_date. It moves only through Extend, which
	// forces a written reason into the same transaction.
	if err := s.repo.Update(ctx, p); err != nil {
		return nil, err
	}
	return s.hydrate(ctx, p)
}

func (s *serviceImpl) Activate(ctx context.Context, orgID, ref string, caller Caller) (*Detail, error) {
	p, err := s.load(ctx, orgID, ref, caller)
	if err != nil {
		return nil, err
	}
	if err := s.authorizeWrite(ctx, orgID, p.EmployeeID, caller); err != nil {
		return nil, err
	}
	if p.Status != StatusDraft {
		return nil, ErrNotOpen
	}
	updated, err := s.repo.SetStatus(ctx, orgID, p.ID, StatusActive)
	if err != nil {
		return nil, err
	}
	return s.hydrate(ctx, updated)
}

// Cancel withdraws a plan without recording an outcome. Distinct from closing
// as 'abandoned': cancelling says the plan should not have been opened,
// abandoning says it ran and was dropped. Both are real, and conflating them
// loses the difference.
func (s *serviceImpl) Cancel(ctx context.Context, orgID, ref string, caller Caller) (*Detail, error) {
	p, err := s.load(ctx, orgID, ref, caller)
	if err != nil {
		return nil, err
	}
	if err := s.authorizeWrite(ctx, orgID, p.EmployeeID, caller); err != nil {
		return nil, err
	}
	if !p.Status.IsOpen() {
		return nil, ErrNotOpen
	}
	updated, err := s.repo.SetStatus(ctx, orgID, p.ID, StatusCancelled)
	if err != nil {
		return nil, err
	}
	return s.hydrate(ctx, updated)
}

func (s *serviceImpl) AddCheckin(ctx context.Context, orgID, ref string, caller Caller, req CheckinRequest) (*Detail, error) {
	p, err := s.load(ctx, orgID, ref, caller)
	if err != nil {
		return nil, err
	}
	if err := s.authorizeWrite(ctx, orgID, p.EmployeeID, caller); err != nil {
		return nil, err
	}
	if p.Status != StatusActive && p.Status != StatusExtended {
		return nil, ErrNotActive
	}
	note := strings.TrimSpace(req.Note)
	if note == "" {
		return nil, ErrNoteRequired
	}
	if req.Progress != nil && !req.Progress.IsValid() {
		return nil, ErrInvalidProgress
	}

	ch := &Checkin{PIPID: p.ID, EntryType: EntryReview, Progress: req.Progress, Note: note}
	if caller.UserID != "" {
		ch.CheckedInBy = &caller.UserID
	}
	if err := s.repo.CreateCheckin(ctx, ch); err != nil {
		return nil, err
	}
	return s.hydrate(ctx, p)
}

// Extend moves the end date later and records why, in one transaction. An
// extension with no reason and a reason with no extension are both states
// this refuses to produce.
func (s *serviceImpl) Extend(ctx context.Context, orgID, ref string, caller Caller, req ExtendRequest) (*Detail, error) {
	p, err := s.load(ctx, orgID, ref, caller)
	if err != nil {
		return nil, err
	}
	if err := s.authorizeWrite(ctx, orgID, p.EmployeeID, caller); err != nil {
		return nil, err
	}
	if p.Status != StatusActive && p.Status != StatusExtended {
		return nil, ErrNotActive
	}
	note := strings.TrimSpace(req.Note)
	if note == "" {
		return nil, ErrNoteRequired
	}
	newEnd, err := time.Parse(dateLayout, strings.TrimSpace(req.NewEndDate))
	if err != nil {
		return nil, ErrInvalidDate
	}
	// chk_hrm_pip_extension enforces end_date >= original_end_date, but the
	// rule callers care about is stricter: an extension moves the date
	// FORWARD from where it is now.
	if !newEnd.After(dateOnly(p.EndDate)) {
		return nil, ErrExtensionBackwards
	}

	previous := p.EndDate
	ch := &Checkin{
		PIPID: p.ID, EntryType: EntryExtension, Note: note,
		PreviousEndDate: &previous, NewEndDate: &newEnd,
	}
	if caller.UserID != "" {
		ch.CheckedInBy = &caller.UserID
	}
	p.EndDate = newEnd

	if err := s.repo.ExtendWithCheckin(ctx, orgID, p, ch); err != nil {
		return nil, err
	}
	return s.hydrate(ctx, p)
}

// Close records the outcome, and for a 'failed' outcome hands off to
// terminations.
//
// The ordering is deliberate and is the interesting part. The PIP closes
// FIRST, in its own transaction, and the draft termination is created after
// it commits. The two are not atomic, and cannot usefully be: hrm_terminations
// is a different module with its own transaction, and the alternative —
// holding the PIP's transaction open across a call into another service — is
// the shape that produces lock contention on the employee row and, worse,
// leaves the whole thing rolled back because a downstream approval-chain
// lookup failed.
//
// The failure mode that leaves is a closed PIP with no draft termination.
// That is recoverable by hand and is reported as ErrTerminationHandoff rather
// than being swallowed. The opposite ordering would risk a draft termination
// with no closed PIP behind it, which is a dismissal document with no
// process attached — strictly worse.
func (s *serviceImpl) Close(ctx context.Context, orgID, ref string, caller Caller, req CloseRequest) (*Detail, error) {
	p, err := s.load(ctx, orgID, ref, caller)
	if err != nil {
		return nil, err
	}
	// Closing is its own permission, NOT covered by manage: closing as
	// 'failed' is the moment this stops being a developmental instrument.
	// 'manager' holds manage and not close.
	if !caller.CanClose {
		return nil, ErrCloseDenied
	}
	ok, err := s.records.AuthorizeRecordAccess(ctx, caller.Tier, orgID, caller.UserID, p.EmployeeID)
	if err != nil {
		return nil, fmt.Errorf("pip: Close: authorize: %w", err)
	}
	if !ok {
		return nil, ErrAccessDenied
	}

	if p.Status == StatusClosed {
		return nil, ErrAlreadyClosed
	}
	if p.Status != StatusActive && p.Status != StatusExtended {
		return nil, ErrNotActive
	}
	if req.Outcome == "" {
		return nil, ErrOutcomeRequired
	}
	if !req.Outcome.IsValid() {
		return nil, ErrInvalidOutcome
	}
	note := strings.TrimSpace(req.Note)
	if note == "" {
		return nil, ErrNoteRequired
	}

	outcome := req.Outcome
	p.Outcome = &outcome
	if caller.UserID != "" {
		p.ClosedBy = &caller.UserID
	}
	ch := &Checkin{PIPID: p.ID, EntryType: EntryClosure, Note: note}
	if caller.UserID != "" {
		ch.CheckedInBy = &caller.UserID
	}
	if err := s.repo.CloseWithCheckin(ctx, orgID, p, ch); err != nil {
		return nil, err
	}

	if outcome != OutcomeFailed {
		return s.hydrate(ctx, p)
	}

	// ── The handoff ─────────────────────────────────────────────────────
	// A DRAFT, and nothing further. Submit and Apply stay with HR on the
	// existing termination endpoints, behind the approval chain that exists
	// specifically to gate dismissals.
	if s.terminations == nil {
		detail, herr := s.hydrate(ctx, p)
		if herr != nil {
			return nil, herr
		}
		return detail, ErrTerminationHandoff
	}

	lastWorking := p.EndDate.Format(dateLayout)
	if req.LastWorkingDate != nil && strings.TrimSpace(*req.LastWorkingDate) != "" {
		lastWorking = strings.TrimSpace(*req.LastWorkingDate)
	}
	terminationID, err := s.terminations.CreateDraftFromPIP(ctx, orgID, p.EmployeeID, caller.UserID,
		DraftTerminationRequest{
			TerminationDate: p.EndDate.Format(dateLayout),
			LastWorkingDate: lastWorking,
			Reason:          fmt.Sprintf("Performance improvement plan %s closed as failed", p.PublicID),
		})
	if err != nil {
		detail, herr := s.hydrate(ctx, p)
		if herr != nil {
			return nil, herr
		}
		// The PIP IS closed. Returning both the record and the error lets a
		// handler report the partial success honestly rather than implying
		// nothing happened.
		return detail, fmt.Errorf("%w: %v", ErrTerminationHandoff, err)
	}

	if err := s.repo.LinkTermination(ctx, orgID, p.ID, terminationID); err != nil {
		return nil, err
	}
	p.TerminationID = &terminationID
	return s.hydrate(ctx, p)
}

// ── Helpers ──────────────────────────────────────────────────────────────────

func parsePeriod(startStr, endStr string) (time.Time, time.Time, error) {
	start, err := time.Parse(dateLayout, strings.TrimSpace(startStr))
	if err != nil {
		return time.Time{}, time.Time{}, ErrInvalidDate
	}
	end, err := time.Parse(dateLayout, strings.TrimSpace(endStr))
	if err != nil {
		return time.Time{}, time.Time{}, ErrInvalidDate
	}
	if end.Before(start) {
		return time.Time{}, time.Time{}, ErrInvalidPeriod
	}
	return start, end, nil
}

// daysRemaining is negative once the end date has passed, which is the state
// a manager most needs to see.
func daysRemaining(end time.Time) int {
	return int(dateOnly(end).Sub(dateOnly(time.Now())).Hours() / 24)
}

func nilIfBlank(s *string) *string {
	if s == nil {
		return nil
	}
	t := strings.TrimSpace(*s)
	if t == "" {
		return nil
	}
	return &t
}

// IsHandoffFailure reports whether an error from Close means "the plan closed
// but the draft termination did not appear". Exposed so a handler can return
// the record alongside a warning rather than a bare error.
func IsHandoffFailure(err error) bool { return errors.Is(err, ErrTerminationHandoff) }
