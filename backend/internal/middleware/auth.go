package middleware

import "github.com/gofiber/fiber/v3"

// RequireAuth validates the JWT access token in the Authorization: Bearer header.
//
// On success it sets in c.Locals:
//   - "user_id"      string — authenticated user's UUID
//   - "business_id"  string — active business UUID (if embedded in token)
//
// On failure it returns 401 Unauthorized.
//
// STATUS: Phase 1-B stub — calls c.Next() so routes compile and respond.
// Real JWT validation wired in Phase 1-B.
func RequireAuth() fiber.Handler {
	return func(c fiber.Ctx) error {
		// TODO (Phase 1-B): implement JWT extraction and validation
		// 1. Extract "Authorization: Bearer <token>" header
		// 2. Call jwtPkg.Parse(token, secret)
		// 3. Validate claims (expiry, issuer)
		// 4. Set c.Locals("user_id", claims.UserID)
		// 5. Set c.Locals("business_id", claims.BusinessID)
		// 6. Return response.Unauthorized(...) on any failure
		return c.Next()
	}
}

// RequireBusiness validates that the user is an active member of the
// business embedded in their JWT. Must run after RequireAuth.
//
// STATUS: Phase 1-C stub — calls c.Next() so routes compile and respond.
func RequireBusiness() fiber.Handler {
	return func(c fiber.Ctx) error {
		// TODO (Phase 1-C): validate membership
		// 1. Extract business_id from c.Locals("business_id")
		// 2. Return 400 if missing (no business selected)
		// 3. Call authzRepo.GetMembership(ctx, userID, businessID)
		// 4. Return 403 if not a member or membership is inactive
		return c.Next()
	}
}
