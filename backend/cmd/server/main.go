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
	"github.com/mridha/businesssaas/internal/database"
	"github.com/mridha/businesssaas/internal/middleware"
	"github.com/mridha/businesssaas/internal/organizations"
	"github.com/mridha/businesssaas/internal/security"
	"github.com/mridha/businesssaas/internal/task"
	"github.com/mridha/businesssaas/internal/user"
	jwtpkg "github.com/mridha/businesssaas/pkg/jwt"
	"github.com/mridha/businesssaas/pkg/response"

	// ── Internal — Platform (shared across modules) ───────────────────────────
	"github.com/mridha/businesssaas/internal/platform/contacts"
	"github.com/mridha/businesssaas/internal/platform/engagement"

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
	hrmpositions "github.com/mridha/businesssaas/internal/hrm/positions"
	hrmreports "github.com/mridha/businesssaas/internal/hrm/reports"

	// ── Internal — HRM Group A (Config / Setup) ───────────────────────────────
	hrmapprovals "github.com/mridha/businesssaas/internal/hrm/approvals"
	hrmcontracts "github.com/mridha/businesssaas/internal/hrm/contracts"
	hrmdoctmpls "github.com/mridha/businesssaas/internal/hrm/doctemplates"
	hrmholidays "github.com/mridha/businesssaas/internal/hrm/holidays"
	hrmsalary "github.com/mridha/businesssaas/internal/hrm/salary"
	hrmshifts "github.com/mridha/businesssaas/internal/hrm/shifts"
	hrmwarntypes "github.com/mridha/businesssaas/internal/hrm/warningtypes"

	// ── Internal — HRM Group B (Core Employee Lifecycle) ─────────────────────
	hrmpromotions "github.com/mridha/businesssaas/internal/hrm/promotions"
	hrmresignations "github.com/mridha/businesssaas/internal/hrm/resignations"
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
	auditRepo := audit.NewRepository(pgPool)
	securityRepo := security.NewRepository(pgPool)
	taskRepo := task.NewRepository(pgPool)

	// ── Platform ──────────────────────────────────────────────────────────────
	contactsRepo := contacts.NewRepository(pgPool)
	engagementRepo := engagement.NewRepository(pgPool)

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

	// ── HRM Group E — Recognition and Communication (migrations 00041–00045) ──
	hrmAwardsRepo := hrmawards.NewRepository(pgPool)
	hrmAnnsRepo := hrmannouncements.NewRepository(pgPool)
	hrmCalRepo := hrmcalendar.NewRepository(pgPool)
	hrmMilestonesRepo := hrmmilestones.NewRepository(pgPool)

	// ═════════════════════════════════════════════════════════════════════════
	// 7. SERVICES   (dependency order: leaf → composite)
	// ═════════════════════════════════════════════════════════════════════════

	// ── Core ──────────────────────────────────────────────────────────────────
	auditSvc := audit.NewService(auditRepo)
	userSvc := user.NewService(userRepo)
	avatarSvc := user.NewAvatarService(avatarRepo)
	authSvc := auth.NewService(authRepo, userRepo, jwtManager, cfg.JWT, auditSvc)
	authzSvc := authz.NewService(authzRepo, redisClient, auditSvc, authRepo)
	businessSvc := organizations.NewService(businessRepo, authzRepo, jwtManager)
	securitySvc := security.NewService(securityRepo)
	taskSvc := task.NewService(taskRepo, auditSvc)

	// ── Platform ──────────────────────────────────────────────────────────────
	contactsSvc := contacts.NewService(contactsRepo)
	engagementSvc := engagement.NewService(engagementRepo)

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
	hrmEmpSvc := hrmemployees.NewService(hrmEmpRepo, auditSvc)
	hrmLeaveSvc := hrmleave.NewService(hrmLeaveRepo, auditSvc)
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

	// ── HRM Group D — time and compensation ───────────────────────────────────
	// D1 attendance: pgPool for shift resolution queries
	// D2 payslips:   pgPool for formula engine (employee+salary+attendance queries)
	hrmAttendanceSvc := hrmattendance.NewService(hrmAttendanceRepo, pgPool)
	hrmPayslipsSvc := hrmpayslips.NewService(hrmPayslipsRepo, pgPool)

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

	// Wire approval-instance completion back into each of the five workflow
	// modules. Must run after all five services above exist. entityType here
	// must match the EntityType string each Submit()/Issue() uses when calling
	// approvalsSvc.CreateInstance — see each module's service.go.
	hrmApprovalsSvc.RegisterCallback("promotion", hrmPromotionsSvc.HandleApprovalDecision)
	hrmApprovalsSvc.RegisterCallback("transfer", hrmTransfersSvc.HandleApprovalDecision)
	hrmApprovalsSvc.RegisterCallback("termination", hrmTerminationsSvc.HandleApprovalDecision)
	hrmApprovalsSvc.RegisterCallback("warning", hrmWarningsSvc.HandleApprovalDecision)
	hrmApprovalsSvc.RegisterCallback("award", hrmAwardsSvc.HandleApprovalDecision)

	// ═════════════════════════════════════════════════════════════════════════
	// 8. HANDLERS
	// ═════════════════════════════════════════════════════════════════════════

	// ── Core ──────────────────────────────────────────────────────────────────
	authHandler := auth.NewHandler(authSvc, cfg.Cookie)
	userHandler := user.NewHandler(userSvc, avatarSvc)
	authzHandler := authz.NewHandler(authzSvc)
	businessHandler := organizations.NewHandler(businessSvc)
	securityHandler := security.NewHandler(securitySvc)
	taskHandler := task.NewHandler(taskSvc)

	// ── Platform ──────────────────────────────────────────────────────────────
	contactsHandler := contacts.NewHandler(contactsSvc)
	// When HRM arrives use: engagement.NewHandler(engagementSvc, "hrm")
	engagementHandler := engagement.NewHandler(engagementSvc, "crm")

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
	hrmEmpHandler := hrmemployees.NewHandler(hrmEmpSvc)
	hrmLeaveHandler := hrmleave.NewHandler(hrmLeaveSvc)
	hrmRptsHandler := hrmreports.NewHandler(hrmPhase1Rpts)

	// ── HRM Group A ───────────────────────────────────────────────────────────
	hrmSalaryHandler := hrmsalary.NewHandler(hrmSalarySvc)
	hrmApprovalsHandler := hrmapprovals.NewHandler(hrmApprovalsSvc)
	hrmWarnTypesHandler := hrmwarntypes.NewHandler(hrmWarnTypesSvc)
	hrmDocTmplsHandler := hrmdoctmpls.NewHandler(hrmDocTmplsSvc)
	hrmShiftsHandler := hrmshifts.NewHandler(hrmShiftsSvc)
	hrmHolidaysHandler := hrmholidays.NewHandler(hrmHolidaysSvc)
	hrmContractsHandler := hrmcontracts.NewHandler(hrmContractsSvc)

	// ── HRM Group B ───────────────────────────────────────────────────────────
	hrmPromotionsHandler := hrmpromotions.NewHandler(hrmPromotionsSvc)
	hrmTransfersHandler := hrmtransfers.NewHandler(hrmTransfersSvc)
	hrmResignationsHandler := hrmresignations.NewHandler(hrmResignationsSvc)
	hrmTerminationsHandler := hrmterminations.NewHandler(hrmTerminationsSvc)

	// ── HRM Group C ───────────────────────────────────────────────────────────
	hrmWarningsHandler := hrmwarnings.NewHandler(hrmWarningsSvc)
	hrmCplHandler := hrmcomplaints.NewHandler(hrmCplSvc)
	hrmEmpDocsHandler := hrmemployeedocs.NewHandler(hrmEmpDocsSvc)
	hrmAcksHandler := hrmacks.NewHandler(hrmAcksSvc)

	// ── HRM Group D ───────────────────────────────────────────────────────────
	hrmAttendanceHandler := hrmattendance.NewHandler(hrmAttendanceSvc)
	hrmPayslipsHandler := hrmpayslips.NewHandler(hrmPayslipsSvc)

	// ── HRM Group E ───────────────────────────────────────────────────────────
	hrmAwardsHandler := hrmawards.NewHandler(hrmAwardsSvc)
	hrmAnnsHandler := hrmannouncements.NewHandler(hrmAnnsSvc)
	hrmCalHandler := hrmcalendar.NewHandler(hrmCalSvc)
	hrmMilestonesHandler := hrmmilestones.NewHandler(hrmMilestonesSvc)

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
		AllowMethods:     []string{"GET", "POST", "PATCH", "DELETE", "OPTIONS"},
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

	permFn := func(perm string) fiber.Handler {
		return middleware.RequirePermission(authzSvc, perm)
	}

	authz.RegisterRoutes(api, authzHandler, permFn, requireAuth, requireBusiness, requireOrgParam)
	security.RegisterRoutes(api, securityHandler, permFn, requireAuth, requireOrgParam)
	task.RegisterRoutes(api, taskHandler, permFn, requireAuth, requireOrgParam)

	// ── Platform (shared layer — CRM and future modules) ──────────────────────
	contacts.RegisterRoutes(api, contactsHandler, permFn, requireAuth, requireOrgMatch)
	engagement.RegisterRoutes(api, engagementHandler, permFn, requireAuth, requireOrgMatch)

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

	// ── HRM Group E — Recognition and Communication (migrations 00041–00045) ──
	// Awards (E1), Announcements (E2), HR Calendar (E3), Employee Milestones (E4)
	// E2 publish + E3 RSVP both write C4 acknowledgement rows (ON CONFLICT DO NOTHING).
	// E4 GenerateUpcoming reads A7 contract dates to auto-create milestone records.
	hrmawards.RegisterRoutes(api, hrmAwardsHandler, permFn, requireAuth, requireOrgMatch)
	hrmannouncements.RegisterRoutes(api, hrmAnnsHandler, permFn, requireAuth, requireOrgMatch)
	hrmcalendar.RegisterRoutes(api, hrmCalHandler, permFn, requireAuth, requireOrgMatch)
	hrmmilestones.RegisterRoutes(api, hrmMilestonesHandler, permFn, requireAuth, requireOrgMatch)

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
