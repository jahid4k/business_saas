// backend/internal/hrm/recruitment/hire_handler.go
package recruitment

import (
	"github.com/gofiber/fiber/v3"

	"github.com/mridha/businesssaas/internal/middleware"
	"github.com/mridha/businesssaas/pkg/response"
)

// HireApplication godoc
//
//	@Summary		Convert a hired application into an employee record
//	@Description	Requires the application to already be in the "hired" status
//	@Description	(reached via the existing move-stage action). Atomic for the
//	@Description	employee insert, the application's converted_employee_id, and
//	@Description	the requisition's filled_count.
//	@Tags			HRM / Recruitment
//	@Accept			json
//	@Produce		json
//	@Security		BearerAuth
//	@Param			orgId			path	string					true	"Organization ID"
//	@Param			applicationId	path	string					true	"Application public ID"
//	@Param			body			body	HireApplicationRequest	true	"Optional overrides"
//	@Success		200				{object}	response.OK{data=HireApplicationResponse}
//	@Failure		409				{object}	response.Error	"APPLICATION_NOT_HIRED or APPLICATION_ALREADY_HIRED"
//	@Router			/organizations/{orgId}/hrm/recruitment/applications/{applicationId}/hire [post]
func (h *Handler) HireApplication(c fiber.Ctx) error {
	userID, ok := middleware.UserIDFromCtx(c)
	if !ok {
		return response.Unauthorized(c, "UNAUTHORIZED", "Authentication required")
	}
	orgID, ok := middleware.OrganizationIDFromCtx(c)
	if !ok {
		return response.BadRequest(c, "NO_ORGANIZATION_CONTEXT", "Organization context is required")
	}
	var req HireApplicationRequest
	if err := c.Bind().JSON(&req); err != nil {
		return response.BadRequest(c, "INVALID_BODY", "Invalid request body")
	}
	res, err := h.service.HireApplication(c.Context(), orgID, c.Params("applicationId"), userID, req)
	if err != nil {
		return h.err(c, err)
	}
	return response.OK(c, res, "Application converted to employee")
}
