// backend/internal/hrm/recruitment/scorecards_handler.go
package recruitment

import (
	"github.com/gofiber/fiber/v3"

	"github.com/mridha/businesssaas/internal/middleware"
	"github.com/mridha/businesssaas/pkg/response"
)

// ListScorecards godoc
//
//	@Summary		List scorecards for an interview
//	@Description	Visibility is narrowed by the service: a panelist who has
//	@Description	not yet submitted their own scorecard sees only their own
//	@Description	draft; everyone else sees every submitted scorecard.
//	@Tags			HRM / Recruitment
//	@Produce		json
//	@Security		BearerAuth
//	@Param			orgId		path	string	true	"Organization ID"
//	@Param			interviewId	path	string	true	"Interview public ID"
//	@Success		200			{object}	response.OK{data=object{scorecards=[]Scorecard}}
//	@Router			/organizations/{orgId}/hrm/recruitment/interviews/{interviewId}/scorecards [get]
func (h *Handler) ListScorecards(c fiber.Ctx) error {
	userID, ok := middleware.UserIDFromCtx(c)
	if !ok {
		return response.Unauthorized(c, "UNAUTHORIZED", "Authentication required")
	}
	orgID, ok := middleware.OrganizationIDFromCtx(c)
	if !ok {
		return response.BadRequest(c, "NO_ORGANIZATION_CONTEXT", "Organization context is required")
	}
	list, err := h.service.ListScorecards(c.Context(), orgID, c.Params("interviewId"), userID)
	if err != nil {
		return h.err(c, err)
	}
	return response.OK(c, fiber.Map{"scorecards": list}, "OK")
}

// UpsertOwnScorecard godoc
//
//	@Summary		Create or update the caller's own draft scorecard
//	@Tags			HRM / Recruitment
//	@Accept			json
//	@Produce		json
//	@Security		BearerAuth
//	@Param			orgId		path	string					true	"Organization ID"
//	@Param			interviewId	path	string					true	"Interview public ID"
//	@Param			body		body	UpsertScorecardRequest	true	"Scorecard"
//	@Success		200			{object}	response.OK{data=object{scorecard=Scorecard}}
//	@Failure		403			{object}	response.Error	"NOT_A_PANELIST"
//	@Failure		409			{object}	response.Error	"SCORECARD_ALREADY_SUBMITTED"
//	@Router			/organizations/{orgId}/hrm/recruitment/interviews/{interviewId}/scorecard [post]
func (h *Handler) UpsertOwnScorecard(c fiber.Ctx) error {
	userID, ok := middleware.UserIDFromCtx(c)
	if !ok {
		return response.Unauthorized(c, "UNAUTHORIZED", "Authentication required")
	}
	orgID, ok := middleware.OrganizationIDFromCtx(c)
	if !ok {
		return response.BadRequest(c, "NO_ORGANIZATION_CONTEXT", "Organization context is required")
	}
	var req UpsertScorecardRequest
	if err := c.Bind().JSON(&req); err != nil {
		return response.BadRequest(c, "INVALID_BODY", "Invalid request body")
	}
	sc, err := h.service.UpsertOwnScorecard(c.Context(), orgID, c.Params("interviewId"), userID, req)
	if err != nil {
		return h.err(c, err)
	}
	return response.OK(c, fiber.Map{"scorecard": sc}, "Scorecard saved")
}

// SubmitOwnScorecard godoc
//
//	@Summary		Submit the caller's own scorecard (locks it)
//	@Tags			HRM / Recruitment
//	@Produce		json
//	@Security		BearerAuth
//	@Param			orgId		path	string	true	"Organization ID"
//	@Param			interviewId	path	string	true	"Interview public ID"
//	@Success		200			{object}	response.OK{data=object{scorecard=Scorecard}}
//	@Failure		409			{object}	response.Error	"SCORECARD_ALREADY_SUBMITTED"
//	@Router			/organizations/{orgId}/hrm/recruitment/interviews/{interviewId}/scorecard/submit [post]
func (h *Handler) SubmitOwnScorecard(c fiber.Ctx) error {
	userID, ok := middleware.UserIDFromCtx(c)
	if !ok {
		return response.Unauthorized(c, "UNAUTHORIZED", "Authentication required")
	}
	orgID, ok := middleware.OrganizationIDFromCtx(c)
	if !ok {
		return response.BadRequest(c, "NO_ORGANIZATION_CONTEXT", "Organization context is required")
	}
	sc, err := h.service.SubmitOwnScorecard(c.Context(), orgID, c.Params("interviewId"), userID)
	if err != nil {
		return h.err(c, err)
	}
	return response.OK(c, fiber.Map{"scorecard": sc}, "Scorecard submitted")
}
