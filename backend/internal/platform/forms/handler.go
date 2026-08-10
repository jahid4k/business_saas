// backend/internal/platform/forms/handler.go
package forms

import (
	"errors"
	"log/slog"
	"strconv"

	"github.com/gofiber/fiber/v3"

	"github.com/mridha/businesssaas/internal/middleware"
	"github.com/mridha/businesssaas/pkg/logger"
	"github.com/mridha/businesssaas/pkg/response"
)

// Handler serves the generic form surface: authoring templates, and reading
// and answering instances. Instantiation is absent by design — see
// Service.Instantiate.
type Handler struct {
	service Service
}

func NewHandler(service Service) *Handler { return &Handler{service: service} }

var errUnauthenticated = errors.New("authentication required")

func orgFromCtx(c fiber.Ctx) (string, bool) { return middleware.OrganizationIDFromCtx(c) }

func callerUserID(c fiber.Ctx) (string, error) {
	userID, ok := middleware.UserIDFromCtx(c)
	if !ok {
		return "", errUnauthenticated
	}
	return userID, nil
}

func (h *Handler) mapError(c fiber.Ctx, err error) error {
	log := logger.FromCtx(c)
	switch {
	case errors.Is(err, errUnauthenticated):
		return response.Unauthorized(c, "UNAUTHORIZED", "Authentication required")

	case errors.Is(err, ErrTemplateNotFound):
		return response.NotFound(c, "FORM_TEMPLATE_NOT_FOUND", "Form template not found")
	case errors.Is(err, ErrSectionNotFound):
		return response.NotFound(c, "FORM_SECTION_NOT_FOUND", "Form section not found")
	case errors.Is(err, ErrQuestionNotFound):
		return response.NotFound(c, "FORM_QUESTION_NOT_FOUND", "Form question not found")
	case errors.Is(err, ErrInstanceNotFound):
		return response.NotFound(c, "FORM_INSTANCE_NOT_FOUND", "Form instance not found")
	case errors.Is(err, ErrResponseNotFound):
		return response.NotFound(c, "FORM_RESPONSE_NOT_FOUND", "Form response not found")

	case errors.Is(err, ErrNotRespondent):
		return response.Forbidden(c, "NOT_RESPONDENT", "Only the assigned respondent may answer this form")

	case errors.Is(err, ErrTemplateHasInstances):
		return response.Conflict(c, "TEMPLATE_HAS_INSTANCES", "This template has form instances and cannot be deleted — deactivate it instead")
	case errors.Is(err, ErrInstanceSubmitted):
		return response.Conflict(c, "FORM_SUBMITTED", "This form has been submitted and can no longer be edited")
	case errors.Is(err, ErrInstanceCancelled):
		return response.Conflict(c, "FORM_CANCELLED", "This form has been cancelled")
	case errors.Is(err, ErrRequiredUnanswered):
		return response.Conflict(c, "REQUIRED_UNANSWERED", "Every required question must be answered before submitting")
	case errors.Is(err, ErrTemplateEmpty):
		return response.Conflict(c, "TEMPLATE_EMPTY", "This template has no questions and cannot be instantiated")

	case errors.Is(err, ErrNameRequired):
		return response.BadRequest(c, "NAME_REQUIRED", "name is required")
	case errors.Is(err, ErrTitleRequired):
		return response.BadRequest(c, "TITLE_REQUIRED", "title is required")
	case errors.Is(err, ErrQuestionTextRequired):
		return response.BadRequest(c, "QUESTION_TEXT_REQUIRED", "question_text is required")
	case errors.Is(err, ErrInvalidFormType):
		return response.BadRequest(c, "INVALID_FORM_TYPE", "form_type must be one of: appraisal, feedback_360, survey, assessment, exit_interview, custom")
	case errors.Is(err, ErrInvalidQuestionType):
		return response.BadRequest(c, "INVALID_QUESTION_TYPE", "question_type must be one of: text, textarea, number, scale, single_select, multi_select, boolean, date")
	case errors.Is(err, ErrInvalidSubjectType):
		return response.BadRequest(c, "INVALID_SUBJECT_TYPE", "subject_type must be one of: employee, candidate")
	case errors.Is(err, ErrInvalidScaleBounds):
		return response.BadRequest(c, "INVALID_SCALE_BOUNDS", "scale_max must be greater than scale_min, and both are required for a scale question")
	case errors.Is(err, ErrOptionsRequired):
		return response.BadRequest(c, "OPTIONS_REQUIRED", "a select question requires at least one option")
	case errors.Is(err, ErrInvalidWeight):
		return response.BadRequest(c, "INVALID_WEIGHT", "weight must be zero or greater")
	case errors.Is(err, ErrAnswerTypeMismatch):
		return response.BadRequest(c, "ANSWER_TYPE_MISMATCH", "The supplied answer does not match the question's type")
	case errors.Is(err, ErrAnswerOutOfRange):
		return response.BadRequest(c, "ANSWER_OUT_OF_RANGE", "The answer falls outside the question's scale bounds")
	case errors.Is(err, ErrOptionNotAllowed):
		return response.BadRequest(c, "OPTION_NOT_ALLOWED", "The selected option is not one of the question's options")
	case errors.Is(err, ErrInvalidDate):
		return response.BadRequest(c, "INVALID_DATE", "answer_date must be a valid date in YYYY-MM-DD format")

	default:
		log.Error("forms: error", slog.Any("error", err))
		return response.InternalServerError(c)
	}
}

// ── Templates ────────────────────────────────────────────────────────────────

// ListTemplates godoc
//
//	@Summary		List form templates
//	@Tags			Platform / Forms
//	@Produce		json
//	@Security		BearerAuth
//	@Param			orgId		path	string	true	"Organization ID"
//	@Param			form_type	query	string	false	"Filter by form type"
//	@Success		200			{object}	response.OK{data=object{templates=[]Template}}
//	@Router			/organizations/{orgId}/forms/templates [get]
func (h *Handler) ListTemplates(c fiber.Ctx) error {
	orgID, ok := orgFromCtx(c)
	if !ok {
		return response.BadRequest(c, "NO_ORGANIZATION_CONTEXT", "Organization context is required")
	}
	var ft *FormType
	if raw := c.Query("form_type"); raw != "" {
		t := FormType(raw)
		ft = &t
	}
	list, err := h.service.ListTemplates(c.Context(), orgID, ft)
	if err != nil {
		return h.mapError(c, err)
	}
	return response.OK(c, fiber.Map{"templates": list}, "OK")
}

// GetTemplate godoc
//
//	@Summary		Get a form template with its sections and questions
//	@Tags			Platform / Forms
//	@Produce		json
//	@Security		BearerAuth
//	@Param			orgId		path	string	true	"Organization ID"
//	@Param			templateId	path	string	true	"Template public ID"
//	@Success		200			{object}	response.OK{data=object{template=TemplateWithSections}}
//	@Router			/organizations/{orgId}/forms/templates/{templateId} [get]
func (h *Handler) GetTemplate(c fiber.Ctx) error {
	orgID, ok := orgFromCtx(c)
	if !ok {
		return response.BadRequest(c, "NO_ORGANIZATION_CONTEXT", "Organization context is required")
	}
	t, err := h.service.GetTemplate(c.Context(), orgID, c.Params("templateId"))
	if err != nil {
		return h.mapError(c, err)
	}
	return response.OK(c, fiber.Map{"template": t}, "OK")
}

// CreateTemplate godoc
//
//	@Summary		Create a form template
//	@Tags			Platform / Forms
//	@Accept			json
//	@Produce		json
//	@Security		BearerAuth
//	@Param			orgId	path	string					true	"Organization ID"
//	@Param			body	body	CreateTemplateRequest	true	"Template"
//	@Success		201		{object}	response.Created{data=object{template=Template}}
//	@Router			/organizations/{orgId}/forms/templates [post]
func (h *Handler) CreateTemplate(c fiber.Ctx) error {
	orgID, ok := orgFromCtx(c)
	if !ok {
		return response.BadRequest(c, "NO_ORGANIZATION_CONTEXT", "Organization context is required")
	}
	userID, err := callerUserID(c)
	if err != nil {
		return h.mapError(c, err)
	}
	var req CreateTemplateRequest
	if err := c.Bind().JSON(&req); err != nil {
		return response.BadRequest(c, "INVALID_BODY", "Invalid request body")
	}
	t, err := h.service.CreateTemplate(c.Context(), orgID, userID, req)
	if err != nil {
		return h.mapError(c, err)
	}
	return response.Created(c, fiber.Map{"template": t}, "Form template created")
}

// UpdateTemplate godoc
//
//	@Summary		Update a form template
//	@Tags			Platform / Forms
//	@Accept			json
//	@Produce		json
//	@Security		BearerAuth
//	@Param			orgId		path	string					true	"Organization ID"
//	@Param			templateId	path	string					true	"Template public ID"
//	@Param			body		body	UpdateTemplateRequest	true	"Fields to update"
//	@Success		200			{object}	response.OK{data=object{template=Template}}
//	@Router			/organizations/{orgId}/forms/templates/{templateId} [patch]
func (h *Handler) UpdateTemplate(c fiber.Ctx) error {
	orgID, ok := orgFromCtx(c)
	if !ok {
		return response.BadRequest(c, "NO_ORGANIZATION_CONTEXT", "Organization context is required")
	}
	var req UpdateTemplateRequest
	if err := c.Bind().JSON(&req); err != nil {
		return response.BadRequest(c, "INVALID_BODY", "Invalid request body")
	}
	t, err := h.service.UpdateTemplate(c.Context(), orgID, c.Params("templateId"), req)
	if err != nil {
		return h.mapError(c, err)
	}
	return response.OK(c, fiber.Map{"template": t}, "Form template updated")
}

// DeleteTemplate godoc
//
//	@Summary		Delete a form template
//	@Description	Refused once instances exist — deactivate instead.
//	@Tags			Platform / Forms
//	@Produce		json
//	@Security		BearerAuth
//	@Param			orgId		path	string	true	"Organization ID"
//	@Param			templateId	path	string	true	"Template public ID"
//	@Success		204
//	@Failure		409			{object}	response.Error	"TEMPLATE_HAS_INSTANCES"
//	@Router			/organizations/{orgId}/forms/templates/{templateId} [delete]
func (h *Handler) DeleteTemplate(c fiber.Ctx) error {
	orgID, ok := orgFromCtx(c)
	if !ok {
		return response.BadRequest(c, "NO_ORGANIZATION_CONTEXT", "Organization context is required")
	}
	if err := h.service.DeleteTemplate(c.Context(), orgID, c.Params("templateId")); err != nil {
		return h.mapError(c, err)
	}
	return response.NoContent(c)
}

// ── Sections ─────────────────────────────────────────────────────────────────

// CreateSection godoc
//
//	@Summary		Add a section to a form template
//	@Tags			Platform / Forms
//	@Accept			json
//	@Produce		json
//	@Security		BearerAuth
//	@Param			orgId		path	string					true	"Organization ID"
//	@Param			templateId	path	string					true	"Template public ID"
//	@Param			body		body	CreateSectionRequest	true	"Section"
//	@Success		201			{object}	response.Created{data=object{section=Section}}
//	@Router			/organizations/{orgId}/forms/templates/{templateId}/sections [post]
func (h *Handler) CreateSection(c fiber.Ctx) error {
	orgID, ok := orgFromCtx(c)
	if !ok {
		return response.BadRequest(c, "NO_ORGANIZATION_CONTEXT", "Organization context is required")
	}
	var req CreateSectionRequest
	if err := c.Bind().JSON(&req); err != nil {
		return response.BadRequest(c, "INVALID_BODY", "Invalid request body")
	}
	sec, err := h.service.CreateSection(c.Context(), orgID, c.Params("templateId"), req)
	if err != nil {
		return h.mapError(c, err)
	}
	return response.Created(c, fiber.Map{"section": sec}, "Section created")
}

// UpdateSection godoc
//
//	@Summary		Update a form section
//	@Tags			Platform / Forms
//	@Accept			json
//	@Produce		json
//	@Security		BearerAuth
//	@Param			orgId		path	string					true	"Organization ID"
//	@Param			sectionId	path	string					true	"Section public ID"
//	@Param			body		body	UpdateSectionRequest	true	"Fields to update"
//	@Success		200			{object}	response.OK{data=object{section=Section}}
//	@Router			/organizations/{orgId}/forms/sections/{sectionId} [patch]
func (h *Handler) UpdateSection(c fiber.Ctx) error {
	orgID, ok := orgFromCtx(c)
	if !ok {
		return response.BadRequest(c, "NO_ORGANIZATION_CONTEXT", "Organization context is required")
	}
	var req UpdateSectionRequest
	if err := c.Bind().JSON(&req); err != nil {
		return response.BadRequest(c, "INVALID_BODY", "Invalid request body")
	}
	sec, err := h.service.UpdateSection(c.Context(), orgID, c.Params("sectionId"), req)
	if err != nil {
		return h.mapError(c, err)
	}
	return response.OK(c, fiber.Map{"section": sec}, "Section updated")
}

// DeleteSection godoc
//
//	@Summary		Delete a form section and its questions
//	@Tags			Platform / Forms
//	@Produce		json
//	@Security		BearerAuth
//	@Param			orgId		path	string	true	"Organization ID"
//	@Param			sectionId	path	string	true	"Section public ID"
//	@Success		204
//	@Router			/organizations/{orgId}/forms/sections/{sectionId} [delete]
func (h *Handler) DeleteSection(c fiber.Ctx) error {
	orgID, ok := orgFromCtx(c)
	if !ok {
		return response.BadRequest(c, "NO_ORGANIZATION_CONTEXT", "Organization context is required")
	}
	if err := h.service.DeleteSection(c.Context(), orgID, c.Params("sectionId")); err != nil {
		return h.mapError(c, err)
	}
	return response.NoContent(c)
}

// ── Questions ────────────────────────────────────────────────────────────────

// CreateQuestion godoc
//
//	@Summary		Add a question to a section
//	@Tags			Platform / Forms
//	@Accept			json
//	@Produce		json
//	@Security		BearerAuth
//	@Param			orgId		path	string					true	"Organization ID"
//	@Param			sectionId	path	string					true	"Section public ID"
//	@Param			body		body	CreateQuestionRequest	true	"Question"
//	@Success		201			{object}	response.Created{data=object{question=Question}}
//	@Router			/organizations/{orgId}/forms/sections/{sectionId}/questions [post]
func (h *Handler) CreateQuestion(c fiber.Ctx) error {
	orgID, ok := orgFromCtx(c)
	if !ok {
		return response.BadRequest(c, "NO_ORGANIZATION_CONTEXT", "Organization context is required")
	}
	var req CreateQuestionRequest
	if err := c.Bind().JSON(&req); err != nil {
		return response.BadRequest(c, "INVALID_BODY", "Invalid request body")
	}
	q, err := h.service.CreateQuestion(c.Context(), orgID, c.Params("sectionId"), req)
	if err != nil {
		return h.mapError(c, err)
	}
	return response.Created(c, fiber.Map{"question": q}, "Question created")
}

// UpdateQuestion godoc
//
//	@Summary		Update a form question
//	@Tags			Platform / Forms
//	@Accept			json
//	@Produce		json
//	@Security		BearerAuth
//	@Param			orgId		path	string					true	"Organization ID"
//	@Param			questionId	path	string					true	"Question public ID"
//	@Param			body		body	UpdateQuestionRequest	true	"Fields to update"
//	@Success		200			{object}	response.OK{data=object{question=Question}}
//	@Router			/organizations/{orgId}/forms/questions/{questionId} [patch]
func (h *Handler) UpdateQuestion(c fiber.Ctx) error {
	orgID, ok := orgFromCtx(c)
	if !ok {
		return response.BadRequest(c, "NO_ORGANIZATION_CONTEXT", "Organization context is required")
	}
	var req UpdateQuestionRequest
	if err := c.Bind().JSON(&req); err != nil {
		return response.BadRequest(c, "INVALID_BODY", "Invalid request body")
	}
	q, err := h.service.UpdateQuestion(c.Context(), orgID, c.Params("questionId"), req)
	if err != nil {
		return h.mapError(c, err)
	}
	return response.OK(c, fiber.Map{"question": q}, "Question updated")
}

// DeleteQuestion godoc
//
//	@Summary		Delete a form question
//	@Tags			Platform / Forms
//	@Produce		json
//	@Security		BearerAuth
//	@Param			orgId		path	string	true	"Organization ID"
//	@Param			questionId	path	string	true	"Question public ID"
//	@Success		204
//	@Router			/organizations/{orgId}/forms/questions/{questionId} [delete]
func (h *Handler) DeleteQuestion(c fiber.Ctx) error {
	orgID, ok := orgFromCtx(c)
	if !ok {
		return response.BadRequest(c, "NO_ORGANIZATION_CONTEXT", "Organization context is required")
	}
	if err := h.service.DeleteQuestion(c.Context(), orgID, c.Params("questionId")); err != nil {
		return h.mapError(c, err)
	}
	return response.NoContent(c)
}

// ── Instances ────────────────────────────────────────────────────────────────

// ListInstances godoc
//
//	@Summary		List form instances
//	@Tags			Platform / Forms
//	@Produce		json
//	@Security		BearerAuth
//	@Param			orgId			path	string	true	"Organization ID"
//	@Param			form_type		query	string	false	"Filter by form type"
//	@Param			subject_id		query	string	false	"Filter by subject"
//	@Param			respondent_id	query	string	false	"Filter by respondent user"
//	@Param			status			query	string	false	"Filter by status"
//	@Success		200				{object}	response.OK{data=InstanceListResponse}
//	@Router			/organizations/{orgId}/forms/instances [get]
func (h *Handler) ListInstances(c fiber.Ctx) error {
	orgID, ok := orgFromCtx(c)
	if !ok {
		return response.BadRequest(c, "NO_ORGANIZATION_CONTEXT", "Organization context is required")
	}
	filter := InstanceListFilter{
		FormType:         c.Query("form_type"),
		SubjectType:      c.Query("subject_type"),
		SubjectID:        c.Query("subject_id"),
		RespondentUserID: c.Query("respondent_id"),
		Status:           c.Query("status"),
	}
	if limit, err := strconv.Atoi(c.Query("limit", "")); err == nil {
		filter.Limit = limit
	}
	if offset, err := strconv.Atoi(c.Query("offset", "")); err == nil {
		filter.Offset = offset
	}
	res, err := h.service.ListInstances(c.Context(), orgID, filter)
	if err != nil {
		return h.mapError(c, err)
	}
	return response.OK(c, res, "OK")
}

// ListMyInstances godoc
//
//	@Summary		List form instances assigned to the caller
//	@Tags			Platform / Forms
//	@Produce		json
//	@Security		BearerAuth
//	@Param			orgId	path	string	true	"Organization ID"
//	@Param			status	query	string	false	"Filter by status"
//	@Success		200		{object}	response.OK{data=InstanceListResponse}
//	@Router			/organizations/{orgId}/forms/instances/mine [get]
func (h *Handler) ListMyInstances(c fiber.Ctx) error {
	orgID, ok := orgFromCtx(c)
	if !ok {
		return response.BadRequest(c, "NO_ORGANIZATION_CONTEXT", "Organization context is required")
	}
	userID, err := callerUserID(c)
	if err != nil {
		return h.mapError(c, err)
	}
	filter := InstanceListFilter{RespondentUserID: userID, Status: c.Query("status")}
	res, err := h.service.ListInstances(c.Context(), orgID, filter)
	if err != nil {
		return h.mapError(c, err)
	}
	return response.OK(c, res, "OK")
}

// GetInstance godoc
//
//	@Summary		Get a form instance with its questions, answers and score
//	@Tags			Platform / Forms
//	@Produce		json
//	@Security		BearerAuth
//	@Param			orgId		path	string	true	"Organization ID"
//	@Param			instanceId	path	string	true	"Instance public ID"
//	@Success		200			{object}	response.OK{data=object{instance=InstanceWithResponses}}
//	@Router			/organizations/{orgId}/forms/instances/{instanceId} [get]
func (h *Handler) GetInstance(c fiber.Ctx) error {
	orgID, ok := orgFromCtx(c)
	if !ok {
		return response.BadRequest(c, "NO_ORGANIZATION_CONTEXT", "Organization context is required")
	}
	inst, err := h.service.GetInstance(c.Context(), orgID, c.Params("instanceId"))
	if err != nil {
		return h.mapError(c, err)
	}
	return response.OK(c, fiber.Map{"instance": inst}, "OK")
}

// SaveAnswers godoc
//
//	@Summary		Save answers to a form instance
//	@Description	A partial save — only the listed responses change, so a long
//	@Description	form can be filled in across several sittings. Required
//	@Description	questions are enforced at submit, not here.
//	@Tags			Platform / Forms
//	@Accept			json
//	@Produce		json
//	@Security		BearerAuth
//	@Param			orgId		path	string				true	"Organization ID"
//	@Param			instanceId	path	string				true	"Instance public ID"
//	@Param			body		body	SaveAnswersRequest	true	"Answers"
//	@Success		200			{object}	response.OK{data=object{instance=InstanceWithResponses}}
//	@Failure		403			{object}	response.Error	"NOT_RESPONDENT"
//	@Failure		409			{object}	response.Error	"FORM_SUBMITTED"
//	@Router			/organizations/{orgId}/forms/instances/{instanceId}/answers [post]
func (h *Handler) SaveAnswers(c fiber.Ctx) error {
	orgID, ok := orgFromCtx(c)
	if !ok {
		return response.BadRequest(c, "NO_ORGANIZATION_CONTEXT", "Organization context is required")
	}
	userID, err := callerUserID(c)
	if err != nil {
		return h.mapError(c, err)
	}
	var req SaveAnswersRequest
	if err := c.Bind().JSON(&req); err != nil {
		return response.BadRequest(c, "INVALID_BODY", "Invalid request body")
	}
	inst, err := h.service.SaveAnswers(c.Context(), orgID, c.Params("instanceId"), userID, req)
	if err != nil {
		return h.mapError(c, err)
	}
	return response.OK(c, fiber.Map{"instance": inst}, "Answers saved")
}

// SubmitInstance godoc
//
//	@Summary		Submit a form instance (locks it)
//	@Description	Every required question must be answered. Once submitted the
//	@Description	form is immutable.
//	@Tags			Platform / Forms
//	@Produce		json
//	@Security		BearerAuth
//	@Param			orgId		path	string	true	"Organization ID"
//	@Param			instanceId	path	string	true	"Instance public ID"
//	@Success		200			{object}	response.OK{data=object{instance=InstanceWithResponses}}
//	@Failure		409			{object}	response.Error	"REQUIRED_UNANSWERED or FORM_SUBMITTED"
//	@Router			/organizations/{orgId}/forms/instances/{instanceId}/submit [post]
func (h *Handler) SubmitInstance(c fiber.Ctx) error {
	orgID, ok := orgFromCtx(c)
	if !ok {
		return response.BadRequest(c, "NO_ORGANIZATION_CONTEXT", "Organization context is required")
	}
	userID, err := callerUserID(c)
	if err != nil {
		return h.mapError(c, err)
	}
	inst, err := h.service.SubmitInstance(c.Context(), orgID, c.Params("instanceId"), userID)
	if err != nil {
		return h.mapError(c, err)
	}
	return response.OK(c, fiber.Map{"instance": inst}, "Form submitted")
}

// CancelInstance godoc
//
//	@Summary		Cancel a form instance
//	@Tags			Platform / Forms
//	@Produce		json
//	@Security		BearerAuth
//	@Param			orgId		path	string	true	"Organization ID"
//	@Param			instanceId	path	string	true	"Instance public ID"
//	@Success		200			{object}	response.OK{data=object{instance=Instance}}
//	@Router			/organizations/{orgId}/forms/instances/{instanceId}/cancel [post]
func (h *Handler) CancelInstance(c fiber.Ctx) error {
	orgID, ok := orgFromCtx(c)
	if !ok {
		return response.BadRequest(c, "NO_ORGANIZATION_CONTEXT", "Organization context is required")
	}
	inst, err := h.service.CancelInstance(c.Context(), orgID, c.Params("instanceId"))
	if err != nil {
		return h.mapError(c, err)
	}
	return response.OK(c, fiber.Map{"instance": inst}, "Form cancelled")
}
