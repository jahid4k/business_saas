// backend/internal/hrm/fx/handler.go
package fx

import (
	"errors"
	"log/slog"
	"strconv"
	"strings"
	"time"

	"github.com/gofiber/fiber/v3"

	"github.com/mridha/businesssaas/internal/authz"
	"github.com/mridha/businesssaas/internal/middleware"
	"github.com/mridha/businesssaas/pkg/logger"
	"github.com/mridha/businesssaas/pkg/response"
	"github.com/shopspring/decimal"
)

// Handler serves the FX endpoints.
type Handler struct {
	service Service
	authz   authz.Service
}

func NewHandler(service Service, authzSvc authz.Service) *Handler {
	return &Handler{service: service, authz: authzSvc}
}

func orgID(c fiber.Ctx) string { return c.Params("orgId") }

var errUnauthenticated = errors.New("unauthenticated")

// caller resolves the manage authority.
// ⚠ Can takes the FULL dotted resource — authz builds its key as
// resource+"."+action, and a bare name denies everything silently (8C).
func (h *Handler) caller(c fiber.Ctx) (Caller, error) {
	userID, ok := middleware.UserIDFromCtx(c)
	if !ok {
		return Caller{}, errUnauthenticated
	}
	canManage, err := h.authz.Can(c.Context(), userID, orgID(c), "hrm.exchange_rates", "manage")
	if err != nil {
		return Caller{}, err
	}
	return Caller{UserID: userID, CanManage: canManage}, nil
}

func mapError(c fiber.Ctx, log *slog.Logger, op string, err error) error {
	switch {
	case errors.Is(err, ErrRateNotFound):
		return response.NotFound(c, "RATE_NOT_FOUND", "Exchange rate not found")
	case errors.Is(err, ErrDuplicateRate):
		return response.Conflict(c, "DUPLICATE_RATE",
			"A rate for this currency pair and date already exists; record a different date")
	case errors.Is(err, ErrSameCurrency):
		return response.BadRequest(c, "SAME_CURRENCY",
			"A currency cannot be converted to itself; no rate applies")
	case errors.Is(err, ErrInvalidRate):
		return response.BadRequest(c, "INVALID_RATE", "rate must be a number greater than zero")
	case errors.Is(err, ErrInvalidCurrency):
		return response.BadRequest(c, "INVALID_CURRENCY",
			"currency must be a three-letter ISO 4217 code, e.g. EUR")
	case errors.Is(err, ErrInvalidDate):
		return response.BadRequest(c, "INVALID_DATE", "rate_date must be YYYY-MM-DD")
	case errors.Is(err, ErrInvalidSource):
		return response.BadRequest(c, "INVALID_SOURCE", "source must be manual or import")
	case errors.Is(err, ErrAccessDenied):
		return response.Forbidden(c, "ACCESS_DENIED", "You do not have access to this resource")
	case errors.Is(err, errUnauthenticated):
		return response.Unauthorized(c, "UNAUTHORIZED", "Authentication required")
	default:
		log.Error("fx: "+op, slog.Any("error", err))
		return response.InternalServerError(c)
	}
}

// ListRates handles GET /organizations/:orgId/hrm/exchange-rates
func (h *Handler) ListRates(c fiber.Ctx) error {
	log := logger.FromCtx(c)
	var from, to *string
	if v := strings.TrimSpace(c.Query("from")); v != "" {
		from = &v
	}
	if v := strings.TrimSpace(c.Query("to")); v != "" {
		to = &v
	}
	limit := 0
	if v := strings.TrimSpace(c.Query("limit")); v != "" {
		limit, _ = strconv.Atoi(v)
	}
	list, err := h.service.ListRates(c.Context(), orgID(c), from, to, limit)
	if err != nil {
		return mapError(c, log, "ListRates", err)
	}
	return response.OK(c, fiber.Map{"exchange_rates": list}, "OK")
}

// RecordRate handles POST /organizations/:orgId/hrm/exchange-rates
func (h *Handler) RecordRate(c fiber.Ctx) error {
	log := logger.FromCtx(c)
	caller, err := h.caller(c)
	if err != nil {
		return mapError(c, log, "RecordRate", err)
	}
	var req RecordRateRequest
	if err := c.Bind().JSON(&req); err != nil {
		return response.BadRequest(c, "INVALID_BODY", "Invalid request body")
	}
	e, err := h.service.RecordRate(c.Context(), orgID(c), caller, req)
	if err != nil {
		return mapError(c, log, "RecordRate", err)
	}
	return response.Created(c, fiber.Map{"exchange_rate": e}, "Exchange rate recorded")
}

// Convert handles GET /organizations/:orgId/hrm/exchange-rates/convert
//
// A preview endpoint: it shows exactly what a conversion WOULD record,
// including the rate, its date, and whether it was inverted — so somebody can
// check a figure before a claim or settlement freezes it.
func (h *Handler) Convert(c fiber.Ctx) error {
	log := logger.FromCtx(c)
	amount, err := decimal.NewFromString(strings.TrimSpace(c.Query("amount")))
	if err != nil {
		return response.BadRequest(c, "INVALID_AMOUNT", "amount must be a number")
	}
	asOf := time.Now().Truncate(24 * time.Hour)
	if raw := strings.TrimSpace(c.Query("as_of")); raw != "" {
		d, err := time.Parse("2006-01-02", raw)
		if err != nil {
			return response.BadRequest(c, "INVALID_DATE", "as_of must be YYYY-MM-DD")
		}
		asOf = d
	}
	res, err := h.service.ConvertAsOf(c.Context(), orgID(c), amount,
		c.Query("from"), c.Query("to"), asOf)
	if err != nil {
		return mapError(c, log, "Convert", err)
	}
	// ⚠ 200 with available=false, not 404. "No rate exists for this pair on
	// this date" is an answer, and the caller has to render it as one rather
	// than treat it as a missing endpoint.
	return response.OK(c, fiber.Map{"result": res}, "OK")
}
