// backend/internal/hrm/attendance/handler.go
package attendance

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

// ListRecords godoc
//
//	@Summary		List attendance records
//	@Description	Returns attendance records. Filter by employee, status, year/month.
//	@Description
//	@Description	**Required permission:** `hrm.attendance.view`
//	@Tags			HRM / Attendance
//	@Produce		json
//	@Security		BearerAuth
//	@Param			orgId		path		string	true	"Organization ID"
//	@Param			employee_id	query		string	false	"Filter by employee UUID"
//	@Param			status		query		string	false	"pending|approved|rejected"
//	@Param			year		query		int		false	"Year (e.g. 2026)"
//	@Param			month		query		int		false	"Month 1–12"
//	@Success		200			{object}	response.OK{data=RecordListResponse}
//	@Router			/organizations/{orgId}/hrm/attendance [get]
func (h *Handler) ListRecords(c fiber.Ctx) error {
	log := logger.FromCtx(c)
	userID, ok := middleware.UserIDFromCtx(c)
	if !ok { return response.Unauthorized(c, "UNAUTHORIZED", "Authentication required") }
	orgID, ok := middleware.OrganizationIDFromCtx(c)
	if !ok { return response.BadRequest(c, "NO_ORGANIZATION_CONTEXT", "Organization context is required") }
	scopeTier, err := h.authz.ResolveScope(c.Context(), userID, orgID, "hrm.attendance")
	if err != nil { log.Error("attendance: ListRecords", slog.Any("error", err)); return response.InternalServerError(c) }
	year, _ := strconv.Atoi(c.Query("year"))
	month, _ := strconv.Atoi(c.Query("month"))
	filter := RecordListFilter{
		EmployeeID:   c.Query("employee_id"),
		Status:       c.Query("status"),
		Year:         year,
		Month:        month,
		Scope:        scopeTier,
		CallerUserID: userID,
	}
	if limit, err := strconv.Atoi(c.Query("limit", "")); err == nil { filter.Limit = limit }
	if offset, err := strconv.Atoi(c.Query("offset", "")); err == nil { filter.Offset = offset }
	res, err := h.service.ListRecords(c.Context(), orgID, filter)
	if err != nil { log.Error("attendance: ListRecords", slog.Any("error", err)); return response.InternalServerError(c) }
	return response.OK(c, res, "OK")
}

// Record godoc
//
//	@Summary		Record attendance
//	@Description	Creates a single attendance record for an employee.
//	@Description	Shift is resolved automatically (employee > dept > org > default).
//	@Description	Regular and overtime hours are computed from check_in/check_out times.
//	@Description
//	@Description	**Required permission:** `hrm.attendance.manage`
//	@Tags			HRM / Attendance
//	@Accept			json
//	@Produce		json
//	@Security		BearerAuth
//	@Param			orgId	path		string				true	"Organization ID"
//	@Param			body	body		CreateRecordRequest	true	"Attendance details"
//	@Success		201		{object}	response.Created{data=object{record=AttendanceRecord}}
//	@Failure		409		{object}	response.Error	"DUPLICATE_RECORD or PERIOD_FINALIZED"
//	@Router			/organizations/{orgId}/hrm/attendance [post]
func (h *Handler) Record(c fiber.Ctx) error {
	userID, ok := middleware.UserIDFromCtx(c)
	if !ok { return response.Unauthorized(c, "UNAUTHORIZED", "Authentication required") }
	orgID, ok := middleware.OrganizationIDFromCtx(c)
	if !ok { return response.BadRequest(c, "NO_ORGANIZATION_CONTEXT", "Organization context is required") }
	var req CreateRecordRequest
	if err := c.Bind().JSON(&req); err != nil { return response.BadRequest(c, "INVALID_BODY", "Invalid request body") }
	rec, err := h.service.Record(c.Context(), orgID, userID, req)
	if err != nil { return h.err(c, err) }
	return response.Created(c, fiber.Map{"record": rec}, "Attendance recorded")
}

func (h *Handler) GetRecord(c fiber.Ctx) error {
	log := logger.FromCtx(c)
	userID, ok := middleware.UserIDFromCtx(c)
	if !ok { return response.Unauthorized(c, "UNAUTHORIZED", "Authentication required") }
	orgID, ok := middleware.OrganizationIDFromCtx(c)
	if !ok { return response.BadRequest(c, "NO_ORGANIZATION_CONTEXT", "Organization context is required") }
	rec, err := h.service.GetRecord(c.Context(), orgID, c.Params("recordId"))
	if err != nil { return h.err(c, err) }
	scopeTier, err := h.authz.ResolveScope(c.Context(), userID, orgID, "hrm.attendance")
	if err != nil { log.Error("attendance: GetRecord", slog.Any("error", err)); return response.InternalServerError(c) }
	allowed, err := h.scopeResolver.AuthorizeRecordAccess(c.Context(), scopeTier, orgID, userID, rec.EmployeeID)
	if err != nil { log.Error("attendance: GetRecord", slog.Any("error", err)); return response.InternalServerError(c) }
	if !allowed { return response.Forbidden(c, "RECORD_ACCESS_DENIED", "You do not have access to this record") }
	return response.OK(c, fiber.Map{"record": rec}, "OK")
}

func (h *Handler) Approve(c fiber.Ctx) error {
	userID, ok := middleware.UserIDFromCtx(c)
	if !ok { return response.Unauthorized(c, "UNAUTHORIZED", "Authentication required") }
	orgID, ok := middleware.OrganizationIDFromCtx(c)
	if !ok { return response.BadRequest(c, "NO_ORGANIZATION_CONTEXT", "Organization context is required") }
	rec, err := h.service.Approve(c.Context(), orgID, c.Params("recordId"), userID)
	if err != nil { return h.err(c, err) }
	return response.OK(c, fiber.Map{"record": rec}, "Regularization approved")
}

func (h *Handler) Reject(c fiber.Ctx) error {
	orgID, ok := middleware.OrganizationIDFromCtx(c)
	if !ok { return response.BadRequest(c, "NO_ORGANIZATION_CONTEXT", "Organization context is required") }
	rec, err := h.service.Reject(c.Context(), orgID, c.Params("recordId"))
	if err != nil { return h.err(c, err) }
	return response.OK(c, fiber.Map{"record": rec}, "Regularization rejected")
}

// Regularize godoc
//
//	@Summary		Request attendance correction (regularization)
//	@Description	Employee requests correction of an approved record. Sets status=pending for HR review.
//	@Description
//	@Description	**Required permission:** `hrm.attendance.manage`
//	@Tags			HRM / Attendance
//	@Accept			json
//	@Produce		json
//	@Security		BearerAuth
//	@Param			orgId		path		string				true	"Organization ID"
//	@Param			recordId	path		string				true	"Record public ID (att_*)"
//	@Param			body		body		RegularizeRequest	true	"Correction details and reason"
//	@Success		200			{object}	response.OK{data=object{record=AttendanceRecord}}
//	@Failure		409			{object}	response.Error	"PERIOD_FINALIZED"
//	@Router			/organizations/{orgId}/hrm/attendance/{recordId}/regularize [post]
func (h *Handler) Regularize(c fiber.Ctx) error {
	userID, ok := middleware.UserIDFromCtx(c)
	if !ok { return response.Unauthorized(c, "UNAUTHORIZED", "Authentication required") }
	orgID, ok := middleware.OrganizationIDFromCtx(c)
	if !ok { return response.BadRequest(c, "NO_ORGANIZATION_CONTEXT", "Organization context is required") }
	var req RegularizeRequest
	if err := c.Bind().JSON(&req); err != nil { return response.BadRequest(c, "INVALID_BODY", "Invalid request body") }
	rec, err := h.service.Regularize(c.Context(), orgID, c.Params("recordId"), userID, req)
	if err != nil { return h.err(c, err) }
	return response.OK(c, fiber.Map{"record": rec}, "Regularization request submitted")
}

func (h *Handler) ListPeriods(c fiber.Ctx) error {
	log := logger.FromCtx(c)
	orgID, ok := middleware.OrganizationIDFromCtx(c)
	if !ok { return response.BadRequest(c, "NO_ORGANIZATION_CONTEXT", "Organization context is required") }
	year, _ := strconv.Atoi(c.Query("year"))
	month, _ := strconv.Atoi(c.Query("month"))
	res, err := h.service.ListPeriods(c.Context(), orgID, year, month)
	if err != nil { log.Error("attendance: ListPeriods", slog.Any("error", err)); return response.InternalServerError(c) }
	return response.OK(c, res, "OK")
}

// GetOrCreatePeriod godoc
//
//	@Summary		Get or create attendance period
//	@Description	Returns the attendance period for a given year/month. Creates it if not yet opened.
//	@Description
//	@Description	**Required permission:** `hrm.attendance.manage`
//	@Tags			HRM / Attendance
//	@Accept			json
//	@Produce		json
//	@Security		BearerAuth
//	@Param			orgId	path		string			true	"Organization ID"
//	@Param			body	body		PeriodRequest	true	"Year and month"
//	@Success		200		{object}	response.OK{data=object{period=AttendancePeriod}}
//	@Router			/organizations/{orgId}/hrm/attendance/periods [post]
func (h *Handler) GetOrCreatePeriod(c fiber.Ctx) error {
	userID, ok := middleware.UserIDFromCtx(c)
	if !ok { return response.Unauthorized(c, "UNAUTHORIZED", "Authentication required") }
	orgID, ok := middleware.OrganizationIDFromCtx(c)
	if !ok { return response.BadRequest(c, "NO_ORGANIZATION_CONTEXT", "Organization context is required") }
	var req PeriodRequest
	if err := c.Bind().JSON(&req); err != nil { return response.BadRequest(c, "INVALID_BODY", "Invalid request body") }
	p, err := h.service.GetOrCreatePeriod(c.Context(), orgID, userID, req.Year, req.Month)
	if err != nil { return h.err(c, err) }
	return response.OK(c, fiber.Map{"period": p}, "OK")
}

// FinalizePeriod godoc
//
//	@Summary		Finalize attendance period
//	@Description	Locks the period for payroll. Aggregates stats. D2 payslip engine requires this.
//	@Description
//	@Description	**Required permission:** `hrm.attendance.finalize`
//	@Tags			HRM / Attendance
//	@Produce		json
//	@Security		BearerAuth
//	@Param			orgId	path	string	true	"Organization ID"
//	@Param			year	path	int		true	"Year"
//	@Param			month	path	int		true	"Month"
//	@Success		200		{object}	response.OK{data=object{period=AttendancePeriod}}
//	@Failure		409		{object}	response.Error	"PERIOD_ALREADY_FINALIZED"
//	@Router			/organizations/{orgId}/hrm/attendance/periods/{year}/{month}/finalize [post]
func (h *Handler) FinalizePeriod(c fiber.Ctx) error {
	userID, ok := middleware.UserIDFromCtx(c)
	if !ok { return response.Unauthorized(c, "UNAUTHORIZED", "Authentication required") }
	orgID, ok := middleware.OrganizationIDFromCtx(c)
	if !ok { return response.BadRequest(c, "NO_ORGANIZATION_CONTEXT", "Organization context is required") }
	year, _ := strconv.Atoi(c.Params("year"))
	month, _ := strconv.Atoi(c.Params("month"))
	p, err := h.service.FinalizePeriod(c.Context(), orgID, userID, year, month)
	if err != nil { return h.err(c, err) }
	return response.OK(c, fiber.Map{"period": p}, "Attendance period finalized")
}

func (h *Handler) LockPeriod(c fiber.Ctx) error {
	userID, ok := middleware.UserIDFromCtx(c)
	if !ok { return response.Unauthorized(c, "UNAUTHORIZED", "Authentication required") }
	orgID, ok := middleware.OrganizationIDFromCtx(c)
	if !ok { return response.BadRequest(c, "NO_ORGANIZATION_CONTEXT", "Organization context is required") }
	year, _ := strconv.Atoi(c.Params("year"))
	month, _ := strconv.Atoi(c.Params("month"))
	p, err := h.service.LockPeriod(c.Context(), orgID, userID, year, month)
	if err != nil { return h.err(c, err) }
	return response.OK(c, fiber.Map{"period": p}, "Attendance period locked")
}

func (h *Handler) err(c fiber.Ctx, err error) error {
	log := logger.FromCtx(c)
	switch {
	case errors.Is(err, ErrNotFound): return response.NotFound(c, "RECORD_NOT_FOUND", "Attendance record not found")
	case errors.Is(err, ErrPeriodNotFound): return response.NotFound(c, "PERIOD_NOT_FOUND", "Attendance period not found")
	case errors.Is(err, ErrEmployeeIDRequired): return response.BadRequest(c, "EMPLOYEE_ID_REQUIRED", "employee_id is required")
	case errors.Is(err, ErrDateRequired): return response.BadRequest(c, "DATE_REQUIRED", "date is required (YYYY-MM-DD)")
	case errors.Is(err, ErrInvalidDate): return response.BadRequest(c, "INVALID_DATE", "date must be a valid YYYY-MM-DD")
	case errors.Is(err, ErrInvalidDayType): return response.BadRequest(c, "INVALID_DAY_TYPE", "invalid day_type value")
	case errors.Is(err, ErrDuplicateRecord): return response.Conflict(c, "DUPLICATE_RECORD", "Attendance record already exists for this employee on this date")
	case errors.Is(err, ErrPeriodFinalized): return response.Conflict(c, "PERIOD_FINALIZED", "Attendance period is finalized — no edits allowed")
	case errors.Is(err, ErrPeriodAlreadyFinalizedOrLocked): return response.Conflict(c, "PERIOD_ALREADY_FINALIZED", "Attendance period is already finalized or locked")
	case errors.Is(err, ErrPeriodNotOpen): return response.Conflict(c, "PERIOD_NOT_OPEN", "Attendance period must be open to finalize")
	case errors.Is(err, ErrWrongStatus): return response.Conflict(c, "WRONG_STATUS", "Action not allowed in current status")
	case errors.Is(err, ErrReasonRequired): return response.BadRequest(c, "REASON_REQUIRED", "Regularization reason is required")
	default: log.Error("attendance: error", slog.Any("error", err)); return response.InternalServerError(c)
	}
}
