// backend/internal/hrm/recruitment/referrals_model.go
package recruitment

import (
	"errors"
	"time"

	"github.com/shopspring/decimal"
)

type ReferralStatus string

const (
	ReferralStatusSubmitted      ReferralStatus = "submitted"
	ReferralStatusCandidateHired ReferralStatus = "candidate_hired"
	ReferralStatusBonusPending   ReferralStatus = "bonus_pending"
	ReferralStatusBonusPaid      ReferralStatus = "bonus_paid"
	ReferralStatusNotEligible    ReferralStatus = "not_eligible"
)

func (s ReferralStatus) IsValid() bool {
	switch s {
	case ReferralStatusSubmitted, ReferralStatusCandidateHired, ReferralStatusBonusPending,
		ReferralStatusBonusPaid, ReferralStatusNotEligible:
		return true
	}
	return false
}

// Referral is the formal bonus-program lifecycle for an employee referring
// a candidate — distinct from hrm_candidates.referred_by_employee_id, which
// is lightweight provenance kept regardless of whether a bonus applies.
type Referral struct {
	ID                   string           `db:"id"                       json:"id"`
	PublicID             string           `db:"public_id"                json:"public_id"`
	OrgID                string           `db:"org_id"                   json:"org_id"`
	CandidateID          string           `db:"candidate_id"             json:"candidate_id"`
	ReferredByEmployeeID *string          `db:"referred_by_employee_id"  json:"referred_by_employee_id,omitempty"`
	ApplicationID        *string          `db:"application_id"           json:"application_id,omitempty"`
	Status               ReferralStatus   `db:"status"                   json:"status"`
	BonusAmount          *decimal.Decimal `db:"bonus_amount"             json:"bonus_amount,omitempty"`
	BonusCurrency        string           `db:"bonus_currency"           json:"bonus_currency"`
	PaidAt               *time.Time       `db:"paid_at"                  json:"paid_at,omitempty"`
	Notes                *string          `db:"notes"                    json:"notes,omitempty"`
	CreatedBy            string           `db:"created_by"               json:"created_by"`
	CreatedAt            time.Time        `db:"created_at"                json:"created_at"`
	UpdatedAt            time.Time        `db:"updated_at"                json:"updated_at"`
}

type CreateReferralRequest struct {
	CandidateID          string  `json:"candidate_id"`
	ReferredByEmployeeID *string `json:"referred_by_employee_id"`
	ApplicationID        *string `json:"application_id"`
	Notes                *string `json:"notes"`
}

// UpdateReferralRequest covers the manual lifecycle transitions — no
// scheduler job exists to move status/paid_at automatically (see migration
// 00080's header and the phase plan's known limitations).
type UpdateReferralRequest struct {
	Status        *string          `json:"status"`
	BonusAmount   *decimal.Decimal `json:"bonus_amount"`
	BonusCurrency *string          `json:"bonus_currency"`
	Notes         *string          `json:"notes"`
}

type ReferralListFilter struct {
	CandidateID string
	Status      string
	Limit       int
	Offset      int
}

func (f *ReferralListFilter) Normalise() {
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

type ReferralListResponse struct {
	Referrals []*Referral `json:"referrals"`
	Total     int         `json:"total"`
	Limit     int         `json:"limit"`
	Offset    int         `json:"offset"`
}

var (
	ErrReferralNotFound          = errors.New("referral not found")
	ErrReferralCandidateRequired = errors.New("candidate_id is required")
	ErrInvalidReferralStatus     = errors.New("invalid referral status")
)
