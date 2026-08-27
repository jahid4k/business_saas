// backend/internal/hrm/payslips/handler.go
package payslips

import (
	"errors"
	"log/slog"
	"strconv"

	"github.com/gofiber/fiber/v3"

	"github.com/mridha/businesssaas/internal/authz"
	"github.com/mridha/businesssaas/internal/hrm/scope"
	"github.com/mridha/businesssaas/internal/middleware"
	"github.com/mridha/businesssaas/pkg/logger"
	"github.com/mridha/businesssaas/pkg/response"
)

type Handler struct {
	service       Service
	authz         authz.Service
	scopeResolver *scope.Resolver
}

func NewHandler(service Service, authzSvc authz.Service, scopeResolver *scope.Resolver) *Handler {
	return &Handler{service: service, authz: authzSvc, scopeResolver: scopeResolver}
}

// ListRuns godoc
//
//	@Summary		List payslip runs
//	@Description	Returns all payroll runs for the organization, sorted by period (newest first).
//	@Description
//	@Description	**Required permission:** `hrm.payroll.view`
//	@Tags			HRM / Payroll
//	@Produce		json
//	@Security		BearerAuth
//	@Param			orgId	path		string	true	"Organization ID"
//	@Success		200		{object}	response.OK{data=RunListResponse}
//	@Router			/organizations/{orgId}/hrm/payroll/runs [get]
func (h *Handler) ListRuns(c fiber.Ctx) error {
	log := logger.FromCtx(c)
	orgID, ok := middleware.OrganizationIDFromCtx(c)
	if !ok {
		return response.BadRequest(c, "NO_ORGANIZATION_CONTEXT", "Organization context is required")
	}
	res, err := h.service.ListRuns(c.Context(), orgID)
	if err != nil {
		log.Error("payslips: ListRuns", slog.Any("error", err))
		return response.InternalServerError(c)
	}
	return response.OK(c, res, "OK")
}

// CreateRun godoc
//
//	@Summary		Create payslip run
//	@Description	Creates a payroll run in draft status for a given year/month.
//	@Description	One run per org per month (enforced).
//	@Description
//	@Description	Optionally link to a finalized attendance period (attendance_period_id).
//	@Description	The compute step enforces this is finalized before running.
//	@Description
//	@Description	**Required permission:** `hrm.payroll.manage`
//	@Tags			HRM / Payroll
//	@Accept			json
//	@Produce		json
//	@Security		BearerAuth
//	@Param			orgId	path		string				true	"Organization ID"
//	@Param			body	body		CreateRunRequest	true	"Run details"
//	@Success		201		{object}	response.Created{data=object{run=PayslipRun}}
//	@Failure		409		{object}	response.Error	"DUPLICATE_RUN"
//	@Router			/organizations/{orgId}/hrm/payroll/runs [post]
func (h *Handler) CreateRun(c fiber.Ctx) error {
	userID, ok := middleware.UserIDFromCtx(c)
	if !ok {
		return response.Unauthorized(c, "UNAUTHORIZED", "Authentication required")
	}
	orgID, ok := middleware.OrganizationIDFromCtx(c)
	if !ok {
		return response.BadRequest(c, "NO_ORGANIZATION_CONTEXT", "Organization context is required")
	}
	var req CreateRunRequest
	if err := c.Bind().JSON(&req); err != nil {
		return response.BadRequest(c, "INVALID_BODY", "Invalid request body")
	}
	run, err := h.service.CreateRun(c.Context(), orgID, userID, req)
	if err != nil {
		return h.err(c, err)
	}
	return response.Created(c, fiber.Map{"run": run}, "Payslip run created")
}

// GetRun godoc
//
//	@Summary		Get payslip run
//	@Tags			HRM / Payroll
//	@Produce		json
//	@Security		BearerAuth
//	@Param			orgId	path		string	true	"Organization ID"
//	@Param			runId	path		string	true	"Run public ID (pr_*)"
//	@Success		200		{object}	response.OK{data=object{run=PayslipRun}}
//	@Router			/organizations/{orgId}/hrm/payroll/runs/{runId} [get]
func (h *Handler) GetRun(c fiber.Ctx) error {
	orgID, ok := middleware.OrganizationIDFromCtx(c)
	if !ok {
		return response.BadRequest(c, "NO_ORGANIZATION_CONTEXT", "Organization context is required")
	}
	run, err := h.service.GetRun(c.Context(), orgID, c.Params("runId"))
	if err != nil {
		return h.err(c, err)
	}
	return response.OK(c, fiber.Map{"run": run}, "OK")
}

// ComputeRun godoc
//
//	@Summary		Compute payroll run
//	@Description	Runs the A1 salary formula engine for all active employees.
//	@Description
//	@Description	**Process:**
//	@Description	1. Checks attendance period is finalized (if linked)
//	@Description	2. Loads each employee's salary structure (A1)
//	@Description	3. Evaluates each component: fixed / pct_of_basic / pct_of_gross / formula
//	@Description	4. Formula context env: `BASIC`, `GROSS`, `PRESENT_DAYS`, `WORK_DAYS`, `TENURE_YEARS`
//	@Description	5. Creates payslip + payslip_lines rows per employee
//	@Description
//	@Description	**Required permission:** `hrm.payroll.compute`
//	@Tags			HRM / Payroll
//	@Produce		json
//	@Security		BearerAuth
//	@Param			orgId	path	string	true	"Organization ID"
//	@Param			runId	path	string	true	"Run public ID"
//	@Success		200		{object}	response.OK{data=object{run=PayslipRun}}
//	@Failure		409		{object}	response.Error	"ALREADY_COMPUTED or ATTENDANCE_NOT_FINALIZED"
//	@Router			/organizations/{orgId}/hrm/payroll/runs/{runId}/compute [post]
func (h *Handler) ComputeRun(c fiber.Ctx) error {
	userID, ok := middleware.UserIDFromCtx(c)
	if !ok {
		return response.Unauthorized(c, "UNAUTHORIZED", "Authentication required")
	}
	orgID, ok := middleware.OrganizationIDFromCtx(c)
	if !ok {
		return response.BadRequest(c, "NO_ORGANIZATION_CONTEXT", "Organization context is required")
	}
	run, err := h.service.ComputeRun(c.Context(), orgID, c.Params("runId"), userID)
	if err != nil {
		return h.err(c, err)
	}
	return response.OK(c, fiber.Map{"run": run}, "Payroll computed")
}

// PreviewRun godoc
//
//	@Summary		Dry-run a payroll computation
//	@Description	Computes exactly what ComputeRun would and persists NOTHING — no payslips, no lines, no status change.
//	@Description	Surfaces negative_net_count, which is the condition that blocks approval.
//	@Description	**Required permission:** `hrm.payroll.preview`
//	@Tags			HRM / Payroll
//	@Produce		json
//	@Security		BearerAuth
//	@Param			orgId	path		string	true	"Organization ID"
//	@Param			runId	path		string	true	"Run public ID"
//	@Success		200		{object}	response.OK{data=object{preview=RunPreview}}
//	@Router			/organizations/{orgId}/hrm/payroll/runs/{runId}/preview [post]
func (h *Handler) PreviewRun(c fiber.Ctx) error {
	orgID, ok := middleware.OrganizationIDFromCtx(c)
	if !ok {
		return response.BadRequest(c, "NO_ORGANIZATION_CONTEXT", "Organization context is required")
	}
	preview, err := h.service.PreviewRun(c.Context(), orgID, c.Params("runId"))
	if err != nil {
		return h.err(c, err)
	}
	return response.OK(c, fiber.Map{"preview": preview}, "Payroll preview computed")
}

// ApproveRun godoc
//
//	@Summary		Approve payroll run
//	@Description	Approves a computed run. Required before marking as paid.
//	@Description	**Required permission:** `hrm.payroll.approve`
//	@Tags			HRM / Payroll
//	@Produce		json
//	@Security		BearerAuth
//	@Param			orgId	path	string	true	"Organization ID"
//	@Param			runId	path	string	true	"Run public ID"
//	@Success		200		{object}	response.OK{data=object{run=PayslipRun}}
//	@Failure		409		{object}	response.Error	"NOT_COMPUTED"
//	@Router			/organizations/{orgId}/hrm/payroll/runs/{runId}/approve [post]
func (h *Handler) ApproveRun(c fiber.Ctx) error {
	userID, ok := middleware.UserIDFromCtx(c)
	if !ok {
		return response.Unauthorized(c, "UNAUTHORIZED", "Authentication required")
	}
	orgID, ok := middleware.OrganizationIDFromCtx(c)
	if !ok {
		return response.BadRequest(c, "NO_ORGANIZATION_CONTEXT", "Organization context is required")
	}
	run, err := h.service.ApproveRun(c.Context(), orgID, c.Params("runId"), userID)
	if err != nil {
		return h.err(c, err)
	}
	return response.OK(c, fiber.Map{"run": run}, "Payroll run approved")
}

// MarkPaid godoc
//
//	@Summary		Mark payroll as paid
//	@Description	Records that payroll disbursement has been completed. Locks the run.
//	@Description	**Required permission:** `hrm.payroll.pay`
//	@Tags			HRM / Payroll
//	@Produce		json
//	@Security		BearerAuth
//	@Param			orgId	path	string	true	"Organization ID"
//	@Param			runId	path	string	true	"Run public ID"
//	@Success		200		{object}	response.OK{data=object{run=PayslipRun}}
//	@Failure		409		{object}	response.Error	"NOT_APPROVED"
//	@Router			/organizations/{orgId}/hrm/payroll/runs/{runId}/pay [post]
func (h *Handler) MarkPaid(c fiber.Ctx) error {
	userID, ok := middleware.UserIDFromCtx(c)
	if !ok {
		return response.Unauthorized(c, "UNAUTHORIZED", "Authentication required")
	}
	orgID, ok := middleware.OrganizationIDFromCtx(c)
	if !ok {
		return response.BadRequest(c, "NO_ORGANIZATION_CONTEXT", "Organization context is required")
	}
	run, err := h.service.MarkPaid(c.Context(), orgID, c.Params("runId"), userID)
	if err != nil {
		return h.err(c, err)
	}
	return response.OK(c, fiber.Map{"run": run}, "Payroll marked as paid")
}

// CancelRun godoc
//
//	@Summary		Cancel payroll run
//	@Tags			HRM / Payroll
//	@Produce		json
//	@Security		BearerAuth
//	@Param			orgId	path	string	true	"Organization ID"
//	@Param			runId	path	string	true	"Run public ID"
//	@Success		200		{object}	response.OK{data=object{run=PayslipRun}}
//	@Router			/organizations/{orgId}/hrm/payroll/runs/{runId}/cancel [post]
func (h *Handler) CancelRun(c fiber.Ctx) error {
	orgID, ok := middleware.OrganizationIDFromCtx(c)
	if !ok {
		return response.BadRequest(c, "NO_ORGANIZATION_CONTEXT", "Organization context is required")
	}
	run, err := h.service.CancelRun(c.Context(), orgID, c.Params("runId"))
	if err != nil {
		return h.err(c, err)
	}
	return response.OK(c, fiber.Map{"run": run}, "Payroll run cancelled")
}

// ListPayslips godoc
//
//	@Summary		List payslips
//	@Description	Returns payslips. Filter by run_id or employee_id.
//	@Description
//	@Description	**Required permission:** `hrm.payroll.view`
//	@Tags			HRM / Payroll
//	@Produce		json
//	@Security		BearerAuth
//	@Param			orgId		path		string	true	"Organization ID"
//	@Param			run_id		query		string	false	"Filter by run public ID"
//	@Param			employee_id	query		string	false	"Filter by employee UUID"
//	@Success		200			{object}	response.OK{data=SlipListResponse}
//	@Router			/organizations/{orgId}/hrm/payroll/payslips [get]
func (h *Handler) ListPayslips(c fiber.Ctx) error {
	log := logger.FromCtx(c)
	userID, ok := middleware.UserIDFromCtx(c)
	if !ok {
		return response.Unauthorized(c, "UNAUTHORIZED", "Authentication required")
	}
	orgID, ok := middleware.OrganizationIDFromCtx(c)
	if !ok {
		return response.BadRequest(c, "NO_ORGANIZATION_CONTEXT", "Organization context is required")
	}
	scopeTier, err := h.authz.ResolveScope(c.Context(), userID, orgID, "hrm.payroll")
	if err != nil {
		log.Error("payslips: ListPayslips", slog.Any("error", err))
		return response.InternalServerError(c)
	}
	filter := SlipListFilter{
		RunID:        c.Query("run_id"),
		EmployeeID:   c.Query("employee_id"),
		Scope:        scopeTier,
		CallerUserID: userID,
	}
	if limit, err := strconv.Atoi(c.Query("limit", "")); err == nil {
		filter.Limit = limit
	}
	if offset, err := strconv.Atoi(c.Query("offset", "")); err == nil {
		filter.Offset = offset
	}
	res, err := h.service.ListPayslips(c.Context(), orgID, filter)
	if err != nil {
		log.Error("payslips: ListPayslips", slog.Any("error", err))
		return response.InternalServerError(c)
	}
	return response.OK(c, res, "OK")
}

// GetPayslip godoc
//
//	@Summary		Get payslip with component lines
//	@Description	Returns a single payslip including all salary component lines.
//	@Description
//	@Description	**Required permission:** `hrm.payroll.view`
//	@Tags			HRM / Payroll
//	@Produce		json
//	@Security		BearerAuth
//	@Param			orgId		path		string	true	"Organization ID"
//	@Param			payslipId	path		string	true	"Payslip public ID (ps_*)"
//	@Success		200			{object}	response.OK{data=object{payslip=Payslip}}
//	@Router			/organizations/{orgId}/hrm/payroll/payslips/{payslipId} [get]
func (h *Handler) GetPayslip(c fiber.Ctx) error {
	log := logger.FromCtx(c)
	userID, ok := middleware.UserIDFromCtx(c)
	if !ok {
		return response.Unauthorized(c, "UNAUTHORIZED", "Authentication required")
	}
	orgID, ok := middleware.OrganizationIDFromCtx(c)
	if !ok {
		return response.BadRequest(c, "NO_ORGANIZATION_CONTEXT", "Organization context is required")
	}
	p, err := h.service.GetPayslip(c.Context(), orgID, c.Params("payslipId"))
	if err != nil {
		return h.err(c, err)
	}
	scopeTier, err := h.authz.ResolveScope(c.Context(), userID, orgID, "hrm.payroll")
	if err != nil {
		log.Error("payslips: GetPayslip", slog.Any("error", err))
		return response.InternalServerError(c)
	}
	allowed, err := h.scopeResolver.AuthorizeRecordAccess(c.Context(), scopeTier, orgID, userID, p.EmployeeID)
	if err != nil {
		log.Error("payslips: GetPayslip", slog.Any("error", err))
		return response.InternalServerError(c)
	}
	if !allowed {
		return response.Forbidden(c, "RECORD_ACCESS_DENIED", "You do not have access to this record")
	}
	return response.OK(c, fiber.Map{"payslip": p}, "OK")
}

func (h *Handler) err(c fiber.Ctx, err error) error {
	log := logger.FromCtx(c)
	switch {
	case errors.Is(err, ErrNotFound):
		return response.NotFound(c, "RUN_NOT_FOUND", "Payslip run not found")
	case errors.Is(err, ErrPayslipNotFound):
		return response.NotFound(c, "PAYSLIP_NOT_FOUND", "Payslip not found")
	case errors.Is(err, ErrYearRequired):
		return response.BadRequest(c, "YEAR_REQUIRED", "year is required")
	case errors.Is(err, ErrMonthRequired):
		return response.BadRequest(c, "MONTH_REQUIRED", "month is required (1-12)")
	case errors.Is(err, ErrInvalidMonth):
		return response.BadRequest(c, "INVALID_MONTH", "month must be between 1 and 12")
	case errors.Is(err, ErrDuplicateRun):
		return response.Conflict(c, "DUPLICATE_RUN", "A payslip run already exists for this period")
	case errors.Is(err, ErrAttendanceNotFinalized):
		return response.Conflict(c, "ATTENDANCE_NOT_FINALIZED", "The linked attendance period must be finalized before computing payroll")
	case errors.Is(err, ErrWrongStatus):
		return response.Conflict(c, "WRONG_STATUS", "Action not allowed in current payroll run status")
	case errors.Is(err, ErrAlreadyComputed):
		return response.Conflict(c, "ALREADY_COMPUTED", "Payslip run has already been computed")
	case errors.Is(err, ErrNotComputed):
		return response.Conflict(c, "NOT_COMPUTED", "Payslip run must be computed before approving")
	case errors.Is(err, ErrNegativeNetPay):
		return response.Conflict(c, "NEGATIVE_NET_PAY", err.Error())
	case errors.Is(err, ErrClearancePending):
		return response.Conflict(c, "CLEARANCE_PENDING",
			"Clearance is incomplete: resolve all blocking items before approving the settlement")
	case errors.Is(err, ErrNoExitForFnFRun):
		return response.BadRequest(c, "NO_EXIT_FOR_FNF_RUN",
			"This full & final run has no exit record attached")
	case errors.Is(err, ErrFnFEmployeeNotFound):
		return response.NotFound(c, "FNF_EMPLOYEE_NOT_FOUND",
			"The employee this settlement names was not found")
	case errors.Is(err, ErrInvalidRunType):
		return response.BadRequest(c, "INVALID_RUN_TYPE", err.Error())
	case errors.Is(err, ErrNotApproved):
		return response.Conflict(c, "NOT_APPROVED", "Payslip run must be approved before marking as paid")
	default:
		log.Error("payslips: error", slog.Any("error", err))
		return response.InternalServerError(c)
	}
}
