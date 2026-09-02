// backend/internal/hrm/payslips/basecurrency.go
package payslips

import "context"

// BaseCurrencySource is this package's OWN narrow view of the legal-entity
// layer (11B-2), used to resolve a payroll run's currency.
//
// ⚠ Declared by the consumer and satisfied structurally by
// internal/hrm/entities. Primitive signature, no adapter, no import in the
// provider's direction — the same shape as BonusSource, LoanSource,
// ReimbursementSource, StatutorySource and FnFSource above it.
//
// ⚠ An empty return means NO LEGAL ENTITY DECLARED A CURRENCY, which is the
// normal case. It does NOT fall back to organizations.currency: that column
// is NOT NULL DEFAULT 'USD', so reading it would relabel every existing
// organization's money on the day this shipped. CreateRun
// then falls back to the historical default rather than erroring, because an
// organization that has configured nothing must still be able to run payroll.
type BaseCurrencySource interface {
	DeclaredCurrency(ctx context.Context, orgID string, legalEntityID *string) (string, error)
}
