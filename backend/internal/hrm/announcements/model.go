// backend/internal/hrm/announcements/model.go
package announcements

import (
	"errors"
	"time"
)

type AnnouncementCategory string
const (
	CatGeneral   AnnouncementCategory = "general"
	CatPolicy    AnnouncementCategory = "policy"
	CatEvent     AnnouncementCategory = "event"
	CatAward     AnnouncementCategory = "award"
	CatReminder  AnnouncementCategory = "reminder"
	CatEmergency AnnouncementCategory = "emergency"
	CatHRUpdate  AnnouncementCategory = "hr_update"
)
func (c AnnouncementCategory) IsValid() bool {
	switch c { case CatGeneral, CatPolicy, CatEvent, CatAward, CatReminder, CatEmergency, CatHRUpdate: return true }
	return false
}

type ScopeType string
const (
	ScopeOrganization ScopeType = "organization"
	ScopeDepartment   ScopeType = "department"
	ScopeIndividual   ScopeType = "individual"
)

type AnnStatus string
const (
	StatusDraft     AnnStatus = "draft"
	StatusScheduled AnnStatus = "scheduled"
	StatusPublished AnnStatus = "published"
	StatusExpired   AnnStatus = "expired"
	StatusArchived  AnnStatus = "archived"
)

// Announcement is an org-wide, department, or individual communication.
type Announcement struct {
	ID                      string               `db:"id"                       json:"id"`
	PublicID                string               `db:"public_id"                json:"public_id"`
	OrgID                   string               `db:"org_id"                   json:"org_id"`
	Title                   string               `db:"title"                    json:"title"`
	Content                 string               `db:"content"                  json:"content"`
	Category                AnnouncementCategory `db:"category"                 json:"category"`
	ScopeType               ScopeType            `db:"scope_type"               json:"scope_type"`
	ScopeIDs                []string             `db:"scope_ids"                json:"scope_ids"`
	ScheduledAt             *time.Time           `db:"scheduled_at"             json:"scheduled_at,omitempty"`
	PublishedAt             *time.Time           `db:"published_at"             json:"published_at,omitempty"`
	ExpiresAt               *time.Time           `db:"expires_at"               json:"expires_at,omitempty"`
	RequiresAcknowledgement bool                 `db:"requires_acknowledgement" json:"requires_acknowledgement"`
	AcknowledgementDeadline *string              `db:"acknowledgement_deadline" json:"acknowledgement_deadline,omitempty"`
	IsPinned                bool                 `db:"is_pinned"                json:"is_pinned"`
	PinOrder                int                  `db:"pin_order"                json:"pin_order"`
	AuthorID                string               `db:"author_id"                json:"author_id"`
	Status                  AnnStatus            `db:"status"                   json:"status"`
	CreatedBy               string               `db:"created_by"               json:"created_by"`
	CreatedAt               time.Time            `db:"created_at"               json:"created_at"`
	UpdatedAt               time.Time            `db:"updated_at"               json:"updated_at"`
}

type CreateAnnouncementRequest struct {
	Title                   string               `json:"title"`
	Content                 string               `json:"content"`
	Category                AnnouncementCategory `json:"category"`
	ScopeType               ScopeType            `json:"scope_type"`
	ScopeIDs                []string             `json:"scope_ids"`
	ScheduledAt             *time.Time           `json:"scheduled_at"`
	ExpiresAt               *time.Time           `json:"expires_at"`
	RequiresAcknowledgement bool                 `json:"requires_acknowledgement"`
	AcknowledgementDeadline *string              `json:"acknowledgement_deadline"`
	IsPinned                bool                 `json:"is_pinned"`
}

type UpdateAnnouncementRequest struct {
	Title                   *string              `json:"title"`
	Content                 *string              `json:"content"`
	Category                *AnnouncementCategory `json:"category"`
	ScopeIDs                []string             `json:"scope_ids"`
	ScheduledAt             *time.Time           `json:"scheduled_at"`
	ExpiresAt               *time.Time           `json:"expires_at"`
	RequiresAcknowledgement *bool                `json:"requires_acknowledgement"`
	AcknowledgementDeadline *string              `json:"acknowledgement_deadline"`
	IsPinned                *bool                `json:"is_pinned"`
	PinOrder                *int                 `json:"pin_order"`
}

type AnnouncementListResponse struct {
	Announcements []*Announcement `json:"announcements"`
	Total         int             `json:"total"`
}

var (
	ErrNotFound        = errors.New("announcement not found")
	ErrTitleRequired   = errors.New("title is required")
	ErrContentRequired = errors.New("content is required")
	ErrInvalidCategory = errors.New("invalid category")
	ErrWrongStatus     = errors.New("action not allowed in current announcement status")
	ErrAlreadyPublished = errors.New("announcement is already published")
)
