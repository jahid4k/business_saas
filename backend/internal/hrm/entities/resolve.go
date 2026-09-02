// backend/internal/hrm/entities/resolve.go
package entities

import "strings"

// Source names where a resolved value actually came from.
//
// ⚠ EVERY RESOLVED FIELD CARRIES ITS SOURCE, and that is not decoration.
// This whole layer is a fallback chain, so "GBP" alone is ambiguous — a
// reader cannot tell whether the entity declares GBP or whether the entity
// declares nothing and the organization happens to be British. Those are
// different facts with different consequences the day somebody adds a second
// entity, and a value with no provenance hides the difference.
type Source string

const (
	SourceEntity  Source = "entity"
	SourceDefault Source = "default_entity"
	SourceOrg     Source = "organization"
	SourceNone    Source = "none"
)

// Candidate is one link in the resolution chain: a set of values that may or
// may not be populated, labelled with where they came from.
type Candidate struct {
	Source      Source
	Present     bool
	CountryCode string
	Currency    string
	Timezone    string
	EntityID    string
	EntityName  string
}

// Resolved is one field's answer plus its provenance.
type Resolved struct {
	Value  string `json:"value"`
	Source Source `json:"source"`
}

// Resolve walks the chain entity-specific → org default → organization.
//
// ⚠ IT RESOLVES EACH FIELD INDEPENDENTLY, NOT THE WHOLE CANDIDATE.
//
// This is the part that is easy to get wrong and expensive when wrong. An
// entity that records a country but no base currency must still get the
// organization's currency — falling through as a unit would take the
// organization's COUNTRY too and silently relocate the subsidiary. Populating
// one field is the normal state of a record somebody is part-way through
// setting up, and the chain has to survive it.
//
// A field nobody anywhere has set resolves to an empty value with
// SourceNone, never to a guess.
func Resolve(chain ...Candidate) (country, currency, timezone Resolved) {
	country = pick(chain, func(c Candidate) string { return normUpper(c.CountryCode) })
	currency = pick(chain, func(c Candidate) string { return normUpper(c.Currency) })
	timezone = pick(chain, func(c Candidate) string { return strings.TrimSpace(c.Timezone) })
	return
}

func pick(chain []Candidate, get func(Candidate) string) Resolved {
	for _, c := range chain {
		if !c.Present {
			continue
		}
		if v := get(c); v != "" {
			return Resolved{Value: v, Source: c.Source}
		}
	}
	return Resolved{Source: SourceNone}
}

func normUpper(s string) string { return strings.ToUpper(strings.TrimSpace(s)) }

// IsSingleEntity reports whether an org is in the state every organization in
// this database is in today: no legal entities configured at all.
//
// ⚠ It is a first-class concept rather than an incidental zero, because the
// regression guard for the whole of Phase 11 is that such an org is
// completely unaffected — every resolution falls through to the organization
// and nothing anywhere requires an entity to exist.
func IsSingleEntity(entityCount int) bool { return entityCount <= 1 }
