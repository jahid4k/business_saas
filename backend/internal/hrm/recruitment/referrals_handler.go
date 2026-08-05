// backend/internal/hrm/recruitment/referrals_handler.go
package recruitment

import (
	"github.com/gofiber/fiber/v3"

	"github.com/mridha/businesssaas/internal/middleware"
	"github.com/mridha/businesssaas/pkg/response"
)

// ListReferrals godoc
//
//	@Summary		List referrals
//	@Tags			HRM / Recruitment
//	@Produce		json
//	@Security		BearerAuth
//	@Param			orgId			path	string	true	"Organization ID"
//	@Param			candidate_id	query	string	false	"Filter by candidate"
//	@Param			status			query	string	false	"Filter by status"
//	@Success		200				{object}	response.OK{data=ReferralListResponse}
//	@Router			/organizations/{orgId}/hrm/recruitment/referrals [get]
func (h *Handler) ListReferrals(c fiber.Ctx) error {
	orgID, ok := middleware.OrganizationIDFromCtx(c)
	if !ok {
		return response.BadRequest(c, "NO_ORGANIZATION_CONTEXT", "Organization context is required")
	}
	filter := ReferralListFilter{CandidateID: c.Query("candidate_id"), Status: c.Query("status")}
	res, err := h.service.ListReferrals(c.Context(), orgID, filter)
	if err != nil {
		return h.err(c, err)
	}
	return response.OK(c, res, "OK")
}

// GetReferral godoc
//
//	@Summary		Get referral
//	@Tags			HRM / Recruitment
//	@Produce		json
//	@Security		BearerAuth
//	@Param			orgId		path	string	true	"Organization ID"
//	@Param			referralId	path	string	true	"Referral public ID"
//	@Success		200			{object}	response.OK{data=object{referral=Referral}}
//	@Router			/organizations/{orgId}/hrm/recruitment/referrals/{referralId} [get]
func (h *Handler) GetReferral(c fiber.Ctx) error {
	orgID, ok := middleware.OrganizationIDFromCtx(c)
	if !ok {
		return response.BadRequest(c, "NO_ORGANIZATION_CONTEXT", "Organization context is required")
	}
	rf, err := h.service.GetReferral(c.Context(), orgID, c.Params("referralId"))
	if err != nil {
		return h.err(c, err)
	}
	return response.OK(c, fiber.Map{"referral": rf}, "OK")
}

// CreateReferral godoc
//
//	@Summary		Create a referral
//	@Tags			HRM / Recruitment
//	@Accept			json
//	@Produce		json
//	@Security		BearerAuth
//	@Param			orgId	path	string					true	"Organization ID"
//	@Param			body	body	CreateReferralRequest	true	"Referral"
//	@Success		201		{object}	response.Created{data=object{referral=Referral}}
//	@Router			/organizations/{orgId}/hrm/recruitment/referrals [post]
func (h *Handler) CreateReferral(c fiber.Ctx) error {
	userID, ok := middleware.UserIDFromCtx(c)
	if !ok {
		return response.Unauthorized(c, "UNAUTHORIZED", "Authentication required")
	}
	orgID, ok := middleware.OrganizationIDFromCtx(c)
	if !ok {
		return response.BadRequest(c, "NO_ORGANIZATION_CONTEXT", "Organization context is required")
	}
	var req CreateReferralRequest
	if err := c.Bind().JSON(&req); err != nil {
		return response.BadRequest(c, "INVALID_BODY", "Invalid request body")
	}
	rf, err := h.service.CreateReferral(c.Context(), orgID, userID, req)
	if err != nil {
		return h.err(c, err)
	}
	return response.Created(c, fiber.Map{"referral": rf}, "Referral created")
}

// UpdateReferral godoc
//
//	@Summary		Update a referral (status, bonus, notes)
//	@Tags			HRM / Recruitment
//	@Accept			json
//	@Produce		json
//	@Security		BearerAuth
//	@Param			orgId		path	string					true	"Organization ID"
//	@Param			referralId	path	string					true	"Referral public ID"
//	@Param			body		body	UpdateReferralRequest	true	"Fields to update"
//	@Success		200			{object}	response.OK{data=object{referral=Referral}}
//	@Router			/organizations/{orgId}/hrm/recruitment/referrals/{referralId} [patch]
func (h *Handler) UpdateReferral(c fiber.Ctx) error {
	orgID, ok := middleware.OrganizationIDFromCtx(c)
	if !ok {
		return response.BadRequest(c, "NO_ORGANIZATION_CONTEXT", "Organization context is required")
	}
	var req UpdateReferralRequest
	if err := c.Bind().JSON(&req); err != nil {
		return response.BadRequest(c, "INVALID_BODY", "Invalid request body")
	}
	rf, err := h.service.UpdateReferral(c.Context(), orgID, c.Params("referralId"), req)
	if err != nil {
		return h.err(c, err)
	}
	return response.OK(c, fiber.Map{"referral": rf}, "Referral updated")
}
