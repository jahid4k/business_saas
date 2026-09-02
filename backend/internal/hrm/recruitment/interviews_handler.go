// backend/internal/hrm/recruitment/interviews_handler.go
package recruitment

import (
	"github.com/gofiber/fiber/v3"

	"github.com/mridha/businesssaas/internal/middleware"
	"github.com/mridha/businesssaas/pkg/response"
)

// ListInterviews godoc
//
//	@Summary		List interviews for an application
//	@Tags			HRM / Recruitment
//	@Produce		json
//	@Security		BearerAuth
//	@Param			orgId			path	string	true	"Organization ID"
//	@Param			applicationId	path	string	true	"Application public ID"
//	@Success		200				{object}	response.OK{data=object{interviews=[]Interview}}
//	@Router			/organizations/{orgId}/hrm/recruitment/applications/{applicationId}/interviews [get]
func (h *Handler) ListInterviews(c fiber.Ctx) error {
	orgID, ok := middleware.OrganizationIDFromCtx(c)
	if !ok {
		return response.BadRequest(c, "NO_ORGANIZATION_CONTEXT", "Organization context is required")
	}
	list, err := h.service.ListInterviews(c.Context(), orgID, c.Params("applicationId"))
	if err != nil {
		return h.err(c, err)
	}
	return response.OK(c, fiber.Map{"interviews": list}, "OK")
}

// GetInterview godoc
//
//	@Summary		Get interview
//	@Tags			HRM / Recruitment
//	@Produce		json
//	@Security		BearerAuth
//	@Param			orgId		path	string	true	"Organization ID"
//	@Param			interviewId	path	string	true	"Interview public ID"
//	@Success		200			{object}	response.OK{data=object{interview=Interview}}
//	@Router			/organizations/{orgId}/hrm/recruitment/interviews/{interviewId} [get]
func (h *Handler) GetInterview(c fiber.Ctx) error {
	orgID, ok := middleware.OrganizationIDFromCtx(c)
	if !ok {
		return response.BadRequest(c, "NO_ORGANIZATION_CONTEXT", "Organization context is required")
	}
	i, err := h.service.GetInterview(c.Context(), orgID, c.Params("interviewId"))
	if err != nil {
		return h.err(c, err)
	}
	return response.OK(c, fiber.Map{"interview": i}, "OK")
}

// CreateInterview godoc
//
//	@Summary		Schedule an interview for an application
//	@Tags			HRM / Recruitment
//	@Accept			json
//	@Produce		json
//	@Security		BearerAuth
//	@Param			orgId			path	string					true	"Organization ID"
//	@Param			applicationId	path	string					true	"Application public ID"
//	@Param			body			body	CreateInterviewRequest	true	"Interview"
//	@Success		201				{object}	response.Created{data=object{interview=Interview}}
//	@Router			/organizations/{orgId}/hrm/recruitment/applications/{applicationId}/interviews [post]
func (h *Handler) CreateInterview(c fiber.Ctx) error {
	userID, ok := middleware.UserIDFromCtx(c)
	if !ok {
		return response.Unauthorized(c, "UNAUTHORIZED", "Authentication required")
	}
	orgID, ok := middleware.OrganizationIDFromCtx(c)
	if !ok {
		return response.BadRequest(c, "NO_ORGANIZATION_CONTEXT", "Organization context is required")
	}
	var req CreateInterviewRequest
	if err := c.Bind().JSON(&req); err != nil {
		return response.BadRequest(c, "INVALID_BODY", "Invalid request body")
	}
	i, err := h.service.CreateInterview(c.Context(), orgID, c.Params("applicationId"), userID, req)
	if err != nil {
		return h.err(c, err)
	}
	return response.Created(c, fiber.Map{"interview": i}, "Interview scheduled")
}

// UpdateInterview godoc
//
//	@Summary		Update an interview
//	@Tags			HRM / Recruitment
//	@Accept			json
//	@Produce		json
//	@Security		BearerAuth
//	@Param			orgId		path	string					true	"Organization ID"
//	@Param			interviewId	path	string					true	"Interview public ID"
//	@Param			body		body	UpdateInterviewRequest	true	"Fields to update"
//	@Success		200			{object}	response.OK{data=object{interview=Interview}}
//	@Router			/organizations/{orgId}/hrm/recruitment/interviews/{interviewId} [patch]
func (h *Handler) UpdateInterview(c fiber.Ctx) error {
	orgID, ok := middleware.OrganizationIDFromCtx(c)
	if !ok {
		return response.BadRequest(c, "NO_ORGANIZATION_CONTEXT", "Organization context is required")
	}
	var req UpdateInterviewRequest
	if err := c.Bind().JSON(&req); err != nil {
		return response.BadRequest(c, "INVALID_BODY", "Invalid request body")
	}
	i, err := h.service.UpdateInterview(c.Context(), orgID, c.Params("interviewId"), req)
	if err != nil {
		return h.err(c, err)
	}
	return response.OK(c, fiber.Map{"interview": i}, "Interview updated")
}

// DeleteInterview godoc
//
//	@Summary		Delete an interview
//	@Tags			HRM / Recruitment
//	@Produce		json
//	@Security		BearerAuth
//	@Param			orgId		path	string	true	"Organization ID"
//	@Param			interviewId	path	string	true	"Interview public ID"
//	@Success		204
//	@Router			/organizations/{orgId}/hrm/recruitment/interviews/{interviewId} [delete]
func (h *Handler) DeleteInterview(c fiber.Ctx) error {
	orgID, ok := middleware.OrganizationIDFromCtx(c)
	if !ok {
		return response.BadRequest(c, "NO_ORGANIZATION_CONTEXT", "Organization context is required")
	}
	if err := h.service.DeleteInterview(c.Context(), orgID, c.Params("interviewId")); err != nil {
		return h.err(c, err)
	}
	return response.NoContent(c)
}

// ListPanelists godoc
//
//	@Summary		List panelists for an interview
//	@Tags			HRM / Recruitment
//	@Produce		json
//	@Security		BearerAuth
//	@Param			orgId		path	string	true	"Organization ID"
//	@Param			interviewId	path	string	true	"Interview public ID"
//	@Success		200			{object}	response.OK{data=object{panelists=[]Panelist}}
//	@Router			/organizations/{orgId}/hrm/recruitment/interviews/{interviewId}/panelists [get]
func (h *Handler) ListPanelists(c fiber.Ctx) error {
	orgID, ok := middleware.OrganizationIDFromCtx(c)
	if !ok {
		return response.BadRequest(c, "NO_ORGANIZATION_CONTEXT", "Organization context is required")
	}
	list, err := h.service.ListPanelists(c.Context(), orgID, c.Params("interviewId"))
	if err != nil {
		return h.err(c, err)
	}
	return response.OK(c, fiber.Map{"panelists": list}, "OK")
}

// AddPanelist godoc
//
//	@Summary		Add a panelist to an interview
//	@Tags			HRM / Recruitment
//	@Accept			json
//	@Produce		json
//	@Security		BearerAuth
//	@Param			orgId		path	string				true	"Organization ID"
//	@Param			interviewId	path	string				true	"Interview public ID"
//	@Param			body		body	AddPanelistRequest	true	"Panelist"
//	@Success		201			{object}	response.Created{data=object{panelist=Panelist}}
//	@Router			/organizations/{orgId}/hrm/recruitment/interviews/{interviewId}/panelists [post]
func (h *Handler) AddPanelist(c fiber.Ctx) error {
	orgID, ok := middleware.OrganizationIDFromCtx(c)
	if !ok {
		return response.BadRequest(c, "NO_ORGANIZATION_CONTEXT", "Organization context is required")
	}
	var req AddPanelistRequest
	if err := c.Bind().JSON(&req); err != nil {
		return response.BadRequest(c, "INVALID_BODY", "Invalid request body")
	}
	p, err := h.service.AddPanelist(c.Context(), orgID, c.Params("interviewId"), req)
	if err != nil {
		return h.err(c, err)
	}
	return response.Created(c, fiber.Map{"panelist": p}, "Panelist added")
}

// RemovePanelist godoc
//
//	@Summary		Remove a panelist from an interview
//	@Tags			HRM / Recruitment
//	@Produce		json
//	@Security		BearerAuth
//	@Param			orgId		path	string	true	"Organization ID"
//	@Param			interviewId	path	string	true	"Interview public ID"
//	@Param			employeeId	path	string	true	"Employee ID"
//	@Success		204
//	@Router			/organizations/{orgId}/hrm/recruitment/interviews/{interviewId}/panelists/{employeeId} [delete]
func (h *Handler) RemovePanelist(c fiber.Ctx) error {
	orgID, ok := middleware.OrganizationIDFromCtx(c)
	if !ok {
		return response.BadRequest(c, "NO_ORGANIZATION_CONTEXT", "Organization context is required")
	}
	if err := h.service.RemovePanelist(c.Context(), orgID, c.Params("interviewId"), c.Params("employeeId")); err != nil {
		return h.err(c, err)
	}
	return response.NoContent(c)
}
