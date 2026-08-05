// backend/internal/hrm/recruitment/applications_handler.go
package recruitment

import (
	"github.com/gofiber/fiber/v3"

	"github.com/mridha/businesssaas/internal/middleware"
	"github.com/mridha/businesssaas/pkg/response"
)

// ListApplications godoc
//
//	@Summary		List applications
//	@Tags			HRM / Recruitment
//	@Produce		json
//	@Security		BearerAuth
//	@Param			orgId		path	string	true	"Organization ID"
//	@Param			candidate_id	query	string	false	"Filter by candidate"
//	@Param			posting_id	query	string	false	"Filter by posting"
//	@Param			status		query	string	false	"Filter by status"
//	@Success		200			{object}	response.OK{data=ApplicationListResponse}
//	@Router			/organizations/{orgId}/hrm/recruitment/applications [get]
func (h *Handler) ListApplications(c fiber.Ctx) error {
	orgID, ok := middleware.OrganizationIDFromCtx(c)
	if !ok {
		return response.BadRequest(c, "NO_ORGANIZATION_CONTEXT", "Organization context is required")
	}
	filter := ApplicationListFilter{
		CandidateID: c.Query("candidate_id"),
		PostingID:   c.Query("posting_id"),
		Status:      c.Query("status"),
	}
	res, err := h.service.ListApplications(c.Context(), orgID, filter)
	if err != nil {
		return h.err(c, err)
	}
	return response.OK(c, res, "OK")
}

// GetApplication godoc
//
//	@Summary		Get application
//	@Tags			HRM / Recruitment
//	@Produce		json
//	@Security		BearerAuth
//	@Param			orgId			path	string	true	"Organization ID"
//	@Param			applicationId	path	string	true	"Application public ID"
//	@Success		200				{object}	response.OK{data=object{application=Application}}
//	@Router			/organizations/{orgId}/hrm/recruitment/applications/{applicationId} [get]
func (h *Handler) GetApplication(c fiber.Ctx) error {
	orgID, ok := middleware.OrganizationIDFromCtx(c)
	if !ok {
		return response.BadRequest(c, "NO_ORGANIZATION_CONTEXT", "Organization context is required")
	}
	app, err := h.service.GetApplication(c.Context(), orgID, c.Params("applicationId"))
	if err != nil {
		return h.err(c, err)
	}
	return response.OK(c, fiber.Map{"application": app}, "OK")
}

// GetApplicationHistory godoc
//
//	@Summary		Get an application's stage movement history
//	@Tags			HRM / Recruitment
//	@Produce		json
//	@Security		BearerAuth
//	@Param			orgId			path	string	true	"Organization ID"
//	@Param			applicationId	path	string	true	"Application public ID"
//	@Success		200				{object}	response.OK{data=object{history=[]ApplicationStageHistory}}
//	@Router			/organizations/{orgId}/hrm/recruitment/applications/{applicationId}/history [get]
func (h *Handler) GetApplicationHistory(c fiber.Ctx) error {
	orgID, ok := middleware.OrganizationIDFromCtx(c)
	if !ok {
		return response.BadRequest(c, "NO_ORGANIZATION_CONTEXT", "Organization context is required")
	}
	list, err := h.service.GetStageHistory(c.Context(), orgID, c.Params("applicationId"))
	if err != nil {
		return h.err(c, err)
	}
	return response.OK(c, fiber.Map{"history": list}, "OK")
}

// CreateApplication godoc
//
//	@Summary		Create application (place a candidate on a posting's pipeline)
//	@Tags			HRM / Recruitment
//	@Accept			json
//	@Produce		json
//	@Security		BearerAuth
//	@Param			orgId	path	string						true	"Organization ID"
//	@Param			body	body	CreateApplicationRequest	true	"Application"
//	@Success		201		{object}	response.Created{data=object{application=Application}}
//	@Failure		409		{object}	response.Error	"DUPLICATE_APPLICATION"
//	@Router			/organizations/{orgId}/hrm/recruitment/applications [post]
func (h *Handler) CreateApplication(c fiber.Ctx) error {
	orgID, ok := middleware.OrganizationIDFromCtx(c)
	if !ok {
		return response.BadRequest(c, "NO_ORGANIZATION_CONTEXT", "Organization context is required")
	}
	userID, _ := middleware.UserIDFromCtx(c)
	var createdBy *string
	if userID != "" {
		createdBy = &userID
	}
	var req CreateApplicationRequest
	if err := c.Bind().JSON(&req); err != nil {
		return response.BadRequest(c, "INVALID_BODY", "Invalid request body")
	}
	app, err := h.service.CreateApplication(c.Context(), orgID, createdBy, req)
	if err != nil {
		return h.err(c, err)
	}
	return response.Created(c, fiber.Map{"application": app}, "Application created")
}

// MoveApplication godoc
//
//	@Summary		Move an application to a different stage
//	@Tags			HRM / Recruitment
//	@Accept			json
//	@Produce		json
//	@Security		BearerAuth
//	@Param			orgId			path	string					true	"Organization ID"
//	@Param			applicationId	path	string					true	"Application public ID"
//	@Param			body			body	MoveApplicationRequest	true	"Target stage"
//	@Success		200				{object}	response.OK{data=object{application=Application}}
//	@Failure		409				{object}	response.Error	"WRONG_STATUS or STAGE_NOT_IN_PIPELINE"
//	@Router			/organizations/{orgId}/hrm/recruitment/applications/{applicationId}/move [post]
func (h *Handler) MoveApplication(c fiber.Ctx) error {
	userID, ok := middleware.UserIDFromCtx(c)
	if !ok {
		return response.Unauthorized(c, "UNAUTHORIZED", "Authentication required")
	}
	orgID, ok := middleware.OrganizationIDFromCtx(c)
	if !ok {
		return response.BadRequest(c, "NO_ORGANIZATION_CONTEXT", "Organization context is required")
	}
	var req MoveApplicationRequest
	if err := c.Bind().JSON(&req); err != nil {
		return response.BadRequest(c, "INVALID_BODY", "Invalid request body")
	}
	app, err := h.service.MoveApplication(c.Context(), orgID, c.Params("applicationId"), userID, req)
	if err != nil {
		return h.err(c, err)
	}
	return response.OK(c, fiber.Map{"application": app}, "Application moved")
}

// RejectApplication godoc
//
//	@Summary		Reject an application
//	@Tags			HRM / Recruitment
//	@Accept			json
//	@Produce		json
//	@Security		BearerAuth
//	@Param			orgId			path	string						true	"Organization ID"
//	@Param			applicationId	path	string						true	"Application public ID"
//	@Param			body			body	RejectApplicationRequest	true	"Reason"
//	@Success		200				{object}	response.OK{data=object{application=Application}}
//	@Router			/organizations/{orgId}/hrm/recruitment/applications/{applicationId}/reject [post]
func (h *Handler) RejectApplication(c fiber.Ctx) error {
	userID, ok := middleware.UserIDFromCtx(c)
	if !ok {
		return response.Unauthorized(c, "UNAUTHORIZED", "Authentication required")
	}
	orgID, ok := middleware.OrganizationIDFromCtx(c)
	if !ok {
		return response.BadRequest(c, "NO_ORGANIZATION_CONTEXT", "Organization context is required")
	}
	var req RejectApplicationRequest
	if err := c.Bind().JSON(&req); err != nil {
		return response.BadRequest(c, "INVALID_BODY", "Invalid request body")
	}
	app, err := h.service.RejectApplication(c.Context(), orgID, c.Params("applicationId"), userID, req)
	if err != nil {
		return h.err(c, err)
	}
	return response.OK(c, fiber.Map{"application": app}, "Application rejected")
}

// WithdrawApplication godoc
//
//	@Summary		Withdraw an application
//	@Tags			HRM / Recruitment
//	@Produce		json
//	@Security		BearerAuth
//	@Param			orgId			path	string	true	"Organization ID"
//	@Param			applicationId	path	string	true	"Application public ID"
//	@Success		200				{object}	response.OK{data=object{application=Application}}
//	@Router			/organizations/{orgId}/hrm/recruitment/applications/{applicationId}/withdraw [post]
func (h *Handler) WithdrawApplication(c fiber.Ctx) error {
	userID, ok := middleware.UserIDFromCtx(c)
	if !ok {
		return response.Unauthorized(c, "UNAUTHORIZED", "Authentication required")
	}
	orgID, ok := middleware.OrganizationIDFromCtx(c)
	if !ok {
		return response.BadRequest(c, "NO_ORGANIZATION_CONTEXT", "Organization context is required")
	}
	app, err := h.service.WithdrawApplication(c.Context(), orgID, c.Params("applicationId"), userID)
	if err != nil {
		return h.err(c, err)
	}
	return response.OK(c, fiber.Map{"application": app}, "Application withdrawn")
}
