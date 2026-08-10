// backend/internal/hrm/certifications/handler.go
package certifications

import (
	"errors"
	"strconv"

	"github.com/gofiber/fiber/v3"

	"github.com/mridha/businesssaas/internal/authz"
	"github.com/mridha/businesssaas/internal/middleware"
	"github.com/mridha/businesssaas/pkg/logger"
	"github.com/mridha/businesssaas/pkg/response"
)

// Handler handles HRM certification HTTP endpoints.
type Handler struct {
	service Service
	authz   authz.Service
}

func NewHandler(service Service, authzSvc authz.Service) *Handler {
	return &Handler{service: service, authz: authzSvc}
}

var errUnauthenticated = errors.New("authentication required")

func requestOrg(c fiber.Ctx) (string, bool) {
	return middleware.OrganizationIDFromCtx(c)
}

func (h *Handler) resolveCaller(c fiber.Ctx, orgID string) (Caller, error) {
	userID, ok := middleware.UserIDFromCtx(c)
	if !ok {
		return Caller{}, errUnauthenticated
	}
	tier, err := h.authz.ResolveScope(c.Context(), userID, orgID, "hrm.certifications")
	if err != nil {
		return Caller{}, err
	}
	canManage, err := h.authz.Can(c.Context(), userID, orgID, "hrm.certifications", "manage")
	if err != nil {
		return Caller{}, err
	}
	return Caller{UserID: userID, Tier: tier, CanManage: canManage}, nil
}

func atoiOr(s string, fallback int) int {
	if s == "" {
		return fallback
	}
	n, err := strconv.Atoi(s)
	if err != nil {
		return fallback
	}
	return n
}

func boolQuery(c fiber.Ctx, key string) *bool {
	v := c.Query(key)
	if v == "" {
		return nil
	}
	b := v == "true" || v == "1"
	return &b
}

func (h *Handler) err(c fiber.Ctx, err error) error {
	log := logger.FromCtx(c)
	switch {
	case errors.Is(err, ErrNotFound):
		return response.NotFound(c, "CERTIFICATION_NOT_FOUND", "Certification not found")
	case errors.Is(err, ErrEmployeeCertNotFound):
		return response.NotFound(c, "EMPLOYEE_CERTIFICATION_NOT_FOUND", "Employee certification not found")
	case errors.Is(err, ErrEmployeeNotFound):
		return response.NotFound(c, "EMPLOYEE_NOT_FOUND", "Employee not found in this organization")

	case errors.Is(err, ErrAccessDenied):
		return response.Forbidden(c, "CERTIFICATION_ACCESS_DENIED", err.Error())
	case errors.Is(err, errUnauthenticated):
		return response.Unauthorized(c, "UNAUTHORIZED", "Authentication required")

	case errors.Is(err, ErrNameTaken):
		return response.Conflict(c, "CERTIFICATION_NAME_TAKEN", err.Error())
	case errors.Is(err, ErrAlreadyHeld):
		return response.Conflict(c, "CERTIFICATION_ALREADY_HELD", err.Error())
	case errors.Is(err, ErrAlreadyRevoked):
		return response.Conflict(c, "CERTIFICATION_ALREADY_REVOKED", err.Error())
	case errors.Is(err, ErrCertInUse):
		return response.Conflict(c, "CERTIFICATION_IN_USE", err.Error())
	case errors.Is(err, ErrCertInactive):
		return response.Conflict(c, "CERTIFICATION_INACTIVE", err.Error())

	case errors.Is(err, ErrNameRequired):
		return response.BadRequest(c, "NAME_REQUIRED", err.Error())
	case errors.Is(err, ErrInvalidDate):
		return response.BadRequest(c, "INVALID_DATE", err.Error())
	case errors.Is(err, ErrExpiryBeforeIssue):
		return response.BadRequest(c, "EXPIRY_BEFORE_ISSUE", err.Error())
	case errors.Is(err, ErrInvalidValidity):
		return response.BadRequest(c, "INVALID_VALIDITY_MONTHS", err.Error())
	}

	log.Error("certifications error", "error", err)
	return response.InternalServerError(c)
}

// List godoc
//
//	@Summary	List the certification catalogue
//	@Tags		HRM - Certifications
//	@Security	BearerAuth
//	@Param		orgId	path		string	true	"Organization ID"
//	@Success	200		{object}	response.Response
//	@Router		/organizations/{orgId}/hrm/certifications [get]
func (h *Handler) List(c fiber.Ctx) error {
	orgID, ok := requestOrg(c)
	if !ok {
		return response.BadRequest(c, "NO_ORGANIZATION_CONTEXT", "Organization context is required")
	}
	res, err := h.service.List(c.Context(), orgID, CertificationListFilter{
		IsActive: boolQuery(c, "is_active"),
		Search:   c.Query("search"),
		Limit:    atoiOr(c.Query("limit"), 0),
		Offset:   atoiOr(c.Query("offset"), 0),
	})
	if err != nil {
		return h.err(c, err)
	}
	return response.OK(c, res, "Certifications retrieved")
}

// Get godoc
//
//	@Summary	Get a certification
//	@Tags		HRM - Certifications
//	@Security	BearerAuth
//	@Param		orgId			path		string	true	"Organization ID"
//	@Param		certificationId	path		string	true	"Certification ID"
//	@Success	200				{object}	response.Response
//	@Router		/organizations/{orgId}/hrm/certifications/{certificationId} [get]
func (h *Handler) Get(c fiber.Ctx) error {
	orgID, ok := requestOrg(c)
	if !ok {
		return response.BadRequest(c, "NO_ORGANIZATION_CONTEXT", "Organization context is required")
	}
	out, err := h.service.Get(c.Context(), orgID, c.Params("certificationId"))
	if err != nil {
		return h.err(c, err)
	}
	return response.OK(c, fiber.Map{"certification": out}, "Certification retrieved")
}

// Create godoc
//
//	@Summary	Add a certification to the catalogue
//	@Tags		HRM - Certifications
//	@Security	BearerAuth
//	@Param		orgId	path		string						true	"Organization ID"
//	@Param		body	body		CreateCertificationRequest	true	"Certification"
//	@Success	201		{object}	response.Response
//	@Router		/organizations/{orgId}/hrm/certifications [post]
func (h *Handler) Create(c fiber.Ctx) error {
	userID, ok := middleware.UserIDFromCtx(c)
	if !ok {
		return response.Unauthorized(c, "UNAUTHORIZED", "Authentication required")
	}
	orgID, ok := requestOrg(c)
	if !ok {
		return response.BadRequest(c, "NO_ORGANIZATION_CONTEXT", "Organization context is required")
	}
	var req CreateCertificationRequest
	if err := c.Bind().JSON(&req); err != nil {
		return response.BadRequest(c, "INVALID_BODY", "Invalid request body")
	}
	out, err := h.service.Create(c.Context(), orgID, userID, req)
	if err != nil {
		return h.err(c, err)
	}
	return response.Created(c, fiber.Map{"certification": out}, "Certification created")
}

// Update godoc
//
//	@Summary		Update a certification
//	@Description	Changing validity_months never moves an already-issued credential's expiry — those are frozen at issue.
//	@Tags			HRM - Certifications
//	@Security		BearerAuth
//	@Param			orgId			path		string						true	"Organization ID"
//	@Param			certificationId	path		string						true	"Certification ID"
//	@Param			body			body		UpdateCertificationRequest	true	"Changes"
//	@Success		200				{object}	response.Response
//	@Router			/organizations/{orgId}/hrm/certifications/{certificationId} [patch]
func (h *Handler) Update(c fiber.Ctx) error {
	orgID, ok := requestOrg(c)
	if !ok {
		return response.BadRequest(c, "NO_ORGANIZATION_CONTEXT", "Organization context is required")
	}
	var req UpdateCertificationRequest
	if err := c.Bind().JSON(&req); err != nil {
		return response.BadRequest(c, "INVALID_BODY", "Invalid request body")
	}
	out, err := h.service.Update(c.Context(), orgID, c.Params("certificationId"), req)
	if err != nil {
		return h.err(c, err)
	}
	return response.OK(c, fiber.Map{"certification": out}, "Certification updated")
}

// Delete godoc
//
//	@Summary	Delete a certification nobody holds
//	@Tags		HRM - Certifications
//	@Security	BearerAuth
//	@Param		orgId			path	string	true	"Organization ID"
//	@Param		certificationId	path	string	true	"Certification ID"
//	@Success	204
//	@Router		/organizations/{orgId}/hrm/certifications/{certificationId} [delete]
func (h *Handler) Delete(c fiber.Ctx) error {
	orgID, ok := requestOrg(c)
	if !ok {
		return response.BadRequest(c, "NO_ORGANIZATION_CONTEXT", "Organization context is required")
	}
	if err := h.service.Delete(c.Context(), orgID, c.Params("certificationId")); err != nil {
		return h.err(c, err)
	}
	return response.NoContent(c)
}

// ListEmployeeCertifications godoc
//
//	@Summary		List employee credentials, filtered by the caller's scope tier
//	@Description	expiring_within_days is the compliance-dashboard query: who needs chasing.
//	@Tags			HRM - Certifications
//	@Security		BearerAuth
//	@Param			orgId					path		string	true	"Organization ID"
//	@Param			employee_id				query		string	false	"Filter by employee"
//	@Param			certification_id		query		string	false	"Filter by certification"
//	@Param			status					query		string	false	"active | expiring | expired | revoked"
//	@Param			expiring_within_days	query		int		false	"Lapsing within N days"
//	@Success		200						{object}	response.Response
//	@Router			/organizations/{orgId}/hrm/employee-certifications [get]
func (h *Handler) ListEmployeeCertifications(c fiber.Ctx) error {
	orgID, ok := requestOrg(c)
	if !ok {
		return response.BadRequest(c, "NO_ORGANIZATION_CONTEXT", "Organization context is required")
	}
	caller, err := h.resolveCaller(c, orgID)
	if err != nil {
		return h.err(c, err)
	}
	res, err := h.service.ListEmployeeCertifications(c.Context(), orgID, caller, EmployeeCertificationListFilter{
		EmployeeID:         c.Query("employee_id"),
		CertificationID:    c.Query("certification_id"),
		Status:             c.Query("status"),
		ExpiringWithinDays: atoiOr(c.Query("expiring_within_days"), 0),
		Limit:              atoiOr(c.Query("limit"), 0),
		Offset:             atoiOr(c.Query("offset"), 0),
	})
	if err != nil {
		return h.err(c, err)
	}
	return response.OK(c, res, "Employee certifications retrieved")
}

// Issue godoc
//
//	@Summary		Issue a credential to an employee
//	@Description	expires_at is derived from the certification's validity_months when omitted, and frozen at issue.
//	@Tags			HRM - Certifications
//	@Security		BearerAuth
//	@Param			orgId	path		string			true	"Organization ID"
//	@Param			body	body		IssueRequest	true	"Credential"
//	@Success		201		{object}	response.Response
//	@Router			/organizations/{orgId}/hrm/employee-certifications [post]
func (h *Handler) Issue(c fiber.Ctx) error {
	orgID, ok := requestOrg(c)
	if !ok {
		return response.BadRequest(c, "NO_ORGANIZATION_CONTEXT", "Organization context is required")
	}
	caller, err := h.resolveCaller(c, orgID)
	if err != nil {
		return h.err(c, err)
	}
	var req IssueRequest
	if err := c.Bind().JSON(&req); err != nil {
		return response.BadRequest(c, "INVALID_BODY", "Invalid request body")
	}
	out, err := h.service.Issue(c.Context(), orgID, caller, req)
	// A credential that issued but whose derived skill failed is a PARTIAL
	// success: reporting it as an error would imply nothing happened.
	if err != nil && out != nil {
		logger.FromCtx(c).Error("certifications: skill grant failed after issue", "error", err)
		return response.Created(c, fiber.Map{"employee_certification": out, "skill_recorded": false},
			"Credential issued, but the linked skill could not be recorded")
	}
	if err != nil {
		return h.err(c, err)
	}
	return response.Created(c, fiber.Map{"employee_certification": out}, "Credential issued")
}

// UpdateEmployeeCertification godoc
//
//	@Summary	Update an issued credential
//	@Tags		HRM - Certifications
//	@Security	BearerAuth
//	@Param		orgId					path		string								true	"Organization ID"
//	@Param		employeeCertificationId	path		string								true	"Employee certification ID"
//	@Param		body					body		UpdateEmployeeCertificationRequest	true	"Changes"
//	@Success	200						{object}	response.Response
//	@Router		/organizations/{orgId}/hrm/employee-certifications/{employeeCertificationId} [patch]
func (h *Handler) UpdateEmployeeCertification(c fiber.Ctx) error {
	orgID, ok := requestOrg(c)
	if !ok {
		return response.BadRequest(c, "NO_ORGANIZATION_CONTEXT", "Organization context is required")
	}
	caller, err := h.resolveCaller(c, orgID)
	if err != nil {
		return h.err(c, err)
	}
	var req UpdateEmployeeCertificationRequest
	if err := c.Bind().JSON(&req); err != nil {
		return response.BadRequest(c, "INVALID_BODY", "Invalid request body")
	}
	out, err := h.service.UpdateEmployeeCertification(c.Context(), orgID,
		c.Params("employeeCertificationId"), caller, req)
	if err != nil {
		return h.err(c, err)
	}
	return response.OK(c, fiber.Map{"employee_certification": out}, "Credential updated")
}

// Revoke godoc
//
//	@Summary		Revoke a credential
//	@Description	Frees the employee to be re-issued, which is what makes the live-credential index partial.
//	@Tags			HRM - Certifications
//	@Security		BearerAuth
//	@Param			orgId					path		string			true	"Organization ID"
//	@Param			employeeCertificationId	path		string			true	"Employee certification ID"
//	@Param			body					body		RevokeRequest	false	"Reason"
//	@Success		200						{object}	response.Response
//	@Router			/organizations/{orgId}/hrm/employee-certifications/{employeeCertificationId}/revoke [post]
func (h *Handler) Revoke(c fiber.Ctx) error {
	orgID, ok := requestOrg(c)
	if !ok {
		return response.BadRequest(c, "NO_ORGANIZATION_CONTEXT", "Organization context is required")
	}
	caller, err := h.resolveCaller(c, orgID)
	if err != nil {
		return h.err(c, err)
	}
	var req RevokeRequest
	if err := c.Bind().JSON(&req); err != nil {
		return response.BadRequest(c, "INVALID_BODY", "Invalid request body")
	}
	out, err := h.service.Revoke(c.Context(), orgID, c.Params("employeeCertificationId"), caller, req)
	if err != nil {
		return h.err(c, err)
	}
	return response.OK(c, fiber.Map{"employee_certification": out}, "Credential revoked")
}
