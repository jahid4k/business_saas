// backend/internal/hrm/promotions/handler.go
package promotions

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

// Handler handles HRM promotion HTTP endpoints.
type Handler struct {
	service       Service
	authz         authz.Service
	scopeResolver *scope.Resolver
}

func NewHandler(service Service, authzSvc authz.Service, scopeResolver *scope.Resolver) *Handler {
	return &Handler{service: service, authz: authzSvc, scopeResolver: scopeResolver}
}

// resolveListFilter builds the shared parts of a PromotionListFilter (scope,
// pagination, status) once userID is already known — ListAll and
// ListForEmployee each add their own employee_id source (query param vs path
// param) on top. err is a plain ResolveScope failure, never a written
// response — callers log and 500 it themselves.
func (h *Handler) resolveListFilter(c fiber.Ctx, orgID, userID string) (PromotionListFilter, error) {
	scopeTier, err := h.authz.ResolveScope(c.Context(), userID, orgID, "hrm.promotions")
	if err != nil {
		return PromotionListFilter{}, err
	}
	filter := PromotionListFilter{
		Status:       c.Query("status"),
		Scope:        scopeTier,
		CallerUserID: userID,
	}
	if limit, err := strconv.Atoi(c.Query("limit", "")); err == nil {
		filter.Limit = limit
	}
	if offset, err := strconv.Atoi(c.Query("offset", "")); err == nil {
		filter.Offset = offset
	}
	return filter, nil
}

// ListAll godoc
//
//	@Summary		List all promotions (HR view)
//	@Description	Returns all promotion records across the organization.
//	@Description	Filter by status or employee_id query params.
//	@Description
//	@Description	**Required permission:** `hrm.promotions.view`
//	@Tags			HRM / Promotions
//	@Produce		json
//	@Security		BearerAuth
//	@Param			orgId		path		string	true	"Organization ID"
//	@Param			status		query		string	false	"draft|pending_approval|approved|rejected|cancelled|applied"
//	@Param			employee_id	query		string	false	"Filter by employee UUID"
//	@Success		200			{object}	response.OK{data=PromotionListResponse}
//	@Failure		401			{object}	response.Error
//	@Failure		403			{object}	response.Error
//	@Failure		500			{object}	response.Error
//	@Router			/organizations/{orgId}/hrm/promotions [get]
func (h *Handler) ListAll(c fiber.Ctx) error {
	log := logger.FromCtx(c)
	userID, ok := middleware.UserIDFromCtx(c)
	if !ok {
		return response.Unauthorized(c, "UNAUTHORIZED", "Authentication required")
	}
	orgID, ok := middleware.OrganizationIDFromCtx(c)
	if !ok {
		return response.BadRequest(c, "NO_ORGANIZATION_CONTEXT", "Organization context is required")
	}
	filter, err := h.resolveListFilter(c, orgID, userID)
	if err != nil {
		log.Error("promotions: ListAll", slog.Any("error", err))
		return response.InternalServerError(c)
	}
	filter.EmployeeID = c.Query("employee_id")
	res, err := h.service.List(c.Context(), orgID, filter)
	if err != nil {
		log.Error("promotions: ListAll", slog.Any("error", err))
		return response.InternalServerError(c)
	}
	return response.OK(c, res, "OK")
}

// ListForEmployee godoc
//
//	@Summary		List employee promotions
//	@Description	Returns all promotion records for a specific employee.
//	@Description
//	@Description	**Required permission:** `hrm.promotions.view`
//	@Tags			HRM / Promotions
//	@Produce		json
//	@Security		BearerAuth
//	@Param			orgId		path		string	true	"Organization ID"
//	@Param			employeeId	path		string	true	"Employee public ID (emp_*)"
//	@Param			status		query		string	false	"Filter by status"
//	@Success		200			{object}	response.OK{data=PromotionListResponse}
//	@Failure		401			{object}	response.Error
//	@Failure		403			{object}	response.Error
//	@Failure		500			{object}	response.Error
//	@Router			/organizations/{orgId}/hrm/employees/{employeeId}/promotions [get]
func (h *Handler) ListForEmployee(c fiber.Ctx) error {
	log := logger.FromCtx(c)
	userID, ok := middleware.UserIDFromCtx(c)
	if !ok {
		return response.Unauthorized(c, "UNAUTHORIZED", "Authentication required")
	}
	orgID, ok := middleware.OrganizationIDFromCtx(c)
	if !ok {
		return response.BadRequest(c, "NO_ORGANIZATION_CONTEXT", "Organization context is required")
	}
	filter, err := h.resolveListFilter(c, orgID, userID)
	if err != nil {
		log.Error("promotions: ListForEmployee", slog.Any("error", err))
		return response.InternalServerError(c)
	}
	filter.EmployeeID = c.Params("employeeId")
	res, err := h.service.List(c.Context(), orgID, filter)
	if err != nil {
		log.Error("promotions: ListForEmployee", slog.Any("error", err))
		return response.InternalServerError(c)
	}
	return response.OK(c, res, "OK")
}

// Create godoc
//
//	@Summary		Create promotion record
//	@Description	Creates a promotion record in draft status. Employee's current
//	@Description	position/department/salary are snapshotted automatically.
//	@Description
//	@Description	Submit via `POST .../submit` → approved → Apply via `POST .../apply`.
//	@Description
//	@Description	**Required permission:** `hrm.promotions.manage`
//	@Description
//	@Description	**Error codes:** `TO_POSITION_REQUIRED` · `EFFECTIVE_DATE_REQUIRED` · `INVALID_DATE`
//	@Tags			HRM / Promotions
//	@Accept			json
//	@Produce		json
//	@Security		BearerAuth
//	@Param			orgId		path		string					true	"Organization ID"
//	@Param			employeeId	path		string					true	"Employee public ID"
//	@Param			body		body		CreatePromotionRequest	true	"Promotion details"
//	@Success		201			{object}	response.Created{data=object{promotion=Promotion}}
//	@Failure		400			{object}	response.Error
//	@Failure		401			{object}	response.Error
//	@Failure		403			{object}	response.Error
//	@Failure		500			{object}	response.Error
//	@Router			/organizations/{orgId}/hrm/employees/{employeeId}/promotions [post]
func (h *Handler) Create(c fiber.Ctx) error {
	userID, ok := middleware.UserIDFromCtx(c)
	if !ok {
		return response.Unauthorized(c, "UNAUTHORIZED", "Authentication required")
	}
	orgID, ok := middleware.OrganizationIDFromCtx(c)
	if !ok {
		return response.BadRequest(c, "NO_ORGANIZATION_CONTEXT", "Organization context is required")
	}
	var req CreatePromotionRequest
	if err := c.Bind().JSON(&req); err != nil {
		return response.BadRequest(c, "INVALID_BODY", "Invalid request body")
	}
	p, err := h.service.Create(c.Context(), orgID, c.Params("employeeId"), userID, req)
	if err != nil {
		return h.err(c, err)
	}
	return response.Created(c, fiber.Map{"promotion": p}, "Promotion record created")
}

// Get godoc
//
//	@Summary		Get promotion record
//	@Description	Returns a single promotion record by its public ID.
//	@Description
//	@Description	**Required permission:** `hrm.promotions.view`
//	@Tags			HRM / Promotions
//	@Produce		json
//	@Security		BearerAuth
//	@Param			orgId		path		string	true	"Organization ID"
//	@Param			employeeId	path		string	true	"Employee public ID"
//	@Param			promotionId	path		string	true	"Promotion public ID (promo_*)"
//	@Success		200			{object}	response.OK{data=object{promotion=Promotion}}
//	@Failure		401			{object}	response.Error
//	@Failure		403			{object}	response.Error
//	@Failure		404			{object}	response.Error	"PROMOTION_NOT_FOUND"
//	@Failure		500			{object}	response.Error
//	@Router			/organizations/{orgId}/hrm/employees/{employeeId}/promotions/{promotionId} [get]
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
	employeeID := c.Params("employeeId")
	scopeTier, err := h.authz.ResolveScope(c.Context(), userID, orgID, "hrm.promotions")
	if err != nil {
		log.Error("promotions: Get", slog.Any("error", err))
		return response.InternalServerError(c)
	}
	allowed, err := h.scopeResolver.AuthorizeRecordAccess(c.Context(), scopeTier, orgID, userID, employeeID)
	if err != nil {
		log.Error("promotions: Get", slog.Any("error", err))
		return response.InternalServerError(c)
	}
	if !allowed {
		return response.Forbidden(c, "RECORD_ACCESS_DENIED", "You do not have access to this record")
	}
	p, err := h.service.Get(c.Context(), orgID, employeeID, c.Params("promotionId"))
	if err != nil {
		return h.err(c, err)
	}
	return response.OK(c, fiber.Map{"promotion": p}, "OK")
}

// Update godoc
//
//	@Summary		Update promotion record
//	@Description	Partially updates a draft promotion. Only draft records can be updated.
//	@Description
//	@Description	**Required permission:** `hrm.promotions.manage`
//	@Tags			HRM / Promotions
//	@Accept			json
//	@Produce		json
//	@Security		BearerAuth
//	@Param			orgId		path		string					true	"Organization ID"
//	@Param			employeeId	path		string					true	"Employee public ID"
//	@Param			promotionId	path		string					true	"Promotion public ID"
//	@Param			body		body		UpdatePromotionRequest	true	"Fields to update"
//	@Success		200			{object}	response.OK{data=object{promotion=Promotion}}
//	@Failure		400			{object}	response.Error
//	@Failure		401			{object}	response.Error
//	@Failure		403			{object}	response.Error
//	@Failure		404			{object}	response.Error
//	@Failure		409			{object}	response.Error	"WRONG_STATUS"
//	@Failure		500			{object}	response.Error
//	@Router			/organizations/{orgId}/hrm/employees/{employeeId}/promotions/{promotionId} [patch]
func (h *Handler) Update(c fiber.Ctx) error {
	orgID, ok := middleware.OrganizationIDFromCtx(c)
	if !ok {
		return response.BadRequest(c, "NO_ORGANIZATION_CONTEXT", "Organization context is required")
	}
	var req UpdatePromotionRequest
	if err := c.Bind().JSON(&req); err != nil {
		return response.BadRequest(c, "INVALID_BODY", "Invalid request body")
	}
	p, err := h.service.Update(c.Context(), orgID, c.Params("employeeId"), c.Params("promotionId"), req)
	if err != nil {
		return h.err(c, err)
	}
	return response.OK(c, fiber.Map{"promotion": p}, "Promotion updated")
}

// Submit godoc
//
//	@Summary		Submit promotion for approval
//	@Description	Moves draft → approved (or pending_approval if an approval chain is configured).
//	@Description
//	@Description	**Required permission:** `hrm.promotions.manage`
//	@Tags			HRM / Promotions
//	@Produce		json
//	@Security		BearerAuth
//	@Param			orgId		path	string	true	"Organization ID"
//	@Param			employeeId	path	string	true	"Employee public ID"
//	@Param			promotionId	path	string	true	"Promotion public ID"
//	@Success		200			{object}	response.OK{data=object{promotion=Promotion}}
//	@Failure		401			{object}	response.Error
//	@Failure		403			{object}	response.Error
//	@Failure		404			{object}	response.Error
//	@Failure		409			{object}	response.Error	"WRONG_STATUS"
//	@Failure		500			{object}	response.Error
//	@Router			/organizations/{orgId}/hrm/employees/{employeeId}/promotions/{promotionId}/submit [post]
func (h *Handler) Submit(c fiber.Ctx) error {
	userID, ok := middleware.UserIDFromCtx(c)
	if !ok {
		return response.Unauthorized(c, "UNAUTHORIZED", "Authentication required")
	}
	orgID, ok := middleware.OrganizationIDFromCtx(c)
	if !ok {
		return response.BadRequest(c, "NO_ORGANIZATION_CONTEXT", "Organization context is required")
	}
	p, err := h.service.Submit(c.Context(), orgID, c.Params("employeeId"), c.Params("promotionId"), userID)
	if err != nil {
		return h.err(c, err)
	}
	return response.OK(c, fiber.Map{"promotion": p}, "Promotion submitted")
}

// Cancel godoc
//
//	@Summary		Cancel promotion
//	@Description	Cancels a promotion that has not yet been applied.
//	@Description
//	@Description	**Required permission:** `hrm.promotions.manage`
//	@Tags			HRM / Promotions
//	@Produce		json
//	@Security		BearerAuth
//	@Param			orgId		path	string	true	"Organization ID"
//	@Param			employeeId	path	string	true	"Employee public ID"
//	@Param			promotionId	path	string	true	"Promotion public ID"
//	@Success		200			{object}	response.OK{data=object{promotion=Promotion}}
//	@Failure		401			{object}	response.Error
//	@Failure		403			{object}	response.Error
//	@Failure		404			{object}	response.Error
//	@Failure		409			{object}	response.Error
//	@Failure		500			{object}	response.Error
//	@Router			/organizations/{orgId}/hrm/employees/{employeeId}/promotions/{promotionId}/cancel [post]
func (h *Handler) Cancel(c fiber.Ctx) error {
	orgID, ok := middleware.OrganizationIDFromCtx(c)
	if !ok {
		return response.BadRequest(c, "NO_ORGANIZATION_CONTEXT", "Organization context is required")
	}
	p, err := h.service.Cancel(c.Context(), orgID, c.Params("employeeId"), c.Params("promotionId"))
	if err != nil {
		return h.err(c, err)
	}
	return response.OK(c, fiber.Map{"promotion": p}, "Promotion cancelled")
}

// Apply godoc
//
//	@Summary		Apply promotion to employee record
//	@Description	Executes the promotion in one transaction:
//	@Description	- Updates `employee.position_id` and optionally `employee.department_id`
//	@Description	- Creates a new `hrm_employee_salary_records` row if pay is changing
//	@Description
//	@Description	**Prerequisites:** status must be `approved`.
//	@Description	**Required permission:** `hrm.promotions.apply`
//	@Tags			HRM / Promotions
//	@Produce		json
//	@Security		BearerAuth
//	@Param			orgId		path	string	true	"Organization ID"
//	@Param			employeeId	path	string	true	"Employee public ID"
//	@Param			promotionId	path	string	true	"Promotion public ID"
//	@Success		200			{object}	response.OK{data=object{promotion=Promotion}}
//	@Failure		401			{object}	response.Error
//	@Failure		403			{object}	response.Error
//	@Failure		404			{object}	response.Error
//	@Failure		409			{object}	response.Error	"ALREADY_APPLIED or NOT_APPROVED"
//	@Failure		500			{object}	response.Error
//	@Router			/organizations/{orgId}/hrm/employees/{employeeId}/promotions/{promotionId}/apply [post]
func (h *Handler) Apply(c fiber.Ctx) error {
	userID, ok := middleware.UserIDFromCtx(c)
	if !ok {
		return response.Unauthorized(c, "UNAUTHORIZED", "Authentication required")
	}
	orgID, ok := middleware.OrganizationIDFromCtx(c)
	if !ok {
		return response.BadRequest(c, "NO_ORGANIZATION_CONTEXT", "Organization context is required")
	}
	p, err := h.service.Apply(c.Context(), orgID, c.Params("employeeId"), c.Params("promotionId"), userID)
	if err != nil {
		return h.err(c, err)
	}
	return response.OK(c, fiber.Map{"promotion": p}, "Promotion applied")
}

func (h *Handler) err(c fiber.Ctx, err error) error {
	log := logger.FromCtx(c)
	switch {
	case errors.Is(err, ErrNotFound):
		return response.NotFound(c, "PROMOTION_NOT_FOUND", "Promotion record not found")
	case errors.Is(err, ErrToPositionRequired):
		return response.BadRequest(c, "TO_POSITION_REQUIRED", "to_position_id is required")
	case errors.Is(err, ErrEffectiveDateReq):
		return response.BadRequest(c, "EFFECTIVE_DATE_REQUIRED", "effective_date is required (YYYY-MM-DD)")
	case errors.Is(err, ErrInvalidDate):
		return response.BadRequest(c, "INVALID_DATE", "effective_date must be a valid YYYY-MM-DD")
	case errors.Is(err, ErrWrongStatus):
		return response.Conflict(c, "WRONG_STATUS", "Action not allowed in current promotion status")
	case errors.Is(err, ErrAlreadyApplied):
		return response.Conflict(c, "ALREADY_APPLIED", "Promotion has already been applied")
	case errors.Is(err, ErrNotApproved):
		return response.Conflict(c, "NOT_APPROVED", "Promotion must be approved before applying")
	default:
		log.Error("promotions: error", slog.Any("error", err))
		return response.InternalServerError(c)
	}
}
