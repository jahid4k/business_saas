// backend/internal/hrm/employees/handler.go
package employees

import (
	"errors"
	"log/slog"
	"strconv"
	"strings"

	"github.com/gofiber/fiber/v3"

	"github.com/mridha/businesssaas/internal/authz"
	"github.com/mridha/businesssaas/internal/hrm/scope"
	"github.com/mridha/businesssaas/internal/middleware"
	"github.com/mridha/businesssaas/pkg/logger"
	"github.com/mridha/businesssaas/pkg/response"
)

// Handler handles HRM employee HTTP endpoints.
type Handler struct {
	service       Service
	authz         authz.Service
	scopeResolver *scope.Resolver
}

func NewHandler(service Service, authzSvc authz.Service, scopeResolver *scope.Resolver) *Handler {
	return &Handler{service: service, authz: authzSvc, scopeResolver: scopeResolver}
}

// List handles GET /api/v1/organizations/:orgId/hrm/employees
// Requires: hrm.employees.view
// Query: status, employment_type, department_id, manager_id, search, limit, offset
func (h *Handler) List(c fiber.Ctx) error {
	log := logger.FromCtx(c)
	userID, ok := middleware.UserIDFromCtx(c)
	if !ok {
		return response.Unauthorized(c, "UNAUTHORIZED", "Authentication required")
	}
	orgID, ok := middleware.OrganizationIDFromCtx(c)
	if !ok {
		return response.BadRequest(c, "NO_ORGANIZATION_CONTEXT", "Organization context is required")
	}

	scopeTier, err := h.authz.ResolveScope(c.Context(), userID, orgID, "hrm.employees")
	if err != nil {
		log.Error("employees: List: resolve scope", slog.Any("error", err))
		return response.InternalServerError(c)
	}

	filter := ListFilter{Scope: scopeTier, CallerUserID: userID}

	if st := strings.TrimSpace(c.Query("status_id")); st != "" {
		filter.StatusID = st
	}
	if et := strings.TrimSpace(c.Query("employment_type")); et != "" {
		t := EmploymentType(et)
		if !t.IsValid() {
			return response.BadRequest(c, "INVALID_EMPLOYMENT_TYPE",
				"employment_type must be one of: full_time, part_time, contractor, intern")
		}
		filter.EmploymentType = t
	}
	filter.DepartmentID = strings.TrimSpace(c.Query("department_id"))
	filter.ManagerID = strings.TrimSpace(c.Query("manager_id"))
	filter.Search = strings.TrimSpace(c.Query("search"))

	if limit, err := strconv.Atoi(c.Query("limit", "")); err == nil {
		filter.Limit = limit
	}
	if offset, err := strconv.Atoi(c.Query("offset", "")); err == nil {
		filter.Offset = offset
	}

	result, err := h.service.List(c.Context(), orgID, filter)
	if err != nil {
		log.Error("employees: List error", slog.Any("error", err))
		return response.InternalServerError(c)
	}
	return response.OK(c, result, "OK")
}

// Create handles POST /api/v1/organizations/:orgId/hrm/employees
// Requires: hrm.employees.create
func (h *Handler) Create(c fiber.Ctx) error {
	userID, ok := middleware.UserIDFromCtx(c)
	if !ok {
		return response.Unauthorized(c, "UNAUTHORIZED", "Authentication required")
	}
	orgID, ok := middleware.OrganizationIDFromCtx(c)
	if !ok {
		return response.BadRequest(c, "NO_ORGANIZATION_CONTEXT", "Organization context is required")
	}

	var req CreateEmployeeRequest
	if err := c.Bind().JSON(&req); err != nil {
		return response.BadRequest(c, "INVALID_BODY", "Invalid request body")
	}

	e, err := h.service.Create(c.Context(), orgID, userID, req)
	if err != nil {
		return h.empError(c, err)
	}
	return response.Created(c, fiber.Map{"employee": e}, "Employee created")
}

// Get handles GET /api/v1/organizations/:orgId/hrm/employees/:empId
// Requires: hrm.employees.view
func (h *Handler) Get(c fiber.Ctx) error {
	log := logger.FromCtx(c)
	userID, ok := middleware.UserIDFromCtx(c)
	if !ok {
		return response.Unauthorized(c, "UNAUTHORIZED", "Authentication required")
	}
	orgID, ok := middleware.OrganizationIDFromCtx(c)
	if !ok {
		return response.BadRequest(c, "NO_ORGANIZATION_CONTEXT", "Organization context is required")
	}
	empID := c.Params("empId")
	scopeTier, err := h.authz.ResolveScope(c.Context(), userID, orgID, "hrm.employees")
	if err != nil {
		log.Error("employees: Get", slog.Any("error", err))
		return response.InternalServerError(c)
	}
	allowed, err := h.scopeResolver.AuthorizeRecordAccess(c.Context(), scopeTier, orgID, userID, empID)
	if err != nil {
		log.Error("employees: Get", slog.Any("error", err))
		return response.InternalServerError(c)
	}
	if !allowed {
		return response.Forbidden(c, "RECORD_ACCESS_DENIED", "You do not have access to this record")
	}
	e, err := h.service.Get(c.Context(), orgID, empID)
	if err != nil {
		return h.empError(c, err)
	}
	return response.OK(c, fiber.Map{"employee": e}, "OK")
}

// Update handles PATCH /api/v1/organizations/:orgId/hrm/employees/:empId
// Requires: hrm.employees.update
func (h *Handler) Update(c fiber.Ctx) error {
	orgID, ok := middleware.OrganizationIDFromCtx(c)
	if !ok {
		return response.BadRequest(c, "NO_ORGANIZATION_CONTEXT", "Organization context is required")
	}

	var req UpdateEmployeeRequest
	if err := c.Bind().JSON(&req); err != nil {
		return response.BadRequest(c, "INVALID_BODY", "Invalid request body")
	}

	e, err := h.service.Update(c.Context(), orgID, c.Params("empId"), req)
	if err != nil {
		return h.empError(c, err)
	}
	return response.OK(c, fiber.Map{"employee": e}, "Employee updated")
}

// Terminate handles POST /api/v1/organizations/:orgId/hrm/employees/:empId/terminate
// Requires: hrm.employees.terminate
func (h *Handler) Terminate(c fiber.Ctx) error {
	userID, ok := middleware.UserIDFromCtx(c)
	if !ok {
		return response.Unauthorized(c, "UNAUTHORIZED", "Authentication required")
	}
	orgID, ok := middleware.OrganizationIDFromCtx(c)
	if !ok {
		return response.BadRequest(c, "NO_ORGANIZATION_CONTEXT", "Organization context is required")
	}

	var req TerminateEmployeeRequest
	if err := c.Bind().JSON(&req); err != nil {
		return response.BadRequest(c, "INVALID_BODY", "Invalid request body")
	}

	e, err := h.service.Terminate(c.Context(), orgID, c.Params("empId"), userID, req)
	if err != nil {
		return h.empError(c, err)
	}
	return response.OK(c, fiber.Map{"employee": e}, "Employee terminated")
}

// Delete handles DELETE /api/v1/organizations/:orgId/hrm/employees/:empId
// Requires: hrm.employees.delete
func (h *Handler) Delete(c fiber.Ctx) error {
	orgID, ok := middleware.OrganizationIDFromCtx(c)
	if !ok {
		return response.BadRequest(c, "NO_ORGANIZATION_CONTEXT", "Organization context is required")
	}
	if err := h.service.Delete(c.Context(), orgID, c.Params("empId")); err != nil {
		return h.empError(c, err)
	}
	return response.NoContent(c)
}

// ListStatuses handles GET /api/v1/organizations/:orgId/hrm/employee-statuses
// Requires: hrm.employees.view
func (h *Handler) ListStatuses(c fiber.Ctx) error {
	log := logger.FromCtx(c)
	orgID, ok := middleware.OrganizationIDFromCtx(c)
	if !ok {
		return response.BadRequest(c, "NO_ORGANIZATION_CONTEXT", "Organization context is required")
	}

	list, err := h.service.ListStatuses(c.Context(), orgID)
	if err != nil {
		log.Error("employees: ListStatuses error", slog.Any("error", err))
		return response.InternalServerError(c)
	}
	return response.OK(c, fiber.Map{"statuses": list}, "OK")
}

// CreateStatus handles POST /api/v1/organizations/:orgId/hrm/employee-statuses
func (h *Handler) CreateStatus(c fiber.Ctx) error {
	orgID, ok := middleware.OrganizationIDFromCtx(c)
	if !ok {
		return response.BadRequest(c, "NO_ORGANIZATION_CONTEXT", "Organization context is required")
	}
	var req CreateEmployeeStatusRequest
	if err := c.Bind().JSON(&req); err != nil {
		return response.BadRequest(c, "INVALID_BODY", "Invalid request body")
	}

	st, err := h.service.CreateStatus(c.Context(), orgID, req)
	if err != nil {
		return h.empError(c, err)
	}
	return response.Created(c, fiber.Map{"status": st}, "Status created")
}

// UpdateStatus handles PATCH /api/v1/organizations/:orgId/hrm/employee-statuses/:id
func (h *Handler) UpdateStatus(c fiber.Ctx) error {
	orgID, ok := middleware.OrganizationIDFromCtx(c)
	if !ok {
		return response.BadRequest(c, "NO_ORGANIZATION_CONTEXT", "Organization context is required")
	}
	var req UpdateEmployeeStatusRequest
	if err := c.Bind().JSON(&req); err != nil {
		return response.BadRequest(c, "INVALID_BODY", "Invalid request body")
	}

	st, err := h.service.UpdateStatus(c.Context(), orgID, c.Params("id"), req)
	if err != nil {
		return h.empError(c, err)
	}
	return response.OK(c, fiber.Map{"status": st}, "Status updated")
}

// DeleteStatus handles DELETE /api/v1/organizations/:orgId/hrm/employee-statuses/:id
func (h *Handler) DeleteStatus(c fiber.Ctx) error {
	orgID, ok := middleware.OrganizationIDFromCtx(c)
	if !ok {
		return response.BadRequest(c, "NO_ORGANIZATION_CONTEXT", "Organization context is required")
	}
	if err := h.service.DeleteStatus(c.Context(), orgID, c.Params("id")); err != nil {
		return h.empError(c, err)
	}
	return response.NoContent(c)
}

func (h *Handler) empError(c fiber.Ctx, err error) error {
	log := logger.FromCtx(c)
	switch {
	case errors.Is(err, ErrEmployeeNotFound):
		return response.NotFound(c, "EMPLOYEE_NOT_FOUND", "Employee not found")
	case errors.Is(err, ErrFirstNameRequired):
		return response.BadRequest(c, "FIRST_NAME_REQUIRED", "first_name is required")
	case errors.Is(err, ErrFirstNameTooLong):
		return response.BadRequest(c, "FIRST_NAME_TOO_LONG", "first_name must not exceed 100 characters")
	case errors.Is(err, ErrHireDateRequired):
		return response.BadRequest(c, "HIRE_DATE_REQUIRED", "hire_date is required")
	case errors.Is(err, ErrInvalidHireDate):
		return response.BadRequest(c, "INVALID_HIRE_DATE", "hire_date must be a valid date in YYYY-MM-DD format")
	case errors.Is(err, ErrInvalidDateOfBirth):
		return response.BadRequest(c, "INVALID_DATE_OF_BIRTH", "date_of_birth must be a valid date in YYYY-MM-DD format")
	case errors.Is(err, ErrInvalidTerminationDate):
		return response.BadRequest(c, "INVALID_TERMINATION_DATE", "termination_date must be a valid date in YYYY-MM-DD format")
	case errors.Is(err, ErrTerminationBeforeHire):
		return response.BadRequest(c, "TERMINATION_BEFORE_HIRE", "termination_date cannot be before hire_date")
	case errors.Is(err, ErrInvalidEmploymentType):
		return response.BadRequest(c, "INVALID_EMPLOYMENT_TYPE", "employment_type must be one of: full_time, part_time, contractor, intern")

	case errors.Is(err, ErrInvalidGender):
		return response.BadRequest(c, "INVALID_GENDER", "gender must be one of: male, female, other, prefer_not_to_say")
	case errors.Is(err, ErrAlreadyTerminated):
		return response.Conflict(c, "ALREADY_TERMINATED", "Employee is already terminated")
	case errors.Is(err, ErrEmployeeNumberConflict):
		return response.Conflict(c, "EMPLOYEE_NUMBER_CONFLICT", "An employee with this employee_number already exists")
	case errors.Is(err, ErrSelfManager):
		return response.BadRequest(c, "SELF_MANAGER", "An employee cannot be their own manager")
	case errors.Is(err, ErrInvalidStatusCategory):
		return response.BadRequest(c, "INVALID_CATEGORY", "Invalid status category")
	case errors.Is(err, ErrStatusNameRequired):
		return response.BadRequest(c, "NAME_REQUIRED", "Status name is required")
	case errors.Is(err, ErrStatusColorRequired):
		return response.BadRequest(c, "COLOR_REQUIRED", "Status color is required")
	case errors.Is(err, ErrCannotModifyDefaultStatus):
		return response.Forbidden(c, "FORBIDDEN", "Cannot modify or delete default system statuses")
	case err != nil && strings.Contains(err.Error(), "status not found"):
		return response.NotFound(c, "STATUS_NOT_FOUND", "Status not found")
	default:
		log.Error("employees error", slog.Any("error", err))
		return response.InternalServerError(c)
	}
}
