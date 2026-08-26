// backend/internal/hrm/approvals/model.go
package approvals

import (
	"encoding/json"
	"errors"
	"time"
)

// ActionType is the type of entity this approval template covers.
type ActionType string

const (
	ActionTypeLeave                    ActionType = "leave"
	ActionTypeResignation              ActionType = "resignation"
	ActionTypePromotion                ActionType = "promotion"
	ActionTypeTransfer                 ActionType = "transfer"
	ActionTypeWarning                  ActionType = "warning"
	ActionTypeDocument                 ActionType = "document"
	ActionTypeTermination              ActionType = "termination"
	ActionTypeAttendanceRegularization ActionType = "attendance_regularization"
	ActionTypeAward                    ActionType = "award"
	ActionTypeJobRequisition           ActionType = "job_requisition"
	ActionTypeOffer                    ActionType = "offer"
	ActionTypeCustom                   ActionType = "custom"
	// ActionTypeSalaryRevision and ActionTypeBonus back Phase 7B — see
	// migration 00098's header on why the template (short form) and instance
	// (long form) CHECKs are separate and both had to be widened.
	ActionTypeSalaryRevision ActionType = "salary_revision"
	ActionTypeBonus          ActionType = "bonus"
	// ActionTypeLoan and ActionTypeReimbursement back Phase 7C — same
	// two-CHECK widening, see migration 00100's header.
	ActionTypeLoan          ActionType = "loan"
	ActionTypeReimbursement ActionType = "reimbursement"
)

func (a ActionType) IsValid() bool {
	switch a {
	case ActionTypeLeave, ActionTypeResignation, ActionTypePromotion, ActionTypeTransfer,
		ActionTypeWarning, ActionTypeDocument, ActionTypeTermination,
		ActionTypeAttendanceRegularization, ActionTypeAward, ActionTypeJobRequisition, ActionTypeOffer, ActionTypeCustom,
		ActionTypeSalaryRevision, ActionTypeBonus, ActionTypeLoan, ActionTypeReimbursement:
		return true
	}
	return false
}

type ApproverType string

const (
	ApproverTypeReportingManager ApproverType = "reporting_manager"
	ApproverTypeDeptHead         ApproverType = "dept_head"
	ApproverTypeRole             ApproverType = "role"
	ApproverTypeSpecificUser     ApproverType = "specific_user"
)

func (t ApproverType) IsValid() bool {
	switch t {
	case ApproverTypeReportingManager, ApproverTypeDeptHead, ApproverTypeRole, ApproverTypeSpecificUser:
		return true
	}
	return false
}

type SLABreachAction string

const (
	SLABreachEscalateNext SLABreachAction = "escalate_next"
	SLABreachAutoApprove  SLABreachAction = "auto_approve"
	SLABreachAutoReject   SLABreachAction = "auto_reject"
)

type InstanceStatus string

const (
	InstanceStatusPending   InstanceStatus = "pending"
	InstanceStatusApproved  InstanceStatus = "approved"
	InstanceStatusRejected  InstanceStatus = "rejected"
	InstanceStatusCancelled InstanceStatus = "cancelled"
)

// ApprovalTemplate is an org-configured approval chain for a given action type.
type ApprovalTemplate struct {
	ID                  string     `db:"id"                   json:"id"`
	PublicID            string     `db:"public_id"            json:"public_id"`
	OrgID               string     `db:"org_id"               json:"org_id"`
	Name                string     `db:"name"                 json:"name"`
	Description         *string    `db:"description"          json:"description,omitempty"`
	ActionType          ActionType `db:"action_type"          json:"action_type"`
	ConditionExpression *string    `db:"condition_expression" json:"condition_expression,omitempty"`
	IsDefault           bool       `db:"is_default"           json:"is_default"`
	IsActive            bool       `db:"is_active"            json:"is_active"`
	CreatedBy           string     `db:"created_by"           json:"created_by"`
	CreatedAt           time.Time  `db:"created_at"           json:"created_at"`
	UpdatedAt           time.Time  `db:"updated_at"           json:"updated_at"`

	Levels []*ApprovalTemplateLevel `db:"-" json:"levels,omitempty"`
}

// ApprovalTemplateLevel is one sequential level within a template.
type ApprovalTemplateLevel struct {
	ID             string          `db:"id"               json:"id"`
	TemplateID     string          `db:"template_id"      json:"template_id"`
	Level          int             `db:"level"            json:"level"`
	ApproverType   ApproverType    `db:"approver_type"    json:"approver_type"`
	ApproverRole   *string         `db:"approver_role"    json:"approver_role,omitempty"`
	ApproverUserID *string         `db:"approver_user_id" json:"approver_user_id,omitempty"`
	SLAHours       int             `db:"sla_hours"        json:"sla_hours"`
	OnSLABreach    SLABreachAction `db:"on_sla_breach"    json:"on_sla_breach"`
}

// ApprovalInstance is a runtime approval workflow for a specific entity.
type ApprovalInstance struct {
	ID               string         `db:"id"                json:"id"`
	PublicID         string         `db:"public_id"         json:"public_id"`
	OrgID            string         `db:"org_id"            json:"org_id"`
	TemplateID       *string        `db:"template_id"       json:"template_id,omitempty"`
	EntityType       string         `db:"entity_type"       json:"entity_type"`
	EntityID         string         `db:"entity_id"         json:"entity_id"`
	InstanceSnapshot []byte         `db:"instance_snapshot" json:"-"`
	CurrentLevel     int            `db:"current_level"     json:"current_level"`
	OverallStatus    InstanceStatus `db:"overall_status"    json:"overall_status"`
	RequestedBy      string         `db:"requested_by"      json:"requested_by"`
	CreatedAt        time.Time      `db:"created_at"        json:"created_at"`
	UpdatedAt        time.Time      `db:"updated_at"        json:"updated_at"`
	CompletedAt      *time.Time     `db:"completed_at"      json:"completed_at,omitempty"`

	Snapshot  []*ApprovalTemplateLevel `db:"-" json:"snapshot,omitempty"`
	Decisions []*ApprovalDecision      `db:"-" json:"decisions,omitempty"`
}

func (a *ApprovalInstance) ParseSnapshot() error {
	if len(a.InstanceSnapshot) == 0 {
		a.Snapshot = []*ApprovalTemplateLevel{}
		return nil
	}
	return json.Unmarshal(a.InstanceSnapshot, &a.Snapshot)
}

// ApprovalDecision is a single approver's action on an instance level.
type ApprovalDecision struct {
	ID         string    `db:"id"          json:"id"`
	InstanceID string    `db:"instance_id" json:"instance_id"`
	Level      int       `db:"level"       json:"level"`
	ApproverID string    `db:"approver_id" json:"approver_id"`
	Action     string    `db:"action"      json:"action"` // approved | rejected | cancelled
	Note       *string   `db:"note"        json:"note,omitempty"`
	DecidedAt  time.Time `db:"decided_at"  json:"decided_at"`
}

// ── Request types ──

type CreateTemplateRequest struct {
	Name                string                       `json:"name"`
	Description         *string                      `json:"description"`
	ActionType          ActionType                   `json:"action_type"`
	ConditionExpression *string                      `json:"condition_expression"`
	IsDefault           bool                         `json:"is_default"`
	Levels              []CreateTemplateLevelRequest `json:"levels"`
}

type CreateTemplateLevelRequest struct {
	Level          int             `json:"level"`
	ApproverType   ApproverType    `json:"approver_type"`
	ApproverRole   *string         `json:"approver_role"`
	ApproverUserID *string         `json:"approver_user_id"`
	SLAHours       int             `json:"sla_hours"`
	OnSLABreach    SLABreachAction `json:"on_sla_breach"`
}

type UpdateTemplateRequest struct {
	Name                *string `json:"name"`
	Description         *string `json:"description"`
	ConditionExpression *string `json:"condition_expression"`
	IsDefault           *bool   `json:"is_default"`
	IsActive            *bool   `json:"is_active"`
}

// CreateInstanceRequest is called by other services (not directly by HTTP —
// the handler is for internal use by leave, promotion, etc.).
type CreateInstanceRequest struct {
	TemplateID  string
	EntityType  string
	EntityID    string
	RequestedBy string
}

type DecisionRequest struct {
	Action string  `json:"action"` // approved | rejected | cancelled
	Note   *string `json:"note"`
}

// ── List responses ──

type TemplateListResponse struct {
	Templates []*ApprovalTemplate `json:"templates"`
	Total     int                 `json:"total"`
}

type InstanceListResponse struct {
	Instances []*ApprovalInstance `json:"instances"`
	Total     int                 `json:"total"`
}

// ── Sentinel errors ──

var (
	ErrTemplateNotFound    = errors.New("approval template not found")
	ErrInstanceNotFound    = errors.New("approval instance not found")
	ErrNameRequired        = errors.New("name is required")
	ErrInvalidActionType   = errors.New("invalid action_type")
	ErrNoLevels            = errors.New("at least one level is required")
	ErrInvalidLevel        = errors.New("level must be >= 1 and sequential")
	ErrInvalidApproverType = errors.New("invalid approver_type")
	ErrAlreadyCompleted    = errors.New("approval instance is already completed")
	ErrNotCurrentLevel     = errors.New("action does not apply to the current approval level")
)
