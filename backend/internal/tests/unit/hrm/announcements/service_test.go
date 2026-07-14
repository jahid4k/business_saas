package announcements_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/mridha/businesssaas/internal/hrm/announcements"
)

type stubRepo struct {
	data       map[string]*announcements.Announcement
	targetEmps map[string][]string // scopeType -> []empID (simplified for tests)
}

func newStubRepo() *stubRepo {
	return &stubRepo{
		data:       make(map[string]*announcements.Announcement),
		targetEmps: make(map[string][]string),
	}
}

func (r *stubRepo) FindAll(ctx context.Context, orgID, category, status string) ([]*announcements.Announcement, error) {
	var res []*announcements.Announcement
	for _, a := range r.data {
		if a.OrgID == orgID {
			if category != "" && string(a.Category) != category {
				continue
			}
			if status != "" && string(a.Status) != status {
				continue
			}
			res = append(res, a)
		}
	}
	return res, nil
}

func (r *stubRepo) FindByRef(ctx context.Context, orgID, ref string) (*announcements.Announcement, error) {
	for _, a := range r.data {
		if a.OrgID == orgID && (a.ID == ref || a.PublicID == ref) {
			return a, nil
		}
	}
	return nil, nil
}

func (r *stubRepo) Create(ctx context.Context, a *announcements.Announcement) error {
	a.ID = uuid.NewString()
	a.PublicID = "ann_" + a.ID
	a.CreatedAt = time.Now()
	a.UpdatedAt = time.Now()
	r.data[a.ID] = a
	return nil
}

func (r *stubRepo) Update(ctx context.Context, a *announcements.Announcement) error {
	a.UpdatedAt = time.Now()
	r.data[a.ID] = a
	return nil
}

func (r *stubRepo) UpdateStatus(ctx context.Context, id string, status announcements.AnnStatus, publishedAt *interface{}) error {
	if a, ok := r.data[id]; ok {
		a.Status = status
		if publishedAt != nil {
			now := time.Now()
			a.PublishedAt = &now
		}
		a.UpdatedAt = time.Now()
	}
	return nil
}

func (r *stubRepo) GetTargetEmployeeIDs(ctx context.Context, orgID string, scopeType announcements.ScopeType, scopeIDs []string) ([]string, error) {
	return r.targetEmps[string(scopeType)], nil
}

func TestAnnouncementsService(t *testing.T) {
	repo := newStubRepo()
	// Pass nil for db since Publish does C4 integrations.
	// Note: in actual implementation s.db.Exec is called if RequiresAcknowledgement is true.
	// Since db is nil, we should test RequiresAcknowledgement=false or handle the panic.
	// Actually, wait, it might panic if db is nil. We'll set RequiresAcknowledgement = false.
	svc := announcements.NewService(repo, nil)
	ctx := context.Background()
	orgID := "org1"

	t.Run("Create Announcement", func(t *testing.T) {
		req := announcements.CreateAnnouncementRequest{
			Title: "Welcome",
			Content: "Hello world",
		}
		a, err := svc.Create(ctx, orgID, "admin", req)
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if a.Title != "Welcome" {
			t.Errorf("expected Welcome, got %s", a.Title)
		}
		if a.Status != announcements.StatusDraft {
			t.Errorf("expected draft, got %s", a.Status)
		}
	})

	t.Run("Create Validation", func(t *testing.T) {
		req := announcements.CreateAnnouncementRequest{Title: "", Content: "x"}
		_, err := svc.Create(ctx, orgID, "admin", req)
		if err != announcements.ErrTitleRequired {
			t.Errorf("expected ErrTitleRequired, got %v", err)
		}
		
		req2 := announcements.CreateAnnouncementRequest{Title: "T", Content: ""}
		_, err = svc.Create(ctx, orgID, "admin", req2)
		if err != announcements.ErrContentRequired {
			t.Errorf("expected ErrContentRequired, got %v", err)
		}
	})

	t.Run("Update Announcement", func(t *testing.T) {
		req := announcements.CreateAnnouncementRequest{Title: "T", Content: "C"}
		a, _ := svc.Create(ctx, orgID, "admin", req)
		
		newContent := "New Content"
		updateReq := announcements.UpdateAnnouncementRequest{Content: &newContent}
		updated, err := svc.Update(ctx, orgID, a.ID, updateReq)
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if updated.Content != "New Content" {
			t.Errorf("expected New Content, got %s", updated.Content)
		}
	})

	t.Run("Status Transitions", func(t *testing.T) {
		req := announcements.CreateAnnouncementRequest{Title: "T", Content: "C", RequiresAcknowledgement: false}
		a, _ := svc.Create(ctx, orgID, "admin", req)

		// Schedule
		scheduled, err := svc.Schedule(ctx, orgID, a.ID)
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if scheduled.Status != announcements.StatusScheduled {
			t.Errorf("expected scheduled, got %s", scheduled.Status)
		}

		// List
		list, err := svc.List(ctx, orgID, "", "")
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		found := false
		for _, v := range list.Announcements {
			if v.ID == a.ID {
				found = true
			}
		}
		if !found {
			t.Errorf("expected to find announcement in list")
		}

		// Publish
		// It's in scheduled state. The logic doesn't explicitly prevent publishing from scheduled, just says `if StatusPublished...`. Let's see: `if a.Status == StatusArchived { return err }`. 
		published, err := svc.Publish(ctx, orgID, a.ID, "admin")
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if published.Status != announcements.StatusPublished {
			t.Errorf("expected published, got %s", published.Status)
		}
		if published.PublishedAt == nil {
			t.Errorf("expected publishedAt to be set")
		}

		// Archive
		archived, err := svc.Archive(ctx, orgID, a.ID)
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if archived.Status != announcements.StatusArchived {
			t.Errorf("expected archived, got %s", archived.Status)
		}
	})

	t.Run("Cross-Org Privacy", func(t *testing.T) {
		req := announcements.CreateAnnouncementRequest{Title: "T", Content: "C"}
		a, _ := svc.Create(ctx, orgID, "admin", req)

		_, err := svc.Get(ctx, "org2", a.ID)
		if err != announcements.ErrNotFound {
			t.Errorf("expected ErrNotFound for wrong org, got %v", err)
		}
	})
}
