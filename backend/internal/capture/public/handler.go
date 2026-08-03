package public

import (
	"log/slog"

	"github.com/gofiber/fiber/v3"

	"github.com/mridha/businesssaas/internal/crm/leads"
	"github.com/mridha/businesssaas/pkg/logger"
	"github.com/mridha/businesssaas/pkg/response"
)

type Handler struct {
	leadsSvc leads.Service
}

func NewHandler(leadsSvc leads.Service) *Handler {
	return &Handler{leadsSvc: leadsSvc}
}

// CaptureLead handles incoming web form submissions.
// It maps standard fields and stores unknown fields in CustomFields.
func (h *Handler) CaptureLead(c fiber.Ctx) error {
	log := logger.FromCtx(c)
	orgID, ok := c.Locals("org_id").(string)
	if !ok || orgID == "" {
		return response.Unauthorized(c, "UNAUTHORIZED", "Missing org identifier")
	}

	var raw map[string]any
	if err := c.Bind().Body(&raw); err != nil {
		return response.BadRequest(c, "BAD_REQUEST", "Invalid JSON payload")
	}

	var req leads.CreateLeadRequest
	req.CustomFields = make(map[string]any)
	source := "website"
	req.CaptureSource = &source
	req.Source = &source

	// Map raw fields
	for k, v := range raw {
		switch k {
		case "first_name":
			if s, ok := v.(string); ok {
				req.FirstName = s
			}
		case "last_name":
			if s, ok := v.(string); ok {
				req.LastName = &s
			}
		case "email":
			if s, ok := v.(string); ok {
				req.Email = &s
			}
		case "phone":
			if s, ok := v.(string); ok {
				req.Phone = &s
			}
		case "company_name":
			if s, ok := v.(string); ok {
				req.CompanyName = &s
			}
		case "title":
			if s, ok := v.(string); ok {
				req.Title = &s
			}
		default:
			// Store everything else in custom fields
			req.CustomFields[k] = v
		}
	}

	if req.FirstName == "" && req.Email != nil {
		// Attempt to use part of email as first name if missing, since it's required
		req.FirstName = "Unknown"
	}

	// crm_leads.created_by is a NOT NULL FK to users.id. For public-form captures there's
	// no logged-in user, so RequireAPIKey (see middleware/apikey.go) sets "user_id" to the
	// API key's creator — the org member who generated the key stands in as the actor.
	userID, _ := c.Locals("user_id").(string)

	// Capture Metadata (IP, User-Agent)
	req.CaptureMetadata = map[string]any{
		"ip":         c.IP(),
		"user_agent": string(c.Request().Header.UserAgent()),
	}

	lead, err := h.leadsSvc.CreateLead(c.Context(), orgID, userID, req)
	if err != nil {
		log.Error("public: CaptureLead", slog.Any("error", err))
		return response.InternalServerError(c)
	}

	// We don't want to leak internal lead details to public endpoints, just a success response.
	return response.Created(c, map[string]string{"id": lead.PublicID}, "Lead captured successfully")
}
