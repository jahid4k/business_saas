// backend/cmd/server/main.go
//
// BusinessSAAS API — Multi-tenant modular Business Operating System.
//
// Swagger/OpenAPI annotations below are consumed by `swag init` to regenerate
// docs/swagger.json. Run `make docs` to regenerate after changing handler comments.
//
// @title           BusinessSAAS API
// @version         1.0.0
// @description     Multi-tenant modular Business Operating System (CRM, HRM, RBAC, Tasks, and more).
// @description
// @description     ### Authentication
// @description     Protected endpoints require `Authorization: Bearer <access_token>`.
// @description     Obtain via `POST /auth/login`; renew via `POST /auth/refresh` (httpOnly cookie).
// @description
// @description     ### Tenant isolation
// @description     Every org-scoped endpoint validates `:orgId` against `business_id` in the JWT.
// @description
// @description     ### Error envelope
// @description     `{ "success": false, "error": { "code": "...", "message": "..." }, "request_id": "..." }`
// @description     Use `code` in frontend switch statements.
//
// @contact.name    BusinessSAAS API Support
// @contact.email   api@businesssaas.dev
// @license.name    Proprietary
//
// @host            localhost:8080
// @BasePath        /api/v1
//
// @securityDefinitions.apikey  BearerAuth
// @in                          header
// @name                        Authorization
// @description                 Short-lived JWT. Format: Bearer <access_token>
package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"sort"
	"strings"
	"syscall"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/middleware/cors"
	"github.com/gofiber/fiber/v3/middleware/static"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/lmittmann/tint"
	"github.com/redis/go-redis/v9"

	// ── Internal — Bootstrap ──────────────────────────────────────────────────
	bsaasdocs "github.com/mridha/businesssaas/docs"
	"github.com/mridha/businesssaas/internal/audit"
	"github.com/mridha/businesssaas/internal/auth"
	"github.com/mridha/businesssaas/internal/authz"
	"github.com/mridha/businesssaas/internal/config"
	"github.com/mridha/businesssaas/internal/dashboard"
	"github.com/mridha/businesssaas/internal/database"
	"github.com/mridha/businesssaas/internal/middleware"
	"github.com/mridha/businesssaas/internal/organizations"
	"github.com/mridha/businesssaas/internal/security"
	"github.com/mridha/businesssaas/internal/task"
	"github.com/mridha/businesssaas/internal/user"
	jwtpkg "github.com/mridha/businesssaas/pkg/jwt"
	"github.com/mridha/businesssaas/pkg/response"

	// ── Internal — Platform (shared across modules) ───────────────────────────
	"github.com/mridha/businesssaas/internal/platform/checklists"
	"github.com/mridha/businesssaas/internal/platform/contacts"
	"github.com/mridha/businesssaas/internal/platform/engagement"
	"github.com/mridha/businesssaas/internal/platform/forms"
	"github.com/mridha/businesssaas/internal/platform/notifications"
	"github.com/mridha/businesssaas/internal/platform/scheduler"

	// ── Internal — Capture ────────────────────────────────────────────────────
	"github.com/mridha/businesssaas/internal/capture/apikeys"
	"github.com/mridha/businesssaas/internal/capture/email"
	"github.com/mridha/businesssaas/internal/capture/public"
	"github.com/mridha/businesssaas/internal/capture/social"
	"github.com/mridha/businesssaas/internal/capture/visitors"

	// ── Internal — CRM ────────────────────────────────────────────────────────
	crmdeals "github.com/mridha/businesssaas/internal/crm/deals"
	crmleads "github.com/mridha/businesssaas/internal/crm/leads"
	crmpipeline "github.com/mridha/businesssaas/internal/crm/pipeline"
	crmreports "github.com/mridha/businesssaas/internal/crm/reports"
	crmsettings "github.com/mridha/businesssaas/internal/crm/settings"
	crmtemplates "github.com/mridha/businesssaas/internal/crm/templates"

	// ── Internal — HRM Phase 1 (Core Employee Management) ────────────────────
	hrmdepts "github.com/mridha/businesssaas/internal/hrm/departments"
	hrmemployees "github.com/mridha/businesssaas/internal/hrm/employees"
	hrmleave "github.com/mridha/businesssaas/internal/hrm/leave"
	hrmonboarding "github.com/mridha/businesssaas/internal/hrm/onboarding"
	hrmpositions "github.com/mridha/businesssaas/internal/hrm/positions"
	hrmreports "github.com/mridha/businesssaas/internal/hrm/reports"

	// ── Internal — HRM Group A (Config / Setup) ───────────────────────────────
	hrmapprovals "github.com/mridha/businesssaas/internal/hrm/approvals"
	hrmcontracts "github.com/mridha/businesssaas/internal/hrm/contracts"
	hrmdoctmpls "github.com/mridha/businesssaas/internal/hrm/doctemplates"
	hrmholidays "github.com/mridha/businesssaas/internal/hrm/holidays"
	hrmsalary "github.com/mridha/businesssaas/internal/hrm/salary"
	hrmscope "github.com/mridha/businesssaas/internal/hrm/scope"
	hrmshifts "github.com/mridha/businesssaas/internal/hrm/shifts"
	hrmwarntypes "github.com/mridha/businesssaas/internal/hrm/warningtypes"

	// ── Internal — HRM Group B (Core Employee Lifecycle) ─────────────────────
	hrmcertifications "github.com/mridha/businesssaas/internal/hrm/certifications"
	hrmfeedback "github.com/mridha/businesssaas/internal/hrm/feedback"
	hrmlearning "github.com/mridha/businesssaas/internal/hrm/learning"
	hrmperformance "github.com/mridha/businesssaas/internal/hrm/performance"
	hrmpip "github.com/mridha/businesssaas/internal/hrm/pip"
	hrmpromotions "github.com/mridha/businesssaas/internal/hrm/promotions"
	hrmrecruitment "github.com/mridha/businesssaas/internal/hrm/recruitment"
	hrmresignations "github.com/mridha/businesssaas/internal/hrm/resignations"
	hrmskills "github.com/mridha/businesssaas/internal/hrm/skills"
	hrmterminations "github.com/mridha/businesssaas/internal/hrm/terminations"
	hrmtransfers "github.com/mridha/businesssaas/internal/hrm/transfers"

	// ── Internal — HRM Group C (Disciplinary and Compliance) ─────────────────
	hrmacks "github.com/mridha/businesssaas/internal/hrm/acknowledgements"
	hrmcomplaints "github.com/mridha/businesssaas/internal/hrm/complaints"
	hrmemployeedocs "github.com/mridha/businesssaas/internal/hrm/employeedocs"
	hrmwarnings "github.com/mridha/businesssaas/internal/hrm/warnings"

	// ── Internal — HRM Group D (Time and Compensation) ───────────────────────
	hrmattendance "github.com/mridha/businesssaas/internal/hrm/attendance"
	hrmpayslips "github.com/mridha/businesssaas/internal/hrm/payslips"

	// ── Internal — HRM Extended Phase 7B (Compensation) ───────────────────────
	hrmcompensation "github.com/mridha/businesssaas/internal/hrm/compensation"

	// ── Internal — HRM Extended Phase 7C (Loans + Reimbursements) ─────────────
	hrmloans "github.com/mridha/businesssaas/internal/hrm/loans"
	hrmreimbursements "github.com/mridha/businesssaas/internal/hrm/reimbursements"

	// ── Internal — HRM Extended Phase 7D (Statutory + Benefits) ───────────────
	hrmbenefits "github.com/mridha/businesssaas/internal/hrm/benefits"
	hrmstatutory "github.com/mridha/businesssaas/internal/hrm/statutory"

	// ── Internal — HRM Group E (Recognition and Communication) ───────────────
	hrmannouncements "github.com/mridha/businesssaas/internal/hrm/announcements"
	hrmawards "github.com/mridha/businesssaas/internal/hrm/awards"
	hrmcalendar "github.com/mridha/businesssaas/internal/hrm/calendar"
	hrmmilestones "github.com/mridha/businesssaas/internal/hrm/milestones"
)

// scalarHTML is the Scalar API reference UI served in development.
// Scalar loads from CDN — it requires internet access in dev.
// For air-gapped setups, replace the CDN script with a local copy.
const scalarHTML = `<!doctype html>
<html lang="en">
<head>
  <meta charset="utf-8"/>
  <meta name="viewport" content="width=device-width, initial-scale=1"/>
  <title>BusinessSAAS API Reference</title>
  <style>body{margin:0}</style>
</head>
<body>
  <script
    id="api-reference"
    data-url="/api/v1/docs/openapi.json"
    data-configuration='{
      "theme": "purple",
      "darkMode": true,
      "metaData": {"title": "BusinessSAAS API Reference"},
      "hideModels": false,
      "showSidebar": true,
      "defaultOpenAllTags": false
    }'></script>
  <script src="https://cdn.jsdelivr.net/npm/@scalar/api-reference"></script>
</body>
</html>`

func main() {
	// ── 1. Config ─────────────────────────────────────────────────────────────
	cfg, err := config.Load()
	if err != nil {
		slog.Error("failed to load configuration", slog.Any("error", err))
		os.Exit(1)
	}

	// ── 2. Logger ─────────────────────────────────────────────────────────────
	setupLogger(cfg.App.IsDevelopment())
	slog.Info("starting BusinessSAAS backend",
		slog.String("env", cfg.App.Env),
		slog.String("port", cfg.App.Port),
	)

	// ── 3. PostgreSQL ─────────────────────────────────────────────────────────
	ctx := context.Background()
	pgPool, err := database.NewPostgresPool(ctx, cfg.Database)
	if err != nil {
		slog.Error("failed to connect to PostgreSQL", slog.Any("error", err))
		os.Exit(1)
	}
	defer pgPool.Close()

	// ── 4. Redis ──────────────────────────────────────────────────────────────
	redisClient, err := database.NewRedisClient(ctx, cfg.Redis)
	if err != nil {
		slog.Error("failed to connect to Redis", slog.Any("error", err))
		os.Exit(1)
	}
	defer func() {
		if closeErr := redisClient.Close(); closeErr != nil {
			slog.Warn("redis close error", slog.Any("error", closeErr))
		}
	}()

	// ── 5. JWT + core middleware ──────────────────────────────────────────────
	jwtManager := jwtpkg.NewManager(cfg.JWT.Secret, cfg.JWT.AccessTokenTTL)
	requireAuth := middleware.RequireAuth(jwtManager)
	authRateLimit := middleware.NewAuthRateLimit(redisClient)

	// ═════════════════════════════════════════════════════════════════════════
	// 6. REPOSITORIES
	// ═════════════════════════════════════════════════════════════════════════

	// ── Core ──────────────────────────────────────────────────────────────────
	authRepo := auth.NewRepository(pgPool)
	userRepo := user.NewRepository(pgPool)
	avatarRepo := user.NewAvatarRepository(pgPool)
	authzRepo := authz.NewRepository(pgPool)
	businessRepo := organizations.NewRepository(pgPool)
	dashboardRepo := dashboard.NewRepository(pgPool)
	auditRepo := audit.NewRepository(pgPool)
	securityRepo := security.NewRepository(pgPool)
	taskRepo := task.NewRepository(pgPool)

	// ── Platform ──────────────────────────────────────────────────────────────
	checklistsRepo := checklists.NewRepository(pgPool)
	formsRepo := forms.NewRepository(pgPool)
	contactsRepo := contacts.NewRepository(pgPool)
	engagementRepo := engagement.NewRepository(pgPool)
	schedulerRepo := scheduler.NewRepository(pgPool)

	// ── CRM ───────────────────────────────────────────────────────────────────
	pipelineRepo := crmpipeline.NewRepository(pgPool)
	dealsRepo := crmdeals.NewRepository(pgPool)
	leadsRepo := crmleads.NewRepository(pgPool)
	reportsRepo := crmreports.NewRepository(pgPool)
	crmTemplatesRepo := crmtemplates.NewRepository(pgPool)
	crmSettingsRepo := crmsettings.NewRepository(pgPool)

	// ── Capture ───────────────────────────────────────────────────────────────
	apikeysRepo := apikeys.NewRepository(pgPool)
	emailRepo := email.NewRepository(pgPool)
	socialRepo := social.NewRepository(pgPool)
	visitorsRepo := visitors.NewRepository(pgPool)

	// ── HRM Phase 1 — Core Employee Management ────────────────────────────────
	// Departments, Positions, Employees, Leave, Reports
	// NOTE: If any Phase 1 service has additional dependencies (e.g. auditSvc),
	// adjust the NewService call after confirming the actual signature.
	hrmDeptsRepo := hrmdepts.NewRepository(pgPool)
	hrmPosRepo := hrmpositions.NewRepository(pgPool)
	hrmEmpRepo := hrmemployees.NewRepository(pgPool)
	hrmLeaveRepo := hrmleave.NewRepository(pgPool)
	hrmOnboardingRepo := hrmonboarding.NewRepository(pgPool)
	hrmReportsRepo := hrmreports.NewRepository(pgPool)

	// ── HRM Group A — Config / Setup (migrations 00021–00028) ────────────────
	hrmSalaryRepo := hrmsalary.NewRepository(pgPool)
	hrmApprovalsRepo := hrmapprovals.NewRepository(pgPool)
	hrmWarnTypesRepo := hrmwarntypes.NewRepository(pgPool)
	hrmDocTmplsRepo := hrmdoctmpls.NewRepository(pgPool)
	hrmShiftsRepo := hrmshifts.NewRepository(pgPool)
	hrmHolidaysRepo := hrmholidays.NewRepository(pgPool)
	hrmContractsRepo := hrmcontracts.NewRepository(pgPool)

	// ── HRM Group B — Core Employee Lifecycle (migrations 00029–00033) ────────
	hrmPromotionsRepo := hrmpromotions.NewRepository(pgPool)
	hrmTransfersRepo := hrmtransfers.NewRepository(pgPool)
	hrmResignationsRepo := hrmresignations.NewRepository(pgPool)
	hrmTerminationsRepo := hrmterminations.NewRepository(pgPool)

	// ── HRM Group C — Disciplinary and Compliance (migrations 00034–00037) ────
	hrmWarningsRepo := hrmwarnings.NewRepository(pgPool)
	hrmCplRepo := hrmcomplaints.NewRepository(pgPool)
	hrmEmpDocsRepo := hrmemployeedocs.NewRepository(pgPool)
	hrmAcksRepo := hrmacks.NewRepository(pgPool)

	// ── HRM Group D — Time and Compensation (migrations 00038–00040) ──────────
	hrmAttendanceRepo := hrmattendance.NewRepository(pgPool)
	hrmPayslipsRepo := hrmpayslips.NewRepository(pgPool)

	// ── HRM Extended Phase 7B — Compensation (migrations 00098–00099) ─────────
	hrmCompensationRepo := hrmcompensation.NewRepository(pgPool)

	// ── HRM Extended Phase 7C — Loans + Reimbursements (migrations 00100–00101) ─
	hrmLoansRepo := hrmloans.NewRepository(pgPool)
	hrmReimbursementsRepo := hrmreimbursements.NewRepository(pgPool)

	// ── HRM Extended Phase 7D — Statutory + Benefits (migrations 00102–00105) ──
	hrmStatutoryRepo := hrmstatutory.NewRepository(pgPool)
	hrmBenefitsRepo := hrmbenefits.NewRepository(pgPool)

	// ── HRM Group E — Recognition and Communication (migrations 00041–00045) ──
	hrmAwardsRepo := hrmawards.NewRepository(pgPool)
	hrmAnnsRepo := hrmannouncements.NewRepository(pgPool)
	hrmCalRepo := hrmcalendar.NewRepository(pgPool)
	hrmMilestonesRepo := hrmmilestones.NewRepository(pgPool)

	// ── HRM Extended Phase 4A — Recruitment / ATS (migration 00078) ───────────
	hrmRecruitmentRepo := hrmrecruitment.NewRepository(pgPool)
	hrmPerformanceRepo := hrmperformance.NewRepository(pgPool)
	hrmFeedbackRepo := hrmfeedback.NewRepository(pgPool)
	hrmLearningRepo := hrmlearning.NewRepository(pgPool)
	hrmSkillsRepo := hrmskills.NewRepository(pgPool)
	hrmCertificationsRepo := hrmcertifications.NewRepository(pgPool)
	hrmPipRepo := hrmpip.NewRepository(pgPool)

	// ═════════════════════════════════════════════════════════════════════════
	// 7. SERVICES   (dependency order: leaf → composite)
	// ═════════════════════════════════════════════════════════════════════════

	// ── Core ──────────────────────────────────────────────────────────────────
	auditSvc := audit.NewService(auditRepo)
	userSvc := user.NewService(userRepo)
	avatarSvc := user.NewAvatarService(avatarRepo)

	// ── Platform ──────────────────────────────────────────────────────────────
	notifRepo := notifications.NewRepository(pgPool)
	notifSvc := notifications.NewService(cfg.Notifications, notifRepo)

	authSvc := auth.NewService(authRepo, userRepo, jwtManager, cfg.JWT, auditSvc, notifSvc)
	authzSvc := authz.NewService(authzRepo, redisClient, auditSvc, authRepo)
	hrmScopeResolver := hrmscope.NewResolver(pgPool)
	businessSvc := organizations.NewService(businessRepo, authzRepo, jwtManager)
	dashboardSvc := dashboard.NewService(dashboardRepo)
	securitySvc := security.NewService(securityRepo)
	taskSvc := task.NewService(taskRepo, auditSvc)

	// ── Platform ──────────────────────────────────────────────────────────────
	// checklistsSvc takes authzSvc directly as its AccessDirectory — authz.Service
	// satisfies that narrow interface structurally, so no adapter is needed.
	checklistsSvc := checklists.NewService(checklistsRepo, authzSvc)
	// authzSvc satisfies forms.AccessDirectory structurally, exactly as it
	// does checklists.AccessDirectory — no adapter needed.
	formsSvc := forms.NewService(formsRepo, authzSvc)
	contactsSvc := contacts.NewService(contactsRepo)
	engagementSvc := engagement.NewService(engagementRepo)
	schedulerSvc := scheduler.NewService(schedulerRepo, redisClient)

	// ── CRM (wire in dependency order) ────────────────────────────────────────
	crmSettingsSvc := crmsettings.NewService(crmSettingsRepo)
	pipelineSvc := crmpipeline.NewService(pipelineRepo)
	dealsSvc := crmdeals.NewService(dealsRepo, pipelineSvc, engagementSvc)
	leadsSvc := crmleads.NewService(leadsRepo, contactsSvc, dealsSvc, crmSettingsSvc, engagementSvc)
	crmRptSvc := crmreports.NewService(reportsRepo, dealsSvc, leadsSvc, engagementSvc)
	crmTemplatesSvc := crmtemplates.NewService(crmTemplatesRepo)

	// ── Capture ───────────────────────────────────────────────────────────────
	apikeysSvc := apikeys.NewService(apikeysRepo)
	emailSvc := email.NewService(emailRepo, leadsSvc)
	socialSvc := social.NewService(socialRepo, leadsSvc, cfg.Social)
	visitorsSvc := visitors.NewService(visitorsRepo, leadsSvc)

	// ── HRM Phase 1 ───────────────────────────────────────────────────────────
	// If your Phase 1 services require auditSvc, replace NewService(repo) with
	// NewService(repo, auditSvc) below.
	hrmDeptsSvc := hrmdepts.NewService(hrmDeptsRepo)
	hrmPosSvc := hrmpositions.NewService(hrmPosRepo)
	// hrmOnboardingSvc must be constructed before hrmEmpSvc — it is passed in
	// as hrmEmpSvc's ChecklistHook (Phase 3). Building it here, not in a Group
	// block below, is deliberate: doing it later would compile fine but wire
	// a nil hook into hrmEmpSvc, a silent no-op rather than a build error.
	hrmOnboardingSvc := hrmonboarding.NewService(hrmOnboardingRepo, checklistsSvc)
	hrmEmpSvc := hrmemployees.NewService(hrmEmpRepo, auditSvc, hrmOnboardingSvc)
	hrmLeaveSvc := hrmleave.NewService(hrmLeaveRepo, auditSvc, pgPool)
	hrmPhase1Rpts := hrmreports.NewService(hrmReportsRepo)

	// ── HRM Group A — config-layer services; no cross-module dependencies ──────
	hrmSalarySvc := hrmsalary.NewService(hrmSalaryRepo)
	hrmApprovalsSvc := hrmapprovals.NewService(hrmApprovalsRepo)
	hrmWarnTypesSvc := hrmwarntypes.NewService(hrmWarnTypesRepo)
	hrmDocTmplsSvc := hrmdoctmpls.NewService(hrmDocTmplsRepo)
	hrmShiftsSvc := hrmshifts.NewService(hrmShiftsRepo)
	hrmHolidaysSvc := hrmholidays.NewService(hrmHolidaysRepo)
	hrmContractsSvc := hrmcontracts.NewService(hrmContractsRepo)

	// ── HRM Group B — lifecycle services; all need pgPool for Apply() txn ─────
	// promotions/transfers/terminations also take hrmApprovalsSvc (Group A, wired
	// above) so Submit() can route into an approval chain when one is configured.
	hrmPromotionsSvc := hrmpromotions.NewService(hrmPromotionsRepo, pgPool, hrmApprovalsSvc)
	hrmTransfersSvc := hrmtransfers.NewService(hrmTransfersRepo, pgPool, hrmApprovalsSvc)
	hrmResignationsSvc := hrmresignations.NewService(hrmResignationsRepo, pgPool)
	hrmTerminationsSvc := hrmterminations.NewService(hrmTerminationsRepo, pgPool, hrmApprovalsSvc)

	// ── HRM Group C — disciplinary; all need pgPool for cross-table writes ─────
	// warnings also takes hrmApprovalsSvc so Issue() can route into an approval
	// chain when the warning type has requires_hr_approval=true.
	hrmWarningsSvc := hrmwarnings.NewService(hrmWarningsRepo, pgPool, hrmApprovalsSvc)
	hrmCplSvc := hrmcomplaints.NewService(hrmCplRepo, pgPool)
	hrmEmpDocsSvc := hrmemployeedocs.NewService(hrmEmpDocsRepo, pgPool)
	hrmAcksSvc := hrmacks.NewService(hrmAcksRepo, pgPool)

	// ── HRM Extended Phase 7B — Compensation ───────────────────────────────────
	// Constructed before Group D so it can be wired into payslips as its
	// BonusSource below — payslips is the CONSUMER of that narrow interface
	// (see payslips.BonusSource's doc comment), so compensation must exist
	// first. compensation imports payslips (not the reverse) to reference
	// payslips.PendingBonus/PaidBonusLine — the recruitment.EmployeeCreator /
	// pip.TerminationCreator direction.
	hrmCompensationSvc := hrmcompensation.NewService(hrmCompensationRepo, pgPool, hrmApprovalsSvc)

	// ── HRM Extended Phase 7C — Loans + Reimbursements ─────────────────────────
	// Same reasoning and same construction-order requirement as compensation
	// above — payslips is the CONSUMER of LoanSource/ReimbursementSource, so
	// both must exist before hrmPayslipsSvc is constructed.
	hrmLoansSvc := hrmloans.NewService(hrmLoansRepo, pgPool, hrmApprovalsSvc)
	hrmReimbursementsSvc := hrmreimbursements.NewService(hrmReimbursementsRepo, pgPool, hrmApprovalsSvc)

	// ── HRM Extended Phase 7D — Statutory + Benefits ───────────────────────────
	// Same construction-order requirement — payslips is the CONSUMER of
	// StatutorySource/BenefitsSource. hrmStatutoryRegistry ships with
	// SlabProvider as the fallback for every country_code; a real
	// country-specific Provider (proration rules, eligibility thresholds a
	// slab table cannot express) registers here later without a schema
	// change — see internal/hrm/statutory/provider.go's doc comment.
	hrmStatutoryRegistry := hrmstatutory.NewRegistry(hrmstatutory.SlabProvider{})
	hrmStatutorySvc := hrmstatutory.NewService(hrmStatutoryRepo, hrmStatutoryRegistry)
	hrmBenefitsSvc := hrmbenefits.NewService(hrmBenefitsRepo)

	// ── HRM Group D — time and compensation ───────────────────────────────────
	// D1 attendance: pgPool for shift resolution queries
	// D2 payslips:   pgPool for formula engine (employee+salary+attendance queries)
	//                bonusSource = hrmCompensationSvc, feeding run_type='bonus' runs.
	//                loanSource / reimbursementSource / statutorySource /
	//                benefitsSource feed every OTHER run type.
	hrmAttendanceSvc := hrmattendance.NewService(hrmAttendanceRepo, pgPool)
	hrmPayslipsSvc := hrmpayslips.NewService(
		hrmPayslipsRepo, pgPool, hrmCompensationSvc, hrmLoansSvc, hrmReimbursementsSvc,
		hrmStatutorySvc, hrmBenefitsSvc,
	)

	// ── HRM Group E — recognition and communication ───────────────────────────
	// E1 awards:         pgPool for auto-creating E2 announcement on Issue()
	// E2 announcements:  pgPool for resolving target employees + C4 ack inserts
	// E3 calendar:       pgPool for RSVP → C4 ack inserts
	// E4 milestones:     pgPool for A7 contract reads in GenerateUpcoming()
	// awards also takes hrmApprovalsSvc so Submit() can route into an approval chain.
	hrmAwardsSvc := hrmawards.NewService(hrmAwardsRepo, pgPool, hrmApprovalsSvc)
	hrmAnnsSvc := hrmannouncements.NewService(hrmAnnsRepo, pgPool)
	hrmCalSvc := hrmcalendar.NewService(hrmCalRepo, pgPool)
	hrmMilestonesSvc := hrmmilestones.NewService(hrmMilestonesRepo, pgPool)

	// ── HRM Extended Phase 4A/4B — Recruitment / ATS ───────────────────────────
	// Requisitions and offers are approval-gated the same way promotions/
	// transfers/etc. are — takes hrmApprovalsSvc so SubmitRequisition()/
	// SubmitOffer() can route into an approval chain when one is configured.
	// hrmEmpSvc (constructed above, in the HRM Phase 1 block) satisfies
	// recruitment.EmployeeCreator structurally — HireApplication uses it to
	// materialize an employee record from a hired application.
	hrmRecruitmentSvc := hrmrecruitment.NewService(hrmRecruitmentRepo, hrmApprovalsSvc, hrmEmpSvc)

	// ── HRM Extended Phase 5A — Performance / Goals ────────────────────────────
	// hrmScopeResolver satisfies performance.RecordAuthorizer structurally, so
	// it is passed directly with no adapter — the same shape as authzSvc
	// satisfying checklists.AccessDirectory. The service takes no authz.Service
	// of its own: the handler resolves the caller's scope tier and manage
	// permission and hands both over on a Caller value.
	// formsSvc (Platform block above) satisfies performance.FormEngine
	// structurally — appraisals instantiate self/manager forms through it.
	hrmPerformanceSvc := hrmperformance.NewService(hrmPerformanceRepo, hrmScopeResolver, formsSvc)

	// ── HRM Extended Phase 5C — 360 feedback + PIP ─────────────────────────────
	// formsSvc satisfies feedback.FormReader structurally. The feedback service
	// reads form instances SERVER-SIDE through it and strips identity before
	// returning anything, which is why no form instance id ever reaches a
	// subject — see internal/hrm/feedback/model.go's anonymity contract.
	hrmFeedbackSvc := hrmfeedback.NewService(hrmFeedbackRepo, hrmScopeResolver, formsSvc)

	// hrmTerminationsSvc satisfies pip.TerminationCreator structurally, via
	// CreateDraftFromPIP. The interface is declared in internal/hrm/pip and
	// terminations imports pip, not the reverse — the consumer-owned narrow
	// interface direction, matching recruitment.EmployeeCreator. A failed PIP
	// creates a DRAFT termination and stops; Submit and Apply stay on the
	// termination endpoints, behind the approval chain.
	hrmPipSvc := hrmpip.NewService(hrmPipRepo, hrmScopeResolver, hrmTerminationsSvc)

	// ── HRM Extended Phase 6A — Learning & Development ─────────────────────────
	// formsSvc satisfies learning.FormEngine structurally. Quizzes are form
	// instances, but the CORRECT ANSWERS live in hrm_quiz_answer_keys, owned by
	// the learning package — platform/forms has no concept of a correct answer,
	// and appraisals and 360 feedback carry no assessment columns as a result.
	hrmLearningSvc := hrmlearning.NewService(hrmLearningRepo, hrmScopeResolver, formsSvc)

	// ── HRM Extended Phase 6B — Certifications + the skills taxonomy ───────────
	// hrmSkillsSvc satisfies certifications.SkillGranter structurally, so
	// issuing a credential that carries a skill records that skill too. The
	// import runs certifications → skills and never the reverse: skills is a
	// SHARED taxonomy, and Phase 10 succession will consume it the same way.
	hrmSkillsSvc := hrmskills.NewService(hrmSkillsRepo, hrmScopeResolver)
	hrmCertificationsSvc := hrmcertifications.NewService(hrmCertificationsRepo, hrmScopeResolver, hrmSkillsSvc)

	// Wire approval-instance completion back into each of the seven workflow
	// modules. Must run after all seven services above exist. entityType here
	// must match the EntityType string each Submit()/Issue() uses when calling
	// approvalsSvc.CreateInstance — see each module's service.go.
	hrmApprovalsSvc.RegisterCallback("promotion", hrmPromotionsSvc.HandleApprovalDecision)
	hrmApprovalsSvc.RegisterCallback("transfer", hrmTransfersSvc.HandleApprovalDecision)
	hrmApprovalsSvc.RegisterCallback("termination", hrmTerminationsSvc.HandleApprovalDecision)
	hrmApprovalsSvc.RegisterCallback("warning", hrmWarningsSvc.HandleApprovalDecision)
	hrmApprovalsSvc.RegisterCallback("award", hrmAwardsSvc.HandleApprovalDecision)
	hrmApprovalsSvc.RegisterCallback("job_requisition", hrmRecruitmentSvc.HandleApprovalDecision)
	hrmApprovalsSvc.RegisterCallback("offer", hrmRecruitmentSvc.HandleOfferApprovalDecision)
	hrmApprovalsSvc.RegisterCallback("salary_revision", hrmCompensationSvc.HandleApprovalDecision)
	hrmApprovalsSvc.RegisterCallback("bonus", hrmCompensationSvc.HandleBonusApprovalDecision)
	hrmApprovalsSvc.RegisterCallback("loan", hrmLoansSvc.HandleApprovalDecision)
	hrmApprovalsSvc.RegisterCallback("reimbursement", hrmReimbursementsSvc.HandleApprovalDecision)

	// ═════════════════════════════════════════════════════════════════════════
	// 8. HANDLERS
	// ═════════════════════════════════════════════════════════════════════════

	// ── Core ──────────────────────────────────────────────────────────────────
	authHandler := auth.NewHandler(authSvc, cfg.Cookie)
	userHandler := user.NewHandler(userSvc, avatarSvc)
	authzHandler := authz.NewHandler(authzSvc)
	businessHandler := organizations.NewHandler(businessSvc)
	dashboardHandler := dashboard.NewHandler(dashboardSvc)
	securityHandler := security.NewHandler(securitySvc)
	taskHandler := task.NewHandler(taskSvc, authzSvc)

	// ── Platform ──────────────────────────────────────────────────────────────
	checklistsHandler := checklists.NewHandler(checklistsSvc)
	formsHandler := forms.NewHandler(formsSvc)
	contactsHandler := contacts.NewHandler(contactsSvc)
	// When HRM arrives use: engagement.NewHandler(engagementSvc, "hrm")
	engagementHandler := engagement.NewHandler(engagementSvc, "crm")
	schedulerHandler := scheduler.NewHandler(schedulerSvc)
	notifHandler := notifications.NewHandler(notifSvc)

	// ── CRM ───────────────────────────────────────────────────────────────────
	pipelineHandler := crmpipeline.NewHandler(pipelineSvc)
	dealsHandler := crmdeals.NewHandler(dealsSvc)
	leadsHandler := crmleads.NewHandler(leadsSvc)
	crmRptHandler := crmreports.NewHandler(crmRptSvc)
	crmTemplatesHandler := crmtemplates.NewHandler(crmTemplatesSvc)
	crmSettingsHandler := crmsettings.NewHandler(crmSettingsSvc)

	// ── Capture ───────────────────────────────────────────────────────────────
	apikeysHandler := apikeys.NewHandler(apikeysSvc)
	publicHandler := public.NewHandler(leadsSvc)
	emailHandler := email.NewHandler(emailSvc)
	socialHandler := social.NewHandler(socialSvc)
	visitorsHandler := visitors.NewHandler(visitorsSvc)

	// ── HRM Phase 1 ───────────────────────────────────────────────────────────
	hrmDeptsHandler := hrmdepts.NewHandler(hrmDeptsSvc)
	hrmPosHandler := hrmpositions.NewHandler(hrmPosSvc)
	hrmEmpHandler := hrmemployees.NewHandler(hrmEmpSvc, authzSvc, hrmScopeResolver)
	hrmLeaveHandler := hrmleave.NewHandler(hrmLeaveSvc, authzSvc, hrmScopeResolver)
	hrmOnboardingHandler := hrmonboarding.NewHandler(hrmOnboardingSvc, authzSvc, hrmScopeResolver)
	hrmRptsHandler := hrmreports.NewHandler(hrmPhase1Rpts)

	// ── HRM Group A ───────────────────────────────────────────────────────────
	hrmSalaryHandler := hrmsalary.NewHandler(hrmSalarySvc, authzSvc, hrmScopeResolver)
	hrmApprovalsHandler := hrmapprovals.NewHandler(hrmApprovalsSvc)
	hrmWarnTypesHandler := hrmwarntypes.NewHandler(hrmWarnTypesSvc)
	hrmDocTmplsHandler := hrmdoctmpls.NewHandler(hrmDocTmplsSvc)
	hrmShiftsHandler := hrmshifts.NewHandler(hrmShiftsSvc)
	hrmHolidaysHandler := hrmholidays.NewHandler(hrmHolidaysSvc)
	hrmContractsHandler := hrmcontracts.NewHandler(hrmContractsSvc)

	// ── HRM Group B ───────────────────────────────────────────────────────────
	hrmPromotionsHandler := hrmpromotions.NewHandler(hrmPromotionsSvc, authzSvc, hrmScopeResolver)
	hrmTransfersHandler := hrmtransfers.NewHandler(hrmTransfersSvc, authzSvc, hrmScopeResolver)
	hrmResignationsHandler := hrmresignations.NewHandler(hrmResignationsSvc, authzSvc, hrmScopeResolver)
	hrmTerminationsHandler := hrmterminations.NewHandler(hrmTerminationsSvc, authzSvc, hrmScopeResolver)

	// ── HRM Group C ───────────────────────────────────────────────────────────
	hrmWarningsHandler := hrmwarnings.NewHandler(hrmWarningsSvc, authzSvc, hrmScopeResolver)
	hrmCplHandler := hrmcomplaints.NewHandler(hrmCplSvc, authzSvc, hrmScopeResolver)
	hrmEmpDocsHandler := hrmemployeedocs.NewHandler(hrmEmpDocsSvc, authzSvc, hrmScopeResolver)
	hrmAcksHandler := hrmacks.NewHandler(hrmAcksSvc)

	// ── HRM Group D ───────────────────────────────────────────────────────────
	hrmAttendanceHandler := hrmattendance.NewHandler(hrmAttendanceSvc, authzSvc, hrmScopeResolver)
	hrmPayslipsHandler := hrmpayslips.NewHandler(hrmPayslipsSvc, authzSvc, hrmScopeResolver)
	hrmCompensationHandler := hrmcompensation.NewHandler(hrmCompensationSvc, authzSvc, hrmScopeResolver)
	hrmLoansHandler := hrmloans.NewHandler(hrmLoansSvc, authzSvc, hrmScopeResolver)
	hrmReimbursementsHandler := hrmreimbursements.NewHandler(hrmReimbursementsSvc, authzSvc, hrmScopeResolver)
	hrmStatutoryHandler := hrmstatutory.NewHandler(hrmStatutorySvc)
	hrmBenefitsHandler := hrmbenefits.NewHandler(hrmBenefitsSvc, authzSvc, hrmScopeResolver)

	// ── HRM Group E ───────────────────────────────────────────────────────────
	hrmAwardsHandler := hrmawards.NewHandler(hrmAwardsSvc)
	hrmAnnsHandler := hrmannouncements.NewHandler(hrmAnnsSvc)
	hrmCalHandler := hrmcalendar.NewHandler(hrmCalSvc)
	hrmMilestonesHandler := hrmmilestones.NewHandler(hrmMilestonesSvc)

	// ── HRM Extended Phase 4A — Recruitment / ATS ──────────────────────────────
	hrmRecruitmentHandler := hrmrecruitment.NewHandler(hrmRecruitmentSvc)
	hrmPerformanceHandler := hrmperformance.NewHandler(hrmPerformanceSvc, authzSvc)
	hrmFeedbackHandler := hrmfeedback.NewHandler(hrmFeedbackSvc, authzSvc)
	hrmPipHandler := hrmpip.NewHandler(hrmPipSvc, authzSvc)
	hrmLearningHandler := hrmlearning.NewHandler(hrmLearningSvc, authzSvc)
	hrmSkillsHandler := hrmskills.NewHandler(hrmSkillsSvc, authzSvc)
	hrmCertificationsHandler := hrmcertifications.NewHandler(hrmCertificationsSvc, authzSvc)

	// ═════════════════════════════════════════════════════════════════════════
	// 9. FIBER
	// ═════════════════════════════════════════════════════════════════════════
	app := fiber.New(fiber.Config{
		AppName:      cfg.App.Name,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 30 * time.Second,
		ErrorHandler: func(c fiber.Ctx, err error) error {
			slog.Error("unhandled error",
				slog.Any("error", err),
				slog.String("path", c.Path()),
				slog.String("method", c.Method()),
			)
			return response.InternalServerError(c)
		},
		ServerHeader:  "",
		StrictRouting: true,
		CaseSensitive: true,
	})

	// ── 10. Global middleware ─────────────────────────────────────────────────
	app.Use(middleware.Recover())
	app.Use(middleware.RequestID())
	app.Use(middleware.Logger())
	app.Use(middleware.SecurityHeaders(cfg.App.IsProduction()))
	app.Use(cors.New(cors.Config{
		Next: func(c fiber.Ctx) bool {
			return strings.HasPrefix(c.Path(), "/api/v1/pub")
		},
		AllowOrigins:     cfg.CORS.AllowedOrigins,
		AllowMethods:     []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
		AllowHeaders:     []string{"Origin", "Content-Type", "Authorization", "X-Request-ID", "X-API-Key"},
		AllowCredentials: true,
		MaxAge:           86400,
	}))
	app.Get("/uploads/*", static.New("./uploads"))

	// ═════════════════════════════════════════════════════════════════════════
	// 11. ROUTES
	// Registration order matters in Fiber v3:
	//   • Static sub-routes must come before parameterised siblings.
	//   • All module routes are guarded by requireAuth + requireOrgMatch.
	//   • permFn wraps middleware.RequirePermission for a named permission key.
	// ═════════════════════════════════════════════════════════════════════════
	api := app.Group("/api/v1")
	registerSystemRoutes(api, app, pgPool, redisClient, cfg)

	// ── Core ──────────────────────────────────────────────────────────────────
	auth.RegisterRoutesWithRateLimit(api, authHandler, requireAuth, authRateLimit)
	user.RegisterRoutes(api, userHandler, requireAuth)
	organizations.RegisterRoutes(api, businessHandler, requireAuth)

	// Shared middleware factories used by every org-scoped module below
	requireBusiness := middleware.RequireBusiness()
	requireOrgParam := middleware.RequireOrganizationParam("orgId")
	requireOrgMatch := middleware.RequireOrganizationParam("orgId")

	dashboard.RegisterRoutes(api, dashboardHandler, requireAuth, requireOrgMatch)

	permFn := func(perm string) fiber.Handler {
		return middleware.RequirePermission(authzSvc, perm)
	}

	authz.RegisterRoutes(api, authzHandler, permFn, requireAuth, requireBusiness, requireOrgParam)
	security.RegisterRoutes(api, securityHandler, permFn, requireAuth, requireOrgParam)
	task.RegisterRoutes(api, taskHandler, permFn, requireAuth, requireOrgParam)

	// ── Platform (shared layer — CRM and future modules) ──────────────────────
	checklists.RegisterRoutes(api, checklistsHandler, permFn, requireAuth, requireOrgMatch)
	forms.RegisterRoutes(api, formsHandler, permFn, requireAuth, requireOrgMatch)
	contacts.RegisterRoutes(api, contactsHandler, permFn, requireAuth, requireOrgMatch)
	engagement.RegisterRoutes(api, engagementHandler, permFn, requireAuth, requireOrgMatch)
	scheduler.RegisterRoutes(api, schedulerHandler, requireAuth, permFn)
	notifications.RegisterRoutes(api, notifHandler, requireAuth)

	// ── CRM ───────────────────────────────────────────────────────────────────
	crmleads.RegisterRoutes(api, leadsHandler, permFn, requireAuth, requireOrgMatch)
	crmpipeline.RegisterRoutes(api, pipelineHandler, permFn, requireAuth, requireOrgMatch)
	crmdeals.RegisterRoutes(api, dealsHandler, permFn, requireAuth, requireOrgMatch)
	crmreports.RegisterRoutes(api, crmRptHandler, permFn, requireAuth, requireOrgMatch)
	crmtemplates.RegisterRoutes(api, crmTemplatesHandler, permFn, requireAuth, requireOrgMatch)
	crmsettings.RegisterRoutes(api, crmSettingsHandler, permFn, requireAuth, requireOrgMatch)

	// ── Capture ───────────────────────────────────────────────────────────────
	apikeys.RegisterRoutes(api, apikeysHandler, permFn, requireAuth, requireOrgMatch)
	public.RegisterRoutes(api, publicHandler, apikeysSvc)
	email.RegisterRoutes(api, emailHandler, permFn, requireAuth, requireOrgMatch)
	social.RegisterRoutes(api, socialHandler, permFn, requireAuth, requireOrgMatch)
	visitors.RegisterRoutes(api, visitorsHandler, apikeysSvc, permFn, requireAuth, requireOrgMatch)

	// ── HRM Phase 1 — Core Employee Management ────────────────────────────────
	// Routes under /organizations/:orgId/hrm/{departments,positions,employees,leave,reports}
	hrmdepts.RegisterRoutes(api, hrmDeptsHandler, permFn, requireAuth, requireOrgMatch)
	hrmpositions.RegisterRoutes(api, hrmPosHandler, permFn, requireAuth, requireOrgMatch)
	hrmemployees.RegisterRoutes(api, hrmEmpHandler, permFn, requireAuth, requireOrgMatch)
	hrmleave.RegisterRoutes(api, hrmLeaveHandler, permFn, requireAuth, requireOrgMatch)
	hrmonboarding.RegisterRoutes(api, hrmOnboardingHandler, permFn, requireAuth, requireOrgMatch)
	hrmreports.RegisterRoutes(api, hrmRptsHandler, permFn, requireAuth, requireOrgMatch)

	// ── HRM Group A — Config / Setup (migrations 00021–00028) ────────────────
	// Salary structures, approval templates, warning types, doc templates,
	// work shifts, holiday calendars, employee contracts
	hrmsalary.RegisterRoutes(api, hrmSalaryHandler, permFn, requireAuth, requireOrgMatch)
	hrmapprovals.RegisterRoutes(api, hrmApprovalsHandler, permFn, requireAuth, requireOrgMatch)
	hrmwarntypes.RegisterRoutes(api, hrmWarnTypesHandler, permFn, requireAuth, requireOrgMatch)
	hrmdoctmpls.RegisterRoutes(api, hrmDocTmplsHandler, permFn, requireAuth, requireOrgMatch)
	hrmshifts.RegisterRoutes(api, hrmShiftsHandler, permFn, requireAuth, requireOrgMatch)
	hrmholidays.RegisterRoutes(api, hrmHolidaysHandler, permFn, requireAuth, requireOrgMatch)
	hrmcontracts.RegisterRoutes(api, hrmContractsHandler, permFn, requireAuth, requireOrgMatch)

	// ── HRM Group B — Core Employee Lifecycle (migrations 00029–00033) ────────
	// Promotions, Transfers, Resignations, Terminations
	// Apply/Accept actions are transactional (pgxpool.BeginTx pattern).
	hrmpromotions.RegisterRoutes(api, hrmPromotionsHandler, permFn, requireAuth, requireOrgMatch)
	hrmtransfers.RegisterRoutes(api, hrmTransfersHandler, permFn, requireAuth, requireOrgMatch)
	hrmresignations.RegisterRoutes(api, hrmResignationsHandler, permFn, requireAuth, requireOrgMatch)
	hrmterminations.RegisterRoutes(api, hrmTerminationsHandler, permFn, requireAuth, requireOrgMatch)

	// ── HRM Group C — Disciplinary and Compliance (migrations 00034–00037) ────
	// Warnings (C1), Complaints (C2), Employee Documents (C3), Acknowledgements (C4)
	// C4 is cross-cutting — used by C1, C3, E2, E3.
	hrmwarnings.RegisterRoutes(api, hrmWarningsHandler, permFn, requireAuth, requireOrgMatch)
	hrmcomplaints.RegisterRoutes(api, hrmCplHandler, permFn, requireAuth, requireOrgMatch)
	hrmemployeedocs.RegisterRoutes(api, hrmEmpDocsHandler, permFn, requireAuth, requireOrgMatch)
	hrmacks.RegisterRoutes(api, hrmAcksHandler, permFn, requireAuth, requireOrgMatch)

	// ── HRM Group D — Time and Compensation (migrations 00038–00040) ──────────
	// Attendance (D1) → Payslips / Payroll engine (D2)
	// D2 ComputeRun() checks D1 period is finalized before running formulas.
	hrmattendance.RegisterRoutes(api, hrmAttendanceHandler, permFn, requireAuth, requireOrgMatch)
	hrmpayslips.RegisterRoutes(api, hrmPayslipsHandler, permFn, requireAuth, requireOrgMatch)
	hrmcompensation.RegisterRoutes(api, hrmCompensationHandler, permFn, requireAuth, requireOrgMatch)
	hrmloans.RegisterRoutes(api, hrmLoansHandler, permFn, requireAuth, requireOrgMatch)
	hrmreimbursements.RegisterRoutes(api, hrmReimbursementsHandler, permFn, requireAuth, requireOrgMatch)
	hrmstatutory.RegisterRoutes(api, hrmStatutoryHandler, permFn, requireAuth, requireOrgMatch)
	hrmbenefits.RegisterRoutes(api, hrmBenefitsHandler, permFn, requireAuth, requireOrgMatch)

	// ── HRM Group E — Recognition and Communication (migrations 00041–00045) ──
	// Awards (E1), Announcements (E2), HR Calendar (E3), Employee Milestones (E4)
	// E2 publish + E3 RSVP both write C4 acknowledgement rows (ON CONFLICT DO NOTHING).
	// E4 GenerateUpcoming reads A7 contract dates to auto-create milestone records.
	hrmawards.RegisterRoutes(api, hrmAwardsHandler, permFn, requireAuth, requireOrgMatch)
	hrmannouncements.RegisterRoutes(api, hrmAnnsHandler, permFn, requireAuth, requireOrgMatch)
	hrmcalendar.RegisterRoutes(api, hrmCalHandler, permFn, requireAuth, requireOrgMatch)
	hrmmilestones.RegisterRoutes(api, hrmMilestonesHandler, permFn, requireAuth, requireOrgMatch)

	// ── HRM Extended Phase 4A — Recruitment / ATS ──────────────────────────────
	// Internal-only: no /pub/careers/* route in this phase (blocked on Capture
	// Fix Pass B rate limiting + real email sending — see
	// docs/HrmExtendedBuildPlan.md PHASE 4 and Project_Instruction.md Section 5
	// → HRM MODULE → Recruitment / ATS).
	hrmrecruitment.RegisterRoutes(api, hrmRecruitmentHandler, permFn, requireAuth, requireOrgMatch)
	hrmperformance.RegisterRoutes(api, hrmPerformanceHandler, permFn, requireAuth, requireOrgMatch)
	hrmfeedback.RegisterRoutes(api, hrmFeedbackHandler, permFn, requireAuth, requireOrgMatch)
	hrmpip.RegisterRoutes(api, hrmPipHandler, permFn, requireAuth, requireOrgMatch)
	hrmlearning.RegisterRoutes(api, hrmLearningHandler, permFn, requireAuth, requireOrgMatch)
	hrmskills.RegisterRoutes(api, hrmSkillsHandler, permFn, requireAuth, requireOrgMatch)
	hrmcertifications.RegisterRoutes(api, hrmCertificationsHandler, permFn, requireAuth, requireOrgMatch)

	// ── 404 fallback — must be registered last ────────────────────────────────
	app.Use(func(c fiber.Ctx) error {
		return response.NotFound(c,
			"ROUTE_NOT_FOUND",
			fmt.Sprintf("route %s %s not found", c.Method(), c.Path()),
		)
	})

	slog.Info("all routes registered")

	// ═════════════════════════════════════════════════════════════════════════
	// 12. Start + graceful shutdown
	// ═════════════════════════════════════════════════════════════════════════
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt, syscall.SIGTERM)

	go func() {
		addr := ":" + cfg.App.Port
		slog.Info("server listening", slog.String("addr", addr))
		if listenErr := app.Listen(addr, fiber.ListenConfig{
			DisableStartupMessage: !cfg.App.IsDevelopment(),
		}); listenErr != nil {
			slog.Error("server error", slog.Any("error", listenErr))
			os.Exit(1)
		}
	}()

	// Start the background scheduler
	go schedulerSvc.Start(ctx)

	// Register scheduler jobs
	schedulerSvc.Register("milestones.generate_upcoming", "0 1 * * *", func(jCtx context.Context) (int, error) {
		orgIDs, err := businessSvc.FindAllIDs(jCtx)
		if err != nil {
			return 0, fmt.Errorf("failed to list organizations: %w", err)
		}
		now := time.Now()
		req := hrmmilestones.GenerateRequest{
			Year:                    now.Year(),
			Month:                   int(now.Month()),
			IncludeAnniversaries:    true,
			IncludeBirthdays:        true,
			IncludeProbation:        true,
			IncludeContractRenewals: true,
		}
		var total int
		for _, orgID := range orgIDs {
			res, err := hrmMilestonesSvc.GenerateUpcoming(jCtx, orgID, scheduler.SystemUserID, req)
			if err != nil {
				slog.Error("scheduler: milestones generation failed for org", "orgID", orgID, "error", err)
				continue
			}
			if res != nil {
				total += res.Generated
			}
		}
		return total, nil
	})

	schedulerSvc.Register("attendance.absence_sweep", "0 2 * * *", func(jCtx context.Context) (int, error) {
		orgIDs, err := businessSvc.FindAllIDs(jCtx)
		if err != nil {
			return 0, fmt.Errorf("failed to list organizations: %w", err)
		}
		sweepDate := time.Now().AddDate(0, 0, -1).Format("2006-01-02") // yesterday
		var total int
		for _, orgID := range orgIDs {
			n, err := hrmAttendanceSvc.RunAbsenceSweep(jCtx, orgID, scheduler.SystemUserID, sweepDate)
			if err != nil {
				slog.Error("scheduler: absence sweep failed for org", "orgID", orgID, "error", err)
				continue
			}
			total += n
		}
		return total, nil
	})

	// Daily (not monthly-only): on_joining grants need to fire promptly for
	// new hires; monthly/annual policies simply no-op on days they're not
	// due. Also rolls the period into a hrm_leave_balances snapshot on the
	// 1st of each month.
	schedulerSvc.Register("leave.accrue_and_snapshot", "0 3 * * *", func(jCtx context.Context) (int, error) {
		orgIDs, err := businessSvc.FindAllIDs(jCtx)
		if err != nil {
			return 0, fmt.Errorf("failed to list organizations: %w", err)
		}
		today := time.Now().Format("2006-01-02")
		var total int
		for _, orgID := range orgIDs {
			n, err := hrmLeaveSvc.RunAccrual(jCtx, orgID, scheduler.SystemUserID, today)
			if err != nil {
				slog.Error("scheduler: leave accrual failed for org", "orgID", orgID, "error", err)
				continue
			}
			total += n
		}
		return total, nil
	})

	// Annual (Jan 1, after the 03:00 accrual run): forfeits any balance in
	// excess of a policy's carry_forward_cap. Its Dec-31-dated forfeiture is
	// picked up by the next accrual run's snapshot window — ordering is
	// enforced only by this cron gap, not an explicit call chain.
	// The build plan calls this the highest-value feature in Phase 6. It runs
	// instance-wide rather than per-org — the sweep queries an indexed slice of
	// hrm_employee_certifications directly, so there is no org loop to write.
	//
	// 04:00, after the leave jobs, so a night's writes are spread out rather
	// than contending on the same connection pool.
	schedulerSvc.Register("certifications.expiry_sweep", "0 4 * * *", func(jCtx context.Context) (int, error) {
		res, err := hrmCertificationsSvc.SweepExpiries(jCtx)
		if err != nil {
			// The partial count is still reported: the expiring pass may have
			// committed before the expired pass failed.
			return res.Total(), fmt.Errorf("certification expiry sweep: %w", err)
		}
		return res.Total(), nil
	})

	schedulerSvc.Register("leave.year_end_carry_forward", "30 2 1 1 *", func(jCtx context.Context) (int, error) {
		orgIDs, err := businessSvc.FindAllIDs(jCtx)
		if err != nil {
			return 0, fmt.Errorf("failed to list organizations: %w", err)
		}
		today := time.Now().Format("2006-01-02")
		var total int
		for _, orgID := range orgIDs {
			n, err := hrmLeaveSvc.RunCarryForward(jCtx, orgID, scheduler.SystemUserID, today)
			if err != nil {
				slog.Error("scheduler: leave carry-forward failed for org", "orgID", orgID, "error", err)
				continue
			}
			total += n
		}
		return total, nil
	})

	// benefits.activate_pending_enrollments — instance-wide, the
	// attendance.absence_sweep / certifications.expiry_sweep shape (r24):
	// flips every 'pending' enrollment whose effective_date has arrived to
	// 'active', so a benefit signed up for ahead of its start date actually
	// begins deducting once that date arrives, without a human having to
	// notice and flip it manually.
	schedulerSvc.Register("benefits.activate_pending_enrollments", "0 5 * * *", func(jCtx context.Context) (int, error) {
		n, err := hrmBenefitsSvc.ActivatePendingEnrollments(jCtx)
		if err != nil {
			return n, fmt.Errorf("benefits enrollment activation sweep: %w", err)
		}
		return n, nil
	})

	<-quit
	slog.Info("shutdown signal received")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := app.ShutdownWithContext(shutdownCtx); err != nil {
		slog.Error("graceful shutdown failed", slog.Any("error", err))
		os.Exit(1)
	}

	slog.Info("server stopped cleanly")
}

func registerSystemRoutes(
	router fiber.Router,
	app *fiber.App,
	pgPool *pgxpool.Pool,
	redisClient *redis.Client,
	cfg *config.Config,
) {
	// Health check
	router.Get("/health", func(c fiber.Ctx) error {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()

		pgStatus := "ok"
		if err := database.Ping(ctx, pgPool); err != nil {
			slog.Error("health: postgres ping failed", slog.Any("error", err))
			pgStatus = "unreachable"
		}

		redisStatus := "ok"
		if err := database.PingRedis(ctx, redisClient); err != nil {
			slog.Error("health: redis ping failed", slog.Any("error", err))
			redisStatus = "unreachable"
		}

		healthy := pgStatus == "ok" && redisStatus == "ok"
		httpStatus := fiber.StatusOK
		if !healthy {
			httpStatus = fiber.StatusServiceUnavailable
		}

		return c.Status(httpStatus).JSON(fiber.Map{
			"success": healthy,
			"data": fiber.Map{
				"status":   healthStatus(healthy),
				"postgres": pgStatus,
				"redis":    redisStatus,
			},
			"message":    healthMessage(healthy),
			"request_id": c.Locals("request_id"),
		})
	})

	router.Get("/hello", func(c fiber.Ctx) error {
		return response.OK(c, nil, "Hello from Go backend")
	})

	// ── API Documentation (development only) ──────────────────────────────────
	if cfg.App.IsDevelopment() {
		router.Get("/docs/openapi.json", func(c fiber.Ctx) error {
			c.Set("Content-Type", "application/json; charset=utf-8")
			c.Set("Cache-Control", "public, max-age=3600")
			return c.Send(bsaasdocs.SwaggerJSON)
		})

		router.Get("/docs", func(c fiber.Ctx) error {
			c.Set("Content-Type", "text/html; charset=utf-8")
			return c.SendString(scalarHTML)
		})

		router.Get("/docs/", func(c fiber.Ctx) error {
			return c.Redirect().To("/api/v1/docs")
		})

		slog.Info("API docs available",
			slog.String("ui", "http://localhost:"+cfg.App.Port+"/api/v1/docs"),
			slog.String("spec", "http://localhost:"+cfg.App.Port+"/api/v1/docs/openapi.json"),
		)
	}

	// Route explorer — dev only
	router.Get("/routes", func(c fiber.Ctx) error {
		if cfg.App.IsProduction() {
			return response.NotFound(c, "ROUTE_NOT_FOUND", "not found")
		}

		type routeInfo struct {
			Method string `json:"method"`
			Path   string `json:"path"`
		}

		grouped := make(map[string][]routeInfo)
		for _, r := range app.GetRoutes(true) {
			if r.Method == fiber.MethodHead {
				continue
			}
			if r.Path == "/*" || r.Path == "" {
				continue
			}
			group := routeGroup(r.Path)
			grouped[group] = append(grouped[group], routeInfo{Method: r.Method, Path: r.Path})
		}

		type groupEntry struct {
			Group  string      `json:"group"`
			Count  int         `json:"count"`
			Routes []routeInfo `json:"routes"`
		}

		groupKeys := make([]string, 0, len(grouped))
		for k := range grouped {
			groupKeys = append(groupKeys, k)
		}
		sort.Strings(groupKeys)

		total := 0
		result := make([]groupEntry, 0, len(groupKeys))
		for _, k := range groupKeys {
			routes := grouped[k]
			sort.Slice(routes, func(i, j int) bool {
				if routes[i].Path != routes[j].Path {
					return routes[i].Path < routes[j].Path
				}
				return routes[i].Method < routes[j].Method
			})
			total += len(routes)
			result = append(result, groupEntry{Group: k, Count: len(routes), Routes: routes})
		}

		return response.OK(c, fiber.Map{"total": total, "groups": result}, "Route table")
	})
}

func routeGroup(path string) string {
	parts := strings.Split(strings.TrimPrefix(path, "/"), "/")
	if len(parts) >= 3 {
		seg := parts[2]
		switch seg {
		case "health", "hello", "routes", "docs":
			return "system"
		default:
			return seg
		}
	}
	return "other"
}

func setupLogger(isDev bool) {
	sensitiveKeys := map[string]bool{
		"password": true, "token": true, "secret": true,
		"refresh_token": true, "access_token": true,
		"reset_token": true, "api_key": true,
	}
	replaceAttr := func(_ []string, a slog.Attr) slog.Attr {
		if sensitiveKeys[a.Key] {
			return slog.String(a.Key, "[REDACTED]")
		}
		return a
	}
	if isDev {
		slog.SetDefault(slog.New(tint.NewHandler(os.Stdout, &tint.Options{
			Level: slog.LevelDebug, TimeFormat: time.TimeOnly, ReplaceAttr: replaceAttr,
		})))
		return
	}
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo, ReplaceAttr: replaceAttr,
	})))
}

func healthStatus(ok bool) string {
	if ok {
		return "ok"
	}
	return "degraded"
}

func healthMessage(ok bool) string {
	if ok {
		return "Service healthy"
	}
	return "Service degraded"
}
