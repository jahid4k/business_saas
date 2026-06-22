// pkg/pagination/pagination.go
package pagination

import (
	"strconv"

	"github.com/gofiber/fiber/v3"
)

const (
	DefaultLimit = 50
	MaxLimit     = 200
)

// Params carries validated page/size values extracted from a request.
// Offset-based pagination is sufficient for Phase 1; switch to cursor later.
type Params struct {
	Limit  int
	Offset int
}

// Page returns the 1-based page number implied by Limit + Offset.
func (p Params) Page() int {
	if p.Limit == 0 {
		return 1
	}
	return p.Offset/p.Limit + 1
}

// FromCtx reads ?limit= and ?offset= query params from a Fiber context.
// Both default to sane values and are clamped to safe ranges.
func FromCtx(c fiber.Ctx) Params {
	limit := parseIntParam(c.Query("limit"), DefaultLimit)
	if limit <= 0 {
		limit = DefaultLimit
	}
	if limit > MaxLimit {
		limit = MaxLimit
	}

	offset := parseIntParam(c.Query("offset"), 0)
	if offset < 0 {
		offset = 0
	}

	return Params{Limit: limit, Offset: offset}
}

func parseIntParam(s string, fallback int) int {
	if s == "" {
		return fallback
	}
	v, err := strconv.Atoi(s)
	if err != nil {
		return fallback
	}
	return v
}

// Meta is included in every paginated list response.
type Meta struct {
	Total  int `json:"total"`
	Limit  int `json:"limit"`
	Offset int `json:"offset"`
}

// NewMeta builds a Meta from pagination params and the true row count.
func NewMeta(p Params, total int) Meta {
	return Meta{Total: total, Limit: p.Limit, Offset: p.Offset}
}
