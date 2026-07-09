// backend/internal/hrm/employeedocs/handler.go
package employeedocs

import (
	"errors"
	"log/slog"

	"github.com/gofiber/fiber/v3"

	"github.com/mridha/businesssaas/internal/middleware"
	"github.com/mridha/businesssaas/pkg/logger"
	"github.com/mridha/businesssaas/pkg/response"
)

type Handler struct{ service Service }
func NewHandler(service Service) *Handler { return &Handler{service: service} }

// List godoc
//
//	@Summary		List employee documents
//	@Description	Returns document records for an employee. Filter by status or related_type.
//	@Description
//	@Description	**Required permission:** `hrm.documents.view`
//	@Tags			HRM / Documents
//	@Produce		json
//	@Security		BearerAuth
//	@Param			orgId		path		string	true	"Organization ID"
//	@Param			employeeId	path		string	true	"Employee public ID"
//	@Param			status		query		string	false	"Filter by status"
//	@Param			related_type query		string	false	"Filter by entity type (warning|promotion|transfer|...)"
//	@Success		200			{object}	response.OK{data=DocListResponse}
//	@Router			/organizations/{orgId}/hrm/employees/{employeeId}/documents [get]
func (h *Handler) List(c fiber.Ctx) error {
	log := logger.FromCtx(c)
	orgID, ok := middleware.OrganizationIDFromCtx(c)
	if !ok { return response.BadRequest(c, "NO_ORGANIZATION_CONTEXT", "Organization context is required") }
	res, err := h.service.List(c.Context(), orgID, c.Params("employeeId"), c.Query("status"), c.Query("related_type"))
	if err != nil { log.Error("employeedocs: List", slog.Any("error", err)); return response.InternalServerError(c) }
	return response.OK(c, res, "OK")
}

// ListAll godoc — HR view: all employees' documents
func (h *Handler) ListAll(c fiber.Ctx) error {
	log := logger.FromCtx(c)
	orgID, ok := middleware.OrganizationIDFromCtx(c)
	if !ok { return response.BadRequest(c, "NO_ORGANIZATION_CONTEXT", "Organization context is required") }
	res, err := h.service.List(c.Context(), orgID, c.Query("employee_id"), c.Query("status"), c.Query("related_type"))
	if err != nil { log.Error("employeedocs: ListAll", slog.Any("error", err)); return response.InternalServerError(c) }
	return response.OK(c, res, "OK")
}

// Create godoc
//
//	@Summary		Create employee document
//	@Description	Creates an employee document (draft). Supply template_id for template-generated,
//	@Description	or leave nil for direct upload. Send via POST .../send.
//	@Description
//	@Description	**Required permission:** `hrm.documents.manage`
//	@Tags			HRM / Documents
//	@Accept			json
//	@Produce		json
//	@Security		BearerAuth
//	@Param			orgId		path		string					true	"Organization ID"
//	@Param			employeeId	path		string					true	"Employee public ID"
//	@Param			body		body		CreateDocumentRequest	true	"Document details"
//	@Success		201			{object}	response.Created{data=object{document=EmployeeDocument}}
//	@Router			/organizations/{orgId}/hrm/employees/{employeeId}/documents [post]
func (h *Handler) Create(c fiber.Ctx) error {
	userID, ok := middleware.UserIDFromCtx(c)
	if !ok { return response.Unauthorized(c, "UNAUTHORIZED", "Authentication required") }
	orgID, ok := middleware.OrganizationIDFromCtx(c)
	if !ok { return response.BadRequest(c, "NO_ORGANIZATION_CONTEXT", "Organization context is required") }
	var req CreateDocumentRequest
	if err := c.Bind().JSON(&req); err != nil { return response.BadRequest(c, "INVALID_BODY", "Invalid request body") }
	d, err := h.service.Create(c.Context(), orgID, c.Params("employeeId"), userID, req)
	if err != nil { return h.err(c, err) }
	return response.Created(c, fiber.Map{"document": d}, "Document created")
}

func (h *Handler) Get(c fiber.Ctx) error {
	orgID, ok := middleware.OrganizationIDFromCtx(c)
	if !ok { return response.BadRequest(c, "NO_ORGANIZATION_CONTEXT", "Organization context is required") }
	d, err := h.service.Get(c.Context(), orgID, c.Params("employeeId"), c.Params("documentId"))
	if err != nil { return h.err(c, err) }
	return response.OK(c, fiber.Map{"document": d}, "OK")
}

func (h *Handler) Send(c fiber.Ctx) error {
	orgID, ok := middleware.OrganizationIDFromCtx(c)
	if !ok { return response.BadRequest(c, "NO_ORGANIZATION_CONTEXT", "Organization context is required") }
	d, err := h.service.Send(c.Context(), orgID, c.Params("employeeId"), c.Params("documentId"))
	if err != nil { return h.err(c, err) }
	return response.OK(c, fiber.Map{"document": d}, "Document sent to employee")
}

func (h *Handler) Acknowledge(c fiber.Ctx) error {
	orgID, ok := middleware.OrganizationIDFromCtx(c)
	if !ok { return response.BadRequest(c, "NO_ORGANIZATION_CONTEXT", "Organization context is required") }
	var req AcknowledgeDocRequest
	_ = c.Bind().JSON(&req)
	d, err := h.service.Acknowledge(c.Context(), orgID, c.Params("employeeId"), c.Params("documentId"), req)
	if err != nil { return h.err(c, err) }
	return response.OK(c, fiber.Map{"document": d}, "Document acknowledged")
}

func (h *Handler) Decline(c fiber.Ctx) error {
	orgID, ok := middleware.OrganizationIDFromCtx(c)
	if !ok { return response.BadRequest(c, "NO_ORGANIZATION_CONTEXT", "Organization context is required") }
	d, err := h.service.Decline(c.Context(), orgID, c.Params("employeeId"), c.Params("documentId"))
	if err != nil { return h.err(c, err) }
	return response.OK(c, fiber.Map{"document": d}, "Document declined")
}

func (h *Handler) Withdraw(c fiber.Ctx) error {
	orgID, ok := middleware.OrganizationIDFromCtx(c)
	if !ok { return response.BadRequest(c, "NO_ORGANIZATION_CONTEXT", "Organization context is required") }
	d, err := h.service.Withdraw(c.Context(), orgID, c.Params("employeeId"), c.Params("documentId"))
	if err != nil { return h.err(c, err) }
	return response.OK(c, fiber.Map{"document": d}, "Document withdrawn")
}

func (h *Handler) err(c fiber.Ctx, err error) error {
	log := logger.FromCtx(c)
	switch {
	case errors.Is(err, ErrNotFound): return response.NotFound(c, "DOCUMENT_NOT_FOUND", "Document not found")
	case errors.Is(err, ErrTitleRequired): return response.BadRequest(c, "TITLE_REQUIRED", "title is required")
	case errors.Is(err, ErrFileURLRequired): return response.BadRequest(c, "FILE_URL_REQUIRED", "file_url is required")
	case errors.Is(err, ErrFileNameRequired): return response.BadRequest(c, "FILE_NAME_REQUIRED", "file_name is required")
	case errors.Is(err, ErrDocTypeRequired): return response.BadRequest(c, "DOC_TYPE_REQUIRED", "document_type is required")
	case errors.Is(err, ErrWrongStatus): return response.Conflict(c, "WRONG_STATUS", "Action not allowed in current document status")
	case errors.Is(err, ErrAlreadyAcknowledged): return response.Conflict(c, "ALREADY_ACKNOWLEDGED", "Document has already been acknowledged")
	default: log.Error("employeedocs: error", slog.Any("error", err)); return response.InternalServerError(c)
	}
}
