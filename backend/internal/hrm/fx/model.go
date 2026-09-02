// backend/internal/hrm/fx/model.go
package fx

import (
	"errors"
	"time"

	"github.com/shopspring/decimal"
)

var (
	ErrAccessDenied  = errors.New("fx: access denied")
	ErrRateNotFound  = errors.New("fx: exchange rate not found")
	ErrDuplicateRate = errors.New("fx: a rate for this pair and date already exists")
	ErrInvalidSource = errors.New("fx: source must be manual or import")
)

// ExchangeRate is one recorded rate: what one unit of from_currency was worth
// in to_currency on a given date.
//
// ⚠ There is no updated_at, and that is deliberate. A rate for a given day is
// a historical fact about two currencies. Correcting one means recording the
// corrected value, not editing the record of what was believed at the time —
// and every figure already converted keeps the rate it actually used.
type ExchangeRate struct {
	ID           string          `json:"id"`
	PublicID     string          `json:"public_id"`
	OrgID        string          `json:"org_id"`
	FromCurrency string          `json:"from_currency"`
	ToCurrency   string          `json:"to_currency"`
	Rate         decimal.Decimal `json:"rate"`
	RateDate     time.Time       `json:"rate_date"`
	Source       string          `json:"source"`
	Note         *string         `json:"note,omitempty"`
	CreatedBy    string          `json:"created_by"`
	CreatedAt    time.Time       `json:"created_at"`
}

type RecordRateRequest struct {
	FromCurrency string  `json:"from_currency"`
	ToCurrency   string  `json:"to_currency"`
	Rate         string  `json:"rate"`
	RateDate     string  `json:"rate_date"`
	Source       *string `json:"source"`
	Note         *string `json:"note"`
}

// Direction says whether a resolved rate was recorded in the direction asked
// for or derived by inverting the opposite pair.
//
// ⚠ It is reported rather than hidden because an inverted rate is a DERIVED
// number. A reader checking why a conversion produced the figure it did needs
// to know whether the rate came from the table as written or from arithmetic
// on its reciprocal.
type Direction string

const (
	DirectionDirect   Direction = "direct"
	DirectionInverted Direction = "inverted"
)

// ResolvedRate is the answer to "what rate applied on this date".
type ResolvedRate struct {
	Rate      decimal.Decimal `json:"rate"`
	RateDate  time.Time       `json:"rate_date"`
	Direction Direction       `json:"direction"`
	// SourceRateID is the row the rate came from, inverted or not, so a
	// conversion can always be traced to a record somebody entered.
	SourceRateID string `json:"source_rate_id"`
}

// ConversionResult is what a caller gets back from a conversion attempt.
//
// ⚠ Available is false when no rate exists, and that is a NORMAL outcome the
// caller must handle — not an error to log and continue past. Phase 9B's
// settlement reports an unconvertible advance at zero with an explanation
// precisely because guessing was worse than admitting.
type ConversionResult struct {
	Available  bool          `json:"available"`
	Conversion *Conversion   `json:"conversion,omitempty"`
	Resolved   *ResolvedRate `json:"resolved_rate,omitempty"`
}
