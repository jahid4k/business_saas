// backend/internal/hrm/terminations/model.go
package terminations

import (
	"errors"
	"time"
)

// TerminationType is always HR-initiated — not employee self-service.
// Use hrm_resignations for employee-initiated departures.
type TerminationType string

const (
	TypeVoluntary     TerminationType = "voluntary"
	TypeInvoluntary   TerminationType = "involuntary"
	TypeLayoff        TerminationType = "layoff"
	TypeRetirement    TerminationType = "retirement"
	TypeContractEnd   TerminationType = "contract_end"
	TypeProbationFail TerminationType = "probation_fail"
)

func (t TerminationType) IsValid() bool {
	switch t {
	case TypeVoluntary, TypeInvoluntary, TypeLayoff, TypeRetirement, TypeContractEnd, TypeProbationFail:
		return true
	}
	return false
}

type TerminationStatus string

const (
	StatusDraft           TerminationStatus = "draft"
	StatusPendingApproval TerminationStatus = "pending_approval"
	StatusApproved        TerminationStatus = "approved"
	StatusRejected        TerminationStatus = "rejected"
	StatusCancelled       TerminationStatus = "cancelled"
	StatusApplied         TerminationStatus = "applied"
)

// Termination is an HR-initiated employee departure record.
// When applied:
//   - employee.status         = 'terminated'
//   - employee.termination_date = last_working_date
type Termination struct {
	ID                     string            `db:"id"                        json:"id"`
	PublicID               string            `db:"public_id"                 json:"public_id"`
	OrgID                  string            `db:"org_id"                    json:"org_id"`
	EmployeeID             string            `db:"employee_id"               json:"employee_id"`
	TerminationType        TerminationType   `db:"termination_type"          json:"termination_type"`
	TerminationDate        string            `db:"termination_date"          json:"termination_date"`
	LastWorkingDate        string            `db:"last_working_date"         json:"last_working_date"`
	Reason                 *string           `db:"reason"                    json:"reason,omitempty"`
	InternalNotes          *string           `db:"internal_notes"            json:"internal_notes,omitempty"`
	ApprovalInstanceID     *string           `db:"approval_instance_id"      json:"approval_instance_id,omitempty"`
	DocumentID             *string           `db:"document_id"               json:"document_id,omitempty"`
	SeveranceAmount        *float64          `db:"severance_amount"          json:"severance_amount,omitempty"`
	SeveranceCurrency      string            `db:"severance_currency"        json:"severance_currency"`
	IsRehireEligible       bool              `db:"is_rehire_eligible"        json:"is_rehire_eligible"`
	ExitClearanceCompleted bool              `db:"exit_clearance_completed"  json:"exit_clearance_completed"`
	Status                 TerminationStatus `db:"status"                    json:"status"`
	AppliedAt              *time.Time        `db:"applied_at"                json:"applied_at,omitempty"`
	AppliedBy              *string           `db:"applied_by"                json:"applied_by,omitempty"`
	CreatedBy              string            `db:"created_by"                json:"created_by"`
	CreatedAt              time.Time         `db:"created_at"                json:"created_at"`
	UpdatedAt              time.Time         `db:"updated_at"                json:"updated_at"`
}

type CreateTerminationRequest struct {
	TerminationType   TerminationType `json:"termination_type"`
	TerminationDate   string          `json:"termination_date"`
	LastWorkingDate   string          `json:"last_working_date"`
	Reason            *string         `json:"reason"`
	InternalNotes     *string         `json:"internal_notes"`
	SeveranceAmount   *float64        `json:"severance_amount"`
	SeveranceCurrency *string         `json:"severance_currency"`
	IsRehireEligible  *bool           `json:"is_rehire_eligible"` // defaults true
}

type UpdateTerminationRequest struct {
	TerminationDate        *string  `json:"termination_date"`
	LastWorkingDate        *string  `json:"last_working_date"`
	Reason                 *string  `json:"reason"`
	InternalNotes          *string  `json:"internal_notes"`
	SeveranceAmount        *float64 `json:"severance_amount"`
	IsRehireEligible       *bool    `json:"is_rehire_eligible"`
	ExitClearanceCompleted *bool    `json:"exit_clearance_completed"`
	DocumentID             *string  `json:"document_id"`
}

type TerminationListResponse struct {
	Terminations []*Termination `json:"terminations"`
	Total        int            `json:"total"`
}

var (
	ErrNotFound                 = errors.New("termination not found")
	ErrAlreadyActiveTermination = errors.New("employee already has an active termination record — cancel it first")
	ErrInvalidTerminationType   = errors.New("invalid termination_type")
	ErrTerminationDateRequired  = errors.New("termination_date is required (YYYY-MM-DD)")
	ErrLastWorkingDateRequired  = errors.New("last_working_date is required (YYYY-MM-DD)")
	ErrInvalidDate              = errors.New("date must be a valid YYYY-MM-DD")
	ErrWrongStatus              = errors.New("action not allowed in current termination status")
	ErrAlreadyApplied           = errors.New("termination has already been applied")
	ErrNotApproved              = errors.New("termination must be approved before applying")
)
