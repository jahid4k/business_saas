// backend/internal/hrm/contracts/handler.go
package contracts

import (
	"errors"
	"log/slog"

	"github.com/gofiber/fiber/v3"

	"github.com/mridha/businesssaas/internal/middleware"
	"github.com/mridha/businesssaas/pkg/logger"
	"github.com/mridha/businesssaas/pkg/response"
)

type Handler struct{ service Service }
func NewHandler(service Service) *Handler { return &Handler{service: service} }

// List godoc
//
//	@Summary		List employee contracts
//	@Description	Returns the full contract history for an employee (newest first).
//	@Description
//	@Description	**Required permission:** `hrm.contracts.view`
//	@Tags			HRM / Contracts
//	@Produce		json
//	@Security		BearerAuth
//	@Param			orgId		path		string	true	"Organization ID"
//	@Param			employeeId	path		string	true	"Employee public ID (emp_*)"
//	@Success		200			{object}	response.OK{data=ContractListResponse}
//	@Failure		401			{object}	response.Error
//	@Failure		403			{object}	response.Error
//	@Failure		500			{object}	response.Error
//	@Router			/organizations/{orgId}/hrm/employees/{employeeId}/contracts [get]
func (h *Handler) List(c fiber.Ctx) error {
	log := logger.FromCtx(c)
	orgID, ok := middleware.OrganizationIDFromCtx(c)
	if !ok { return response.BadRequest(c, "NO_ORGANIZATION_CONTEXT", "Organization context is required") }
	res, err := h.service.List(c.Context(), orgID, c.Params("employeeId"))
	if err != nil { log.Error("contracts: List", slog.Any("error", err)); return response.InternalServerError(c) }
	return response.OK(c, res, "OK")
}

// Create godoc
//
//	@Summary		Create employee contract
//	@Description	Creates a new employment contract for an employee.
//	@Description
//	@Description	Only one active contract is allowed per employee at a time.
//	@Description	If an active contract exists, deactivate it first before creating a new one.
//	@Description
//	@Description	`notice_period_days` defaults to 30 if not provided. Used by the resignation
//	@Description	flow to auto-compute the last working day.
//	@Description
//	@Description	**Required permission:** `hrm.contracts.manage`
//	@Description
//	@Description	**Error codes:**
//	@Description	- `INVALID_CONTRACT_TYPE` — not in allowed set
//	@Description	- `START_DATE_REQUIRED` — missing or invalid date
//	@Description	- `ACTIVE_CONTRACT_EXISTS` — employee has an active contract; deactivate first
//	@Tags			HRM / Contracts
//	@Accept			json
//	@Produce		json
//	@Security		BearerAuth
//	@Param			orgId		path		string					true	"Organization ID"
//	@Param			employeeId	path		string					true	"Employee public ID (emp_*)"
//	@Param			body		body		CreateContractRequest	true	"Contract details"
//	@Success		201			{object}	response.Created{data=object{contract=EmployeeContract}}
//	@Failure		400			{object}	response.Error
//	@Failure		401			{object}	response.Error
//	@Failure		403			{object}	response.Error
//	@Failure		409			{object}	response.Error	"ACTIVE_CONTRACT_EXISTS"
//	@Failure		500			{object}	response.Error
//	@Router			/organizations/{orgId}/hrm/employees/{employeeId}/contracts [post]
func (h *Handler) Create(c fiber.Ctx) error {
	userID, ok := middleware.UserIDFromCtx(c)
	if !ok { return response.Unauthorized(c, "UNAUTHORIZED", "Authentication required") }
	orgID, ok := middleware.OrganizationIDFromCtx(c)
	if !ok { return response.BadRequest(c, "NO_ORGANIZATION_CONTEXT", "Organization context is required") }
	var req CreateContractRequest
	if err := c.Bind().JSON(&req); err != nil { return response.BadRequest(c, "INVALID_BODY", "Invalid request body") }
	contract, err := h.service.Create(c.Context(), orgID, c.Params("employeeId"), userID, req)
	if err != nil { return h.contractError(c, err) }
	return response.Created(c, fiber.Map{"contract": contract}, "Contract created")
}

// Get godoc
//
//	@Summary		Get employee contract
//	@Description	Returns a single contract by its public ID.
//	@Description
//	@Description	**Required permission:** `hrm.contracts.view`
//	@Tags			HRM / Contracts
//	@Produce		json
//	@Security		BearerAuth
//	@Param			orgId		path		string	true	"Organization ID"
//	@Param			employeeId	path		string	true	"Employee public ID (emp_*)"
//	@Param			contractId	path		string	true	"Contract public ID (ec_*)"
//	@Success		200			{object}	response.OK{data=object{contract=EmployeeContract}}
//	@Failure		401			{object}	response.Error
//	@Failure		403			{object}	response.Error
//	@Failure		404			{object}	response.Error	"CONTRACT_NOT_FOUND"
//	@Failure		500			{object}	response.Error
//	@Router			/organizations/{orgId}/hrm/employees/{employeeId}/contracts/{contractId} [get]
func (h *Handler) Get(c fiber.Ctx) error {
	orgID, ok := middleware.OrganizationIDFromCtx(c)
	if !ok { return response.BadRequest(c, "NO_ORGANIZATION_CONTEXT", "Organization context is required") }
	contract, err := h.service.Get(c.Context(), orgID, c.Params("employeeId"), c.Params("contractId"))
	if err != nil { return h.contractError(c, err) }
	return response.OK(c, fiber.Map{"contract": contract}, "OK")
}

// Update godoc
//
//	@Summary		Update employee contract
//	@Description	Partially updates a contract. Cannot change contract_type or start_date.
//	@Description	To change those, deactivate the current contract and create a new one.
//	@Description
//	@Description	**Required permission:** `hrm.contracts.manage`
//	@Tags			HRM / Contracts
//	@Accept			json
//	@Produce		json
//	@Security		BearerAuth
//	@Param			orgId		path		string					true	"Organization ID"
//	@Param			employeeId	path		string					true	"Employee public ID (emp_*)"
//	@Param			contractId	path		string					true	"Contract public ID (ec_*)"
//	@Param			body		body		UpdateContractRequest	true	"Fields to update"
//	@Success		200			{object}	response.OK{data=object{contract=EmployeeContract}}
//	@Failure		400			{object}	response.Error
//	@Failure		401			{object}	response.Error
//	@Failure		403			{object}	response.Error
//	@Failure		404			{object}	response.Error
//	@Failure		500			{object}	response.Error
//	@Router			/organizations/{orgId}/hrm/employees/{employeeId}/contracts/{contractId} [patch]
func (h *Handler) Update(c fiber.Ctx) error {
	orgID, ok := middleware.OrganizationIDFromCtx(c)
	if !ok { return response.BadRequest(c, "NO_ORGANIZATION_CONTEXT", "Organization context is required") }
	var req UpdateContractRequest
	if err := c.Bind().JSON(&req); err != nil { return response.BadRequest(c, "INVALID_BODY", "Invalid request body") }
	contract, err := h.service.Update(c.Context(), orgID, c.Params("employeeId"), c.Params("contractId"), req)
	if err != nil { return h.contractError(c, err) }
	return response.OK(c, fiber.Map{"contract": contract}, "Contract updated")
}

// Deactivate godoc
//
//	@Summary		Deactivate employee contract
//	@Description	Marks a contract as inactive. The employee can then have a new contract created.
//	@Description	This is the correct way to end a contract — not deletion.
//	@Description
//	@Description	**Required permission:** `hrm.contracts.manage`
//	@Tags			HRM / Contracts
//	@Produce		json
//	@Security		BearerAuth
//	@Param			orgId		path	string	true	"Organization ID"
//	@Param			employeeId	path	string	true	"Employee public ID (emp_*)"
//	@Param			contractId	path	string	true	"Contract public ID (ec_*)"
//	@Success		200			{object}	response.OK	"Contract deactivated"
//	@Failure		401			{object}	response.Error
//	@Failure		403			{object}	response.Error
//	@Failure		404			{object}	response.Error
//	@Failure		500			{object}	response.Error
//	@Router			/organizations/{orgId}/hrm/employees/{employeeId}/contracts/{contractId}/deactivate [post]
func (h *Handler) Deactivate(c fiber.Ctx) error {
	orgID, ok := middleware.OrganizationIDFromCtx(c)
	if !ok { return response.BadRequest(c, "NO_ORGANIZATION_CONTEXT", "Organization context is required") }
	if err := h.service.Deactivate(c.Context(), orgID, c.Params("employeeId"), c.Params("contractId")); err != nil { return h.contractError(c, err) }
	return response.OK(c, nil, "Contract deactivated")
}

func (h *Handler) contractError(c fiber.Ctx, err error) error {
	log := logger.FromCtx(c)
	switch {
	case errors.Is(err, ErrContractNotFound): return response.NotFound(c, "CONTRACT_NOT_FOUND", "Contract not found")
	case errors.Is(err, ErrInvalidContractType): return response.BadRequest(c, "INVALID_CONTRACT_TYPE", "contract_type must be: permanent, fixed_term, probation, internship, or consultant")
	case errors.Is(err, ErrStartDateRequired): return response.BadRequest(c, "START_DATE_REQUIRED", "start_date is required (YYYY-MM-DD)")
	case errors.Is(err, ErrInvalidStartDate): return response.BadRequest(c, "INVALID_START_DATE", "start_date must be a valid YYYY-MM-DD date")
	case errors.Is(err, ErrActiveContractExists): return response.Conflict(c, "ACTIVE_CONTRACT_EXISTS", "Employee already has an active contract — deactivate it first")
	default: log.Error("contracts: error", slog.Any("error", err)); return response.InternalServerError(c)
	}
}
