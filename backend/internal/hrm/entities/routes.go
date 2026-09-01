// backend/internal/hrm/entities/routes.go
package entities

import "github.com/gofiber/fiber/v3"

// PermissionFunc is a factory returning permission-enforcing middleware.
type PermissionFunc func(permission string) fiber.Handler

// RegisterRoutes mounts the legal-entity layer under three base paths.
//
// Legal entities:
//
//	GET    /organizations/:orgId/hrm/legal-entities                 <- hrm.entities.view
//	       ?active=false to include retired entities
//	POST   /organizations/:orgId/hrm/legal-entities                 <- hrm.entities.manage
//	GET    /organizations/:orgId/hrm/legal-entities/context         <- hrm.entities.view
//	       ?legal_entity_id=<id> — the RESOLVED country/currency/timezone
//	GET    /organizations/:orgId/hrm/legal-entities/:entityId       <- hrm.entities.view
//	PATCH  /organizations/:orgId/hrm/legal-entities/:entityId       <- hrm.entities.manage
//
// Country configurations:
//
//	GET    /organizations/:orgId/hrm/country-configs                <- hrm.entities.view
//	POST   /organizations/:orgId/hrm/country-configs                <- hrm.entities.manage
//	       (upsert by country_code — two configs for one country would be two
//	        answers to the same question)
//
// Locations:
//
//	GET    /organizations/:orgId/hrm/locations                      <- hrm.locations.view
//	       ?legal_entity_id=<id>&active=false
//	POST   /organizations/:orgId/hrm/locations                      <- hrm.locations.manage
//	PATCH  /organizations/:orgId/hrm/locations/:locationId          <- hrm.locations.manage
//
// ⚠ NEITHER RESOURCE IS SCOPE-TIERED. A legal entity and a work site are org
// STRUCTURE, like departments and positions — there is no "your own legal
// entity". No ResolveScope call exists in this package.
//
// ⚠ These keys govern the entity RECORDS, not what data an entity's members
// can see. Entity scoping of employee and payroll data is 11B, and it arrives
// as a LegalEntityFilter applied alongside the existing own/team/all tier
// rather than as a fourth tier (decided r38) — a fourth tier would force
// every scope-tiered resource in HRM to seed a new key or trip
// TestPermissions_ScopeTiersSeeded.
//
// ⚠ /context is registered BEFORE /:entityId. Fiber would otherwise match
// "context" as an entity id and return a 404 for the one endpoint that is
// supposed to work when no entity exists at all.
func RegisterRoutes(
	router fiber.Router,
	handler *Handler,
	permFn PermissionFunc,
	requireAuth fiber.Handler,
	requireOrgMatch fiber.Handler,
) {
	ents := router.Group("/organizations/:orgId/hrm/legal-entities", requireAuth, requireOrgMatch)

	ctxGroup := ents.Group("/context")
	ctxGroup.Get("", permFn("hrm.entities.view"), handler.ResolveContext)

	ents.Get("", permFn("hrm.entities.view"), handler.ListEntities)
	ents.Post("", permFn("hrm.entities.manage"), handler.CreateEntity)
	ents.Get("/:entityId", permFn("hrm.entities.view"), handler.GetEntity)
	ents.Patch("/:entityId", permFn("hrm.entities.manage"), handler.UpdateEntity)

	configs := router.Group("/organizations/:orgId/hrm/country-configs", requireAuth, requireOrgMatch)
	configs.Get("", permFn("hrm.entities.view"), handler.ListCountryConfigs)
	configs.Post("", permFn("hrm.entities.manage"), handler.UpsertCountryConfig)

	locs := router.Group("/organizations/:orgId/hrm/locations", requireAuth, requireOrgMatch)
	locs.Get("", permFn("hrm.locations.view"), handler.ListLocations)
	locs.Post("", permFn("hrm.locations.manage"), handler.CreateLocation)
	locs.Patch("/:locationId", permFn("hrm.locations.manage"), handler.UpdateLocation)
}
