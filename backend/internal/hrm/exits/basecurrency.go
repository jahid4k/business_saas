// backend/internal/hrm/exits/basecurrency.go
package exits

import "context"

// BaseCurrencySource is this package's OWN narrow view of the legal-entity
// layer (11B-2), used for clearance-item amounts and as the settlement's
// currency of last resort.
//
// ⚠ Declared by the consumer and satisfied structurally by
// internal/hrm/entities — primitive signature, no adapter.
type BaseCurrencySource interface {
	DeclaredCurrency(ctx context.Context, orgID string, legalEntityID *string) (string, error)
}
