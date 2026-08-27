// backend/internal/platform/kb/model.go
package kb

import (
	"context"
	"errors"
	"time"
)

// AccessDirectory is the minimal slice of authz.Service this package needs.
// Declared locally so this package gains no platform → authz import edge;
// authz.Service satisfies it structurally.
//
// ⚠ Parameter ORDER is load-bearing: satisfaction is structural, not
// declared. The checklists/forms/tickets AccessDirectory precedent. Unlike
// tickets, no UserRoleName here — the KB has no role-restricted content.
//
// ⚠ The resource string passed to Can must carry the FULL dotted prefix
// ("platform.kb"), because authz.Can builds its key as resource+"."+action.
// A bare name denies everything silently and uniformly.
type AccessDirectory interface {
	Can(ctx context.Context, userID, orgID, resource, action string) (bool, error)
}

// ── Categories ───────────────────────────────────────────────────────────────

type Category struct {
	ID          string    `db:"id"           json:"id"`
	PublicID    string    `db:"public_id"     json:"public_id"`
	OrgID       string    `db:"org_id"        json:"org_id"`
	Name        string    `db:"name"          json:"name"`
	Description *string   `db:"description"   json:"description,omitempty"`
	IsActive    bool      `db:"is_active"     json:"is_active"`
	CreatedBy   string    `db:"created_by"    json:"created_by"`
	CreatedAt   time.Time `db:"created_at"    json:"created_at"`
	UpdatedAt   time.Time `db:"updated_at"    json:"updated_at"`
}

type CreateCategoryRequest struct {
	Name        string  `json:"name"`
	Description *string `json:"description"`
	IsActive    *bool   `json:"is_active"`
}

// ── Articles ─────────────────────────────────────────────────────────────────

type Status string

const (
	StatusDraft     Status = "draft"
	StatusPublished Status = "published"
	StatusArchived  Status = "archived"
)

func (s Status) IsValid() bool {
	switch s {
	case StatusDraft, StatusPublished, StatusArchived:
		return true
	}
	return false
}

// Article carries no view_count — see migration 00112. A counter nothing can
// recompute is unauditable the moment it drifts.
type Article struct {
	ID           string     `db:"id"              json:"id"`
	PublicID     string     `db:"public_id"        json:"public_id"`
	OrgID        string     `db:"org_id"           json:"org_id"`
	CategoryID   *string    `db:"category_id"      json:"category_id,omitempty"`
	Title        string     `db:"title"            json:"title"`
	Body         string     `db:"body"             json:"body"`
	Status       Status     `db:"status"           json:"status"`
	AuthorUserID string     `db:"author_user_id"   json:"author_user_id"`
	PublishedAt  *time.Time `db:"published_at"     json:"published_at,omitempty"`
	CreatedAt    time.Time  `db:"created_at"       json:"created_at"`
	UpdatedAt    time.Time  `db:"updated_at"       json:"updated_at"`
}

type CreateArticleRequest struct {
	CategoryID *string `json:"category_id"`
	Title      string  `json:"title"`
	Body       string  `json:"body"`
}

type UpdateArticleRequest struct {
	CategoryID *string `json:"category_id"`
	Title      *string `json:"title"`
	Body       *string `json:"body"`
}

// ── Listing ──────────────────────────────────────────────────────────────────

// ArticleFilter has no Scope field, for the same reason tickets' ListFilter
// does not: internal/hrm/scope hard-codes FROM hrm_employees and a platform
// package cannot use it. Unlike tickets there is nothing to narrow per-user
// either — a knowledge base is org-wide reading material. The one visibility
// rule is IncludeUnpublished, set by the SERVICE from platform.kb.manage and
// never read off the request.
type ArticleFilter struct {
	// Query is a full-text search over title and body. Empty means list all.
	Query      string
	CategoryID string
	Status     string

	// IncludeUnpublished is resolved from the caller's permissions. A draft
	// HR policy read as authoritative is worse than no article at all.
	IncludeUnpublished bool

	Limit  int
	Offset int
}

const (
	DefaultLimit = 25
	MaxLimit     = 100
)

func (f *ArticleFilter) Normalise() {
	if f.Limit <= 0 {
		f.Limit = DefaultLimit
	}
	if f.Limit > MaxLimit {
		f.Limit = MaxLimit
	}
	if f.Offset < 0 {
		f.Offset = 0
	}
}

type ArticleListResponse struct {
	Articles []*Article `json:"articles"`
	Total    int        `json:"total"`
	Limit    int        `json:"limit"`
	Offset   int        `json:"offset"`
}

// ── Errors ───────────────────────────────────────────────────────────────────

var (
	ErrArticleNotFound  = errors.New("knowledge base article not found")
	ErrCategoryNotFound = errors.New("knowledge base category not found")
	ErrTitleRequired    = errors.New("title is required")
	ErrBodyRequired     = errors.New("body is required")
	ErrNameRequired     = errors.New("name is required")
	ErrInvalidStatus    = errors.New("status is not a recognised value")
	ErrAlreadyPublished = errors.New("article is already published")
	ErrNotPublished     = errors.New("article is not published")
	ErrAccessDenied     = errors.New("access denied")
)
