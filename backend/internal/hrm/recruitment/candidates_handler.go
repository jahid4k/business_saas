// backend/internal/hrm/recruitment/candidates_handler.go
package recruitment

import (
	"io"

	"github.com/gofiber/fiber/v3"

	"github.com/mridha/businesssaas/internal/middleware"
	"github.com/mridha/businesssaas/pkg/response"
)

// ListCandidates godoc
//
//	@Summary		List candidates
//	@Tags			HRM / Recruitment
//	@Produce		json
//	@Security		BearerAuth
//	@Param			orgId	path	string	true	"Organization ID"
//	@Param			search	query	string	false	"Search by name or email"
//	@Success		200		{object}	response.OK{data=CandidateListResponse}
//	@Router			/organizations/{orgId}/hrm/recruitment/candidates [get]
func (h *Handler) ListCandidates(c fiber.Ctx) error {
	orgID, ok := middleware.OrganizationIDFromCtx(c)
	if !ok {
		return response.BadRequest(c, "NO_ORGANIZATION_CONTEXT", "Organization context is required")
	}
	filter := CandidateListFilter{Search: c.Query("search")}
	res, err := h.service.ListCandidates(c.Context(), orgID, filter)
	if err != nil {
		return h.err(c, err)
	}
	return response.OK(c, res, "OK")
}

// GetCandidate godoc
//
//	@Summary		Get candidate
//	@Tags			HRM / Recruitment
//	@Produce		json
//	@Security		BearerAuth
//	@Param			orgId		path	string	true	"Organization ID"
//	@Param			candidateId	path	string	true	"Candidate public ID"
//	@Success		200			{object}	response.OK{data=object{candidate=Candidate}}
//	@Router			/organizations/{orgId}/hrm/recruitment/candidates/{candidateId} [get]
func (h *Handler) GetCandidate(c fiber.Ctx) error {
	orgID, ok := middleware.OrganizationIDFromCtx(c)
	if !ok {
		return response.BadRequest(c, "NO_ORGANIZATION_CONTEXT", "Organization context is required")
	}
	cand, err := h.service.GetCandidate(c.Context(), orgID, c.Params("candidateId"))
	if err != nil {
		return h.err(c, err)
	}
	return response.OK(c, fiber.Map{"candidate": cand}, "OK")
}

// CreateCandidate godoc
//
//	@Summary		Create candidate
//	@Tags			HRM / Recruitment
//	@Accept			json
//	@Produce		json
//	@Security		BearerAuth
//	@Param			orgId	path	string					true	"Organization ID"
//	@Param			body	body	CreateCandidateRequest	true	"Candidate"
//	@Success		201		{object}	response.Created{data=object{candidate=Candidate}}
//	@Failure		409		{object}	response.Error	"CANDIDATE_EMAIL_EXISTS"
//	@Router			/organizations/{orgId}/hrm/recruitment/candidates [post]
func (h *Handler) CreateCandidate(c fiber.Ctx) error {
	orgID, ok := middleware.OrganizationIDFromCtx(c)
	if !ok {
		return response.BadRequest(c, "NO_ORGANIZATION_CONTEXT", "Organization context is required")
	}
	userID, _ := middleware.UserIDFromCtx(c)
	var createdBy *string
	if userID != "" {
		createdBy = &userID
	}
	var req CreateCandidateRequest
	if err := c.Bind().JSON(&req); err != nil {
		return response.BadRequest(c, "INVALID_BODY", "Invalid request body")
	}
	cand, err := h.service.CreateCandidate(c.Context(), orgID, createdBy, req)
	if err != nil {
		return h.err(c, err)
	}
	return response.Created(c, fiber.Map{"candidate": cand}, "Candidate created")
}

// UpdateCandidate godoc
//
//	@Summary		Update candidate
//	@Tags			HRM / Recruitment
//	@Accept			json
//	@Produce		json
//	@Security		BearerAuth
//	@Param			orgId		path	string					true	"Organization ID"
//	@Param			candidateId	path	string					true	"Candidate public ID"
//	@Param			body		body	UpdateCandidateRequest	true	"Fields to update"
//	@Success		200			{object}	response.OK{data=object{candidate=Candidate}}
//	@Router			/organizations/{orgId}/hrm/recruitment/candidates/{candidateId} [patch]
func (h *Handler) UpdateCandidate(c fiber.Ctx) error {
	orgID, ok := middleware.OrganizationIDFromCtx(c)
	if !ok {
		return response.BadRequest(c, "NO_ORGANIZATION_CONTEXT", "Organization context is required")
	}
	var req UpdateCandidateRequest
	if err := c.Bind().JSON(&req); err != nil {
		return response.BadRequest(c, "INVALID_BODY", "Invalid request body")
	}
	cand, err := h.service.UpdateCandidate(c.Context(), orgID, c.Params("candidateId"), req)
	if err != nil {
		return h.err(c, err)
	}
	return response.OK(c, fiber.Map{"candidate": cand}, "Candidate updated")
}

// DeleteCandidate godoc
//
//	@Summary		Delete candidate
//	@Tags			HRM / Recruitment
//	@Produce		json
//	@Security		BearerAuth
//	@Param			orgId		path	string	true	"Organization ID"
//	@Param			candidateId	path	string	true	"Candidate public ID"
//	@Success		204
//	@Router			/organizations/{orgId}/hrm/recruitment/candidates/{candidateId} [delete]
func (h *Handler) DeleteCandidate(c fiber.Ctx) error {
	orgID, ok := middleware.OrganizationIDFromCtx(c)
	if !ok {
		return response.BadRequest(c, "NO_ORGANIZATION_CONTEXT", "Organization context is required")
	}
	if err := h.service.DeleteCandidate(c.Context(), orgID, c.Params("candidateId")); err != nil {
		return h.err(c, err)
	}
	return response.NoContent(c)
}

// UploadResume godoc
//
//	@Summary		Upload a candidate's resume (PDF only)
//	@Tags			HRM / Recruitment
//	@Accept			multipart/form-data
//	@Produce		json
//	@Security		BearerAuth
//	@Param			orgId		path	string	true	"Organization ID"
//	@Param			candidateId	path	string	true	"Candidate public ID"
//	@Param			resume		formData	file	true	"PDF file"
//	@Success		200			{object}	response.OK{data=object{candidate=Candidate}}
//	@Failure		400			{object}	response.Error	"INVALID_RESUME_TYPE or RESUME_TOO_LARGE"
//	@Router			/organizations/{orgId}/hrm/recruitment/candidates/{candidateId}/resume [post]
func (h *Handler) UploadResume(c fiber.Ctx) error {
	orgID, ok := middleware.OrganizationIDFromCtx(c)
	if !ok {
		return response.BadRequest(c, "NO_ORGANIZATION_CONTEXT", "Organization context is required")
	}
	fileHeader, err := c.FormFile("resume")
	if err != nil {
		return response.BadRequest(c, "FILE_REQUIRED", "resume file is required")
	}
	if fileHeader.Size > resumeMaxUploadBytes {
		return response.BadRequest(c, "RESUME_TOO_LARGE", "Resume file exceeds the size limit")
	}
	f, err := fileHeader.Open()
	if err != nil {
		return response.BadRequest(c, "INVALID_FILE", "Could not read uploaded file")
	}
	defer f.Close()
	raw := make([]byte, fileHeader.Size)
	if _, err := io.ReadFull(f, raw); err != nil {
		return response.BadRequest(c, "INVALID_FILE", "Could not read uploaded file")
	}

	cand, err := h.service.UploadResume(c.Context(), orgID, c.Params("candidateId"), raw, fileHeader.Filename)
	if err != nil {
		return h.err(c, err)
	}
	return response.OK(c, fiber.Map{"candidate": cand}, "Resume uploaded")
}

// DownloadResume godoc
//
//	@Summary		Download a candidate's resume
//	@Description	Loads the candidate and verifies org_id before touching the
//	@Description	filesystem — tenant isolation does not depend on path secrecy.
//	@Tags			HRM / Recruitment
//	@Produce		application/pdf
//	@Security		BearerAuth
//	@Param			orgId		path	string	true	"Organization ID"
//	@Param			candidateId	path	string	true	"Candidate public ID"
//	@Success		200
//	@Failure		404			{object}	response.Error	"NO_RESUME_ON_FILE"
//	@Router			/organizations/{orgId}/hrm/recruitment/candidates/{candidateId}/resume [get]
func (h *Handler) DownloadResume(c fiber.Ctx) error {
	orgID, ok := middleware.OrganizationIDFromCtx(c)
	if !ok {
		return response.BadRequest(c, "NO_ORGANIZATION_CONTEXT", "Organization context is required")
	}
	cand, diskPath, err := h.service.GetResumeFile(c.Context(), orgID, c.Params("candidateId"))
	if err != nil {
		return h.err(c, err)
	}
	fileName := "resume.pdf"
	if cand.ResumeFileName != nil && *cand.ResumeFileName != "" {
		fileName = *cand.ResumeFileName
	}
	c.Set("Content-Disposition", `attachment; filename="`+fileName+`"`)
	c.Set("Content-Type", "application/pdf")
	return c.SendFile(diskPath)
}
