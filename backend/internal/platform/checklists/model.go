// backend/internal/platform/checklists/model.go
package checklists

import (
	"errors"
	"time"

	"github.com/mridha/businesssaas/pkg/pagination"
)

// ============================================================
// Enums
// ============================================================

// ChecklistType is the kind of process a template drives.
type ChecklistType string

const (
	ChecklistTypeOnboarding            ChecklistType = "onboarding"
	ChecklistTypeOffboarding           ChecklistType = "offboarding"
	ChecklistTypeProbationConfirmation ChecklistType = "probation_confirmation"
	ChecklistTypeTransferHandover      ChecklistType = "transfer_handover"
)

func (t ChecklistType) IsValid() bool {
	switch t {
	case ChecklistTypeOnboarding, ChecklistTypeOffboarding, ChecklistTypeProbationConfirmation, ChecklistTypeTransferHandover:
		return true
	}
	return false
}

// OwnerType drives who a template item resolves to. Mirrors
// hrm_approval_template_levels.approver_type's shape rather than literal
// hr/it/finance roles, which are not seeded system roles.
type OwnerType string

const (
	OwnerTypeSubject      OwnerType = "subject"
	OwnerTypeManager      OwnerType = "manager"
	OwnerTypeRole         OwnerType = "role"
	OwnerTypeSpecificUser OwnerType = "specific_user"
)

func (t OwnerType) IsValid() bool {
	switch t {
	case OwnerTypeSubject, OwnerTypeManager, OwnerTypeRole, OwnerTypeSpecificUser:
		return true
	}
	return false
}

// SubjectType is the polymorphic discriminator for an instance's subject.
// Deliberately narrow today — Phase 9 widens it.
type SubjectType string

const (
	SubjectTypeEmployee SubjectType = "employee"
)

func (t SubjectType) IsValid() bool {
	return t == SubjectTypeEmployee
}

// InstanceStatus is the overall status of a checklist instance.
type InstanceStatus string

const (
	InstanceStatusInProgress InstanceStatus = "in_progress"
	InstanceStatusCompleted  InstanceStatus = "completed"
	InstanceStatusCancelled  InstanceStatus = "cancelled"
)

// ItemStatus is the status of a single instance item.
type ItemStatus string

const (
	ItemStatusPending   ItemStatus = "pending"
	ItemStatusCompleted ItemStatus = "completed"
	ItemStatusSkipped   ItemStatus = "skipped"
)

func (s ItemStatus) IsTerminal() bool {
	return s == ItemStatusCompleted || s == ItemStatusSkipped
}

// ============================================================
// Templates
// ============================================================

// Template is an org-defined checklist configuration.
type Template struct {
	ID            string        `db:"id"             json:"id"`
	PublicID      string        `db:"public_id"       json:"public_id"`
	OrgID         string        `db:"org_id"          json:"org_id"`
	Name          string        `db:"name"            json:"name"`
	Description   *string       `db:"description"     json:"description,omitempty"`
	ChecklistType ChecklistType `db:"checklist_type"  json:"checklist_type"`
	IsDefault     bool          `db:"is_default"      json:"is_default"`
	IsActive      bool          `db:"is_active"       json:"is_active"`
	CreatedBy     string        `db:"created_by"      json:"created_by"`
	CreatedAt     time.Time     `db:"created_at"      json:"created_at"`
	UpdatedAt     time.Time     `db:"updated_at"      json:"updated_at"`
}

// TemplateItem is a single item within a template.
type TemplateItem struct {
	ID                 string    `db:"id"                   json:"id"`
	PublicID           string    `db:"public_id"            json:"public_id"`
	TemplateID         string    `db:"template_id"          json:"template_id"`
	Title              string    `db:"title"                json:"title"`
	Description        *string   `db:"description"          json:"description,omitempty"`
	OwnerType          OwnerType `db:"owner_type"           json:"owner_type"`
	OwnerRole          *string   `db:"owner_role"           json:"owner_role,omitempty"`
	OwnerUserID        *string   `db:"owner_user_id"        json:"owner_user_id,omitempty"`
	DueOffsetDays      int       `db:"due_offset_days"      json:"due_offset_days"`
	IsBlocking         bool      `db:"is_blocking"          json:"is_blocking"`
	RequiresAttachment bool      `db:"requires_attachment"  json:"requires_attachment"`
	DisplayOrder       int       `db:"display_order"        json:"display_order"`
	IsActive           bool      `db:"is_active"            json:"is_active"`
	CreatedAt          time.Time `db:"created_at"           json:"created_at"`
	UpdatedAt          time.Time `db:"updated_at"           json:"updated_at"`
	// OwnerRoleExists is populated on read only (not stored) so the UI can
	// flag a template item whose owner_role no longer names a live role —
	// the mitigation for the rename-fragility limitation documented in the
	// Phase 3 plan. Nil unless the caller asked for it to be resolved.
	OwnerRoleExists *bool `db:"-" json:"owner_role_exists,omitempty"`
}

// TemplateWithItems is a template plus its items, in display order.
type TemplateWithItems struct {
	*Template
	Items []*TemplateItem `json:"items"`
}

// CreateTemplateRequest creates a template together with its items in one
// call — a template with zero items can exist only transiently (Instantiate
// rejects it), so item authoring is not a separate two-step flow.
type CreateTemplateRequest struct {
	Name          string                      `json:"name"`
	Description   *string                     `json:"description"`
	ChecklistType ChecklistType               `json:"checklist_type"`
	IsDefault     bool                        `json:"is_default"`
	Items         []CreateTemplateItemRequest `json:"items"`
}

type CreateTemplateItemRequest struct {
	Title              string    `json:"title"`
	Description        *string   `json:"description"`
	OwnerType          OwnerType `json:"owner_type"`
	OwnerRole          *string   `json:"owner_role"`
	OwnerUserID        *string   `json:"owner_user_id"`
	DueOffsetDays      int       `json:"due_offset_days"`
	IsBlocking         bool      `json:"is_blocking"`
	RequiresAttachment bool      `json:"requires_attachment"`
	DisplayOrder       int       `json:"display_order"`
}

type UpdateTemplateRequest struct {
	Name        *string `json:"name"`
	Description *string `json:"description"`
	IsDefault   *bool   `json:"is_default"`
	IsActive    *bool   `json:"is_active"`
}

type UpdateTemplateItemRequest struct {
	Title              *string    `json:"title"`
	Description        *string    `json:"description"`
	OwnerType          *OwnerType `json:"owner_type"`
	OwnerRole          *string    `json:"owner_role"`
	OwnerUserID        *string    `json:"owner_user_id"`
	DueOffsetDays      *int       `json:"due_offset_days"`
	IsBlocking         *bool      `json:"is_blocking"`
	RequiresAttachment *bool      `json:"requires_attachment"`
	DisplayOrder       *int       `json:"display_order"`
	IsActive           *bool      `json:"is_active"`
}

// ============================================================
// Instances
// ============================================================

// SubjectContext is everything the engine needs to resolve owners without
// knowing what a "subject" is. The consuming module (e.g. hrm/onboarding)
// resolves these from its own tables — this is what keeps this package free
// of any hrm_* dependency, and what prevents an HTTP caller from choosing
// its own assignees (there is deliberately no generic instantiate route).
type SubjectContext struct {
	SubjectType   SubjectType
	SubjectID     string
	SubjectLabel  string
	SubjectUserID *string // nil when the subject has no platform account
	ManagerUserID *string // nil when there is no manager, or the manager has no platform account
	AnchorDate    time.Time
	CreatedBy     string
}

// Instance is a template applied to a subject.
type Instance struct {
	ID            string         `db:"id"              json:"id"`
	PublicID      string         `db:"public_id"       json:"public_id"`
	OrgID         string         `db:"org_id"          json:"org_id"`
	TemplateID    *string        `db:"template_id"     json:"template_id,omitempty"`
	TemplateName  string         `db:"template_name"   json:"template_name"`
	ChecklistType ChecklistType  `db:"checklist_type"  json:"checklist_type"`
	SubjectType   SubjectType    `db:"subject_type"    json:"subject_type"`
	SubjectID     string         `db:"subject_id"      json:"subject_id"`
	SubjectLabel  string         `db:"subject_label"   json:"subject_label"`
	SubjectUserID *string        `db:"subject_user_id" json:"subject_user_id,omitempty"`
	AnchorDate    time.Time      `db:"anchor_date"     json:"anchor_date"`
	Status        InstanceStatus `db:"status"          json:"status"`
	CompletedAt   *time.Time     `db:"completed_at"    json:"completed_at,omitempty"`
	CancelledAt   *time.Time     `db:"cancelled_at"    json:"cancelled_at,omitempty"`
	CancelReason  *string        `db:"cancel_reason"   json:"cancel_reason,omitempty"`
	CreatedBy     string         `db:"created_by"      json:"created_by"`
	CreatedAt     time.Time      `db:"created_at"      json:"created_at"`
	UpdatedAt     time.Time      `db:"updated_at"      json:"updated_at"`
}

// InstanceItem is per-item runtime state for an instance — a column-level
// snapshot of the template item at instantiation time, not a live join.
type InstanceItem struct {
	ID                 string     `db:"id"                  json:"id"`
	PublicID           string     `db:"public_id"           json:"public_id"`
	InstanceID         string     `db:"instance_id"         json:"instance_id"`
	TemplateItemID     *string    `db:"template_item_id"    json:"template_item_id,omitempty"`
	Title              string     `db:"title"               json:"title"`
	Description        *string    `db:"description"         json:"description,omitempty"`
	OwnerType          OwnerType  `db:"owner_type"          json:"owner_type"`
	OwnerRole          *string    `db:"owner_role"          json:"owner_role,omitempty"`
	IsBlocking         bool       `db:"is_blocking"         json:"is_blocking"`
	RequiresAttachment bool       `db:"requires_attachment" json:"requires_attachment"`
	DisplayOrder       int        `db:"display_order"       json:"display_order"`
	DueOffsetDays      int        `db:"due_offset_days"     json:"due_offset_days"`
	AssigneeUserID     *string    `db:"assignee_user_id"    json:"assignee_user_id,omitempty"`
	DueDate            *time.Time `db:"due_date"            json:"due_date,omitempty"`
	Status             ItemStatus `db:"status"              json:"status"`
	CompletedBy        *string    `db:"completed_by"        json:"completed_by,omitempty"`
	CompletedAt        *time.Time `db:"completed_at"        json:"completed_at,omitempty"`
	CompletionNote     *string    `db:"completion_note"     json:"completion_note,omitempty"`
	AttachmentURL      *string    `db:"attachment_url"      json:"attachment_url,omitempty"`
	SkipReason         *string    `db:"skip_reason"         json:"skip_reason,omitempty"`
	CreatedAt          time.Time  `db:"created_at"          json:"created_at"`
	UpdatedAt          time.Time  `db:"updated_at"          json:"updated_at"`
	// IsUnassigned is a derived, never-stored flag: true when the item
	// resolved to no assignee at all (a role item is NOT unassigned — any
	// role holder can claim it; this is only true when resolution genuinely
	// found nobody). Lets the UI flag the fail-soft consequence of decision 2.
	IsUnassigned bool `db:"-" json:"is_unassigned"`
}

// InstanceWithItems is an instance plus its items, in display order.
type InstanceWithItems struct {
	*Instance
	Items    []*InstanceItem `json:"items"`
	Progress *Progress       `json:"progress"`
}

// Progress is computed from instance items — never stored.
type Progress struct {
	InstanceID     string `json:"instance_id"`
	TotalItems     int    `json:"total_items"`
	CompletedItems int    `json:"completed_items"`
	SkippedItems   int    `json:"skipped_items"`
	PendingItems   int    `json:"pending_items"`
	BlockingOpen   int    `json:"blocking_open"`
	PercentDone    int    `json:"percent_done"` // 0-100, rounded
}

// InstantiateResult is returned by Instantiate/InstantiateDefault so callers
// can log unresolved-owner counts without the engine importing a logger.
type InstantiateResult struct {
	Instance        *Instance
	Items           []*InstanceItem
	UnresolvedCount int
}

// InstanceListItem pairs an instance with its computed progress for list
// views — GetProgressBatch avoids an N+1 here.
type InstanceListItem struct {
	*Instance
	Progress *Progress `json:"progress"`
}

type InstanceListResponse struct {
	Instances []*InstanceListItem `json:"instances"`
	Meta      pagination.Meta     `json:"meta"`
}

type InstanceFilter struct {
	SubjectType *SubjectType
	SubjectID   *string
	Status      *InstanceStatus
	Limit       int
	Offset      int
}

type CancelInstanceRequest struct {
	Reason string `json:"reason"`
}

type CompleteItemRequest struct {
	CompletionNote *string `json:"completion_note"`
	AttachmentURL  *string `json:"attachment_url"`
}

type SkipItemRequest struct {
	Reason string `json:"reason"`
}

// ============================================================
// Sentinel errors
// ============================================================

var (
	ErrTemplateNotFound     = errors.New("checklist template not found")
	ErrTemplateItemNotFound = errors.New("checklist template item not found")
	ErrInstanceNotFound     = errors.New("checklist instance not found")
	ErrInstanceItemNotFound = errors.New("checklist instance item not found")

	ErrNameRequired          = errors.New("name is required")
	ErrTitleRequired         = errors.New("title is required")
	ErrInvalidChecklistType  = errors.New("invalid checklist_type")
	ErrInvalidOwnerType      = errors.New("invalid owner_type")
	ErrInvalidSubjectType    = errors.New("invalid subject_type")
	ErrOwnerRoleRequired     = errors.New("owner_role is required when owner_type is 'role'")
	ErrOwnerUserRequired     = errors.New("owner_user_id is required when owner_type is 'specific_user'")
	ErrUnknownRole           = errors.New("owner_role does not name an existing role")
	ErrTemplateHasNoItems    = errors.New("template has no active items")
	ErrTemplateInactive      = errors.New("template is not active")
	ErrNotItemOwner          = errors.New("caller is not authorized to act on this item")
	ErrItemAlreadyTerminal   = errors.New("item is already completed or skipped")
	ErrItemNotTerminal       = errors.New("item is not completed or skipped")
	ErrSkipReasonRequired    = errors.New("reason is required to skip an item")
	ErrAttachmentRequired    = errors.New("this item requires an attachment before it can be completed")
	ErrInstanceAlreadyClosed = errors.New("instance is already completed or cancelled")
)
