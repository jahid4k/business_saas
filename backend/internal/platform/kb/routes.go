// backend/internal/platform/kb/routes.go
package kb

import "github.com/gofiber/fiber/v3"

// PermissionFunc is a factory returning permission-enforcing middleware.
// Same pattern as checklists, forms, tickets — breaks the kb ↔ middleware
// import cycle.
type PermissionFunc func(permission string) fiber.Handler

// RegisterRoutes mounts the knowledge base under /organizations/:orgId/kb,
// the platform/checklists prefix shape (requireAuth + requireOrgMatch, no
// module segment, because this is a shared platform primitive).
//
// Categories:
//
//	GET    /organizations/:orgId/kb/categories                    <- platform.kb.view
//	POST   /organizations/:orgId/kb/categories                    <- platform.kb.manage
//	PATCH  /organizations/:orgId/kb/categories/:categoryId        <- platform.kb.manage
//
// Articles:
//
//	GET    /organizations/:orgId/kb/articles                      <- platform.kb.view
//	POST   /organizations/:orgId/kb/articles                      <- platform.kb.manage
//	GET    /organizations/:orgId/kb/articles/:articleId           <- platform.kb.view
//	PATCH  /organizations/:orgId/kb/articles/:articleId           <- platform.kb.manage
//	POST   /organizations/:orgId/kb/articles/:articleId/publish   <- platform.kb.manage
//	POST   /organizations/:orgId/kb/articles/:articleId/archive   <- platform.kb.manage
//
// The read routes gate on .view, and the service then widens what they
// return for a .manage holder — drafts are excluded in SQL, not by the route.
// A separate .view_unpublished key would imply a contributor role this
// product does not have; see migration 00113's header.
//
// GET /articles carries the search: ?q= is full-text over title and body,
// matching the GIN expression index in migration 00112.
func RegisterRoutes(
	router fiber.Router,
	handler *Handler,
	permFn PermissionFunc,
	requireAuth fiber.Handler,
	requireOrgMatch fiber.Handler,
) {
	base := router.Group("/organizations/:orgId/kb", requireAuth, requireOrgMatch)

	categories := base.Group("/categories")
	categories.Get("", permFn("platform.kb.view"), handler.ListCategories)
	categories.Post("", permFn("platform.kb.manage"), handler.CreateCategory)
	categories.Patch("/:categoryId", permFn("platform.kb.manage"), handler.UpdateCategory)

	articles := base.Group("/articles")
	articles.Get("", permFn("platform.kb.view"), handler.ListArticles)
	articles.Post("", permFn("platform.kb.manage"), handler.CreateArticle)
	articles.Get("/:articleId", permFn("platform.kb.view"), handler.GetArticle)
	articles.Patch("/:articleId", permFn("platform.kb.manage"), handler.UpdateArticle)
	articles.Post("/:articleId/publish", permFn("platform.kb.manage"), handler.PublishArticle)
	articles.Post("/:articleId/archive", permFn("platform.kb.manage"), handler.ArchiveArticle)
}
