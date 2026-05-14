package business

import (
	"github.com/gofiber/fiber/v3"

	"github.com/mridha/businesssaas/pkg/response"
)

// Handler handles business/workspace endpoints.
type Handler struct {
	service Service
}

// NewHandler creates a new business Handler.
func NewHandler(service Service) *Handler {
	return &Handler{service: service}
}

// Create handles POST /api/v1/businesses
// STATUS: Phase 1-C stub.
func (h *Handler) Create(c fiber.Ctx) error {
	return response.NotImplemented(c)
}

// List handles GET /api/v1/businesses
// Lists all businesses the authenticated user belongs to.
// STATUS: Phase 1-C stub.
func (h *Handler) List(c fiber.Ctx) error {
	return response.NotImplemented(c)
}

// Get handles GET /api/v1/businesses/:id
// STATUS: Phase 1-C stub.
func (h *Handler) Get(c fiber.Ctx) error {
	return response.NotImplemented(c)
}

// Switch handles POST /api/v1/businesses/:id/switch
// Issues a new JWT with the selected business_id embedded.
// STATUS: Phase 1-C stub.
func (h *Handler) Switch(c fiber.Ctx) error {
	return response.NotImplemented(c)
}
