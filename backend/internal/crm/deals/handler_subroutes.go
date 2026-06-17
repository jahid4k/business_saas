// backend/internal/crm/deals/handler_subroutes.go
// Extra handler methods needed by the CRM routes sub-resource adapters.
package deals

import (
	"log/slog"

	"github.com/gofiber/fiber/v3"

	"github.com/mridha/businesssaas/pkg/response"
)

// GetDealsByContact handles GET /crm/contacts/:contactId/deals
func (h *Handler) GetDealsByContact(c fiber.Ctx) error {
	result, err := h.service.GetDealsByContact(c.Context(), orgID(c), c.Params("contactId"))
	if err != nil {
		slog.Error("deals: GetDealsByContact", slog.Any("error", err))
		return response.InternalServerError(c)
	}
	return response.OK(c, fiber.Map{"deals": result}, "OK")
}

// GetDealsByCompany handles GET /crm/companies/:companyId/deals
func (h *Handler) GetDealsByCompany(c fiber.Ctx) error {
	result, err := h.service.GetDealsByCompany(c.Context(), orgID(c), c.Params("companyId"))
	if err != nil {
		slog.Error("deals: GetDealsByCompany", slog.Any("error", err))
		return response.InternalServerError(c)
	}
	return response.OK(c, fiber.Map{"deals": result}, "OK")
}
