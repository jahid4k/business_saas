// backend/internal/hrm/compensation/handler.go
package compensation

import (
	"errors"
	"log/slog"

	"github.com/gofiber/fiber/v3"

	"github.com/mridha/businesssaas/internal/authz"
	"github.com/mridha/businesssaas/internal/hrm/scope"
	"github.com/mridha/businesssaas/internal/middleware"
	"github.com/mridha/businesssaas/pkg/logger"
	"github.com/mridha/businesssaas/pkg/response"
)

// Handler handles HRM compensation HTTP endpoints. Holds authz.Service and a
// scope.Resolver the same way payslips.Handler does — hrm.salary_revisions
// and hrm.bonuses are both scope-tiered, and single-record reads need
// AuthorizeRecordAccess the way payslips.GetPayslip does.
type Handler struct {
	service       Service
	authz         authz.Service
	scopeResolver *scope.Resolver
}

func NewHandler(service Service, authzSvc authz.Service, scopeResolver *scope.Resolver) *Handler {
	return &Handler{service: service, authz: authzSvc, scopeResolver: scopeResolver}
}

func requestUser(c fiber.Ctx) (string, bool) {
	return middleware.UserIDFromCtx(c)
}

func requestOrg(c fiber.Ctx) (string, bool) {
	return middleware.OrganizationIDFromCtx(c)
}

func (h *Handler) err(c fiber.Ctx, err error) error {
	log := logger.FromCtx(c)
	switch {
	case errors.Is(err, ErrBandNotFound):
		return response.NotFound(c, "BAND_NOT_FOUND", "Compensation band not found")
	case errors.Is(err, ErrMatrixCellNotFound):
		return response.NotFound(c, "MATRIX_CELL_NOT_FOUND", "Merit matrix cell not found")
	case errors.Is(err, ErrCycleNotFound):
		return response.NotFound(c, "CYCLE_NOT_FOUND", "Salary revision cycle not found")
	case errors.Is(err, ErrRevisionNotFound):
		return response.NotFound(c, "REVISION_NOT_FOUND", "Salary revision not found")
	case errors.Is(err, ErrBonusNotFound):
		return response.NotFound(c, "BONUS_NOT_FOUND", "Bonus not found")
	case errors.Is(err, ErrInvalidAmount):
		return response.BadRequest(c, "INVALID_AMOUNT", err.Error())
	case errors.Is(err, ErrInvalidBandRange):
		return response.BadRequest(c, "INVALID_BAND_RANGE", err.Error())
	case errors.Is(err, ErrInvalidBonusType):
		return response.BadRequest(c, "INVALID_BONUS_TYPE", err.Error())
	case errors.Is(err, ErrWrongCycleStatus):
		return response.Conflict(c, "WRONG_CYCLE_STATUS", err.Error())
	case errors.Is(err, ErrWrongBonusStatus):
		return response.Conflict(c, "WRONG_BONUS_STATUS", err.Error())
	case errors.Is(err, ErrCycleHasNoRevisions):
		return response.Conflict(c, "CYCLE_HAS_NO_REVISIONS", err.Error())
	case errors.Is(err, ErrAccessDenied):
		return response.Forbidden(c, "RECORD_ACCESS_DENIED", "You do not have access to this record")
	default:
		log.Error("compensation: error", slog.Any("error", err))
		return response.InternalServerError(c)
	}
}
