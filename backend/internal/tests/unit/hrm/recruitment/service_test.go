// backend/internal/tests/unit/hrm/recruitment/service_test.go
package recruitment_test

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/mridha/businesssaas/internal/hrm/approvals"
	"github.com/mridha/businesssaas/internal/hrm/recruitment"
)

// ── Stub Repository ──────────────────────────────────────────────────────────

type stubRepo struct {
	seq int

	pipelines    map[string]*recruitment.Pipeline
	stages       map[string]*recruitment.Stage
	requisitions map[string]*recruitment.Requisition
	postings     map[string]*recruitment.Posting
	candidates   map[string]*recruitment.Candidate
	applications map[string]*recruitment.Application
	history      map[string][]*recruitment.ApplicationStageHistory

	interviews    map[string]*recruitment.Interview
	panelists     map[string]*recruitment.Panelist
	scorecards    map[string]*recruitment.Scorecard
	offers        map[string]*recruitment.Offer
	referrals     map[string]*recruitment.Referral
	employeeUsers map[string]string // userID -> employeeID, seeded directly by tests
}

func newStubRepo() *stubRepo {
	return &stubRepo{
		pipelines:    map[string]*recruitment.Pipeline{},
		stages:       map[string]*recruitment.Stage{},
		requisitions: map[string]*recruitment.Requisition{},
		postings:     map[string]*recruitment.Posting{},
		candidates:   map[string]*recruitment.Candidate{},
		applications: map[string]*recruitment.Application{},
		history:      map[string][]*recruitment.ApplicationStageHistory{},

		interviews:    map[string]*recruitment.Interview{},
		panelists:     map[string]*recruitment.Panelist{},
		scorecards:    map[string]*recruitment.Scorecard{},
		offers:        map[string]*recruitment.Offer{},
		referrals:     map[string]*recruitment.Referral{},
		employeeUsers: map[string]string{},
	}
}

func (r *stubRepo) nextID(prefix string) string {
	r.seq++
	return fmt.Sprintf("%s_%d", prefix, r.seq)
}

func matchRef(id, publicID, ref string) bool { return id == ref || publicID == ref }

// ── Pipelines ────────────────────────────────────────────────────────────────

func (r *stubRepo) FindPipelines(_ context.Context, orgID string) ([]*recruitment.Pipeline, error) {
	var out []*recruitment.Pipeline
	for _, p := range r.pipelines {
		if p.OrgID == orgID {
			out = append(out, p)
		}
	}
	return out, nil
}
func (r *stubRepo) FindPipelineByRef(_ context.Context, orgID, ref string) (*recruitment.Pipeline, error) {
	for _, p := range r.pipelines {
		if p.OrgID == orgID && matchRef(p.ID, p.PublicID, ref) {
			return p, nil
		}
	}
	return nil, nil
}
func (r *stubRepo) FindDefaultPipeline(_ context.Context, orgID string) (*recruitment.Pipeline, error) {
	for _, p := range r.pipelines {
		if p.OrgID == orgID && p.IsDefault && p.IsActive {
			return p, nil
		}
	}
	return nil, nil
}
func (r *stubRepo) CreatePipeline(_ context.Context, p *recruitment.Pipeline) error {
	if p.IsDefault {
		for _, existing := range r.pipelines {
			if existing.OrgID == p.OrgID {
				existing.IsDefault = false
			}
		}
	}
	p.ID = r.nextID("pipe")
	p.PublicID = "pub_" + p.ID
	p.CreatedAt, p.UpdatedAt = time.Now(), time.Now()
	r.pipelines[p.ID] = p
	return nil
}
func (r *stubRepo) UpdatePipeline(_ context.Context, p *recruitment.Pipeline) error {
	if _, ok := r.pipelines[p.ID]; !ok {
		return recruitment.ErrPipelineNotFound
	}
	p.UpdatedAt = time.Now()
	r.pipelines[p.ID] = p
	return nil
}
func (r *stubRepo) DeletePipeline(_ context.Context, _, pipelineID string) error {
	if _, ok := r.pipelines[pipelineID]; !ok {
		return recruitment.ErrPipelineNotFound
	}
	delete(r.pipelines, pipelineID)
	return nil
}
func (r *stubRepo) SetPipelineDefault(_ context.Context, orgID, pipelineID string) error {
	target, ok := r.pipelines[pipelineID]
	if !ok {
		return recruitment.ErrPipelineNotFound
	}
	for _, existing := range r.pipelines {
		if existing.OrgID == orgID {
			existing.IsDefault = false
		}
	}
	target.IsDefault = true
	return nil
}
func (r *stubRepo) FindStages(_ context.Context, orgID, pipelineID string) ([]*recruitment.Stage, error) {
	var out []*recruitment.Stage
	for _, s := range r.stages {
		if s.OrgID == orgID && s.PipelineID == pipelineID {
			out = append(out, s)
		}
	}
	// stable order by Position, matching the real repo's ORDER BY position ASC
	for i := 0; i < len(out); i++ {
		for j := i + 1; j < len(out); j++ {
			if out[j].Position < out[i].Position {
				out[i], out[j] = out[j], out[i]
			}
		}
	}
	return out, nil
}
func (r *stubRepo) FindStageByRef(_ context.Context, orgID, pipelineID, ref string) (*recruitment.Stage, error) {
	for _, s := range r.stages {
		if s.OrgID == orgID && s.PipelineID == pipelineID && matchRef(s.ID, s.PublicID, ref) {
			return s, nil
		}
	}
	return nil, nil
}
func (r *stubRepo) FindStageByRefAnyPipeline(_ context.Context, orgID, ref string) (*recruitment.Stage, error) {
	for _, s := range r.stages {
		if s.OrgID == orgID && matchRef(s.ID, s.PublicID, ref) {
			return s, nil
		}
	}
	return nil, nil
}
func (r *stubRepo) CreateStage(_ context.Context, s *recruitment.Stage) error {
	s.ID = r.nextID("stage")
	s.PublicID = "pub_" + s.ID
	s.CreatedAt, s.UpdatedAt = time.Now(), time.Now()
	r.stages[s.ID] = s
	return nil
}
func (r *stubRepo) UpdateStage(_ context.Context, s *recruitment.Stage) error {
	if _, ok := r.stages[s.ID]; !ok {
		return recruitment.ErrStageNotFound
	}
	r.stages[s.ID] = s
	return nil
}
func (r *stubRepo) DeleteStage(_ context.Context, _, _, stageID string) error {
	delete(r.stages, stageID)
	return nil
}
func (r *stubRepo) ReorderStages(_ context.Context, _, _ string, stageIDs []string) error {
	for i, id := range stageIDs {
		if s, ok := r.stages[id]; ok {
			s.Position = i
		}
	}
	return nil
}

// ── Candidates ───────────────────────────────────────────────────────────────

func (r *stubRepo) FindCandidates(_ context.Context, orgID string, _ recruitment.CandidateListFilter) ([]*recruitment.Candidate, error) {
	var out []*recruitment.Candidate
	for _, c := range r.candidates {
		if c.OrgID == orgID {
			out = append(out, c)
		}
	}
	return out, nil
}
func (r *stubRepo) CountCandidates(ctx context.Context, orgID string, f recruitment.CandidateListFilter) (int, error) {
	out, _ := r.FindCandidates(ctx, orgID, f)
	return len(out), nil
}
func (r *stubRepo) FindCandidateByRef(_ context.Context, orgID, ref string) (*recruitment.Candidate, error) {
	for _, c := range r.candidates {
		if c.OrgID == orgID && matchRef(c.ID, c.PublicID, ref) {
			return c, nil
		}
	}
	return nil, nil
}
func (r *stubRepo) FindCandidateByEmail(_ context.Context, orgID, email string) (*recruitment.Candidate, error) {
	for _, c := range r.candidates {
		if c.OrgID == orgID && c.Email != nil && *c.Email == email {
			return c, nil
		}
	}
	return nil, nil
}
func (r *stubRepo) CreateCandidate(_ context.Context, c *recruitment.Candidate) error {
	c.ID = r.nextID("cand")
	c.PublicID = "pub_" + c.ID
	c.CreatedAt, c.UpdatedAt = time.Now(), time.Now()
	r.candidates[c.ID] = c
	return nil
}
func (r *stubRepo) UpdateCandidate(_ context.Context, c *recruitment.Candidate) error {
	if _, ok := r.candidates[c.ID]; !ok {
		return recruitment.ErrCandidateNotFound
	}
	r.candidates[c.ID] = c
	return nil
}
func (r *stubRepo) SoftDeleteCandidate(_ context.Context, _, candidateID string) error {
	if _, ok := r.candidates[candidateID]; !ok {
		return recruitment.ErrCandidateNotFound
	}
	delete(r.candidates, candidateID)
	return nil
}
func (r *stubRepo) SetCandidateResume(_ context.Context, candidateID string, filePath, fileName, mimeType string, sizeBytes int64, sha256 string) error {
	c, ok := r.candidates[candidateID]
	if !ok {
		return recruitment.ErrCandidateNotFound
	}
	c.ResumeFilePath, c.ResumeFileName, c.ResumeMimeType, c.ResumeSizeBytes, c.ResumeSHA256 = &filePath, &fileName, &mimeType, &sizeBytes, &sha256
	return nil
}
func (r *stubRepo) CountCandidatesByResumeSHA256(_ context.Context, sha256 string) (int, error) {
	n := 0
	for _, c := range r.candidates {
		if c.ResumeSHA256 != nil && *c.ResumeSHA256 == sha256 {
			n++
		}
	}
	return n, nil
}

// ── Applications ─────────────────────────────────────────────────────────────

func (r *stubRepo) FindApplications(_ context.Context, orgID string, _ recruitment.ApplicationListFilter) ([]*recruitment.Application, error) {
	var out []*recruitment.Application
	for _, a := range r.applications {
		if a.OrgID == orgID {
			out = append(out, a)
		}
	}
	return out, nil
}
func (r *stubRepo) CountApplications(ctx context.Context, orgID string, f recruitment.ApplicationListFilter) (int, error) {
	out, _ := r.FindApplications(ctx, orgID, f)
	return len(out), nil
}
func (r *stubRepo) FindApplicationByRef(_ context.Context, orgID, ref string) (*recruitment.Application, error) {
	for _, a := range r.applications {
		if a.OrgID == orgID && matchRef(a.ID, a.PublicID, ref) {
			return a, nil
		}
	}
	return nil, nil
}
func (r *stubRepo) FindActiveApplication(_ context.Context, orgID, candidateID, postingID string) (*recruitment.Application, error) {
	for _, a := range r.applications {
		if a.OrgID == orgID && a.CandidateID == candidateID && a.PostingID == postingID && a.Status != recruitment.ApplicationStatusWithdrawn {
			return a, nil
		}
	}
	return nil, nil
}
func (r *stubRepo) CreateApplication(_ context.Context, app *recruitment.Application, initialStageName string) error {
	app.ID = r.nextID("app")
	app.PublicID = "pub_" + app.ID
	app.AppliedAt, app.CreatedAt, app.UpdatedAt = time.Now(), time.Now(), time.Now()
	r.applications[app.ID] = app
	r.history[app.ID] = append(r.history[app.ID], &recruitment.ApplicationStageHistory{
		ID: r.nextID("hist"), ApplicationID: app.ID, ToStageID: &app.StageID, ToStageName: initialStageName, MovedAt: time.Now(),
	})
	return nil
}
func (r *stubRepo) MoveApplicationStage(_ context.Context, orgID, applicationID, toStageID, toStageName string, newStatus recruitment.ApplicationStatus, movedBy, note *string) (*recruitment.Application, *recruitment.ApplicationStageHistory, error) {
	app, ok := r.applications[applicationID]
	if !ok || app.OrgID != orgID {
		return nil, nil, recruitment.ErrApplicationNotFound
	}
	fromStageID := app.StageID
	app.StageID = toStageID
	app.Status = newStatus
	now := time.Now()
	if newStatus == recruitment.ApplicationStatusHired {
		app.HiredAt = &now
	}
	if newStatus == recruitment.ApplicationStatusRejected {
		app.RejectedAt = &now
	}
	app.UpdatedAt = now
	hist := &recruitment.ApplicationStageHistory{
		ID: r.nextID("hist"), ApplicationID: applicationID, FromStageID: &fromStageID, ToStageID: &toStageID,
		ToStageName: toStageName, MovedBy: movedBy, MovedAt: now, Note: note,
	}
	r.history[applicationID] = append(r.history[applicationID], hist)
	return app, hist, nil
}
func (r *stubRepo) UpdateApplicationStatus(_ context.Context, orgID, applicationID string, status recruitment.ApplicationStatus, reason *string) (*recruitment.Application, error) {
	app, ok := r.applications[applicationID]
	if !ok || app.OrgID != orgID {
		return nil, recruitment.ErrApplicationNotFound
	}
	app.Status = status
	now := time.Now()
	if status == recruitment.ApplicationStatusRejected {
		app.RejectionReason, app.RejectedAt = reason, &now
	}
	if status == recruitment.ApplicationStatusWithdrawn {
		app.WithdrawnAt = &now
	}
	app.UpdatedAt = now
	return app, nil
}
func (r *stubRepo) FindStageHistory(_ context.Context, _, applicationID string) ([]*recruitment.ApplicationStageHistory, error) {
	return r.history[applicationID], nil
}

// ── Requisitions ─────────────────────────────────────────────────────────────

func (r *stubRepo) FindRequisitions(_ context.Context, orgID string, _ recruitment.RequisitionListFilter) ([]*recruitment.Requisition, error) {
	var out []*recruitment.Requisition
	for _, req := range r.requisitions {
		if req.OrgID == orgID {
			out = append(out, req)
		}
	}
	return out, nil
}
func (r *stubRepo) CountRequisitions(ctx context.Context, orgID string, f recruitment.RequisitionListFilter) (int, error) {
	out, _ := r.FindRequisitions(ctx, orgID, f)
	return len(out), nil
}
func (r *stubRepo) FindRequisitionByRef(_ context.Context, orgID, ref string) (*recruitment.Requisition, error) {
	for _, req := range r.requisitions {
		if req.OrgID == orgID && matchRef(req.ID, req.PublicID, ref) {
			return req, nil
		}
	}
	return nil, nil
}
func (r *stubRepo) CreateRequisition(_ context.Context, req *recruitment.Requisition) error {
	req.ID = r.nextID("req")
	req.PublicID = "pub_" + req.ID
	req.CreatedAt, req.UpdatedAt = time.Now(), time.Now()
	r.requisitions[req.ID] = req
	return nil
}
func (r *stubRepo) UpdateRequisition(_ context.Context, req *recruitment.Requisition) error {
	if _, ok := r.requisitions[req.ID]; !ok {
		return recruitment.ErrRequisitionNotFound
	}
	r.requisitions[req.ID] = req
	return nil
}
func (r *stubRepo) SetRequisitionApprovalInstance(_ context.Context, id, instanceID string, status recruitment.RequisitionStatus) error {
	req, ok := r.requisitions[id]
	if !ok {
		return recruitment.ErrRequisitionNotFound
	}
	req.ApprovalInstanceID, req.Status = &instanceID, status
	return nil
}
func (r *stubRepo) UpdateRequisitionStatus(_ context.Context, id string, status recruitment.RequisitionStatus) error {
	req, ok := r.requisitions[id]
	if !ok {
		return recruitment.ErrRequisitionNotFound
	}
	req.Status = status
	return nil
}
func (r *stubRepo) CloseRequisition(_ context.Context, id string, reason string) error {
	req, ok := r.requisitions[id]
	if !ok {
		return recruitment.ErrRequisitionNotFound
	}
	req.Status = recruitment.RequisitionStatusClosed
	req.CloseReason = &reason
	return nil
}

// ── Postings ─────────────────────────────────────────────────────────────────

func (r *stubRepo) FindPostings(_ context.Context, orgID string, _ recruitment.PostingListFilter) ([]*recruitment.Posting, error) {
	var out []*recruitment.Posting
	for _, p := range r.postings {
		if p.OrgID == orgID {
			out = append(out, p)
		}
	}
	return out, nil
}
func (r *stubRepo) CountPostings(ctx context.Context, orgID string, f recruitment.PostingListFilter) (int, error) {
	out, _ := r.FindPostings(ctx, orgID, f)
	return len(out), nil
}
func (r *stubRepo) FindPostingByRef(_ context.Context, orgID, ref string) (*recruitment.Posting, error) {
	for _, p := range r.postings {
		if p.OrgID == orgID && matchRef(p.ID, p.PublicID, ref) {
			return p, nil
		}
	}
	return nil, nil
}
func (r *stubRepo) CreatePosting(_ context.Context, p *recruitment.Posting) error {
	p.ID = r.nextID("post")
	p.PublicID = "pub_" + p.ID
	p.CreatedAt, p.UpdatedAt = time.Now(), time.Now()
	r.postings[p.ID] = p
	return nil
}
func (r *stubRepo) UpdatePosting(_ context.Context, p *recruitment.Posting) error {
	if _, ok := r.postings[p.ID]; !ok {
		return recruitment.ErrPostingNotFound
	}
	r.postings[p.ID] = p
	return nil
}
func (r *stubRepo) DeletePosting(_ context.Context, _, postingID string) error {
	if _, ok := r.postings[postingID]; !ok {
		return recruitment.ErrPostingNotFound
	}
	delete(r.postings, postingID)
	return nil
}
func (r *stubRepo) SetPostingStatus(_ context.Context, id string, status recruitment.PostingStatus, publishedAt, closedAt *string) error {
	p, ok := r.postings[id]
	if !ok {
		return recruitment.ErrPostingNotFound
	}
	p.Status = status
	return nil
}
func (r *stubRepo) SlugExists(_ context.Context, orgID, slug, excludeID string) (bool, error) {
	for _, p := range r.postings {
		if p.OrgID == orgID && p.PublicSlug == slug && p.ID != excludeID {
			return true, nil
		}
	}
	return false, nil
}

// ── Tx plumbing ──────────────────────────────────────────────────────────────
//
// fakeTx is a minimal no-op pgx.Tx — the crm/leads_service_test.go precedent.
// HireApplication is the only caller of BeginTx in this package; the stub
// repo methods it drives (LockApplicationForHireTx,
// SetApplicationConvertedEmployeeTx, IncrementRequisitionFilledCountTx) don't
// actually need a real transaction to exercise service-level logic.
type fakeTx struct{}

func (t *fakeTx) Begin(_ context.Context) (pgx.Tx, error)                    { return t, nil }
func (t *fakeTx) Commit(_ context.Context) error                             { return nil }
func (t *fakeTx) Rollback(_ context.Context) error                           { return nil }
func (t *fakeTx) Conn() *pgx.Conn                                            { return nil }
func (t *fakeTx) LargeObjects() pgx.LargeObjects                             { return pgx.LargeObjects{} }
func (t *fakeTx) SendBatch(_ context.Context, _ *pgx.Batch) pgx.BatchResults { return nil }
func (t *fakeTx) CopyFrom(_ context.Context, _ pgx.Identifier, _ []string, _ pgx.CopyFromSource) (int64, error) {
	return 0, nil
}
func (t *fakeTx) Prepare(_ context.Context, _, _ string) (*pgconn.StatementDescription, error) {
	return nil, nil
}
func (t *fakeTx) Exec(_ context.Context, _ string, _ ...any) (pgconn.CommandTag, error) {
	return pgconn.CommandTag{}, nil
}
func (t *fakeTx) Query(_ context.Context, _ string, _ ...any) (pgx.Rows, error) { return nil, nil }
func (t *fakeTx) QueryRow(_ context.Context, _ string, _ ...any) pgx.Row        { return nil }

func (r *stubRepo) BeginTx(_ context.Context) (pgx.Tx, error) { return &fakeTx{}, nil }

func (r *stubRepo) IncrementRequisitionFilledCountTx(_ context.Context, _ pgx.Tx, requisitionID string) error {
	req, ok := r.requisitions[requisitionID]
	if !ok {
		return recruitment.ErrRequisitionNotFound
	}
	req.FilledCount++
	return nil
}

func (r *stubRepo) LockApplicationForHireTx(_ context.Context, _ pgx.Tx, orgID, applicationID string) (*recruitment.Application, error) {
	app, ok := r.applications[applicationID]
	if !ok || app.OrgID != orgID {
		return nil, nil
	}
	return app, nil
}

func (r *stubRepo) SetApplicationConvertedEmployeeTx(_ context.Context, _ pgx.Tx, applicationID, employeeID string) error {
	app, ok := r.applications[applicationID]
	if !ok {
		return recruitment.ErrApplicationNotFound
	}
	app.ConvertedEmployeeID = &employeeID
	return nil
}

// ── Interviews + panelists ───────────────────────────────────────────────────

func (r *stubRepo) FindInterviews(_ context.Context, orgID, applicationID string) ([]*recruitment.Interview, error) {
	var out []*recruitment.Interview
	for _, i := range r.interviews {
		if i.OrgID == orgID && i.ApplicationID == applicationID {
			out = append(out, i)
		}
	}
	return out, nil
}
func (r *stubRepo) FindInterviewByRef(_ context.Context, orgID, ref string) (*recruitment.Interview, error) {
	for _, i := range r.interviews {
		if i.OrgID == orgID && matchRef(i.ID, i.PublicID, ref) {
			return i, nil
		}
	}
	return nil, nil
}
func (r *stubRepo) CreateInterview(_ context.Context, i *recruitment.Interview) error {
	i.ID = r.nextID("intv")
	i.PublicID = "pub_" + i.ID
	i.Status = recruitment.InterviewStatusScheduled
	i.CreatedAt, i.UpdatedAt = time.Now(), time.Now()
	r.interviews[i.ID] = i
	return nil
}
func (r *stubRepo) UpdateInterview(_ context.Context, i *recruitment.Interview) error {
	if _, ok := r.interviews[i.ID]; !ok {
		return recruitment.ErrInterviewNotFound
	}
	i.UpdatedAt = time.Now()
	r.interviews[i.ID] = i
	return nil
}
func (r *stubRepo) DeleteInterview(_ context.Context, _, interviewID string) error {
	if _, ok := r.interviews[interviewID]; !ok {
		return recruitment.ErrInterviewNotFound
	}
	delete(r.interviews, interviewID)
	return nil
}
func (r *stubRepo) FindPanelists(_ context.Context, interviewID string) ([]*recruitment.Panelist, error) {
	var out []*recruitment.Panelist
	for _, p := range r.panelists {
		if p.InterviewID == interviewID {
			out = append(out, p)
		}
	}
	return out, nil
}
func (r *stubRepo) FindPanelist(_ context.Context, interviewID, employeeID string) (*recruitment.Panelist, error) {
	for _, p := range r.panelists {
		if p.InterviewID == interviewID && p.EmployeeID == employeeID {
			return p, nil
		}
	}
	return nil, nil
}
func (r *stubRepo) AddPanelist(_ context.Context, p *recruitment.Panelist) error {
	p.ID = r.nextID("panl")
	p.PublicID = "pub_" + p.ID
	p.CreatedAt = time.Now()
	r.panelists[p.ID] = p
	return nil
}
func (r *stubRepo) RemovePanelist(_ context.Context, interviewID, employeeID string) error {
	for id, p := range r.panelists {
		if p.InterviewID == interviewID && p.EmployeeID == employeeID {
			delete(r.panelists, id)
			return nil
		}
	}
	return recruitment.ErrPanelistNotFound
}
func (r *stubRepo) FindEmployeeIDByUserID(_ context.Context, _, userID string) (string, error) {
	return r.employeeUsers[userID], nil
}

// ── Scorecards ────────────────────────────────────────────────────────────────

func (r *stubRepo) FindScorecards(_ context.Context, interviewID string) ([]*recruitment.Scorecard, error) {
	var out []*recruitment.Scorecard
	for _, sc := range r.scorecards {
		if sc.InterviewID == interviewID {
			out = append(out, sc)
		}
	}
	return out, nil
}
func (r *stubRepo) FindScorecard(_ context.Context, interviewID, panelistEmployeeID string) (*recruitment.Scorecard, error) {
	for _, sc := range r.scorecards {
		if sc.InterviewID == interviewID && sc.PanelistEmployeeID == panelistEmployeeID {
			return sc, nil
		}
	}
	return nil, nil
}
func (r *stubRepo) UpsertScorecardDraft(_ context.Context, sc *recruitment.Scorecard) error {
	for _, existing := range r.scorecards {
		if existing.InterviewID == sc.InterviewID && existing.PanelistEmployeeID == sc.PanelistEmployeeID {
			if existing.SubmittedAt != nil {
				return recruitment.ErrScorecardAlreadySubmitted
			}
			sc.ID, sc.PublicID, sc.CreatedAt = existing.ID, existing.PublicID, existing.CreatedAt
			sc.UpdatedAt = time.Now()
			r.scorecards[sc.ID] = sc
			return nil
		}
	}
	sc.ID = r.nextID("sc")
	sc.PublicID = "pub_" + sc.ID
	sc.CreatedAt, sc.UpdatedAt = time.Now(), time.Now()
	r.scorecards[sc.ID] = sc
	return nil
}
func (r *stubRepo) SubmitScorecard(_ context.Context, interviewID, panelistEmployeeID string) (*recruitment.Scorecard, error) {
	for _, sc := range r.scorecards {
		if sc.InterviewID == interviewID && sc.PanelistEmployeeID == panelistEmployeeID {
			if sc.SubmittedAt != nil {
				return nil, recruitment.ErrScorecardAlreadySubmitted
			}
			now := time.Now()
			sc.SubmittedAt = &now
			sc.UpdatedAt = now
			return sc, nil
		}
	}
	return nil, recruitment.ErrScorecardNotFound
}

// ── Offers ────────────────────────────────────────────────────────────────────

func (r *stubRepo) FindOffers(_ context.Context, orgID, applicationID string) ([]*recruitment.Offer, error) {
	var out []*recruitment.Offer
	for _, o := range r.offers {
		if o.OrgID == orgID && o.ApplicationID == applicationID {
			out = append(out, o)
		}
	}
	return out, nil
}
func (r *stubRepo) FindOfferByRef(_ context.Context, orgID, ref string) (*recruitment.Offer, error) {
	for _, o := range r.offers {
		if o.OrgID == orgID && matchRef(o.ID, o.PublicID, ref) {
			return o, nil
		}
	}
	return nil, nil
}
func (r *stubRepo) CreateOffer(_ context.Context, o *recruitment.Offer) error {
	o.ID = r.nextID("ofr")
	o.PublicID = "pub_" + o.ID
	o.Status = recruitment.OfferStatusDraft
	o.CreatedAt, o.UpdatedAt = time.Now(), time.Now()
	r.offers[o.ID] = o
	return nil
}
func (r *stubRepo) UpdateOffer(_ context.Context, o *recruitment.Offer) error {
	if _, ok := r.offers[o.ID]; !ok {
		return recruitment.ErrOfferNotFound
	}
	o.UpdatedAt = time.Now()
	r.offers[o.ID] = o
	return nil
}
func (r *stubRepo) SetOfferApprovalInstance(_ context.Context, id, instanceID string, status recruitment.OfferStatus) error {
	o, ok := r.offers[id]
	if !ok {
		return recruitment.ErrOfferNotFound
	}
	o.ApprovalInstanceID, o.Status = &instanceID, status
	return nil
}
func (r *stubRepo) UpdateOfferStatus(_ context.Context, id string, status recruitment.OfferStatus) error {
	o, ok := r.offers[id]
	if !ok {
		return recruitment.ErrOfferNotFound
	}
	o.Status = status
	return nil
}

// ── Referrals ─────────────────────────────────────────────────────────────────

func (r *stubRepo) FindReferrals(_ context.Context, orgID string, _ recruitment.ReferralListFilter) ([]*recruitment.Referral, error) {
	var out []*recruitment.Referral
	for _, ref := range r.referrals {
		if ref.OrgID == orgID {
			out = append(out, ref)
		}
	}
	return out, nil
}
func (r *stubRepo) CountReferrals(ctx context.Context, orgID string, f recruitment.ReferralListFilter) (int, error) {
	out, _ := r.FindReferrals(ctx, orgID, f)
	return len(out), nil
}
func (r *stubRepo) FindReferralByRef(_ context.Context, orgID, ref string) (*recruitment.Referral, error) {
	for _, rf := range r.referrals {
		if rf.OrgID == orgID && matchRef(rf.ID, rf.PublicID, ref) {
			return rf, nil
		}
	}
	return nil, nil
}
func (r *stubRepo) CreateReferral(_ context.Context, rf *recruitment.Referral) error {
	rf.ID = r.nextID("ref")
	rf.PublicID = "pub_" + rf.ID
	rf.Status = recruitment.ReferralStatusSubmitted
	rf.BonusCurrency = "USD"
	rf.CreatedAt, rf.UpdatedAt = time.Now(), time.Now()
	r.referrals[rf.ID] = rf
	return nil
}
func (r *stubRepo) UpdateReferral(_ context.Context, rf *recruitment.Referral) error {
	if _, ok := r.referrals[rf.ID]; !ok {
		return recruitment.ErrReferralNotFound
	}
	if rf.Status == recruitment.ReferralStatusBonusPaid && rf.PaidAt == nil {
		now := time.Now()
		rf.PaidAt = &now
	}
	rf.UpdatedAt = time.Now()
	r.referrals[rf.ID] = rf
	return nil
}

var _ recruitment.Repository = (*stubRepo)(nil)

// ── Stub approvals.Service ──────────────────────────────────────────────────

type stubApprovalsSvc struct {
	defaultTemplate *approvals.ApprovalTemplate
}

func (s *stubApprovalsSvc) FindDefault(_ context.Context, _ string, _ approvals.ActionType) (*approvals.ApprovalTemplate, error) {
	return s.defaultTemplate, nil
}
func (s *stubApprovalsSvc) CreateInstance(_ context.Context, orgID string, req approvals.CreateInstanceRequest) (*approvals.ApprovalInstance, error) {
	return &approvals.ApprovalInstance{
		ID: "inst-" + req.EntityID, OrgID: orgID, TemplateID: &req.TemplateID,
		EntityType: req.EntityType, EntityID: req.EntityID, RequestedBy: req.RequestedBy,
		CurrentLevel: 1, OverallStatus: approvals.InstanceStatusPending,
	}, nil
}
func (s *stubApprovalsSvc) ListTemplates(context.Context, string, string) (*approvals.TemplateListResponse, error) {
	return nil, nil
}
func (s *stubApprovalsSvc) GetTemplate(context.Context, string, string) (*approvals.ApprovalTemplate, error) {
	return nil, nil
}
func (s *stubApprovalsSvc) CreateTemplate(context.Context, string, string, approvals.CreateTemplateRequest) (*approvals.ApprovalTemplate, error) {
	return nil, nil
}
func (s *stubApprovalsSvc) UpdateTemplate(context.Context, string, string, approvals.UpdateTemplateRequest) (*approvals.ApprovalTemplate, error) {
	return nil, nil
}
func (s *stubApprovalsSvc) DeleteTemplate(context.Context, string, string) error { return nil }
func (s *stubApprovalsSvc) GetInstance(context.Context, string, string) (*approvals.ApprovalInstance, error) {
	return nil, nil
}
func (s *stubApprovalsSvc) Decide(context.Context, string, string, string, approvals.DecisionRequest) (*approvals.ApprovalInstance, error) {
	return nil, nil
}
func (s *stubApprovalsSvc) CancelInstance(context.Context, string, string, string) (*approvals.ApprovalInstance, error) {
	return nil, nil
}
func (s *stubApprovalsSvc) RegisterCallback(string, approvals.EntityCallback) {}
func (s *stubApprovalsSvc) ListInstances(context.Context, string, int, int, string, string) (*approvals.InstanceListResponse, error) {
	return nil, nil
}

var _ approvals.Service = (*stubApprovalsSvc)(nil)

// ── Stub EmployeeCreator ─────────────────────────────────────────────────────

type stubEmployeeCreator struct {
	seq            int
	createErr      error
	created        []recruitment.HireEmployeeRequest
	afterHireCalls []string // employeeIDs passed to AfterHireCommit
}

func (e *stubEmployeeCreator) CreateEmployeeTx(_ context.Context, _ pgx.Tx, _, _ string, req recruitment.HireEmployeeRequest) (string, string, error) {
	if e.createErr != nil {
		return "", "", e.createErr
	}
	e.seq++
	id := fmt.Sprintf("emp_%d", e.seq)
	e.created = append(e.created, req)
	return id, "pub_" + id, nil
}
func (e *stubEmployeeCreator) AfterHireCommit(_ context.Context, _, _, employeeID string) {
	e.afterHireCalls = append(e.afterHireCalls, employeeID)
}

var _ recruitment.EmployeeCreator = (*stubEmployeeCreator)(nil)

// ── Test helpers ─────────────────────────────────────────────────────────────

const testOrg = "org_1"

// newTestSvc passes a nil RehireChecker deliberately: it is a nil-safe
// dependency, and these tests are about the recruitment pipeline rather than
// the exit-management seam. Candidate creation must work identically with no
// checker wired — see TestCreateCandidate_NilRehireCheckerIsSafe.
func newTestSvc(appSvc approvals.Service) (recruitment.Service, *stubRepo) {
	repo := newStubRepo()
	return recruitment.NewService(repo, appSvc, &stubEmployeeCreator{}, nil), repo
}

func newTestSvcWithEmployeeCreator(appSvc approvals.Service, ec recruitment.EmployeeCreator) (recruitment.Service, *stubRepo) {
	repo := newStubRepo()
	return recruitment.NewService(repo, appSvc, ec, nil), repo
}

func seedPipelineWithStages(t *testing.T, repo *stubRepo, kinds ...recruitment.StageKind) (*recruitment.Pipeline, []*recruitment.Stage) {
	t.Helper()
	p := &recruitment.Pipeline{OrgID: testOrg, Name: "Engineering", IsActive: true, CreatedBy: "user_1"}
	if err := repo.CreatePipeline(context.Background(), p); err != nil {
		t.Fatalf("seed pipeline: %v", err)
	}
	var stages []*recruitment.Stage
	for i, k := range kinds {
		s := &recruitment.Stage{OrgID: testOrg, PipelineID: p.ID, Name: string(k), Position: i, StageKind: k}
		if err := repo.CreateStage(context.Background(), s); err != nil {
			t.Fatalf("seed stage: %v", err)
		}
		stages = append(stages, s)
	}
	return p, stages
}

// ============================================================
// Requisition approval branching
// ============================================================

func TestSubmitRequisition_NoTemplate_AutoApproves(t *testing.T) {
	svc, _ := newTestSvc(&stubApprovalsSvc{defaultTemplate: nil})
	ctx := context.Background()

	r, err := svc.CreateRequisition(ctx, testOrg, "user_1", recruitment.CreateRequisitionRequest{Title: "Backend Engineer"})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	submitted, err := svc.SubmitRequisition(ctx, testOrg, r.ID, "user_1")
	if err != nil {
		t.Fatalf("submit: %v", err)
	}
	if submitted.Status != recruitment.RequisitionStatusApproved {
		t.Errorf("expected auto-approve when no template exists, got status=%q", submitted.Status)
	}
}

func TestSubmitRequisition_WithTemplate_GoesPendingApproval(t *testing.T) {
	tmpl := &approvals.ApprovalTemplate{ID: "tmpl_1", ActionType: approvals.ActionTypeJobRequisition}
	svc, _ := newTestSvc(&stubApprovalsSvc{defaultTemplate: tmpl})
	ctx := context.Background()

	r, err := svc.CreateRequisition(ctx, testOrg, "user_1", recruitment.CreateRequisitionRequest{Title: "Backend Engineer"})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	submitted, err := svc.SubmitRequisition(ctx, testOrg, r.ID, "user_1")
	if err != nil {
		t.Fatalf("submit: %v", err)
	}
	if submitted.Status != recruitment.RequisitionStatusPendingApproval {
		t.Errorf("expected pending_approval when a template exists, got status=%q", submitted.Status)
	}
	if submitted.ApprovalInstanceID == nil {
		t.Error("expected approval_instance_id to be set")
	}
}

func TestHandleApprovalDecision_Idempotent(t *testing.T) {
	tmpl := &approvals.ApprovalTemplate{ID: "tmpl_1", ActionType: approvals.ActionTypeJobRequisition}
	svc, _ := newTestSvc(&stubApprovalsSvc{defaultTemplate: tmpl})
	ctx := context.Background()

	r, _ := svc.CreateRequisition(ctx, testOrg, "user_1", recruitment.CreateRequisitionRequest{Title: "X"})
	r, err := svc.SubmitRequisition(ctx, testOrg, r.ID, "user_1")
	if err != nil {
		t.Fatalf("submit: %v", err)
	}

	if err := svc.HandleApprovalDecision(ctx, testOrg, r.ID, true); err != nil {
		t.Fatalf("first decision: %v", err)
	}
	after1, _ := svc.GetRequisition(ctx, testOrg, r.ID)
	if after1.Status != recruitment.RequisitionStatusApproved {
		t.Fatalf("expected approved, got %q", after1.Status)
	}

	// A second callback (e.g. a duplicate webhook) must be a no-op — the
	// idempotency guard mirrors promotions.HandleApprovalDecision.
	if err := svc.HandleApprovalDecision(ctx, testOrg, r.ID, false); err != nil {
		t.Fatalf("second decision: %v", err)
	}
	after2, _ := svc.GetRequisition(ctx, testOrg, r.ID)
	if after2.Status != recruitment.RequisitionStatusApproved {
		t.Errorf("expected status to remain approved after a second callback, got %q", after2.Status)
	}
}

func TestSubmitRequisition_RejectedByApproval(t *testing.T) {
	tmpl := &approvals.ApprovalTemplate{ID: "tmpl_1", ActionType: approvals.ActionTypeJobRequisition}
	svc, _ := newTestSvc(&stubApprovalsSvc{defaultTemplate: tmpl})
	ctx := context.Background()

	r, _ := svc.CreateRequisition(ctx, testOrg, "user_1", recruitment.CreateRequisitionRequest{Title: "X"})
	r, _ = svc.SubmitRequisition(ctx, testOrg, r.ID, "user_1")

	if err := svc.HandleApprovalDecision(ctx, testOrg, r.ID, false); err != nil {
		t.Fatalf("decision: %v", err)
	}
	after, _ := svc.GetRequisition(ctx, testOrg, r.ID)
	if after.Status != recruitment.RequisitionStatusRejected {
		t.Errorf("expected rejected, got %q", after.Status)
	}
}

// ============================================================
// Candidate dedup
// ============================================================

func TestCreateCandidate_DuplicateEmail_CaseInsensitive(t *testing.T) {
	svc, _ := newTestSvc(&stubApprovalsSvc{})
	ctx := context.Background()
	email := "Jane@Example.com"

	_, err := svc.CreateCandidate(ctx, testOrg, nil, recruitment.CreateCandidateRequest{FirstName: "Jane", Email: &email})
	if err != nil {
		t.Fatalf("first create: %v", err)
	}

	dup := "jane@example.com"
	_, err = svc.CreateCandidate(ctx, testOrg, nil, recruitment.CreateCandidateRequest{FirstName: "Jane2", Email: &dup})
	if !errors.Is(err, recruitment.ErrCandidateEmailExists) {
		t.Fatalf("expected ErrCandidateEmailExists for a case-different duplicate, got %v", err)
	}
}

// TestCreateCandidate_NilRehireCheckerIsSafe pins the nil-safety the wiring
// depends on: an install without exit management still recruits. A nil
// checker must produce no flag rather than a nil-pointer panic inside
// candidate creation.
func TestCreateCandidate_NilRehireCheckerIsSafe(t *testing.T) {
	svc, _ := newTestSvc(nil) // nil RehireChecker
	email := "someone@example.com"
	c, err := svc.CreateCandidate(context.Background(), testOrg, nil,
		recruitment.CreateCandidateRequest{FirstName: "Alice", Email: &email})
	if err != nil {
		t.Fatalf("CreateCandidate with a nil rehire checker: %v", err)
	}
	if c.RehireFlag != nil {
		t.Errorf("a nil checker produced a flag: %+v", c.RehireFlag)
	}
}

func TestCreateCandidate_RequiresFirstName(t *testing.T) {
	svc, _ := newTestSvc(&stubApprovalsSvc{})
	_, err := svc.CreateCandidate(context.Background(), testOrg, nil, recruitment.CreateCandidateRequest{FirstName: "  "})
	if !errors.Is(err, recruitment.ErrFirstNameRequired) {
		t.Fatalf("expected ErrFirstNameRequired, got %v", err)
	}
}

// ============================================================
// Posting slug
// ============================================================

func TestCreatePosting_SlugifiesTitle(t *testing.T) {
	svc, repo := newTestSvc(&stubApprovalsSvc{})
	ctx := context.Background()
	pipeline, _ := seedPipelineWithStages(t, repo, recruitment.StageKindApplied)
	repo.pipelines[pipeline.ID].IsDefault = true

	req, err := svc.CreateRequisition(ctx, testOrg, "user_1", recruitment.CreateRequisitionRequest{Title: "X"})
	if err != nil {
		t.Fatalf("create requisition: %v", err)
	}
	p, err := svc.CreatePosting(ctx, testOrg, "user_1", recruitment.CreatePostingRequest{
		RequisitionID: req.ID, Title: "Senior Backend Engineer!",
	})
	if err != nil {
		t.Fatalf("create posting: %v", err)
	}
	if p.PublicSlug != "senior-backend-engineer" {
		t.Errorf("expected slugified title, got %q", p.PublicSlug)
	}
}

func TestCreatePosting_SlugTaken(t *testing.T) {
	svc, repo := newTestSvc(&stubApprovalsSvc{})
	ctx := context.Background()
	pipeline, _ := seedPipelineWithStages(t, repo, recruitment.StageKindApplied)
	repo.pipelines[pipeline.ID].IsDefault = true

	req, _ := svc.CreateRequisition(ctx, testOrg, "user_1", recruitment.CreateRequisitionRequest{Title: "X"})
	slug := "backend-engineer"
	if _, err := svc.CreatePosting(ctx, testOrg, "user_1", recruitment.CreatePostingRequest{
		RequisitionID: req.ID, Title: "Backend Engineer", PublicSlug: &slug,
	}); err != nil {
		t.Fatalf("create posting 1: %v", err)
	}
	req2, _ := svc.CreateRequisition(ctx, testOrg, "user_1", recruitment.CreateRequisitionRequest{Title: "Y"})
	_, err := svc.CreatePosting(ctx, testOrg, "user_1", recruitment.CreatePostingRequest{
		RequisitionID: req2.ID, Title: "Another Role", PublicSlug: &slug,
	})
	if !errors.Is(err, recruitment.ErrSlugTaken) {
		t.Fatalf("expected ErrSlugTaken, got %v", err)
	}
}

func TestCreatePosting_NoDefaultPipeline_ReturnsErrPipelineRequired(t *testing.T) {
	svc, _ := newTestSvc(&stubApprovalsSvc{})
	ctx := context.Background()
	req, _ := svc.CreateRequisition(ctx, testOrg, "user_1", recruitment.CreateRequisitionRequest{Title: "X"})
	_, err := svc.CreatePosting(ctx, testOrg, "user_1", recruitment.CreatePostingRequest{RequisitionID: req.ID, Title: "Y"})
	if !errors.Is(err, recruitment.ErrPipelineRequired) {
		t.Fatalf("expected ErrPipelineRequired when no default pipeline exists, got %v", err)
	}
}

// ============================================================
// Application stage move
// ============================================================

func seedApplication(t *testing.T, svc recruitment.Service, repo *stubRepo, pipeline *recruitment.Pipeline) *recruitment.Application {
	t.Helper()
	ctx := context.Background()
	cand, err := svc.CreateCandidate(ctx, testOrg, nil, recruitment.CreateCandidateRequest{FirstName: "Alex"})
	if err != nil {
		t.Fatalf("seed candidate: %v", err)
	}
	req, _ := svc.CreateRequisition(ctx, testOrg, "user_1", recruitment.CreateRequisitionRequest{Title: "Role"})
	pID := pipeline.ID
	posting, err := svc.CreatePosting(ctx, testOrg, "user_1", recruitment.CreatePostingRequest{
		RequisitionID: req.ID, Title: "Role", PipelineID: &pID,
	})
	if err != nil {
		t.Fatalf("seed posting: %v", err)
	}
	app, err := svc.CreateApplication(ctx, testOrg, nil, recruitment.CreateApplicationRequest{CandidateID: cand.ID, PostingID: posting.ID})
	if err != nil {
		t.Fatalf("seed application: %v", err)
	}
	return app
}

func TestMoveApplication_RejectsStageFromDifferentPipeline(t *testing.T) {
	svc, repo := newTestSvc(&stubApprovalsSvc{})
	pipelineA, _ := seedPipelineWithStages(t, repo, recruitment.StageKindApplied, recruitment.StageKindInProgress)
	pipelineB, stagesB := seedPipelineWithStages(t, repo, recruitment.StageKindApplied)

	app := seedApplication(t, svc, repo, pipelineA)

	_, err := svc.MoveApplication(context.Background(), testOrg, app.ID, "user_1", recruitment.MoveApplicationRequest{StageID: stagesB[0].ID})
	if !errors.Is(err, recruitment.ErrStageNotInPipeline) {
		t.Fatalf("expected ErrStageNotInPipeline for a stage from pipeline %q while app is on %q, got %v", pipelineB.ID, pipelineA.ID, err)
	}
}

func TestMoveApplication_TerminalStageSetsStatusAndTimestamp(t *testing.T) {
	svc, repo := newTestSvc(&stubApprovalsSvc{})
	pipeline, stages := seedPipelineWithStages(t, repo, recruitment.StageKindApplied, recruitment.StageKindHired)

	app := seedApplication(t, svc, repo, pipeline)

	updated, err := svc.MoveApplication(context.Background(), testOrg, app.ID, "user_1", recruitment.MoveApplicationRequest{StageID: stages[1].ID})
	if err != nil {
		t.Fatalf("move: %v", err)
	}
	if updated.Status != recruitment.ApplicationStatusHired {
		t.Errorf("expected status=hired after moving to a hired-kind stage, got %q", updated.Status)
	}
	if updated.HiredAt == nil {
		t.Error("expected hired_at to be set")
	}
}

func TestMoveApplication_NonActiveApplication_Rejected(t *testing.T) {
	svc, repo := newTestSvc(&stubApprovalsSvc{})
	pipeline, stages := seedPipelineWithStages(t, repo, recruitment.StageKindApplied, recruitment.StageKindRejected, recruitment.StageKindHired)

	app := seedApplication(t, svc, repo, pipeline)
	if _, err := svc.MoveApplication(context.Background(), testOrg, app.ID, "user_1", recruitment.MoveApplicationRequest{StageID: stages[1].ID}); err != nil {
		t.Fatalf("first move (to rejected): %v", err)
	}

	_, err := svc.MoveApplication(context.Background(), testOrg, app.ID, "user_1", recruitment.MoveApplicationRequest{StageID: stages[2].ID})
	if !errors.Is(err, recruitment.ErrApplicationNotActive) {
		t.Fatalf("expected ErrApplicationNotActive for a rejected application, got %v", err)
	}
}

func TestCreateApplication_DuplicateForSamePosting_Rejected(t *testing.T) {
	svc, repo := newTestSvc(&stubApprovalsSvc{})
	ctx := context.Background()
	pipeline, _ := seedPipelineWithStages(t, repo, recruitment.StageKindApplied)

	cand, _ := svc.CreateCandidate(ctx, testOrg, nil, recruitment.CreateCandidateRequest{FirstName: "Sam"})
	req, _ := svc.CreateRequisition(ctx, testOrg, "user_1", recruitment.CreateRequisitionRequest{Title: "Role"})
	pID := pipeline.ID
	posting, _ := svc.CreatePosting(ctx, testOrg, "user_1", recruitment.CreatePostingRequest{RequisitionID: req.ID, Title: "Role", PipelineID: &pID})

	if _, err := svc.CreateApplication(ctx, testOrg, nil, recruitment.CreateApplicationRequest{CandidateID: cand.ID, PostingID: posting.ID}); err != nil {
		t.Fatalf("first application: %v", err)
	}
	_, err := svc.CreateApplication(ctx, testOrg, nil, recruitment.CreateApplicationRequest{CandidateID: cand.ID, PostingID: posting.ID})
	if !errors.Is(err, recruitment.ErrDuplicateApplication) {
		t.Fatalf("expected ErrDuplicateApplication, got %v", err)
	}
}

func TestRejectApplication_RequiresReason(t *testing.T) {
	svc, repo := newTestSvc(&stubApprovalsSvc{})
	pipeline, _ := seedPipelineWithStages(t, repo, recruitment.StageKindApplied)
	app := seedApplication(t, svc, repo, pipeline)

	_, err := svc.RejectApplication(context.Background(), testOrg, app.ID, "user_1", recruitment.RejectApplicationRequest{Reason: "  "})
	if !errors.Is(err, recruitment.ErrRejectReasonRequired) {
		t.Fatalf("expected ErrRejectReasonRequired, got %v", err)
	}
}

func TestWithdrawApplication_TwiceIsRejected(t *testing.T) {
	svc, repo := newTestSvc(&stubApprovalsSvc{})
	pipeline, _ := seedPipelineWithStages(t, repo, recruitment.StageKindApplied)
	app := seedApplication(t, svc, repo, pipeline)

	if _, err := svc.WithdrawApplication(context.Background(), testOrg, app.ID, "user_1"); err != nil {
		t.Fatalf("first withdraw: %v", err)
	}
	_, err := svc.WithdrawApplication(context.Background(), testOrg, app.ID, "user_1")
	if !errors.Is(err, recruitment.ErrApplicationNotActive) {
		t.Fatalf("expected ErrApplicationNotActive on a second withdraw, got %v", err)
	}
}

// ============================================================
// Resume validation (no DB interaction on the rejection path)
// ============================================================

func TestUploadResume_RejectsNonPDF(t *testing.T) {
	svc, _ := newTestSvc(&stubApprovalsSvc{})
	notAPDF := []byte("this is plain text, not a PDF")
	_, err := svc.UploadResume(context.Background(), testOrg, "does-not-matter", notAPDF, "resume.pdf")
	if !errors.Is(err, recruitment.ErrInvalidResumeType) {
		t.Fatalf("expected ErrInvalidResumeType for non-PDF content regardless of filename, got %v", err)
	}
}

func TestUploadResume_RejectsOversized(t *testing.T) {
	svc, _ := newTestSvc(&stubApprovalsSvc{})
	huge := make([]byte, 11*1024*1024) // over the 10MB limit
	_, err := svc.UploadResume(context.Background(), testOrg, "does-not-matter", huge, "resume.pdf")
	if !errors.Is(err, recruitment.ErrResumeTooLarge) {
		t.Fatalf("expected ErrResumeTooLarge, got %v", err)
	}
}

// ============================================================
// Filter normalisation
// ============================================================

func TestRequisitionListFilter_Normalise_Clamps(t *testing.T) {
	f := recruitment.RequisitionListFilter{Limit: -5, Offset: -1}
	f.Normalise()
	if f.Limit != recruitment.DefaultLimit {
		t.Errorf("expected default limit for a non-positive input, got %d", f.Limit)
	}
	if f.Offset != 0 {
		t.Errorf("expected offset clamped to 0, got %d", f.Offset)
	}

	f2 := recruitment.RequisitionListFilter{Limit: 10000}
	f2.Normalise()
	if f2.Limit != recruitment.MaxLimit {
		t.Errorf("expected limit clamped to MaxLimit, got %d", f2.Limit)
	}
}

// ============================================================
// Phase 4B — helpers
// ============================================================

func strPtr(s string) *string { return &s }

func seedApplicationFull(t *testing.T, svc recruitment.Service, repo *stubRepo, pipeline *recruitment.Pipeline) (*recruitment.Application, *recruitment.Candidate, *recruitment.Requisition) {
	t.Helper()
	ctx := context.Background()
	cand, err := svc.CreateCandidate(ctx, testOrg, nil, recruitment.CreateCandidateRequest{FirstName: "Alex", LastName: strPtr("Doe")})
	if err != nil {
		t.Fatalf("seed candidate: %v", err)
	}
	req, err := svc.CreateRequisition(ctx, testOrg, "user_1", recruitment.CreateRequisitionRequest{Title: "Role"})
	if err != nil {
		t.Fatalf("seed requisition: %v", err)
	}
	pID := pipeline.ID
	posting, err := svc.CreatePosting(ctx, testOrg, "user_1", recruitment.CreatePostingRequest{
		RequisitionID: req.ID, Title: "Role", PipelineID: &pID,
	})
	if err != nil {
		t.Fatalf("seed posting: %v", err)
	}
	app, err := svc.CreateApplication(ctx, testOrg, nil, recruitment.CreateApplicationRequest{CandidateID: cand.ID, PostingID: posting.ID})
	if err != nil {
		t.Fatalf("seed application: %v", err)
	}
	return app, cand, req
}

func seedHiredApplication(t *testing.T, svc recruitment.Service, repo *stubRepo) (*recruitment.Application, *recruitment.Candidate, *recruitment.Requisition) {
	t.Helper()
	ctx := context.Background()
	pipeline, stages := seedPipelineWithStages(t, repo, recruitment.StageKindApplied, recruitment.StageKindHired)
	app, cand, req := seedApplicationFull(t, svc, repo, pipeline)
	moved, err := svc.MoveApplication(ctx, testOrg, app.ID, "user_1", recruitment.MoveApplicationRequest{StageID: stages[1].ID})
	if err != nil {
		t.Fatalf("move to hired stage: %v", err)
	}
	return moved, cand, req
}

func seedInterviewWithPanelist(t *testing.T, svc recruitment.Service, repo *stubRepo) (*recruitment.Interview, string, string) {
	t.Helper()
	ctx := context.Background()
	pipeline, _ := seedPipelineWithStages(t, repo, recruitment.StageKindApplied, recruitment.StageKindHired)
	app := seedApplication(t, svc, repo, pipeline)
	interview, err := svc.CreateInterview(ctx, testOrg, app.ID, "user_1", recruitment.CreateInterviewRequest{ScheduledAt: time.Now().Format(time.RFC3339)})
	if err != nil {
		t.Fatalf("create interview: %v", err)
	}
	employeeID, userID := "emp_panelist_1", "user_panelist_1"
	repo.employeeUsers[userID] = employeeID
	if _, err := svc.AddPanelist(ctx, testOrg, interview.ID, recruitment.AddPanelistRequest{EmployeeID: employeeID}); err != nil {
		t.Fatalf("add panelist: %v", err)
	}
	return interview, employeeID, userID
}

// ============================================================
// Scorecards — visibility, immutability, panelist narrowing
// ============================================================

func TestListScorecards_HidesOthersUntilOwnSubmitted(t *testing.T) {
	svc, repo := newTestSvc(&stubApprovalsSvc{})
	ctx := context.Background()
	interview, _, userA := seedInterviewWithPanelist(t, svc, repo)

	employeeB, userB := "emp_panelist_2", "user_panelist_2"
	repo.employeeUsers[userB] = employeeB
	if _, err := svc.AddPanelist(ctx, testOrg, interview.ID, recruitment.AddPanelistRequest{EmployeeID: employeeB}); err != nil {
		t.Fatalf("add panelist B: %v", err)
	}

	ratingA := 5
	if _, err := svc.UpsertOwnScorecard(ctx, testOrg, interview.ID, userA, recruitment.UpsertScorecardRequest{OverallRating: &ratingA}); err != nil {
		t.Fatalf("A upsert: %v", err)
	}
	if _, err := svc.SubmitOwnScorecard(ctx, testOrg, interview.ID, userA); err != nil {
		t.Fatalf("A submit: %v", err)
	}

	// B has not submitted their own yet — must not see A's, even though
	// A's is already submitted. This is the core rule: reveal is gated on
	// the CALLER's own submission, not on whether anyone else has submitted.
	listB, err := svc.ListScorecards(ctx, testOrg, interview.ID, userB)
	if err != nil {
		t.Fatalf("list as B before B submits: %v", err)
	}
	if len(listB) != 0 {
		t.Fatalf("expected B to see nothing before submitting their own, got %d", len(listB))
	}

	ratingB := 3
	if _, err := svc.UpsertOwnScorecard(ctx, testOrg, interview.ID, userB, recruitment.UpsertScorecardRequest{OverallRating: &ratingB}); err != nil {
		t.Fatalf("B upsert: %v", err)
	}
	if _, err := svc.SubmitOwnScorecard(ctx, testOrg, interview.ID, userB); err != nil {
		t.Fatalf("B submit: %v", err)
	}

	listB2, err := svc.ListScorecards(ctx, testOrg, interview.ID, userB)
	if err != nil {
		t.Fatalf("list as B after B submits: %v", err)
	}
	if len(listB2) != 2 {
		t.Fatalf("expected B to see both submitted scorecards after submitting their own, got %d", len(listB2))
	}
}

func TestListScorecards_NonPanelistSeesOnlySubmitted(t *testing.T) {
	svc, repo := newTestSvc(&stubApprovalsSvc{})
	ctx := context.Background()
	interview, _, userA := seedInterviewWithPanelist(t, svc, repo)

	rating := 4
	if _, err := svc.UpsertOwnScorecard(ctx, testOrg, interview.ID, userA, recruitment.UpsertScorecardRequest{OverallRating: &rating}); err != nil {
		t.Fatalf("upsert: %v", err)
	}

	// An HR admin auditing after the fact, with no employee record at all —
	// before A submits, nothing is visible (drafts are always private).
	admin := "user_hr_admin"
	before, err := svc.ListScorecards(ctx, testOrg, interview.ID, admin)
	if err != nil {
		t.Fatalf("list as admin before submit: %v", err)
	}
	if len(before) != 0 {
		t.Fatalf("expected admin to see nothing while A's scorecard is still a draft, got %d", len(before))
	}

	if _, err := svc.SubmitOwnScorecard(ctx, testOrg, interview.ID, userA); err != nil {
		t.Fatalf("submit: %v", err)
	}
	after, err := svc.ListScorecards(ctx, testOrg, interview.ID, admin)
	if err != nil {
		t.Fatalf("list as admin after submit: %v", err)
	}
	if len(after) != 1 {
		t.Fatalf("expected admin (not a panelist) to see the submitted scorecard, got %d", len(after))
	}
}

func TestSubmitOwnScorecard_Twice_Fails(t *testing.T) {
	svc, repo := newTestSvc(&stubApprovalsSvc{})
	ctx := context.Background()
	interview, _, userID := seedInterviewWithPanelist(t, svc, repo)

	if _, err := svc.UpsertOwnScorecard(ctx, testOrg, interview.ID, userID, recruitment.UpsertScorecardRequest{}); err != nil {
		t.Fatalf("upsert: %v", err)
	}
	if _, err := svc.SubmitOwnScorecard(ctx, testOrg, interview.ID, userID); err != nil {
		t.Fatalf("first submit: %v", err)
	}
	_, err := svc.SubmitOwnScorecard(ctx, testOrg, interview.ID, userID)
	if !errors.Is(err, recruitment.ErrScorecardAlreadySubmitted) {
		t.Fatalf("expected ErrScorecardAlreadySubmitted, got %v", err)
	}
}

func TestUpsertOwnScorecard_AfterSubmit_Immutable(t *testing.T) {
	svc, repo := newTestSvc(&stubApprovalsSvc{})
	ctx := context.Background()
	interview, _, userID := seedInterviewWithPanelist(t, svc, repo)

	if _, err := svc.UpsertOwnScorecard(ctx, testOrg, interview.ID, userID, recruitment.UpsertScorecardRequest{}); err != nil {
		t.Fatalf("upsert: %v", err)
	}
	if _, err := svc.SubmitOwnScorecard(ctx, testOrg, interview.ID, userID); err != nil {
		t.Fatalf("submit: %v", err)
	}
	_, err := svc.UpsertOwnScorecard(ctx, testOrg, interview.ID, userID, recruitment.UpsertScorecardRequest{})
	if !errors.Is(err, recruitment.ErrScorecardAlreadySubmitted) {
		t.Fatalf("expected ErrScorecardAlreadySubmitted on edit-after-submit, got %v", err)
	}
}

func TestUpsertOwnScorecard_NonPanelist_Rejected(t *testing.T) {
	svc, repo := newTestSvc(&stubApprovalsSvc{})
	ctx := context.Background()
	interview, _, _ := seedInterviewWithPanelist(t, svc, repo)

	outsider := "user_outsider"
	repo.employeeUsers[outsider] = "emp_outsider"

	_, err := svc.UpsertOwnScorecard(ctx, testOrg, interview.ID, outsider, recruitment.UpsertScorecardRequest{})
	if !errors.Is(err, recruitment.ErrNotAPanelist) {
		t.Fatalf("expected ErrNotAPanelist, got %v", err)
	}
}

func TestUpsertOwnScorecard_NoEmployeeRecord_Rejected(t *testing.T) {
	svc, repo := newTestSvc(&stubApprovalsSvc{})
	ctx := context.Background()
	interview, _, _ := seedInterviewWithPanelist(t, svc, repo)

	_, err := svc.UpsertOwnScorecard(ctx, testOrg, interview.ID, "user_with_no_employee_row", recruitment.UpsertScorecardRequest{})
	if !errors.Is(err, recruitment.ErrCallerHasNoEmployeeRecord) {
		t.Fatalf("expected ErrCallerHasNoEmployeeRecord, got %v", err)
	}
}

func TestUpsertOwnScorecard_InvalidScoreRange_Rejected(t *testing.T) {
	svc, repo := newTestSvc(&stubApprovalsSvc{})
	ctx := context.Background()
	interview, _, userID := seedInterviewWithPanelist(t, svc, repo)

	tooHigh := 6
	_, err := svc.UpsertOwnScorecard(ctx, testOrg, interview.ID, userID, recruitment.UpsertScorecardRequest{OverallRating: &tooHigh})
	if !errors.Is(err, recruitment.ErrInvalidScoreRange) {
		t.Fatalf("expected ErrInvalidScoreRange, got %v", err)
	}
}

func TestAddPanelist_Duplicate_Rejected(t *testing.T) {
	svc, repo := newTestSvc(&stubApprovalsSvc{})
	ctx := context.Background()
	interview, employeeID, _ := seedInterviewWithPanelist(t, svc, repo)

	_, err := svc.AddPanelist(ctx, testOrg, interview.ID, recruitment.AddPanelistRequest{EmployeeID: employeeID})
	if !errors.Is(err, recruitment.ErrPanelistAlreadyOnPanel) {
		t.Fatalf("expected ErrPanelistAlreadyOnPanel, got %v", err)
	}
}

// ============================================================
// Offers — approval branching (mirrors requisitions)
// ============================================================

func TestSubmitOffer_NoTemplate_AutoApproves(t *testing.T) {
	svc, repo := newTestSvc(&stubApprovalsSvc{defaultTemplate: nil})
	ctx := context.Background()
	pipeline, _ := seedPipelineWithStages(t, repo, recruitment.StageKindApplied, recruitment.StageKindHired)
	app, _, req := seedApplicationFull(t, svc, repo, pipeline)

	offer, err := svc.CreateOffer(ctx, testOrg, app.ID, "user_1", recruitment.CreateOfferRequest{RequisitionID: req.ID})
	if err != nil {
		t.Fatalf("create offer: %v", err)
	}
	submitted, err := svc.SubmitOffer(ctx, testOrg, offer.ID, "user_1")
	if err != nil {
		t.Fatalf("submit: %v", err)
	}
	if submitted.Status != recruitment.OfferStatusApproved {
		t.Errorf("expected auto-approve when no template exists, got status=%q", submitted.Status)
	}
}

func TestSubmitOffer_WithTemplate_GoesPendingApproval(t *testing.T) {
	tmpl := &approvals.ApprovalTemplate{ID: "tmpl_1", ActionType: approvals.ActionTypeOffer}
	svc, repo := newTestSvc(&stubApprovalsSvc{defaultTemplate: tmpl})
	ctx := context.Background()
	pipeline, _ := seedPipelineWithStages(t, repo, recruitment.StageKindApplied, recruitment.StageKindHired)
	app, _, req := seedApplicationFull(t, svc, repo, pipeline)

	offer, err := svc.CreateOffer(ctx, testOrg, app.ID, "user_1", recruitment.CreateOfferRequest{RequisitionID: req.ID})
	if err != nil {
		t.Fatalf("create offer: %v", err)
	}
	submitted, err := svc.SubmitOffer(ctx, testOrg, offer.ID, "user_1")
	if err != nil {
		t.Fatalf("submit: %v", err)
	}
	if submitted.Status != recruitment.OfferStatusPendingApproval {
		t.Errorf("expected pending_approval when a template exists, got status=%q", submitted.Status)
	}
	if submitted.ApprovalInstanceID == nil {
		t.Error("expected approval_instance_id to be set")
	}
}

func TestHandleOfferApprovalDecision_Idempotent(t *testing.T) {
	tmpl := &approvals.ApprovalTemplate{ID: "tmpl_1", ActionType: approvals.ActionTypeOffer}
	svc, repo := newTestSvc(&stubApprovalsSvc{defaultTemplate: tmpl})
	ctx := context.Background()
	pipeline, _ := seedPipelineWithStages(t, repo, recruitment.StageKindApplied, recruitment.StageKindHired)
	app, _, req := seedApplicationFull(t, svc, repo, pipeline)

	offer, _ := svc.CreateOffer(ctx, testOrg, app.ID, "user_1", recruitment.CreateOfferRequest{RequisitionID: req.ID})
	offer, err := svc.SubmitOffer(ctx, testOrg, offer.ID, "user_1")
	if err != nil {
		t.Fatalf("submit: %v", err)
	}

	if err := svc.HandleOfferApprovalDecision(ctx, testOrg, offer.ID, true); err != nil {
		t.Fatalf("first decision: %v", err)
	}
	after1, _ := svc.GetOffer(ctx, testOrg, offer.ID)
	if after1.Status != recruitment.OfferStatusApproved {
		t.Fatalf("expected approved, got %q", after1.Status)
	}

	// A second callback (e.g. a duplicate webhook) must be a no-op.
	if err := svc.HandleOfferApprovalDecision(ctx, testOrg, offer.ID, false); err != nil {
		t.Fatalf("second decision: %v", err)
	}
	after2, _ := svc.GetOffer(ctx, testOrg, offer.ID)
	if after2.Status != recruitment.OfferStatusApproved {
		t.Errorf("expected status to remain approved after a second callback, got %q", after2.Status)
	}
}

func TestRescindOffer_AfterAccepted_Rejected(t *testing.T) {
	svc, repo := newTestSvc(&stubApprovalsSvc{})
	ctx := context.Background()
	pipeline, _ := seedPipelineWithStages(t, repo, recruitment.StageKindApplied, recruitment.StageKindHired)
	app, _, req := seedApplicationFull(t, svc, repo, pipeline)

	offer, _ := svc.CreateOffer(ctx, testOrg, app.ID, "user_1", recruitment.CreateOfferRequest{RequisitionID: req.ID})
	offer, _ = svc.SubmitOffer(ctx, testOrg, offer.ID, "user_1") // auto-approved, no template
	offer, _ = svc.SendOffer(ctx, testOrg, offer.ID)
	offer, err := svc.AcceptOffer(ctx, testOrg, offer.ID)
	if err != nil {
		t.Fatalf("accept: %v", err)
	}

	_, err = svc.RescindOffer(ctx, testOrg, offer.ID)
	if !errors.Is(err, recruitment.ErrOfferWrongStatus) {
		t.Fatalf("expected ErrOfferWrongStatus rescinding an accepted offer, got %v", err)
	}
}

// ============================================================
// Hire conversion
// ============================================================

func TestHireApplication_WrongStatus_Rejected(t *testing.T) {
	svc, repo := newTestSvc(&stubApprovalsSvc{})
	ctx := context.Background()
	pipeline, _ := seedPipelineWithStages(t, repo, recruitment.StageKindApplied)
	app := seedApplication(t, svc, repo, pipeline) // still "active" — never moved to a hired-kind stage

	_, err := svc.HireApplication(ctx, testOrg, app.ID, "user_1", recruitment.HireApplicationRequest{})
	if !errors.Is(err, recruitment.ErrApplicationNotHired) {
		t.Fatalf("expected ErrApplicationNotHired, got %v", err)
	}
}

func TestHireApplication_Success_PopulatesConvertedEmployeeAndFilledCount(t *testing.T) {
	ec := &stubEmployeeCreator{}
	svc, repo := newTestSvcWithEmployeeCreator(&stubApprovalsSvc{}, ec)
	ctx := context.Background()
	app, cand, req := seedHiredApplication(t, svc, repo)

	res, err := svc.HireApplication(ctx, testOrg, app.ID, "user_1", recruitment.HireApplicationRequest{})
	if err != nil {
		t.Fatalf("hire: %v", err)
	}
	if res.EmployeeID == "" {
		t.Fatal("expected a non-empty employee ID")
	}
	if res.Application.ConvertedEmployeeID == nil || *res.Application.ConvertedEmployeeID != res.EmployeeID {
		t.Fatal("expected application.converted_employee_id to be set to the new employee ID")
	}

	updatedReq, _ := svc.GetRequisition(ctx, testOrg, req.ID)
	if updatedReq.FilledCount != 1 {
		t.Errorf("expected requisition filled_count incremented to 1, got %d", updatedReq.FilledCount)
	}

	if len(ec.afterHireCalls) != 1 || ec.afterHireCalls[0] != res.EmployeeID {
		t.Errorf("expected AfterHireCommit called exactly once with the new employee ID, got %v", ec.afterHireCalls)
	}
	if len(ec.created) != 1 {
		t.Fatalf("expected exactly one CreateEmployeeTx call, got %d", len(ec.created))
	}
	got := ec.created[0]
	if got.FirstName != cand.FirstName || got.SourceCandidateID != cand.ID {
		t.Errorf("expected hire request populated from the candidate (FirstName=%q SourceCandidateID=%q), got FirstName=%q SourceCandidateID=%q",
			cand.FirstName, cand.ID, got.FirstName, got.SourceCandidateID)
	}
}

func TestHireApplication_AlreadyHired_Rejected(t *testing.T) {
	ec := &stubEmployeeCreator{}
	svc, repo := newTestSvcWithEmployeeCreator(&stubApprovalsSvc{}, ec)
	ctx := context.Background()
	app, _, _ := seedHiredApplication(t, svc, repo)

	if _, err := svc.HireApplication(ctx, testOrg, app.ID, "user_1", recruitment.HireApplicationRequest{}); err != nil {
		t.Fatalf("first hire: %v", err)
	}
	_, err := svc.HireApplication(ctx, testOrg, app.ID, "user_1", recruitment.HireApplicationRequest{})
	if !errors.Is(err, recruitment.ErrApplicationAlreadyHired) {
		t.Fatalf("expected ErrApplicationAlreadyHired on a second hire call, got %v", err)
	}
}

func TestHireApplication_EmployeeCreatorFails_NothingConverted(t *testing.T) {
	ec := &stubEmployeeCreator{createErr: errors.New("boom")}
	svc, repo := newTestSvcWithEmployeeCreator(&stubApprovalsSvc{}, ec)
	ctx := context.Background()
	app, _, req := seedHiredApplication(t, svc, repo)

	if _, err := svc.HireApplication(ctx, testOrg, app.ID, "user_1", recruitment.HireApplicationRequest{}); err == nil {
		t.Fatal("expected an error when the employee creator fails")
	}

	after, _ := svc.GetApplication(ctx, testOrg, app.ID)
	if after.ConvertedEmployeeID != nil {
		t.Error("expected converted_employee_id to remain unset when employee creation fails")
	}
	updatedReq, _ := svc.GetRequisition(ctx, testOrg, req.ID)
	if updatedReq.FilledCount != 0 {
		t.Errorf("expected filled_count to remain 0 when employee creation fails, got %d", updatedReq.FilledCount)
	}
	if len(ec.afterHireCalls) != 0 {
		t.Errorf("expected AfterHireCommit not to be called when the transaction never commits, got %v", ec.afterHireCalls)
	}
}
