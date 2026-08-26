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
	"github.com/mridha/businesssaas/internal/config"
	hrmapprovals "github.com/mridha/businesssaas/internal/hrm/approvals"
	hrmbenefits "github.com/mridha/businesssaas/internal/hrm/benefits"
	hrmcertifications "github.com/mridha/businesssaas/internal/hrm/certifications"
	hrmcompensation "github.com/mridha/businesssaas/internal/hrm/compensation"
	hrmemployees "github.com/mridha/businesssaas/internal/hrm/employees"
	hrmfeedback "github.com/mridha/businesssaas/internal/hrm/feedback"
	hrmlearning "github.com/mridha/businesssaas/internal/hrm/learning"
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
	"github.com/mridha/businesssaas/internal/platform/forms"
	"github.com/mridha/businesssaas/internal/platform/notifications"
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
	hrmLoansSvc          hrmloans.Service
	hrmReimbursementsSvc hrmreimbursements.Service
	hrmStatutorySvc      hrmstatutory.Service
	hrmBenefitsSvc       hrmbenefits.Service
	hrmPipSvc            hrmpip.Service
	hrmScopeResolver     *hrmscope.Resolver
	formsSvc             forms.Service
	hrmResignationSvc    hrmresignations.Service
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

	hrmRecruitmentRepo := hrmrecruitment.NewRepository(db)
	hrmRecruitmentSvc := hrmrecruitment.NewService(hrmRecruitmentRepo, hrmApprovalsSvc, hrmEmpSvc)
	hrmApprovalsSvc.RegisterCallback("job_requisition", hrmRecruitmentSvc.HandleApprovalDecision)
	hrmApprovalsSvc.RegisterCallback("offer", hrmRecruitmentSvc.HandleOfferApprovalDecision)

	hrmTerminationSvc := hrmterminations.NewService(hrmterminations.NewRepository(db), db, hrmApprovalsSvc)
	hrmApprovalsSvc.RegisterCallback("termination", hrmTerminationSvc.HandleApprovalDecision)
	hrmResignationSvc := hrmresignations.NewService(hrmresignations.NewRepository(db), db)

	// authzSvc satisfies forms.AccessDirectory structurally, mirroring main.go.
	formsSvc := forms.NewService(forms.NewRepository(db), authzSvc)

	// *hrmscope.Resolver satisfies performance.RecordAuthorizer structurally,
	// mirroring main.go — no adapter.
	hrmScopeResolver := hrmscope.NewResolver(db)
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
	hrmLoansSvc := hrmloans.NewService(hrmloans.NewRepository(db), db, hrmApprovalsSvc)
	hrmReimbursementsSvc := hrmreimbursements.NewService(hrmreimbursements.NewRepository(db), db, hrmApprovalsSvc)
	// hrmStatutorySvc/hrmBenefitsSvc satisfy payslips.StatutorySource/
	// BenefitsSource structurally, exactly as in main.go.
	hrmStatutorySvc := hrmstatutory.NewService(hrmstatutory.NewRepository(db), hrmstatutory.NewRegistry(hrmstatutory.SlabProvider{}))
	hrmBenefitsSvc := hrmbenefits.NewService(hrmbenefits.NewRepository(db))
	hrmPayslipsSvc := hrmpayslips.NewService(
		hrmpayslips.NewRepository(db), db, hrmCompensationSvc, hrmLoansSvc, hrmReimbursementsSvc,
		hrmStatutorySvc, hrmBenefitsSvc,
	)
	hrmApprovalsSvc.RegisterCallback("salary_revision", hrmCompensationSvc.HandleApprovalDecision)
	hrmApprovalsSvc.RegisterCallback("bonus", hrmCompensationSvc.HandleBonusApprovalDecision)
	hrmApprovalsSvc.RegisterCallback("loan", hrmLoansSvc.HandleApprovalDecision)
	hrmApprovalsSvc.RegisterCallback("reimbursement", hrmReimbursementsSvc.HandleApprovalDecision)

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
		hrmLoansSvc:          hrmLoansSvc,
		hrmReimbursementsSvc: hrmReimbursementsSvc,
		hrmStatutorySvc:      hrmStatutorySvc,
		hrmBenefitsSvc:       hrmBenefitsSvc,
		hrmPipSvc:            hrmPipSvc,
		hrmScopeResolver:     hrmScopeResolver,
		formsSvc:             formsSvc,
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
