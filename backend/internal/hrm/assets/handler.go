// backend/internal/hrm/assets/handler.go
package assets

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

// Handler handles HRM asset HTTP endpoints. hrm.assets is scope-tiered
// (00107), so this mirrors payslips / compensation / loans / benefits.
type Handler struct {
	service       Service
	authz         authz.Service
	scopeResolver *scope.Resolver
}

func NewHandler(service Service, authzSvc authz.Service, scopeResolver *scope.Resolver) *Handler {
	return &Handler{service: service, authz: authzSvc, scopeResolver: scopeResolver}
}

func requestUser(c fiber.Ctx) (string, bool) { return middleware.UserIDFromCtx(c) }
func requestOrg(c fiber.Ctx) (string, bool)  { return middleware.OrganizationIDFromCtx(c) }

func (h *Handler) err(c fiber.Ctx, err error) error {
	log := logger.FromCtx(c)
	switch {
	case errors.Is(err, ErrCategoryNotFound):
		return response.NotFound(c, "CATEGORY_NOT_FOUND", "Asset category not found")
	case errors.Is(err, ErrAssetNotFound):
		return response.NotFound(c, "ASSET_NOT_FOUND", "Asset not found")
	case errors.Is(err, ErrAssignmentNotFound):
		return response.NotFound(c, "ASSIGNMENT_NOT_FOUND", "Asset assignment not found")
	case errors.Is(err, ErrRequestNotFound):
		return response.NotFound(c, "REQUEST_NOT_FOUND", "Asset request not found")
	case errors.Is(err, ErrLicenseNotFound):
		return response.NotFound(c, "LICENSE_NOT_FOUND", "Software licence not found")
	case errors.Is(err, ErrSeatNotFound):
		return response.NotFound(c, "SEAT_NOT_FOUND", "Licence seat assignment not found")
	case errors.Is(err, ErrInvalidAmount):
		return response.BadRequest(c, "INVALID_AMOUNT", err.Error())
	case errors.Is(err, ErrInvalidCondition):
		return response.BadRequest(c, "INVALID_CONDITION", err.Error())
	case errors.Is(err, ErrInvalidMaintenanceType):
		return response.BadRequest(c, "INVALID_MAINTENANCE_TYPE", err.Error())
	case errors.Is(err, ErrInvalidSeatsTotal):
		return response.BadRequest(c, "INVALID_SEATS_TOTAL", err.Error())
	case errors.Is(err, ErrAlreadyAssigned):
		return response.Conflict(c, "ALREADY_ASSIGNED", err.Error())
	case errors.Is(err, ErrNotAssigned):
		return response.Conflict(c, "NOT_ASSIGNED", err.Error())
	case errors.Is(err, ErrNoSeatsLeft):
		return response.Conflict(c, "NO_SEATS_LEFT", err.Error())
	case errors.Is(err, ErrSeatAlreadyHeld):
		return response.Conflict(c, "SEAT_ALREADY_HELD", err.Error())
	case errors.Is(err, ErrWrongStatus):
		return response.Conflict(c, "WRONG_STATUS", err.Error())
	case errors.Is(err, ErrAccessDenied):
		return response.Forbidden(c, "RECORD_ACCESS_DENIED", "You do not have access to this record")
	default:
		log.Error("assets: error", slog.Any("error", err))
		return response.InternalServerError(c)
	}
}

// resolveFilter builds a scope-aware ListFilter. hrm.assets' tiers govern the
// ASSIGNMENT's employee_id — see migration 00107's header on the asymmetry.
func (h *Handler) resolveFilter(c fiber.Ctx) (ListFilter, string, error) {
	userID, ok := requestUser(c)
	if !ok {
		return ListFilter{}, "", errors.New("unauthenticated")
	}
	orgID, ok := requestOrg(c)
	if !ok {
		return ListFilter{}, "", errors.New("no org context")
	}
	tier, err := h.authz.ResolveScope(c.Context(), userID, orgID, "hrm.assets")
	if err != nil {
		return ListFilter{}, "", err
	}
	f := ListFilter{
		EmployeeID: c.Query("employee_id"),
		CategoryID: c.Query("category_id"),
		Status:     c.Query("status"),
		Scope:      tier, CallerUserID: userID,
	}
	if limit, err := strconv.Atoi(c.Query("limit", "")); err == nil {
		f.Limit = limit
	}
	if offset, err := strconv.Atoi(c.Query("offset", "")); err == nil {
		f.Offset = offset
	}
	return f, orgID, nil
}

// ── Categories ───────────────────────────────────────────────────────────────

func (h *Handler) ListCategories(c fiber.Ctx) error {
	orgID, ok := requestOrg(c)
	if !ok {
		return response.BadRequest(c, "NO_ORGANIZATION_CONTEXT", "Organization context is required")
	}
	list, err := h.service.ListCategories(c.Context(), orgID)
	if err != nil {
		return h.err(c, err)
	}
	return response.OK(c, fiber.Map{"categories": list}, "OK")
}

func (h *Handler) CreateCategory(c fiber.Ctx) error {
	userID, ok := requestUser(c)
	if !ok {
		return response.Unauthorized(c, "UNAUTHORIZED", "Authentication required")
	}
	orgID, ok := requestOrg(c)
	if !ok {
		return response.BadRequest(c, "NO_ORGANIZATION_CONTEXT", "Organization context is required")
	}
	var req CreateCategoryRequest
	if err := c.Bind().JSON(&req); err != nil {
		return response.BadRequest(c, "INVALID_BODY", "Invalid request body")
	}
	cat, err := h.service.CreateCategory(c.Context(), orgID, userID, req)
	if err != nil {
		return h.err(c, err)
	}
	return response.Created(c, fiber.Map{"category": cat}, "Asset category created")
}

// ── Licences ─────────────────────────────────────────────────────────────────

func (h *Handler) ListLicenses(c fiber.Ctx) error {
	orgID, ok := requestOrg(c)
	if !ok {
		return response.BadRequest(c, "NO_ORGANIZATION_CONTEXT", "Organization context is required")
	}
	list, err := h.service.ListLicenses(c.Context(), orgID)
	if err != nil {
		return h.err(c, err)
	}
	return response.OK(c, fiber.Map{"licenses": list}, "OK")
}

func (h *Handler) GetLicense(c fiber.Ctx) error {
	orgID, ok := requestOrg(c)
	if !ok {
		return response.BadRequest(c, "NO_ORGANIZATION_CONTEXT", "Organization context is required")
	}
	l, err := h.service.GetLicense(c.Context(), orgID, c.Params("licenseId"))
	if err != nil {
		return h.err(c, err)
	}
	return response.OK(c, fiber.Map{"license": l}, "OK")
}

func (h *Handler) CreateLicense(c fiber.Ctx) error {
	userID, ok := requestUser(c)
	if !ok {
		return response.Unauthorized(c, "UNAUTHORIZED", "Authentication required")
	}
	orgID, ok := requestOrg(c)
	if !ok {
		return response.BadRequest(c, "NO_ORGANIZATION_CONTEXT", "Organization context is required")
	}
	var req CreateLicenseRequest
	if err := c.Bind().JSON(&req); err != nil {
		return response.BadRequest(c, "INVALID_BODY", "Invalid request body")
	}
	l, err := h.service.CreateLicense(c.Context(), orgID, userID, req)
	if err != nil {
		return h.err(c, err)
	}
	return response.Created(c, fiber.Map{"license": l}, "Software licence created")
}

func (h *Handler) ListSeats(c fiber.Ctx) error {
	orgID, ok := requestOrg(c)
	if !ok {
		return response.BadRequest(c, "NO_ORGANIZATION_CONTEXT", "Organization context is required")
	}
	list, err := h.service.ListSeats(c.Context(), orgID, c.Params("licenseId"))
	if err != nil {
		return h.err(c, err)
	}
	return response.OK(c, fiber.Map{"seats": list}, "OK")
}

func (h *Handler) AssignSeat(c fiber.Ctx) error {
	userID, ok := requestUser(c)
	if !ok {
		return response.Unauthorized(c, "UNAUTHORIZED", "Authentication required")
	}
	orgID, ok := requestOrg(c)
	if !ok {
		return response.BadRequest(c, "NO_ORGANIZATION_CONTEXT", "Organization context is required")
	}
	var req AssignSeatRequest
	if err := c.Bind().JSON(&req); err != nil {
		return response.BadRequest(c, "INVALID_BODY", "Invalid request body")
	}
	seat, err := h.service.AssignSeat(c.Context(), orgID, c.Params("licenseId"), userID, req)
	if err != nil {
		return h.err(c, err)
	}
	return response.Created(c, fiber.Map{"seat": seat}, "Licence seat assigned")
}

func (h *Handler) ReleaseSeat(c fiber.Ctx) error {
	orgID, ok := requestOrg(c)
	if !ok {
		return response.BadRequest(c, "NO_ORGANIZATION_CONTEXT", "Organization context is required")
	}
	if err := h.service.ReleaseSeat(c.Context(), orgID, c.Params("licenseId"), c.Params("employeeId")); err != nil {
		return h.err(c, err)
	}
	return response.OK(c, fiber.Map{}, "Licence seat released")
}

// ── Assets ───────────────────────────────────────────────────────────────────

func (h *Handler) ListAssets(c fiber.Ctx) error {
	log := logger.FromCtx(c)
	filter, orgID, err := h.resolveFilter(c)
	if err != nil {
		log.Error("assets: ListAssets", slog.Any("error", err))
		return response.InternalServerError(c)
	}
	res, err := h.service.ListAssets(c.Context(), orgID, filter)
	if err != nil {
		return h.err(c, err)
	}
	return response.OK(c, res, "OK")
}

func (h *Handler) GetAsset(c fiber.Ctx) error {
	log := logger.FromCtx(c)
	userID, ok := requestUser(c)
	if !ok {
		return response.Unauthorized(c, "UNAUTHORIZED", "Authentication required")
	}
	orgID, ok := requestOrg(c)
	if !ok {
		return response.BadRequest(c, "NO_ORGANIZATION_CONTEXT", "Organization context is required")
	}
	a, err := h.service.GetAsset(c.Context(), orgID, c.Params("assetId"))
	if err != nil {
		return h.err(c, err)
	}
	// An UNASSIGNED asset has no employee to scope against — it is org
	// inventory, visible to anyone holding hrm.assets.view. Only an assigned
	// one is narrowed to its holder's scope.
	if a.CurrentHolderEmployeeID != nil {
		tier, err := h.authz.ResolveScope(c.Context(), userID, orgID, "hrm.assets")
		if err != nil {
			log.Error("assets: GetAsset", slog.Any("error", err))
			return response.InternalServerError(c)
		}
		allowed, err := h.scopeResolver.AuthorizeRecordAccess(c.Context(), tier, orgID, userID, *a.CurrentHolderEmployeeID)
		if err != nil {
			log.Error("assets: GetAsset", slog.Any("error", err))
			return response.InternalServerError(c)
		}
		if !allowed {
			return h.err(c, ErrAccessDenied)
		}
	}
	return response.OK(c, fiber.Map{"asset": a}, "OK")
}

func (h *Handler) CreateAsset(c fiber.Ctx) error {
	userID, ok := requestUser(c)
	if !ok {
		return response.Unauthorized(c, "UNAUTHORIZED", "Authentication required")
	}
	orgID, ok := requestOrg(c)
	if !ok {
		return response.BadRequest(c, "NO_ORGANIZATION_CONTEXT", "Organization context is required")
	}
	var req CreateAssetRequest
	if err := c.Bind().JSON(&req); err != nil {
		return response.BadRequest(c, "INVALID_BODY", "Invalid request body")
	}
	a, err := h.service.CreateAsset(c.Context(), orgID, userID, req)
	if err != nil {
		return h.err(c, err)
	}
	return response.Created(c, fiber.Map{"asset": a}, "Asset created")
}

func (h *Handler) AssignAsset(c fiber.Ctx) error {
	userID, ok := requestUser(c)
	if !ok {
		return response.Unauthorized(c, "UNAUTHORIZED", "Authentication required")
	}
	orgID, ok := requestOrg(c)
	if !ok {
		return response.BadRequest(c, "NO_ORGANIZATION_CONTEXT", "Organization context is required")
	}
	var req AssignAssetRequest
	if err := c.Bind().JSON(&req); err != nil {
		return response.BadRequest(c, "INVALID_BODY", "Invalid request body")
	}
	asgn, err := h.service.AssignAsset(c.Context(), orgID, c.Params("assetId"), userID, req)
	if err != nil {
		return h.err(c, err)
	}
	return response.Created(c, fiber.Map{"assignment": asgn}, "Asset assigned")
}

func (h *Handler) ReturnAsset(c fiber.Ctx) error {
	userID, ok := requestUser(c)
	if !ok {
		return response.Unauthorized(c, "UNAUTHORIZED", "Authentication required")
	}
	orgID, ok := requestOrg(c)
	if !ok {
		return response.BadRequest(c, "NO_ORGANIZATION_CONTEXT", "Organization context is required")
	}
	var req ReturnAssetRequest
	if err := c.Bind().JSON(&req); err != nil {
		return response.BadRequest(c, "INVALID_BODY", "Invalid request body")
	}
	asgn, err := h.service.ReturnAsset(c.Context(), orgID, c.Params("assetId"), userID, req)
	if err != nil {
		return h.err(c, err)
	}
	return response.OK(c, fiber.Map{"assignment": asgn}, "Asset returned")
}

func (h *Handler) ListAssignments(c fiber.Ctx) error {
	log := logger.FromCtx(c)
	filter, orgID, err := h.resolveFilter(c)
	if err != nil {
		log.Error("assets: ListAssignments", slog.Any("error", err))
		return response.InternalServerError(c)
	}
	res, err := h.service.ListAssignments(c.Context(), orgID, filter)
	if err != nil {
		return h.err(c, err)
	}
	return response.OK(c, res, "OK")
}

// ── Maintenance ──────────────────────────────────────────────────────────────

func (h *Handler) ListMaintenance(c fiber.Ctx) error {
	orgID, ok := requestOrg(c)
	if !ok {
		return response.BadRequest(c, "NO_ORGANIZATION_CONTEXT", "Organization context is required")
	}
	list, err := h.service.ListMaintenance(c.Context(), orgID, c.Params("assetId"))
	if err != nil {
		return h.err(c, err)
	}
	return response.OK(c, fiber.Map{"maintenance": list}, "OK")
}

func (h *Handler) AddMaintenance(c fiber.Ctx) error {
	userID, ok := requestUser(c)
	if !ok {
		return response.Unauthorized(c, "UNAUTHORIZED", "Authentication required")
	}
	orgID, ok := requestOrg(c)
	if !ok {
		return response.BadRequest(c, "NO_ORGANIZATION_CONTEXT", "Organization context is required")
	}
	var req CreateMaintenanceRequest
	if err := c.Bind().JSON(&req); err != nil {
		return response.BadRequest(c, "INVALID_BODY", "Invalid request body")
	}
	m, err := h.service.AddMaintenance(c.Context(), orgID, c.Params("assetId"), userID, req)
	if err != nil {
		return h.err(c, err)
	}
	return response.Created(c, fiber.Map{"maintenance": m}, "Maintenance recorded")
}

// ── Requests ─────────────────────────────────────────────────────────────────

func (h *Handler) ListRequests(c fiber.Ctx) error {
	log := logger.FromCtx(c)
	filter, orgID, err := h.resolveFilter(c)
	if err != nil {
		log.Error("assets: ListRequests", slog.Any("error", err))
		return response.InternalServerError(c)
	}
	res, err := h.service.ListRequests(c.Context(), orgID, filter)
	if err != nil {
		return h.err(c, err)
	}
	return response.OK(c, res, "OK")
}

func (h *Handler) GetRequest(c fiber.Ctx) error {
	log := logger.FromCtx(c)
	userID, ok := requestUser(c)
	if !ok {
		return response.Unauthorized(c, "UNAUTHORIZED", "Authentication required")
	}
	orgID, ok := requestOrg(c)
	if !ok {
		return response.BadRequest(c, "NO_ORGANIZATION_CONTEXT", "Organization context is required")
	}
	rq, err := h.service.GetRequest(c.Context(), orgID, c.Params("requestId"))
	if err != nil {
		return h.err(c, err)
	}
	tier, err := h.authz.ResolveScope(c.Context(), userID, orgID, "hrm.assets")
	if err != nil {
		log.Error("assets: GetRequest", slog.Any("error", err))
		return response.InternalServerError(c)
	}
	allowed, err := h.scopeResolver.AuthorizeRecordAccess(c.Context(), tier, orgID, userID, rq.EmployeeID)
	if err != nil {
		log.Error("assets: GetRequest", slog.Any("error", err))
		return response.InternalServerError(c)
	}
	if !allowed {
		return h.err(c, ErrAccessDenied)
	}
	return response.OK(c, fiber.Map{"request": rq}, "OK")
}

// RequestAsset is self-service: the SERVICE resolves the caller's own
// employeeID, so hrm.assets.request cannot raise a request for someone else.
func (h *Handler) RequestAsset(c fiber.Ctx) error {
	userID, ok := requestUser(c)
	if !ok {
		return response.Unauthorized(c, "UNAUTHORIZED", "Authentication required")
	}
	orgID, ok := requestOrg(c)
	if !ok {
		return response.BadRequest(c, "NO_ORGANIZATION_CONTEXT", "Organization context is required")
	}
	var req CreateAssetRequestRequest
	if err := c.Bind().JSON(&req); err != nil {
		return response.BadRequest(c, "INVALID_BODY", "Invalid request body")
	}
	rq, err := h.service.RequestAsset(c.Context(), orgID, userID, req)
	if err != nil {
		return h.err(c, err)
	}
	return response.Created(c, fiber.Map{"request": rq}, "Asset request created")
}

func (h *Handler) SubmitRequest(c fiber.Ctx) error {
	userID, ok := requestUser(c)
	if !ok {
		return response.Unauthorized(c, "UNAUTHORIZED", "Authentication required")
	}
	orgID, ok := requestOrg(c)
	if !ok {
		return response.BadRequest(c, "NO_ORGANIZATION_CONTEXT", "Organization context is required")
	}
	rq, err := h.service.SubmitRequest(c.Context(), orgID, c.Params("requestId"), userID)
	if err != nil {
		return h.err(c, err)
	}
	return response.OK(c, fiber.Map{"request": rq}, "Asset request submitted")
}

func (h *Handler) FulfillRequest(c fiber.Ctx) error {
	userID, ok := requestUser(c)
	if !ok {
		return response.Unauthorized(c, "UNAUTHORIZED", "Authentication required")
	}
	orgID, ok := requestOrg(c)
	if !ok {
		return response.BadRequest(c, "NO_ORGANIZATION_CONTEXT", "Organization context is required")
	}
	var req FulfillRequestRequest
	if err := c.Bind().JSON(&req); err != nil {
		return response.BadRequest(c, "INVALID_BODY", "Invalid request body")
	}
	rq, err := h.service.FulfillRequest(c.Context(), orgID, c.Params("requestId"), userID, req)
	if err != nil {
		return h.err(c, err)
	}
	return response.OK(c, fiber.Map{"request": rq}, "Asset request fulfilled")
}
