// backend/internal/crm/deals/model.go
package deals

import (
	"errors"
	"time"
)

// DealStatus defines the allowed values for deal status.
type DealStatus string

const (
	DealStatusOpen DealStatus = "open"
	DealStatusWon  DealStatus = "won"
	DealStatusLost DealStatus = "lost"
)

func (s DealStatus) IsValid() bool {
	switch s {
	case DealStatusOpen, DealStatusWon, DealStatusLost:
		return true
	}
	return false
}

// Deal is the core CRM deal record.
type Deal struct {
	ID         string     `db:"id"          json:"id"`
	PublicID   string     `db:"public_id"   json:"public_id"`
	OrgID      string     `db:"org_id"      json:"org_id"`
	Title      string     `db:"title"       json:"title"`
	Value      float64    `db:"value"       json:"value"`
	Currency   string     `db:"currency"    json:"currency"`
	PipelineID string     `db:"pipeline_id" json:"pipeline_id"`
	StageID    string     `db:"stage_id"    json:"stage_id"`
	ContactID  *string    `db:"contact_id"  json:"contact_id,omitempty"`
	CompanyID  *string    `db:"company_id"  json:"company_id,omitempty"`
	Status     DealStatus `db:"status"      json:"status"`
	CloseDate  *time.Time `db:"close_date"  json:"close_date,omitempty"`
	LostReason *string    `db:"lost_reason" json:"lost_reason,omitempty"`
	OwnerID    *string    `db:"owner_id"    json:"owner_id,omitempty"`
	WonAt      *time.Time `db:"won_at"      json:"won_at,omitempty"`
	LostAt     *time.Time `db:"lost_at"     json:"lost_at,omitempty"`
	CreatedBy  string     `db:"created_by"  json:"created_by"`
	CreatedAt  time.Time  `db:"created_at"  json:"created_at"`
	UpdatedAt  time.Time  `db:"updated_at"  json:"updated_at"`
}

type CreateDealRequest struct {
	Title      string  `json:"title"`
	Value      float64 `json:"value"`
	Currency   *string `json:"currency"`
	PipelineID string  `json:"pipeline_id"`
	StageID    string  `json:"stage_id"`
	ContactID  *string `json:"contact_id"`
	CompanyID  *string `json:"company_id"`
	CloseDate  *string `json:"close_date"`
	OwnerID    *string `json:"owner_id"`
}

type UpdateDealRequest struct {
	Title     *string  `json:"title"`
	Value     *float64 `json:"value"`
	Currency  *string  `json:"currency"`
	ContactID *string  `json:"contact_id"`
	CompanyID *string  `json:"company_id"`
	CloseDate *string  `json:"close_date"`
	OwnerID   *string  `json:"owner_id"`
}

type MoveDealStageRequest struct {
	StageID string `json:"stage_id"`
}

type MarkLostRequest struct {
	LostReason *string `json:"lost_reason"`
}

type DealListResponse struct {
	Deals []*Deal `json:"deals"`
	Total int     `json:"total"`
}

// Board types for pipeline kanban view
type PipelineBoard struct {
	PipelineID   string            `json:"pipeline_id"`
	PipelineName string            `json:"pipeline_name"`
	Stages       []*StageWithDeals `json:"stages"`
}

type StageWithDeals struct {
	StageID   string  `json:"stage_id"`
	StageName string  `json:"stage_name"`
	Position  int     `json:"position"`
	Deals     []*Deal `json:"deals"`
	Total     int     `json:"total"`
}

// DealsByStage is used in reports.
type DealsByStage struct {
	StageID    string  `json:"stage_id"`
	StageName  string  `json:"stage_name"`
	Count      int     `json:"count"`
	TotalValue float64 `json:"total_value"`
}

// DealsByOwner is used in reports.
type DealsByOwner struct {
	OwnerID    *string `json:"owner_id"`
	OwnerName  string  `json:"owner_name"`
	Count      int     `json:"count"`
	TotalValue float64 `json:"total_value"`
}

// Sentinel errors
var (
	ErrDealNotFound       = errors.New("deal not found")
	ErrTitleRequired      = errors.New("title is required")
	ErrPipelineRequired   = errors.New("pipeline_id is required")
	ErrStageRequired      = errors.New("stage_id is required")
	ErrStageNotInPipeline = errors.New("stage does not belong to this deal's pipeline")
	ErrStageNotFound      = errors.New("stage not found")
)
