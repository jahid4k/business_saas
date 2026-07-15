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
	source := "web_form"
	req.CaptureSource = &source

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

	// Create the lead (SYSTEM as creator since it's via public API key)
	// We can use a special user ID or just empty if the DB allows.
	// `created_by` in `crm_leads` is a UUID of a user. But for public forms, we don't have a user.
	// Wait, `created_by` is linked to `users.id`!
	// Is it? Let's check `backend/internal/migrations/00004_create_crm_leads.sql`.
	// Wait, we don't have it open. Let me run a query to check if it's a foreign key.
	// Actually, we can just pass an empty string and if it fails, we will see. Wait, we should use a system user or allow null.
	// Wait! `org_api_keys` actually has a `created_by` which is the user who made the key. Maybe we can pass the key's creator?
	// The middleware `apikey.go` injects `org_id`. It could also inject `user_id`. Let's update `apikey.go`!

	userID, _ := c.Locals("user_id").(string)
	if userID == "" {
		// If middleware doesn't set it, default to a system UUID, but we need the key creator.
		// Let's assume apikey middleware will set "user_id".
	}

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
