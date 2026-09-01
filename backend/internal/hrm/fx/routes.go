// backend/internal/hrm/fx/routes.go
package fx

import "github.com/gofiber/fiber/v3"

// PermissionFunc is a factory returning permission-enforcing middleware.
type PermissionFunc func(permission string) fiber.Handler

// RegisterRoutes mounts exchange rates under
// /organizations/:orgId/hrm/exchange-rates.
//
//	GET    /organizations/:orgId/hrm/exchange-rates              <- hrm.exchange_rates.view
//	       ?from=EUR&to=BDT&limit=200
//	POST   /organizations/:orgId/hrm/exchange-rates              <- hrm.exchange_rates.manage
//	GET    /organizations/:orgId/hrm/exchange-rates/convert      <- hrm.exchange_rates.view
//	       ?amount=100&from=EUR&to=BDT&as_of=YYYY-MM-DD
//
// ⚠ NOT SCOPE-TIERED. A rate is a fact about two currencies on a date; there
// is no "your own USD→EUR rate". No ResolveScope call exists in this package.
//
// ⚠ view IS GRANTED TO EVERY ROLE (00130). A rate is what explains why
// somebody's foreign expense claim converted to the figure it did — an
// employee who can see the claim but not the rate behind it has been handed a
// number they cannot check, which is the failure the
// never-store-converted-only rule exists to prevent.
//
// /convert is a PREVIEW. It resolves and applies a rate without recording
// anything, so a figure can be checked before a claim or settlement freezes
// it. The literal /convert segment is its own group registered before any
// :param route.
func RegisterRoutes(
	router fiber.Router,
	handler *Handler,
	permFn PermissionFunc,
	requireAuth fiber.Handler,
	requireOrgMatch fiber.Handler,
) {
	base := router.Group("/organizations/:orgId/hrm/exchange-rates", requireAuth, requireOrgMatch)

	convert := base.Group("/convert")
	convert.Get("", permFn("hrm.exchange_rates.view"), handler.Convert)

	base.Get("", permFn("hrm.exchange_rates.view"), handler.ListRates)
	base.Post("", permFn("hrm.exchange_rates.manage"), handler.RecordRate)
}
