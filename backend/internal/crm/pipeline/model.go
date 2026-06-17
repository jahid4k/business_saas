// backend/internal/crm/pipeline/model.go
package pipeline

import (
	"errors"
	"time"
)

// Pipeline represents a sales process with ordered stages.
type Pipeline struct {
	ID          string    `db:"id"          json:"id"`
	PublicID    string    `db:"public_id"   json:"public_id"`
	OrgID       string    `db:"org_id"      json:"org_id"`
	Name        string    `db:"name"        json:"name"`
	Description *string   `db:"description" json:"description,omitempty"`
	IsDefault   bool      `db:"is_default"  json:"is_default"`
	CreatedBy   string    `db:"created_by"  json:"created_by"`
	CreatedAt   time.Time `db:"created_at"  json:"created_at"`
	UpdatedAt   time.Time `db:"updated_at"  json:"updated_at"`
	Stages      []*Stage  `db:"-"           json:"stages,omitempty"`
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
}

type PipelineListResponse struct {
	Pipelines []*Pipeline `json:"pipelines"`
	Total     int         `json:"total"`
}

// Stage represents a step within a pipeline.
type Stage struct {
	ID          string    `db:"id"          json:"id"`
	PublicID    string    `db:"public_id"   json:"public_id"`
	OrgID       string    `db:"org_id"      json:"org_id"`
	PipelineID  string    `db:"pipeline_id" json:"pipeline_id"`
	Name        string    `db:"name"        json:"name"`
	Position    int       `db:"position"    json:"position"`
	Probability int       `db:"probability" json:"probability"`
	CreatedAt   time.Time `db:"created_at"  json:"created_at"`
	UpdatedAt   time.Time `db:"updated_at"  json:"updated_at"`
}

type CreateStageRequest struct {
	Name        string `json:"name"`
	Position    *int   `json:"position"`
	Probability *int   `json:"probability"`
}

type UpdateStageRequest struct {
	Name        *string `json:"name"`
	Position    *int    `json:"position"`
	Probability *int    `json:"probability"`
}

type ReorderStagesRequest struct {
	StageIDs []string `json:"stage_ids"`
}

type StageListResponse struct {
	Stages []*Stage `json:"stages"`
	Total  int      `json:"total"`
}

// Sentinel errors
var (
	ErrPipelineNotFound   = errors.New("pipeline not found")
	ErrStageNotFound      = errors.New("stage not found")
	ErrNameRequired       = errors.New("name is required")
	ErrStageNotInPipeline = errors.New("stage does not belong to this pipeline")
)
