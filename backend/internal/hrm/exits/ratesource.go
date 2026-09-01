// backend/internal/hrm/exits/ratesource.go
package exits

import (
	"context"
	"time"

	"github.com/shopspring/decimal"
)

// RateSource is this package's OWN narrow view of the FX layer (11B-1).
//
// ⚠ Declared by the consumer and satisfied structurally by internal/hrm/fx
// with no adapter — the same pattern as LeaveEncashmentSource,
// LoanSettlementSource and AdvanceSettlementSource in this very package.
//
// ok=false means NO RATE EXISTS for that pair on that date, and this package
// treats that as the answer it was already giving before the FX table
// existed: report the advance, recover nothing, say why.
type RateSource interface {
	RateAsOfPrimitive(ctx context.Context, orgID, from, to string, asOf time.Time) (
		rate decimal.Decimal, rateDate time.Time, ok bool, err error)
}
