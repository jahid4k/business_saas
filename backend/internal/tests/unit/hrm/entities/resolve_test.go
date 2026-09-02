// backend/internal/tests/unit/hrm/entities/resolve_test.go
package entities_test

import (
	"testing"

	"github.com/mridha/businesssaas/internal/hrm/entities"
)

func entityLink(country, currency, tz string) entities.Candidate {
	return entities.Candidate{
		Source: entities.SourceEntity, Present: true,
		CountryCode: country, Currency: currency, Timezone: tz,
	}
}
func defaultLink(country, currency, tz string) entities.Candidate {
	return entities.Candidate{
		Source: entities.SourceDefault, Present: true,
		CountryCode: country, Currency: currency, Timezone: tz,
	}
}
func orgLink(country, currency, tz string) entities.Candidate {
	return entities.Candidate{
		Source: entities.SourceOrg, Present: true,
		CountryCode: country, Currency: currency, Timezone: tz,
	}
}

// TestResolve_FieldsFallThroughIndEPENDENTLY is the claim that makes the
// chain safe to use on half-configured records.
//
// An entity recording a country but no currency must get the organization's
// CURRENCY while keeping its own COUNTRY. Falling through as a unit would
// take the organization's country too and silently relocate the subsidiary —
// and a half-populated record is the normal state of one somebody is part-way
// through setting up.
func TestResolve_FieldsFallThroughIndependently(t *testing.T) {
	country, currency, tz := entities.Resolve(
		entityLink("GB", "", ""),   // country only
		defaultLink("", "USD", ""), // currency only
		orgLink("US", "EUR", "America/New_York"),
	)

	if country.Value != "GB" || country.Source != entities.SourceEntity {
		t.Errorf("country = %q from %q, want GB from the entity — the entity declares GB and "+
			"must not be relocated by a currency it did not set", country.Value, country.Source)
	}
	if currency.Value != "USD" || currency.Source != entities.SourceDefault {
		t.Errorf("currency = %q from %q, want USD from the default entity — the entity set no "+
			"currency, so the next link supplies it", currency.Value, currency.Source)
	}
	if tz.Value != "America/New_York" || tz.Source != entities.SourceOrg {
		t.Errorf("timezone = %q from %q, want the organization's", tz.Value, tz.Source)
	}
}

// TestResolve_EntityWinsWhenItSpeaks
func TestResolve_EntityWinsWhenItSpeaks(t *testing.T) {
	country, currency, tz := entities.Resolve(
		entityLink("DE", "EUR", "Europe/Berlin"),
		defaultLink("GB", "GBP", "Europe/London"),
		orgLink("US", "USD", "America/New_York"),
	)
	for _, c := range []struct {
		got  entities.Resolved
		want string
	}{{country, "DE"}, {currency, "EUR"}, {tz, "Europe/Berlin"}} {
		if c.got.Value != c.want || c.got.Source != entities.SourceEntity {
			t.Errorf("got %q from %q, want %q from the entity", c.got.Value, c.got.Source, c.want)
		}
	}
}

// TestResolve_NoEntitiesAtAllFallsToTheOrganization is the state EVERY
// organization in this database is in today, and the regression guard for the
// whole of Phase 11: nothing may require an entity to exist.
func TestResolve_NoEntitiesAtAllFallsToTheOrganization(t *testing.T) {
	country, currency, tz := entities.Resolve(
		entities.Candidate{Source: entities.SourceEntity, Present: false},
		entities.Candidate{Source: entities.SourceDefault, Present: false},
		orgLink("BD", "BDT", "Asia/Dhaka"),
	)
	if country.Value != "BD" || country.Source != entities.SourceOrg {
		t.Errorf("country = %q from %q, want BD from the organization", country.Value, country.Source)
	}
	if currency.Value != "BDT" || tz.Value != "Asia/Dhaka" {
		t.Errorf("currency=%q timezone=%q, want BDT and Asia/Dhaka", currency.Value, tz.Value)
	}
}

// TestResolve_UnknownIsNotAGuess — a field nobody has set anywhere must come
// back empty and say so. Defaulting a currency to USD because it is common
// would produce payslips in the wrong money.
func TestResolve_UnknownIsNotAGuess(t *testing.T) {
	country, currency, tz := entities.Resolve(
		entityLink("", "", ""),
		orgLink("", "", ""),
	)
	for _, c := range []struct {
		name string
		got  entities.Resolved
	}{{"country", country}, {"currency", currency}, {"timezone", tz}} {
		if c.got.Value != "" {
			t.Errorf("%s resolved to %q with nothing set anywhere", c.name, c.got.Value)
		}
		if c.got.Source != entities.SourceNone {
			t.Errorf("%s reported source %q, want %q so a caller can tell it is unset",
				c.name, c.got.Source, entities.SourceNone)
		}
	}
}

// TestResolve_AbsentLinksAreSkippedNotTreatedAsBlank — Present=false and
// "populated with empty strings" must behave the same way, because a caller
// building the chain has both shapes available and should not have to know
// which one the resolver prefers.
func TestResolve_AbsentLinksAreSkippedNotTreatedAsBlank(t *testing.T) {
	withAbsent, _, _ := entities.Resolve(
		entities.Candidate{Source: entities.SourceEntity, Present: false, CountryCode: "XX"},
		orgLink("GB", "GBP", ""),
	)
	if withAbsent.Value != "GB" {
		t.Errorf("country = %q — an absent link's contents were read", withAbsent.Value)
	}
}

// TestResolve_NormalisesCodes — ISO codes are compared and stored uppercase,
// and a lowercase 'gb' from an import must not resolve to a different country
// from 'GB'.
func TestResolve_NormalisesCodes(t *testing.T) {
	country, currency, tz := entities.Resolve(entityLink(" gb ", " gbp ", "  Europe/London  "))
	if country.Value != "GB" {
		t.Errorf("country = %q, want GB", country.Value)
	}
	if currency.Value != "GBP" {
		t.Errorf("currency = %q, want GBP", currency.Value)
	}
	if tz.Value != "Europe/London" {
		t.Errorf("timezone = %q, want it trimmed but NOT uppercased — IANA zone names are "+
			"case-sensitive", tz.Value)
	}
}

func TestIsSingleEntity(t *testing.T) {
	// Zero entities is the state of every org in this database today, and one
	// entity is a company that has named itself. Neither is multi-entity.
	for _, n := range []int{0, 1} {
		if !entities.IsSingleEntity(n) {
			t.Errorf("%d entities reported as multi-entity", n)
		}
	}
	if entities.IsSingleEntity(2) {
		t.Error("2 entities reported as single-entity")
	}
}
