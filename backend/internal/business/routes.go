// backend/internal/business/routes.go
package business

import "github.com/gofiber/fiber/v3"

// RegisterRoutes mounts all business routes onto the given Fiber router group.
//
// requireAuth is injected from main.go — same wired JWT manager everywhere.
//
// All business routes require a valid JWT.
// The /switch endpoint is how a user enters a business context — it is
// intentionally NOT behind RequireBusiness() because the user may not
// have a business_id in their token yet.
//
//	POST /api/v1/businesses              ← JWT required
//	GET  /api/v1/businesses              ← JWT required
//	GET  /api/v1/businesses/:id          ← JWT required
//	POST /api/v1/businesses/:id/switch   ← JWT required
func RegisterRoutes(router fiber.Router, handler *Handler, requireAuth fiber.Handler) {
	businesses := router.Group("/businesses", requireAuth)

	businesses.Post("", handler.Create)
	businesses.Get("", handler.List)
	businesses.Get("/:id", handler.Get)
	businesses.Post("/:id/switch", handler.Switch)
}
