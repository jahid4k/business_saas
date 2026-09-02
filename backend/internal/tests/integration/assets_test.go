// backend/internal/tests/integration/assets_test.go
// hrm/assets against real Postgres. The load-bearing claims here are
// STRUCTURAL — that the current holder and seats_used have no backing column
// and are derived on every read — which only a live schema can prove.
// Gate: INTEGRATION=1
package integration

import (
	"context"
	"testing"

	"github.com/mridha/businesssaas/internal/auth"
	"github.com/mridha/businesssaas/internal/authz"
	acks "github.com/mridha/businesssaas/internal/hrm/acknowledgements"
	"github.com/mridha/businesssaas/internal/hrm/approvals"
	"github.com/mridha/businesssaas/internal/hrm/assets"
)

// authSignup is the standard test-user signup shape used across this suite.
func authSignup(email string) auth.SignupRequest {
	return auth.SignupRequest{Email: email, Password: "AssetTestPass123!"}
}

// ackTypeRequest builds a minimal acknowledgement request for one type — used
// to prove every value the DB CHECK permits is reachable through the Go enum.
func ackTypeRequest(employeeID, ackType string) acks.CreateAcknowledgementRequest {
	return acks.CreateAcknowledgementRequest{
		EmployeeID:          employeeID,
		AcknowledgeableType: acks.AckType(ackType),
		AcknowledgeableID:   "00000000-0000-0000-0000-000000000001",
		EntityTitle:         "Enum coverage probe: " + ackType,
	}
}

// assetFixture is one org with an employee and an asset category.
type assetFixture struct {
	orgID      string
	statusID   string
	ownerID    string
	employeeID string
	categoryID string
}

func seedAssetFixture(t *testing.T, env *testEnv, usefulLifeMonths *int) *assetFixture {
	t.Helper()
	ctx := context.Background()
	orgID, statusID, ownerID := seedScopeTestOrg(t, env)
	empID := seedEmployee(t, env, orgID, statusID, ownerID, "", "Asset Holder", nil)

	cat, err := env.hrmAssetsSvc.CreateCategory(ctx, orgID, ownerID, assets.CreateCategoryRequest{
		Name: "Laptops " + uniqueSlug("cat"), UsefulLifeMonths: usefulLifeMonths,
	})
	if err != nil {
		t.Fatalf("create category: %v", err)
	}
	return &assetFixture{orgID: orgID, statusID: statusID, ownerID: ownerID, employeeID: empID, categoryID: cat.ID}
}

func seedAsset(t *testing.T, env *testEnv, fx *assetFixture, name, cost, purchaseDate string) *assets.Asset {
	t.Helper()
	ctx := context.Background()
	req := assets.CreateAssetRequest{Name: name, CategoryID: &fx.categoryID, PurchaseCost: &cost}
	if purchaseDate != "" {
		req.PurchaseDate = &purchaseDate
	}
	a, err := env.hrmAssetsSvc.CreateAsset(ctx, fx.orgID, fx.ownerID, req)
	if err != nil {
		t.Fatalf("create asset %s: %v", name, err)
	}
	return a
}

// ============================================================
// The structural claims — no stored current holder, no stored seats_used
// ============================================================

// TestIntegration_Assets_NoCurrentHolderColumnExists is the build plan's most
// emphatic rule for this module: "assignment history where current holder is
// a derived query, NEVER a stored column". A denormalized holder is a second
// source of truth that drifts the first time a return is recorded without
// updating it.
//
// Introspecting information_schema is the only way to prove a column is
// ABSENT — no behavioural test can. The 6A completion-percentage precedent.
func TestIntegration_Assets_NoCurrentHolderColumnExists(t *testing.T) {
	env := newTestEnv(t)
	ctx := context.Background()

	forbidden := []struct{ table, column string }{
		{"hrm_assets", "current_holder_id"},
		{"hrm_assets", "current_holder_employee_id"},
		{"hrm_assets", "assigned_to"},
		{"hrm_assets", "assigned_to_employee_id"},
		{"hrm_assets", "current_assignment_id"},
		{"hrm_assets", "book_value"},
		{"hrm_software_licenses", "seats_used"},
		{"hrm_software_licenses", "seats_available"},
	}
	for _, f := range forbidden {
		var n int
		if err := env.db.QueryRow(ctx,
			`SELECT COUNT(*) FROM information_schema.columns
			  WHERE table_name = $1 AND column_name = $2`,
			f.table, f.column).Scan(&n); err != nil {
			t.Fatalf("introspect %s.%s: %v", f.table, f.column, err)
		}
		if n != 0 {
			t.Errorf("%s.%s EXISTS — the current holder / seat usage / book value must be DERIVED, never stored (build plan + the 00076 rule)",
				f.table, f.column)
		}
	}
}

// TestIntegration_Assets_CurrentHolderIsDerivedAcrossAFullCycle proves the
// derived query actually tracks reality: assign, read, return, read again.
func TestIntegration_Assets_CurrentHolderIsDerivedAcrossAFullCycle(t *testing.T) {
	env := newTestEnv(t)
	ctx := context.Background()
	fx := seedAssetFixture(t, env, nil)
	a := seedAsset(t, env, fx, "MacBook Pro", "2400", "")

	// Nobody holds it yet.
	fresh, err := env.hrmAssetsSvc.GetAsset(ctx, fx.orgID, a.ID)
	if err != nil {
		t.Fatalf("get asset: %v", err)
	}
	if fresh.CurrentHolderEmployeeID != nil {
		t.Errorf("a brand-new asset has a holder: %s", *fresh.CurrentHolderEmployeeID)
	}
	if fresh.Status != assets.AssetAvailable {
		t.Errorf("status = %s, want available", fresh.Status)
	}

	good := "good"
	if _, err := env.hrmAssetsSvc.AssignAsset(ctx, fx.orgID, a.ID, fx.ownerID, assets.AssignAssetRequest{
		EmployeeID: fx.employeeID, ConditionOut: &good,
	}); err != nil {
		t.Fatalf("assign: %v", err)
	}

	held, err := env.hrmAssetsSvc.GetAsset(ctx, fx.orgID, a.ID)
	if err != nil {
		t.Fatalf("get asset after assign: %v", err)
	}
	if held.CurrentHolderEmployeeID == nil || *held.CurrentHolderEmployeeID != fx.employeeID {
		t.Fatalf("current holder = %v, want %s", held.CurrentHolderEmployeeID, fx.employeeID)
	}
	if held.Status != assets.AssetAssigned {
		t.Errorf("status = %s, want assigned", held.Status)
	}

	if _, err := env.hrmAssetsSvc.ReturnAsset(ctx, fx.orgID, a.ID, fx.ownerID, assets.ReturnAssetRequest{
		ConditionIn: &good,
	}); err != nil {
		t.Fatalf("return: %v", err)
	}

	returned, err := env.hrmAssetsSvc.GetAsset(ctx, fx.orgID, a.ID)
	if err != nil {
		t.Fatalf("get asset after return: %v", err)
	}
	if returned.CurrentHolderEmployeeID != nil {
		t.Errorf("asset still shows a holder after return: %s", *returned.CurrentHolderEmployeeID)
	}
	if returned.Status != assets.AssetAvailable {
		t.Errorf("status = %s, want available after return", returned.Status)
	}

	// The HISTORY survives — a return closes a row, it does not delete one.
	res, err := env.hrmAssetsSvc.ListAssignments(ctx, fx.orgID, assets.ListFilter{Scope: authz.ScopeAll})
	if err != nil {
		t.Fatalf("list assignments: %v", err)
	}
	if len(res.Assignments) != 1 {
		t.Fatalf("expected the closed assignment to remain as history, got %d rows", len(res.Assignments))
	}
	if res.Assignments[0].IsCurrent() {
		t.Error("the returned assignment still reports IsCurrent()")
	}
}

// TestIntegration_Assets_CannotDoubleAssign proves uq_hrm_asgn_active — the
// constraint that makes "current holder" single-valued rather than a guess.
func TestIntegration_Assets_CannotDoubleAssign(t *testing.T) {
	env := newTestEnv(t)
	ctx := context.Background()
	fx := seedAssetFixture(t, env, nil)
	a := seedAsset(t, env, fx, "Monitor", "300", "")
	second := seedEmployee(t, env, fx.orgID, fx.statusID, fx.ownerID, "", "Second Holder", nil)

	if _, err := env.hrmAssetsSvc.AssignAsset(ctx, fx.orgID, a.ID, fx.ownerID, assets.AssignAssetRequest{
		EmployeeID: fx.employeeID,
	}); err != nil {
		t.Fatalf("first assign: %v", err)
	}

	_, err := env.hrmAssetsSvc.AssignAsset(ctx, fx.orgID, a.ID, fx.ownerID, assets.AssignAssetRequest{
		EmployeeID: second,
	})
	if err == nil {
		t.Fatal("assigning an already-held asset succeeded — the current holder would be ambiguous")
	}

	// After a return it may be reassigned — the constraint is partial, not absolute.
	if _, err := env.hrmAssetsSvc.ReturnAsset(ctx, fx.orgID, a.ID, fx.ownerID, assets.ReturnAssetRequest{}); err != nil {
		t.Fatalf("return: %v", err)
	}
	if _, err := env.hrmAssetsSvc.AssignAsset(ctx, fx.orgID, a.ID, fx.ownerID, assets.AssignAssetRequest{
		EmployeeID: second,
	}); err != nil {
		t.Fatalf("reassign after return: %v", err)
	}

	held, err := env.hrmAssetsSvc.GetAsset(ctx, fx.orgID, a.ID)
	if err != nil {
		t.Fatalf("get asset: %v", err)
	}
	if held.CurrentHolderEmployeeID == nil || *held.CurrentHolderEmployeeID != second {
		t.Errorf("current holder = %v, want the second employee %s", held.CurrentHolderEmployeeID, second)
	}
}

// TestIntegration_Assets_DamagedReturnGoesToMaintenance — a damaged asset must
// not silently rejoin the available pool.
func TestIntegration_Assets_DamagedReturnGoesToMaintenance(t *testing.T) {
	env := newTestEnv(t)
	ctx := context.Background()
	fx := seedAssetFixture(t, env, nil)
	a := seedAsset(t, env, fx, "Tablet", "800", "")

	if _, err := env.hrmAssetsSvc.AssignAsset(ctx, fx.orgID, a.ID, fx.ownerID, assets.AssignAssetRequest{
		EmployeeID: fx.employeeID,
	}); err != nil {
		t.Fatalf("assign: %v", err)
	}
	damaged := "damaged"
	if _, err := env.hrmAssetsSvc.ReturnAsset(ctx, fx.orgID, a.ID, fx.ownerID, assets.ReturnAssetRequest{
		ConditionIn: &damaged,
	}); err != nil {
		t.Fatalf("return damaged: %v", err)
	}

	after, err := env.hrmAssetsSvc.GetAsset(ctx, fx.orgID, a.ID)
	if err != nil {
		t.Fatalf("get asset: %v", err)
	}
	if after.Status != assets.AssetInMaintenance {
		t.Errorf("status = %s, want in_maintenance — a damaged return must not rejoin the pool", after.Status)
	}
}

// ============================================================
// Depreciation — computed on read, from the CATEGORY's useful life
// ============================================================

func TestIntegration_Assets_BookValueIsComputedFromCategoryUsefulLife(t *testing.T) {
	env := newTestEnv(t)
	ctx := context.Background()
	life := 24
	fx := seedAssetFixture(t, env, &life)

	// Purchased 12 months ago on a 24-month life -> half its cost remains.
	var twelveAgo string
	if err := env.db.QueryRow(ctx,
		`SELECT to_char(CURRENT_DATE - INTERVAL '12 months', 'YYYY-MM-DD')`).Scan(&twelveAgo); err != nil {
		t.Fatalf("compute date: %v", err)
	}
	a := seedAsset(t, env, fx, "Depreciating Laptop", "2400", twelveAgo)

	got, err := env.hrmAssetsSvc.GetAsset(ctx, fx.orgID, a.ID)
	if err != nil {
		t.Fatalf("get asset: %v", err)
	}
	if got.BookValue == nil {
		t.Fatal("book value was not computed on read")
	}
	if !got.BookValue.Equal(dec(t, "1200")) {
		t.Errorf("book value = %s, want 1200 (half of 2400 at 12/24 months)", got.BookValue)
	}
	// Purchase cost itself must be untouched — depreciation is a read-time view.
	if !got.PurchaseCost.Equal(dec(t, "2400")) {
		t.Errorf("purchase cost = %s, want 2400 unchanged", got.PurchaseCost)
	}
}

func TestIntegration_Assets_CategoryWithoutUsefulLifeIsNotDepreciated(t *testing.T) {
	env := newTestEnv(t)
	ctx := context.Background()
	fx := seedAssetFixture(t, env, nil) // no useful life on the category

	var longAgo string
	if err := env.db.QueryRow(ctx,
		`SELECT to_char(CURRENT_DATE - INTERVAL '60 months', 'YYYY-MM-DD')`).Scan(&longAgo); err != nil {
		t.Fatalf("compute date: %v", err)
	}
	a := seedAsset(t, env, fx, "Non-depreciating Desk", "500", longAgo)

	got, err := env.hrmAssetsSvc.GetAsset(ctx, fx.orgID, a.ID)
	if err != nil {
		t.Fatalf("get asset: %v", err)
	}
	if got.BookValue == nil || !got.BookValue.Equal(dec(t, "500")) {
		t.Errorf("book value = %v, want full 500 when the category sets no useful life", got.BookValue)
	}
}

// ============================================================
// Licence seats — seats_used derived, oversubscription refused
// ============================================================

func TestIntegration_Assets_LicenceSeatsAreDerivedAndCapped(t *testing.T) {
	env := newTestEnv(t)
	ctx := context.Background()
	fx := seedAssetFixture(t, env, nil)

	lic, err := env.hrmAssetsSvc.CreateLicense(ctx, fx.orgID, fx.ownerID, assets.CreateLicenseRequest{
		Name: "Design Suite " + uniqueSlug("lic"), SeatsTotal: 2,
	})
	if err != nil {
		t.Fatalf("create licence: %v", err)
	}
	if lic.SeatsUsed == nil || *lic.SeatsUsed != 0 {
		t.Fatalf("new licence seats_used = %v, want 0", lic.SeatsUsed)
	}

	emp2 := seedEmployee(t, env, fx.orgID, fx.statusID, fx.ownerID, "", "Seat Two", nil)
	emp3 := seedEmployee(t, env, fx.orgID, fx.statusID, fx.ownerID, "", "Seat Three", nil)

	for _, e := range []string{fx.employeeID, emp2} {
		if _, err := env.hrmAssetsSvc.AssignSeat(ctx, fx.orgID, lic.ID, fx.ownerID, assets.AssignSeatRequest{EmployeeID: e}); err != nil {
			t.Fatalf("assign seat to %s: %v", e, err)
		}
	}

	full, err := env.hrmAssetsSvc.GetLicense(ctx, fx.orgID, lic.ID)
	if err != nil {
		t.Fatalf("get licence: %v", err)
	}
	if full.SeatsUsed == nil || *full.SeatsUsed != 2 {
		t.Errorf("seats_used = %v, want 2 (counted live, not from a counter)", full.SeatsUsed)
	}

	// The third seat must be refused — the licence is full.
	if _, err := env.hrmAssetsSvc.AssignSeat(ctx, fx.orgID, lic.ID, fx.ownerID, assets.AssignSeatRequest{EmployeeID: emp3}); err == nil {
		t.Fatal("assigning a third seat on a 2-seat licence succeeded")
	}

	// Releasing one frees capacity, and seats_used follows without any counter update.
	if err := env.hrmAssetsSvc.ReleaseSeat(ctx, fx.orgID, lic.ID, fx.employeeID); err != nil {
		t.Fatalf("release seat: %v", err)
	}
	afterRelease, err := env.hrmAssetsSvc.GetLicense(ctx, fx.orgID, lic.ID)
	if err != nil {
		t.Fatalf("get licence: %v", err)
	}
	if afterRelease.SeatsUsed == nil || *afterRelease.SeatsUsed != 1 {
		t.Errorf("seats_used after release = %v, want 1", afterRelease.SeatsUsed)
	}
	if _, err := env.hrmAssetsSvc.AssignSeat(ctx, fx.orgID, lic.ID, fx.ownerID, assets.AssignSeatRequest{EmployeeID: emp3}); err != nil {
		t.Fatalf("assign seat after a release freed capacity: %v", err)
	}
}

func TestIntegration_Assets_CannotHoldTwoSeatsOnOneLicence(t *testing.T) {
	env := newTestEnv(t)
	ctx := context.Background()
	fx := seedAssetFixture(t, env, nil)

	lic, err := env.hrmAssetsSvc.CreateLicense(ctx, fx.orgID, fx.ownerID, assets.CreateLicenseRequest{
		Name: "IDE " + uniqueSlug("lic"), SeatsTotal: 5,
	})
	if err != nil {
		t.Fatalf("create licence: %v", err)
	}
	if _, err := env.hrmAssetsSvc.AssignSeat(ctx, fx.orgID, lic.ID, fx.ownerID, assets.AssignSeatRequest{EmployeeID: fx.employeeID}); err != nil {
		t.Fatalf("first seat: %v", err)
	}
	if _, err := env.hrmAssetsSvc.AssignSeat(ctx, fx.orgID, lic.ID, fx.ownerID, assets.AssignSeatRequest{EmployeeID: fx.employeeID}); err == nil {
		t.Error("the same employee took two seats on one licence")
	}
}

// ============================================================
// Handover sign-off — reuses hrm_acknowledgements
// ============================================================

// TestIntegration_Assets_AssignmentRequestsHandoverSignoff proves the
// acknowledgements reuse actually lands a row. This is also the regression
// test for the AckType drift found in 8A: migrations 00086/00094 widened the
// DB CHECK for 'appraisal'/'course_completion' without widening the Go enum,
// leaving both unreachable through Create()'s IsValid() gate. If
// 'asset_handover' were added to only one of the two, this test fails.
func TestIntegration_Assets_AssignmentRequestsHandoverSignoff(t *testing.T) {
	env := newTestEnv(t)
	ctx := context.Background()
	fx := seedAssetFixture(t, env, nil)
	a := seedAsset(t, env, fx, "Handover Laptop", "1500", "")

	if _, err := env.hrmAssetsSvc.AssignAsset(ctx, fx.orgID, a.ID, fx.ownerID, assets.AssignAssetRequest{
		EmployeeID: fx.employeeID,
	}); err != nil {
		t.Fatalf("assign: %v", err)
	}

	var count int
	var ackType, title string
	if err := env.db.QueryRow(ctx,
		`SELECT COUNT(*), COALESCE(MAX(acknowledgeable_type),''), COALESCE(MAX(entity_title),'')
		   FROM hrm_acknowledgements
		  WHERE org_id=$1 AND employee_id=$2 AND acknowledgeable_id=$3`,
		fx.orgID, fx.employeeID, a.ID).Scan(&count, &ackType, &title); err != nil {
		t.Fatalf("read acknowledgement: %v", err)
	}
	if count != 1 {
		t.Fatalf("expected exactly 1 handover acknowledgement, got %d", count)
	}
	if ackType != "asset_handover" {
		t.Errorf("acknowledgeable_type = %q, want asset_handover", ackType)
	}
	if title != "Handover Laptop" {
		t.Errorf("entity_title = %q, want the asset name", title)
	}
}

// TestIntegration_Assets_AckTypeEnumMatchesTheDatabaseCheck closes the drift
// that 8A found: every value the DB permits must be reachable through the
// only typed write path, or the migration that added it created a dead value.
func TestIntegration_Assets_AckTypeEnumMatchesTheDatabaseCheck(t *testing.T) {
	env := newTestEnv(t)
	ctx := context.Background()
	orgID, statusID, ownerID := seedScopeTestOrg(t, env)
	empID := seedEmployee(t, env, orgID, statusID, ownerID, "", "Ack Target", nil)

	// Every type the CHECK allows, exercised through acknowledgements.Create —
	// which gates on AckType.IsValid(). A value in the DB but not the enum
	// fails here.
	for _, ackType := range []string{
		"warning", "document", "announcement", "calendar_event", "policy",
		"appraisal", "course_completion", "asset_handover",
	} {
		if _, err := env.hrmAcksSvc.Create(ctx, orgID, ownerID, ackTypeRequest(empID, ackType)); err != nil {
			t.Errorf("acknowledgeable_type %q is permitted by the DB CHECK but rejected by the Go enum: %v", ackType, err)
		}
	}
}

// ============================================================
// Requests — approval-gated, fulfilment distinct from approval
// ============================================================

func TestIntegration_Assets_RequestApprovalAndFulfilment(t *testing.T) {
	env := newTestEnv(t)
	ctx := context.Background()
	fx := seedAssetFixture(t, env, nil)
	a := seedAsset(t, env, fx, "Requested Laptop", "1800", "")

	if _, err := env.hrmApprovalsSvc.CreateTemplate(ctx, fx.orgID, fx.ownerID, approvals.CreateTemplateRequest{
		Name: "Asset Request Approval", ActionType: approvals.ActionTypeAssetRequest, IsDefault: true,
		Levels: []approvals.CreateTemplateLevelRequest{
			{Level: 1, ApproverType: approvals.ApproverTypeSpecificUser, ApproverUserID: &fx.ownerID, SLAHours: 24, OnSLABreach: approvals.SLABreachEscalateNext},
		},
	}); err != nil {
		t.Fatalf("create approval template: %v", err)
	}

	// The owner is not linked to an employee record in this fixture, so
	// request self-service is exercised through the employee's own user.
	// seedScopeTestOrg's owner IS the caller here; link them.
	if _, err := env.db.Exec(ctx, `UPDATE hrm_employees SET user_id=$1 WHERE id=$2`, fx.ownerID, fx.employeeID); err != nil {
		t.Fatalf("link employee to user: %v", err)
	}

	rq, err := env.hrmAssetsSvc.RequestAsset(ctx, fx.orgID, fx.ownerID, assets.CreateAssetRequestRequest{
		CategoryID: &fx.categoryID,
	})
	if err != nil {
		t.Fatalf("request asset: %v", err)
	}
	if rq.EmployeeID != fx.employeeID {
		t.Errorf("request raised for %s, want the caller's own employee %s", rq.EmployeeID, fx.employeeID)
	}

	submitted, err := env.hrmAssetsSvc.SubmitRequest(ctx, fx.orgID, rq.ID, fx.ownerID)
	if err != nil {
		t.Fatalf("submit: %v", err)
	}
	if submitted.Status != assets.RequestPendingApproval {
		t.Fatalf("status = %s, want pending_approval", submitted.Status)
	}
	if submitted.ApprovalInstanceID == nil {
		t.Fatal("expected an approval_instance_id")
	}

	// Decide through the REAL approvals service — exercises the
	// RegisterCallback("asset_request", ...) wiring from newTestEnv.
	if _, err := env.hrmApprovalsSvc.Decide(ctx, fx.orgID, *submitted.ApprovalInstanceID, fx.ownerID,
		approvals.DecisionRequest{Action: "approved"}); err != nil {
		t.Fatalf("decide: %v", err)
	}

	approved, err := env.hrmAssetsSvc.GetRequest(ctx, fx.orgID, rq.ID)
	if err != nil {
		t.Fatalf("get request: %v", err)
	}
	if approved.Status != assets.RequestApproved {
		t.Fatalf("the approval decision did not reach the request via the callback: status = %s", approved.Status)
	}
	// Approval alone must NOT hand over an asset — fulfilment is a separate step.
	if approved.FulfilledAssetID != nil {
		t.Error("approval alone fulfilled the request; fulfilment must be a distinct call")
	}

	fulfilled, err := env.hrmAssetsSvc.FulfillRequest(ctx, fx.orgID, rq.ID, fx.ownerID, assets.FulfillRequestRequest{
		AssetID: a.ID,
	})
	if err != nil {
		t.Fatalf("fulfill: %v", err)
	}
	if fulfilled.Status != assets.RequestFulfilled {
		t.Errorf("status = %s, want fulfilled", fulfilled.Status)
	}
	if fulfilled.FulfilledAssetID == nil || *fulfilled.FulfilledAssetID != a.ID {
		t.Error("fulfilled_asset_id was not recorded")
	}

	// And the asset really is in that employee's hands now.
	held, err := env.hrmAssetsSvc.GetAsset(ctx, fx.orgID, a.ID)
	if err != nil {
		t.Fatalf("get asset: %v", err)
	}
	if held.CurrentHolderEmployeeID == nil || *held.CurrentHolderEmployeeID != fx.employeeID {
		t.Errorf("current holder = %v, want %s", held.CurrentHolderEmployeeID, fx.employeeID)
	}
}

// ============================================================
// Scope tiers
// ============================================================

func TestIntegration_Assets_ScopeOwnSeesOnlyOwnAssignments(t *testing.T) {
	env := newTestEnv(t)
	ctx := context.Background()
	fx := seedAssetFixture(t, env, nil)

	aliceEmail := uniqueEmail("asset-alice")
	alice, err := env.authSvc.Signup(ctx, authSignup(aliceEmail))
	if err != nil {
		t.Fatalf("signup alice: %v", err)
	}
	t.Cleanup(func() { cleanupUser(t, env, alice.ID) })
	aliceEmp := seedEmployee(t, env, fx.orgID, fx.statusID, fx.ownerID, alice.ID, "Alice", nil)

	aliceAsset := seedAsset(t, env, fx, "Alice Laptop", "1000", "")
	bobAsset := seedAsset(t, env, fx, "Bob Laptop", "1000", "")

	if _, err := env.hrmAssetsSvc.AssignAsset(ctx, fx.orgID, aliceAsset.ID, fx.ownerID, assets.AssignAssetRequest{EmployeeID: aliceEmp}); err != nil {
		t.Fatalf("assign to alice: %v", err)
	}
	if _, err := env.hrmAssetsSvc.AssignAsset(ctx, fx.orgID, bobAsset.ID, fx.ownerID, assets.AssignAssetRequest{EmployeeID: fx.employeeID}); err != nil {
		t.Fatalf("assign to bob: %v", err)
	}

	own, err := env.hrmAssetsSvc.ListAssignments(ctx, fx.orgID, assets.ListFilter{
		Scope: authz.ScopeOwn, CallerUserID: alice.ID,
	})
	if err != nil {
		t.Fatalf("list assignments with ScopeOwn: %v", err)
	}
	if len(own.Assignments) != 1 {
		t.Fatalf("ScopeOwn returned %d assignments, want exactly Alice's 1", len(own.Assignments))
	}
	if own.Assignments[0].EmployeeID != aliceEmp {
		t.Error("ScopeOwn returned someone else's assignment")
	}

	all, err := env.hrmAssetsSvc.ListAssignments(ctx, fx.orgID, assets.ListFilter{Scope: authz.ScopeAll})
	if err != nil {
		t.Fatalf("list assignments with ScopeAll: %v", err)
	}
	if len(all.Assignments) != 2 {
		t.Errorf("ScopeAll returned %d, want 2", len(all.Assignments))
	}
}
