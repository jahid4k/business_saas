// backend/internal/hrm/feedback/service.go
package feedback

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/shopspring/decimal"

	"github.com/mridha/businesssaas/internal/platform/forms"
)

type Service interface {
	// ── Cycles ──────────────────────────────────────────────────────────
	ListCycles(ctx context.Context, orgID string, f CycleListFilter) (*CycleListResponse, error)
	GetCycle(ctx context.Context, orgID, ref string) (*Cycle, error)
	CreateCycle(ctx context.Context, orgID, createdBy string, req CreateCycleRequest) (*Cycle, error)
	UpdateCycle(ctx context.Context, orgID, ref string, req UpdateCycleRequest) (*Cycle, error)
	ActivateCycle(ctx context.Context, orgID, ref string) (*Cycle, error)
	CloseCycle(ctx context.Context, orgID, ref string) (*Cycle, error)

	// ── Requests: coordination ──────────────────────────────────────────
	CreateRequests(ctx context.Context, orgID, cycleRef, createdBy string, req CreateRequestsRequest) ([]*RequestSummary, error)
	ListRequests(ctx context.Context, orgID string, caller Caller, f RequestListFilter) (*RequestListResponse, error)

	// ── Requests: responding ────────────────────────────────────────────
	ListMyRequests(ctx context.Context, orgID, userID string) (*MyRequestsResponse, error)
	SubmitResponse(ctx context.Context, orgID, ref string, caller Caller) (*MyRequest, error)
	DeclineRequest(ctx context.Context, orgID, ref string, caller Caller, req DeclineRequest) (*MyRequest, error)

	// ── The content path ────────────────────────────────────────────────
	GetAggregate(ctx context.Context, orgID, cycleRef, employeeRef string, caller Caller) (*Aggregate, error)
}

type serviceImpl struct {
	repo    Repository
	records RecordAuthorizer
	forms   FormReader
}

func NewService(repo Repository, records RecordAuthorizer, formReader FormReader) Service {
	return &serviceImpl{repo: repo, records: records, forms: formReader}
}

// ── Cycles ───────────────────────────────────────────────────────────────────

func (s *serviceImpl) ListCycles(ctx context.Context, orgID string, f CycleListFilter) (*CycleListResponse, error) {
	f.Normalise()
	cycles, err := s.repo.FindCycles(ctx, orgID, f)
	if err != nil {
		return nil, err
	}
	total, err := s.repo.CountCycles(ctx, orgID, f)
	if err != nil {
		return nil, err
	}
	return &CycleListResponse{Cycles: cycles, Total: total, Limit: f.Limit, Offset: f.Offset}, nil
}

func (s *serviceImpl) GetCycle(ctx context.Context, orgID, ref string) (*Cycle, error) {
	c, err := s.repo.FindCycleByRef(ctx, orgID, ref)
	if err != nil {
		return nil, err
	}
	if c == nil {
		return nil, ErrCycleNotFound
	}
	return c, nil
}

func (s *serviceImpl) CreateCycle(ctx context.Context, orgID, createdBy string, req CreateCycleRequest) (*Cycle, error) {
	name := strings.TrimSpace(req.Name)
	if name == "" {
		return nil, ErrCycleNameRequired
	}
	if strings.TrimSpace(req.FormTemplateID) == "" {
		return nil, ErrTemplateRequired
	}
	start, end, err := parsePeriod(req.PeriodStart, req.PeriodEnd)
	if err != nil {
		return nil, err
	}

	minResponses := 3
	if req.MinResponses != nil {
		if *req.MinResponses < 1 {
			return nil, ErrMinResponses
		}
		minResponses = *req.MinResponses
	}

	taken, err := s.repo.CycleNameExists(ctx, orgID, name, "")
	if err != nil {
		return nil, err
	}
	if taken {
		return nil, ErrCycleNameTaken
	}

	c := &Cycle{
		OrgID: orgID, Name: name, Description: nilIfBlank(req.Description),
		PeriodStart: start, PeriodEnd: end,
		FormTemplateID: strings.TrimSpace(req.FormTemplateID),
		MinResponses:   minResponses, CreatedBy: createdBy,
	}
	if err := s.repo.CreateCycle(ctx, c); err != nil {
		return nil, err
	}
	return c, nil
}

func (s *serviceImpl) UpdateCycle(ctx context.Context, orgID, ref string, req UpdateCycleRequest) (*Cycle, error) {
	c, err := s.GetCycle(ctx, orgID, ref)
	if err != nil {
		return nil, err
	}
	// A closed cycle's threshold is frozen. Lowering min_responses after the
	// fact would retroactively unsuppress groups respondents answered under a
	// stricter promise.
	if c.Status == CycleClosed {
		return nil, ErrCycleClosed
	}

	if req.Name != nil {
		name := strings.TrimSpace(*req.Name)
		if name == "" {
			return nil, ErrCycleNameRequired
		}
		taken, err := s.repo.CycleNameExists(ctx, orgID, name, c.ID)
		if err != nil {
			return nil, err
		}
		if taken {
			return nil, ErrCycleNameTaken
		}
		c.Name = name
	}
	if req.Description != nil {
		c.Description = nilIfBlank(req.Description)
	}
	if req.MinResponses != nil {
		if *req.MinResponses < 1 {
			return nil, ErrMinResponses
		}
		c.MinResponses = *req.MinResponses
	}
	if req.PeriodStart != nil || req.PeriodEnd != nil {
		startStr := c.PeriodStart.Format(dateLayout)
		endStr := c.PeriodEnd.Format(dateLayout)
		if req.PeriodStart != nil {
			startStr = *req.PeriodStart
		}
		if req.PeriodEnd != nil {
			endStr = *req.PeriodEnd
		}
		start, end, err := parsePeriod(startStr, endStr)
		if err != nil {
			return nil, err
		}
		c.PeriodStart, c.PeriodEnd = start, end
	}

	if err := s.repo.UpdateCycle(ctx, c); err != nil {
		return nil, err
	}
	return c, nil
}

func (s *serviceImpl) ActivateCycle(ctx context.Context, orgID, ref string) (*Cycle, error) {
	c, err := s.GetCycle(ctx, orgID, ref)
	if err != nil {
		return nil, err
	}
	if c.Status != CycleDraft {
		return nil, ErrCycleStatus
	}
	return s.repo.SetCycleStatus(ctx, orgID, c.ID, CycleActive)
}

func (s *serviceImpl) CloseCycle(ctx context.Context, orgID, ref string) (*Cycle, error) {
	c, err := s.GetCycle(ctx, orgID, ref)
	if err != nil {
		return nil, err
	}
	if c.Status != CycleActive {
		return nil, ErrCycleStatus
	}
	return s.repo.SetCycleStatus(ctx, orgID, c.ID, CycleClosed)
}

// ── Requests: coordination ───────────────────────────────────────────────────

// CreateRequests asks a batch of people about one subject, instantiating one
// form per respondent.
//
// The subject/respondent split is exactly why the form engine keeps them as
// separate columns: the SUBJECT is the person being reviewed, the RESPONDENT
// is whoever answers. Passing the respondent as the subject would file every
// response under the wrong person.
//
// Returns RequestSummary, not Request: even the coordinator who just created
// these never receives a form instance id from this package.
func (s *serviceImpl) CreateRequests(ctx context.Context, orgID, cycleRef, createdBy string, req CreateRequestsRequest) ([]*RequestSummary, error) {
	cycle, err := s.GetCycle(ctx, orgID, cycleRef)
	if err != nil {
		return nil, err
	}
	if cycle.Status != CycleActive {
		return nil, ErrCycleNotActive
	}
	if len(req.Respondents) == 0 {
		return nil, ErrNoRespondents
	}

	subject, err := s.repo.FindEmployeeSubject(ctx, orgID, strings.TrimSpace(req.SubjectEmployeeID))
	if err != nil {
		return nil, err
	}
	if subject == nil {
		return nil, ErrEmployeeNotFound
	}

	rows := make([]*Request, 0, len(req.Respondents))
	for _, spec := range req.Respondents {
		row, err := s.buildRequest(ctx, orgID, cycle, subject, createdBy, spec)
		if err != nil {
			return nil, err
		}
		rows = append(rows, row)
	}

	if err := s.repo.CreateRequests(ctx, rows); err != nil {
		return nil, err
	}

	out := make([]*RequestSummary, 0, len(rows))
	for _, row := range rows {
		out = append(out, &RequestSummary{
			ID: row.ID, PublicID: row.PublicID, RespondentName: row.RespondentName,
			Relationship: row.Relationship, Status: row.Status,
		})
	}
	return out, nil
}

// buildRequest resolves one respondent and instantiates their form. Split out
// because the resolution rules (internal vs external, self must be the
// subject) are the part worth reading on their own.
func (s *serviceImpl) buildRequest(ctx context.Context, orgID string, cycle *Cycle, subject *EmployeeSubject, createdBy string, spec RespondentSpec) (*Request, error) {
	if !spec.Relationship.IsValid() {
		return nil, ErrInvalidRelationship
	}

	row := &Request{
		OrgID: orgID, CycleID: cycle.ID, SubjectEmployeeID: subject.EmployeeID,
		Relationship: spec.Relationship, RequestedBy: createdBy,
	}

	switch {
	case spec.EmployeeID != nil && strings.TrimSpace(*spec.EmployeeID) != "":
		resp, err := s.repo.FindEmployeeSubject(ctx, orgID, strings.TrimSpace(*spec.EmployeeID))
		if err != nil {
			return nil, err
		}
		if resp == nil {
			return nil, ErrEmployeeNotFound
		}
		row.RespondentEmployeeID = &resp.EmployeeID
		row.RespondentUserID = resp.UserID
		row.RespondentName = resp.DisplayName
		row.RespondentEmail = resp.Email

	case spec.Email != nil && strings.TrimSpace(*spec.Email) != "" &&
		spec.Name != nil && strings.TrimSpace(*spec.Name) != "":
		email := strings.TrimSpace(*spec.Email)
		row.RespondentEmail = &email
		row.RespondentName = strings.TrimSpace(*spec.Name)

	default:
		return nil, ErrRespondentRequired
	}

	// The DB CHECK enforces this too; raising it here turns a 23514 into a
	// message that says which respondent was wrong.
	if spec.Relationship == RelationshipSelf &&
		(row.RespondentEmployeeID == nil || *row.RespondentEmployeeID != subject.EmployeeID) {
		return nil, ErrSelfMismatch
	}

	dup, err := s.repo.RequestExists(ctx, cycle.ID, subject.EmployeeID, row.RespondentEmployeeID, row.RespondentEmail)
	if err != nil {
		return nil, err
	}
	if dup {
		return nil, fmt.Errorf("%w: %s", ErrDuplicateRequest, row.RespondentName)
	}

	if s.forms != nil {
		inst, err := s.forms.Instantiate(ctx, orgID, cycle.FormTemplateID, forms.SubjectContext{
			SubjectType:  forms.SubjectEmployee,
			SubjectID:    subject.EmployeeID,
			SubjectLabel: subject.DisplayName,
			// Nil for an external respondent with no platform account. The
			// form still exists and is answerable by a forms.manage holder.
			RespondentUserID: row.RespondentUserID,
			RespondentRole:   string(spec.Relationship),
			CreatedBy:        createdBy,
		})
		if err != nil {
			return nil, fmt.Errorf("feedback: instantiate %s form: %w", spec.Relationship, err)
		}
		row.FormInstanceID = &inst.ID
	}
	return row, nil
}

func (s *serviceImpl) ListRequests(ctx context.Context, orgID string, caller Caller, f RequestListFilter) (*RequestListResponse, error) {
	// The coordination path is gated a second time here, not only at the
	// route. The route gate proves the caller holds hrm.feedback.coordinate;
	// this proves the service was not reached another way.
	if !caller.CanCoordinate {
		return nil, ErrAccessDenied
	}
	f.Normalise()
	f.Scope = caller.Tier
	f.CallerUserID = caller.UserID

	reqs, err := s.repo.FindRequestSummaries(ctx, orgID, f)
	if err != nil {
		return nil, err
	}
	total, err := s.repo.CountRequests(ctx, orgID, f)
	if err != nil {
		return nil, err
	}
	return &RequestListResponse{Requests: reqs, Total: total, Limit: f.Limit, Offset: f.Offset}, nil
}

// ── Requests: responding ─────────────────────────────────────────────────────

func (s *serviceImpl) ListMyRequests(ctx context.Context, orgID, userID string) (*MyRequestsResponse, error) {
	reqs, err := s.repo.FindRequestsForRespondent(ctx, orgID, userID)
	if err != nil {
		return nil, err
	}
	return &MyRequestsResponse{Requests: reqs, Total: len(reqs)}, nil
}

// SubmitResponse marks a request answered. The form itself is submitted
// through the form engine's own endpoint; this records that the ask is
// discharged, which is what the coordination view and the suppression count
// both read.
func (s *serviceImpl) SubmitResponse(ctx context.Context, orgID, ref string, caller Caller) (*MyRequest, error) {
	row, err := s.loadOwnRequest(ctx, orgID, ref, caller)
	if err != nil {
		return nil, err
	}
	if row.Status == RequestSubmitted {
		return nil, ErrAlreadySubmitted
	}
	if row.Status != RequestPending {
		return nil, ErrRequestClosed
	}
	updated, err := s.repo.SetRequestSubmitted(ctx, orgID, row.ID)
	if err != nil {
		return nil, err
	}
	return s.toMyRequest(ctx, orgID, updated)
}

func (s *serviceImpl) DeclineRequest(ctx context.Context, orgID, ref string, caller Caller, req DeclineRequest) (*MyRequest, error) {
	row, err := s.loadOwnRequest(ctx, orgID, ref, caller)
	if err != nil {
		return nil, err
	}
	if row.Status != RequestPending {
		return nil, ErrRequestClosed
	}
	updated, err := s.repo.SetRequestDeclined(ctx, orgID, row.ID, nilIfBlank(req.Reason))
	if err != nil {
		return nil, err
	}
	return s.toMyRequest(ctx, orgID, updated)
}

// loadOwnRequest narrows to the request's own respondent. hrm.feedback.respond
// reaches every member, so the route gate cannot express "is this YOUR
// request" — this is where that is decided. The platform.forms.respond
// precedent.
func (s *serviceImpl) loadOwnRequest(ctx context.Context, orgID, ref string, caller Caller) (*Request, error) {
	row, err := s.repo.FindRequestByRef(ctx, orgID, ref)
	if err != nil {
		return nil, err
	}
	if row == nil {
		return nil, ErrRequestNotFound
	}
	if row.RespondentUserID == nil || *row.RespondentUserID != caller.UserID {
		return nil, ErrNotRespondent
	}
	return row, nil
}

func (s *serviceImpl) toMyRequest(ctx context.Context, orgID string, row *Request) (*MyRequest, error) {
	cycle, err := s.repo.FindCycleByRef(ctx, orgID, row.CycleID)
	if err != nil {
		return nil, err
	}
	m := &MyRequest{
		ID: row.ID, PublicID: row.PublicID, CycleID: row.CycleID,
		SubjectEmployeeID: row.SubjectEmployeeID, Relationship: row.Relationship,
		Status: row.Status, FormInstanceID: row.FormInstanceID, SubmittedAt: row.SubmittedAt,
	}
	if cycle != nil {
		m.CycleName, m.PeriodEnd = cycle.Name, cycle.PeriodEnd
	}
	if subj, err := s.repo.FindEmployeeSubject(ctx, orgID, row.SubjectEmployeeID); err == nil && subj != nil {
		m.SubjectName = subj.DisplayName
	}
	return m, nil
}

// ── The content path ─────────────────────────────────────────────────────────

// GetAggregate is the subject-facing read: what was said, grouped by
// relationship, with anonymous groups suppressed below the cycle's threshold.
//
// Note what this method never does: it never returns a respondent, and it
// never returns a form instance id. It fetches instances server-side through
// FormReader precisely so the caller cannot.
//
// Authorization is the scope tier, applied per-record — the same control the
// list path uses, applied here to a single subject. An hrm.feedback.view_all
// holder still receives SUPPRESSED groups: an "admin sees everything"
// exception would make the promise to respondents false, and a promise of
// anonymity that is false for one role is false.
func (s *serviceImpl) GetAggregate(ctx context.Context, orgID, cycleRef, employeeRef string, caller Caller) (*Aggregate, error) {
	cycle, err := s.GetCycle(ctx, orgID, cycleRef)
	if err != nil {
		return nil, err
	}
	subject, err := s.repo.FindEmployeeSubject(ctx, orgID, strings.TrimSpace(employeeRef))
	if err != nil {
		return nil, err
	}
	if subject == nil {
		return nil, ErrEmployeeNotFound
	}

	ok, err := s.records.AuthorizeRecordAccess(ctx, caller.Tier, orgID, caller.UserID, subject.EmployeeID)
	if err != nil {
		return nil, fmt.Errorf("feedback: GetAggregate: authorize: %w", err)
	}
	if !ok {
		return nil, ErrAccessDenied
	}

	refs, err := s.repo.FindSubmittedForSubject(ctx, orgID, cycle.ID, subject.EmployeeID)
	if err != nil {
		return nil, err
	}

	byRelationship := map[Relationship][]*SubmittedRef{}
	order := make([]Relationship, 0, 5)
	for _, ref := range refs {
		if _, seen := byRelationship[ref.Relationship]; !seen {
			order = append(order, ref.Relationship)
		}
		byRelationship[ref.Relationship] = append(byRelationship[ref.Relationship], ref)
	}

	agg := &Aggregate{
		CycleID: cycle.ID, SubjectEmployeeID: subject.EmployeeID,
		Groups: make([]RelationshipGroup, 0, len(order)),
	}
	for _, rel := range order {
		group, err := s.buildGroup(ctx, orgID, rel, byRelationship[rel], cycle.MinResponses)
		if err != nil {
			return nil, err
		}
		agg.Groups = append(agg.Groups, group)
		if !group.Suppressed {
			agg.TotalResponses += group.ResponseCount
		}
	}
	return agg, nil
}

// buildGroup assembles one relationship's results, applying suppression.
//
// The suppression check comes FIRST and returns early. Assembling the content
// and then filtering it is the shape that leaks: every later edit to the
// assembly code is one `return` away from shipping suppressed content.
func (s *serviceImpl) buildGroup(ctx context.Context, orgID string, rel Relationship, refs []*SubmittedRef, minResponses int) (RelationshipGroup, error) {
	group := RelationshipGroup{Relationship: rel, MinResponses: minResponses}

	// Attributed groups (self, manager) are not subject to a threshold: there
	// is exactly one such respondent and the subject already knows who they
	// are. See Relationship.IsAnonymous.
	if rel.IsAnonymous() && len(refs) < minResponses {
		group.Suppressed = true
		// Nothing else is set — not even ResponseCount, which is itself a
		// signal when combined with any knowledge of who was asked.
		return group, nil
	}

	group.ResponseCount = len(refs)
	group.Responses = make([]AnonymousResponse, 0, len(refs))

	total, scored := decimal.Zero, 0
	for _, ref := range refs {
		if ref.FormInstanceID == nil || s.forms == nil {
			continue
		}
		inst, err := s.forms.GetInstance(ctx, orgID, *ref.FormInstanceID)
		if err != nil {
			return group, fmt.Errorf("feedback: read response: %w", err)
		}
		group.Responses = append(group.Responses, AnonymousResponse{
			Relationship: rel, Answers: stripAnswers(inst),
		})

		score, err := s.forms.ScoreInstance(ctx, orgID, *ref.FormInstanceID)
		if err != nil {
			return group, fmt.Errorf("feedback: score response: %w", err)
		}
		if score.ScoredCount > 0 {
			total = total.Add(score.Percent)
			scored++
		}
	}
	if scored > 0 {
		avg := total.Div(decimal.NewFromInt(int64(scored))).Round(2)
		group.AverageScore = &avg
	}
	return group, nil
}

// stripAnswers converts a form instance to anonymised answers.
//
// It copies FIELD BY FIELD rather than embedding the response, so a column
// added to platform_form_responses in a later phase does not silently appear
// on this path. The cost is one line per new answer type; the alternative is
// a leak nobody notices.
func stripAnswers(inst *forms.InstanceWithResponses) []AnonymousAnswer {
	if inst == nil {
		return nil
	}
	out := make([]AnonymousAnswer, 0, len(inst.Responses))
	for _, r := range inst.Responses {
		out = append(out, AnonymousAnswer{
			QuestionText:  r.QuestionText,
			QuestionType:  string(r.QuestionType),
			AnswerText:    r.AnswerText,
			AnswerNumber:  r.AnswerNumber,
			AnswerBoolean: r.AnswerBoolean,
			AnswerDate:    formatDate(r.AnswerDate),
			AnswerOptions: r.AnswerOptions,
		})
	}
	return out
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

// formatDate renders a form engine date as the ISO string this package's
// responses use, so a client parses one date format across the whole API.
func formatDate(t *time.Time) *string {
	if t == nil {
		return nil
	}
	s := t.Format(dateLayout)
	return &s
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
