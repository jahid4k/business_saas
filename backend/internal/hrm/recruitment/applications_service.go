// backend/internal/hrm/recruitment/applications_service.go
package recruitment

import (
	"context"
	"fmt"
	"strings"
)

// ApplicationService is embedded into Service — see service.go.
type ApplicationService interface {
	ListApplications(ctx context.Context, orgID string, filter ApplicationListFilter) (*ApplicationListResponse, error)
	GetApplication(ctx context.Context, orgID, ref string) (*Application, error)
	GetStageHistory(ctx context.Context, orgID, applicationRef string) ([]*ApplicationStageHistory, error)
	CreateApplication(ctx context.Context, orgID string, createdBy *string, req CreateApplicationRequest) (*Application, error)
	MoveApplication(ctx context.Context, orgID, applicationRef, actorID string, req MoveApplicationRequest) (*Application, error)
	RejectApplication(ctx context.Context, orgID, applicationRef, actorID string, req RejectApplicationRequest) (*Application, error)
	WithdrawApplication(ctx context.Context, orgID, applicationRef, actorID string) (*Application, error)
}

func (s *serviceImpl) ListApplications(ctx context.Context, orgID string, filter ApplicationListFilter) (*ApplicationListResponse, error) {
	filter.Normalise()
	list, err := s.repo.FindApplications(ctx, orgID, filter)
	if err != nil {
		return nil, fmt.Errorf("recruitment: ListApplications: %w", err)
	}
	if list == nil {
		list = []*Application{}
	}
	total, err := s.repo.CountApplications(ctx, orgID, filter)
	if err != nil {
		return nil, fmt.Errorf("recruitment: ListApplications: count: %w", err)
	}
	return &ApplicationListResponse{Applications: list, Total: total, Limit: filter.Limit, Offset: filter.Offset}, nil
}

func (s *serviceImpl) GetApplication(ctx context.Context, orgID, ref string) (*Application, error) {
	a, err := s.repo.FindApplicationByRef(ctx, orgID, ref)
	if err != nil {
		return nil, fmt.Errorf("recruitment: GetApplication: %w", err)
	}
	if a == nil {
		return nil, ErrApplicationNotFound
	}
	return a, nil
}

func (s *serviceImpl) GetStageHistory(ctx context.Context, orgID, applicationRef string) ([]*ApplicationStageHistory, error) {
	a, err := s.repo.FindApplicationByRef(ctx, orgID, applicationRef)
	if err != nil {
		return nil, fmt.Errorf("recruitment: GetStageHistory: %w", err)
	}
	if a == nil {
		return nil, ErrApplicationNotFound
	}
	list, err := s.repo.FindStageHistory(ctx, orgID, a.ID)
	if err != nil {
		return nil, fmt.Errorf("recruitment: GetStageHistory: %w", err)
	}
	if list == nil {
		list = []*ApplicationStageHistory{}
	}
	return list, nil
}

// CreateApplication resolves the posting's pipeline and places the new
// application on that pipeline's first stage (lowest position). Returns
// ErrDuplicateApplication rather than relying solely on the DB's
// uq_hrm_appl_candidate_posting index, so the caller gets a clean 409
// instead of a raw constraint-violation error.
func (s *serviceImpl) CreateApplication(ctx context.Context, orgID string, createdBy *string, req CreateApplicationRequest) (*Application, error) {
	candidateID := strings.TrimSpace(req.CandidateID)
	if candidateID == "" {
		return nil, ErrCandidateIDRequired
	}
	postingID := strings.TrimSpace(req.PostingID)
	if postingID == "" {
		return nil, ErrPostingIDRequired
	}

	cand, err := s.repo.FindCandidateByRef(ctx, orgID, candidateID)
	if err != nil {
		return nil, fmt.Errorf("recruitment: CreateApplication: candidate: %w", err)
	}
	if cand == nil {
		return nil, ErrCandidateNotFound
	}

	posting, err := s.repo.FindPostingByRef(ctx, orgID, postingID)
	if err != nil {
		return nil, fmt.Errorf("recruitment: CreateApplication: posting: %w", err)
	}
	if posting == nil {
		return nil, ErrPostingNotFound
	}

	existing, err := s.repo.FindActiveApplication(ctx, orgID, cand.ID, posting.ID)
	if err != nil {
		return nil, fmt.Errorf("recruitment: CreateApplication: dedup check: %w", err)
	}
	if existing != nil {
		return nil, ErrDuplicateApplication
	}

	stages, err := s.repo.FindStages(ctx, orgID, posting.PipelineID)
	if err != nil {
		return nil, fmt.Errorf("recruitment: CreateApplication: stages: %w", err)
	}
	if len(stages) == 0 {
		return nil, ErrStageNotFound
	}
	initialStage := stages[0] // FindStages orders by position ASC

	app := &Application{
		OrgID: orgID, CandidateID: cand.ID, PostingID: posting.ID, PipelineID: posting.PipelineID,
		StageID: initialStage.ID, Status: ApplicationStatusActive, CoverLetter: req.CoverLetter,
		Source: req.Source, CreatedBy: createdBy,
	}
	if err := s.repo.CreateApplication(ctx, app, initialStage.Name); err != nil {
		return nil, fmt.Errorf("recruitment: CreateApplication: %w", err)
	}
	return app, nil
}

// MoveApplication validates the target stage belongs to the application's
// own pipeline (crm_deals' redundant pipeline_id+stage_id storage is what
// makes this check possible without a join) and that the application is
// still active, then delegates the atomic write to the repository.
func (s *serviceImpl) MoveApplication(ctx context.Context, orgID, applicationRef, actorID string, req MoveApplicationRequest) (*Application, error) {
	app, err := s.repo.FindApplicationByRef(ctx, orgID, applicationRef)
	if err != nil {
		return nil, fmt.Errorf("recruitment: MoveApplication: %w", err)
	}
	if app == nil {
		return nil, ErrApplicationNotFound
	}
	if app.Status != ApplicationStatusActive {
		return nil, ErrApplicationNotActive
	}

	targetStageID := strings.TrimSpace(req.StageID)
	if targetStageID == "" {
		return nil, ErrStageNotFound
	}
	// Deliberately unscoped by pipeline (unlike FindStageByRef) so a stage
	// belonging to a different pipeline is distinguished from one that does
	// not exist at all — FindStageByRef's pipeline-scoped query would
	// collapse both into "not found" and make ErrStageNotInPipeline dead code.
	stage, err := s.repo.FindStageByRefAnyPipeline(ctx, orgID, targetStageID)
	if err != nil {
		return nil, fmt.Errorf("recruitment: MoveApplication: stage lookup: %w", err)
	}
	if stage == nil {
		return nil, ErrStageNotFound
	}
	if stage.PipelineID != app.PipelineID {
		return nil, ErrStageNotInPipeline
	}

	newStatus := ApplicationStatusActive
	switch stage.StageKind {
	case StageKindHired:
		newStatus = ApplicationStatusHired
	case StageKindRejected:
		newStatus = ApplicationStatusRejected
	}

	var movedBy *string
	if actorID != "" {
		movedBy = &actorID
	}
	updated, _, err := s.repo.MoveApplicationStage(ctx, orgID, app.ID, stage.ID, stage.Name, newStatus, movedBy, req.Note)
	if err != nil {
		return nil, fmt.Errorf("recruitment: MoveApplication: %w", err)
	}
	return updated, nil
}

func (s *serviceImpl) RejectApplication(ctx context.Context, orgID, applicationRef, actorID string, req RejectApplicationRequest) (*Application, error) {
	reason := strings.TrimSpace(req.Reason)
	if reason == "" {
		return nil, ErrRejectReasonRequired
	}
	app, err := s.repo.FindApplicationByRef(ctx, orgID, applicationRef)
	if err != nil {
		return nil, fmt.Errorf("recruitment: RejectApplication: %w", err)
	}
	if app == nil {
		return nil, ErrApplicationNotFound
	}
	if app.Status != ApplicationStatusActive {
		return nil, ErrApplicationNotActive
	}
	updated, err := s.repo.UpdateApplicationStatus(ctx, orgID, app.ID, ApplicationStatusRejected, &reason)
	if err != nil {
		return nil, fmt.Errorf("recruitment: RejectApplication: %w", err)
	}
	return updated, nil
}

func (s *serviceImpl) WithdrawApplication(ctx context.Context, orgID, applicationRef, actorID string) (*Application, error) {
	app, err := s.repo.FindApplicationByRef(ctx, orgID, applicationRef)
	if err != nil {
		return nil, fmt.Errorf("recruitment: WithdrawApplication: %w", err)
	}
	if app == nil {
		return nil, ErrApplicationNotFound
	}
	if app.Status != ApplicationStatusActive {
		return nil, ErrApplicationNotActive
	}
	updated, err := s.repo.UpdateApplicationStatus(ctx, orgID, app.ID, ApplicationStatusWithdrawn, nil)
	if err != nil {
		return nil, fmt.Errorf("recruitment: WithdrawApplication: %w", err)
	}
	return updated, nil
}
