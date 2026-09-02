// backend/internal/platform/kb/handler.go
package kb

import (
	"errors"
	"log/slog"
	"strconv"
	"strings"

	"github.com/gofiber/fiber/v3"

	"github.com/mridha/businesssaas/internal/middleware"
	"github.com/mridha/businesssaas/pkg/logger"
	"github.com/mridha/businesssaas/pkg/response"
)

// Handler serves the knowledge base's HTTP endpoints. It holds the service
// and nothing else — every access decision belongs to the service, which
// resolves it from the AccessDirectory. The platform/checklists and
// platform/tickets shape.
type Handler struct {
	service Service
}

func NewHandler(service Service) *Handler { return &Handler{service: service} }

func orgID(c fiber.Ctx) string { return c.Params("orgId") }

func callerUserID(c fiber.Ctx) (string, error) {
	id, ok := middleware.UserIDFromCtx(c)
	if !ok {
		return "", errUnauthenticated
	}
	return id, nil
}

var errUnauthenticated = errors.New("unauthenticated")

func mapError(c fiber.Ctx, log *slog.Logger, op string, err error) error {
	switch {
	case errors.Is(err, ErrArticleNotFound):
		return response.NotFound(c, "KB_ARTICLE_NOT_FOUND", "Knowledge base article not found")
	case errors.Is(err, ErrCategoryNotFound):
		return response.NotFound(c, "KB_CATEGORY_NOT_FOUND", "Knowledge base category not found")
	case errors.Is(err, ErrTitleRequired):
		return response.BadRequest(c, "TITLE_REQUIRED", "title is required")
	case errors.Is(err, ErrBodyRequired):
		return response.BadRequest(c, "BODY_REQUIRED", "body is required")
	case errors.Is(err, ErrNameRequired):
		return response.BadRequest(c, "NAME_REQUIRED", "name is required")
	case errors.Is(err, ErrInvalidStatus):
		return response.BadRequest(c, "INVALID_STATUS", "status must be one of draft, published, archived")
	case errors.Is(err, ErrAlreadyPublished):
		return response.Conflict(c, "ALREADY_PUBLISHED", "Article is already published")
	case errors.Is(err, ErrNotPublished):
		return response.Conflict(c, "NOT_PUBLISHED", "Article is not published")
	case errors.Is(err, ErrAccessDenied):
		return response.Forbidden(c, "ACCESS_DENIED", "You do not have access to this resource")
	case errors.Is(err, errUnauthenticated):
		return response.Unauthorized(c, "UNAUTHORIZED", "Authentication required")
	default:
		log.Error("kb: "+op, slog.Any("error", err))
		return response.InternalServerError(c)
	}
}

func atoiOr(raw string, fallback int) int {
	v, err := strconv.Atoi(strings.TrimSpace(raw))
	if err != nil {
		return fallback
	}
	return v
}

// ============================================================
// Categories
// ============================================================

// ListCategories handles GET /organizations/:orgId/kb/categories
func (h *Handler) ListCategories(c fiber.Ctx) error {
	log := logger.FromCtx(c)
	userID, err := callerUserID(c)
	if err != nil {
		return mapError(c, log, "ListCategories", err)
	}
	list, err := h.service.ListCategories(c.Context(), orgID(c), userID, c.Query("active") != "false")
	if err != nil {
		return mapError(c, log, "ListCategories", err)
	}
	return response.OK(c, fiber.Map{"categories": list}, "OK")
}

// CreateCategory handles POST /organizations/:orgId/kb/categories
func (h *Handler) CreateCategory(c fiber.Ctx) error {
	log := logger.FromCtx(c)
	userID, err := callerUserID(c)
	if err != nil {
		return mapError(c, log, "CreateCategory", err)
	}
	var req CreateCategoryRequest
	if err := c.Bind().JSON(&req); err != nil {
		return response.BadRequest(c, "INVALID_BODY", "Invalid request body")
	}
	cat, err := h.service.CreateCategory(c.Context(), orgID(c), userID, req)
	if err != nil {
		return mapError(c, log, "CreateCategory", err)
	}
	return response.Created(c, fiber.Map{"category": cat}, "Knowledge base category created")
}

// UpdateCategory handles PATCH /organizations/:orgId/kb/categories/:categoryId
func (h *Handler) UpdateCategory(c fiber.Ctx) error {
	log := logger.FromCtx(c)
	userID, err := callerUserID(c)
	if err != nil {
		return mapError(c, log, "UpdateCategory", err)
	}
	var req CreateCategoryRequest
	if err := c.Bind().JSON(&req); err != nil {
		return response.BadRequest(c, "INVALID_BODY", "Invalid request body")
	}
	cat, err := h.service.UpdateCategory(c.Context(), orgID(c), userID, c.Params("categoryId"), req)
	if err != nil {
		return mapError(c, log, "UpdateCategory", err)
	}
	return response.OK(c, fiber.Map{"category": cat}, "Knowledge base category updated")
}

// ============================================================
// Articles
// ============================================================

// ListArticles handles GET /organizations/:orgId/kb/articles
func (h *Handler) ListArticles(c fiber.Ctx) error {
	log := logger.FromCtx(c)
	userID, err := callerUserID(c)
	if err != nil {
		return mapError(c, log, "ListArticles", err)
	}
	// IncludeUnpublished is deliberately NOT read from the query — the
	// service sets it from the caller's own platform.kb.manage grant.
	f := ArticleFilter{
		Query:      strings.TrimSpace(c.Query("q")),
		CategoryID: strings.TrimSpace(c.Query("category_id")),
		Status:     strings.TrimSpace(c.Query("status")),
		Limit:      atoiOr(c.Query("limit"), 0),
		Offset:     atoiOr(c.Query("offset"), 0),
	}
	res, err := h.service.ListArticles(c.Context(), orgID(c), userID, f)
	if err != nil {
		return mapError(c, log, "ListArticles", err)
	}
	return response.OK(c, res, "OK")
}

// CreateArticle handles POST /organizations/:orgId/kb/articles
func (h *Handler) CreateArticle(c fiber.Ctx) error {
	log := logger.FromCtx(c)
	userID, err := callerUserID(c)
	if err != nil {
		return mapError(c, log, "CreateArticle", err)
	}
	var req CreateArticleRequest
	if err := c.Bind().JSON(&req); err != nil {
		return response.BadRequest(c, "INVALID_BODY", "Invalid request body")
	}
	a, err := h.service.CreateArticle(c.Context(), orgID(c), userID, req)
	if err != nil {
		return mapError(c, log, "CreateArticle", err)
	}
	return response.Created(c, fiber.Map{"article": a}, "Knowledge base article created")
}

// GetArticle handles GET /organizations/:orgId/kb/articles/:articleId
func (h *Handler) GetArticle(c fiber.Ctx) error {
	log := logger.FromCtx(c)
	userID, err := callerUserID(c)
	if err != nil {
		return mapError(c, log, "GetArticle", err)
	}
	a, err := h.service.GetArticle(c.Context(), orgID(c), userID, c.Params("articleId"))
	if err != nil {
		return mapError(c, log, "GetArticle", err)
	}
	return response.OK(c, fiber.Map{"article": a}, "OK")
}

// UpdateArticle handles PATCH /organizations/:orgId/kb/articles/:articleId
func (h *Handler) UpdateArticle(c fiber.Ctx) error {
	log := logger.FromCtx(c)
	userID, err := callerUserID(c)
	if err != nil {
		return mapError(c, log, "UpdateArticle", err)
	}
	var req UpdateArticleRequest
	if err := c.Bind().JSON(&req); err != nil {
		return response.BadRequest(c, "INVALID_BODY", "Invalid request body")
	}
	a, err := h.service.UpdateArticle(c.Context(), orgID(c), userID, c.Params("articleId"), req)
	if err != nil {
		return mapError(c, log, "UpdateArticle", err)
	}
	return response.OK(c, fiber.Map{"article": a}, "Knowledge base article updated")
}

// PublishArticle handles POST /organizations/:orgId/kb/articles/:articleId/publish
func (h *Handler) PublishArticle(c fiber.Ctx) error {
	log := logger.FromCtx(c)
	userID, err := callerUserID(c)
	if err != nil {
		return mapError(c, log, "PublishArticle", err)
	}
	a, err := h.service.Publish(c.Context(), orgID(c), userID, c.Params("articleId"))
	if err != nil {
		return mapError(c, log, "PublishArticle", err)
	}
	return response.OK(c, fiber.Map{"article": a}, "Knowledge base article published")
}

// ArchiveArticle handles POST /organizations/:orgId/kb/articles/:articleId/archive
func (h *Handler) ArchiveArticle(c fiber.Ctx) error {
	log := logger.FromCtx(c)
	userID, err := callerUserID(c)
	if err != nil {
		return mapError(c, log, "ArchiveArticle", err)
	}
	a, err := h.service.Archive(c.Context(), orgID(c), userID, c.Params("articleId"))
	if err != nil {
		return mapError(c, log, "ArchiveArticle", err)
	}
	return response.OK(c, fiber.Map{"article": a}, "Knowledge base article archived")
}
