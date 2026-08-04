// backend/internal/hrm/salary/handler.go
package salary

import (
	"errors"
	"log/slog"
	"strings"

	"github.com/gofiber/fiber/v3"

	"github.com/mridha/businesssaas/internal/authz"
	"github.com/mridha/businesssaas/internal/hrm/scope"
	"github.com/mridha/businesssaas/internal/middleware"
	"github.com/mridha/businesssaas/pkg/logger"
	"github.com/mridha/businesssaas/pkg/response"
)

// Handler handles all HRM salary HTTP endpoints.
type Handler struct {
	service       Service
	authz         authz.Service
	scopeResolver *scope.Resolver
}

func NewHandler(service Service, authzSvc authz.Service, scopeResolver *scope.Resolver) *Handler {
	return &Handler{service: service, authz: authzSvc, scopeResolver: scopeResolver}
}

// ─────────────────────────────────────────────────────────────
// Salary Components
// ─────────────────────────────────────────────────────────────

// ListComponents godoc
//
//	@Summary		List salary components
//	@Description	Returns all salary component definitions for the organization.
//	@Description	Components are the building blocks used inside salary structures.
//	@Description
//	@Description	**Required permission:** `hrm.salary.view`
//	@Tags			HRM / Salary Setup
//	@Produce		json
//	@Security		BearerAuth
//	@Param			orgId	path		string	true	"Organization ID"
//	@Param			active	query		bool	false	"When true, return only active components"
//	@Success		200		{object}	response.OK{data=ComponentListResponse}
//	@Failure		401		{object}	response.Error
//	@Failure		403		{object}	response.Error
//	@Failure		500		{object}	response.Error
//	@Router			/organizations/{orgId}/hrm/setup/salary/components [get]
func (h *Handler) ListComponents(c fiber.Ctx) error {
	log := logger.FromCtx(c)
	orgID, ok := middleware.OrganizationIDFromCtx(c)
	if !ok {
		return response.BadRequest(c, "NO_ORGANIZATION_CONTEXT", "Organization context is required")
	}
	activeOnly := strings.ToLower(c.Query("active")) == "true"
	result, err := h.service.ListComponents(c.Context(), orgID, activeOnly)
	if err != nil {
		log.Error("salary: ListComponents", slog.Any("error", err))
		return response.InternalServerError(c)
	}
	return response.OK(c, result, "OK")
}

// CreateComponent godoc
//
//	@Summary		Create salary component
//	@Description	Creates a new salary component (earning, deduction, or employer contribution).
//	@Description
//	@Description	**calc_method values:**
//	@Description	- `fixed` — use fixed_value directly
//	@Description	- `pct_of_basic` — fixed_value as percentage of BASIC (e.g. 40.0 = 40%)
//	@Description	- `pct_of_gross` — fixed_value as percentage of GROSS earnings so far
//	@Description	- `formula` — expr-lang expression; formula_expression required
//	@Description	- `manual` — entered per payroll run; no formula
//	@Description	- `slab` — progressive brackets; slab_config required
//	@Description
//	@Description	**Formula variables available:** BASIC, GROSS, WORKING_DAYS, ACTUAL_DAYS,
//	@Description	ABSENT_DAYS, OVERTIME_HOURS, LATE_MINUTES, TENURE_YEARS, TENURE_MONTHS, PERIOD_DAYS
//	@Description
//	@Description	**Required permission:** `hrm.salary.manage`
//	@Description
//	@Description	**Error codes:**
//	@Description	- `NAME_REQUIRED` · `NAME_TOO_LONG` · `NAME_CONFLICT`
//	@Description	- `INVALID_COMPONENT_TYPE` · `INVALID_CALC_METHOD`
//	@Description	- `FORMULA_REQUIRED` · `INVALID_FORMULA`
//	@Description	- `SLAB_REQUIRED` · `INVALID_SLAB`
//	@Tags			HRM / Salary Setup
//	@Accept			json
//	@Produce		json
//	@Security		BearerAuth
//	@Param			orgId	path		string					true	"Organization ID"
//	@Param			body	body		CreateComponentRequest	true	"Component definition"
//	@Success		201		{object}	response.Created{data=object{component=SalaryComponent}}
//	@Failure		400		{object}	response.Error
//	@Failure		401		{object}	response.Error
//	@Failure		403		{object}	response.Error
//	@Failure		409		{object}	response.Error	"NAME_CONFLICT"
//	@Failure		500		{object}	response.Error
//	@Router			/organizations/{orgId}/hrm/setup/salary/components [post]
func (h *Handler) CreateComponent(c fiber.Ctx) error {
	userID, ok := middleware.UserIDFromCtx(c)
	if !ok {
		return response.Unauthorized(c, "UNAUTHORIZED", "Authentication required")
	}
	orgID, ok := middleware.OrganizationIDFromCtx(c)
	if !ok {
		return response.BadRequest(c, "NO_ORGANIZATION_CONTEXT", "Organization context is required")
	}
	var req CreateComponentRequest
	if err := c.Bind().JSON(&req); err != nil {
		return response.BadRequest(c, "INVALID_BODY", "Invalid request body")
	}
	comp, err := h.service.CreateComponent(c.Context(), orgID, userID, req)
	if err != nil {
		return h.compError(c, err)
	}
	return response.Created(c, fiber.Map{"component": comp}, "Salary component created")
}

// GetComponent godoc
//
//	@Summary		Get salary component
//	@Description	Returns a single salary component by its public ID.
//	@Description
//	@Description	**Required permission:** `hrm.salary.view`
//	@Tags			HRM / Salary Setup
//	@Produce		json
//	@Security		BearerAuth
//	@Param			orgId	path		string	true	"Organization ID"
//	@Param			compId	path		string	true	"Component public ID (sc_*)"
//	@Success		200		{object}	response.OK{data=object{component=SalaryComponent}}
//	@Failure		401		{object}	response.Error
//	@Failure		403		{object}	response.Error
//	@Failure		404		{object}	response.Error	"SALARY_COMPONENT_NOT_FOUND"
//	@Failure		500		{object}	response.Error
//	@Router			/organizations/{orgId}/hrm/setup/salary/components/{compId} [get]
func (h *Handler) GetComponent(c fiber.Ctx) error {
	orgID, ok := middleware.OrganizationIDFromCtx(c)
	if !ok {
		return response.BadRequest(c, "NO_ORGANIZATION_CONTEXT", "Organization context is required")
	}
	comp, err := h.service.GetComponent(c.Context(), orgID, c.Params("compId"))
	if err != nil {
		return h.compError(c, err)
	}
	return response.OK(c, fiber.Map{"component": comp}, "OK")
}

// UpdateComponent godoc
//
//	@Summary		Update salary component
//	@Description	Partially updates a salary component. Only non-null fields are applied.
//	@Description
//	@Description	**Note:** Changing a component's formula after payslips have been generated
//	@Description	does NOT retroactively change those payslips (payslips store snapshots).
//	@Description
//	@Description	**Required permission:** `hrm.salary.manage`
//	@Tags			HRM / Salary Setup
//	@Accept			json
//	@Produce		json
//	@Security		BearerAuth
//	@Param			orgId	path		string					true	"Organization ID"
//	@Param			compId	path		string					true	"Component public ID (sc_*)"
//	@Param			body	body		UpdateComponentRequest	true	"Fields to update"
//	@Success		200		{object}	response.OK{data=object{component=SalaryComponent}}
//	@Failure		400		{object}	response.Error
//	@Failure		401		{object}	response.Error
//	@Failure		403		{object}	response.Error
//	@Failure		404		{object}	response.Error	"SALARY_COMPONENT_NOT_FOUND"
//	@Failure		409		{object}	response.Error	"NAME_CONFLICT"
//	@Failure		500		{object}	response.Error
//	@Router			/organizations/{orgId}/hrm/setup/salary/components/{compId} [patch]
func (h *Handler) UpdateComponent(c fiber.Ctx) error {
	orgID, ok := middleware.OrganizationIDFromCtx(c)
	if !ok {
		return response.BadRequest(c, "NO_ORGANIZATION_CONTEXT", "Organization context is required")
	}
	var req UpdateComponentRequest
	if err := c.Bind().JSON(&req); err != nil {
		return response.BadRequest(c, "INVALID_BODY", "Invalid request body")
	}
	comp, err := h.service.UpdateComponent(c.Context(), orgID, c.Params("compId"), req)
	if err != nil {
		return h.compError(c, err)
	}
	return response.OK(c, fiber.Map{"component": comp}, "Salary component updated")
}

// DeleteComponent godoc
//
//	@Summary		Delete salary component
//	@Description	Permanently deletes a salary component.
//	@Description
//	@Description	**Warning:** Deletion will fail (DB constraint) if this component is
//	@Description	referenced by any salary structure. Remove it from structures first,
//	@Description	or deactivate it instead (`is_active: false`).
//	@Description
//	@Description	**Required permission:** `hrm.salary.manage`
//	@Tags			HRM / Salary Setup
//	@Produce		json
//	@Security		BearerAuth
//	@Param			orgId	path	string	true	"Organization ID"
//	@Param			compId	path	string	true	"Component public ID (sc_*)"
//	@Success		204		"Component deleted"
//	@Failure		401		{object}	response.Error
//	@Failure		403		{object}	response.Error
//	@Failure		404		{object}	response.Error	"SALARY_COMPONENT_NOT_FOUND"
//	@Failure		500		{object}	response.Error
//	@Router			/organizations/{orgId}/hrm/setup/salary/components/{compId} [delete]
func (h *Handler) DeleteComponent(c fiber.Ctx) error {
	orgID, ok := middleware.OrganizationIDFromCtx(c)
	if !ok {
		return response.BadRequest(c, "NO_ORGANIZATION_CONTEXT", "Organization context is required")
	}
	if err := h.service.DeleteComponent(c.Context(), orgID, c.Params("compId")); err != nil {
		return h.compError(c, err)
	}
	return response.NoContent(c)
}

// TestFormula godoc
//
//	@Summary		Test a salary formula
//	@Description	Evaluates a formula expression with provided test variable values.
//	@Description	Use this before saving a component to verify the expression is correct.
//	@Description
//	@Description	**Example body:**
//	@Description	`{"expression":"BASIC * 0.40","variables":{"BASIC":50000,"GROSS":75000}}`
//	@Description
//	@Description	**Required permission:** `hrm.salary.manage`
//	@Tags			HRM / Salary Setup
//	@Accept			json
//	@Produce		json
//	@Security		BearerAuth
//	@Param			orgId	path		string				true	"Organization ID"
//	@Param			body	body		TestFormulaRequest	true	"Formula and test variables"
//	@Success		200		{object}	response.OK{data=TestFormulaResponse}
//	@Failure		400		{object}	response.Error	"INVALID_BODY"
//	@Failure		401		{object}	response.Error
//	@Failure		403		{object}	response.Error
//	@Router			/organizations/{orgId}/hrm/setup/salary/formula/test [post]
func (h *Handler) TestFormula(c fiber.Ctx) error {
	var req TestFormulaRequest
	if err := c.Bind().JSON(&req); err != nil {
		return response.BadRequest(c, "INVALID_BODY", "Invalid request body")
	}
	result := h.service.TestFormula(c.Context(), req)
	return response.OK(c, fiber.Map{"formula_test": result}, "OK")
}

// ─────────────────────────────────────────────────────────────
// Salary Structures
// ─────────────────────────────────────────────────────────────

// ListStructures godoc
//
//	@Summary		List salary structures
//	@Description	Returns all salary structures defined for the organization.
//	@Description
//	@Description	**Required permission:** `hrm.salary.view`
//	@Tags			HRM / Salary Setup
//	@Produce		json
//	@Security		BearerAuth
//	@Param			orgId	path		string	true	"Organization ID"
//	@Param			active	query		bool	false	"When true, return only active structures"
//	@Success		200		{object}	response.OK{data=StructureListResponse}
//	@Failure		401		{object}	response.Error
//	@Failure		403		{object}	response.Error
//	@Failure		500		{object}	response.Error
//	@Router			/organizations/{orgId}/hrm/setup/salary/structures [get]
func (h *Handler) ListStructures(c fiber.Ctx) error {
	log := logger.FromCtx(c)
	orgID, ok := middleware.OrganizationIDFromCtx(c)
	if !ok {
		return response.BadRequest(c, "NO_ORGANIZATION_CONTEXT", "Organization context is required")
	}
	activeOnly := strings.ToLower(c.Query("active")) == "true"
	result, err := h.service.ListStructures(c.Context(), orgID, activeOnly)
	if err != nil {
		log.Error("salary: ListStructures", slog.Any("error", err))
		return response.InternalServerError(c)
	}
	return response.OK(c, result, "OK")
}

// CreateStructure godoc
//
//	@Summary		Create salary structure
//	@Description	Creates a new salary structure (named grouping of components).
//	@Description	After creation, add components via `POST /structures/{structId}/components`.
//	@Description
//	@Description	**Required permission:** `hrm.salary.manage`
//	@Tags			HRM / Salary Setup
//	@Accept			json
//	@Produce		json
//	@Security		BearerAuth
//	@Param			orgId	path		string					true	"Organization ID"
//	@Param			body	body		CreateStructureRequest	true	"Structure definition"
//	@Success		201		{object}	response.Created{data=object{structure=SalaryStructure}}
//	@Failure		400		{object}	response.Error
//	@Failure		401		{object}	response.Error
//	@Failure		403		{object}	response.Error
//	@Failure		409		{object}	response.Error	"STRUCTURE_NAME_CONFLICT"
//	@Failure		500		{object}	response.Error
//	@Router			/organizations/{orgId}/hrm/setup/salary/structures [post]
func (h *Handler) CreateStructure(c fiber.Ctx) error {
	userID, ok := middleware.UserIDFromCtx(c)
	if !ok {
		return response.Unauthorized(c, "UNAUTHORIZED", "Authentication required")
	}
	orgID, ok := middleware.OrganizationIDFromCtx(c)
	if !ok {
		return response.BadRequest(c, "NO_ORGANIZATION_CONTEXT", "Organization context is required")
	}
	var req CreateStructureRequest
	if err := c.Bind().JSON(&req); err != nil {
		return response.BadRequest(c, "INVALID_BODY", "Invalid request body")
	}
	st, err := h.service.CreateStructure(c.Context(), orgID, userID, req)
	if err != nil {
		return h.structError(c, err)
	}
	return response.Created(c, fiber.Map{"structure": st}, "Salary structure created")
}

// GetStructure godoc
//
//	@Summary		Get salary structure
//	@Description	Returns a salary structure including its full component list.
//	@Description
//	@Description	**Required permission:** `hrm.salary.view`
//	@Tags			HRM / Salary Setup
//	@Produce		json
//	@Security		BearerAuth
//	@Param			orgId		path		string	true	"Organization ID"
//	@Param			structId	path		string	true	"Structure public ID (ss_*)"
//	@Success		200			{object}	response.OK{data=object{structure=SalaryStructure}}
//	@Failure		401			{object}	response.Error
//	@Failure		403			{object}	response.Error
//	@Failure		404			{object}	response.Error	"SALARY_STRUCTURE_NOT_FOUND"
//	@Failure		500			{object}	response.Error
//	@Router			/organizations/{orgId}/hrm/setup/salary/structures/{structId} [get]
func (h *Handler) GetStructure(c fiber.Ctx) error {
	orgID, ok := middleware.OrganizationIDFromCtx(c)
	if !ok {
		return response.BadRequest(c, "NO_ORGANIZATION_CONTEXT", "Organization context is required")
	}
	st, err := h.service.GetStructure(c.Context(), orgID, c.Params("structId"))
	if err != nil {
		return h.structError(c, err)
	}
	return response.OK(c, fiber.Map{"structure": st}, "OK")
}

// UpdateStructure godoc
//
//	@Summary		Update salary structure
//	@Description	Partially updates a salary structure's metadata. To add/remove components use
//	@Description	the dedicated component endpoints.
//	@Description
//	@Description	**Required permission:** `hrm.salary.manage`
//	@Tags			HRM / Salary Setup
//	@Accept			json
//	@Produce		json
//	@Security		BearerAuth
//	@Param			orgId		path		string					true	"Organization ID"
//	@Param			structId	path		string					true	"Structure public ID (ss_*)"
//	@Param			body		body		UpdateStructureRequest	true	"Fields to update"
//	@Success		200			{object}	response.OK{data=object{structure=SalaryStructure}}
//	@Failure		400			{object}	response.Error
//	@Failure		401			{object}	response.Error
//	@Failure		403			{object}	response.Error
//	@Failure		404			{object}	response.Error	"SALARY_STRUCTURE_NOT_FOUND"
//	@Failure		409			{object}	response.Error	"STRUCTURE_NAME_CONFLICT"
//	@Failure		500			{object}	response.Error
//	@Router			/organizations/{orgId}/hrm/setup/salary/structures/{structId} [patch]
func (h *Handler) UpdateStructure(c fiber.Ctx) error {
	orgID, ok := middleware.OrganizationIDFromCtx(c)
	if !ok {
		return response.BadRequest(c, "NO_ORGANIZATION_CONTEXT", "Organization context is required")
	}
	var req UpdateStructureRequest
	if err := c.Bind().JSON(&req); err != nil {
		return response.BadRequest(c, "INVALID_BODY", "Invalid request body")
	}
	st, err := h.service.UpdateStructure(c.Context(), orgID, c.Params("structId"), req)
	if err != nil {
		return h.structError(c, err)
	}
	return response.OK(c, fiber.Map{"structure": st}, "Salary structure updated")
}

// DeleteStructure godoc
//
//	@Summary		Delete salary structure
//	@Description	Permanently deletes a salary structure. Fails if any employees have
//	@Description	salary records referencing this structure. Deactivate instead if in use.
//	@Description
//	@Description	**Required permission:** `hrm.salary.manage`
//	@Tags			HRM / Salary Setup
//	@Produce		json
//	@Security		BearerAuth
//	@Param			orgId		path	string	true	"Organization ID"
//	@Param			structId	path	string	true	"Structure public ID (ss_*)"
//	@Success		204			"Structure deleted"
//	@Failure		401			{object}	response.Error
//	@Failure		403			{object}	response.Error
//	@Failure		404			{object}	response.Error	"SALARY_STRUCTURE_NOT_FOUND"
//	@Failure		500			{object}	response.Error
//	@Router			/organizations/{orgId}/hrm/setup/salary/structures/{structId} [delete]
func (h *Handler) DeleteStructure(c fiber.Ctx) error {
	orgID, ok := middleware.OrganizationIDFromCtx(c)
	if !ok {
		return response.BadRequest(c, "NO_ORGANIZATION_CONTEXT", "Organization context is required")
	}
	if err := h.service.DeleteStructure(c.Context(), orgID, c.Params("structId")); err != nil {
		return h.structError(c, err)
	}
	return response.NoContent(c)
}

// AddComponentToStructure godoc
//
//	@Summary		Add component to structure
//	@Description	Adds an existing salary component to a structure.
//	@Description	Both must belong to the same organization.
//	@Description
//	@Description	**Required permission:** `hrm.salary.manage`
//	@Tags			HRM / Salary Setup
//	@Accept			json
//	@Produce		json
//	@Security		BearerAuth
//	@Param			orgId		path		string							true	"Organization ID"
//	@Param			structId	path		string							true	"Structure public ID (ss_*)"
//	@Param			body		body		AddComponentToStructureRequest	true	"Component to add"
//	@Success		200			{object}	response.OK						"Component added"
//	@Failure		400			{object}	response.Error
//	@Failure		401			{object}	response.Error
//	@Failure		403			{object}	response.Error
//	@Failure		404			{object}	response.Error
//	@Failure		409			{object}	response.Error	"COMPONENT_IN_STRUCTURE"
//	@Failure		500			{object}	response.Error
//	@Router			/organizations/{orgId}/hrm/setup/salary/structures/{structId}/components [post]
func (h *Handler) AddComponentToStructure(c fiber.Ctx) error {
	orgID, ok := middleware.OrganizationIDFromCtx(c)
	if !ok {
		return response.BadRequest(c, "NO_ORGANIZATION_CONTEXT", "Organization context is required")
	}
	var req AddComponentToStructureRequest
	if err := c.Bind().JSON(&req); err != nil {
		return response.BadRequest(c, "INVALID_BODY", "Invalid request body")
	}
	if req.ComponentID == "" {
		return response.BadRequest(c, "COMPONENT_ID_REQUIRED", "component_id is required")
	}
	if err := h.service.AddComponentToStructure(c.Context(), orgID, c.Params("structId"), req); err != nil {
		return h.compError(c, err)
	}
	return response.OK(c, nil, "Component added to structure")
}

// RemoveComponentFromStructure godoc
//
//	@Summary		Remove component from structure
//	@Description	Removes a salary component from a structure. The component itself is not deleted.
//	@Description
//	@Description	**Required permission:** `hrm.salary.manage`
//	@Tags			HRM / Salary Setup
//	@Produce		json
//	@Security		BearerAuth
//	@Param			orgId		path	string	true	"Organization ID"
//	@Param			structId	path	string	true	"Structure public ID (ss_*)"
//	@Param			compId		path	string	true	"Component public ID (sc_*)"
//	@Success		204			"Component removed"
//	@Failure		401			{object}	response.Error
//	@Failure		403			{object}	response.Error
//	@Failure		404			{object}	response.Error
//	@Failure		500			{object}	response.Error
//	@Router			/organizations/{orgId}/hrm/setup/salary/structures/{structId}/components/{compId} [delete]
func (h *Handler) RemoveComponentFromStructure(c fiber.Ctx) error {
	orgID, ok := middleware.OrganizationIDFromCtx(c)
	if !ok {
		return response.BadRequest(c, "NO_ORGANIZATION_CONTEXT", "Organization context is required")
	}
	if err := h.service.RemoveComponentFromStructure(c.Context(), orgID, c.Params("structId"), c.Params("compId")); err != nil {
		return h.compError(c, err)
	}
	return response.NoContent(c)
}

// ─────────────────────────────────────────────────────────────
// Employee Salary Records
// ─────────────────────────────────────────────────────────────

// GetSalaryHistory godoc
//
//	@Summary		Get employee salary history
//	@Description	Returns the full append-only salary history for an employee, newest first.
//	@Description	The most recent record with effective_date <= today is the active salary.
//	@Description
//	@Description	**Required permission:** `hrm.salary.employee.view`
//	@Tags			HRM / Employee Salary
//	@Produce		json
//	@Security		BearerAuth
//	@Param			orgId		path		string	true	"Organization ID"
//	@Param			employeeId	path		string	true	"Employee public ID (emp_*)"
//	@Success		200			{object}	response.OK{data=SalaryHistoryResponse}
//	@Failure		401			{object}	response.Error
//	@Failure		403			{object}	response.Error
//	@Failure		500			{object}	response.Error
//	@Router			/organizations/{orgId}/hrm/employees/{employeeId}/salary [get]
func (h *Handler) GetSalaryHistory(c fiber.Ctx) error {
	log := logger.FromCtx(c)
	userID, ok := middleware.UserIDFromCtx(c)
	if !ok {
		return response.Unauthorized(c, "UNAUTHORIZED", "Authentication required")
	}
	orgID, ok := middleware.OrganizationIDFromCtx(c)
	if !ok {
		return response.BadRequest(c, "NO_ORGANIZATION_CONTEXT", "Organization context is required")
	}
	employeeID := c.Params("employeeId")
	scopeTier, err := h.authz.ResolveScope(c.Context(), userID, orgID, "hrm.salary.employee")
	if err != nil {
		log.Error("salary: GetSalaryHistory", slog.Any("error", err))
		return response.InternalServerError(c)
	}
	allowed, err := h.scopeResolver.AuthorizeRecordAccess(c.Context(), scopeTier, orgID, userID, employeeID)
	if err != nil {
		log.Error("salary: GetSalaryHistory", slog.Any("error", err))
		return response.InternalServerError(c)
	}
	if !allowed {
		return response.Forbidden(c, "RECORD_ACCESS_DENIED", "You do not have access to this record")
	}
	result, err := h.service.GetSalaryHistory(c.Context(), orgID, employeeID)
	if err != nil {
		log.Error("salary: GetSalaryHistory", slog.Any("error", err))
		return response.InternalServerError(c)
	}
	return response.OK(c, result, "OK")
}

// AssignSalary godoc
//
//	@Summary		Assign salary to employee
//	@Description	Creates a new append-only salary record for an employee.
//	@Description	The previous record remains intact — this adds a new effective_date entry.
//	@Description
//	@Description	Use `change_reason: joining` for first assignments, `promotion` for raises,
//	@Description	`annual_revision` for yearly increments, etc.
//	@Description
//	@Description	**Required permission:** `hrm.salary.employee.manage`
//	@Description
//	@Description	**Error codes:**
//	@Description	- `BASIC_PAY_REQUIRED` — basic_pay missing or negative
//	@Description	- `EFFECTIVE_DATE_REQUIRED` — effective_date missing
//	@Description	- `INVALID_EFFECTIVE_DATE` — not in YYYY-MM-DD format
//	@Description	- `INVALID_CHANGE_REASON` — reason not in allowed set
//	@Description	- `SALARY_STRUCTURE_NOT_FOUND` — structure_id does not exist in this org
//	@Tags			HRM / Employee Salary
//	@Accept			json
//	@Produce		json
//	@Security		BearerAuth
//	@Param			orgId		path		string				true	"Organization ID"
//	@Param			employeeId	path		string				true	"Employee public ID (emp_*)"
//	@Param			body		body		AssignSalaryRequest	true	"Salary assignment"
//	@Success		201			{object}	response.Created{data=object{salary_record=EmployeeSalaryRecord}}
//	@Failure		400			{object}	response.Error
//	@Failure		401			{object}	response.Error
//	@Failure		403			{object}	response.Error
//	@Failure		404			{object}	response.Error	"SALARY_STRUCTURE_NOT_FOUND"
//	@Failure		500			{object}	response.Error
//	@Router			/organizations/{orgId}/hrm/employees/{employeeId}/salary [post]
func (h *Handler) AssignSalary(c fiber.Ctx) error {
	userID, ok := middleware.UserIDFromCtx(c)
	if !ok {
		return response.Unauthorized(c, "UNAUTHORIZED", "Authentication required")
	}
	orgID, ok := middleware.OrganizationIDFromCtx(c)
	if !ok {
		return response.BadRequest(c, "NO_ORGANIZATION_CONTEXT", "Organization context is required")
	}
	var req AssignSalaryRequest
	if err := c.Bind().JSON(&req); err != nil {
		return response.BadRequest(c, "INVALID_BODY", "Invalid request body")
	}
	rec, err := h.service.AssignSalary(c.Context(), orgID, c.Params("employeeId"), userID, req)
	if err != nil {
		return h.salaryRecordError(c, err)
	}
	return response.Created(c, fiber.Map{"salary_record": rec}, "Salary assigned")
}

// ─────────────────────────────────────────────────────────────
// Error mappers
// ─────────────────────────────────────────────────────────────

func (h *Handler) compError(c fiber.Ctx, err error) error {
	log := logger.FromCtx(c)
	switch {
	case errors.Is(err, ErrComponentNotFound):
		return response.NotFound(c, "SALARY_COMPONENT_NOT_FOUND", "Salary component not found")
	case errors.Is(err, ErrNameRequired):
		return response.BadRequest(c, "NAME_REQUIRED", "Name is required")
	case errors.Is(err, ErrNameTooLong):
		return response.BadRequest(c, "NAME_TOO_LONG", "Name must not exceed 150 characters")
	case errors.Is(err, ErrNameConflict):
		return response.Conflict(c, "NAME_CONFLICT", "A component with this name already exists")
	case errors.Is(err, ErrInvalidComponentType):
		return response.BadRequest(c, "INVALID_COMPONENT_TYPE", "component_type must be: earning, deduction, or employer_contribution")
	case errors.Is(err, ErrInvalidCalcMethod):
		return response.BadRequest(c, "INVALID_CALC_METHOD", "calc_method must be: fixed, pct_of_basic, pct_of_gross, formula, manual, or slab")
	case errors.Is(err, ErrFormulaRequired):
		return response.BadRequest(c, "FORMULA_REQUIRED", "formula_expression is required when calc_method is formula")
	case errors.Is(err, ErrInvalidFormula):
		return response.BadRequest(c, "INVALID_FORMULA", "formula_expression syntax is invalid")
	case errors.Is(err, ErrSlabRequired):
		return response.BadRequest(c, "SLAB_REQUIRED", "slab_config is required when calc_method is slab")
	case errors.Is(err, ErrInvalidSlab):
		return response.BadRequest(c, "INVALID_SLAB", "slab_config slabs must be in ascending order with one null up_to at the end")
	case errors.Is(err, ErrStructureNotFound):
		return response.NotFound(c, "SALARY_STRUCTURE_NOT_FOUND", "Salary structure not found")
	case errors.Is(err, ErrComponentInStructure):
		return response.Conflict(c, "COMPONENT_IN_STRUCTURE", "Component is already in this structure")
	case errors.Is(err, ErrComponentNotInStructure):
		return response.NotFound(c, "COMPONENT_NOT_IN_STRUCTURE", "Component is not in this structure")
	default:
		log.Error("salary: component error", slog.Any("error", err))
		return response.InternalServerError(c)
	}
}

func (h *Handler) structError(c fiber.Ctx, err error) error {
	log := logger.FromCtx(c)
	switch {
	case errors.Is(err, ErrStructureNotFound):
		return response.NotFound(c, "SALARY_STRUCTURE_NOT_FOUND", "Salary structure not found")
	case errors.Is(err, ErrNameRequired):
		return response.BadRequest(c, "NAME_REQUIRED", "Name is required")
	case errors.Is(err, ErrNameTooLong):
		return response.BadRequest(c, "NAME_TOO_LONG", "Name must not exceed 150 characters")
	case errors.Is(err, ErrStructureNameConflict):
		return response.Conflict(c, "STRUCTURE_NAME_CONFLICT", "A structure with this name already exists")
	default:
		log.Error("salary: structure error", slog.Any("error", err))
		return response.InternalServerError(c)
	}
}

func (h *Handler) salaryRecordError(c fiber.Ctx, err error) error {
	log := logger.FromCtx(c)
	switch {
	case errors.Is(err, ErrBasicPayRequired):
		return response.BadRequest(c, "BASIC_PAY_REQUIRED", "basic_pay is required and must be >= 0")
	case errors.Is(err, ErrEffectiveDateRequired):
		return response.BadRequest(c, "EFFECTIVE_DATE_REQUIRED", "effective_date is required")
	case errors.Is(err, ErrInvalidEffectiveDate):
		return response.BadRequest(c, "INVALID_EFFECTIVE_DATE", "effective_date must be in YYYY-MM-DD format")
	case errors.Is(err, ErrInvalidChangeReason):
		return response.BadRequest(c, "INVALID_CHANGE_REASON", "change_reason must be: joining, promotion, annual_revision, transfer, correction, or other")
	case errors.Is(err, ErrStructureNotFound):
		return response.NotFound(c, "SALARY_STRUCTURE_NOT_FOUND", "The specified salary structure was not found")
	default:
		log.Error("salary: record error", slog.Any("error", err))
		return response.InternalServerError(c)
	}
}
