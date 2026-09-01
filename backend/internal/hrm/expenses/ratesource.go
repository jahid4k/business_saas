// backend/internal/hrm/expenses/ratesource.go
package expenses

import (
	"context"
	"time"

	"github.com/shopspring/decimal"
)

// RateSource is this package's OWN narrow view of the FX layer.
//
// ⚠ Declared by the consumer, satisfied structurally by internal/hrm/fx with
// no adapter and no import in this direction — the pattern used for
// payslips.BonusSource, exits.LoanSettlementSource and nine others.
//
// The primitive return signature is what makes that possible: handing back
// fx.ResolvedRate would force this package to import fx and invert the
// dependency.
//
// ok=false means NO RATE EXISTS for that pair on that date. It is a normal
// answer, not an error — see AddLine for what this package does about it.
type RateSource interface {
	RateAsOfPrimitive(ctx context.Context, orgID, from, to string, asOf time.Time) (
		rate decimal.Decimal, rateDate time.Time, ok bool, err error)
}

// BaseCurrencySource is this package's narrow view of the legal-entity layer
// (11A). It answers "what currency should base_amount be in".
//
// ⚠ An empty string is a valid answer meaning "nobody has said". A caller
// that receives it must skip the conversion rather than assume a currency —
// converting to a guessed base is how a claim gets reimbursed in the wrong
// money.
type BaseCurrencySource interface {
	BaseCurrency(ctx context.Context, orgID string, legalEntityID *string) (string, error)
}
