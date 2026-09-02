// backend/internal/hrm/performance/handler.go
package performance

import (
	"errors"
	"log/slog"

	"github.com/gofiber/fiber/v3"

	"github.com/mridha/businesssaas/internal/authz"
	"github.com/mridha/businesssaas/internal/middleware"
	"github.com/mridha/businesssaas/pkg/logger"
	"github.com/mridha/businesssaas/pkg/response"
)

// Handler handles HRM performance HTTP endpoints.
//
// It holds authz.Service because this module resolves TWO authorization facts
// per request — the caller's scope tier and whether they hold
// hrm.goals.manage — and hands both to the service on a Caller value. That
// keeps the service free of an authz dependency and testable against stubs.
type Handler struct {
	service Service
	authz   authz.Service
}

func NewHandler(service Service, authzSvc authz.Service) *Handler {
	return &Handler{service: service, authz: authzSvc}
}

// errUnauthenticated is returned by the context helpers when the request
// carries no user, so each handler maps it once rather than repeating the
// check inline.
var errUnauthenticated = errors.New("authentication required")

func requestOrg(c fiber.Ctx) (string, bool) {
	return middleware.OrganizationIDFromCtx(c)
}

// resolveCaller assembles the Caller the service needs. hrm.goals.manage is
// checked here rather than at the route because the route gate cannot express
// "is this your own goal" — the service does that narrowing, and needs to know
// whether the caller may write other people's goals at all.
func (h *Handler) resolveCaller(c fiber.Ctx, orgID string) (Caller, error) {
	userID, ok := middleware.UserIDFromCtx(c)
	if !ok {
		return Caller{}, errUnauthenticated
	}
	tier, err := h.authz.ResolveScope(c.Context(), userID, orgID, "hrm.goals")
	if err != nil {
		return Caller{}, err
	}
	canManage, err := h.authz.Can(c.Context(), userID, orgID, "hrm.goals", "manage")
	if err != nil {
		return Caller{}, err
	}
	return Caller{UserID: userID, Tier: tier, CanManage: canManage}, nil
}

// err maps every sentinel this package raises onto an HTTP response. Anything
// unmapped is logged and 500s rather than leaking an internal message.
func (h *Handler) err(c fiber.Ctx, err error) error {
	log := logger.FromCtx(c)
	switch {
	// ── Not found ────────────────────────────────────────────────────────
	case errors.Is(err, ErrCycleNotFound):
		return response.NotFound(c, "CYCLE_NOT_FOUND", "Goal cycle not found")
	case errors.Is(err, ErrGoalNotFound):
		return response.NotFound(c, "GOAL_NOT_FOUND", "Goal not found")
	case errors.Is(err, ErrEmployeeNotFound):
		return response.NotFound(c, "EMPLOYEE_NOT_FOUND", "Employee not found in this organization")
	case errors.Is(err, ErrScaleNotFound):
		return response.NotFound(c, "RATING_SCALE_NOT_FOUND", "Rating scale not found")
	case errors.Is(err, ErrLevelNotFound):
		return response.NotFound(c, "RATING_LEVEL_NOT_FOUND", "Rating level not found")
	case errors.Is(err, ErrAppraisalCycleNotFound):
		return response.NotFound(c, "APPRAISAL_CYCLE_NOT_FOUND", "Appraisal cycle not found")
	case errors.Is(err, ErrAppraisalNotFound):
		return response.NotFound(c, "APPRAISAL_NOT_FOUND", "Appraisal not found")

	// ── Forbidden ────────────────────────────────────────────────────────
	case errors.Is(err, errUnauthenticated):
		return response.Unauthorized(c, "UNAUTHORIZED", "Authentication required")
	case errors.Is(err, ErrGoalAccessDenied):
		return response.Forbidden(c, "GOAL_ACCESS_DENIED", "You do not have access to this goal")
	case errors.Is(err, ErrCallerHasNoEmployee):
		return response.Forbidden(c, "NO_EMPLOYEE_RECORD", "You have no employee record in this organization")
	case errors.Is(err, ErrAppraisalAccessDenied):
		return response.Forbidden(c, "APPRAISAL_ACCESS_DENIED", "You do not have access to this appraisal")

	// ── Conflict ─────────────────────────────────────────────────────────
	case errors.Is(err, ErrCycleNameTaken):
		return response.Conflict(c, "CYCLE_NAME_TAKEN", "A goal cycle with this name already exists")
	case errors.Is(err, ErrCycleWrongStatus):
		return response.Conflict(c, "CYCLE_WRONG_STATUS", "Action not allowed in the cycle's current status")
	case errors.Is(err, ErrCycleNotActive):
		return response.Conflict(c, "CYCLE_NOT_ACTIVE", "The goal cycle is not active — goal definitions cannot be changed")
	case errors.Is(err, ErrCycleWeightsIncomplete):
		return response.Conflict(c, "CYCLE_WEIGHTS_INCOMPLETE", "Some employees' goal weights do not total the cycle target — see the weight-audit endpoint")
	case errors.Is(err, ErrWeightExceedsCycleTarget):
		return response.Conflict(c, "WEIGHT_EXCEEDS_CYCLE_TARGET", "This would push the employee's total goal weight above the cycle target")
	case errors.Is(err, ErrGoalWrongStatus):
		return response.Conflict(c, "GOAL_WRONG_STATUS", "Action not allowed in the goal's current status")
	case errors.Is(err, ErrGoalHasHistory):
		return response.Conflict(c, "GOAL_HAS_HISTORY", "This goal has check-ins or aligned goals and cannot be deleted — cancel it instead")
	case errors.Is(err, ErrGoalAlignmentCycle):
		return response.Conflict(c, "GOAL_ALIGNMENT_CYCLE", "A goal cannot be aligned to itself or to one of its own descendants")
	case errors.Is(err, ErrCheckinGoalNotOpen):
		return response.Conflict(c, "GOAL_NOT_OPEN", "Cannot check in on a completed or cancelled goal")
	case errors.Is(err, ErrScaleNameTaken):
		return response.Conflict(c, "RATING_SCALE_NAME_TAKEN", "A rating scale with this name already exists")
	case errors.Is(err, ErrLevelLabelTaken):
		return response.Conflict(c, "RATING_LEVEL_LABEL_TAKEN", "A level with this label already exists on this scale")
	case errors.Is(err, ErrScaleInUse):
		return response.Conflict(c, "SCALE_IN_USE", "This rating scale is used by an appraisal cycle and cannot be deleted")
	case errors.Is(err, ErrScaleNoLevels):
		return response.Conflict(c, "SCALE_HAS_NO_LEVELS", "A rating scale needs at least one level before a cycle can use it")
	case errors.Is(err, ErrAppraisalCycleNameTaken):
		return response.Conflict(c, "APPRAISAL_CYCLE_NAME_TAKEN", "An appraisal cycle with this name already exists")
	case errors.Is(err, ErrAppraisalCycleStatus):
		return response.Conflict(c, "APPRAISAL_CYCLE_WRONG_STATUS", "Action not allowed in the cycle's current status")
	case errors.Is(err, ErrAppraisalCycleNotActive):
		return response.Conflict(c, "APPRAISAL_CYCLE_NOT_ACTIVE", "The appraisal cycle is not active")
	case errors.Is(err, ErrAppraisalExists):
		return response.Conflict(c, "APPRAISAL_EXISTS", "This employee already has an appraisal in this cycle")
	case errors.Is(err, ErrIllegalPhaseTransition):
		return response.Conflict(c, "ILLEGAL_PHASE_TRANSITION", "That phase transition is not allowed from the appraisal's current phase")
	case errors.Is(err, ErrAppraisalPublished):
		return response.Conflict(c, "APPRAISAL_PUBLISHED", "This appraisal has been published and can no longer be changed")
	case errors.Is(err, ErrNotInCalibration):
		return response.Conflict(c, "NOT_IN_CALIBRATION", "Ratings can only be calibrated during the calibration phase")
	case errors.Is(err, ErrRatingRequiredToPublish):
		return response.Conflict(c, "RATING_REQUIRED_TO_PUBLISH", "A final rating must be set before publishing")
	case errors.Is(err, ErrSelfReviewIncomplete):
		return response.Conflict(c, "SELF_REVIEW_INCOMPLETE", "The self-review must be submitted first")
	case errors.Is(err, ErrManagerReviewIncomplete):
		return response.Conflict(c, "MANAGER_REVIEW_INCOMPLETE", "The manager review must be submitted first")

	// ── Bad request ──────────────────────────────────────────────────────
	case errors.Is(err, ErrCycleNameRequired):
		return response.BadRequest(c, "CYCLE_NAME_REQUIRED", "name is required")
	case errors.Is(err, ErrCycleDateRequired):
		return response.BadRequest(c, "CYCLE_DATE_REQUIRED", "period_start and period_end are required (YYYY-MM-DD)")
	case errors.Is(err, ErrCycleInvalidDate):
		return response.BadRequest(c, "CYCLE_INVALID_DATE", "dates must be valid and in YYYY-MM-DD format")
	case errors.Is(err, ErrCyclePeriodInvalid):
		return response.BadRequest(c, "CYCLE_PERIOD_INVALID", "period_end must be on or after period_start")
	case errors.Is(err, ErrInvalidWeightTarget):
		return response.BadRequest(c, "INVALID_WEIGHT_TARGET", "weight_target must be greater than zero")
	case errors.Is(err, ErrGoalTitleRequired):
		return response.BadRequest(c, "GOAL_TITLE_REQUIRED", "title is required")
	case errors.Is(err, ErrGoalCycleRequired):
		return response.BadRequest(c, "GOAL_CYCLE_REQUIRED", "cycle_id is required")
	case errors.Is(err, ErrGoalInvalidLevel):
		return response.BadRequest(c, "GOAL_INVALID_LEVEL", "goal_level must be one of: individual, team, department, company")
	case errors.Is(err, ErrGoalInvalidMeasure):
		return response.BadRequest(c, "GOAL_INVALID_MEASUREMENT", "measurement_type must be one of: percentage, numeric, currency, boolean")
	case errors.Is(err, ErrGoalInvalidDir):
		return response.BadRequest(c, "GOAL_INVALID_DIRECTION", "direction must be one of: increase, decrease")
	case errors.Is(err, ErrGoalInvalidOutcome):
		return response.BadRequest(c, "GOAL_INVALID_OUTCOME", "outcome must be one of: exceeded, achieved, partially_achieved, missed")
	case errors.Is(err, ErrGoalInvalidWeight):
		return response.BadRequest(c, "GOAL_INVALID_WEIGHT", "weight must be between 0 and 100")
	case errors.Is(err, ErrGoalInvalidDate):
		return response.BadRequest(c, "GOAL_INVALID_DATE", "dates must be valid and in YYYY-MM-DD format")
	case errors.Is(err, ErrGoalDatesInvalid):
		return response.BadRequest(c, "GOAL_DATES_INVALID", "due_date must be on or after start_date")
	case errors.Is(err, ErrGoalTargetEqualsStart):
		return response.BadRequest(c, "GOAL_TARGET_EQUALS_START", "target_value must differ from start_value")
	case errors.Is(err, ErrGoalDirectionMismatch):
		return response.BadRequest(c, "GOAL_DIRECTION_MISMATCH", "target_value must be greater than start_value for an increase goal, and less for a decrease goal")
	case errors.Is(err, ErrScaleNameRequired):
		return response.BadRequest(c, "RATING_SCALE_NAME_REQUIRED", "name is required")
	case errors.Is(err, ErrLevelLabelReq):
		return response.BadRequest(c, "RATING_LEVEL_LABEL_REQUIRED", "label is required")
	case errors.Is(err, ErrAppraisalCycleNameReq):
		return response.BadRequest(c, "APPRAISAL_CYCLE_NAME_REQUIRED", "name is required")
	case errors.Is(err, ErrRatingScaleRequired):
		return response.BadRequest(c, "RATING_SCALE_REQUIRED", "rating_scale_id is required")
	case errors.Is(err, ErrFormTemplateRequired):
		return response.BadRequest(c, "FORM_TEMPLATE_REQUIRED", "At least one of self_form_template_id or manager_form_template_id is required")
	case errors.Is(err, ErrCalibrationNoteReq):
		return response.BadRequest(c, "CALIBRATION_NOTE_REQUIRED", "A calibration note is required — an unexplained rating override is not permitted")
	case errors.Is(err, ErrRatingLevelWrongScale):
		return response.BadRequest(c, "RATING_LEVEL_WRONG_SCALE", "That rating level does not belong to the cycle's rating scale")
	case errors.Is(err, ErrCheckinInvalidConfidence):
		return response.BadRequest(c, "INVALID_CONFIDENCE", "confidence must be one of: on_track, at_risk, off_track")

	default:
		log.Error("performance: error", slog.Any("error", err))
		return response.InternalServerError(c)
	}
}
