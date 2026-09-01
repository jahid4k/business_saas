// backend/internal/hrm/entities/handler.go
package entities

import (
	"errors"
	"log/slog"
	"strings"

	"github.com/gofiber/fiber/v3"

	"github.com/mridha/businesssaas/internal/authz"
	"github.com/mridha/businesssaas/internal/middleware"
	"github.com/mridha/businesssaas/pkg/logger"
	"github.com/mridha/businesssaas/pkg/response"
)

// Handler serves the legal-entity layer's HTTP endpoints.
type Handler struct {
	service Service
	authz   authz.Service
}

func NewHandler(service Service, authzSvc authz.Service) *Handler {
	return &Handler{service: service, authz: authzSvc}
}

func orgID(c fiber.Ctx) string { return c.Params("orgId") }

var errUnauthenticated = errors.New("unauthenticated")

// caller resolves the two manage authorities.
//
// ⚠ Can takes the FULL dotted resource — authz builds its key as
// resource+"."+action, and a bare name denies everything silently (8C).
func (h *Handler) caller(c fiber.Ctx) (Caller, error) {
	userID, ok := middleware.UserIDFromCtx(c)
	if !ok {
		return Caller{}, errUnauthenticated
	}
	org := orgID(c)
	manageEntities, err := h.authz.Can(c.Context(), userID, org, "hrm.entities", "manage")
	if err != nil {
		return Caller{}, err
	}
	manageLocations, err := h.authz.Can(c.Context(), userID, org, "hrm.locations", "manage")
	if err != nil {
		return Caller{}, err
	}
	return Caller{
		UserID: userID, CanManageEntities: manageEntities, CanManageLocations: manageLocations,
	}, nil
}

func mapError(c fiber.Ctx, log *slog.Logger, op string, err error) error {
	switch {
	case errors.Is(err, ErrEntityNotFound):
		return response.NotFound(c, "ENTITY_NOT_FOUND", "Legal entity not found in this organization")
	case errors.Is(err, ErrConfigNotFound):
		return response.NotFound(c, "CONFIG_NOT_FOUND", "Country configuration not found")
	case errors.Is(err, ErrLocationNotFound):
		return response.NotFound(c, "LOCATION_NOT_FOUND", "Location not found")
	case errors.Is(err, ErrNameRequired):
		return response.BadRequest(c, "NAME_REQUIRED", "A name is required")
	case errors.Is(err, ErrInvalidCountry):
		return response.BadRequest(c, "INVALID_COUNTRY",
			"country_code must be a two-letter ISO 3166-1 code, e.g. GB")
	case errors.Is(err, ErrInvalidCurrency):
		return response.BadRequest(c, "INVALID_CURRENCY",
			"currency must be a three-letter ISO 4217 code, e.g. GBP")
	case errors.Is(err, ErrInvalidCycle):
		return response.BadRequest(c, "INVALID_CYCLE",
			"payroll_cycle must be monthly, semi_monthly, biweekly or weekly")
	case errors.Is(err, ErrInvalidPayDay):
		return response.BadRequest(c, "INVALID_PAY_DAY", "pay_day_of_month must be between 1 and 31")
	case errors.Is(err, ErrInvalidMonth):
		return response.BadRequest(c, "INVALID_MONTH",
			"fiscal_year_start_month must be between 1 and 12")
	case errors.Is(err, ErrDuplicateCode):
		return response.Conflict(c, "DUPLICATE_CODE",
			"Another active location already uses this code")
	case errors.Is(err, ErrHeadquartersTaken):
		return response.Conflict(c, "HEADQUARTERS_TAKEN",
			"Another active location is already the headquarters")
	case errors.Is(err, ErrCannotUndefault):
		return response.Conflict(c, "CANNOT_UNDEFAULT",
			"Promote another entity to default instead; an organization with entities and no "+
				"default has nothing for its country and currency to fall back to")
	case errors.Is(err, ErrAccessDenied):
		return response.Forbidden(c, "ACCESS_DENIED", "You do not have access to this resource")
	case errors.Is(err, errUnauthenticated):
		return response.Unauthorized(c, "UNAUTHORIZED", "Authentication required")
	default:
		log.Error("entities: "+op, slog.Any("error", err))
		return response.InternalServerError(c)
	}
}

// ============================================================
// Legal entities
// ============================================================

// ListEntities handles GET /organizations/:orgId/hrm/legal-entities
func (h *Handler) ListEntities(c fiber.Ctx) error {
	log := logger.FromCtx(c)
	list, err := h.service.ListEntities(c.Context(), orgID(c), c.Query("active") != "false")
	if err != nil {
		return mapError(c, log, "ListEntities", err)
	}
	return response.OK(c, fiber.Map{"legal_entities": list}, "OK")
}

// CreateEntity handles POST /organizations/:orgId/hrm/legal-entities
func (h *Handler) CreateEntity(c fiber.Ctx) error {
	log := logger.FromCtx(c)
	caller, err := h.caller(c)
	if err != nil {
		return mapError(c, log, "CreateEntity", err)
	}
	var req CreateEntityRequest
	if err := c.Bind().JSON(&req); err != nil {
		return response.BadRequest(c, "INVALID_BODY", "Invalid request body")
	}
	e, err := h.service.CreateEntity(c.Context(), orgID(c), caller, req)
	if err != nil {
		return mapError(c, log, "CreateEntity", err)
	}
	return response.Created(c, fiber.Map{"legal_entity": e}, "Legal entity created")
}

// GetEntity handles GET /organizations/:orgId/hrm/legal-entities/:entityId
func (h *Handler) GetEntity(c fiber.Ctx) error {
	log := logger.FromCtx(c)
	e, err := h.service.GetEntity(c.Context(), orgID(c), c.Params("entityId"))
	if err != nil {
		return mapError(c, log, "GetEntity", err)
	}
	return response.OK(c, fiber.Map{"legal_entity": e}, "OK")
}

// UpdateEntity handles PATCH /organizations/:orgId/hrm/legal-entities/:entityId
func (h *Handler) UpdateEntity(c fiber.Ctx) error {
	log := logger.FromCtx(c)
	caller, err := h.caller(c)
	if err != nil {
		return mapError(c, log, "UpdateEntity", err)
	}
	var req UpdateEntityRequest
	if err := c.Bind().JSON(&req); err != nil {
		return response.BadRequest(c, "INVALID_BODY", "Invalid request body")
	}
	e, err := h.service.UpdateEntity(c.Context(), orgID(c), caller, c.Params("entityId"), req)
	if err != nil {
		return mapError(c, log, "UpdateEntity", err)
	}
	return response.OK(c, fiber.Map{"legal_entity": e}, "Legal entity updated")
}

// ResolveContext handles GET /organizations/:orgId/hrm/legal-entities/context
//
// This is the endpoint that shows what payroll, statutory resolution and the
// 11B currency layer will actually see — including which link of the chain
// each value came from.
func (h *Handler) ResolveContext(c fiber.Ctx) error {
	log := logger.FromCtx(c)
	var entityID *string
	if raw := strings.TrimSpace(c.Query("legal_entity_id")); raw != "" {
		entityID = &raw
	}
	ctxOut, err := h.service.ResolveContext(c.Context(), orgID(c), entityID)
	if err != nil {
		return mapError(c, log, "ResolveContext", err)
	}
	return response.OK(c, fiber.Map{"context": ctxOut}, "OK")
}

// ============================================================
// Country configs
// ============================================================

// ListCountryConfigs handles GET /organizations/:orgId/hrm/country-configs
func (h *Handler) ListCountryConfigs(c fiber.Ctx) error {
	log := logger.FromCtx(c)
	list, err := h.service.ListCountryConfigs(c.Context(), orgID(c), c.Query("active") != "false")
	if err != nil {
		return mapError(c, log, "ListCountryConfigs", err)
	}
	return response.OK(c, fiber.Map{"country_configs": list}, "OK")
}

// UpsertCountryConfig handles POST /organizations/:orgId/hrm/country-configs
func (h *Handler) UpsertCountryConfig(c fiber.Ctx) error {
	log := logger.FromCtx(c)
	caller, err := h.caller(c)
	if err != nil {
		return mapError(c, log, "UpsertCountryConfig", err)
	}
	var req CountryConfigRequest
	if err := c.Bind().JSON(&req); err != nil {
		return response.BadRequest(c, "INVALID_BODY", "Invalid request body")
	}
	cfg, err := h.service.UpsertCountryConfig(c.Context(), orgID(c), caller, req)
	if err != nil {
		return mapError(c, log, "UpsertCountryConfig", err)
	}
	return response.OK(c, fiber.Map{"country_config": cfg}, "Country configuration saved")
}

// ============================================================
// Locations
// ============================================================

// ListLocations handles GET /organizations/:orgId/hrm/locations
func (h *Handler) ListLocations(c fiber.Ctx) error {
	log := logger.FromCtx(c)
	var entityID *string
	if raw := strings.TrimSpace(c.Query("legal_entity_id")); raw != "" {
		entityID = &raw
	}
	list, err := h.service.ListLocations(c.Context(), orgID(c), entityID, c.Query("active") != "false")
	if err != nil {
		return mapError(c, log, "ListLocations", err)
	}
	return response.OK(c, fiber.Map{"locations": list}, "OK")
}

// CreateLocation handles POST /organizations/:orgId/hrm/locations
func (h *Handler) CreateLocation(c fiber.Ctx) error {
	log := logger.FromCtx(c)
	caller, err := h.caller(c)
	if err != nil {
		return mapError(c, log, "CreateLocation", err)
	}
	var req CreateLocationRequest
	if err := c.Bind().JSON(&req); err != nil {
		return response.BadRequest(c, "INVALID_BODY", "Invalid request body")
	}
	l, err := h.service.CreateLocation(c.Context(), orgID(c), caller, req)
	if err != nil {
		return mapError(c, log, "CreateLocation", err)
	}
	return response.Created(c, fiber.Map{"location": l}, "Location created")
}

// UpdateLocation handles PATCH /organizations/:orgId/hrm/locations/:locationId
func (h *Handler) UpdateLocation(c fiber.Ctx) error {
	log := logger.FromCtx(c)
	caller, err := h.caller(c)
	if err != nil {
		return mapError(c, log, "UpdateLocation", err)
	}
	var req UpdateLocationRequest
	if err := c.Bind().JSON(&req); err != nil {
		return response.BadRequest(c, "INVALID_BODY", "Invalid request body")
	}
	l, err := h.service.UpdateLocation(c.Context(), orgID(c), caller, c.Params("locationId"), req)
	if err != nil {
		return mapError(c, log, "UpdateLocation", err)
	}
	return response.OK(c, fiber.Map{"location": l}, "Location updated")
}
