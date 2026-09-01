// backend/internal/tests/integration/entity_scoping_test.go
// Phase 11B-2: entity re-scoping of payroll and statutory resolution.
//
// ⚠ EVERY CLAIM HERE IS PAIRED WITH ITS FAIL-OPEN COUNTERPART, because the
// dangerous direction is not "did the filter work" but "did the filter fire
// when it should not have". Narrowing statutory rules wrongly means
// withholding NOTHING; narrowing a payroll run wrongly means paying NOBODY.
// Both are worse than the defect being fixed.
//
// ⚠ AND THE TEST SUITE CANNOT CATCH THAT BY ACCIDENT: test organizations set
// no country, so a strict filter would leave every other test in this file
// green. The fail-open cases below are written deliberately.
//
// Gate: INTEGRATION=1
package integration

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/mridha/businesssaas/internal/hrm/entities"
	hrmpayslips "github.com/mridha/businesssaas/internal/hrm/payslips"
	"github.com/shopspring/decimal"
)

// seedStatutoryRule creates an active rule with one 100%-of-gross slab, so a
// rule that applies is unmistakable in the payslip figures.
func seedCountryRule(t *testing.T, env *testEnv, orgID, ownerID, name, country string, ratePct string) string {
	t.Helper()
	ctx := context.Background()
	var ruleID string
	if err := env.db.QueryRow(ctx,
		`INSERT INTO hrm_statutory_rules (org_id, name, country_code, rule_type, base_variable,
		     is_employer_contribution, is_active, created_by)
		 VALUES ($1,$2,$3,'income_tax','GROSS',FALSE,TRUE,$4) RETURNING id`,
		orgID, name, country, ownerID).Scan(&ruleID); err != nil {
		t.Fatalf("seed rule %s: %v", name, err)
	}
	if _, err := env.db.Exec(ctx,
		`INSERT INTO hrm_statutory_slabs (rule_id, up_to, rate_pct, effective_date, created_by)
		 VALUES ($1,NULL,$2,CURRENT_DATE - INTERVAL '1 year',$3)`,
		ruleID, ratePct, ownerID); err != nil {
		t.Fatalf("seed slab for %s: %v", name, err)
	}
	return ruleID
}

func entityFor(t *testing.T, env *testEnv, orgID, ownerID, name, country string) *entities.LegalEntity {
	t.Helper()
	e, err := env.hrmEntitiesSvc.CreateEntity(context.Background(), orgID,
		entities.Caller{UserID: ownerID, CanManageEntities: true},
		entities.CreateEntityRequest{Name: name, CountryCode: &country})
	if err != nil {
		t.Fatalf("create entity %s: %v", name, err)
	}
	return e
}

func assignEntity(t *testing.T, env *testEnv, employeeID, entityID string) {
	t.Helper()
	if _, err := env.db.Exec(context.Background(),
		`UPDATE hrm_employees SET legal_entity_id=$2 WHERE id=$1`, employeeID, entityID); err != nil {
		t.Fatalf("assign entity: %v", err)
	}
}

// ============================================================
// The defect: statutory rules ignored country entirely
// ============================================================

// TestIntegration_EntityScoping_StatutoryRulesNarrowToTheEmployeesCountry is
// the live defect this slice fixes. Before 11B-2 an organization operating in
// two countries applied BOTH countries' deductions to everybody.
func TestIntegration_EntityScoping_StatutoryRulesNarrowToTheEmployeesCountry(t *testing.T) {
	env := newTestEnv(t)
	ctx := context.Background()
	orgID, statusID, ownerID := seedScopeTestOrg(t, env)

	seedCountryRule(t, env, orgID, ownerID, "German Income Tax", "DE", "10")
	seedCountryRule(t, env, orgID, ownerID, "British Income Tax", "GB", "20")

	german := entityFor(t, env, orgID, ownerID, "Acme GmbH", "DE")
	british := entityFor(t, env, orgID, ownerID, "Acme UK Ltd", "GB")

	berlin := seedEmployee(t, env, orgID, statusID, ownerID, "", "Berlin", nil)
	london := seedEmployee(t, env, orgID, statusID, ownerID, "", "London", nil)
	assignEntity(t, env, berlin, german.ID)
	assignEntity(t, env, london, british.ID)

	gross := decimal.RequireFromString("1000")
	for _, c := range []struct {
		name, employeeID, wantRule string
		wantAmount                 string
	}{
		{"the German employee", berlin, "German Income Tax", "100"},
		{"the British employee", london, "British Income Tax", "200"},
	} {
		lines, err := env.hrmStatutorySvc.ComputeForEmployee(ctx, orgID, c.employeeID,
			time.Now().Year(), int(time.Now().Month()), gross, gross, gross)
		if err != nil {
			t.Fatalf("%s: compute: %v", c.name, err)
		}
		if len(lines) != 1 {
			names := []string{}
			for _, l := range lines {
				names = append(names, l.Description)
			}
			t.Fatalf("%s got %d statutory lines %v, want exactly 1 — both countries' rules "+
				"are being applied to everybody, which is the defect", c.name, len(lines), names)
		}
		if lines[0].Description != c.wantRule {
			t.Errorf("%s got rule %q, want %q", c.name, lines[0].Description, c.wantRule)
		}
		if !lines[0].Amount.Equal(decimal.RequireFromString(c.wantAmount)) {
			t.Errorf("%s deduction = %s, want %s", c.name, lines[0].Amount, c.wantAmount)
		}
	}
}

// TestIntegration_EntityScoping_StatutoryFailsOpenWithoutALegalEntity is the
// counterpart, and the more important half.
//
// ⚠ organizations.country is a PROFILE FIELD somebody filled in at signup, not
// a declaration of where payroll runs. Narrowing statutory withholding on the
// strength of it would silently zero the deductions of any organization whose
// profile country does not match how its rules are tagged — and no existing
// test would have failed, because test orgs set no country at all.
func TestIntegration_EntityScoping_StatutoryFailsOpenWithoutALegalEntity(t *testing.T) {
	env := newTestEnv(t)
	ctx := context.Background()
	orgID, statusID, ownerID := seedScopeTestOrg(t, env)

	seedCountryRule(t, env, orgID, ownerID, "German Income Tax", "DE", "10")
	seedCountryRule(t, env, orgID, ownerID, "British Income Tax", "GB", "20")

	// ⚠ The profile country must MATCH one of the rule countries, or this
	// test cannot tell "did not narrow" apart from "narrowed, found no rules
	// for BD, and fell back". An earlier version used BD here and stayed
	// green under an injection that narrowed on the profile — it proved
	// nothing. With GB, narrowing on the profile yields exactly 1 rule and
	// the assertion below fails loudly.
	setOrgDefaults(t, env, orgID, "GB", "GBP", "Europe/London")
	emp := seedEmployee(t, env, orgID, statusID, ownerID, "", "Unassigned", nil)

	gross := decimal.RequireFromString("1000")
	lines, err := env.hrmStatutorySvc.ComputeForEmployee(ctx, orgID, emp,
		time.Now().Year(), int(time.Now().Month()), gross, gross, gross)
	if err != nil {
		t.Fatalf("compute: %v", err)
	}
	if len(lines) != 2 {
		names := []string{}
		for _, l := range lines {
			names = append(names, l.Description)
		}
		t.Fatalf("%d statutory lines %v, want 2 — with NO legal entity declaring a country, "+
			"every active rule must still apply. The org profile says GB, so narrowing on it "+
			"would leave only the British rule and silently drop the German one", len(lines), names)
	}
}

// TestIntegration_EntityScoping_StatutoryFailsOpenWhenTheCountryHasNoRules —
// a company that opens a German subsidiary before writing its German rules
// must keep withholding what it was withholding, not silently stop.
func TestIntegration_EntityScoping_StatutoryFailsOpenWhenTheCountryHasNoRules(t *testing.T) {
	env := newTestEnv(t)
	ctx := context.Background()
	orgID, statusID, ownerID := seedScopeTestOrg(t, env)

	seedCountryRule(t, env, orgID, ownerID, "British Income Tax", "GB", "20")
	// A French entity, and no French rules anywhere.
	french := entityFor(t, env, orgID, ownerID, "Acme France SARL", "FR")
	paris := seedEmployee(t, env, orgID, statusID, ownerID, "", "Paris", nil)
	assignEntity(t, env, paris, french.ID)

	gross := decimal.RequireFromString("1000")
	lines, err := env.hrmStatutorySvc.ComputeForEmployee(ctx, orgID, paris,
		time.Now().Year(), int(time.Now().Month()), gross, gross, gross)
	if err != nil {
		t.Fatalf("compute: %v", err)
	}
	if len(lines) == 0 {
		t.Fatal("a French employee had NOTHING withheld because the org has no French rules " +
			"yet — under-withholding is a liability the employee discovers at year end, and " +
			"is worse than the over-application being fixed")
	}
}

// ============================================================
// Payroll runs scoped to an entity
// ============================================================

// TestIntegration_EntityScoping_PayrollRunNarrowsToItsEntity gives
// hrm_payslip_runs.legal_entity_id its first reader.
func TestIntegration_EntityScoping_PayrollRunNarrowsToItsEntity(t *testing.T) {
	env := newTestEnv(t)
	ctx := context.Background()
	pf := seedPayrollFixture(t, env, "30000")
	addComponent(t, env, pf, "Basic Pay", "earning", "pct_of_basic", "100", nil, 1)

	german := entityFor(t, env, pf.orgID, pf.ownerID, "Acme GmbH", "DE")
	british := entityFor(t, env, pf.orgID, pf.ownerID, "Acme UK Ltd", "GB")

	// The fixture's own employee joins the German entity; a second joins the
	// British one.
	assignEntity(t, env, pf.employeeID, german.ID)
	other := seedEmployee(t, env, pf.orgID, pf.statusID, pf.ownerID, "", "Londoner", nil)
	seedSalaryRecord(t, env, pf.orgID, other, pf.structureID, "30000", pf.ownerID)
	assignEntity(t, env, other, british.ID)

	// An org-wide run covers both.
	orgWide, err := env.hrmPayslipsSvc.CreateRun(ctx, pf.orgID, pf.ownerID, hrmpayslips.CreateRunRequest{
		Year: pf.year, Month: pf.month,
	})
	if err != nil {
		t.Fatalf("create org-wide run: %v", err)
	}
	if _, err := env.hrmPayslipsSvc.ComputeRun(ctx, pf.orgID, orgWide.ID, pf.ownerID); err != nil {
		t.Fatalf("compute org-wide: %v", err)
	}
	if n := payslipCount(t, env, orgWide.ID); n != 2 {
		t.Errorf("org-wide run produced %d payslips, want 2 — a run with no entity covers the "+
			"whole organization, which is every run that already exists", n)
	}

	// A run scoped to the German entity covers only its employee.
	scoped, err := env.hrmPayslipsSvc.CreateRun(ctx, pf.orgID, pf.ownerID, hrmpayslips.CreateRunRequest{
		Year: pf.year, Month: pf.month,
		LegalEntityID: &german.ID,
		Description:   strPtr("German entity run"),
	})
	if err != nil {
		t.Fatalf("create scoped run: %v", err)
	}
	if scoped.LegalEntityID == nil || *scoped.LegalEntityID != german.ID {
		t.Fatalf("the run did not store its legal entity: %v", scoped.LegalEntityID)
	}
	if _, err := env.hrmPayslipsSvc.ComputeRun(ctx, pf.orgID, scoped.ID, pf.ownerID); err != nil {
		t.Fatalf("compute scoped: %v", err)
	}
	if n := payslipCount(t, env, scoped.ID); n != 1 {
		t.Errorf("the German run produced %d payslips, want 1", n)
	}
}

// TestIntegration_EntityScoping_RunWithNoEntityStillPaysEveryone is the
// fail-open guard for payroll.
//
// ⚠ Entity narrowing written as a plain equality would have produced an EMPTY
// payroll run for every organization in this database, none of which has an
// entity configured. Nobody gets paid is the worst possible failure mode of
// this slice.
func TestIntegration_EntityScoping_RunWithNoEntityStillPaysEveryone(t *testing.T) {
	env := newTestEnv(t)
	ctx := context.Background()
	pf := seedPayrollFixture(t, env, "30000")
	addComponent(t, env, pf, "Basic Pay", "earning", "pct_of_basic", "100", nil, 1)

	// No entities exist at all, and the employee has no legal_entity_id —
	// the state of every organization in this database.
	run, err := env.hrmPayslipsSvc.CreateRun(ctx, pf.orgID, pf.ownerID, hrmpayslips.CreateRunRequest{
		Year: pf.year, Month: pf.month,
	})
	if err != nil {
		t.Fatalf("create run: %v", err)
	}
	if run.LegalEntityID != nil {
		t.Errorf("an entity (%s) was invented for an org-wide run", *run.LegalEntityID)
	}
	if _, err := env.hrmPayslipsSvc.ComputeRun(ctx, pf.orgID, run.ID, pf.ownerID); err != nil {
		t.Fatalf("compute: %v", err)
	}
	if n := payslipCount(t, env, run.ID); n != 1 {
		t.Fatalf("%d payslips, want 1 — entity narrowing emptied a run for an org with no "+
			"entities", n)
	}
}

// ============================================================
// The BDT default
// ============================================================

// TestIntegration_EntityScoping_RunCurrencyComesFromADeclarationNotAProfile
//
// ⚠ organizations.currency is NOT NULL DEFAULT 'USD', so EVERY organization
// carries USD whether or not a human ever chose it, and nothing distinguishes
// a deliberate USD from an untouched default. Resolving payroll currency from
// it would have silently relabelled every existing organization's payslips
// from BDT to USD on the day this shipped.
//
// So a currency counts only when a LEGAL ENTITY declares one — an act, not a
// default. Same principle as the statutory country rule above.
func TestIntegration_EntityScoping_RunCurrencyComesFromADeclarationNotAProfile(t *testing.T) {
	env := newTestEnv(t)
	ctx := context.Background()
	pf := seedPayrollFixture(t, env, "30000")

	// The organization profile says USD — as every organization's does.
	setOrgDefaults(t, env, pf.orgID, "US", "USD", "America/New_York")

	run, err := env.hrmPayslipsSvc.CreateRun(ctx, pf.orgID, pf.ownerID, hrmpayslips.CreateRunRequest{
		Year: pf.year, Month: pf.month,
	})
	if err != nil {
		t.Fatalf("create run: %v", err)
	}
	if run.Currency != "BDT" {
		t.Errorf("run currency = %q, want BDT — the organization profile is a schema default "+
			"nobody chose, and reading it here relabels every existing org's payslips",
			run.Currency)
	}

	// A legal entity's declared currency DOES count.
	euro := "EUR"
	entity, err := env.hrmEntitiesSvc.CreateEntity(ctx, pf.orgID,
		entities.Caller{UserID: pf.ownerID, CanManageEntities: true},
		entities.CreateEntityRequest{Name: "Acme GmbH", CountryCode: strPtr("DE"), BaseCurrency: &euro})
	if err != nil {
		t.Fatalf("create entity: %v", err)
	}
	scoped, err := env.hrmPayslipsSvc.CreateRun(ctx, pf.orgID, pf.ownerID, hrmpayslips.CreateRunRequest{
		Year: pf.year, Month: previousMonth(pf.month), LegalEntityID: &entity.ID,
	})
	if err != nil {
		t.Fatalf("create scoped run: %v", err)
	}
	if scoped.Currency != "EUR" {
		t.Errorf("the German entity's run is in %q, want EUR — a declared currency must win",
			scoped.Currency)
	}

	// ⚠ And once ONE entity is the default, an org-wide run inherits its
	// declaration too — that is the first entity becoming the default (11A),
	// which IS an act somebody took.
	orgWide, err := env.hrmPayslipsSvc.CreateRun(ctx, pf.orgID, pf.ownerID, hrmpayslips.CreateRunRequest{
		Year: pf.year - 1, Month: 3,
	})
	if err != nil {
		t.Fatalf("create later org-wide run: %v", err)
	}
	if orgWide.Currency != "EUR" {
		t.Errorf("org-wide run currency = %q, want EUR from the default entity", orgWide.Currency)
	}

	// A stated currency still wins over everything.
	stated := "GBP"
	explicit, err := env.hrmPayslipsSvc.CreateRun(ctx, pf.orgID, pf.ownerID, hrmpayslips.CreateRunRequest{
		Year: pf.year - 1, Month: 6, Currency: &stated,
	})
	if err != nil {
		t.Fatalf("create explicit run: %v", err)
	}
	if explicit.Currency != "GBP" {
		t.Errorf("an explicitly stated currency was overridden: %q", explicit.Currency)
	}
}

// TestIntegration_EntityScoping_UnconfiguredOrgKeepsTheHistoricalDefault —
// the literal survives as the LAST resort so an org that has declared nothing
// gets exactly what it got before 11B-2.
func TestIntegration_EntityScoping_UnconfiguredOrgKeepsTheHistoricalDefault(t *testing.T) {
	env := newTestEnv(t)
	ctx := context.Background()
	pf := seedPayrollFixture(t, env, "30000")

	run, err := env.hrmPayslipsSvc.CreateRun(ctx, pf.orgID, pf.ownerID, hrmpayslips.CreateRunRequest{
		Year: pf.year, Month: pf.month,
	})
	if err != nil {
		t.Fatalf("create run: %v", err)
	}
	if run.Currency != "BDT" {
		t.Errorf("run currency = %q, want the historical BDT", run.Currency)
	}
}

// ============================================================
// Analytics
// ============================================================

// TestIntegration_EntityScoping_AnalyticsSnapshotsByEntity — the
// legal_entity dimension has always been permitted by
// chk_hrm_hcsnap_dimension; the nightly job never populated it.
func TestIntegration_EntityScoping_AnalyticsSnapshotsByEntity(t *testing.T) {
	env := newTestEnv(t)
	ctx := context.Background()
	orgID, statusID, ownerID := seedScopeTestOrg(t, env)
	caller := analyst(ownerID)
	today := time.Now().Truncate(24 * time.Hour)

	german := entityFor(t, env, orgID, ownerID, "Acme GmbH", "DE")
	for i := 0; i < 3; i++ {
		emp := seedHire(t, env, &anFixture{orgID: orgID, statusID: statusID, ownerID: ownerID},
			fmt.Sprintf("DE%d", i), today.AddDate(-1, 0, 0), nil)
		assignEntity(t, env, emp, german.ID)
	}
	// One employee with NO entity — must not appear in any entity row.
	seedHire(t, env, &anFixture{orgID: orgID, statusID: statusID, ownerID: ownerID},
		"Unassigned", today.AddDate(-1, 0, 0), nil)

	if _, err := env.hrmAnalyticsSvc.RunSnapshotForOrg(ctx, orgID, caller, today); err != nil {
		t.Fatalf("snapshot: %v", err)
	}

	entityRows, err := env.hrmAnalyticsSvc.Headcount(ctx, orgID, caller, today, today, "legal_entity")
	if err != nil {
		t.Fatalf("headcount by entity: %v", err)
	}
	if len(entityRows) != 1 {
		t.Fatalf("%d legal_entity rows, want 1 — the employee with no entity must not form a "+
			"row keyed on NULL", len(entityRows))
	}
	if entityRows[0].Headcount != 3 {
		t.Errorf("entity headcount = %d, want 3", entityRows[0].Headcount)
	}
	if entityRows[0].DimensionID == nil || *entityRows[0].DimensionID != german.ID {
		t.Errorf("dimension id = %v, want the German entity", entityRows[0].DimensionID)
	}

	// The org total still counts everybody, entity or not.
	orgRows, err := env.hrmAnalyticsSvc.Headcount(ctx, orgID, caller, today, today, "org")
	if err != nil {
		t.Fatalf("headcount org: %v", err)
	}
	if len(orgRows) != 1 || orgRows[0].Headcount != 4 {
		t.Errorf("org headcount = %v, want 4 — entity scoping must not drop the unassigned "+
			"employee from the organization total", orgRows)
	}
}

// TestIntegration_EntityScoping_OrgWithNoEntitiesProducesNoEntityRows
func TestIntegration_EntityScoping_OrgWithNoEntitiesProducesNoEntityRows(t *testing.T) {
	env := newTestEnv(t)
	ctx := context.Background()
	orgID, statusID, ownerID := seedScopeTestOrg(t, env)
	caller := analyst(ownerID)
	today := time.Now().Truncate(24 * time.Hour)

	seedHire(t, env, &anFixture{orgID: orgID, statusID: statusID, ownerID: ownerID},
		"Solo", today.AddDate(-1, 0, 0), nil)
	if _, err := env.hrmAnalyticsSvc.RunSnapshotForOrg(ctx, orgID, caller, today); err != nil {
		t.Fatalf("snapshot: %v", err)
	}
	rows, err := env.hrmAnalyticsSvc.Headcount(ctx, orgID, caller, today, today, "legal_entity")
	if err != nil {
		t.Fatalf("headcount by entity: %v", err)
	}
	if len(rows) != 0 {
		t.Errorf("%d legal_entity rows for an org with no entities, want 0", len(rows))
	}
	orgRows, _ := env.hrmAnalyticsSvc.Headcount(ctx, orgID, caller, today, today, "org")
	if len(orgRows) != 1 || orgRows[0].Headcount != 1 {
		t.Errorf("the org total is wrong for an entity-less org: %v", orgRows)
	}
}

// ── helpers ──────────────────────────────────────────────────────────────────

// payslipCount is the observable outcome of employee-set narrowing.
func payslipCount(t *testing.T, env *testEnv, runID string) int {
	t.Helper()
	var n int
	if err := env.db.QueryRow(context.Background(),
		`SELECT count(*)::int FROM hrm_payslips WHERE payslip_run_id=$1`, runID).Scan(&n); err != nil {
		t.Fatalf("count payslips: %v", err)
	}
	return n
}

func seedSalaryRecord(t *testing.T, env *testEnv, orgID, employeeID, structureID, basicPay, ownerID string) {
	t.Helper()
	if _, err := env.db.Exec(context.Background(),
		`INSERT INTO hrm_employee_salary_records
		    (org_id, employee_id, structure_id, basic_pay, effective_date, change_reason, created_by)
		 VALUES ($1,$2,$3,$4,'2020-01-01','joining',$5)`,
		orgID, employeeID, structureID, basicPay, ownerID); err != nil {
		t.Fatalf("seed salary record: %v", err)
	}
}

// previousMonth keeps a second run in the same year from colliding with the
// first on the (org, year, month, run_type) uniqueness rule.
func previousMonth(m int) int {
	if m <= 1 {
		return 12
	}
	return m - 1
}
