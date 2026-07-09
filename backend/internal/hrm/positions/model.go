// backend/internal/hrm/positions/model.go
package positions

import "errors"
import "time"

// Position is the core domain type for a job position/title in HRM.
// Mirrors hrm_positions table columns exactly.
type Position struct {
	ID           string    `db:"id"            json:"id"`
	PublicID     string    `db:"public_id"     json:"public_id"`
	OrgID        string    `db:"org_id"        json:"org_id"`
	DepartmentID *string   `db:"department_id" json:"department_id,omitempty"`
	Title        string    `db:"title"         json:"title"`
	Description  *string   `db:"description"   json:"description,omitempty"`
	IsActive     bool      `db:"is_active"     json:"is_active"`
	CreatedBy    string    `db:"created_by"    json:"created_by"`
	CreatedAt    time.Time `db:"created_at"    json:"created_at"`
	UpdatedAt    time.Time `db:"updated_at"    json:"updated_at"`
}

// CreatePositionRequest is the body for POST /hrm/positions.
type CreatePositionRequest struct {
	Title        string  `json:"title"`
	Description  *string `json:"description"`
	DepartmentID *string `json:"department_id"` // optional
}

// UpdatePositionRequest is the body for PATCH /hrm/positions/:posId.
// All fields optional.
type UpdatePositionRequest struct {
	Title        *string `json:"title"`
	Description  *string `json:"description"`
	DepartmentID *string `json:"department_id"`
	IsActive     *bool   `json:"is_active"`
}

// PositionListResponse wraps list results.
type PositionListResponse struct {
	Positions []*Position `json:"positions"`
	Total     int         `json:"total"`
}

// Sentinel errors
var (
	ErrPositionNotFound = errors.New("position not found")
	ErrTitleRequired    = errors.New("title is required")
	ErrTitleTooLong     = errors.New("title must not exceed 150 characters")
	ErrTitleConflict    = errors.New("a position with this title already exists in the organization")
)
