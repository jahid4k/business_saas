// backend/internal/hrm/assets/routes.go
package assets

import "github.com/gofiber/fiber/v3"

// PermissionFunc is a factory that returns permission-enforcing middleware.
// Redeclared per package to break the package <-> middleware import cycle.
type PermissionFunc func(permission string) fiber.Handler

// RegisterRoutes mounts every HRM asset route.
//
// Conventions enforced by internal/tests/unit/architecture:
//
//   - Every registration carries a permFn("hrm....") argument whose value is
//     an INLINE STRING LITERAL — TestPermissions_AllRoutesProtected reads
//     Args[0].(*ast.BasicLit), so a named constant compiles and fails.
//   - `categories`, `licenses`, `assetsGroup` and `requests` are separate
//     group variables — TestRouting_NoDuplicates normalizes every ":x" to
//     ":param" and keys on the receiver, so /:assetId and /:licenseId sharing
//     one group would collide.
//   - Every permFn string appears as the first element of an INSERT tuple in
//     migration 00107 — TestPermissions_UsedStringsExistInMigrations.
//
// .assign gates handing an asset over and taking it back; .manage gates
// editing the asset catalog. Deliberately separate authorities — an org may
// let IT assign hardware without letting them create or retire records, the
// hrm.loans.disburse / hrm.loans.manage split (00101).
//
// .request is granted through 'member' and cannot express "for yourself
// only"; Service.RequestAsset narrows it by resolving the caller's own
// employeeID. The hrm.goals.set_own precedent.
//
// Deciding a submitted request's approval instance goes through
// hrm.approvals.action, not a route here.
//
//	Categories  GET/POST  /organizations/:orgId/hrm/asset-categories
//	Licences    GET/POST  /organizations/:orgId/hrm/software-licenses
//	            GET       .../software-licenses/:licenseId
//	            GET/POST  .../software-licenses/:licenseId/seats
//	            DELETE    .../software-licenses/:licenseId/seats/:employeeId
//	Assets      GET/POST  /organizations/:orgId/hrm/assets
//	            GET       .../assets/:assetId
//	            POST      .../assets/:assetId/{assign,return,maintenance}
//	            GET       .../assets/:assetId/maintenance
//	            GET       /organizations/:orgId/hrm/asset-assignments
//	Requests    GET/POST  /organizations/:orgId/hrm/asset-requests
//	            GET       .../asset-requests/:requestId
//	            POST      .../asset-requests/:requestId/{submit,fulfill}
func RegisterRoutes(
	router fiber.Router,
	handler *Handler,
	permFn PermissionFunc,
	requireAuth fiber.Handler,
	requireOrgMatch fiber.Handler,
) {
	// ── Catalog: categories (hrm.asset_config, untiered) ────────────────
	categories := router.Group("/organizations/:orgId/hrm/asset-categories", requireAuth, requireOrgMatch)
	categories.Get("", permFn("hrm.asset_config.view"), handler.ListCategories)
	categories.Post("", permFn("hrm.asset_config.manage"), handler.CreateCategory)

	// ── Catalog: software licences (hrm.asset_config, untiered) ─────────
	// Seat assignment is .manage rather than hrm.assets.assign: a licence
	// seat is catalog capacity, not a physical handover, and there is no
	// condition/sign-off involved.
	licenses := router.Group("/organizations/:orgId/hrm/software-licenses", requireAuth, requireOrgMatch)
	licenses.Get("", permFn("hrm.asset_config.view"), handler.ListLicenses)
	licenses.Post("", permFn("hrm.asset_config.manage"), handler.CreateLicense)
	licenses.Get("/:licenseId", permFn("hrm.asset_config.view"), handler.GetLicense)
	licenses.Get("/:licenseId/seats", permFn("hrm.asset_config.view"), handler.ListSeats)
	licenses.Post("/:licenseId/seats", permFn("hrm.asset_config.manage"), handler.AssignSeat)
	licenses.Delete("/:licenseId/seats/:employeeId", permFn("hrm.asset_config.manage"), handler.ReleaseSeat)

	// ── Assets (hrm.assets, scope-tiered) ───────────────────────────────
	assetsGroup := router.Group("/organizations/:orgId/hrm/assets", requireAuth, requireOrgMatch)
	assetsGroup.Get("", permFn("hrm.assets.view"), handler.ListAssets)
	assetsGroup.Post("", permFn("hrm.assets.manage"), handler.CreateAsset)
	assetsGroup.Get("/:assetId", permFn("hrm.assets.view"), handler.GetAsset)
	assetsGroup.Post("/:assetId/assign", permFn("hrm.assets.assign"), handler.AssignAsset)
	assetsGroup.Post("/:assetId/return", permFn("hrm.assets.assign"), handler.ReturnAsset)
	assetsGroup.Get("/:assetId/maintenance", permFn("hrm.assets.view"), handler.ListMaintenance)
	assetsGroup.Post("/:assetId/maintenance", permFn("hrm.assets.manage"), handler.AddMaintenance)

	// Own group variable: /:assetId and the bare assignment list would
	// otherwise collide under TestRouting_NoDuplicates.
	assignments := router.Group("/organizations/:orgId/hrm/asset-assignments", requireAuth, requireOrgMatch)
	assignments.Get("", permFn("hrm.assets.view"), handler.ListAssignments)

	// ── Requests (hrm.assets, scope-tiered) ─────────────────────────────
	requests := router.Group("/organizations/:orgId/hrm/asset-requests", requireAuth, requireOrgMatch)
	requests.Get("", permFn("hrm.assets.view"), handler.ListRequests)
	requests.Post("", permFn("hrm.assets.request"), handler.RequestAsset)
	requests.Get("/:requestId", permFn("hrm.assets.view"), handler.GetRequest)
	requests.Post("/:requestId/submit", permFn("hrm.assets.request"), handler.SubmitRequest)
	requests.Post("/:requestId/fulfill", permFn("hrm.assets.assign"), handler.FulfillRequest)
}
