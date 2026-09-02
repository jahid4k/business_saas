// backend/internal/hrm/recruitment/offers_handler.go
package recruitment

import (
	"github.com/gofiber/fiber/v3"

	"github.com/mridha/businesssaas/internal/middleware"
	"github.com/mridha/businesssaas/pkg/response"
)

// ListOffers godoc
//
//	@Summary		List offers for an application
//	@Tags			HRM / Recruitment
//	@Produce		json
//	@Security		BearerAuth
//	@Param			orgId			path	string	true	"Organization ID"
//	@Param			applicationId	path	string	true	"Application public ID"
//	@Success		200				{object}	response.OK{data=object{offers=[]Offer}}
//	@Router			/organizations/{orgId}/hrm/recruitment/applications/{applicationId}/offers [get]
func (h *Handler) ListOffers(c fiber.Ctx) error {
	orgID, ok := middleware.OrganizationIDFromCtx(c)
	if !ok {
		return response.BadRequest(c, "NO_ORGANIZATION_CONTEXT", "Organization context is required")
	}
	list, err := h.service.ListOffers(c.Context(), orgID, c.Params("applicationId"))
	if err != nil {
		return h.err(c, err)
	}
	return response.OK(c, fiber.Map{"offers": list}, "OK")
}

// GetOffer godoc
//
//	@Summary		Get offer
//	@Tags			HRM / Recruitment
//	@Produce		json
//	@Security		BearerAuth
//	@Param			orgId	path	string	true	"Organization ID"
//	@Param			offerId	path	string	true	"Offer public ID"
//	@Success		200		{object}	response.OK{data=object{offer=Offer}}
//	@Router			/organizations/{orgId}/hrm/recruitment/offers/{offerId} [get]
func (h *Handler) GetOffer(c fiber.Ctx) error {
	orgID, ok := middleware.OrganizationIDFromCtx(c)
	if !ok {
		return response.BadRequest(c, "NO_ORGANIZATION_CONTEXT", "Organization context is required")
	}
	o, err := h.service.GetOffer(c.Context(), orgID, c.Params("offerId"))
	if err != nil {
		return h.err(c, err)
	}
	return response.OK(c, fiber.Map{"offer": o}, "OK")
}

// CreateOffer godoc
//
//	@Summary		Create a compensation offer for an application
//	@Tags			HRM / Recruitment
//	@Accept			json
//	@Produce		json
//	@Security		BearerAuth
//	@Param			orgId			path	string				true	"Organization ID"
//	@Param			applicationId	path	string				true	"Application public ID"
//	@Param			body			body	CreateOfferRequest	true	"Offer"
//	@Success		201				{object}	response.Created{data=object{offer=Offer}}
//	@Router			/organizations/{orgId}/hrm/recruitment/applications/{applicationId}/offers [post]
func (h *Handler) CreateOffer(c fiber.Ctx) error {
	userID, ok := middleware.UserIDFromCtx(c)
	if !ok {
		return response.Unauthorized(c, "UNAUTHORIZED", "Authentication required")
	}
	orgID, ok := middleware.OrganizationIDFromCtx(c)
	if !ok {
		return response.BadRequest(c, "NO_ORGANIZATION_CONTEXT", "Organization context is required")
	}
	var req CreateOfferRequest
	if err := c.Bind().JSON(&req); err != nil {
		return response.BadRequest(c, "INVALID_BODY", "Invalid request body")
	}
	o, err := h.service.CreateOffer(c.Context(), orgID, c.Params("applicationId"), userID, req)
	if err != nil {
		return h.err(c, err)
	}
	return response.Created(c, fiber.Map{"offer": o}, "Offer created")
}

// UpdateOffer godoc
//
//	@Summary		Update an offer (draft only)
//	@Tags			HRM / Recruitment
//	@Accept			json
//	@Produce		json
//	@Security		BearerAuth
//	@Param			orgId	path	string				true	"Organization ID"
//	@Param			offerId	path	string				true	"Offer public ID"
//	@Param			body	body	UpdateOfferRequest	true	"Fields to update"
//	@Success		200		{object}	response.OK{data=object{offer=Offer}}
//	@Failure		409		{object}	response.Error	"OFFER_WRONG_STATUS"
//	@Router			/organizations/{orgId}/hrm/recruitment/offers/{offerId} [patch]
func (h *Handler) UpdateOffer(c fiber.Ctx) error {
	orgID, ok := middleware.OrganizationIDFromCtx(c)
	if !ok {
		return response.BadRequest(c, "NO_ORGANIZATION_CONTEXT", "Organization context is required")
	}
	var req UpdateOfferRequest
	if err := c.Bind().JSON(&req); err != nil {
		return response.BadRequest(c, "INVALID_BODY", "Invalid request body")
	}
	o, err := h.service.UpdateOffer(c.Context(), orgID, c.Params("offerId"), req)
	if err != nil {
		return h.err(c, err)
	}
	return response.OK(c, fiber.Map{"offer": o}, "Offer updated")
}

// SubmitOffer godoc
//
//	@Summary		Submit offer for approval
//	@Tags			HRM / Recruitment
//	@Produce		json
//	@Security		BearerAuth
//	@Param			orgId	path	string	true	"Organization ID"
//	@Param			offerId	path	string	true	"Offer public ID"
//	@Success		200		{object}	response.OK{data=object{offer=Offer}}
//	@Router			/organizations/{orgId}/hrm/recruitment/offers/{offerId}/submit [post]
func (h *Handler) SubmitOffer(c fiber.Ctx) error {
	userID, ok := middleware.UserIDFromCtx(c)
	if !ok {
		return response.Unauthorized(c, "UNAUTHORIZED", "Authentication required")
	}
	orgID, ok := middleware.OrganizationIDFromCtx(c)
	if !ok {
		return response.BadRequest(c, "NO_ORGANIZATION_CONTEXT", "Organization context is required")
	}
	o, err := h.service.SubmitOffer(c.Context(), orgID, c.Params("offerId"), userID)
	if err != nil {
		return h.err(c, err)
	}
	return response.OK(c, fiber.Map{"offer": o}, "Offer submitted")
}

// SendOffer godoc
//
//	@Summary		Mark an approved offer as sent to the candidate
//	@Tags			HRM / Recruitment
//	@Produce		json
//	@Security		BearerAuth
//	@Param			orgId	path	string	true	"Organization ID"
//	@Param			offerId	path	string	true	"Offer public ID"
//	@Success		200		{object}	response.OK{data=object{offer=Offer}}
//	@Router			/organizations/{orgId}/hrm/recruitment/offers/{offerId}/send [post]
func (h *Handler) SendOffer(c fiber.Ctx) error {
	orgID, ok := middleware.OrganizationIDFromCtx(c)
	if !ok {
		return response.BadRequest(c, "NO_ORGANIZATION_CONTEXT", "Organization context is required")
	}
	o, err := h.service.SendOffer(c.Context(), orgID, c.Params("offerId"))
	if err != nil {
		return h.err(c, err)
	}
	return response.OK(c, fiber.Map{"offer": o}, "Offer marked as sent")
}

// AcceptOffer godoc
//
//	@Summary		Record candidate acceptance of an offer
//	@Tags			HRM / Recruitment
//	@Produce		json
//	@Security		BearerAuth
//	@Param			orgId	path	string	true	"Organization ID"
//	@Param			offerId	path	string	true	"Offer public ID"
//	@Success		200		{object}	response.OK{data=object{offer=Offer}}
//	@Router			/organizations/{orgId}/hrm/recruitment/offers/{offerId}/accept [post]
func (h *Handler) AcceptOffer(c fiber.Ctx) error {
	orgID, ok := middleware.OrganizationIDFromCtx(c)
	if !ok {
		return response.BadRequest(c, "NO_ORGANIZATION_CONTEXT", "Organization context is required")
	}
	o, err := h.service.AcceptOffer(c.Context(), orgID, c.Params("offerId"))
	if err != nil {
		return h.err(c, err)
	}
	return response.OK(c, fiber.Map{"offer": o}, "Offer accepted")
}

// DeclineOffer godoc
//
//	@Summary		Record candidate decline of an offer
//	@Tags			HRM / Recruitment
//	@Produce		json
//	@Security		BearerAuth
//	@Param			orgId	path	string	true	"Organization ID"
//	@Param			offerId	path	string	true	"Offer public ID"
//	@Success		200		{object}	response.OK{data=object{offer=Offer}}
//	@Router			/organizations/{orgId}/hrm/recruitment/offers/{offerId}/decline [post]
func (h *Handler) DeclineOffer(c fiber.Ctx) error {
	orgID, ok := middleware.OrganizationIDFromCtx(c)
	if !ok {
		return response.BadRequest(c, "NO_ORGANIZATION_CONTEXT", "Organization context is required")
	}
	o, err := h.service.DeclineOffer(c.Context(), orgID, c.Params("offerId"))
	if err != nil {
		return h.err(c, err)
	}
	return response.OK(c, fiber.Map{"offer": o}, "Offer declined")
}

// RescindOffer godoc
//
//	@Summary		Rescind an offer
//	@Tags			HRM / Recruitment
//	@Produce		json
//	@Security		BearerAuth
//	@Param			orgId	path	string	true	"Organization ID"
//	@Param			offerId	path	string	true	"Offer public ID"
//	@Success		200		{object}	response.OK{data=object{offer=Offer}}
//	@Router			/organizations/{orgId}/hrm/recruitment/offers/{offerId}/rescind [post]
func (h *Handler) RescindOffer(c fiber.Ctx) error {
	orgID, ok := middleware.OrganizationIDFromCtx(c)
	if !ok {
		return response.BadRequest(c, "NO_ORGANIZATION_CONTEXT", "Organization context is required")
	}
	o, err := h.service.RescindOffer(c.Context(), orgID, c.Params("offerId"))
	if err != nil {
		return h.err(c, err)
	}
	return response.OK(c, fiber.Map{"offer": o}, "Offer rescinded")
}
