// backend/internal/tests/integration/entities_test.go
// Phase 11A: legal entities, country configs and locations.
//
// ⚠ THE CENTRAL CLAIM IS A NEGATIVE ONE: an organization with no legal
// entities — the state EVERY organization in this database is in today — must
// be completely unaffected. legal_entity_id is a nullable, un-backfilled FK on
// 38 tables, and nothing in this phase may make any of them required.
//
// The rest is the resolution chain: entity-specific → org default →
// organization, resolved FIELD BY FIELD.
//
// Gate: INTEGRATION=1
package integration

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/mridha/businesssaas/internal/hrm/entities"
	"github.com/shopspring/decimal"
)

type entFixture struct {
	orgID    string
	statusID string
	ownerID  string
}

func entAdmin(userID string) entities.Caller {
	return entities.Caller{UserID: userID, CanManageEntities: true, CanManageLocations: true}
}

func seedEntFixture(t *testing.T, env *testEnv) *entFixture {
	t.Helper()
	orgID, statusID, ownerID := seedScopeTestOrg(t, env)
	return &entFixture{orgID: orgID, statusID: statusID, ownerID: ownerID}
}

// setOrgDefaults writes the organization-level country/currency/timezone that
// form the LAST link of the chain.
func setOrgDefaults(t *testing.T, env *testEnv, orgID, country, currency, tz string) {
	t.Helper()
	if _, err := env.db.Exec(context.Background(),
		`UPDATE organizations SET country=$2, currency=$3, timezone=$4 WHERE id=$1`,
		orgID, country, currency, tz); err != nil {
		t.Fatalf("set org defaults: %v", err)
	}
}

func sp(s string) *string { return &s }

// ============================================================
// The regression guard: a zero/single-entity org is untouched
// ============================================================

// TestIntegration_Entities_OrgWithNoEntitiesResolvesToItself is the guard for
// the whole of Phase 11.
//
// Every organization in this database has zero legal entities. If resolution
// required one — or errored without one, or defaulted to a guess — every
// payroll run, statutory lookup and expense conversion in the product would
// break the moment 11B started consuming this.
func TestIntegration_Entities_OrgWithNoEntitiesResolvesToItself(t *testing.T) {
	env := newTestEnv(t)
	ctx := context.Background()
	fx := seedEntFixture(t, env)
	setOrgDefaults(t, env, fx.orgID, "BD", "BDT", "Asia/Dhaka")

	entCtx, err := env.hrmEntitiesSvc.ResolveContext(ctx, fx.orgID, nil)
	if err != nil {
		t.Fatalf("resolving with no entities errored: %v — an org with no legal entities is "+
			"the normal case, not a misconfiguration", err)
	}
	if entCtx.EntityCount != 0 || !entCtx.SingleEntity {
		t.Errorf("entity_count=%d single_entity=%v, want 0 and true",
			entCtx.EntityCount, entCtx.SingleEntity)
	}
	if entCtx.LegalEntityID != nil {
		t.Errorf("an entity id (%s) was invented for an org that has none", *entCtx.LegalEntityID)
	}
	if entCtx.CountryCode.Value != "BD" || entCtx.CountryCode.Source != entities.SourceOrg {
		t.Errorf("country = %q from %q, want BD from the organization",
			entCtx.CountryCode.Value, entCtx.CountryCode.Source)
	}
	if entCtx.Currency.Value != "BDT" || entCtx.Currency.Source != entities.SourceOrg {
		t.Errorf("currency = %q from %q, want BDT from the organization",
			entCtx.Currency.Value, entCtx.Currency.Source)
	}
	if entCtx.Timezone.Value != "Asia/Dhaka" {
		t.Errorf("timezone = %q, want Asia/Dhaka", entCtx.Timezone.Value)
	}
	// A missing country config is normal, never an error.
	if entCtx.Config != nil {
		t.Errorf("a country config was returned for an org that recorded none: %+v", entCtx.Config)
	}
}

// TestIntegration_Entities_LegalEntityIDStaysNullableEverywhere is the
// structural half of the same guard, asserted against information_schema.
//
// 11A must not have made a single one of the 38 legal_entity_id columns
// required, and must not have backfilled any of them.
func TestIntegration_Entities_LegalEntityIDStaysNullableEverywhere(t *testing.T) {
	env := newTestEnv(t)
	ctx := context.Background()

	var total, nullable int
	if err := env.db.QueryRow(ctx,
		`SELECT count(*)::int, count(*) FILTER (WHERE is_nullable='YES')::int
		   FROM information_schema.columns
		  WHERE column_name='legal_entity_id' AND table_schema='public'`).Scan(&total, &nullable); err != nil {
		t.Fatalf("introspect: %v", err)
	}
	if total == 0 {
		t.Fatal("no legal_entity_id columns found at all — the 0.4 layer is missing")
	}
	if nullable != total {
		t.Errorf("%d of %d legal_entity_id columns are NOT NULL — every one must stay nullable, "+
			"because a single-entity org has nothing to put in them", total-nullable, total)
	}

	// And hrm_employees.location_id, the one column 11A added to an existing
	// table, is nullable for the same reason.
	var loc string
	if err := env.db.QueryRow(ctx,
		`SELECT is_nullable FROM information_schema.columns
		  WHERE table_name='hrm_employees' AND column_name='location_id'`).Scan(&loc); err != nil {
		t.Fatalf("introspect location_id: %v", err)
	}
	if loc != "YES" {
		t.Error("hrm_employees.location_id is NOT NULL — every existing employee row predates it")
	}
}

// TestIntegration_Entities_ExistingHRMPathsStillWorkWithNoEntity — an
// employee can still be hired, and every structural FK stays optional.
func TestIntegration_Entities_ExistingHRMPathsStillWorkWithNoEntity(t *testing.T) {
	env := newTestEnv(t)
	ctx := context.Background()
	fx := seedEntFixture(t, env)

	emp := seedEmployee(t, env, fx.orgID, fx.statusID, fx.ownerID, "", "Unaffected", nil)
	var entityID, locationID *string
	if err := env.db.QueryRow(ctx,
		`SELECT legal_entity_id::text, location_id::text FROM hrm_employees WHERE id=$1`,
		emp).Scan(&entityID, &locationID); err != nil {
		t.Fatalf("read employee: %v", err)
	}
	if entityID != nil || locationID != nil {
		t.Errorf("a newly hired employee got legal_entity_id=%v location_id=%v — nothing may "+
			"backfill these", entityID, locationID)
	}
}

// ============================================================
// The resolution chain
// ============================================================

// TestIntegration_Entities_ChainResolvesFieldByField is the claim that makes
// the chain safe on half-configured records, proved against a real database.
func TestIntegration_Entities_ChainResolvesFieldByField(t *testing.T) {
	env := newTestEnv(t)
	ctx := context.Background()
	fx := seedEntFixture(t, env)
	caller := entAdmin(fx.ownerID)
	setOrgDefaults(t, env, fx.orgID, "US", "USD", "America/New_York")

	// The default entity: currency and timezone, no country.
	def, err := env.hrmEntitiesSvc.CreateEntity(ctx, fx.orgID, caller, entities.CreateEntityRequest{
		Name: "Acme Group", BaseCurrency: sp("EUR"), Timezone: sp("Europe/Berlin"),
	})
	if err != nil {
		t.Fatalf("create default entity: %v", err)
	}
	if !def.IsDefault {
		t.Fatal("the first entity an org creates did not become its default — every lookup " +
			"would then skip step two of the chain and silently ignore it")
	}

	// A subsidiary: country only.
	sub, err := env.hrmEntitiesSvc.CreateEntity(ctx, fx.orgID, caller, entities.CreateEntityRequest{
		Name: "Acme UK Ltd", CountryCode: sp("GB"),
	})
	if err != nil {
		t.Fatalf("create subsidiary: %v", err)
	}

	got, err := env.hrmEntitiesSvc.ResolveContext(ctx, fx.orgID, &sub.ID)
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	if got.CountryCode.Value != "GB" || got.CountryCode.Source != entities.SourceEntity {
		t.Errorf("country = %q from %q, want GB from the entity",
			got.CountryCode.Value, got.CountryCode.Source)
	}
	// ⚠ The load-bearing assertion: the subsidiary set no currency, so the
	// currency comes from the default entity — WITHOUT dragging that entity's
	// (absent) country along and relocating the subsidiary.
	if got.Currency.Value != "EUR" || got.Currency.Source != entities.SourceDefault {
		t.Errorf("currency = %q from %q, want EUR from the default entity — falling through as "+
			"a unit would have taken its country too", got.Currency.Value, got.Currency.Source)
	}
	if got.Timezone.Value != "Europe/Berlin" || got.Timezone.Source != entities.SourceDefault {
		t.Errorf("timezone = %q from %q, want Europe/Berlin from the default entity",
			got.Timezone.Value, got.Timezone.Source)
	}
	if got.EntityCount != 2 || got.SingleEntity {
		t.Errorf("entity_count=%d single_entity=%v, want 2 and false",
			got.EntityCount, got.SingleEntity)
	}
}

// TestIntegration_Entities_CountryConfigAttachesToTheResolvedCountry — the
// config follows the RESOLVED country, not the one on the entity row, so a
// subsidiary inheriting its country from the default entity still gets that
// country's rules.
func TestIntegration_Entities_CountryConfigAttachesToTheResolvedCountry(t *testing.T) {
	env := newTestEnv(t)
	ctx := context.Background()
	fx := seedEntFixture(t, env)
	caller := entAdmin(fx.ownerID)
	setOrgDefaults(t, env, fx.orgID, "", "", "")

	parent, err := env.hrmEntitiesSvc.CreateEntity(ctx, fx.orgID, caller, entities.CreateEntityRequest{
		Name: "Acme Deutschland GmbH", CountryCode: sp("DE"), BaseCurrency: sp("EUR"),
	})
	if err != nil {
		t.Fatalf("create parent: %v", err)
	}
	// A branch that records nothing at all.
	branch, err := env.hrmEntitiesSvc.CreateEntity(ctx, fx.orgID, caller, entities.CreateEntityRequest{
		Name: "Acme Munich Branch",
	})
	if err != nil {
		t.Fatalf("create branch: %v", err)
	}

	notice := 28
	days := decimal.RequireFromString("30")
	if _, err := env.hrmEntitiesSvc.UpsertCountryConfig(ctx, fx.orgID, caller,
		entities.CountryConfigRequest{
			CountryCode: "DE", CountryName: sp("Germany"), PayrollCycle: sp("monthly"),
			NoticePeriodDays: &notice, GratuityDaysPerYear: &days,
		}); err != nil {
		t.Fatalf("upsert config: %v", err)
	}

	got, err := env.hrmEntitiesSvc.ResolveContext(ctx, fx.orgID, &branch.ID)
	if err != nil {
		t.Fatalf("resolve branch: %v", err)
	}
	if got.CountryCode.Value != "DE" || got.CountryCode.Source != entities.SourceDefault {
		t.Fatalf("country = %q from %q, want DE inherited from the default entity",
			got.CountryCode.Value, got.CountryCode.Source)
	}
	if got.Config == nil {
		t.Fatal("no country config attached, though DE has one — the config must follow the " +
			"RESOLVED country, not only a country recorded on the entity itself")
	}
	if got.Config.NoticePeriodDays == nil || *got.Config.NoticePeriodDays != 28 {
		t.Errorf("notice period = %v, want 28", got.Config.NoticePeriodDays)
	}
	if got.Config.GratuityDaysPerYear == nil || !got.Config.GratuityDaysPerYear.Equal(days) {
		t.Errorf("gratuity days = %v, want 30", got.Config.GratuityDaysPerYear)
	}

	// The parent, which states DE itself, gets the same config from the
	// entity source.
	got, err = env.hrmEntitiesSvc.ResolveContext(ctx, fx.orgID, &parent.ID)
	if err != nil {
		t.Fatalf("resolve parent: %v", err)
	}
	if got.CountryCode.Source != entities.SourceEntity || got.Config == nil {
		t.Errorf("parent resolved country from %q with config=%v", got.CountryCode.Source, got.Config)
	}
}

// TestIntegration_Entities_ConfigIsUpsertedNotDuplicated — two configurations
// for one country would be two answers to "what is the notice period in
// Germany" with nothing to say which wins.
func TestIntegration_Entities_ConfigIsUpsertedNotDuplicated(t *testing.T) {
	env := newTestEnv(t)
	ctx := context.Background()
	fx := seedEntFixture(t, env)
	caller := entAdmin(fx.ownerID)

	first, second := 28, 90
	a, err := env.hrmEntitiesSvc.UpsertCountryConfig(ctx, fx.orgID, caller,
		entities.CountryConfigRequest{CountryCode: "de", NoticePeriodDays: &first})
	if err != nil {
		t.Fatalf("first upsert: %v", err)
	}
	if a.CountryCode != "DE" {
		t.Errorf("country code stored as %q, want DE — a lowercase code from an import must "+
			"not become a second country", a.CountryCode)
	}
	b, err := env.hrmEntitiesSvc.UpsertCountryConfig(ctx, fx.orgID, caller,
		entities.CountryConfigRequest{CountryCode: "DE", NoticePeriodDays: &second})
	if err != nil {
		t.Fatalf("second upsert: %v", err)
	}
	if b.ID != a.ID {
		t.Errorf("a second write for DE created a new row (%s vs %s)", b.ID, a.ID)
	}
	list, err := env.hrmEntitiesSvc.ListCountryConfigs(ctx, fx.orgID, true)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(list) != 1 || list[0].NoticePeriodDays == nil || *list[0].NoticePeriodDays != 90 {
		t.Errorf("configs = %d rows with notice %v, want 1 row at 90", len(list),
			list[0].NoticePeriodDays)
	}
}

// TestIntegration_Entities_RejectsMalformedCodes — an ISO code is the join
// key between an entity, its country config and (in 11B) its exchange rates.
// A free-text country would silently fail to match any of them.
func TestIntegration_Entities_RejectsMalformedCodes(t *testing.T) {
	env := newTestEnv(t)
	ctx := context.Background()
	fx := seedEntFixture(t, env)
	caller := entAdmin(fx.ownerID)

	for _, bad := range []string{"GBR", "U", "United Kingdom", "g1"} {
		if _, err := env.hrmEntitiesSvc.CreateEntity(ctx, fx.orgID, caller,
			entities.CreateEntityRequest{Name: "X", CountryCode: &bad}); !errors.Is(err, entities.ErrInvalidCountry) {
			t.Errorf("country %q returned %v, want ErrInvalidCountry", bad, err)
		}
	}
	for _, bad := range []string{"GB", "POUND", "12"} {
		if _, err := env.hrmEntitiesSvc.CreateEntity(ctx, fx.orgID, caller,
			entities.CreateEntityRequest{Name: "X", BaseCurrency: &bad}); !errors.Is(err, entities.ErrInvalidCurrency) {
			t.Errorf("currency %q returned %v, want ErrInvalidCurrency", bad, err)
		}
	}
	if _, err := env.hrmEntitiesSvc.UpsertCountryConfig(ctx, fx.orgID, caller,
		entities.CountryConfigRequest{CountryCode: "DE", PayrollCycle: sp("fortnightly")}); !errors.Is(err, entities.ErrInvalidCycle) {
		t.Errorf("payroll cycle 'fortnightly' returned %v, want ErrInvalidCycle", err)
	}
	day := 32
	if _, err := env.hrmEntitiesSvc.UpsertCountryConfig(ctx, fx.orgID, caller,
		entities.CountryConfigRequest{CountryCode: "DE", PayDayOfMonth: &day}); !errors.Is(err, entities.ErrInvalidPayDay) {
		t.Errorf("pay day 32 returned %v, want ErrInvalidPayDay", err)
	}
}

// ============================================================
// The default entity
// ============================================================

// TestIntegration_Entities_OrgAlwaysHasExactlyOneDefault — step two of the
// chain has to be single-valued, and an org with entities but no default has
// no step two at all.
func TestIntegration_Entities_OrgAlwaysHasExactlyOneDefault(t *testing.T) {
	env := newTestEnv(t)
	ctx := context.Background()
	fx := seedEntFixture(t, env)
	caller := entAdmin(fx.ownerID)

	first, err := env.hrmEntitiesSvc.CreateEntity(ctx, fx.orgID, caller,
		entities.CreateEntityRequest{Name: "First Co", CountryCode: sp("GB")})
	if err != nil {
		t.Fatalf("create first: %v", err)
	}
	if !first.IsDefault {
		t.Fatal("the first entity is not the default")
	}
	second, err := env.hrmEntitiesSvc.CreateEntity(ctx, fx.orgID, caller,
		entities.CreateEntityRequest{Name: "Second Co", CountryCode: sp("FR")})
	if err != nil {
		t.Fatalf("create second: %v", err)
	}
	if second.IsDefault {
		t.Error("a second entity took the default without being asked to")
	}

	// Promoting the second demotes the first, atomically.
	yes := true
	promoted, err := env.hrmEntitiesSvc.UpdateEntity(ctx, fx.orgID, caller, second.ID,
		entities.UpdateEntityRequest{IsDefault: &yes})
	if err != nil {
		t.Fatalf("promote: %v", err)
	}
	if !promoted.IsDefault {
		t.Fatal("promotion did not take")
	}
	var defaults int
	if err := env.db.QueryRow(ctx,
		`SELECT count(*)::int FROM hrm_legal_entities WHERE org_id=$1 AND is_default`,
		fx.orgID).Scan(&defaults); err != nil {
		t.Fatalf("count defaults: %v", err)
	}
	if defaults != 1 {
		t.Errorf("%d default entities after promotion, want exactly 1", defaults)
	}

	// ⚠ Unsetting the default is REFUSED. An org with entities and no default
	// silently skips step two of the chain and ignores its own entities.
	no := false
	if _, err := env.hrmEntitiesSvc.UpdateEntity(ctx, fx.orgID, caller, second.ID,
		entities.UpdateEntityRequest{IsDefault: &no}); !errors.Is(err, entities.ErrCannotUndefault) {
		t.Errorf("unsetting the default returned %v, want ErrCannotUndefault", err)
	}
}

// ============================================================
// Locations
// ============================================================

// TestIntegration_Entities_LocationsBelongToEntitiesOptionally
func TestIntegration_Entities_LocationsBelongToEntitiesOptionally(t *testing.T) {
	env := newTestEnv(t)
	ctx := context.Background()
	fx := seedEntFixture(t, env)
	caller := entAdmin(fx.ownerID)

	// A site recorded before any entity exists — the common starting state.
	unattached, err := env.hrmEntitiesSvc.CreateLocation(ctx, fx.orgID, caller,
		entities.CreateLocationRequest{Name: "Dhaka Office", Code: sp("dhk"), CountryCode: sp("BD")})
	if err != nil {
		t.Fatalf("create unattached location: %v", err)
	}
	if unattached.LegalEntityID != nil {
		t.Error("a location without an entity was given one")
	}
	if unattached.Code == nil || *unattached.Code != "DHK" {
		t.Errorf("code stored as %v, want DHK — codes are matched case-insensitively in imports",
			unattached.Code)
	}

	ent, err := env.hrmEntitiesSvc.CreateEntity(ctx, fx.orgID, caller,
		entities.CreateEntityRequest{Name: "Acme BD Ltd", CountryCode: sp("BD")})
	if err != nil {
		t.Fatalf("create entity: %v", err)
	}
	attached, err := env.hrmEntitiesSvc.UpdateLocation(ctx, fx.orgID, caller, unattached.ID,
		entities.UpdateLocationRequest{LegalEntityID: &ent.ID})
	if err != nil {
		t.Fatalf("attach: %v", err)
	}
	if attached.LegalEntityID == nil || *attached.LegalEntityID != ent.ID {
		t.Errorf("attach did not take: %v", attached.LegalEntityID)
	}
	if attached.LegalEntityName != "Acme BD Ltd" {
		t.Errorf("entity name %q not resolved for display", attached.LegalEntityName)
	}

	// An entity from ANOTHER org is refused: the FK enforces existence, not
	// tenancy.
	other := seedEntFixture(t, env)
	foreign, err := env.hrmEntitiesSvc.CreateEntity(ctx, other.orgID, entAdmin(other.ownerID),
		entities.CreateEntityRequest{Name: "Foreign Co"})
	if err != nil {
		t.Fatalf("create foreign entity: %v", err)
	}
	if _, err := env.hrmEntitiesSvc.CreateLocation(ctx, fx.orgID, caller,
		entities.CreateLocationRequest{Name: "Sneaky", LegalEntityID: &foreign.ID}); !errors.Is(err, entities.ErrEntityNotFound) {
		t.Errorf("a cross-tenant entity id returned %v, want ErrEntityNotFound", err)
	}
}

// TestIntegration_Entities_LocationCodeAndHeadquartersAreUniqueWhileActive —
// a code is how a site is named in an import or a payroll file, and two
// headquarters is two answers to one question. Retiring a site frees both.
func TestIntegration_Entities_LocationCodeAndHeadquartersAreUniqueWhileActive(t *testing.T) {
	env := newTestEnv(t)
	ctx := context.Background()
	fx := seedEntFixture(t, env)
	caller := entAdmin(fx.ownerID)

	yes := true
	hq, err := env.hrmEntitiesSvc.CreateLocation(ctx, fx.orgID, caller,
		entities.CreateLocationRequest{Name: "HQ", Code: sp("HQ"), IsHeadquarters: &yes})
	if err != nil {
		t.Fatalf("create hq: %v", err)
	}
	if _, err := env.hrmEntitiesSvc.CreateLocation(ctx, fx.orgID, caller,
		entities.CreateLocationRequest{Name: "Duplicate code", Code: sp("hq")}); !errors.Is(err, entities.ErrDuplicateCode) {
		t.Errorf("a duplicate code returned %v, want ErrDuplicateCode", err)
	}
	if _, err := env.hrmEntitiesSvc.CreateLocation(ctx, fx.orgID, caller,
		entities.CreateLocationRequest{Name: "Second HQ", Code: sp("HQ2"), IsHeadquarters: &yes}); !errors.Is(err, entities.ErrHeadquartersTaken) {
		t.Errorf("a second headquarters returned %v, want ErrHeadquartersTaken", err)
	}

	// Retiring the first frees both.
	no := false
	if _, err := env.hrmEntitiesSvc.UpdateLocation(ctx, fx.orgID, caller, hq.ID,
		entities.UpdateLocationRequest{IsActive: &no}); err != nil {
		t.Fatalf("retire: %v", err)
	}
	if _, err := env.hrmEntitiesSvc.CreateLocation(ctx, fx.orgID, caller,
		entities.CreateLocationRequest{Name: "New HQ", Code: sp("HQ"), IsHeadquarters: &yes}); err != nil {
		t.Errorf("reusing a retired site's code and headquarters flag was refused: %v", err)
	}
	// And the retired site is still readable.
	all, err := env.hrmEntitiesSvc.ListLocations(ctx, fx.orgID, nil, false)
	if err != nil {
		t.Fatalf("list all: %v", err)
	}
	if len(all) != 2 {
		t.Errorf("%d locations in history, want 2 — retiring must not delete the record", len(all))
	}
}

// TestIntegration_Entities_LocationCountsItsPeople — the headcount is derived
// at read time from hrm_employees.location_id rather than stored, the 00076
// rule.
func TestIntegration_Entities_LocationCountsItsPeople(t *testing.T) {
	env := newTestEnv(t)
	ctx := context.Background()
	fx := seedEntFixture(t, env)
	caller := entAdmin(fx.ownerID)

	loc, err := env.hrmEntitiesSvc.CreateLocation(ctx, fx.orgID, caller,
		entities.CreateLocationRequest{Name: "Dhaka Office"})
	if err != nil {
		t.Fatalf("create location: %v", err)
	}
	if loc.EmployeeCount != 0 {
		t.Errorf("a new location reports %d employees", loc.EmployeeCount)
	}

	for i := 0; i < 3; i++ {
		emp := seedEmployee(t, env, fx.orgID, fx.statusID, fx.ownerID, "",
			fmt.Sprintf("Worker %d", i), nil)
		if _, err := env.db.Exec(ctx,
			`UPDATE hrm_employees SET location_id=$2 WHERE id=$1`, emp, loc.ID); err != nil {
			t.Fatalf("assign location: %v", err)
		}
		if i == 2 {
			// A departed employee must not be counted as sitting there.
			if _, err := env.db.Exec(ctx,
				`UPDATE hrm_employees SET termination_date=$2 WHERE id=$1`,
				emp, time.Now().AddDate(0, 0, -1)); err != nil {
				t.Fatalf("terminate: %v", err)
			}
		}
	}

	list, err := env.hrmEntitiesSvc.ListLocations(ctx, fx.orgID, nil, true)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(list) != 1 || list[0].EmployeeCount != 2 {
		t.Errorf("location reports %d employees, want 2 — a departed employee is not sitting "+
			"in the building", list[0].EmployeeCount)
	}
}

// ============================================================
// Permissions
// ============================================================

// TestIntegration_Entities_ManageIsSeparateFromView — a legal entity carries
// a tax identifier and is edited by finance; a work site is a building
// somebody adds when the company opens an office.
func TestIntegration_Entities_ManageIsSeparateFromView(t *testing.T) {
	env := newTestEnv(t)
	ctx := context.Background()
	fx := seedEntFixture(t, env)

	// A manager: both view keys, neither manage key (the 00128 grant).
	viewer := entities.Caller{UserID: fx.ownerID}
	if _, err := env.hrmEntitiesSvc.CreateEntity(ctx, fx.orgID, viewer,
		entities.CreateEntityRequest{Name: "Nope"}); !errors.Is(err, entities.ErrAccessDenied) {
		t.Errorf("creating an entity without manage returned %v, want ErrAccessDenied", err)
	}
	if _, err := env.hrmEntitiesSvc.UpsertCountryConfig(ctx, fx.orgID, viewer,
		entities.CountryConfigRequest{CountryCode: "GB"}); !errors.Is(err, entities.ErrAccessDenied) {
		t.Errorf("writing a country config without manage returned %v, want ErrAccessDenied", err)
	}
	if _, err := env.hrmEntitiesSvc.CreateLocation(ctx, fx.orgID, viewer,
		entities.CreateLocationRequest{Name: "Nope"}); !errors.Is(err, entities.ErrAccessDenied) {
		t.Errorf("creating a location without manage returned %v, want ErrAccessDenied", err)
	}

	// Reading is open to everybody who reached the route, and resolution has
	// no manage gate at all — every 11B consumer will call it.
	if _, err := env.hrmEntitiesSvc.ResolveContext(ctx, fx.orgID, nil); err != nil {
		t.Errorf("resolving without manage was refused: %v", err)
	}

	// ⚠ Location management is its OWN key: somebody who may add an office
	// must not thereby be able to change the company's tax registration.
	locOnly := entities.Caller{UserID: fx.ownerID, CanManageLocations: true}
	if _, err := env.hrmEntitiesSvc.CreateLocation(ctx, fx.orgID, locOnly,
		entities.CreateLocationRequest{Name: "Branch"}); err != nil {
		t.Errorf("a location manager could not create a location: %v", err)
	}
	if _, err := env.hrmEntitiesSvc.CreateEntity(ctx, fx.orgID, locOnly,
		entities.CreateEntityRequest{Name: "Acme Ltd"}); !errors.Is(err, entities.ErrAccessDenied) {
		t.Errorf("a location manager created a legal entity: %v", err)
	}
}
