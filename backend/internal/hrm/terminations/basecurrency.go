// backend/internal/hrm/terminations/basecurrency.go
package terminations

import "context"

// BaseCurrencySource is this package's OWN narrow view of the legal-entity
// layer (11B-2), used to resolve a money amount's currency instead of
// assuming one.
//
// ⚠ Declared by the consumer and satisfied structurally by
// internal/hrm/entities — primitive signature, no adapter, no import in the
// provider's direction.
//
// ⚠ An empty return means no legal entity declared one, and the caller falls back
// to the historical default rather than erroring. An organization that has
// configured nothing must still be able to record one of these.
type BaseCurrencySource interface {
	DeclaredCurrency(ctx context.Context, orgID string, legalEntityID *string) (string, error)
}
