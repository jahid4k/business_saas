// backend/internal/tests/unit/hrm/performance/appraisals_stub_test.go
// Stub implementations for the Phase 5B halves of the composite Repository
// (rating scales and appraisals), plus a stub FormEngine. Split from
// service_test.go purely for file size — they extend the same *stubRepo.
package performance_test

import (
	"context"
	"time"

	"github.com/shopspring/decimal"

	"github.com/mridha/businesssaas/internal/hrm/performance"
	"github.com/mridha/businesssaas/internal/platform/forms"
)

// ── Rating scales ────────────────────────────────────────────────────────────

func (r *stubRepo) FindScales(_ context.Context, orgID string) ([]*performance.RatingScale, error) {
	out := make([]*performance.RatingScale, 0)
	for _, s := range r.scales {
		if s.OrgID == orgID {
			out = append(out, s)
		}
	}
	return out, nil
}

func (r *stubRepo) FindScaleByRef(_ context.Context, orgID, ref string) (*performance.RatingScale, error) {
	for _, s := range r.scales {
		if s.OrgID == orgID && matchRef(s.ID, s.PublicID, ref) {
			return s, nil
		}
	}
	return nil, nil
}

func (r *stubRepo) FindDefaultScale(_ context.Context, orgID string) (*performance.RatingScale, error) {
	for _, s := range r.scales {
		if s.OrgID == orgID && s.IsDefault && s.IsActive {
			return s, nil
		}
	}
	return nil, nil
}

func (r *stubRepo) CreateScale(_ context.Context, s *performance.RatingScale) error {
	if s.IsDefault {
		for _, e := range r.scales {
			if e.OrgID == s.OrgID {
				e.IsDefault = false
			}
		}
	}
	s.ID = r.nextID("rscl")
	s.PublicID = "pub_" + s.ID
	s.IsActive = true
	s.CreatedAt, s.UpdatedAt = time.Now(), time.Now()
	r.scales[s.ID] = s
	return nil
}

func (r *stubRepo) UpdateScale(_ context.Context, s *performance.RatingScale) error {
	if _, ok := r.scales[s.ID]; !ok {
		return performance.ErrScaleNotFound
	}
	r.scales[s.ID] = s
	return nil
}

func (r *stubRepo) SetScaleDefault(_ context.Context, orgID, scaleID string) error {
	target, ok := r.scales[scaleID]
	if !ok {
		return performance.ErrScaleNotFound
	}
	for _, e := range r.scales {
		if e.OrgID == orgID {
			e.IsDefault = false
		}
	}
	target.IsDefault = true
	return nil
}

func (r *stubRepo) DeleteScale(_ context.Context, _, scaleID string) error {
	if _, ok := r.scales[scaleID]; !ok {
		return performance.ErrScaleNotFound
	}
	delete(r.scales, scaleID)
	return nil
}

func (r *stubRepo) ScaleNameExists(_ context.Context, orgID, name, excludeID string) (bool, error) {
	for _, s := range r.scales {
		if s.OrgID == orgID && s.Name == name && s.ID != excludeID {
			return true, nil
		}
	}
	return false, nil
}

func (r *stubRepo) CountCyclesUsingScale(_ context.Context, scaleID string) (int, error) {
	n := 0
	for _, c := range r.appraisalCycles {
		if c.RatingScaleID == scaleID {
			n++
		}
	}
	return n, nil
}

func (r *stubRepo) FindLevels(_ context.Context, _, scaleID string) ([]*performance.RatingLevel, error) {
	out := make([]*performance.RatingLevel, 0)
	for _, l := range r.levels {
		if l.ScaleID == scaleID {
			out = append(out, l)
		}
	}
	for i := 0; i < len(out); i++ {
		for j := i + 1; j < len(out); j++ {
			if out[j].DisplayOrder < out[i].DisplayOrder {
				out[i], out[j] = out[j], out[i]
			}
		}
	}
	return out, nil
}

func (r *stubRepo) FindLevelByRef(_ context.Context, _, ref string) (*performance.RatingLevel, error) {
	for _, l := range r.levels {
		if matchRef(l.ID, l.PublicID, ref) {
			return l, nil
		}
	}
	return nil, nil
}

func (r *stubRepo) CreateLevel(_ context.Context, l *performance.RatingLevel) error {
	l.ID = r.nextID("rlvl")
	l.PublicID = "pub_" + l.ID
	l.CreatedAt, l.UpdatedAt = time.Now(), time.Now()
	r.levels[l.ID] = l
	return nil
}

func (r *stubRepo) UpdateLevel(_ context.Context, _ string, l *performance.RatingLevel) error {
	if _, ok := r.levels[l.ID]; !ok {
		return performance.ErrLevelNotFound
	}
	r.levels[l.ID] = l
	return nil
}

func (r *stubRepo) DeleteLevel(_ context.Context, _, levelID string) error {
	if _, ok := r.levels[levelID]; !ok {
		return performance.ErrLevelNotFound
	}
	delete(r.levels, levelID)
	return nil
}

func (r *stubRepo) LevelLabelExists(_ context.Context, scaleID, label, excludeID string) (bool, error) {
	for _, l := range r.levels {
		if l.ScaleID == scaleID && l.Label == label && l.ID != excludeID {
			return true, nil
		}
	}
	return false, nil
}

// ── Appraisal cycles ─────────────────────────────────────────────────────────

func (r *stubRepo) FindAppraisalCycles(_ context.Context, orgID string, _ performance.AppraisalCycleListFilter) ([]*performance.AppraisalCycle, error) {
	out := make([]*performance.AppraisalCycle, 0)
	for _, c := range r.appraisalCycles {
		if c.OrgID == orgID {
			out = append(out, c)
		}
	}
	return out, nil
}

func (r *stubRepo) CountAppraisalCycles(ctx context.Context, orgID string, f performance.AppraisalCycleListFilter) (int, error) {
	out, _ := r.FindAppraisalCycles(ctx, orgID, f)
	return len(out), nil
}

func (r *stubRepo) FindAppraisalCycleByRef(_ context.Context, orgID, ref string) (*performance.AppraisalCycle, error) {
	for _, c := range r.appraisalCycles {
		if c.OrgID == orgID && matchRef(c.ID, c.PublicID, ref) {
			return c, nil
		}
	}
	return nil, nil
}

func (r *stubRepo) CreateAppraisalCycle(_ context.Context, c *performance.AppraisalCycle) error {
	c.ID = r.nextID("acyc")
	c.PublicID = "pub_" + c.ID
	c.Status = performance.AppraisalCycleDraft
	c.CreatedAt, c.UpdatedAt = time.Now(), time.Now()
	r.appraisalCycles[c.ID] = c
	return nil
}

func (r *stubRepo) UpdateAppraisalCycle(_ context.Context, c *performance.AppraisalCycle) error {
	if _, ok := r.appraisalCycles[c.ID]; !ok {
		return performance.ErrAppraisalCycleNotFound
	}
	r.appraisalCycles[c.ID] = c
	return nil
}

func (r *stubRepo) SetAppraisalCycleStatus(_ context.Context, _, id string, status performance.AppraisalCycleStatus) error {
	c, ok := r.appraisalCycles[id]
	if !ok {
		return performance.ErrAppraisalCycleNotFound
	}
	c.Status = status
	return nil
}

func (r *stubRepo) AppraisalCycleNameExists(_ context.Context, orgID, name, excludeID string) (bool, error) {
	for _, c := range r.appraisalCycles {
		if c.OrgID == orgID && c.Name == name && c.ID != excludeID {
			return true, nil
		}
	}
	return false, nil
}

// ── Appraisals ───────────────────────────────────────────────────────────────

func (r *stubRepo) FindAppraisals(_ context.Context, orgID string, f performance.AppraisalListFilter) ([]*performance.Appraisal, error) {
	out := make([]*performance.Appraisal, 0)
	for _, a := range r.appraisals {
		if a.OrgID != orgID {
			continue
		}
		if f.CycleID != "" && a.CycleID != f.CycleID {
			continue
		}
		if f.EmployeeID != "" && a.EmployeeID != f.EmployeeID {
			continue
		}
		out = append(out, a)
	}
	return out, nil
}

func (r *stubRepo) CountAppraisals(ctx context.Context, orgID string, f performance.AppraisalListFilter) (int, error) {
	out, _ := r.FindAppraisals(ctx, orgID, f)
	return len(out), nil
}

func (r *stubRepo) FindAppraisalByRef(_ context.Context, orgID, ref string) (*performance.Appraisal, error) {
	for _, a := range r.appraisals {
		if a.OrgID == orgID && matchRef(a.ID, a.PublicID, ref) {
			return a, nil
		}
	}
	return nil, nil
}

func (r *stubRepo) FindAppraisalForEmployee(_ context.Context, orgID, cycleID, employeeID string) (*performance.Appraisal, error) {
	for _, a := range r.appraisals {
		if a.OrgID == orgID && a.CycleID == cycleID && a.EmployeeID == employeeID {
			return a, nil
		}
	}
	return nil, nil
}

func (r *stubRepo) CreateAppraisal(_ context.Context, a *performance.Appraisal) error {
	a.ID = r.nextID("appr")
	a.PublicID = "pub_" + a.ID
	a.Phase = performance.PhaseDraft
	a.CreatedAt, a.UpdatedAt = time.Now(), time.Now()
	r.appraisals[a.ID] = a
	return nil
}

func (r *stubRepo) AdvanceAppraisalPhase(_ context.Context, _ string, a *performance.Appraisal, h *performance.PhaseHistory) error {
	if _, ok := r.appraisals[a.ID]; !ok {
		return performance.ErrAppraisalNotFound
	}
	r.appraisals[a.ID] = a
	r.appendHistory(h)
	return nil
}

func (r *stubRepo) SetAppraisalRating(_ context.Context, _ string, a *performance.Appraisal, h *performance.PhaseHistory) error {
	if _, ok := r.appraisals[a.ID]; !ok {
		return performance.ErrAppraisalNotFound
	}
	r.appraisals[a.ID] = a
	r.appendHistory(h)
	return nil
}

func (r *stubRepo) PublishAppraisal(_ context.Context, _ string, a *performance.Appraisal, h *performance.PhaseHistory) error {
	if _, ok := r.appraisals[a.ID]; !ok {
		return performance.ErrAppraisalNotFound
	}
	a.Phase = performance.PhasePublished
	now := time.Now()
	a.PublishedAt = &now
	r.appraisals[a.ID] = a
	r.appendHistory(h)
	return nil
}

func (r *stubRepo) appendHistory(h *performance.PhaseHistory) {
	h.ID = r.nextID("aphs")
	h.PublicID = "pub_" + h.ID
	h.ChangedAt = time.Now()
	r.phaseHistory[h.AppraisalID] = append(r.phaseHistory[h.AppraisalID], h)
}

func (r *stubRepo) FindPhaseHistory(_ context.Context, appraisalID string) ([]*performance.PhaseHistory, error) {
	return r.phaseHistory[appraisalID], nil
}

func (r *stubRepo) FindEmployeeSubject(_ context.Context, _, employeeRef string) (*performance.EmployeeSubject, error) {
	s, ok := r.employeeSubjects[employeeRef]
	if !ok {
		return nil, nil
	}
	return s, nil
}

// ── Stub FormEngine ──────────────────────────────────────────────────────────

// stubFormEngine records what was instantiated and lets a test control
// whether each instance reads as submitted, which is what the phase
// preconditions depend on.
type stubFormEngine struct {
	seq          int
	instances    map[string]*forms.InstanceWithResponses
	submitted    map[string]bool
	scores       map[string]decimal.Decimal
	instantiated []forms.SubjectContext
}

func newStubFormEngine() *stubFormEngine {
	return &stubFormEngine{
		instances: map[string]*forms.InstanceWithResponses{},
		submitted: map[string]bool{},
		scores:    map[string]decimal.Decimal{},
	}
}

func (e *stubFormEngine) Instantiate(_ context.Context, orgID, templateRef string, subj forms.SubjectContext) (*forms.InstanceWithResponses, error) {
	e.seq++
	id := "fins_stub_" + itoa(e.seq)
	inst := &forms.InstanceWithResponses{
		Instance: &forms.Instance{
			ID: id, PublicID: "pub_" + id, OrgID: orgID,
			TemplateName: "stub", SubjectType: subj.SubjectType, SubjectID: subj.SubjectID,
			SubjectLabel: subj.SubjectLabel, RespondentUserID: subj.RespondentUserID,
			Status: forms.InstanceDraft,
		},
		Responses: []*forms.Response{},
	}
	e.instances[id] = inst
	e.instantiated = append(e.instantiated, subj)
	return inst, nil
}

func (e *stubFormEngine) GetInstance(_ context.Context, _, ref string) (*forms.InstanceWithResponses, error) {
	inst, ok := e.instances[ref]
	if !ok {
		return nil, forms.ErrInstanceNotFound
	}
	if e.submitted[ref] && inst.SubmittedAt == nil {
		now := time.Now()
		inst.SubmittedAt = &now
		inst.Status = forms.InstanceSubmitted
	}
	return inst, nil
}

func (e *stubFormEngine) ScoreInstance(_ context.Context, _, ref string) (forms.Score, error) {
	if _, ok := e.instances[ref]; !ok {
		return forms.Score{}, forms.ErrInstanceNotFound
	}
	return forms.Score{Percent: e.scores[ref], ScoredCount: 1}, nil
}

// markSubmitted makes a form instance read as submitted, so the phase
// preconditions can be exercised.
func (e *stubFormEngine) markSubmitted(ref string, score string) {
	e.submitted[ref] = true
	if score != "" {
		e.scores[ref] = dec(score)
	}
}

var _ performance.FormEngine = (*stubFormEngine)(nil)

// strPtr is shared by the Phase 5B test files.
func strPtr(s string) *string { return &s }

func itoa(i int) string {
	if i == 0 {
		return "0"
	}
	var b []byte
	for i > 0 {
		b = append([]byte{byte('0' + i%10)}, b...)
		i /= 10
	}
	return string(b)
}
