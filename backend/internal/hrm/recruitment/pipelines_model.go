// backend/internal/hrm/recruitment/pipelines_model.go
package recruitment

import (
	"errors"
	"time"
)

// StageKind marks what a stage semantically means, beyond its position.
// crm_pipeline_stages has no equivalent (crm_deals.status carries win/loss
// instead) — Phase 4B's hire conversion needs to know which stage means
// hired, and rejection needs a trigger point, so it lives on the stage here.
type StageKind string

const (
	StageKindApplied    StageKind = "applied"
	StageKindInProgress StageKind = "in_progress"
	StageKindOffer      StageKind = "offer"
	StageKindHired      StageKind = "hired"
	StageKindRejected   StageKind = "rejected"
)

func (k StageKind) IsValid() bool {
	switch k {
	case StageKindApplied, StageKindInProgress, StageKindOffer, StageKindHired, StageKindRejected:
		return true
	}
	return false
}

func (k StageKind) IsTerminal() bool {
	return k == StageKindHired || k == StageKindRejected
}

// Pipeline mirrors crm_pipelines but enforces one default per org via
// uq_hrm_rpipe_default — a guard crm_pipelines never got.
type Pipeline struct {
	ID          string    `db:"id"          json:"id"`
	PublicID    string    `db:"public_id"   json:"public_id"`
	OrgID       string    `db:"org_id"      json:"org_id"`
	Name        string    `db:"name"        json:"name"`
	Description *string   `db:"description" json:"description,omitempty"`
	IsDefault   bool      `db:"is_default"  json:"is_default"`
	IsActive    bool      `db:"is_active"   json:"is_active"`
	CreatedBy   string    `db:"created_by"  json:"created_by"`
	CreatedAt   time.Time `db:"created_at"  json:"created_at"`
	UpdatedAt   time.Time `db:"updated_at"  json:"updated_at"`

	Stages []*Stage `db:"-" json:"stages,omitempty"`
}

type Stage struct {
	ID         string    `db:"id"          json:"id"`
	PublicID   string    `db:"public_id"   json:"public_id"`
	OrgID      string    `db:"org_id"      json:"org_id"`
	PipelineID string    `db:"pipeline_id" json:"pipeline_id"`
	Name       string    `db:"name"        json:"name"`
	Position   int       `db:"position"    json:"position"`
	StageKind  StageKind `db:"stage_kind"  json:"stage_kind"`
	CreatedAt  time.Time `db:"created_at"  json:"created_at"`
	UpdatedAt  time.Time `db:"updated_at"  json:"updated_at"`
}

type CreatePipelineRequest struct {
	Name        string  `json:"name"`
	Description *string `json:"description"`
	IsDefault   bool    `json:"is_default"`
}

type UpdatePipelineRequest struct {
	Name        *string `json:"name"`
	Description *string `json:"description"`
	IsDefault   *bool   `json:"is_default"`
	IsActive    *bool   `json:"is_active"`
}

type CreateStageRequest struct {
	Name      string  `json:"name"`
	StageKind *string `json:"stage_kind"`
	Position  *int    `json:"position"`
}

type UpdateStageRequest struct {
	Name      *string `json:"name"`
	StageKind *string `json:"stage_kind"`
}

type ReorderStagesRequest struct {
	StageIDs []string `json:"stage_ids"`
}

var (
	ErrPipelineNotFound   = errors.New("recruitment pipeline not found")
	ErrStageNotFound      = errors.New("recruitment stage not found")
	ErrPipelineNameReq    = errors.New("name is required")
	ErrStageNameReq       = errors.New("name is required")
	ErrInvalidStageKind   = errors.New("stage_kind must be one of: applied, in_progress, offer, hired, rejected")
	ErrStageNotInPipeline = errors.New("stage does not belong to this pipeline")
)
