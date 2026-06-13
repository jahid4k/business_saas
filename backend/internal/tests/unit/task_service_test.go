// backend/internal/tests/unit/task_service_test.go
package unit

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/mridha/businesssaas/internal/audit"
	"github.com/mridha/businesssaas/internal/task"
)

// ── Stub repository ──────────────────────────────────────────────────────

type stubTaskRepo struct {
	tasks   map[string]*task.Task      // keyed by ID
	members map[string]map[string]bool // orgID -> set of valid member user IDs
	seq     int
}

func newStubTaskRepo() *stubTaskRepo {
	return &stubTaskRepo{
		tasks:   map[string]*task.Task{},
		members: map[string]map[string]bool{},
	}
}

func (r *stubTaskRepo) addMember(orgID, userID string) {
	if r.members[orgID] == nil {
		r.members[orgID] = map[string]bool{}
	}
	r.members[orgID][userID] = true
}

func (r *stubTaskRepo) FindAll(_ context.Context, orgID string, filter task.ListFilter) ([]*task.Task, error) {
	var out []*task.Task
	for _, t := range r.tasks {
		if t.OrgID != orgID {
			continue
		}
		if filter.Status != "" && t.Status != filter.Status {
			continue
		}
		if filter.AssignedTo != "" && (t.AssignedTo == nil || *t.AssignedTo != filter.AssignedTo) {
			continue
		}
		out = append(out, t)
	}
	return out, nil
}

func (r *stubTaskRepo) Count(_ context.Context, orgID string, filter task.ListFilter) (int, error) {
	tasks, _ := r.FindAll(context.Background(), orgID, filter)
	return len(tasks), nil
}

func (r *stubTaskRepo) FindByRef(_ context.Context, orgID, taskRef string) (*task.Task, error) {
	t, ok := r.tasks[taskRef]
	if !ok || t.OrgID != orgID {
		return nil, nil
	}
	return t, nil
}

func (r *stubTaskRepo) Create(_ context.Context, t *task.Task) error {
	r.seq++
	t.ID = "task-" + itoa(r.seq)
	t.PublicID = "task_pub_" + itoa(r.seq)
	t.CreatedAt = time.Now()
	t.UpdatedAt = time.Now()
	r.tasks[t.ID] = t
	return nil
}

func (r *stubTaskRepo) Update(_ context.Context, t *task.Task) error {
	existing, ok := r.tasks[t.ID]
	if !ok || existing.OrgID != t.OrgID {
		return task.ErrNotFound
	}
	t.UpdatedAt = time.Now()
	r.tasks[t.ID] = t
	return nil
}

func (r *stubTaskRepo) Delete(_ context.Context, orgID, taskRef string) error {
	existing, ok := r.tasks[taskRef]
	if !ok || existing.OrgID != orgID {
		return task.ErrNotFound
	}
	delete(r.tasks, taskRef)
	return nil
}

func (r *stubTaskRepo) ResolveOrgMember(_ context.Context, orgID, userRef string) (string, error) {
	if r.members[orgID] != nil && r.members[orgID][userRef] {
		return userRef, nil
	}
	return "", task.ErrAssigneeNotFound
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	digits := "0123456789"
	var b []byte
	for n > 0 {
		b = append([]byte{digits[n%10]}, b...)
		n /= 10
	}
	return string(b)
}

// ── Helpers ──────────────────────────────────────────────────────────────

func newTestTaskService(repo task.Repository) task.Service {
	return task.NewService(repo, audit.NewService(audit.NewNoopRepository()))
}

func ptr(s string) *string { return &s }

// ── Tests ────────────────────────────────────────────────────────────────

func TestTaskCreate_DefaultsStatusToTodo(t *testing.T) {
	svc := newTestTaskService(newStubTaskRepo())

	created, err := svc.Create(context.Background(), "org-1", "user-1", task.CreateTaskRequest{
		Title: "Write tests",
	})
	if err != nil {
		t.Fatalf("Create() unexpected error: %v", err)
	}
	if created.Status != task.StatusTodo {
		t.Errorf("expected default status %q, got %q", task.StatusTodo, created.Status)
	}
	if created.CreatedBy == nil || *created.CreatedBy != "user-1" {
		t.Errorf("expected CreatedBy=user-1, got %v", created.CreatedBy)
	}
}

func TestTaskCreate_EmptyTitle(t *testing.T) {
	svc := newTestTaskService(newStubTaskRepo())

	_, err := svc.Create(context.Background(), "org-1", "user-1", task.CreateTaskRequest{Title: "   "})
	if !errors.Is(err, task.ErrTitleRequired) {
		t.Fatalf("expected ErrTitleRequired, got %v", err)
	}
}

func TestTaskCreate_TitleTooLong(t *testing.T) {
	svc := newTestTaskService(newStubTaskRepo())

	_, err := svc.Create(context.Background(), "org-1", "user-1", task.CreateTaskRequest{
		Title: strings.Repeat("a", 256),
	})
	if !errors.Is(err, task.ErrTitleTooLong) {
		t.Fatalf("expected ErrTitleTooLong, got %v", err)
	}
}

func TestTaskCreate_InvalidStatus(t *testing.T) {
	svc := newTestTaskService(newStubTaskRepo())

	_, err := svc.Create(context.Background(), "org-1", "user-1", task.CreateTaskRequest{
		Title: "Task", Status: "not_a_status",
	})
	if !errors.Is(err, task.ErrInvalidStatus) {
		t.Fatalf("expected ErrInvalidStatus, got %v", err)
	}
}

func TestTaskCreate_ValidAssignee(t *testing.T) {
	repo := newStubTaskRepo()
	repo.addMember("org-1", "user-2")
	svc := newTestTaskService(repo)

	created, err := svc.Create(context.Background(), "org-1", "user-1", task.CreateTaskRequest{
		Title: "Task", AssignedTo: ptr("user-2"),
	})
	if err != nil {
		t.Fatalf("Create() unexpected error: %v", err)
	}
	if created.AssignedTo == nil || *created.AssignedTo != "user-2" {
		t.Errorf("expected AssignedTo=user-2, got %v", created.AssignedTo)
	}
}

func TestTaskCreate_AssigneeNotOrgMember(t *testing.T) {
	svc := newTestTaskService(newStubTaskRepo()) // no members registered

	_, err := svc.Create(context.Background(), "org-1", "user-1", task.CreateTaskRequest{
		Title: "Task", AssignedTo: ptr("user-99"),
	})
	if !errors.Is(err, task.ErrAssigneeNotFound) {
		t.Fatalf("expected ErrAssigneeNotFound, got %v", err)
	}
}

func TestTaskGet_TenantIsolation(t *testing.T) {
	repo := newStubTaskRepo()
	svc := newTestTaskService(repo)

	created, err := svc.Create(context.Background(), "org-1", "user-1", task.CreateTaskRequest{Title: "Task"})
	if err != nil {
		t.Fatalf("Create() unexpected error: %v", err)
	}

	// Same task, different org -> must look exactly like "not found"
	if _, err := svc.Get(context.Background(), "org-2", created.ID); !errors.Is(err, task.ErrNotFound) {
		t.Fatalf("expected ErrNotFound for cross-tenant Get, got %v", err)
	}

	// Correct org -> succeeds
	got, err := svc.Get(context.Background(), "org-1", created.ID)
	if err != nil {
		t.Fatalf("Get() unexpected error: %v", err)
	}
	if got.ID != created.ID {
		t.Errorf("expected to get back task %s, got %s", created.ID, got.ID)
	}
}

func TestTaskUpdate_PartialUpdateOnlyChangesSentFields(t *testing.T) {
	svc := newTestTaskService(newStubTaskRepo())

	created, _ := svc.Create(context.Background(), "org-1", "user-1", task.CreateTaskRequest{
		Title: "Original title", Description: "Original description",
	})

	updated, err := svc.Update(context.Background(), "org-1", created.ID, task.UpdateTaskRequest{
		Status: ptr("in_progress"),
	})
	if err != nil {
		t.Fatalf("Update() unexpected error: %v", err)
	}
	if updated.Title != "Original title" {
		t.Errorf("Title should be unchanged, got %q", updated.Title)
	}
	if updated.Description != "Original description" {
		t.Errorf("Description should be unchanged, got %q", updated.Description)
	}
	if updated.Status != task.StatusInProgress {
		t.Errorf("Status should be in_progress, got %q", updated.Status)
	}
}

func TestTaskUpdate_UnassignViaEmptyString(t *testing.T) {
	repo := newStubTaskRepo()
	repo.addMember("org-1", "user-2")
	svc := newTestTaskService(repo)

	created, _ := svc.Create(context.Background(), "org-1", "user-1", task.CreateTaskRequest{
		Title: "Task", AssignedTo: ptr("user-2"),
	})
	if created.AssignedTo == nil {
		t.Fatal("expected task to be assigned after create")
	}

	updated, err := svc.Update(context.Background(), "org-1", created.ID, task.UpdateTaskRequest{
		AssignedTo: ptr(""),
	})
	if err != nil {
		t.Fatalf("Update() unexpected error: %v", err)
	}
	if updated.AssignedTo != nil {
		t.Errorf("expected AssignedTo to be nil after unassign, got %v", *updated.AssignedTo)
	}
}

func TestTaskUpdate_CrossTenantReturnsNotFound(t *testing.T) {
	svc := newTestTaskService(newStubTaskRepo())

	created, _ := svc.Create(context.Background(), "org-1", "user-1", task.CreateTaskRequest{Title: "Task"})

	if _, err := svc.Update(context.Background(), "org-2", created.ID, task.UpdateTaskRequest{Title: ptr("Hacked")}); !errors.Is(err, task.ErrNotFound) {
		t.Fatalf("expected ErrNotFound for cross-tenant Update, got %v", err)
	}
}

func TestTaskList_EmptySliceNotNilWithTotal(t *testing.T) {
	svc := newTestTaskService(newStubTaskRepo())

	result, err := svc.List(context.Background(), "org-1", task.ListFilter{})
	if err != nil {
		t.Fatalf("List() unexpected error: %v", err)
	}
	if result.Tasks == nil {
		t.Error("expected non-nil empty slice, got nil")
	}
	if result.Total != 0 {
		t.Errorf("expected Total=0, got %d", result.Total)
	}
	if result.Limit != task.DefaultLimit {
		t.Errorf("expected default Limit=%d, got %d", task.DefaultLimit, result.Limit)
	}
}

func TestTaskDelete_TenantIsolation(t *testing.T) {
	svc := newTestTaskService(newStubTaskRepo())

	created, _ := svc.Create(context.Background(), "org-1", "user-1", task.CreateTaskRequest{Title: "Task"})

	if err := svc.Delete(context.Background(), "org-2", created.ID); !errors.Is(err, task.ErrNotFound) {
		t.Fatalf("expected ErrNotFound for cross-tenant Delete, got %v", err)
	}

	if err := svc.Delete(context.Background(), "org-1", created.ID); err != nil {
		t.Fatalf("Delete() unexpected error: %v", err)
	}

	if _, err := svc.Get(context.Background(), "org-1", created.ID); !errors.Is(err, task.ErrNotFound) {
		t.Fatalf("expected task to be gone after delete, got %v", err)
	}
}
