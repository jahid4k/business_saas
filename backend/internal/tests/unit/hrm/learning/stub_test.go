// backend/internal/tests/unit/hrm/learning/stub_test.go
// In-memory stubs for the Phase 6A learning service.
package learning_test

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/shopspring/decimal"

	"github.com/mridha/businesssaas/internal/authz"
	"github.com/mridha/businesssaas/internal/hrm/learning"
	"github.com/mridha/businesssaas/internal/platform/forms"
)

const (
	testOrg     = "org_1"
	ownerUserID = "usr_owner"
	learnerUser = "usr_learner"
	learnerEmp  = "emp_learner"
	otherEmp    = "emp_other"
	templateID  = "tmpl_quiz"
)

// ── Repository stub ──────────────────────────────────────────────────────────

type stubRepo struct {
	seq int

	courses   map[string]*learning.Course
	versions  map[string]*learning.CourseVersion
	modules   map[string]*learning.Module
	lessons   map[string]*learning.Lesson
	enrolls   map[string]*learning.Enrollment
	progress  map[string][]*learning.LessonProgress
	attempts  map[string][]*learning.QuizAttempt
	keys      map[string]*learning.AnswerKey // by question id
	employees map[string]*learning.EmployeeRef
	empUsers  map[string]string

	// templateQuestions maps a template id to its question ids, standing in
	// for the sections → questions join the real query walks.
	templateQuestions map[string][]string
}

var _ learning.Repository = (*stubRepo)(nil)

func newStubRepo() *stubRepo {
	return &stubRepo{
		courses:           map[string]*learning.Course{},
		versions:          map[string]*learning.CourseVersion{},
		modules:           map[string]*learning.Module{},
		lessons:           map[string]*learning.Lesson{},
		enrolls:           map[string]*learning.Enrollment{},
		progress:          map[string][]*learning.LessonProgress{},
		attempts:          map[string][]*learning.QuizAttempt{},
		keys:              map[string]*learning.AnswerKey{},
		employees:         map[string]*learning.EmployeeRef{},
		empUsers:          map[string]string{},
		templateQuestions: map[string][]string{},
	}
}

func (r *stubRepo) nextID(prefix string) string {
	r.seq++
	return fmt.Sprintf("%s_%d", prefix, r.seq)
}

func matchRef(id, publicID, ref string) bool { return id == ref || publicID == ref }

// ── Courses ──────────────────────────────────────────────────────────────────

func (r *stubRepo) FindCourses(_ context.Context, orgID string, f learning.CourseListFilter) ([]*learning.Course, error) {
	out := make([]*learning.Course, 0)
	for _, c := range r.courses {
		if c.OrgID != orgID {
			continue
		}
		if f.IsActive != nil && c.IsActive != *f.IsActive {
			continue
		}
		out = append(out, c)
	}
	return out, nil
}

func (r *stubRepo) CountCourses(ctx context.Context, orgID string, f learning.CourseListFilter) (int, error) {
	out, err := r.FindCourses(ctx, orgID, f)
	return len(out), err
}

func (r *stubRepo) FindCourseByRef(_ context.Context, orgID, ref string) (*learning.Course, error) {
	for _, c := range r.courses {
		if c.OrgID == orgID && matchRef(c.ID, c.PublicID, ref) {
			return c, nil
		}
	}
	return nil, nil
}

func (r *stubRepo) CourseTitleExists(_ context.Context, orgID, title, excludeID string) (bool, error) {
	for _, c := range r.courses {
		if c.OrgID == orgID && strings.EqualFold(c.Title, title) && c.ID != excludeID {
			return true, nil
		}
	}
	return false, nil
}

func (r *stubRepo) CreateCourse(_ context.Context, c *learning.Course) error {
	c.ID = r.nextID("crs")
	c.PublicID = "crs_pub_" + c.ID
	c.IsActive = true
	r.courses[c.ID] = c
	return nil
}

func (r *stubRepo) UpdateCourse(_ context.Context, c *learning.Course) error {
	if _, ok := r.courses[c.ID]; !ok {
		return learning.ErrCourseNotFound
	}
	r.courses[c.ID] = c
	return nil
}

func (r *stubRepo) DeleteCourse(_ context.Context, orgID, id string) error {
	if _, ok := r.courses[id]; !ok {
		return learning.ErrCourseNotFound
	}
	delete(r.courses, id)
	return nil
}

// ── Versions ─────────────────────────────────────────────────────────────────

func (r *stubRepo) FindVersions(_ context.Context, orgID, courseID string) ([]*learning.CourseVersion, error) {
	out := make([]*learning.CourseVersion, 0)
	for _, v := range r.versions {
		if v.OrgID == orgID && v.CourseID == courseID {
			out = append(out, v)
		}
	}
	return out, nil
}

func (r *stubRepo) FindVersionByRef(_ context.Context, orgID, ref string) (*learning.CourseVersion, error) {
	for _, v := range r.versions {
		if v.OrgID == orgID && matchRef(v.ID, v.PublicID, ref) {
			return v, nil
		}
	}
	return nil, nil
}

func (r *stubRepo) FindPublishedVersion(_ context.Context, orgID, courseID string) (*learning.CourseVersion, error) {
	var best *learning.CourseVersion
	for _, v := range r.versions {
		if v.OrgID == orgID && v.CourseID == courseID && v.Status == learning.VersionPublished {
			if best == nil || v.VersionNumber > best.VersionNumber {
				best = v
			}
		}
	}
	return best, nil
}

func (r *stubRepo) NextVersionNumber(_ context.Context, courseID string) (int, error) {
	max := 0
	for _, v := range r.versions {
		if v.CourseID == courseID && v.VersionNumber > max {
			max = v.VersionNumber
		}
	}
	return max + 1, nil
}

func (r *stubRepo) CreateVersion(_ context.Context, v *learning.CourseVersion) error {
	v.ID = r.nextID("crsv")
	v.PublicID = "crsv_pub_" + v.ID
	v.Status = learning.VersionDraft
	r.versions[v.ID] = v
	return nil
}

func (r *stubRepo) UpdateVersion(_ context.Context, v *learning.CourseVersion) error {
	if _, ok := r.versions[v.ID]; !ok {
		return learning.ErrVersionNotFound
	}
	r.versions[v.ID] = v
	return nil
}

func (r *stubRepo) SetVersionStatus(_ context.Context, orgID, id string, status learning.VersionStatus, actorID string) (*learning.CourseVersion, error) {
	v, ok := r.versions[id]
	if !ok || v.OrgID != orgID {
		return nil, learning.ErrVersionNotFound
	}
	v.Status = status
	now := time.Now()
	switch status {
	case learning.VersionPublished:
		v.PublishedAt = &now
		if actorID != "" {
			v.PublishedBy = &actorID
		}
	case learning.VersionArchived:
		v.ArchivedAt = &now
	}
	return v, nil
}

func (r *stubRepo) DeleteVersion(_ context.Context, orgID, id string) error {
	if _, ok := r.versions[id]; !ok {
		return learning.ErrVersionNotFound
	}
	delete(r.versions, id)
	return nil
}

func (r *stubRepo) VersionHasEnrollments(_ context.Context, versionID string) (bool, error) {
	for _, e := range r.enrolls {
		if e.VersionID == versionID {
			return true, nil
		}
	}
	return false, nil
}

func (r *stubRepo) CopyVersionContent(ctx context.Context, fromVersionID, toVersionID string) error {
	mods, _ := r.FindModules(ctx, fromVersionID)
	for _, m := range mods {
		newM := &learning.Module{
			VersionID: toVersionID, Title: m.Title, Description: m.Description,
			DisplayOrder: m.DisplayOrder,
		}
		if err := r.CreateModule(ctx, newM); err != nil {
			return err
		}
		for _, l := range r.lessons {
			if l.ModuleID != m.ID {
				continue
			}
			copied := *l
			copied.ModuleID = newM.ID
			if err := r.CreateLesson(ctx, &copied); err != nil {
				return err
			}
		}
	}
	return nil
}

// ── Modules ──────────────────────────────────────────────────────────────────

func (r *stubRepo) FindModules(_ context.Context, versionID string) ([]*learning.Module, error) {
	out := make([]*learning.Module, 0)
	for _, m := range r.modules {
		if m.VersionID == versionID {
			out = append(out, m)
		}
	}
	return out, nil
}

func (r *stubRepo) FindModuleByRef(_ context.Context, orgID, ref string) (*learning.Module, error) {
	for _, m := range r.modules {
		if matchRef(m.ID, m.PublicID, ref) {
			if v, ok := r.versions[m.VersionID]; ok && v.OrgID == orgID {
				return m, nil
			}
		}
	}
	return nil, nil
}

func (r *stubRepo) CreateModule(_ context.Context, m *learning.Module) error {
	m.ID = r.nextID("crsm")
	m.PublicID = "crsm_pub_" + m.ID
	r.modules[m.ID] = m
	return nil
}

func (r *stubRepo) UpdateModule(_ context.Context, m *learning.Module) error {
	if _, ok := r.modules[m.ID]; !ok {
		return learning.ErrModuleNotFound
	}
	r.modules[m.ID] = m
	return nil
}

func (r *stubRepo) DeleteModule(_ context.Context, id string) error {
	if _, ok := r.modules[id]; !ok {
		return learning.ErrModuleNotFound
	}
	delete(r.modules, id)
	return nil
}

// ── Lessons ──────────────────────────────────────────────────────────────────

func (r *stubRepo) FindLessons(ctx context.Context, versionID string) ([]*learning.Lesson, error) {
	mods, _ := r.FindModules(ctx, versionID)
	ids := map[string]bool{}
	for _, m := range mods {
		ids[m.ID] = true
	}
	out := make([]*learning.Lesson, 0)
	for _, l := range r.lessons {
		if ids[l.ModuleID] {
			out = append(out, l)
		}
	}
	return out, nil
}

func (r *stubRepo) FindLessonByRef(_ context.Context, orgID, ref string) (*learning.Lesson, error) {
	for _, l := range r.lessons {
		if matchRef(l.ID, l.PublicID, ref) {
			if m, ok := r.modules[l.ModuleID]; ok {
				if v, ok := r.versions[m.VersionID]; ok && v.OrgID == orgID {
					return l, nil
				}
			}
		}
	}
	return nil, nil
}

func (r *stubRepo) CreateLesson(_ context.Context, l *learning.Lesson) error {
	l.ID = r.nextID("crsl")
	l.PublicID = "crsl_pub_" + l.ID
	r.lessons[l.ID] = l
	return nil
}

func (r *stubRepo) UpdateLesson(_ context.Context, l *learning.Lesson) error {
	if _, ok := r.lessons[l.ID]; !ok {
		return learning.ErrLessonNotFound
	}
	r.lessons[l.ID] = l
	return nil
}

func (r *stubRepo) DeleteLesson(_ context.Context, id string) error {
	if _, ok := r.lessons[id]; !ok {
		return learning.ErrLessonNotFound
	}
	delete(r.lessons, id)
	return nil
}

func (r *stubRepo) CountRequiredLessons(ctx context.Context, versionID string) (int, error) {
	lessons, _ := r.FindLessons(ctx, versionID)
	n := 0
	for _, l := range lessons {
		if l.IsRequired {
			n++
		}
	}
	return n, nil
}

// ── Enrollments ──────────────────────────────────────────────────────────────

func (r *stubRepo) FindEnrollments(_ context.Context, orgID string, f learning.EnrollmentListFilter) ([]*learning.Enrollment, error) {
	out := make([]*learning.Enrollment, 0)
	for _, e := range r.enrolls {
		if e.OrgID != orgID {
			continue
		}
		if f.EmployeeID != "" && e.EmployeeID != f.EmployeeID {
			continue
		}
		if f.CourseID != "" && e.CourseID != f.CourseID {
			continue
		}
		if f.Status != "" && string(e.Status) != f.Status {
			continue
		}
		out = append(out, e)
	}
	return out, nil
}

func (r *stubRepo) CountEnrollments(ctx context.Context, orgID string, f learning.EnrollmentListFilter) (int, error) {
	out, err := r.FindEnrollments(ctx, orgID, f)
	return len(out), err
}

func (r *stubRepo) FindEnrollmentByRef(_ context.Context, orgID, ref string) (*learning.Enrollment, error) {
	for _, e := range r.enrolls {
		if e.OrgID == orgID && matchRef(e.ID, e.PublicID, ref) {
			return e, nil
		}
	}
	return nil, nil
}

func (r *stubRepo) HasLiveEnrollment(_ context.Context, orgID, employeeID, courseID string) (bool, error) {
	for _, e := range r.enrolls {
		if e.OrgID == orgID && e.EmployeeID == employeeID && e.CourseID == courseID && e.Status.IsLive() {
			return true, nil
		}
	}
	return false, nil
}

func (r *stubRepo) CreateEnrollment(_ context.Context, e *learning.Enrollment) error {
	e.ID = r.nextID("enr")
	e.PublicID = "enr_pub_" + e.ID
	e.Status = learning.EnrollmentAssigned
	r.enrolls[e.ID] = e
	return nil
}

func (r *stubRepo) UpdateEnrollment(_ context.Context, e *learning.Enrollment) error {
	if _, ok := r.enrolls[e.ID]; !ok {
		return learning.ErrEnrollmentNotFound
	}
	r.enrolls[e.ID] = e
	return nil
}

func (r *stubRepo) SetEnrollmentStatus(_ context.Context, orgID, id string, status learning.EnrollmentStatus) (*learning.Enrollment, error) {
	e, ok := r.enrolls[id]
	if !ok || e.OrgID != orgID {
		return nil, learning.ErrEnrollmentNotFound
	}
	e.Status = status
	return e, nil
}

func (r *stubRepo) MarkEnrollmentStarted(_ context.Context, orgID, id string) error {
	if e, ok := r.enrolls[id]; ok && e.Status == learning.EnrollmentAssigned {
		e.Status = learning.EnrollmentInProgress
		now := time.Now()
		e.StartedAt = &now
	}
	return nil
}

func (r *stubRepo) MarkEnrollmentCompleted(_ context.Context, orgID, id string) error {
	if e, ok := r.enrolls[id]; ok && e.Status.IsLive() {
		e.Status = learning.EnrollmentCompleted
		now := time.Now()
		e.CompletedAt = &now
	}
	return nil
}

// ── Progress ─────────────────────────────────────────────────────────────────

func (r *stubRepo) FindProgress(_ context.Context, enrollmentID string) ([]*learning.LessonProgress, error) {
	return r.progress[enrollmentID], nil
}

func (r *stubRepo) UpsertProgress(_ context.Context, enrollmentID, lessonID string, status learning.ProgressStatus) (*learning.LessonProgress, error) {
	for _, p := range r.progress[enrollmentID] {
		if p.LessonID == lessonID {
			p.Status = status
			if status == learning.ProgressCompleted && p.CompletedAt == nil {
				now := time.Now()
				p.CompletedAt = &now
			}
			if status != learning.ProgressCompleted {
				p.CompletedAt = nil
			}
			return p, nil
		}
	}
	p := &learning.LessonProgress{
		ID: r.nextID("lprg"), EnrollmentID: enrollmentID, LessonID: lessonID, Status: status,
	}
	if status == learning.ProgressCompleted {
		now := time.Now()
		p.CompletedAt = &now
	}
	r.progress[enrollmentID] = append(r.progress[enrollmentID], p)
	return p, nil
}

func (r *stubRepo) CountCompletedRequired(_ context.Context, enrollmentID string) (int, error) {
	n := 0
	for _, p := range r.progress[enrollmentID] {
		if p.Status != learning.ProgressCompleted {
			continue
		}
		if l, ok := r.lessons[p.LessonID]; ok && l.IsRequired {
			n++
		}
	}
	return n, nil
}

// ── Attempts ─────────────────────────────────────────────────────────────────

func (r *stubRepo) FindAttempts(_ context.Context, enrollmentID string) ([]*learning.QuizAttempt, error) {
	return r.attempts[enrollmentID], nil
}

func (r *stubRepo) FindAttemptByRef(_ context.Context, orgID, ref string) (*learning.QuizAttempt, error) {
	for _, list := range r.attempts {
		for _, a := range list {
			if a.OrgID == orgID && matchRef(a.ID, a.PublicID, ref) {
				return a, nil
			}
		}
	}
	return nil, nil
}

func (r *stubRepo) FindOpenAttempt(_ context.Context, enrollmentID, lessonID string) (*learning.QuizAttempt, error) {
	for _, a := range r.attempts[enrollmentID] {
		if a.LessonID == lessonID && a.GradedAt == nil {
			return a, nil
		}
	}
	return nil, nil
}

func (r *stubRepo) CountAttempts(_ context.Context, enrollmentID, lessonID string) (int, error) {
	n := 0
	for _, a := range r.attempts[enrollmentID] {
		if a.LessonID == lessonID {
			n++
		}
	}
	return n, nil
}

func (r *stubRepo) HasPassedAttempt(_ context.Context, enrollmentID, lessonID string) (bool, error) {
	for _, a := range r.attempts[enrollmentID] {
		if a.LessonID == lessonID && a.Passed != nil && *a.Passed {
			return true, nil
		}
	}
	return false, nil
}

func (r *stubRepo) CreateAttempt(_ context.Context, a *learning.QuizAttempt) error {
	a.ID = r.nextID("qatt")
	a.PublicID = "qatt_pub_" + a.ID
	a.StartedAt = time.Now()
	r.attempts[a.EnrollmentID] = append(r.attempts[a.EnrollmentID], a)
	return nil
}

func (r *stubRepo) GradeAttempt(_ context.Context, orgID string, a *learning.QuizAttempt) error {
	now := time.Now()
	a.SubmittedAt = &now
	a.GradedAt = &now
	return nil
}

// ── Answer keys ──────────────────────────────────────────────────────────────

func (r *stubRepo) FindAnswerKeysForTemplate(_ context.Context, orgID, templateID string) (map[string]*learning.AnswerKey, error) {
	out := map[string]*learning.AnswerKey{}
	for _, qid := range r.templateQuestions[templateID] {
		if k, ok := r.keys[qid]; ok && k.OrgID == orgID {
			out[qid] = k
		}
	}
	return out, nil
}

func (r *stubRepo) FindAnswerKeyByQuestion(_ context.Context, orgID, questionID string) (*learning.AnswerKey, error) {
	if k, ok := r.keys[questionID]; ok && k.OrgID == orgID {
		return k, nil
	}
	return nil, nil
}

func (r *stubRepo) UpsertAnswerKey(_ context.Context, k *learning.AnswerKey) error {
	k.ID = r.nextID("qkey")
	k.PublicID = "qkey_pub_" + k.ID
	r.keys[k.QuestionID] = k
	return nil
}

func (r *stubRepo) DeleteAnswerKey(_ context.Context, orgID, questionID string) error {
	delete(r.keys, questionID)
	return nil
}

// ── Employees ────────────────────────────────────────────────────────────────

func (r *stubRepo) FindEmployeeRef(_ context.Context, orgID, employeeRef string) (*learning.EmployeeRef, error) {
	if e, ok := r.employees[employeeRef]; ok {
		return e, nil
	}
	return nil, nil
}

func (r *stubRepo) FindEmployeeIDByUserID(_ context.Context, orgID, userID string) (string, error) {
	return r.empUsers[userID], nil
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

// stubForms models the engine closely enough to exercise grading: an
// instantiated instance carries one response per template question, and
// SaveAnswers writes onto those responses.
type stubForms struct {
	seq       int
	instances map[string]*forms.InstanceWithResponses
	// questions maps template id → (question id, type).
	questions map[string][]stubQuestion
	submitted map[string]bool
}

type stubQuestion struct {
	id    string
	qType forms.QuestionType
	text  string
}

func newStubForms() *stubForms {
	return &stubForms{
		instances: map[string]*forms.InstanceWithResponses{},
		questions: map[string][]stubQuestion{},
		submitted: map[string]bool{},
	}
}

func (f *stubForms) Instantiate(_ context.Context, orgID, templateRef string, subj forms.SubjectContext) (*forms.InstanceWithResponses, error) {
	f.seq++
	id := fmt.Sprintf("fins_%d", f.seq)
	inst := &forms.InstanceWithResponses{
		Instance: &forms.Instance{
			ID: id, OrgID: orgID, SubjectID: subj.SubjectID,
			RespondentUserID: subj.RespondentUserID,
		},
	}
	for i, q := range f.questions[templateRef] {
		qid := q.id
		inst.Responses = append(inst.Responses, &forms.Response{
			ID:           fmt.Sprintf("%s_r%d", id, i),
			QuestionID:   &qid,
			QuestionText: q.text,
			QuestionType: q.qType,
			DisplayOrder: i,
		})
	}
	f.instances[id] = inst
	return inst, nil
}

func (f *stubForms) GetInstance(_ context.Context, _, ref string) (*forms.InstanceWithResponses, error) {
	inst, ok := f.instances[ref]
	if !ok {
		return nil, fmt.Errorf("no such instance %s", ref)
	}
	return inst, nil
}

func (f *stubForms) GetTemplate(_ context.Context, _, ref string) (*forms.TemplateWithSections, error) {
	return &forms.TemplateWithSections{Template: &forms.Template{ID: ref}}, nil
}

func (f *stubForms) SaveAnswers(_ context.Context, _, ref, _ string, req forms.SaveAnswersRequest) (*forms.InstanceWithResponses, error) {
	inst, ok := f.instances[ref]
	if !ok {
		return nil, fmt.Errorf("no such instance %s", ref)
	}
	for _, a := range req.Answers {
		for _, r := range inst.Responses {
			if r.ID != a.ResponseID {
				continue
			}
			r.AnswerText = a.AnswerText
			r.AnswerNumber = a.AnswerNumber
			r.AnswerBoolean = a.AnswerBoolean
			r.AnswerOptions = a.AnswerOptions
		}
	}
	return inst, nil
}

func (f *stubForms) SubmitInstance(_ context.Context, _, ref, _ string) (*forms.InstanceWithResponses, error) {
	inst, ok := f.instances[ref]
	if !ok {
		return nil, fmt.Errorf("no such instance %s", ref)
	}
	f.submitted[ref] = true
	now := time.Now()
	inst.SubmittedAt = &now
	return inst, nil
}

// unused keeps decimal imported for stubs that build numeric answers.
var _ = decimal.Zero
