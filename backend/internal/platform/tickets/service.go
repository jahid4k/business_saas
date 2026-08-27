// backend/internal/platform/tickets/service.go
package tickets

import (
	"context"
	"fmt"
	"strings"
	"time"
)

// Service is the ticket engine's business layer.
//
// Every method takes callerUserID and resolves the caller's own reach from
// the AccessDirectory. There is no authz.Scope anywhere in this package: the
// hrm scope tiers are backed by scope.Predicate, which hard-codes
// FROM hrm_employees, and a platform primitive must not reach into hrm_*.
// "See only my own" is expressed as ListFilter.CanViewAll instead, resolved
// once here and applied in SQL — the platform.checklists.complete precedent,
// where the route gate cannot express "is this yours" so the service does.
type Service interface {
	// Categories
	ListCategories(ctx context.Context, orgID, callerUserID string, activeOnly bool) ([]*Category, error)
	CreateCategory(ctx context.Context, orgID, callerUserID string, req CreateCategoryRequest) (*Category, error)
	UpdateCategory(ctx context.Context, orgID, callerUserID, ref string, req CreateCategoryRequest, isActive *bool) (*Category, error)

	// SLA policies
	ListPolicies(ctx context.Context, orgID, callerUserID string) ([]*SLAPolicy, error)
	CreatePolicy(ctx context.Context, orgID, callerUserID string, req CreateSLAPolicyRequest) (*SLAPolicy, error)
	UpdatePolicy(ctx context.Context, orgID, callerUserID, ref string, req CreateSLAPolicyRequest) (*SLAPolicy, error)

	// Tickets
	Create(ctx context.Context, orgID, callerUserID string, req CreateTicketRequest) (*Ticket, error)
	List(ctx context.Context, orgID, callerUserID string, f ListFilter) (*TicketListResponse, error)
	Get(ctx context.Context, orgID, callerUserID, ref string) (*Ticket, error)
	Assign(ctx context.Context, orgID, callerUserID, ref string, req AssignTicketRequest) (*Ticket, error)
	Resolve(ctx context.Context, orgID, callerUserID, ref string) (*Ticket, error)
	Close(ctx context.Context, orgID, callerUserID, ref string) (*Ticket, error)
	Cancel(ctx context.Context, orgID, callerUserID, ref string) (*Ticket, error)

	// SLA clock
	Pause(ctx context.Context, orgID, callerUserID, ref string, req PauseTicketRequest) (*Ticket, error)
	Resume(ctx context.Context, orgID, callerUserID, ref string) (*Ticket, error)

	// Comments
	AddComment(ctx context.Context, orgID, callerUserID, ref string, req CreateCommentRequest) (*Comment, error)
	ListComments(ctx context.Context, orgID, callerUserID, ref string) ([]*Comment, error)

	// MarkConverted records the ONE-WAY conversion of a ticket into something
	// carrying more weight — today an HR complaint. Called by the HRM side,
	// which reads the ticket, creates the complaint, then calls back:
	// hrm → platform is the allowed direction, and this package must never
	// import hrm to close the loop itself. It is not exposed as a route,
	// exactly like checklists' Instantiate: a generic HTTP endpoint would
	// have to trust a client-supplied converted_to_id, letting a caller
	// point a ticket at a complaint that does not exist or is not theirs.
	MarkConverted(ctx context.Context, orgID, callerUserID, ref, targetType, targetID string) error
}

type serviceImpl struct {
	repo      Repository
	directory AccessDirectory
}

func NewService(repo Repository, directory AccessDirectory) Service {
	return &serviceImpl{repo: repo, directory: directory}
}

// ── Access helpers ───────────────────────────────────────────────────────────

func (s *serviceImpl) can(ctx context.Context, orgID, userID, resource, action string) (bool, error) {
	ok, err := s.directory.Can(ctx, userID, orgID, resource, action)
	if err != nil {
		return false, fmt.Errorf("tickets: access check %s.%s: %w", resource, action, err)
	}
	return ok, nil
}

func (s *serviceImpl) require(ctx context.Context, orgID, userID, resource, action string) error {
	ok, err := s.can(ctx, orgID, userID, resource, action)
	if err != nil {
		return err
	}
	if !ok {
		return ErrAccessDenied
	}
	return nil
}

// isAgent reports whether the caller works tickets rather than merely raising
// them. It gates the internal-comment read path and the agent-only fields.
func (s *serviceImpl) isAgent(ctx context.Context, orgID, userID string) (bool, error) {
	for _, action := range []string{"view_all", "assign", "resolve"} {
		ok, err := s.can(ctx, orgID, userID, "platform.tickets", action)
		if err != nil {
			return false, err
		}
		if ok {
			return true, nil
		}
	}
	return false, nil
}

// visible reports whether the caller may see this specific ticket: they raised
// it, it is assigned to them, or they hold .view_all. The same rule
// ListFilter applies in SQL, restated for single-row reads — a filtered list
// that hides a ticket is worthless if fetching it by id returns it anyway.
func (s *serviceImpl) visible(ctx context.Context, orgID, userID string, t *Ticket) (bool, error) {
	if t.RequesterUserID == userID {
		return true, nil
	}
	if t.AssigneeUserID != nil && *t.AssigneeUserID == userID {
		return true, nil
	}
	return s.can(ctx, orgID, userID, "platform.tickets", "view_all")
}

// loadTicket fetches a ticket the caller is allowed to see, or returns
// ErrTicketNotFound. Invisible reads report not-found rather than denied:
// "you may not see ticket X" still confirms ticket X exists, and in a
// helpdesk carrying harassment reports that is itself a disclosure.
func (s *serviceImpl) loadTicket(ctx context.Context, orgID, callerUserID, ref string) (*Ticket, error) {
	if err := s.require(ctx, orgID, callerUserID, "platform.tickets", "view"); err != nil {
		return nil, err
	}
	t, err := s.repo.FindTicketByRef(ctx, orgID, ref)
	if err != nil {
		return nil, fmt.Errorf("tickets: load %s: %w", ref, err)
	}
	if t == nil {
		return nil, ErrTicketNotFound
	}
	ok, err := s.visible(ctx, orgID, callerUserID, t)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, ErrTicketNotFound
	}
	return t, nil
}

// ── Categories ───────────────────────────────────────────────────────────────

func (s *serviceImpl) ListCategories(ctx context.Context, orgID, callerUserID string, activeOnly bool) ([]*Category, error) {
	// Anyone who can raise a ticket needs to see the categories to pick one,
	// so this reads through tickets.view rather than ticket_config.view.
	if err := s.require(ctx, orgID, callerUserID, "platform.tickets", "view"); err != nil {
		return nil, err
	}
	return s.repo.FindCategories(ctx, orgID, activeOnly)
}

func (s *serviceImpl) CreateCategory(ctx context.Context, orgID, callerUserID string, req CreateCategoryRequest) (*Category, error) {
	if err := s.require(ctx, orgID, callerUserID, "platform.ticket_config", "manage"); err != nil {
		return nil, err
	}
	name := strings.TrimSpace(req.Name)
	if name == "" {
		return nil, fmt.Errorf("tickets: CreateCategory: name is required")
	}
	c := &Category{
		OrgID: orgID, Name: name, Description: req.Description,
		RestrictedRole: normaliseRole(req.RestrictedRole), CreatedBy: callerUserID,
	}
	if req.IsSensitive != nil {
		c.IsSensitive = *req.IsSensitive
	}
	if err := s.validateSensitivity(c); err != nil {
		return nil, err
	}
	if err := s.repo.CreateCategory(ctx, c); err != nil {
		return nil, err
	}
	return c, nil
}

func (s *serviceImpl) UpdateCategory(ctx context.Context, orgID, callerUserID, ref string, req CreateCategoryRequest, isActive *bool) (*Category, error) {
	if err := s.require(ctx, orgID, callerUserID, "platform.ticket_config", "manage"); err != nil {
		return nil, err
	}
	c, err := s.repo.FindCategoryByRef(ctx, orgID, ref)
	if err != nil {
		return nil, fmt.Errorf("tickets: UpdateCategory: %w", err)
	}
	if c == nil {
		return nil, ErrCategoryNotFound
	}
	if n := strings.TrimSpace(req.Name); n != "" {
		c.Name = n
	}
	if req.Description != nil {
		c.Description = req.Description
	}
	if req.IsSensitive != nil {
		c.IsSensitive = *req.IsSensitive
	}
	if req.RestrictedRole != nil {
		c.RestrictedRole = normaliseRole(req.RestrictedRole)
	}
	if isActive != nil {
		c.IsActive = *isActive
	}
	if err := s.validateSensitivity(c); err != nil {
		return nil, err
	}
	if err := s.repo.UpdateCategory(ctx, c); err != nil {
		return nil, err
	}
	return c, nil
}

func normaliseRole(r *string) *string {
	if r == nil {
		return nil
	}
	v := strings.TrimSpace(*r)
	if v == "" {
		return nil
	}
	return &v
}

// validateSensitivity enforces the is_sensitive ↔ restricted_role pairing in
// Go rather than as a CHECK. A CHECK would be the 00076 trap the moment
// either column gained an ON DELETE SET NULL FK, and a sensitive category
// with no restricted role restricts nothing while looking like it does.
func (s *serviceImpl) validateSensitivity(c *Category) error {
	if c.IsSensitive && c.RestrictedRole == nil {
		return ErrRestrictedRoleMissing
	}
	return nil
}

// ── SLA policies ─────────────────────────────────────────────────────────────

func (s *serviceImpl) ListPolicies(ctx context.Context, orgID, callerUserID string) ([]*SLAPolicy, error) {
	if err := s.require(ctx, orgID, callerUserID, "platform.ticket_config", "view"); err != nil {
		return nil, err
	}
	return s.repo.FindPolicies(ctx, orgID)
}

func (s *serviceImpl) CreatePolicy(ctx context.Context, orgID, callerUserID string, req CreateSLAPolicyRequest) (*SLAPolicy, error) {
	if err := s.require(ctx, orgID, callerUserID, "platform.ticket_config", "manage"); err != nil {
		return nil, err
	}
	p := Priority(req.Priority)
	if !p.IsValid() {
		return nil, ErrInvalidPriority
	}
	if req.FirstResponseMinutes <= 0 || req.ResolutionMinutes <= 0 || req.ResolutionMinutes < req.FirstResponseMinutes {
		return nil, ErrInvalidSLAMinutes
	}
	var categoryID *string
	if req.CategoryID != nil {
		c, err := s.repo.FindCategoryByRef(ctx, orgID, *req.CategoryID)
		if err != nil {
			return nil, fmt.Errorf("tickets: CreatePolicy: %w", err)
		}
		if c == nil {
			return nil, ErrCategoryNotFound
		}
		categoryID = &c.ID
	}
	pol := &SLAPolicy{
		OrgID: orgID, CategoryID: categoryID, Priority: p,
		FirstResponseMinutes: req.FirstResponseMinutes,
		ResolutionMinutes:    req.ResolutionMinutes,
		CreatedBy:            callerUserID,
	}
	if err := s.repo.CreatePolicy(ctx, pol); err != nil {
		return nil, err
	}
	return pol, nil
}

func (s *serviceImpl) UpdatePolicy(ctx context.Context, orgID, callerUserID, ref string, req CreateSLAPolicyRequest) (*SLAPolicy, error) {
	if err := s.require(ctx, orgID, callerUserID, "platform.ticket_config", "manage"); err != nil {
		return nil, err
	}
	pol, err := s.repo.FindPolicyByRef(ctx, orgID, ref)
	if err != nil {
		return nil, fmt.Errorf("tickets: UpdatePolicy: %w", err)
	}
	if pol == nil {
		return nil, ErrPolicyNotFound
	}
	if req.FirstResponseMinutes > 0 {
		pol.FirstResponseMinutes = req.FirstResponseMinutes
	}
	if req.ResolutionMinutes > 0 {
		pol.ResolutionMinutes = req.ResolutionMinutes
	}
	if pol.ResolutionMinutes < pol.FirstResponseMinutes {
		return nil, ErrInvalidSLAMinutes
	}
	if err := s.repo.UpdatePolicy(ctx, pol); err != nil {
		return nil, err
	}
	return pol, nil
}

// ── Tickets ──────────────────────────────────────────────────────────────────

func (s *serviceImpl) Create(ctx context.Context, orgID, callerUserID string, req CreateTicketRequest) (*Ticket, error) {
	if err := s.require(ctx, orgID, callerUserID, "platform.tickets", "create"); err != nil {
		return nil, err
	}
	subject := strings.TrimSpace(req.Subject)
	if subject == "" {
		return nil, fmt.Errorf("tickets: Create: subject is required")
	}
	if strings.TrimSpace(req.RequesterID) == "" {
		return nil, fmt.Errorf("tickets: Create: requester_id is required")
	}
	priority := PriorityNormal
	if req.Priority != nil {
		priority = Priority(*req.Priority)
		if !priority.IsValid() {
			return nil, ErrInvalidPriority
		}
	}
	var categoryID *string
	if req.CategoryID != nil && strings.TrimSpace(*req.CategoryID) != "" {
		c, err := s.repo.FindCategoryByRef(ctx, orgID, *req.CategoryID)
		if err != nil {
			return nil, fmt.Errorf("tickets: Create: %w", err)
		}
		if c == nil {
			return nil, ErrCategoryNotFound
		}
		categoryID = &c.ID
	}
	// The governing policy is resolved and PINNED at creation. A later
	// policy edit tightening the target must not retroactively breach
	// tickets raised under the old one — the 7B calculation_snapshot /
	// 7D employee_cost_snapshot discipline, applied to a target rather than
	// a price.
	var policyID *string
	pol, err := s.repo.ResolvePolicy(ctx, orgID, categoryID, priority)
	if err != nil {
		return nil, fmt.Errorf("tickets: Create: resolve policy: %w", err)
	}
	if pol != nil {
		policyID = &pol.ID
	}

	t := &Ticket{
		OrgID:           orgID,
		RequesterType:   RequesterEmployee,
		RequesterID:     req.RequesterID,
		RequesterUserID: callerUserID,
		CategoryID:      categoryID,
		Subject:         subject,
		Description:     req.Description,
		Priority:        priority,
		SLAPolicyID:     policyID,
	}
	if err := s.repo.CreateTicket(ctx, t); err != nil {
		return nil, err
	}
	return t, nil
}

func (s *serviceImpl) List(ctx context.Context, orgID, callerUserID string, f ListFilter) (*TicketListResponse, error) {
	if err := s.require(ctx, orgID, callerUserID, "platform.tickets", "view"); err != nil {
		return nil, err
	}
	viewAll, err := s.can(ctx, orgID, callerUserID, "platform.tickets", "view_all")
	if err != nil {
		return nil, err
	}
	// The caller never supplies these two. Reading them off the request
	// would let anyone hand themselves CanViewAll.
	f.ViewerUserID = callerUserID
	f.CanViewAll = viewAll
	f.Normalise()

	list, err := s.repo.FindTickets(ctx, orgID, f)
	if err != nil {
		return nil, err
	}
	total, err := s.repo.CountTickets(ctx, orgID, f)
	if err != nil {
		return nil, err
	}
	now := time.Now()
	for _, t := range list {
		if err := s.attachSLA(ctx, orgID, t, now); err != nil {
			return nil, err
		}
	}
	return &TicketListResponse{Tickets: list, Total: total, Limit: f.Limit, Offset: f.Offset}, nil
}

func (s *serviceImpl) Get(ctx context.Context, orgID, callerUserID, ref string) (*Ticket, error) {
	t, err := s.loadTicket(ctx, orgID, callerUserID, ref)
	if err != nil {
		return nil, err
	}
	if err := s.attachSLA(ctx, orgID, t, time.Now()); err != nil {
		return nil, err
	}
	comments, err := s.commentsFor(ctx, orgID, callerUserID, t.ID)
	if err != nil {
		return nil, err
	}
	t.Comments = comments
	return t, nil
}

// attachSLA computes both SLA figures from the pause ledger. Neither is
// stored — see migration 00110's header and the 00076 rule.
func (s *serviceImpl) attachSLA(ctx context.Context, orgID string, t *Ticket, now time.Time) error {
	if t.SLAPolicyID == nil {
		return nil
	}
	pol, err := s.repo.FindPolicyByRef(ctx, orgID, *t.SLAPolicyID)
	if err != nil {
		return fmt.Errorf("tickets: attachSLA: %w", err)
	}
	if pol == nil {
		return nil
	}
	events, err := s.repo.FindSLAEvents(ctx, orgID, t.ID)
	if err != nil {
		return fmt.Errorf("tickets: attachSLA: %w", err)
	}

	// Each clock stops at the event that satisfies it, not at "now" — a
	// ticket answered inside its window must not drift into breach simply
	// because nobody has looked at it since.
	firstResponseUntil := now
	if t.FirstResponseAt != nil {
		firstResponseUntil = *t.FirstResponseAt
	}
	resolutionUntil := now
	if t.ResolvedAt != nil {
		resolutionUntil = *t.ResolvedAt
	}

	fr := EvaluateSLA(t.CreatedAt, firstResponseUntil, pol.FirstResponseMinutes, events)
	res := EvaluateSLA(t.CreatedAt, resolutionUntil, pol.ResolutionMinutes, events)
	t.FirstResponseSLA = &fr
	t.ResolutionSLA = &res
	return nil
}

func (s *serviceImpl) Assign(ctx context.Context, orgID, callerUserID, ref string, req AssignTicketRequest) (*Ticket, error) {
	if err := s.require(ctx, orgID, callerUserID, "platform.tickets", "assign"); err != nil {
		return nil, err
	}
	t, err := s.loadTicket(ctx, orgID, callerUserID, ref)
	if err != nil {
		return nil, err
	}
	if t.Status == StatusClosed || t.Status == StatusConverted || t.Status == StatusCancelled {
		return nil, ErrWrongStatus
	}
	assignee := strings.TrimSpace(req.AssigneeUserID)
	if assignee == "" {
		return nil, fmt.Errorf("tickets: Assign: assignee_user_id is required")
	}
	if err := s.checkSensitiveAssignee(ctx, orgID, t, assignee); err != nil {
		return nil, err
	}
	t.AssigneeUserID = &assignee
	if t.Status == StatusOpen {
		t.Status = StatusAssigned
	}
	if err := s.repo.UpdateTicket(ctx, t); err != nil {
		return nil, err
	}
	return t, nil
}

// checkSensitiveAssignee is the whole point of marking a category sensitive:
// a harassment report does not land in the general helpdesk queue. The role
// is resolved through the directory rather than a local query, so this
// package holds exactly one notion of "what role does this user have" and it
// is authz's.
func (s *serviceImpl) checkSensitiveAssignee(ctx context.Context, orgID string, t *Ticket, assigneeUserID string) error {
	if t.CategoryID == nil {
		return nil
	}
	c, err := s.repo.FindCategoryByRef(ctx, orgID, *t.CategoryID)
	if err != nil {
		return fmt.Errorf("tickets: checkSensitiveAssignee: %w", err)
	}
	if c == nil || !c.IsSensitive || c.RestrictedRole == nil {
		return nil
	}
	role, err := s.directory.UserRoleName(ctx, orgID, assigneeUserID)
	if err != nil {
		return fmt.Errorf("tickets: checkSensitiveAssignee: %w", err)
	}
	if !strings.EqualFold(strings.TrimSpace(role), strings.TrimSpace(*c.RestrictedRole)) {
		return ErrSensitiveCategoryRole
	}
	return nil
}

func (s *serviceImpl) Resolve(ctx context.Context, orgID, callerUserID, ref string) (*Ticket, error) {
	if err := s.require(ctx, orgID, callerUserID, "platform.tickets", "resolve"); err != nil {
		return nil, err
	}
	t, err := s.loadTicket(ctx, orgID, callerUserID, ref)
	if err != nil {
		return nil, err
	}
	if t.Status != StatusOpen && t.Status != StatusAssigned && t.Status != StatusPaused {
		return nil, ErrWrongStatus
	}
	now := time.Now()
	t.Status = StatusResolved
	t.ResolvedAt = &now
	if err := s.repo.UpdateTicket(ctx, t); err != nil {
		return nil, err
	}
	return t, nil
}

func (s *serviceImpl) Close(ctx context.Context, orgID, callerUserID, ref string) (*Ticket, error) {
	t, err := s.loadTicket(ctx, orgID, callerUserID, ref)
	if err != nil {
		return nil, err
	}
	// A requester may close their own ticket without holding .resolve —
	// "never mind, sorted it myself" should not need an agent.
	if t.RequesterUserID != callerUserID {
		if err := s.require(ctx, orgID, callerUserID, "platform.tickets", "resolve"); err != nil {
			return nil, err
		}
	}
	if t.Status == StatusClosed || t.Status == StatusConverted || t.Status == StatusCancelled {
		return nil, ErrWrongStatus
	}
	now := time.Now()
	t.Status = StatusClosed
	t.ClosedAt = &now
	if err := s.repo.UpdateTicket(ctx, t); err != nil {
		return nil, err
	}
	return t, nil
}

func (s *serviceImpl) Cancel(ctx context.Context, orgID, callerUserID, ref string) (*Ticket, error) {
	t, err := s.loadTicket(ctx, orgID, callerUserID, ref)
	if err != nil {
		return nil, err
	}
	if t.RequesterUserID != callerUserID {
		if err := s.require(ctx, orgID, callerUserID, "platform.tickets", "resolve"); err != nil {
			return nil, err
		}
	}
	if t.Status != StatusOpen && t.Status != StatusAssigned && t.Status != StatusPaused {
		return nil, ErrWrongStatus
	}
	t.Status = StatusCancelled
	if err := s.repo.UpdateTicket(ctx, t); err != nil {
		return nil, err
	}
	return t, nil
}

// ── SLA clock ────────────────────────────────────────────────────────────────

func (s *serviceImpl) Pause(ctx context.Context, orgID, callerUserID, ref string, req PauseTicketRequest) (*Ticket, error) {
	if err := s.require(ctx, orgID, callerUserID, "platform.tickets", "pause"); err != nil {
		return nil, err
	}
	t, err := s.loadTicket(ctx, orgID, callerUserID, ref)
	if err != nil {
		return nil, err
	}
	// The ledger is consulted BEFORE the status guard. A paused ticket sits
	// in status 'paused', so guarding first would answer a second pause with
	// WRONG_STATUS and leave ErrAlreadyPaused unreachable — a caller would be
	// told the ticket is in the wrong state rather than that it is already
	// paused, which is the one thing they needed to know.
	events, err := s.repo.FindSLAEvents(ctx, orgID, t.ID)
	if err != nil {
		return nil, err
	}
	if IsPaused(events) {
		return nil, ErrAlreadyPaused
	}
	if t.Status != StatusOpen && t.Status != StatusAssigned {
		return nil, ErrWrongStatus
	}
	if err := s.repo.CreateSLAEvent(ctx, orgID, t.ID, "pause", req.Reason, callerUserID); err != nil {
		return nil, err
	}
	t.Status = StatusPaused
	if err := s.repo.UpdateTicket(ctx, t); err != nil {
		return nil, err
	}
	return t, nil
}

func (s *serviceImpl) Resume(ctx context.Context, orgID, callerUserID, ref string) (*Ticket, error) {
	if err := s.require(ctx, orgID, callerUserID, "platform.tickets", "pause"); err != nil {
		return nil, err
	}
	t, err := s.loadTicket(ctx, orgID, callerUserID, ref)
	if err != nil {
		return nil, err
	}
	events, err := s.repo.FindSLAEvents(ctx, orgID, t.ID)
	if err != nil {
		return nil, err
	}
	if !IsPaused(events) {
		return nil, ErrNotPaused
	}
	if err := s.repo.CreateSLAEvent(ctx, orgID, t.ID, "resume", nil, callerUserID); err != nil {
		return nil, err
	}
	// Back to assigned if it has an owner, open if not — resuming must not
	// invent an assignment that was never made.
	if t.AssigneeUserID != nil {
		t.Status = StatusAssigned
	} else {
		t.Status = StatusOpen
	}
	if err := s.repo.UpdateTicket(ctx, t); err != nil {
		return nil, err
	}
	return t, nil
}

// ── Comments ─────────────────────────────────────────────────────────────────

func (s *serviceImpl) AddComment(ctx context.Context, orgID, callerUserID, ref string, req CreateCommentRequest) (*Comment, error) {
	if err := s.require(ctx, orgID, callerUserID, "platform.tickets", "comment"); err != nil {
		return nil, err
	}
	t, err := s.loadTicket(ctx, orgID, callerUserID, ref)
	if err != nil {
		return nil, err
	}
	if t.Status == StatusClosed || t.Status == StatusConverted || t.Status == StatusCancelled {
		return nil, ErrWrongStatus
	}
	body := strings.TrimSpace(req.Body)
	if body == "" {
		return nil, fmt.Errorf("tickets: AddComment: body is required")
	}
	internal := req.IsInternal != nil && *req.IsInternal
	if internal {
		// Writing an internal note needs its own permission, separate from
		// commenting at all — a requester holding .comment must not be able
		// to author a note they would then be unable to read back.
		if err := s.require(ctx, orgID, callerUserID, "platform.tickets", "comment_internal"); err != nil {
			return nil, err
		}
	}

	c := &Comment{TicketID: t.ID, AuthorUserID: callerUserID, Body: body, IsInternal: internal}
	if err := s.repo.CreateComment(ctx, orgID, c); err != nil {
		return nil, err
	}

	// A public reply from somebody other than the requester is the first
	// response. Internal notes do not count — the SLA measures what the
	// requester actually received.
	if !internal && t.FirstResponseAt == nil && t.RequesterUserID != callerUserID {
		now := time.Now()
		t.FirstResponseAt = &now
		if err := s.repo.UpdateTicket(ctx, t); err != nil {
			return nil, err
		}
	}
	return c, nil
}

// ListComments chooses the read path by role. An agent gets FindAllComments,
// everyone else FindPublicComments — the requester's path never selects an
// internal row, so there is nothing in memory to forget to strip.
func (s *serviceImpl) ListComments(ctx context.Context, orgID, callerUserID, ref string) ([]*Comment, error) {
	t, err := s.loadTicket(ctx, orgID, callerUserID, ref)
	if err != nil {
		return nil, err
	}
	return s.commentsFor(ctx, orgID, callerUserID, t.ID)
}

// commentsFor is the read-path selector, taking an already-authorised ticket
// id so Get does not re-fetch and re-authorise the ticket it is holding.
func (s *serviceImpl) commentsFor(ctx context.Context, orgID, callerUserID, ticketID string) ([]*Comment, error) {
	agent, err := s.isAgent(ctx, orgID, callerUserID)
	if err != nil {
		return nil, err
	}
	if agent {
		return s.repo.FindAllComments(ctx, orgID, ticketID)
	}
	return s.repo.FindPublicComments(ctx, orgID, ticketID)
}

// ── Conversion ───────────────────────────────────────────────────────────────

func (s *serviceImpl) MarkConverted(ctx context.Context, orgID, callerUserID, ref, targetType, targetID string) error {
	if targetType != "complaint" {
		return fmt.Errorf("tickets: MarkConverted: unsupported target type %q", targetType)
	}
	if strings.TrimSpace(targetID) == "" {
		return fmt.Errorf("tickets: MarkConverted: target id is required")
	}
	t, err := s.repo.FindTicketByRef(ctx, orgID, ref)
	if err != nil {
		return fmt.Errorf("tickets: MarkConverted: %w", err)
	}
	if t == nil {
		return ErrTicketNotFound
	}
	// One-way, and idempotency is NOT the kind thing to do here: silently
	// accepting a second conversion would leave two complaints believing
	// they own the same ticket, with only the later one recorded.
	if t.ConvertedToID != nil {
		return ErrAlreadyConverted
	}
	if t.Status == StatusCancelled {
		return ErrWrongStatus
	}
	now := time.Now()
	t.ConvertedToType = &targetType
	t.ConvertedToID = &targetID
	t.ConvertedAt = &now
	t.Status = StatusConverted
	return s.repo.UpdateTicket(ctx, t)
}
