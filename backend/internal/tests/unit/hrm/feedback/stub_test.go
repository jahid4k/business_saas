// backend/internal/tests/unit/hrm/feedback/stub_test.go
// Stubs for the 360 feedback service: an in-memory repository, a scope
// authorizer, and a form engine whose instances can be driven per test.
package feedback_test

import (
	"context"
	"fmt"
	"strings"

	"github.com/shopspring/decimal"

	"github.com/mridha/businesssaas/internal/authz"
	"github.com/mridha/businesssaas/internal/hrm/feedback"
	"github.com/mridha/businesssaas/internal/platform/forms"
)

const (
	testOrg     = "org_1"
	ownerUserID = "usr_owner"
	otherUserID = "usr_other"
	subjectEmp  = "emp_subject"
	otherEmp    = "emp_other"
	templateID  = "tmpl_360"
)

// ── Repository stub ──────────────────────────────────────────────────────────

type stubRepo struct {
	seq int

	cycles    map[string]*feedback.Cycle
	requests  map[string]*feedback.Request
	employees map[string]*feedback.EmployeeSubject
	// employeeUsers maps platform user_id → hrm_employees.id.
	employeeUsers map[string]string
}

var _ feedback.Repository = (*stubRepo)(nil)

func newStubRepo() *stubRepo {
	return &stubRepo{
		cycles:        map[string]*feedback.Cycle{},
		requests:      map[string]*feedback.Request{},
		employees:     map[string]*feedback.EmployeeSubject{},
		employeeUsers: map[string]string{},
	}
}

func (r *stubRepo) nextID(prefix string) string {
	r.seq++
	return fmt.Sprintf("%s_%d", prefix, r.seq)
}

func matchRef(id, publicID, ref string) bool { return id == ref || publicID == ref }

func (r *stubRepo) FindCycles(_ context.Context, orgID string, f feedback.CycleListFilter) ([]*feedback.Cycle, error) {
	out := make([]*feedback.Cycle, 0)
	for _, c := range r.cycles {
		if c.OrgID == orgID && (f.Status == "" || string(c.Status) == f.Status) {
			out = append(out, c)
		}
	}
	return out, nil
}

func (r *stubRepo) CountCycles(ctx context.Context, orgID string, f feedback.CycleListFilter) (int, error) {
	out, err := r.FindCycles(ctx, orgID, f)
	return len(out), err
}

func (r *stubRepo) FindCycleByRef(_ context.Context, orgID, ref string) (*feedback.Cycle, error) {
	for _, c := range r.cycles {
		if c.OrgID == orgID && matchRef(c.ID, c.PublicID, ref) {
			return c, nil
		}
	}
	return nil, nil
}

func (r *stubRepo) CycleNameExists(_ context.Context, orgID, name, excludeID string) (bool, error) {
	for _, c := range r.cycles {
		if c.OrgID == orgID && strings.EqualFold(c.Name, name) && c.ID != excludeID {
			return true, nil
		}
	}
	return false, nil
}

func (r *stubRepo) CreateCycle(_ context.Context, c *feedback.Cycle) error {
	c.ID = r.nextID("fbc")
	c.PublicID = "fbc_pub_" + c.ID
	c.Status = feedback.CycleDraft
	r.cycles[c.ID] = c
	return nil
}

func (r *stubRepo) UpdateCycle(_ context.Context, c *feedback.Cycle) error {
	if _, ok := r.cycles[c.ID]; !ok {
		return feedback.ErrCycleNotFound
	}
	r.cycles[c.ID] = c
	return nil
}

func (r *stubRepo) SetCycleStatus(_ context.Context, orgID, id string, status feedback.CycleStatus) (*feedback.Cycle, error) {
	c, ok := r.cycles[id]
	if !ok || c.OrgID != orgID {
		return nil, feedback.ErrCycleNotFound
	}
	c.Status = status
	return c, nil
}

func (r *stubRepo) FindRequestSummaries(_ context.Context, orgID string, f feedback.RequestListFilter) ([]*feedback.RequestSummary, error) {
	out := make([]*feedback.RequestSummary, 0)
	for _, q := range r.requests {
		if q.OrgID != orgID {
			continue
		}
		if f.CycleID != "" && q.CycleID != f.CycleID {
			continue
		}
		if f.SubjectEmployeeID != "" && q.SubjectEmployeeID != f.SubjectEmployeeID {
			continue
		}
		if f.Status != "" && string(q.Status) != f.Status {
			continue
		}
		out = append(out, &feedback.RequestSummary{
			ID: q.ID, PublicID: q.PublicID, RespondentName: q.RespondentName,
			Relationship: q.Relationship, Status: q.Status, SubmittedAt: q.SubmittedAt,
		})
	}
	return out, nil
}

func (r *stubRepo) CountRequests(ctx context.Context, orgID string, f feedback.RequestListFilter) (int, error) {
	out, err := r.FindRequestSummaries(ctx, orgID, f)
	return len(out), err
}

func (r *stubRepo) FindRequestByRef(_ context.Context, orgID, ref string) (*feedback.Request, error) {
	for _, q := range r.requests {
		if q.OrgID == orgID && matchRef(q.ID, q.PublicID, ref) {
			return q, nil
		}
	}
	return nil, nil
}

func (r *stubRepo) CreateRequests(_ context.Context, reqs []*feedback.Request) error {
	for _, q := range reqs {
		q.ID = r.nextID("fbr")
		q.PublicID = "fbr_pub_" + q.ID
		q.Status = feedback.RequestPending
		r.requests[q.ID] = q
	}
	return nil
}

func (r *stubRepo) SetRequestSubmitted(_ context.Context, orgID, id string) (*feedback.Request, error) {
	q, ok := r.requests[id]
	if !ok || q.OrgID != orgID {
		return nil, feedback.ErrRequestNotFound
	}
	q.Status = feedback.RequestSubmitted
	return q, nil
}

func (r *stubRepo) SetRequestDeclined(_ context.Context, orgID, id string, reason *string) (*feedback.Request, error) {
	q, ok := r.requests[id]
	if !ok || q.OrgID != orgID {
		return nil, feedback.ErrRequestNotFound
	}
	q.Status = feedback.RequestDeclined
	q.DeclineReason = reason
	return q, nil
}

func (r *stubRepo) RequestExists(_ context.Context, cycleID, subjectEmployeeID string, respondentEmployeeID, email *string) (bool, error) {
	for _, q := range r.requests {
		if q.CycleID != cycleID || q.SubjectEmployeeID != subjectEmployeeID {
			continue
		}
		if respondentEmployeeID != nil && q.RespondentEmployeeID != nil &&
			*q.RespondentEmployeeID == *respondentEmployeeID {
			return true, nil
		}
		if respondentEmployeeID == nil && q.RespondentEmployeeID == nil &&
			email != nil && q.RespondentEmail != nil &&
			strings.EqualFold(*q.RespondentEmail, *email) {
			return true, nil
		}
	}
	return false, nil
}

// FindSubmittedForSubject mirrors the real query's shape: relationship and
// form instance only, never identity.
func (r *stubRepo) FindSubmittedForSubject(_ context.Context, orgID, cycleID, subjectEmployeeID string) ([]*feedback.SubmittedRef, error) {
	out := make([]*feedback.SubmittedRef, 0)
	for _, q := range r.requests {
		if q.OrgID == orgID && q.CycleID == cycleID &&
			q.SubjectEmployeeID == subjectEmployeeID && q.Status == feedback.RequestSubmitted {
			out = append(out, &feedback.SubmittedRef{
				Relationship: q.Relationship, FormInstanceID: q.FormInstanceID,
			})
		}
	}
	return out, nil
}

func (r *stubRepo) FindRequestsForRespondent(_ context.Context, orgID, userID string) ([]*feedback.MyRequest, error) {
	out := make([]*feedback.MyRequest, 0)
	for _, q := range r.requests {
		if q.OrgID == orgID && q.RespondentUserID != nil && *q.RespondentUserID == userID {
			out = append(out, &feedback.MyRequest{
				ID: q.ID, PublicID: q.PublicID, CycleID: q.CycleID,
				SubjectEmployeeID: q.SubjectEmployeeID, Relationship: q.Relationship,
				Status: q.Status, FormInstanceID: q.FormInstanceID,
			})
		}
	}
	return out, nil
}

func (r *stubRepo) FindEmployeeSubject(_ context.Context, orgID, employeeRef string) (*feedback.EmployeeSubject, error) {
	if e, ok := r.employees[employeeRef]; ok {
		return e, nil
	}
	return nil, nil
}

func (r *stubRepo) FindEmployeeIDByUserID(_ context.Context, orgID, userID string) (string, error) {
	return r.employeeUsers[userID], nil
}

// ── Authorizer stub ──────────────────────────────────────────────────────────

type stubAuthorizer struct{ allow bool }

func (a *stubAuthorizer) AuthorizeRecordAccess(_ context.Context, tier authz.Scope, _, _, _ string) (bool, error) {
	if tier == authz.ScopeAll {
		return true, nil
	}
	return a.allow, nil
}

// ── Form engine stub ─────────────────────────────────────────────────────────

type stubForms struct {
	seq       int
	instances map[string]*forms.InstanceWithResponses
	scores    map[string]decimal.Decimal
	// instantiated records every SubjectContext passed in, so a test can
	// assert the subject/respondent split landed the right way round.
	instantiated []forms.SubjectContext
}

func newStubForms() *stubForms {
	return &stubForms{
		instances: map[string]*forms.InstanceWithResponses{},
		scores:    map[string]decimal.Decimal{},
	}
}

func (f *stubForms) Instantiate(_ context.Context, orgID, templateRef string, subj forms.SubjectContext) (*forms.InstanceWithResponses, error) {
	f.seq++
	id := fmt.Sprintf("fins_%d", f.seq)
	f.instantiated = append(f.instantiated, subj)
	inst := &forms.InstanceWithResponses{
		Instance: &forms.Instance{
			ID: id, OrgID: orgID, SubjectID: subj.SubjectID,
			RespondentUserID: subj.RespondentUserID,
		},
		Responses: []*forms.Response{{
			ID: id + "_r1", QuestionText: "Overall", QuestionType: forms.QuestionTextarea,
			AnswerText: strPtr("Solid contributor"),
		}},
	}
	f.instances[id] = inst
	f.scores[id] = decimal.NewFromInt(80)
	return inst, nil
}

func (f *stubForms) GetInstance(_ context.Context, _, ref string) (*forms.InstanceWithResponses, error) {
	inst, ok := f.instances[ref]
	if !ok {
		return nil, fmt.Errorf("no such instance %s", ref)
	}
	return inst, nil
}

func (f *stubForms) ScoreInstance(_ context.Context, _, ref string) (forms.Score, error) {
	return forms.Score{Percent: f.scores[ref], ScoredCount: 1, TotalWeight: decimal.NewFromInt(100)}, nil
}

func strPtr(s string) *string { return &s }
