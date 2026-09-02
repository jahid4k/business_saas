// backend/internal/hrm/analytics/handler.go
package analytics

import (
	"errors"
	"log/slog"
	"strings"
	"time"

	"github.com/gofiber/fiber/v3"

	"github.com/mridha/businesssaas/internal/authz"
	"github.com/mridha/businesssaas/internal/middleware"
	"github.com/mridha/businesssaas/pkg/logger"
	"github.com/mridha/businesssaas/pkg/response"
)

// Handler serves analytics' HTTP endpoints.
type Handler struct {
	service Service
	authz   authz.Service
}

func NewHandler(service Service, authzSvc authz.Service) *Handler {
	return &Handler{service: service, authz: authzSvc}
}

func orgID(c fiber.Ctx) string { return c.Params("orgId") }

var errUnauthenticated = errors.New("unauthenticated")

// caller resolves all five authorities once per request.
//
// ⚠ Can takes the FULL dotted resource — authz builds its key as
// resource+"."+action, and a bare name denies everything silently (8C).
//
// Each is resolved here as well as gated on the route so the service can
// re-check it: a route gate protects the HTTP path, and the service check is
// what protects the scheduler and any future report generator.
func (h *Handler) caller(c fiber.Ctx) (Caller, error) {
	userID, ok := middleware.UserIDFromCtx(c)
	if !ok {
		return Caller{}, errUnauthenticated
	}
	org := orgID(c)
	cl := Caller{UserID: userID}
	for _, spec := range []struct {
		action string
		into   *bool
	}{
		{"view", &cl.CanView},
		{"view_compensation", &cl.CanViewCompensation},
		{"view_dei", &cl.CanViewDEI},
		{"export", &cl.CanExport},
		{"manage", &cl.CanManage},
	} {
		granted, err := h.authz.Can(c.Context(), userID, org, "hrm.analytics", spec.action)
		if err != nil {
			return Caller{}, err
		}
		*spec.into = granted
	}
	return cl, nil
}

func mapError(c fiber.Ctx, log *slog.Logger, op string, err error) error {
	switch {
	case errors.Is(err, ErrMetricNotFound):
		return response.NotFound(c, "METRIC_NOT_FOUND", "Metric definition not found")
	case errors.Is(err, ErrDuplicateMetric):
		return response.Conflict(c, "DUPLICATE_METRIC", "A metric with this key already exists")
	case errors.Is(err, ErrInvalidComputation):
		return response.BadRequest(c, "INVALID_COMPUTATION",
			"computation must name a supported calculation; predictive scoring is not one")
	case errors.Is(err, ErrInvalidGrain):
		return response.BadRequest(c, "INVALID_GRAIN", "grain must be org, department or legal_entity")
	case errors.Is(err, ErrKeyRequired):
		return response.BadRequest(c, "KEY_REQUIRED", "A metric needs a key and a name")
	case errors.Is(err, ErrStatementRequired):
		return response.BadRequest(c, "STATEMENT_REQUIRED",
			"A metric definition must state its formula in words so a reader can check it")
	case errors.Is(err, ErrThresholdTooLow):
		return response.BadRequest(c, "THRESHOLD_TOO_LOW",
			"A suppression threshold below 2 discloses an individual")
	case errors.Is(err, ErrInvalidPeriod):
		return response.BadRequest(c, "INVALID_PERIOD", "from must not be after to")
	case errors.Is(err, ErrAccessDenied):
		return response.Forbidden(c, "ACCESS_DENIED", "You do not have access to this resource")
	case errors.Is(err, errUnauthenticated):
		return response.Unauthorized(c, "UNAUTHORIZED", "Authentication required")
	default:
		log.Error("analytics: "+op, slog.Any("error", err))
		return response.InternalServerError(c)
	}
}

// period reads from/to, defaulting to the trailing twelve months — the window
// every attrition figure is conventionally quoted over.
func period(c fiber.Ctx) (time.Time, time.Time, error) {
	to := time.Now().Truncate(24 * time.Hour)
	from := to.AddDate(-1, 0, 0)
	if raw := strings.TrimSpace(c.Query("to")); raw != "" {
		d, err := time.Parse("2006-01-02", raw)
		if err != nil {
			return from, to, err
		}
		to = d
	}
	if raw := strings.TrimSpace(c.Query("from")); raw != "" {
		d, err := time.Parse("2006-01-02", raw)
		if err != nil {
			return from, to, err
		}
		from = d
	}
	return from, to, nil
}

func grainOf(c fiber.Ctx) Grain {
	g := Grain(strings.TrimSpace(c.Query("grain")))
	if g == "" {
		return GrainOrg
	}
	return g
}

// ============================================================
// Metric definitions
// ============================================================

// ListMetrics handles GET /organizations/:orgId/hrm/analytics/metrics
func (h *Handler) ListMetrics(c fiber.Ctx) error {
	log := logger.FromCtx(c)
	caller, err := h.caller(c)
	if err != nil {
		return mapError(c, log, "ListMetrics", err)
	}
	list, err := h.service.ListMetrics(c.Context(), orgID(c), caller, c.Query("active") != "false")
	if err != nil {
		return mapError(c, log, "ListMetrics", err)
	}
	return response.OK(c, fiber.Map{"metrics": list}, "OK")
}

// CreateMetric handles POST /organizations/:orgId/hrm/analytics/metrics
func (h *Handler) CreateMetric(c fiber.Ctx) error {
	log := logger.FromCtx(c)
	caller, err := h.caller(c)
	if err != nil {
		return mapError(c, log, "CreateMetric", err)
	}
	var req CreateMetricRequest
	if err := c.Bind().JSON(&req); err != nil {
		return response.BadRequest(c, "INVALID_BODY", "Invalid request body")
	}
	m, err := h.service.CreateMetric(c.Context(), orgID(c), caller, req)
	if err != nil {
		return mapError(c, log, "CreateMetric", err)
	}
	return response.Created(c, fiber.Map{"metric": m}, "Metric definition created")
}

// UpdateMetric handles PATCH /organizations/:orgId/hrm/analytics/metrics/:metricId
func (h *Handler) UpdateMetric(c fiber.Ctx) error {
	log := logger.FromCtx(c)
	caller, err := h.caller(c)
	if err != nil {
		return mapError(c, log, "UpdateMetric", err)
	}
	var req UpdateMetricRequest
	if err := c.Bind().JSON(&req); err != nil {
		return response.BadRequest(c, "INVALID_BODY", "Invalid request body")
	}
	m, err := h.service.UpdateMetric(c.Context(), orgID(c), caller, c.Params("metricId"), req)
	if err != nil {
		return mapError(c, log, "UpdateMetric", err)
	}
	return response.OK(c, fiber.Map{"metric": m}, "Metric definition updated")
}

// ============================================================
// Read path
// ============================================================

// Headcount handles GET /organizations/:orgId/hrm/analytics/headcount
func (h *Handler) Headcount(c fiber.Ctx) error {
	log := logger.FromCtx(c)
	caller, err := h.caller(c)
	if err != nil {
		return mapError(c, log, "Headcount", err)
	}
	from, to, err := period(c)
	if err != nil {
		return response.BadRequest(c, "INVALID_PERIOD", "from and to must be YYYY-MM-DD")
	}
	snaps, err := h.service.Headcount(c.Context(), orgID(c), caller, from, to, grainOf(c))
	if err != nil {
		return mapError(c, log, "Headcount", err)
	}
	return response.OK(c, fiber.Map{"snapshots": snaps}, "OK")
}

// Attrition handles GET /organizations/:orgId/hrm/analytics/attrition
func (h *Handler) Attrition(c fiber.Ctx) error {
	log := logger.FromCtx(c)
	caller, err := h.caller(c)
	if err != nil {
		return mapError(c, log, "Attrition", err)
	}
	from, to, err := period(c)
	if err != nil {
		return response.BadRequest(c, "INVALID_PERIOD", "from and to must be YYYY-MM-DD")
	}
	sum, err := h.service.Attrition(c.Context(), orgID(c), caller, from, to)
	if err != nil {
		return mapError(c, log, "Attrition", err)
	}
	return response.OK(c, fiber.Map{"attrition": sum}, "OK")
}

// Cohorts handles GET /organizations/:orgId/hrm/analytics/cohorts
func (h *Handler) Cohorts(c fiber.Ctx) error {
	log := logger.FromCtx(c)
	caller, err := h.caller(c)
	if err != nil {
		return mapError(c, log, "Cohorts", err)
	}
	from, to, err := period(c)
	if err != nil {
		return response.BadRequest(c, "INVALID_PERIOD", "from and to must be YYYY-MM-DD")
	}
	rows, err := h.service.Cohorts(c.Context(), orgID(c), caller, from, to)
	if err != nil {
		return mapError(c, log, "Cohorts", err)
	}
	return response.OK(c, fiber.Map{"cohorts": rows}, "OK")
}

// Diversity handles GET /organizations/:orgId/hrm/analytics/diversity
func (h *Handler) Diversity(c fiber.Ctx) error {
	log := logger.FromCtx(c)
	caller, err := h.caller(c)
	if err != nil {
		return mapError(c, log, "Diversity", err)
	}
	from, to, err := period(c)
	if err != nil {
		return response.BadRequest(c, "INVALID_PERIOD", "from and to must be YYYY-MM-DD")
	}
	dists, err := h.service.Diversity(c.Context(), orgID(c), caller, from, to)
	if err != nil {
		return mapError(c, log, "Diversity", err)
	}
	return response.OK(c, fiber.Map{"distributions": dists}, "OK")
}

// Compensation handles GET /organizations/:orgId/hrm/analytics/compensation
func (h *Handler) Compensation(c fiber.Ctx) error {
	log := logger.FromCtx(c)
	caller, err := h.caller(c)
	if err != nil {
		return mapError(c, log, "Compensation", err)
	}
	on := time.Now().Truncate(24 * time.Hour)
	if raw := strings.TrimSpace(c.Query("on")); raw != "" {
		d, err := time.Parse("2006-01-02", raw)
		if err != nil {
			return response.BadRequest(c, "INVALID_DATE", "on must be YYYY-MM-DD")
		}
		on = d
	}
	bands, err := h.service.Compensation(c.Context(), orgID(c), caller, on, grainOf(c))
	if err != nil {
		return mapError(c, log, "Compensation", err)
	}
	return response.OK(c, fiber.Map{"bands": bands}, "OK")
}

// ExportAttrition handles GET /organizations/:orgId/hrm/analytics/export/attrition
func (h *Handler) ExportAttrition(c fiber.Ctx) error {
	log := logger.FromCtx(c)
	caller, err := h.caller(c)
	if err != nil {
		return mapError(c, log, "ExportAttrition", err)
	}
	from, to, err := period(c)
	if err != nil {
		return response.BadRequest(c, "INVALID_PERIOD", "from and to must be YYYY-MM-DD")
	}
	csv, err := h.service.ExportAttrition(c.Context(), orgID(c), caller, from, to)
	if err != nil {
		return mapError(c, log, "ExportAttrition", err)
	}
	c.Set("Content-Type", "text/csv; charset=utf-8")
	c.Set("Content-Disposition", `attachment; filename="attrition.csv"`)
	return c.SendString(csv)
}

// RunSnapshot handles POST /organizations/:orgId/hrm/analytics/snapshots/run
func (h *Handler) RunSnapshot(c fiber.Ctx) error {
	log := logger.FromCtx(c)
	caller, err := h.caller(c)
	if err != nil {
		return mapError(c, log, "RunSnapshot", err)
	}
	on := time.Now().Truncate(24 * time.Hour)
	if raw := strings.TrimSpace(c.Query("on")); raw != "" {
		d, err := time.Parse("2006-01-02", raw)
		if err != nil {
			return response.BadRequest(c, "INVALID_DATE", "on must be YYYY-MM-DD")
		}
		on = d
	}
	res, err := h.service.RunSnapshotForOrg(c.Context(), orgID(c), caller, on)
	if err != nil {
		return mapError(c, log, "RunSnapshot", err)
	}
	return response.OK(c, fiber.Map{"result": res}, "Snapshot built")
}
