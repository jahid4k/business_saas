// backend/internal/hrm/acknowledgements/handler.go
package acknowledgements

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
//	@Summary		List acknowledgements
//	@Description	HR: list all. Employee: filter by employee_id.
//	@Description	Filter by acknowledgeable_type (warning|document|announcement|calendar_event|policy) and status.
//	@Description
//	@Description	**Required permission:** `hrm.acknowledgements.view`
//	@Tags			HRM / Acknowledgements
//	@Produce		json
//	@Security		BearerAuth
//	@Param			orgId					path		string	true	"Organization ID"
//	@Param			employee_id				query		string	false	"Filter by employee UUID"
//	@Param			acknowledgeable_type	query		string	false	"Filter by entity type"
//	@Param			status					query		string	false	"pending|acknowledged|declined|expired"
//	@Success		200						{object}	response.OK{data=AckListResponse}
//	@Router			/organizations/{orgId}/hrm/acknowledgements [get]
func (h *Handler) List(c fiber.Ctx) error {
	log := logger.FromCtx(c)
	orgID, ok := middleware.OrganizationIDFromCtx(c)
	if !ok { return response.BadRequest(c, "NO_ORGANIZATION_CONTEXT", "Organization context is required") }
	res, err := h.service.List(c.Context(), orgID, c.Query("employee_id"), c.Query("acknowledgeable_type"), c.Query("status"))
	if err != nil { log.Error("acknowledgements: List", slog.Any("error", err)); return response.InternalServerError(c) }
	return response.OK(c, res, "OK")
}

// ListByEntity godoc
//
//	@Summary		List acknowledgements for an entity
//	@Description	Returns all acknowledgement records for a specific entity (e.g. all acks for warning ew_xxx).
//	@Description
//	@Description	**Required permission:** `hrm.acknowledgements.view`
//	@Tags			HRM / Acknowledgements
//	@Produce		json
//	@Security		BearerAuth
//	@Param			orgId	path		string	true	"Organization ID"
//	@Param			type	path		string	true	"Entity type (warning|document|announcement|calendar_event|policy)"
//	@Param			id		path		string	true	"Entity UUID"
//	@Success		200		{object}	response.OK{data=AckListResponse}
//	@Router			/organizations/{orgId}/hrm/acknowledgements/entity/{type}/{id} [get]
func (h *Handler) ListByEntity(c fiber.Ctx) error {
	log := logger.FromCtx(c)
	orgID, ok := middleware.OrganizationIDFromCtx(c)
	if !ok { return response.BadRequest(c, "NO_ORGANIZATION_CONTEXT", "Organization context is required") }
	res, err := h.service.ListByEntity(c.Context(), orgID, c.Params("type"), c.Params("id"))
	if err != nil { log.Error("acknowledgements: ListByEntity", slog.Any("error", err)); return response.InternalServerError(c) }
	return response.OK(c, res, "OK")
}

// Create godoc
//
//	@Summary		Create acknowledgement request (HR action)
//	@Description	Sends an acknowledgement request to an employee for any entity type.
//	@Description
//	@Description	**Required permission:** `hrm.acknowledgements.manage`
//	@Tags			HRM / Acknowledgements
//	@Accept			json
//	@Produce		json
//	@Security		BearerAuth
//	@Param			orgId	path		string							true	"Organization ID"
//	@Param			body	body		CreateAcknowledgementRequest	true	"Acknowledgement request"
//	@Success		201		{object}	response.Created{data=object{acknowledgement=Acknowledgement}}
//	@Router			/organizations/{orgId}/hrm/acknowledgements [post]
func (h *Handler) Create(c fiber.Ctx) error {
	userID, ok := middleware.UserIDFromCtx(c)
	if !ok { return response.Unauthorized(c, "UNAUTHORIZED", "Authentication required") }
	orgID, ok := middleware.OrganizationIDFromCtx(c)
	if !ok { return response.BadRequest(c, "NO_ORGANIZATION_CONTEXT", "Organization context is required") }
	var req CreateAcknowledgementRequest
	if err := c.Bind().JSON(&req); err != nil { return response.BadRequest(c, "INVALID_BODY", "Invalid request body") }
	a, err := h.service.Create(c.Context(), orgID, userID, req)
	if err != nil { return h.err(c, err) }
	return response.Created(c, fiber.Map{"acknowledgement": a}, "Acknowledgement request sent")
}

// Get godoc
//
//	@Summary		Get acknowledgement record
//	@Tags			HRM / Acknowledgements
//	@Produce		json
//	@Security		BearerAuth
//	@Param			orgId	path		string	true	"Organization ID"
//	@Param			ackId	path		string	true	"Acknowledgement public ID (ack_*)"
//	@Success		200		{object}	response.OK{data=object{acknowledgement=Acknowledgement}}
//	@Router			/organizations/{orgId}/hrm/acknowledgements/{ackId} [get]
func (h *Handler) Get(c fiber.Ctx) error {
	orgID, ok := middleware.OrganizationIDFromCtx(c)
	if !ok { return response.BadRequest(c, "NO_ORGANIZATION_CONTEXT", "Organization context is required") }
	a, err := h.service.Get(c.Context(), orgID, c.Params("ackId"))
	if err != nil { return h.err(c, err) }
	return response.OK(c, fiber.Map{"acknowledgement": a}, "OK")
}

// Respond godoc
//
//	@Summary		Acknowledge (employee action)
//	@Description	Employee acknowledges. If signature_required=true, signature_data must be provided.
//	@Description
//	@Description	**Required permission:** `hrm.acknowledgements.respond`
//	@Tags			HRM / Acknowledgements
//	@Accept			json
//	@Produce		json
//	@Security		BearerAuth
//	@Param			orgId	path		string			true	"Organization ID"
//	@Param			ackId	path		string			true	"Acknowledgement public ID"
//	@Param			body	body		RespondRequest	false	"Optional notes and signature"
//	@Success		200		{object}	response.OK{data=object{acknowledgement=Acknowledgement}}
//	@Failure		409		{object}	response.Error	"WRONG_STATUS or SIGNATURE_REQUIRED"
//	@Router			/organizations/{orgId}/hrm/acknowledgements/{ackId}/acknowledge [post]
func (h *Handler) Respond(c fiber.Ctx) error {
	orgID, ok := middleware.OrganizationIDFromCtx(c)
	if !ok { return response.BadRequest(c, "NO_ORGANIZATION_CONTEXT", "Organization context is required") }
	var req RespondRequest
	_ = c.Bind().JSON(&req)
	a, err := h.service.Respond(c.Context(), orgID, c.Params("ackId"), req)
	if err != nil { return h.err(c, err) }
	return response.OK(c, fiber.Map{"acknowledgement": a}, "Acknowledged")
}

// Decline godoc
//
//	@Summary		Decline acknowledgement (employee action)
//	@Description	Employee declines. Reason is recorded.
//	@Description
//	@Description	**Required permission:** `hrm.acknowledgements.respond`
//	@Tags			HRM / Acknowledgements
//	@Accept			json
//	@Produce		json
//	@Security		BearerAuth
//	@Param			orgId	path		string			true	"Organization ID"
//	@Param			ackId	path		string			true	"Acknowledgement public ID"
//	@Param			body	body		DeclineRequest	false	"Decline reason"
//	@Success		200		{object}	response.OK{data=object{acknowledgement=Acknowledgement}}
//	@Failure		409		{object}	response.Error
//	@Router			/organizations/{orgId}/hrm/acknowledgements/{ackId}/decline [post]
func (h *Handler) Decline(c fiber.Ctx) error {
	orgID, ok := middleware.OrganizationIDFromCtx(c)
	if !ok { return response.BadRequest(c, "NO_ORGANIZATION_CONTEXT", "Organization context is required") }
	var req DeclineRequest
	_ = c.Bind().JSON(&req)
	a, err := h.service.Decline(c.Context(), orgID, c.Params("ackId"), req)
	if err != nil { return h.err(c, err) }
	return response.OK(c, fiber.Map{"acknowledgement": a}, "Declined")
}

func (h *Handler) err(c fiber.Ctx, err error) error {
	log := logger.FromCtx(c)
	switch {
	case errors.Is(err, ErrNotFound): return response.NotFound(c, "ACK_NOT_FOUND", "Acknowledgement not found")
	case errors.Is(err, ErrEmployeeIDRequired): return response.BadRequest(c, "EMPLOYEE_ID_REQUIRED", "employee_id is required")
	case errors.Is(err, ErrEntityTitleRequired): return response.BadRequest(c, "ENTITY_TITLE_REQUIRED", "entity_title is required")
	case errors.Is(err, ErrInvalidAckType): return response.BadRequest(c, "INVALID_ACK_TYPE", "acknowledgeable_type must be: warning, document, announcement, calendar_event, or policy")
	case errors.Is(err, ErrAckIDRequired): return response.BadRequest(c, "ACK_ID_REQUIRED", "acknowledgeable_id is required")
	case errors.Is(err, ErrInvalidDate): return response.BadRequest(c, "INVALID_DATE", "date must be a valid YYYY-MM-DD")
	case errors.Is(err, ErrWrongStatus): return response.Conflict(c, "WRONG_STATUS", "Action not allowed in current acknowledgement status")
	case errors.Is(err, ErrSignatureRequired): return response.BadRequest(c, "SIGNATURE_REQUIRED", "signature_data is required when signature_required=true")
	default: log.Error("acknowledgements: error", slog.Any("error", err)); return response.InternalServerError(c)
	}
}
