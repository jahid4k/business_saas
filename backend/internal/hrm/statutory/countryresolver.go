// backend/internal/hrm/statutory/countryresolver.go
package statutory

import "context"

// EmployeeCountryResolver is this package's OWN narrow view of the
// legal-entity layer (11B-2).
//
// ⚠ Declared by the consumer and satisfied structurally by
// internal/hrm/entities with no adapter — the pattern used throughout this
// codebase. The primitive signature is what keeps the dependency running one
// way.
//
// ⚠ fromLegalEntity IS PART OF THE CONTRACT, NOT A CONVENIENCE. It reports
// whether a LEGAL ENTITY declared the country, as opposed to the country
// having been read off organizations.country — a profile field. This package
// withholds real money from people's pay, and it may only narrow its rule set
// on the strength of an actual declaration about where a company operates.
type EmployeeCountryResolver interface {
	CountryForEmployee(ctx context.Context, orgID, employeeID string) (
		country string, fromLegalEntity bool, err error)
}
