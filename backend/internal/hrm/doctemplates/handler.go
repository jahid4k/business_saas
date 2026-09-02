// backend/internal/hrm/doctemplates/handler.go
package doctemplates

import (
	"errors"
	"log/slog"
	"strings"

	"github.com/gofiber/fiber/v3"

	"github.com/mridha/businesssaas/internal/middleware"
	"github.com/mridha/businesssaas/pkg/logger"
	"github.com/mridha/businesssaas/pkg/response"
)

type Handler struct{ service Service }

func NewHandler(service Service) *Handler { return &Handler{service: service} }

// List godoc
//
//	@Summary		List document templates
//	@Description	Returns document template definitions. Filter by type with ?document_type=.
//	@Description
//	@Description	**Required permission:** `hrm.doc_templates.view`
//	@Tags			HRM / Document Templates
//	@Produce		json
//	@Security		BearerAuth
//	@Param			orgId			path		string	true	"Organization ID"
//	@Param			active			query		bool	false	"When true, return only active templates"
//	@Param			document_type	query		string	false	"Filter by document type (offer_letter|contract|warning_letter|...)"
//	@Success		200	{object}	response.OK{data=DocumentTemplateListResponse}
//	@Failure		401	{object}	response.Error
//	@Failure		403	{object}	response.Error
//	@Failure		500	{object}	response.Error
//	@Router			/organizations/{orgId}/hrm/setup/document-templates [get]
func (h *Handler) List(c fiber.Ctx) error {
	log := logger.FromCtx(c)
	orgID, ok := middleware.OrganizationIDFromCtx(c)
	if !ok {
		return response.BadRequest(c, "NO_ORGANIZATION_CONTEXT", "Organization context is required")
	}
	activeOnly := strings.ToLower(c.Query("active")) == "true"
	result, err := h.service.List(c.Context(), orgID, activeOnly, c.Query("document_type"))
	if err != nil {
		log.Error("doctemplates: List", slog.Any("error", err))
		return response.InternalServerError(c)
	}
	return response.OK(c, result, "OK")
}

// Create godoc
//
//	@Summary		Create document template
//	@Description	Creates a new Markdown document template with placeholder support.
//	@Description
//	@Description	Use `{{employee.first_name}}` style placeholders in body_markdown.
//	@Description	List the supported placeholders in available_variables for UI display.
//	@Description
//	@Description	**Required permission:** `hrm.doc_templates.manage`
//	@Tags			HRM / Document Templates
//	@Accept			json
//	@Produce		json
//	@Security		BearerAuth
//	@Param			orgId	path		string							true	"Organization ID"
//	@Param			body	body		CreateDocumentTemplateRequest	true	"Template definition"
//	@Success		201		{object}	response.Created{data=object{template=DocumentTemplate}}
//	@Failure		400		{object}	response.Error
//	@Failure		401		{object}	response.Error
//	@Failure		403		{object}	response.Error
//	@Failure		409		{object}	response.Error	"NAME_CONFLICT"
//	@Failure		500		{object}	response.Error
//	@Router			/organizations/{orgId}/hrm/setup/document-templates [post]
func (h *Handler) Create(c fiber.Ctx) error {
	userID, ok := middleware.UserIDFromCtx(c)
	if !ok {
		return response.Unauthorized(c, "UNAUTHORIZED", "Authentication required")
	}
	orgID, ok := middleware.OrganizationIDFromCtx(c)
	if !ok {
		return response.BadRequest(c, "NO_ORGANIZATION_CONTEXT", "Organization context is required")
	}
	var req CreateDocumentTemplateRequest
	if err := c.Bind().JSON(&req); err != nil {
		return response.BadRequest(c, "INVALID_BODY", "Invalid request body")
	}
	t, err := h.service.Create(c.Context(), orgID, userID, req)
	if err != nil {
		return h.tmplError(c, err)
	}
	return response.Created(c, fiber.Map{"template": t}, "Document template created")
}

// Get godoc
//
//	@Summary		Get document template
//	@Description	Returns a single document template by its public ID.
//	@Description
//	@Description	**Required permission:** `hrm.doc_templates.view`
//	@Tags			HRM / Document Templates
//	@Produce		json
//	@Security		BearerAuth
//	@Param			orgId		path		string	true	"Organization ID"
//	@Param			templateId	path		string	true	"Template public ID (dt_*)"
//	@Success		200			{object}	response.OK{data=object{template=DocumentTemplate}}
//	@Failure		401			{object}	response.Error
//	@Failure		403			{object}	response.Error
//	@Failure		404			{object}	response.Error	"TEMPLATE_NOT_FOUND"
//	@Failure		500			{object}	response.Error
//	@Router			/organizations/{orgId}/hrm/setup/document-templates/{templateId} [get]
func (h *Handler) Get(c fiber.Ctx) error {
	orgID, ok := middleware.OrganizationIDFromCtx(c)
	if !ok {
		return response.BadRequest(c, "NO_ORGANIZATION_CONTEXT", "Organization context is required")
	}
	t, err := h.service.Get(c.Context(), orgID, c.Params("templateId"))
	if err != nil {
		return h.tmplError(c, err)
	}
	return response.OK(c, fiber.Map{"template": t}, "OK")
}

// Update godoc
//
//	@Summary		Update document template
//	@Description	Partially updates a document template.
//	@Description
//	@Description	**Required permission:** `hrm.doc_templates.manage`
//	@Tags			HRM / Document Templates
//	@Accept			json
//	@Produce		json
//	@Security		BearerAuth
//	@Param			orgId		path		string							true	"Organization ID"
//	@Param			templateId	path		string							true	"Template public ID (dt_*)"
//	@Param			body		body		UpdateDocumentTemplateRequest	true	"Fields to update"
//	@Success		200			{object}	response.OK{data=object{template=DocumentTemplate}}
//	@Failure		400			{object}	response.Error
//	@Failure		401			{object}	response.Error
//	@Failure		403			{object}	response.Error
//	@Failure		404			{object}	response.Error
//	@Failure		409			{object}	response.Error
//	@Failure		500			{object}	response.Error
//	@Router			/organizations/{orgId}/hrm/setup/document-templates/{templateId} [patch]
func (h *Handler) Update(c fiber.Ctx) error {
	orgID, ok := middleware.OrganizationIDFromCtx(c)
	if !ok {
		return response.BadRequest(c, "NO_ORGANIZATION_CONTEXT", "Organization context is required")
	}
	var req UpdateDocumentTemplateRequest
	if err := c.Bind().JSON(&req); err != nil {
		return response.BadRequest(c, "INVALID_BODY", "Invalid request body")
	}
	t, err := h.service.Update(c.Context(), orgID, c.Params("templateId"), req)
	if err != nil {
		return h.tmplError(c, err)
	}
	return response.OK(c, fiber.Map{"template": t}, "Template updated")
}

// Delete godoc
//
//	@Summary		Delete document template
//	@Description	Permanently deletes a document template. Fails if employee documents reference it.
//	@Description	Deactivate instead (`is_active: false`) if the template has been used.
//	@Description
//	@Description	**Required permission:** `hrm.doc_templates.manage`
//	@Tags			HRM / Document Templates
//	@Produce		json
//	@Security		BearerAuth
//	@Param			orgId		path	string	true	"Organization ID"
//	@Param			templateId	path	string	true	"Template public ID (dt_*)"
//	@Success		204			"Template deleted"
//	@Failure		401			{object}	response.Error
//	@Failure		403			{object}	response.Error
//	@Failure		404			{object}	response.Error
//	@Failure		500			{object}	response.Error
//	@Router			/organizations/{orgId}/hrm/setup/document-templates/{templateId} [delete]
func (h *Handler) Delete(c fiber.Ctx) error {
	orgID, ok := middleware.OrganizationIDFromCtx(c)
	if !ok {
		return response.BadRequest(c, "NO_ORGANIZATION_CONTEXT", "Organization context is required")
	}
	if err := h.service.Delete(c.Context(), orgID, c.Params("templateId")); err != nil {
		return h.tmplError(c, err)
	}
	return response.NoContent(c)
}

// Preview godoc
//
//	@Summary		Preview document template
//	@Description	Fills placeholder variables in the template body and returns the rendered Markdown.
//	@Description	Use this to verify placeholder substitution before sending to an employee.
//	@Description
//	@Description	**Required permission:** `hrm.doc_templates.view`
//	@Tags			HRM / Document Templates
//	@Accept			json
//	@Produce		json
//	@Security		BearerAuth
//	@Param			orgId		path		string					true	"Organization ID"
//	@Param			templateId	path		string					true	"Template public ID (dt_*)"
//	@Param			body		body		PreviewTemplateRequest	true	"Variable values for substitution"
//	@Success		200			{object}	response.OK{data=PreviewResult}
//	@Failure		400			{object}	response.Error
//	@Failure		401			{object}	response.Error
//	@Failure		403			{object}	response.Error
//	@Failure		404			{object}	response.Error
//	@Failure		500			{object}	response.Error
//	@Router			/organizations/{orgId}/hrm/setup/document-templates/{templateId}/preview [post]
func (h *Handler) Preview(c fiber.Ctx) error {
	orgID, ok := middleware.OrganizationIDFromCtx(c)
	if !ok {
		return response.BadRequest(c, "NO_ORGANIZATION_CONTEXT", "Organization context is required")
	}
	var req PreviewTemplateRequest
	if err := c.Bind().JSON(&req); err != nil {
		return response.BadRequest(c, "INVALID_BODY", "Invalid request body")
	}
	if req.Variables == nil {
		req.Variables = map[string]string{}
	}
	result, err := h.service.Preview(c.Context(), orgID, c.Params("templateId"), req)
	if err != nil {
		return h.tmplError(c, err)
	}
	return response.OK(c, fiber.Map{"preview": result}, "OK")
}

func (h *Handler) tmplError(c fiber.Ctx, err error) error {
	log := logger.FromCtx(c)
	switch {
	case errors.Is(err, ErrTemplateNotFound):
		return response.NotFound(c, "TEMPLATE_NOT_FOUND", "Document template not found")
	case errors.Is(err, ErrNameRequired):
		return response.BadRequest(c, "NAME_REQUIRED", "Name is required")
	case errors.Is(err, ErrNameTooLong):
		return response.BadRequest(c, "NAME_TOO_LONG", "Name must not exceed 100 characters")
	case errors.Is(err, ErrNameConflict):
		return response.Conflict(c, "NAME_CONFLICT", "A template with this name already exists")
	case errors.Is(err, ErrInvalidDocumentType):
		return response.BadRequest(c, "INVALID_DOCUMENT_TYPE", "Invalid document_type value")
	case errors.Is(err, ErrBodyRequired):
		return response.BadRequest(c, "BODY_REQUIRED", "body_markdown is required")
	default:
		log.Error("doctemplates: error", slog.Any("error", err))
		return response.InternalServerError(c)
	}
}
