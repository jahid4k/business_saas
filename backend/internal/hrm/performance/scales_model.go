// backend/internal/hrm/performance/scales_model.go
package performance

import (
	"errors"
	"time"

	"github.com/shopspring/decimal"
)

// RatingScale is an org-configured rating vocabulary. Its levels carry the
// numeric anchors Phase 7's merit matrix and Phase 10's 9-box read.
type RatingScale struct {
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

	Levels []*RatingLevel `db:"-" json:"levels,omitempty"`
}

// RatingLevel is one step on a scale. Value is what makes a rating orderable
// and computable — a label alone is neither.
type RatingLevel struct {
	ID           string          `db:"id"            json:"id"`
	PublicID     string          `db:"public_id"     json:"public_id"`
	ScaleID      string          `db:"scale_id"      json:"scale_id"`
	Label        string          `db:"label"         json:"label"`
	Description  *string         `db:"description"   json:"description,omitempty"`
	Value        decimal.Decimal `db:"value"         json:"value"`
	DisplayOrder int             `db:"display_order" json:"display_order"`
	Color        *string         `db:"color"         json:"color,omitempty"`
	CreatedAt    time.Time       `db:"created_at"    json:"created_at"`
	UpdatedAt    time.Time       `db:"updated_at"    json:"updated_at"`
}

type CreateScaleRequest struct {
	Name        string  `json:"name"`
	Description *string `json:"description"`
	IsDefault   bool    `json:"is_default"`
}

type UpdateScaleRequest struct {
	Name        *string `json:"name"`
	Description *string `json:"description"`
	IsDefault   *bool   `json:"is_default"`
	IsActive    *bool   `json:"is_active"`
}

type CreateLevelRequest struct {
	Label        string          `json:"label"`
	Description  *string         `json:"description"`
	Value        decimal.Decimal `json:"value"`
	DisplayOrder *int            `json:"display_order"`
	Color        *string         `json:"color"`
}

type UpdateLevelRequest struct {
	Label        *string          `json:"label"`
	Description  *string          `json:"description"`
	Value        *decimal.Decimal `json:"value"`
	DisplayOrder *int             `json:"display_order"`
	Color        *string          `json:"color"`
}

var (
	ErrScaleNotFound     = errors.New("rating scale not found")
	ErrLevelNotFound     = errors.New("rating level not found")
	ErrScaleNameRequired = errors.New("name is required")
	ErrScaleNameTaken    = errors.New("a rating scale with this name already exists in this organization")
	ErrLevelLabelReq     = errors.New("label is required")
	ErrLevelLabelTaken   = errors.New("a level with this label already exists on this scale")
	// ErrScaleInUse blocks deleting a scale an appraisal cycle references.
	// The FK is RESTRICT; this turns a raw 23503 into a usable message.
	ErrScaleInUse    = errors.New("this rating scale is used by an appraisal cycle and cannot be deleted")
	ErrScaleNoLevels = errors.New("a rating scale needs at least one level before a cycle can use it")
)
