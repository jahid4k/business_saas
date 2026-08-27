// backend/internal/hrm/recruitment/service.go
package recruitment

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/mridha/businesssaas/internal/hrm/approvals"
	"github.com/mridha/businesssaas/internal/hrm/exits"
)

// Service defines business logic for the recruitment module. Composed of
// the sub-feature interfaces defined in pipelines_service.go,
// candidates_service.go, and applications_service.go.
type Service interface {
	PipelineService
	CandidateService
	ApplicationService
	InterviewService
	ScorecardService
	OfferService
	ReferralService
	HireService

	// Requisitions
	ListRequisitions(ctx context.Context, orgID string, filter RequisitionListFilter) (*RequisitionListResponse, error)
	GetRequisition(ctx context.Context, orgID, ref string) (*Requisition, error)
	CreateRequisition(ctx context.Context, orgID, createdBy string, req CreateRequisitionRequest) (*Requisition, error)
	UpdateRequisition(ctx context.Context, orgID, ref string, req UpdateRequisitionRequest) (*Requisition, error)
	SubmitRequisition(ctx context.Context, orgID, ref, submittedBy string) (*Requisition, error)
	CloseRequisition(ctx context.Context, orgID, ref string, req CloseRequisitionRequest) (*Requisition, error)
	// HandleApprovalDecision implements approvals.EntityCallback — registered
	// via approvalsSvc.RegisterCallback("job_requisition", ...) in main.go.
	HandleApprovalDecision(ctx context.Context, orgID, entityID string, approved bool) error

	// Postings
	ListPostings(ctx context.Context, orgID string, filter PostingListFilter) (*PostingListResponse, error)
	GetPosting(ctx context.Context, orgID, ref string) (*Posting, error)
	CreatePosting(ctx context.Context, orgID, createdBy string, req CreatePostingRequest) (*Posting, error)
	UpdatePosting(ctx context.Context, orgID, ref string, req UpdatePostingRequest) (*Posting, error)
	DeletePosting(ctx context.Context, orgID, ref string) error
	PublishPosting(ctx context.Context, orgID, ref string) (*Posting, error)
	ClosePosting(ctx context.Context, orgID, ref string) (*Posting, error)
}

type serviceImpl struct {
	repo            Repository
	approvalsSvc    approvals.Service
	employeeCreator EmployeeCreator
	rehireChecker   RehireChecker
}

// NewService takes rehireChecker as a NIL-SAFE dependency: an install without
// exit management still recruits, and a nil checker simply produces no
// warning rather than failing candidate creation.
func NewService(repo Repository, approvalsSvc approvals.Service, employeeCreator EmployeeCreator, rehireChecker RehireChecker) Service {
	return &serviceImpl{
		repo: repo, approvalsSvc: approvalsSvc,
		employeeCreator: employeeCreator, rehireChecker: rehireChecker,
	}
}

// RehireChecker is the minimal slice of exit management this package needs.
//
// Declared HERE, by the consumer, and naming exits' own result type so
// exits.Service satisfies it structurally with no adapter — the corrected
// certifications.SkillGranter precedent, also used for
// assets.HandoverAcknowledger, expenses.ReimbursementCreator and
// email.TicketRaiser. hrm/recruitment imports hrm/exits; never the reverse.
type RehireChecker interface {
	CheckRehireEligibility(ctx context.Context, orgID, email string) (*exits.RehireEligibility, error)
}

// attachRehireFlag looks up whether this candidate is a former employee the
// org decided not to take back. Best-effort by design: a lookup failure must
// not stop a recruiter creating a candidate, so it is logged and skipped.
func (s *serviceImpl) attachRehireFlag(ctx context.Context, orgID string, c *Candidate) {
	if s.rehireChecker == nil || c == nil || c.Email == nil {
		return
	}
	re, err := s.rehireChecker.CheckRehireEligibility(ctx, orgID, *c.Email)
	if err != nil {
		slog.Warn("recruitment: rehire eligibility lookup failed",
			slog.String("org_id", orgID), slog.Any("error", err))
		return
	}
	// Only a negative decision is worth surfacing. Flagging every former
	// employee as "eligible" would be noise a recruiter learns to ignore,
	// and the one that matters would go unread with it.
	if re == nil || re.Status == exits.RehireEligible {
		return
	}
	c.RehireFlag = &RehireFlag{Status: string(re.Status), Reason: re.Reason}
}

// ============================================================
// Requisitions
// ============================================================

func (s *serviceImpl) ListRequisitions(ctx context.Context, orgID string, filter RequisitionListFilter) (*RequisitionListResponse, error) {
	filter.Normalise()
	list, err := s.repo.FindRequisitions(ctx, orgID, filter)
	if err != nil {
		return nil, fmt.Errorf("recruitment: ListRequisitions: %w", err)
	}
	if list == nil {
		list = []*Requisition{}
	}
	total, err := s.repo.CountRequisitions(ctx, orgID, filter)
	if err != nil {
		return nil, fmt.Errorf("recruitment: ListRequisitions: count: %w", err)
	}
	return &RequisitionListResponse{Requisitions: list, Total: total, Limit: filter.Limit, Offset: filter.Offset}, nil
}

func (s *serviceImpl) GetRequisition(ctx context.Context, orgID, ref string) (*Requisition, error) {
	r, err := s.repo.FindRequisitionByRef(ctx, orgID, ref)
	if err != nil {
		return nil, fmt.Errorf("recruitment: GetRequisition: %w", err)
	}
	if r == nil {
		return nil, ErrRequisitionNotFound
	}
	return r, nil
}

func (s *serviceImpl) CreateRequisition(ctx context.Context, orgID, createdBy string, req CreateRequisitionRequest) (*Requisition, error) {
	title := strings.TrimSpace(req.Title)
	if title == "" {
		return nil, ErrTitleRequired
	}

	empType := EmploymentTypeFullTime
	if req.EmploymentType != nil && strings.TrimSpace(*req.EmploymentType) != "" {
		empType = EmploymentType(strings.TrimSpace(*req.EmploymentType))
		if !empType.IsValid() {
			return nil, ErrInvalidEmploymentType
		}
	}

	if req.SalaryMin != nil && req.SalaryMax != nil && req.SalaryMax.LessThan(*req.SalaryMin) {
		return nil, ErrInvalidSalaryRange
	}

	openings := 1
	if req.Openings != nil && *req.Openings > 0 {
		openings = *req.Openings
	}
	currency := "USD"
	if req.SalaryCurrency != nil && strings.TrimSpace(*req.SalaryCurrency) != "" {
		currency = strings.ToUpper(strings.TrimSpace(*req.SalaryCurrency))
	}

	var targetStart *time.Time
	if req.TargetStartDate != nil && strings.TrimSpace(*req.TargetStartDate) != "" {
		d, err := time.Parse(dateLayout, strings.TrimSpace(*req.TargetStartDate))
		if err != nil {
			return nil, fmt.Errorf("recruitment: CreateRequisition: %w", err)
		}
		targetStart = &d
	}

	r := &Requisition{
		OrgID: orgID, Title: title, DepartmentID: req.DepartmentID, PositionID: req.PositionID,
		HiringManagerID: req.HiringManagerID, EmploymentType: empType, Openings: openings,
		Location: req.Location, SalaryMin: req.SalaryMin, SalaryMax: req.SalaryMax, SalaryCurrency: currency,
		Justification: req.Justification, TargetStartDate: targetStart,
		Status: RequisitionStatusDraft, CreatedBy: createdBy,
	}
	if err := s.repo.CreateRequisition(ctx, r); err != nil {
		return nil, fmt.Errorf("recruitment: CreateRequisition: %w", err)
	}
	return r, nil
}

func (s *serviceImpl) UpdateRequisition(ctx context.Context, orgID, ref string, req UpdateRequisitionRequest) (*Requisition, error) {
	r, err := s.repo.FindRequisitionByRef(ctx, orgID, ref)
	if err != nil {
		return nil, fmt.Errorf("recruitment: UpdateRequisition: %w", err)
	}
	if r == nil {
		return nil, ErrRequisitionNotFound
	}
	if r.Status != RequisitionStatusDraft {
		return nil, ErrWrongStatus
	}

	if req.Title != nil {
		title := strings.TrimSpace(*req.Title)
		if title == "" {
			return nil, ErrTitleRequired
		}
		r.Title = title
	}
	if req.DepartmentID != nil {
		r.DepartmentID = req.DepartmentID
	}
	if req.PositionID != nil {
		r.PositionID = req.PositionID
	}
	if req.HiringManagerID != nil {
		r.HiringManagerID = req.HiringManagerID
	}
	if req.EmploymentType != nil {
		et := EmploymentType(strings.TrimSpace(*req.EmploymentType))
		if !et.IsValid() {
			return nil, ErrInvalidEmploymentType
		}
		r.EmploymentType = et
	}
	if req.Openings != nil && *req.Openings > 0 {
		r.Openings = *req.Openings
	}
	if req.Location != nil {
		r.Location = req.Location
	}
	if req.SalaryMin != nil {
		r.SalaryMin = req.SalaryMin
	}
	if req.SalaryMax != nil {
		r.SalaryMax = req.SalaryMax
	}
	if r.SalaryMin != nil && r.SalaryMax != nil && r.SalaryMax.LessThan(*r.SalaryMin) {
		return nil, ErrInvalidSalaryRange
	}
	if req.SalaryCurrency != nil && strings.TrimSpace(*req.SalaryCurrency) != "" {
		r.SalaryCurrency = strings.ToUpper(strings.TrimSpace(*req.SalaryCurrency))
	}
	if req.Justification != nil {
		r.Justification = req.Justification
	}
	if req.TargetStartDate != nil {
		if strings.TrimSpace(*req.TargetStartDate) == "" {
			r.TargetStartDate = nil
		} else {
			d, err := time.Parse(dateLayout, strings.TrimSpace(*req.TargetStartDate))
			if err != nil {
				return nil, fmt.Errorf("recruitment: UpdateRequisition: %w", err)
			}
			r.TargetStartDate = &d
		}
	}

	if err := s.repo.UpdateRequisition(ctx, r); err != nil {
		return nil, fmt.Errorf("recruitment: UpdateRequisition: %w", err)
	}
	return r, nil
}

// SubmitRequisition mirrors promotions.Submit's approval contract exactly:
// FindDefault → if a template exists, CreateInstance + persist
// approval_instance_id + status=pending_approval; if none exists, auto-
// approve. That auto-approve fallback is the established behaviour across
// promotions/transfers/terminations/warnings/awards — deviating here would
// be the inconsistency, however surprising the fallback looks in isolation.
func (s *serviceImpl) SubmitRequisition(ctx context.Context, orgID, ref, submittedBy string) (*Requisition, error) {
	r, err := s.repo.FindRequisitionByRef(ctx, orgID, ref)
	if err != nil {
		return nil, fmt.Errorf("recruitment: SubmitRequisition: %w", err)
	}
	if r == nil {
		return nil, ErrRequisitionNotFound
	}
	if r.Status != RequisitionStatusDraft {
		return nil, ErrWrongStatus
	}

	tmpl, tErr := s.approvalsSvc.FindDefault(ctx, orgID, approvals.ActionTypeJobRequisition)
	if tErr == nil && tmpl != nil {
		inst, iErr := s.approvalsSvc.CreateInstance(ctx, orgID, approvals.CreateInstanceRequest{
			TemplateID: tmpl.ID, EntityType: "job_requisition", EntityID: r.ID, RequestedBy: submittedBy,
		})
		if iErr != nil {
			return nil, fmt.Errorf("recruitment: SubmitRequisition: creating approval instance: %w", iErr)
		}
		if err := s.repo.SetRequisitionApprovalInstance(ctx, r.ID, inst.ID, RequisitionStatusPendingApproval); err != nil {
			return nil, fmt.Errorf("recruitment: SubmitRequisition: %w", err)
		}
		r.ApprovalInstanceID = &inst.ID
		r.Status = RequisitionStatusPendingApproval
		return r, nil
	}

	// No approval template configured — auto-approve, matching every other
	// approval-gated HRM workflow's fallback.
	if err := s.repo.UpdateRequisitionStatus(ctx, r.ID, RequisitionStatusApproved); err != nil {
		return nil, fmt.Errorf("recruitment: SubmitRequisition: %w", err)
	}
	r.Status = RequisitionStatusApproved
	return r, nil
}

// HandleApprovalDecision reacts to the requisition's approval instance
// completing. Idempotency guard mirrors promotions.HandleApprovalDecision.
func (s *serviceImpl) HandleApprovalDecision(ctx context.Context, orgID, entityID string, approved bool) error {
	r, err := s.repo.FindRequisitionByRef(ctx, orgID, entityID)
	if err != nil {
		return fmt.Errorf("recruitment: HandleApprovalDecision: %w", err)
	}
	if r == nil {
		return ErrRequisitionNotFound
	}
	if r.Status != RequisitionStatusPendingApproval {
		return nil
	}
	status := RequisitionStatusApproved
	if !approved {
		status = RequisitionStatusRejected
	}
	if err := s.repo.UpdateRequisitionStatus(ctx, r.ID, status); err != nil {
		return fmt.Errorf("recruitment: HandleApprovalDecision: %w", err)
	}
	return nil
}

func (s *serviceImpl) CloseRequisition(ctx context.Context, orgID, ref string, req CloseRequisitionRequest) (*Requisition, error) {
	r, err := s.repo.FindRequisitionByRef(ctx, orgID, ref)
	if err != nil {
		return nil, fmt.Errorf("recruitment: CloseRequisition: %w", err)
	}
	if r == nil {
		return nil, ErrRequisitionNotFound
	}
	if r.Status == RequisitionStatusClosed || r.Status == RequisitionStatusCancelled {
		return nil, ErrWrongStatus
	}
	reason := strings.TrimSpace(req.Reason)
	if err := s.repo.CloseRequisition(ctx, r.ID, reason); err != nil {
		return nil, fmt.Errorf("recruitment: CloseRequisition: %w", err)
	}
	r.Status = RequisitionStatusClosed
	return r, nil
}

// ============================================================
// Postings
// ============================================================

func (s *serviceImpl) ListPostings(ctx context.Context, orgID string, filter PostingListFilter) (*PostingListResponse, error) {
	filter.Normalise()
	list, err := s.repo.FindPostings(ctx, orgID, filter)
	if err != nil {
		return nil, fmt.Errorf("recruitment: ListPostings: %w", err)
	}
	if list == nil {
		list = []*Posting{}
	}
	total, err := s.repo.CountPostings(ctx, orgID, filter)
	if err != nil {
		return nil, fmt.Errorf("recruitment: ListPostings: count: %w", err)
	}
	return &PostingListResponse{Postings: list, Total: total, Limit: filter.Limit, Offset: filter.Offset}, nil
}

func (s *serviceImpl) GetPosting(ctx context.Context, orgID, ref string) (*Posting, error) {
	p, err := s.repo.FindPostingByRef(ctx, orgID, ref)
	if err != nil {
		return nil, fmt.Errorf("recruitment: GetPosting: %w", err)
	}
	if p == nil {
		return nil, ErrPostingNotFound
	}
	return p, nil
}

func slugify(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	var b strings.Builder
	lastHyphen := false
	for _, r := range s {
		switch {
		case r >= 'a' && r <= 'z' || r >= '0' && r <= '9':
			b.WriteRune(r)
			lastHyphen = false
		default:
			if !lastHyphen && b.Len() > 0 {
				b.WriteRune('-')
				lastHyphen = true
			}
		}
	}
	return strings.Trim(b.String(), "-")
}

func (s *serviceImpl) CreatePosting(ctx context.Context, orgID, createdBy string, req CreatePostingRequest) (*Posting, error) {
	requisitionRef := strings.TrimSpace(req.RequisitionID)
	if requisitionRef == "" {
		return nil, ErrRequisitionRequired
	}
	requisition, err := s.repo.FindRequisitionByRef(ctx, orgID, requisitionRef)
	if err != nil {
		return nil, fmt.Errorf("recruitment: CreatePosting: requisition: %w", err)
	}
	if requisition == nil {
		return nil, ErrRequisitionNotFound
	}

	title := strings.TrimSpace(req.Title)
	if title == "" {
		return nil, ErrTitleRequired
	}

	var pipelineID string
	if req.PipelineID != nil && strings.TrimSpace(*req.PipelineID) != "" {
		p, err := s.repo.FindPipelineByRef(ctx, orgID, strings.TrimSpace(*req.PipelineID))
		if err != nil {
			return nil, fmt.Errorf("recruitment: CreatePosting: pipeline: %w", err)
		}
		if p == nil {
			return nil, ErrPipelineNotFound
		}
		pipelineID = p.ID
	} else {
		p, err := s.repo.FindDefaultPipeline(ctx, orgID)
		if err != nil {
			return nil, fmt.Errorf("recruitment: CreatePosting: default pipeline: %w", err)
		}
		if p == nil {
			return nil, ErrPipelineRequired
		}
		pipelineID = p.ID
	}

	slug := ""
	if req.PublicSlug != nil {
		slug = slugify(*req.PublicSlug)
	}
	if slug == "" {
		slug = slugify(title)
	}
	if slug == "" {
		return nil, ErrSlugRequired
	}
	taken, err := s.repo.SlugExists(ctx, orgID, slug, "")
	if err != nil {
		return nil, fmt.Errorf("recruitment: CreatePosting: slug check: %w", err)
	}
	if taken {
		return nil, ErrSlugTaken
	}

	empType := EmploymentTypeFullTime
	if req.EmploymentType != nil && strings.TrimSpace(*req.EmploymentType) != "" {
		empType = EmploymentType(strings.TrimSpace(*req.EmploymentType))
		if !empType.IsValid() {
			return nil, ErrInvalidEmploymentType
		}
	}

	desc := ""
	if req.DescriptionMarkdown != nil {
		desc = *req.DescriptionMarkdown
	}
	isRemote := false
	if req.IsRemote != nil {
		isRemote = *req.IsRemote
	}

	p := &Posting{
		OrgID: orgID, RequisitionID: requisition.ID, PipelineID: pipelineID, Title: title,
		DescriptionMarkdown: desc, PublicSlug: slug, Location: req.Location, IsRemote: isRemote,
		EmploymentType: empType, Status: PostingStatusDraft, CreatedBy: createdBy,
	}
	if err := s.repo.CreatePosting(ctx, p); err != nil {
		return nil, fmt.Errorf("recruitment: CreatePosting: %w", err)
	}
	return p, nil
}

func (s *serviceImpl) UpdatePosting(ctx context.Context, orgID, ref string, req UpdatePostingRequest) (*Posting, error) {
	p, err := s.repo.FindPostingByRef(ctx, orgID, ref)
	if err != nil {
		return nil, fmt.Errorf("recruitment: UpdatePosting: %w", err)
	}
	if p == nil {
		return nil, ErrPostingNotFound
	}
	if req.Title != nil {
		title := strings.TrimSpace(*req.Title)
		if title == "" {
			return nil, ErrTitleRequired
		}
		p.Title = title
	}
	if req.DescriptionMarkdown != nil {
		p.DescriptionMarkdown = *req.DescriptionMarkdown
	}
	if req.PublicSlug != nil {
		slug := slugify(*req.PublicSlug)
		if slug == "" {
			return nil, ErrSlugRequired
		}
		taken, err := s.repo.SlugExists(ctx, orgID, slug, p.ID)
		if err != nil {
			return nil, fmt.Errorf("recruitment: UpdatePosting: slug check: %w", err)
		}
		if taken {
			return nil, ErrSlugTaken
		}
		p.PublicSlug = slug
	}
	if req.Location != nil {
		p.Location = req.Location
	}
	if req.IsRemote != nil {
		p.IsRemote = *req.IsRemote
	}
	if req.EmploymentType != nil {
		et := EmploymentType(strings.TrimSpace(*req.EmploymentType))
		if !et.IsValid() {
			return nil, ErrInvalidEmploymentType
		}
		p.EmploymentType = et
	}
	if err := s.repo.UpdatePosting(ctx, p); err != nil {
		return nil, fmt.Errorf("recruitment: UpdatePosting: %w", err)
	}
	return p, nil
}

func (s *serviceImpl) DeletePosting(ctx context.Context, orgID, ref string) error {
	p, err := s.repo.FindPostingByRef(ctx, orgID, ref)
	if err != nil {
		return fmt.Errorf("recruitment: DeletePosting: %w", err)
	}
	if p == nil {
		return ErrPostingNotFound
	}
	if err := s.repo.DeletePosting(ctx, orgID, p.ID); err != nil {
		return fmt.Errorf("recruitment: DeletePosting: %w", err)
	}
	return nil
}

func (s *serviceImpl) PublishPosting(ctx context.Context, orgID, ref string) (*Posting, error) {
	p, err := s.repo.FindPostingByRef(ctx, orgID, ref)
	if err != nil {
		return nil, fmt.Errorf("recruitment: PublishPosting: %w", err)
	}
	if p == nil {
		return nil, ErrPostingNotFound
	}
	if p.Status != PostingStatusDraft {
		return nil, ErrWrongStatus
	}
	now := time.Now().Format(time.RFC3339)
	if err := s.repo.SetPostingStatus(ctx, p.ID, PostingStatusPublished, &now, nil); err != nil {
		return nil, fmt.Errorf("recruitment: PublishPosting: %w", err)
	}
	p.Status = PostingStatusPublished
	return p, nil
}

func (s *serviceImpl) ClosePosting(ctx context.Context, orgID, ref string) (*Posting, error) {
	p, err := s.repo.FindPostingByRef(ctx, orgID, ref)
	if err != nil {
		return nil, fmt.Errorf("recruitment: ClosePosting: %w", err)
	}
	if p == nil {
		return nil, ErrPostingNotFound
	}
	if p.Status == PostingStatusClosed {
		return nil, ErrWrongStatus
	}
	now := time.Now().Format(time.RFC3339)
	if err := s.repo.SetPostingStatus(ctx, p.ID, PostingStatusClosed, nil, &now); err != nil {
		return nil, fmt.Errorf("recruitment: ClosePosting: %w", err)
	}
	p.Status = PostingStatusClosed
	return p, nil
}
