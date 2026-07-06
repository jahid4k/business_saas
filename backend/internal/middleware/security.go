// backend/internal/middleware/security.go
package middleware

import "github.com/gofiber/fiber/v3"

// SecurityHeaders sets a fixed set of browser-security response headers
// on every request. These are defense-in-depth: they don't replace CORS,
// input validation, or auth — they reduce the blast radius of certain
// client-side attack classes (clickjacking, MIME-sniffing, referrer
// leakage, and unwanted browser feature access) if something else fails.
//
// This API is JSON + static file only (avatars under /uploads) — no HTML
// is ever rendered by this server. That makes a strict, near-default-deny
// Content-Security-Policy safe here: there is no inline script or
// stylesheet to break.
//
// isProduction gates Strict-Transport-Security only. HSTS tells the
// browser "always use HTTPS for this host, even if the user types http://"
// — sending that in local development would be actively wrong (dev runs
// on plain HTTP), so it must never fire outside production.
func SecurityHeaders(isProduction bool) fiber.Handler {
	return func(c fiber.Ctx) error {
		// Prevents the browser from guessing ("sniffing") a response's
		// MIME type. Without this, a maliciously crafted file uploaded as
		// an "avatar" could be served and interpreted as HTML/JS by a
		// browser that navigates to it directly.
		c.Set("X-Content-Type-Options", "nosniff")

		// Blocks this API's responses from being rendered inside a
		// <frame>/<iframe> on another site — classic clickjacking defense.
		// Kept even though we serve no HTML today, in case a future route
		// (e.g. an embedded widget) is added without revisiting this file.
		c.Set("X-Frame-Options", "DENY")

		// Sends only the origin (not the full URL with path/query) as the
		// Referer header when a request leaves this API to a third party,
		// and sends nothing at all when downgrading from HTTPS to HTTP.
		c.Set("Referrer-Policy", "strict-origin-when-cross-origin")

		// This server has no legitimate use for the browser's camera,
		// microphone, geolocation, or payment APIs. Explicitly denying
		// them means an XSS elsewhere in the browser context can't abuse
		// them via a page that embeds something from this origin.
		c.Set("Permissions-Policy", "camera=(), microphone=(), geolocation=(), payment=(), usb=()")

		// Near-default-deny CSP. Safe because this server never renders
		// HTML — default-src 'none' would break inline scripts/styles,
		// but there are none to break. If a future route ever serves
		// HTML (e.g. a Swagger UI page), that route needs its own,
		// looser CSP — don't loosen this global one for it.
		c.Set("Content-Security-Policy", "default-src 'none'; frame-ancestors 'none'")

		// HSTS: only in production, and only because this app is only
		// ever deployed behind TLS in production (see cfg.App.IsProduction
		// gating cookie Secure flag — same reasoning applies here).
		if isProduction {
			c.Set("Strict-Transport-Security", "max-age=31536000; includeSubDomains")
		}

		return c.Next()
	}
}
