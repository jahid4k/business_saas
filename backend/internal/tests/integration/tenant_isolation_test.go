// backend/internal/tests/integration/tenant_isolation_test.go
// Critical security tests: verify that one tenant CANNOT read, write, or delete
// another tenant's data — against a real Postgres + Redis.
// Gate: INTEGRATION=1
package integration

import (
	"context"
	"errors"
	"testing"

	"github.com/mridha/businesssaas/internal/auth"
	"github.com/mridha/businesssaas/internal/organizations"
	"github.com/mridha/businesssaas/internal/task"
)

// setupTwoOrgs creates two isolated organizations, each with their own owner.
// Returns the two org IDs and a cleanup function.
func setupTwoOrgs(t *testing.T, env *testEnv) (orgAID, orgBID, userAID, userBID string) {
	t.Helper()
	ctx := context.Background()

	emailA := uniqueEmail("tenant-a")
	emailB := uniqueEmail("tenant-b")
	slugA := uniqueSlug("org-a")
	slugB := uniqueSlug("org-b")

	safeA, err := env.authSvc.Signup(ctx, auth.SignupRequest{Email: emailA, Password: "PassA123!"})
	if err != nil {
		t.Fatalf("Signup user-A: %v", err)
	}
	safeB, err := env.authSvc.Signup(ctx, auth.SignupRequest{Email: emailB, Password: "PassB123!"})
	if err != nil {
		t.Fatalf("Signup user-B: %v", err)
	}

	orgA, err := env.orgSvc.Create(ctx, safeA.ID, organizations.CreateBusinessRequest{
		Name: "Org A " + slugA,
		Slug: slugA,
	})
	if err != nil {
		t.Fatalf("Create org-A: %v", err)
	}
	orgB, err := env.orgSvc.Create(ctx, safeB.ID, organizations.CreateBusinessRequest{
		Name: "Org B " + slugB,
		Slug: slugB,
	})
	if err != nil {
		t.Fatalf("Create org-B: %v", err)
	}

	t.Cleanup(func() {
		cleanupUser(t, env, safeA.ID)
		cleanupUser(t, env, safeB.ID)
		_, _ = env.db.Exec(ctx, `DELETE FROM organizations WHERE id IN ($1, $2)`, orgA.ID, orgB.ID)
	})

	return orgA.ID, orgB.ID, safeA.ID, safeB.ID
}

// TestIntegration_TenantIsolation_Tasks verifies that org-B cannot read, update,
// or delete a task that belongs to org-A.
func TestIntegration_TenantIsolation_Tasks(t *testing.T) {
	env := newTestEnv(t)
	ctx := context.Background()
	orgAID, orgBID, userAID, _ := setupTwoOrgs(t, env)

	// Create a task in org-A
	taskA, err := env.taskSvc.Create(ctx, orgAID, userAID, task.CreateTaskRequest{
		Title: "Org-A Secret Task",
	})
	if err != nil {
		t.Fatalf("Create task in org-A: %v", err)
	}

	// Attempt to read from org-B — must fail with not found
	_, err = env.taskSvc.Get(ctx, orgBID, taskA.ID)
	if !errors.Is(err, task.ErrNotFound) {
		t.Errorf("SECURITY: org-B can read org-A task — expected ErrNotFound, got %v", err)
	}

	// Attempt to update from org-B — must fail
	newTitle := "Hacked Title"
	_, err = env.taskSvc.Update(ctx, orgBID, taskA.ID, task.UpdateTaskRequest{
		Title: &newTitle,
	})
	if !errors.Is(err, task.ErrNotFound) {
		t.Errorf("SECURITY: org-B can update org-A task — expected ErrNotFound, got %v", err)
	}

	// Attempt to delete from org-B — must fail
	err = env.taskSvc.Delete(ctx, orgBID, taskA.ID)
	if !errors.Is(err, task.ErrNotFound) {
		t.Errorf("SECURITY: org-B can delete org-A task — expected ErrNotFound, got %v", err)
	}

	// The original task must still exist in org-A
	original, err := env.taskSvc.Get(ctx, orgAID, taskA.ID)
	if err != nil {
		t.Fatalf("task should still exist in org-A after cross-org attempts: %v", err)
	}
	if original.Title != "Org-A Secret Task" {
		t.Errorf("task title was modified by cross-org update attempt: got %q", original.Title)
	}
}

// TestIntegration_TenantIsolation_TaskList verifies that listing tasks in org-B
// never returns tasks that belong to org-A.
func TestIntegration_TenantIsolation_TaskList(t *testing.T) {
	env := newTestEnv(t)
	ctx := context.Background()
	orgAID, orgBID, userAID, _ := setupTwoOrgs(t, env)

	// Create tasks in org-A
	taskA1, _ := env.taskSvc.Create(ctx, orgAID, userAID, task.CreateTaskRequest{Title: "Org-A Task 1"})
	taskA2, _ := env.taskSvc.Create(ctx, orgAID, userAID, task.CreateTaskRequest{Title: "Org-A Task 2"})

	// List from org-B — must return 0 tasks
	result, err := env.taskSvc.List(ctx, orgBID, task.ListFilter{})
	if err != nil {
		t.Fatalf("List() error: %v", err)
	}
	if result.Total != 0 {
		t.Errorf("SECURITY: org-B list returned %d tasks, expected 0", result.Total)
	}
	for _, tk := range result.Tasks {
		if tk.ID == taskA1.ID || tk.ID == taskA2.ID {
			t.Errorf("SECURITY: org-A task %s appeared in org-B list", tk.ID)
		}
	}
}

// TestIntegration_TenantIsolation_OrgAccess verifies that user-A cannot view org-B.
func TestIntegration_TenantIsolation_OrgAccess(t *testing.T) {
	env := newTestEnv(t)
	ctx := context.Background()
	_, orgBID, userAID, _ := setupTwoOrgs(t, env)

	// user-A tries to access org-B
	_, err := env.orgSvc.GetByID(ctx, orgBID, userAID)
	if !errors.Is(err, organizations.ErrNotMember) {
		t.Errorf("SECURITY: user-A can access org-B data — expected ErrNotMember, got %v", err)
	}
}

// TestIntegration_TenantIsolation_BusinessSwitch verifies that user-A cannot
// switch into org-B (and would never receive a scoped access token for it).
func TestIntegration_TenantIsolation_BusinessSwitch(t *testing.T) {
	env := newTestEnv(t)
	ctx := context.Background()
	_, orgBID, userAID, _ := setupTwoOrgs(t, env)

	// user-A tries to switch into org-B
	_, _, err := env.orgSvc.Switch(ctx, orgBID, userAID)
	if !errors.Is(err, organizations.ErrNotMember) {
		t.Errorf("SECURITY: user-A received org-B access token — expected ErrNotMember, got %v", err)
	}
}

// TestIntegration_TenantIsolation_RBAC verifies that permissions held in org-A
// have no effect when checking access in org-B.
func TestIntegration_TenantIsolation_RBAC(t *testing.T) {
	env := newTestEnv(t)
	ctx := context.Background()
	orgAID, orgBID, userAID, _ := setupTwoOrgs(t, env)

	// userAID is owner of org-A — confirm they have permission there
	allowedA, err := env.authzSvc.Can(ctx, userAID, orgAID, "crm.contacts", "view")
	if err != nil {
		t.Fatalf("Can() in org-A: %v", err)
	}

	// If they have permission in org-A (likely as owner), they must NOT in org-B
	if allowedA {
		allowedB, err := env.authzSvc.Can(ctx, userAID, orgBID, "crm.contacts", "view")
		if err != nil {
			t.Fatalf("Can() in org-B: %v", err)
		}
		if allowedB {
			t.Error("SECURITY VIOLATION: user-A's org-A permissions leaked into org-B")
		}
	}
}

// TestIntegration_TenantIsolation_TaskCreation verifies org isolation on writes:
// tasks created in org-A have orgID=org-A and are never visible in org-B queries.
func TestIntegration_TenantIsolation_TaskCreation(t *testing.T) {
	env := newTestEnv(t)
	ctx := context.Background()
	orgAID, orgBID, userAID, userBID := setupTwoOrgs(t, env)

	// Create one task in each org
	_, _ = env.taskSvc.Create(ctx, orgAID, userAID, task.CreateTaskRequest{Title: "Task for Org A"})
	_, _ = env.taskSvc.Create(ctx, orgBID, userBID, task.CreateTaskRequest{Title: "Task for Org B"})

	// Org-A list must only contain org-A tasks
	listA, _ := env.taskSvc.List(ctx, orgAID, task.ListFilter{})
	for _, tk := range listA.Tasks {
		if tk.OrgID != orgAID {
			t.Errorf("SECURITY: task with orgID=%q appeared in org-A list", tk.OrgID)
		}
	}

	// Org-B list must only contain org-B tasks
	listB, _ := env.taskSvc.List(ctx, orgBID, task.ListFilter{})
	for _, tk := range listB.Tasks {
		if tk.OrgID != orgBID {
			t.Errorf("SECURITY: task with orgID=%q appeared in org-B list", tk.OrgID)
		}
	}
}
