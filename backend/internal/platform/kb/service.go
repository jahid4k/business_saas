// backend/internal/platform/kb/service.go
package kb

import (
	"context"
	"fmt"
	"strings"
	"time"
)

// Service is the knowledge base's business layer.
//
// Two permissions only: platform.kb.view reads published articles,
// platform.kb.manage reads and writes everything including drafts. The
// draft/published split is what makes two keys enough — a KB is org-wide
// reading material with no "mine" to narrow to, unlike tickets.
type Service interface {
	// Categories
	ListCategories(ctx context.Context, orgID, callerUserID string, activeOnly bool) ([]*Category, error)
	CreateCategory(ctx context.Context, orgID, callerUserID string, req CreateCategoryRequest) (*Category, error)
	UpdateCategory(ctx context.Context, orgID, callerUserID, ref string, req CreateCategoryRequest) (*Category, error)

	// Articles
	CreateArticle(ctx context.Context, orgID, callerUserID string, req CreateArticleRequest) (*Article, error)
	ListArticles(ctx context.Context, orgID, callerUserID string, f ArticleFilter) (*ArticleListResponse, error)
	GetArticle(ctx context.Context, orgID, callerUserID, ref string) (*Article, error)
	UpdateArticle(ctx context.Context, orgID, callerUserID, ref string, req UpdateArticleRequest) (*Article, error)
	Publish(ctx context.Context, orgID, callerUserID, ref string) (*Article, error)
	Archive(ctx context.Context, orgID, callerUserID, ref string) (*Article, error)
}

type serviceImpl struct {
	repo      Repository
	directory AccessDirectory
}

func NewService(repo Repository, directory AccessDirectory) Service {
	return &serviceImpl{repo: repo, directory: directory}
}

// ── Access helpers ───────────────────────────────────────────────────────────

// can passes the FULL dotted resource prefix, because authz.Can builds its
// key as resource+"."+action. Passing a bare "kb" would deny everything
// silently and uniformly — a trap this codebase has now hit once.
func (s *serviceImpl) can(ctx context.Context, orgID, userID, action string) (bool, error) {
	ok, err := s.directory.Can(ctx, userID, orgID, "platform.kb", action)
	if err != nil {
		return false, fmt.Errorf("kb: access check platform.kb.%s: %w", action, err)
	}
	return ok, nil
}

func (s *serviceImpl) requireView(ctx context.Context, orgID, userID string) error {
	ok, err := s.can(ctx, orgID, userID, "view")
	if err != nil {
		return err
	}
	if ok {
		return nil
	}
	// A manage holder can always read. Granting .manage without .view would
	// otherwise lock an author out of their own drafts.
	manage, err := s.can(ctx, orgID, userID, "manage")
	if err != nil {
		return err
	}
	if !manage {
		return ErrAccessDenied
	}
	return nil
}

func (s *serviceImpl) requireManage(ctx context.Context, orgID, userID string) error {
	ok, err := s.can(ctx, orgID, userID, "manage")
	if err != nil {
		return err
	}
	if !ok {
		return ErrAccessDenied
	}
	return nil
}

// ── Categories ───────────────────────────────────────────────────────────────

func (s *serviceImpl) ListCategories(ctx context.Context, orgID, callerUserID string, activeOnly bool) ([]*Category, error) {
	if err := s.requireView(ctx, orgID, callerUserID); err != nil {
		return nil, err
	}
	return s.repo.FindCategories(ctx, orgID, activeOnly)
}

func (s *serviceImpl) CreateCategory(ctx context.Context, orgID, callerUserID string, req CreateCategoryRequest) (*Category, error) {
	if err := s.requireManage(ctx, orgID, callerUserID); err != nil {
		return nil, err
	}
	name := strings.TrimSpace(req.Name)
	if name == "" {
		return nil, ErrNameRequired
	}
	c := &Category{OrgID: orgID, Name: name, Description: req.Description, CreatedBy: callerUserID}
	if err := s.repo.CreateCategory(ctx, c); err != nil {
		return nil, err
	}
	return c, nil
}

func (s *serviceImpl) UpdateCategory(ctx context.Context, orgID, callerUserID, ref string, req CreateCategoryRequest) (*Category, error) {
	if err := s.requireManage(ctx, orgID, callerUserID); err != nil {
		return nil, err
	}
	c, err := s.repo.FindCategoryByRef(ctx, orgID, ref)
	if err != nil {
		return nil, fmt.Errorf("kb: UpdateCategory: %w", err)
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
	if req.IsActive != nil {
		c.IsActive = *req.IsActive
	}
	if err := s.repo.UpdateCategory(ctx, c); err != nil {
		return nil, err
	}
	return c, nil
}

// ── Articles ─────────────────────────────────────────────────────────────────

func (s *serviceImpl) CreateArticle(ctx context.Context, orgID, callerUserID string, req CreateArticleRequest) (*Article, error) {
	if err := s.requireManage(ctx, orgID, callerUserID); err != nil {
		return nil, err
	}
	title := strings.TrimSpace(req.Title)
	if title == "" {
		return nil, ErrTitleRequired
	}
	body := strings.TrimSpace(req.Body)
	if body == "" {
		return nil, ErrBodyRequired
	}
	categoryID, err := s.resolveCategory(ctx, orgID, req.CategoryID)
	if err != nil {
		return nil, err
	}
	// Always born a draft. Creating something already published would make
	// "write it, then decide" impossible, and the first save of an article
	// is the least likely to be the one worth publishing.
	a := &Article{
		OrgID: orgID, CategoryID: categoryID, Title: title, Body: body,
		AuthorUserID: callerUserID,
	}
	if err := s.repo.CreateArticle(ctx, a); err != nil {
		return nil, err
	}
	return a, nil
}

func (s *serviceImpl) resolveCategory(ctx context.Context, orgID string, ref *string) (*string, error) {
	if ref == nil || strings.TrimSpace(*ref) == "" {
		return nil, nil
	}
	c, err := s.repo.FindCategoryByRef(ctx, orgID, *ref)
	if err != nil {
		return nil, fmt.Errorf("kb: resolve category: %w", err)
	}
	if c == nil {
		return nil, ErrCategoryNotFound
	}
	return &c.ID, nil
}

func (s *serviceImpl) ListArticles(ctx context.Context, orgID, callerUserID string, f ArticleFilter) (*ArticleListResponse, error) {
	if err := s.requireView(ctx, orgID, callerUserID); err != nil {
		return nil, err
	}
	manage, err := s.can(ctx, orgID, callerUserID, "manage")
	if err != nil {
		return nil, err
	}
	// Never read off the request — a caller who could set this would hand
	// themselves every draft in the org.
	f.IncludeUnpublished = manage
	if f.Status != "" && !Status(f.Status).IsValid() {
		return nil, ErrInvalidStatus
	}
	f.Normalise()

	list, err := s.repo.FindArticles(ctx, orgID, f)
	if err != nil {
		return nil, err
	}
	total, err := s.repo.CountArticles(ctx, orgID, f)
	if err != nil {
		return nil, err
	}
	return &ArticleListResponse{Articles: list, Total: total, Limit: f.Limit, Offset: f.Offset}, nil
}

func (s *serviceImpl) GetArticle(ctx context.Context, orgID, callerUserID, ref string) (*Article, error) {
	if err := s.requireView(ctx, orgID, callerUserID); err != nil {
		return nil, err
	}
	manage, err := s.can(ctx, orgID, callerUserID, "manage")
	if err != nil {
		return nil, err
	}
	a, err := s.repo.FindArticleByRef(ctx, orgID, ref, manage)
	if err != nil {
		return nil, fmt.Errorf("kb: GetArticle: %w", err)
	}
	// A draft reports NOT-FOUND to a non-manage caller rather than denied,
	// so the single-row read agrees with the list that hides it. The
	// exclusion is in SQL — FindArticleByRef never selected the row.
	if a == nil {
		return nil, ErrArticleNotFound
	}
	return a, nil
}

func (s *serviceImpl) UpdateArticle(ctx context.Context, orgID, callerUserID, ref string, req UpdateArticleRequest) (*Article, error) {
	if err := s.requireManage(ctx, orgID, callerUserID); err != nil {
		return nil, err
	}
	a, err := s.loadForManage(ctx, orgID, ref)
	if err != nil {
		return nil, err
	}
	if req.Title != nil {
		t := strings.TrimSpace(*req.Title)
		if t == "" {
			return nil, ErrTitleRequired
		}
		a.Title = t
	}
	if req.Body != nil {
		b := strings.TrimSpace(*req.Body)
		if b == "" {
			return nil, ErrBodyRequired
		}
		a.Body = b
	}
	if req.CategoryID != nil {
		categoryID, err := s.resolveCategory(ctx, orgID, req.CategoryID)
		if err != nil {
			return nil, err
		}
		a.CategoryID = categoryID
	}
	// Editing a published article leaves it published — correcting a live
	// policy must not silently unpublish it and leave employees reading
	// nothing. published_at keeps its original value: it records when the
	// article first went live, not when it was last touched.
	if err := s.repo.UpdateArticle(ctx, a); err != nil {
		return nil, err
	}
	return a, nil
}

func (s *serviceImpl) loadForManage(ctx context.Context, orgID, ref string) (*Article, error) {
	a, err := s.repo.FindArticleByRef(ctx, orgID, ref, true)
	if err != nil {
		return nil, fmt.Errorf("kb: load article %s: %w", ref, err)
	}
	if a == nil {
		return nil, ErrArticleNotFound
	}
	return a, nil
}

func (s *serviceImpl) Publish(ctx context.Context, orgID, callerUserID, ref string) (*Article, error) {
	if err := s.requireManage(ctx, orgID, callerUserID); err != nil {
		return nil, err
	}
	a, err := s.loadForManage(ctx, orgID, ref)
	if err != nil {
		return nil, err
	}
	if a.Status == StatusPublished {
		return nil, ErrAlreadyPublished
	}
	a.Status = StatusPublished
	// Only stamped on the FIRST publish. Re-publishing an archived article
	// restores it; it does not pretend to be newly written, and the list
	// order (COALESCE(published_at, created_at)) would otherwise jump it to
	// the top as if it were new.
	if a.PublishedAt == nil {
		now := time.Now()
		a.PublishedAt = &now
	}
	if err := s.repo.UpdateArticle(ctx, a); err != nil {
		return nil, err
	}
	return a, nil
}

func (s *serviceImpl) Archive(ctx context.Context, orgID, callerUserID, ref string) (*Article, error) {
	if err := s.requireManage(ctx, orgID, callerUserID); err != nil {
		return nil, err
	}
	a, err := s.loadForManage(ctx, orgID, ref)
	if err != nil {
		return nil, err
	}
	if a.Status == StatusArchived {
		return a, nil
	}
	// Archive rather than delete: an article people have relied on is a
	// record of what they were told, and superseded guidance still explains
	// why somebody acted the way they did.
	a.Status = StatusArchived
	if err := s.repo.UpdateArticle(ctx, a); err != nil {
		return nil, err
	}
	return a, nil
}
