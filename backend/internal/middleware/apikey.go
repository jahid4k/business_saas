package middleware

import (
	"context"
	"strings"

	"github.com/gofiber/fiber/v3"
	
	"github.com/mridha/businesssaas/internal/capture/apikeys"
	"github.com/mridha/businesssaas/pkg/response"
)

// RequireAPIKey requires a valid API key with the given scope.
func RequireAPIKey(apiKeySvc apikeys.Service, requiredScope apikeys.Scope) fiber.Handler {
	return func(c fiber.Ctx) error {
		rawKey := c.Get("X-API-Key")
		if rawKey == "" {
			return response.Unauthorized(c, "UNAUTHORIZED", "missing X-API-Key header")
		}

		key, err := apiKeySvc.ValidateKey(c.Context(), rawKey)
		if err != nil {
			if err == apikeys.ErrKeyRevoked || err == apikeys.ErrKeyNotFound || err == apikeys.ErrKeyExpired {
				return response.Unauthorized(c, "UNAUTHORIZED", "invalid or expired api key")
			}
			return response.InternalServerError(c)
		}

		// Check scope
		hasScope := false
		for _, s := range key.Scopes {
			if s == requiredScope {
				hasScope = true
				break
			}
		}

		if !hasScope {
			return response.Forbidden(c, "FORBIDDEN", "api key does not have required scope")
		}

		// Ensure origin if allowed_origins is not empty
		origin := c.Get("Origin")
		if len(key.AllowedOrigins) > 0 && origin != "" {
			originAllowed := false
			for _, o := range key.AllowedOrigins {
				if o == "*" || strings.EqualFold(o, origin) {
					originAllowed = true
					break
				}
			}
			if !originAllowed {
				return response.Forbidden(c, "FORBIDDEN", "origin not allowed by this api key")
			}
		}

		// Update last_used_at in a non-blocking way
		go func(keyID string) {
			_ = apiKeySvc.UpdateLastUsed(context.Background(), keyID)
		}(key.ID)

		// Set org_id and user_id in locals (parallel to what RequireAuth sets from JWT)
		c.Locals("org_id", key.OrgID)
		c.Locals("user_id", key.CreatedBy)
		return c.Next()
	}
}
