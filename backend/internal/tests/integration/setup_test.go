// backend/internal/tests/integration/setup_test.go
// Integration test harness.
// Tests only run when INTEGRATION=1 is set.
// Requires a live Postgres + Redis reachable at DATABASE_URL / REDIS_URL.
package integration

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"

	"github.com/mridha/businesssaas/internal/audit"
	"github.com/mridha/businesssaas/internal/auth"
	"github.com/mridha/businesssaas/internal/authz"
	captureemail "github.com/mridha/businesssaas/internal/capture/email"
	"github.com/mridha/businesssaas/internal/config"
	crmdeals "github.com/mridha/businesssaas/internal/crm/deals"
	crmleads "github.com/mridha/businesssaas/internal/crm/leads"
	crmpipeline "github.com/mridha/businesssaas/internal/crm/pipeline"
	crmsettings "github.com/mridha/businesssaas/internal/crm/settings"
	hrmacks "github.com/mridha/businesssaas/internal/hrm/acknowledgements"
	hrmapprovals "github.com/mridha/businesssaas/internal/hrm/approvals"
	hrmassets "github.com/mridha/businesssaas/internal/hrm/assets"
	hrmbenefits "github.com/mridha/businesssaas/internal/hrm/benefits"
	hrmcertifications "github.com/mridha/businesssaas/internal/hrm/certifications"
	hrmcompensation "github.com/mridha/businesssaas/internal/hrm/compensation"
	hrmemployees "github.com/mridha/businesssaas/internal/hrm/employees"
	hrmexits "github.com/mridha/businesssaas/internal/hrm/exits"
	hrmexpenses "github.com/mridha/businesssaas/internal/hrm/expenses"
	hrmfeedback "github.com/mridha/businesssaas/internal/hrm/feedback"
	hrmlearning "github.com/mridha/businesssaas/internal/hrm/learning"
	hrmleave "github.com/mridha/businesssaas/internal/hrm/leave"
	hrmloans "github.com/mridha/businesssaas/internal/hrm/loans"
	hrmonboarding "github.com/mridha/businesssaas/internal/hrm/onboarding"
	hrmpayslips "github.com/mridha/businesssaas/internal/hrm/payslips"
	hrmperformance "github.com/mridha/businesssaas/internal/hrm/performance"
	hrmpip "github.com/mridha/businesssaas/internal/hrm/pip"
	hrmrecruitment "github.com/mridha/businesssaas/internal/hrm/recruitment"
	hrmreimbursements "github.com/mridha/businesssaas/internal/hrm/reimbursements"
	hrmresignations "github.com/mridha/businesssaas/internal/hrm/resignations"
	hrmscope "github.com/mridha/businesssaas/internal/hrm/scope"
	hrmskills "github.com/mridha/businesssaas/internal/hrm/skills"
	hrmstatutory "github.com/mridha/businesssaas/internal/hrm/statutory"
	hrmterminations "github.com/mridha/businesssaas/internal/hrm/terminations"
	"github.com/mridha/businesssaas/internal/organizations"
	"github.com/mridha/businesssaas/internal/platform/checklists"
	crmcontacts "github.com/mridha/businesssaas/internal/platform/contacts"
	platformengagement "github.com/mridha/businesssaas/internal/platform/engagement"
	"github.com/mridha/businesssaas/internal/platform/forms"
	"github.com/mridha/businesssaas/internal/platform/kb"
	"github.com/mridha/businesssaas/internal/platform/notifications"
	"github.com/mridha/businesssaas/internal/platform/tickets"
	"github.com/mridha/businesssaas/internal/task"
	"github.com/mridha/businesssaas/internal/user"
	jwtpkg "github.com/mridha/businesssaas/pkg/jwt"
)

// testEnv holds fully-wired services for integration tests.
type testEnv struct {
	db                   *pgxpool.Pool
	redis                *redis.Client
	authSvc              auth.Service
	userSvc              user.Service
	authzSvc             authz.Service
	orgSvc               organizations.Service
	taskSvc              task.Service
	checklistsSvc        checklists.Service
	hrmOnboardingSvc     hrmonboarding.Service
	hrmEmpSvc            hrmemployees.Service
	hrmApprovalsSvc      hrmapprovals.Service
	hrmRecruitmentSvc    hrmrecruitment.Service
	hrmTerminationSvc    hrmterminations.Service
	hrmPerformanceSvc    hrmperformance.Service
	hrmFeedbackSvc       hrmfeedback.Service
	hrmLearningSvc       hrmlearning.Service
	hrmSkillsSvc         hrmskills.Service
	hrmCertSvc           hrmcertifications.Service
	hrmPayslipsSvc       hrmpayslips.Service
	hrmCompensationSvc   hrmcompensation.Service
	hrmLeaveSvc          hrmleave.Service
	hrmLoansSvc          hrmloans.Service
	hrmReimbursementsSvc hrmreimbursements.Service
	hrmStatutorySvc      hrmstatutory.Service
	hrmBenefitsSvc       hrmbenefits.Service
	hrmAssetsSvc         hrmassets.Service
	hrmExpensesSvc       hrmexpenses.Service
	hrmAcksSvc           hrmacks.Service
	hrmPipSvc            hrmpip.Service
	hrmScopeResolver     *hrmscope.Resolver
	formsSvc             forms.Service
	ticketsSvc           tickets.Service
	kbSvc                kb.Service
	emailSvc             captureemail.Service
	leadsSvc             crmleads.Service
	hrmResignationSvc    hrmresignations.Service
	hrmExitsSvc          hrmexits.Service
}

// skipIfUnit gates all integration tests behind INTEGRATION=1.
func skipIfUnit(t *testing.T) {
	t.Helper()
	if os.Getenv("INTEGRATION") != "1" {
		t.Skip("skipping integration test (set INTEGRATION=1 to run)")
	}
}

// newTestEnv creates a fully-wired test environment connected to the live DB/Redis.
func newTestEnv(t *testing.T) *testEnv {
	t.Helper()
	skipIfUnit(t)

	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		dbURL = "postgres://saas_user:saas_password@localhost:5432/businesssaas_test"
	}
	redisURL := os.Getenv("REDIS_URL")
	if redisURL == "" {
		redisURL = "redis://localhost:6379/1"
	}

	ctx := context.Background()

	db, err := pgxpool.New(ctx, dbURL)
	if err != nil {
		t.Fatalf("connect to postgres: %v", err)
	}
	if err := db.Ping(ctx); err != nil {
		t.Fatalf("ping postgres: %v", err)
	}

	opt, err := redis.ParseURL(redisURL)
	if err != nil {
		t.Fatalf("parse redis URL: %v", err)
	}
	rdb := redis.NewClient(opt)
	if err := rdb.Ping(ctx).Err(); err != nil {
		t.Fatalf("ping redis: %v", err)
	}

	t.Cleanup(func() {
		db.Close()
		_ = rdb.Close()
	})

	jwtMgr := jwtpkg.NewManager("integration-test-secret-32bytesxx", 15*time.Minute)
	jwtCfg := config.JWTConfig{
		Secret:          "integration-test-secret-32bytesxx",
		AccessTokenTTL:  15 * time.Minute,
		RefreshTokenTTL: 7 * 24 * time.Hour,
	}
	auditSvc := audit.NewService(audit.NewRepository(db))

	userRepo := user.NewRepository(db)
	authRepo := auth.NewRepository(db)
	authzRepo := authz.NewRepository(db)
	orgRepo := organizations.NewRepository(db)
	taskRepo := task.NewRepository(db)
	notifRepo := notifications.NewRepository(db)
	notifSvc := notifications.NewService(config.NotificationsConfig{}, notifRepo)

	authzSvc := authz.NewService(authzRepo, rdb, auditSvc, authRepo)

	// checklistsSvc takes authzSvc directly as its AccessDirectory, mirroring
	// main.go's wiring — authz.Service satisfies that narrow interface
	// structurally, so no adapter is needed here either.
	checklistsRepo := checklists.NewRepository(db)
	checklistsSvc := checklists.NewService(checklistsRepo, authzSvc)

	hrmOnboardingRepo := hrmonboarding.NewRepository(db)
	hrmOnboardingSvc := hrmonboarding.NewService(hrmOnboardingRepo, checklistsSvc)

	hrmEmpRepo := hrmemployees.NewRepository(db)
	hrmEmpSvc := hrmemployees.NewService(hrmEmpRepo, auditSvc, hrmOnboardingSvc)

	hrmApprovalsRepo := hrmapprovals.NewRepository(db)
	hrmApprovalsSvc := hrmapprovals.NewService(hrmApprovalsRepo)

	// The scope resolver is constructed here rather than further down because
	// exits needs it, and recruitment needs exits — mirroring main.go's
	// dependency order.
	hrmScopeResolver := hrmscope.NewResolver(db)

	// hrmExitsSvc before recruitment: exits.Service satisfies
	// recruitment.RehireChecker structurally, so it is passed with no adapter,
	// exactly as in main.go. Wiring the REAL service rather than a stub is the
	// point — the claim under test is that a not-rehire-eligible leaver
	// actually surfaces on a candidate.
	// leave / loans / expenses are constructed HERE, ahead of exits, because
	// exits consumes all three as settlement sources — mirroring main.go's
	// dependency order. Wiring the REAL services rather than stubs is the
	// point: the claims under test are that recorded encashment becomes money,
	// that a loan actually forecloses, and that an advance is actually
	// recovered.
	hrmLeaveSvc := hrmleave.NewService(hrmleave.NewRepository(db), auditSvc, db)
	hrmLoansSvc := hrmloans.NewService(hrmloans.NewRepository(db), db, hrmApprovalsSvc)
	hrmReimbursementsSvc := hrmreimbursements.NewService(hrmreimbursements.NewRepository(db), db, hrmApprovalsSvc)
	hrmExpensesSvc := hrmexpenses.NewService(hrmexpenses.NewRepository(db), hrmApprovalsSvc, hrmReimbursementsSvc)
	hrmApprovalsSvc.RegisterCallback("travel_request", hrmExpensesSvc.HandleTravelApprovalDecision)
	hrmApprovalsSvc.RegisterCallback("expense_claim", hrmExpensesSvc.HandleClaimApprovalDecision)

	hrmExitsSvc := hrmexits.NewService(hrmexits.NewRepository(db), checklistsSvc, hrmScopeResolver,
		hrmLeaveSvc, hrmLoansSvc, hrmExpensesSvc)

	hrmRecruitmentRepo := hrmrecruitment.NewRepository(db)
	hrmRecruitmentSvc := hrmrecruitment.NewService(hrmRecruitmentRepo, hrmApprovalsSvc, hrmEmpSvc, hrmExitsSvc)
	hrmApprovalsSvc.RegisterCallback("job_requisition", hrmRecruitmentSvc.HandleApprovalDecision)
	hrmApprovalsSvc.RegisterCallback("offer", hrmRecruitmentSvc.HandleOfferApprovalDecision)

	hrmTerminationSvc := hrmterminations.NewService(hrmterminations.NewRepository(db), db, hrmApprovalsSvc)
	hrmApprovalsSvc.RegisterCallback("termination", hrmTerminationSvc.HandleApprovalDecision)
	hrmResignationSvc := hrmresignations.NewService(hrmresignations.NewRepository(db), db)

	// authzSvc satisfies forms.AccessDirectory structurally, mirroring main.go.
	formsSvc := forms.NewService(forms.NewRepository(db), authzSvc)
	// authzSvc satisfies tickets.AccessDirectory structurally too, mirroring
	// main.go — Can plus UserRoleName, the latter backing the
	// sensitive-category assignee gate.
	ticketsSvc := tickets.NewService(tickets.NewRepository(db), authzSvc)

	// The inbound-email pipeline with its REAL leads dependency, wired in the
	// same dependency order main.go uses. Stubbing leads here would defeat the
	// point: the claim under test is that lead capture keeps working.
	crmSettingsSvc := crmsettings.NewService(crmsettings.NewRepository(db))
	platformEngagementSvc := platformengagement.NewService(platformengagement.NewRepository(db))
	crmDealsSvc := crmdeals.NewService(crmdeals.NewRepository(db),
		crmpipeline.NewService(crmpipeline.NewRepository(db)), platformEngagementSvc)
	leadsSvc := crmleads.NewService(crmleads.NewRepository(db),
		crmcontacts.NewService(crmcontacts.NewRepository(db)), crmDealsSvc,
		crmSettingsSvc, platformEngagementSvc)
	// ticketsSvc satisfies captureemail.TicketRaiser structurally, mirroring
	// main.go — wiring the REAL ticket service is the point, since the claim
	// under test is that a support@ address produces a real ticket.
	emailSvc := captureemail.NewService(captureemail.NewRepository(db), leadsSvc, ticketsSvc)
	kbSvc := kb.NewService(kb.NewRepository(db), authzSvc)

	// *hrmscope.Resolver (built above, before exits) satisfies
	// performance.RecordAuthorizer structurally, mirroring main.go — no adapter.
	hrmPerformanceSvc := hrmperformance.NewService(hrmperformance.NewRepository(db), hrmScopeResolver, formsSvc)
	hrmFeedbackSvc := hrmfeedback.NewService(hrmfeedback.NewRepository(db), hrmScopeResolver, formsSvc)

	// hrmTerminationSvc satisfies pip.TerminationCreator structurally, exactly
	// as in main.go. Wiring the REAL service rather than a stub is the point:
	// the failed-PIP handoff is only proved by a draft actually landing in
	// hrm_terminations.
	hrmPipSvc := hrmpip.NewService(hrmpip.NewRepository(db), hrmScopeResolver, hrmTerminationSvc)
	hrmLearningSvc := hrmlearning.NewService(hrmlearning.NewRepository(db), hrmScopeResolver, formsSvc)
	// hrmSkillsSvc satisfies certifications.SkillGranter structurally, as in main.go.
	hrmSkillsSvc := hrmskills.NewService(hrmskills.NewRepository(db), hrmScopeResolver)
	hrmCertSvc := hrmcertifications.NewService(hrmcertifications.NewRepository(db), hrmScopeResolver, hrmSkillsSvc)

	// hrmCompensationSvc/hrmLoansSvc/hrmReimbursementsSvc satisfy
	// payslips.BonusSource/LoanSource/ReimbursementSource structurally,
	// exactly as in main.go — constructed first so they can be wired into
	// payslips.
	hrmCompensationSvc := hrmcompensation.NewService(hrmcompensation.NewRepository(db), db, hrmApprovalsSvc)
	// hrmStatutorySvc/hrmBenefitsSvc satisfy payslips.StatutorySource/
	// BenefitsSource structurally, exactly as in main.go.
	hrmStatutorySvc := hrmstatutory.NewService(hrmstatutory.NewRepository(db), hrmstatutory.NewRegistry(hrmstatutory.SlabProvider{}))
	hrmBenefitsSvc := hrmbenefits.NewService(hrmbenefits.NewRepository(db))
	// hrmExitsSvc satisfies payslips.FnFSource structurally, mirroring
	// main.go — wiring the REAL service is the point: the claim under test is
	// that an F&F run picks up its settlement from a real exit record.
	hrmPayslipsSvc := hrmpayslips.NewService(
		hrmpayslips.NewRepository(db), db, hrmCompensationSvc, hrmLoansSvc, hrmReimbursementsSvc,
		hrmStatutorySvc, hrmBenefitsSvc, hrmExitsSvc,
	)
	hrmApprovalsSvc.RegisterCallback("salary_revision", hrmCompensationSvc.HandleApprovalDecision)
	hrmApprovalsSvc.RegisterCallback("bonus", hrmCompensationSvc.HandleBonusApprovalDecision)
	hrmApprovalsSvc.RegisterCallback("loan", hrmLoansSvc.HandleApprovalDecision)
	hrmApprovalsSvc.RegisterCallback("reimbursement", hrmReimbursementsSvc.HandleApprovalDecision)

	// hrmAcksSvc satisfies assets.HandoverAcknowledger structurally, exactly as
	// in main.go. Wiring the REAL acknowledgements service rather than a stub is
	// the point — handover sign-off is only proved by a row actually landing in
	// hrm_acknowledgements. The hrmPipSvc/hrmTerminationSvc precedent.
	hrmAcksSvc := hrmacks.NewService(hrmacks.NewRepository(db), db)
	hrmAssetsSvc := hrmassets.NewService(hrmassets.NewRepository(db), hrmApprovalsSvc, hrmAcksSvc)
	hrmApprovalsSvc.RegisterCallback("asset_request", hrmAssetsSvc.HandleApprovalDecision)

	// hrmReimbursementsSvc satisfies expenses.ReimbursementCreator structurally,
	// exactly as in main.go. Wiring the REAL reimbursements service is the
	// point — the 7C boundary is only proved by a row actually landing in
	// hrm_reimbursements, ready for payroll to pay.

	return &testEnv{
		db:                   db,
		redis:                rdb,
		authSvc:              auth.NewService(authRepo, userRepo, jwtMgr, jwtCfg, auditSvc, notifSvc),
		userSvc:              user.NewService(userRepo),
		authzSvc:             authzSvc,
		orgSvc:               organizations.NewService(orgRepo, authzRepo, jwtMgr),
		taskSvc:              task.NewService(taskRepo, auditSvc),
		checklistsSvc:        checklistsSvc,
		hrmOnboardingSvc:     hrmOnboardingSvc,
		hrmEmpSvc:            hrmEmpSvc,
		hrmApprovalsSvc:      hrmApprovalsSvc,
		hrmRecruitmentSvc:    hrmRecruitmentSvc,
		hrmTerminationSvc:    hrmTerminationSvc,
		hrmResignationSvc:    hrmResignationSvc,
		hrmPerformanceSvc:    hrmPerformanceSvc,
		hrmFeedbackSvc:       hrmFeedbackSvc,
		hrmLearningSvc:       hrmLearningSvc,
		hrmSkillsSvc:         hrmSkillsSvc,
		hrmCertSvc:           hrmCertSvc,
		hrmPayslipsSvc:       hrmPayslipsSvc,
		hrmCompensationSvc:   hrmCompensationSvc,
		hrmLeaveSvc:          hrmLeaveSvc,
		hrmLoansSvc:          hrmLoansSvc,
		hrmReimbursementsSvc: hrmReimbursementsSvc,
		hrmStatutorySvc:      hrmStatutorySvc,
		hrmBenefitsSvc:       hrmBenefitsSvc,
		hrmAssetsSvc:         hrmAssetsSvc,
		hrmExpensesSvc:       hrmExpensesSvc,
		hrmAcksSvc:           hrmAcksSvc,
		hrmPipSvc:            hrmPipSvc,
		hrmScopeResolver:     hrmScopeResolver,
		hrmExitsSvc:          hrmExitsSvc,
		formsSvc:             formsSvc,
		ticketsSvc:           ticketsSvc,
		emailSvc:             emailSvc,
		kbSvc:                kbSvc,
		leadsSvc:             leadsSvc,
	}
}

// uniqueEmail generates a unique email for each test run to avoid conflicts.
func uniqueEmail(prefix string) string {
	return fmt.Sprintf("%s_%d@integration-test.local", prefix, time.Now().UnixNano())
}

// uniqueSlug generates a unique slug for each test run.
func uniqueSlug(prefix string) string {
	return fmt.Sprintf("%s-%d", prefix, time.Now().UnixNano()%999999)
}

// cleanupUser deletes a test user and all their sessions. Best-effort.
func cleanupUser(t *testing.T, env *testEnv, userID string) {
	t.Helper()
	ctx := context.Background()
	_ = env.authSvc.LogoutAll(ctx, userID)
	_, _ = env.db.Exec(ctx, `DELETE FROM users WHERE id = $1`, userID)
}
