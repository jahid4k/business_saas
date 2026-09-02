// backend/internal/platform/kb/repository.go
package kb

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Repository is the data access interface for the knowledge base.
//
// TENANT ISOLATION: every method takes orgID and every query filters on it.
//
// ⚠ UNPUBLISHED ARTICLES ARE EXCLUDED IN SQL, not by the caller.
// ArticleFilter.IncludeUnpublished adds a WHERE clause rather than removing
// one, so the default — a zero-valued filter — is the SAFE state: a filter
// nobody configured returns published articles only. A draft leaks here only
// if somebody deliberately asks for drafts, which is the opposite of the
// usual "forgot to filter" failure. Same reasoning as tickets' two comment
// read paths, expressed as a default rather than a second method because
// there is one query shape here, not two.
type Repository interface {
	// Categories
	FindCategories(ctx context.Context, orgID string, activeOnly bool) ([]*Category, error)
	FindCategoryByRef(ctx context.Context, orgID, ref string) (*Category, error)
	CreateCategory(ctx context.Context, c *Category) error
	UpdateCategory(ctx context.Context, c *Category) error

	// Articles
	CreateArticle(ctx context.Context, a *Article) error
	FindArticleByRef(ctx context.Context, orgID, ref string, includeUnpublished bool) (*Article, error)
	FindArticles(ctx context.Context, orgID string, f ArticleFilter) ([]*Article, error)
	CountArticles(ctx context.Context, orgID string, f ArticleFilter) (int, error)
	UpdateArticle(ctx context.Context, a *Article) error
}

type repoImpl struct{ db *pgxpool.Pool }

func NewRepository(db *pgxpool.Pool) Repository { return &repoImpl{db: db} }

// ── Categories ───────────────────────────────────────────────────────────────

const categorySel = `id, public_id, org_id, name, description, is_active,
	created_by, created_at, updated_at`

func scanCategory(row pgx.Row) (*Category, error) {
	c := &Category{}
	err := row.Scan(&c.ID, &c.PublicID, &c.OrgID, &c.Name, &c.Description, &c.IsActive,
		&c.CreatedBy, &c.CreatedAt, &c.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return c, nil
}

func (r *repoImpl) FindCategories(ctx context.Context, orgID string, activeOnly bool) ([]*Category, error) {
	q := `SELECT ` + categorySel + ` FROM platform_kb_categories WHERE org_id=$1`
	if activeOnly {
		q += ` AND is_active=TRUE`
	}
	q += ` ORDER BY name`
	rows, err := r.db.Query(ctx, q, orgID)
	if err != nil {
		return nil, fmt.Errorf("kb: FindCategories: %w", err)
	}
	defer rows.Close()
	list := make([]*Category, 0)
	for rows.Next() {
		c, err := scanCategory(rows)
		if err != nil {
			return nil, err
		}
		list = append(list, c)
	}
	return list, rows.Err()
}

func (r *repoImpl) FindCategoryByRef(ctx context.Context, orgID, ref string) (*Category, error) {
	return scanCategory(r.db.QueryRow(ctx,
		`SELECT `+categorySel+` FROM platform_kb_categories
		  WHERE org_id=$1 AND (id::text=$2 OR public_id=$2)`, orgID, ref))
}

func (r *repoImpl) CreateCategory(ctx context.Context, c *Category) error {
	err := r.db.QueryRow(ctx,
		`INSERT INTO platform_kb_categories (org_id, name, description, created_by)
		 VALUES ($1,$2,$3,$4) RETURNING id, public_id, is_active, created_at, updated_at`,
		c.OrgID, c.Name, c.Description, c.CreatedBy,
	).Scan(&c.ID, &c.PublicID, &c.IsActive, &c.CreatedAt, &c.UpdatedAt)
	if err != nil {
		return fmt.Errorf("kb: CreateCategory: %w", err)
	}
	return nil
}

func (r *repoImpl) UpdateCategory(ctx context.Context, c *Category) error {
	ct, err := r.db.Exec(ctx,
		`UPDATE platform_kb_categories SET name=$3, description=$4, is_active=$5, updated_at=NOW()
		  WHERE org_id=$1 AND id=$2::uuid`,
		c.OrgID, c.ID, c.Name, c.Description, c.IsActive)
	if err != nil {
		return fmt.Errorf("kb: UpdateCategory: %w", err)
	}
	if ct.RowsAffected() == 0 {
		return ErrCategoryNotFound
	}
	return nil
}

// ── Articles ─────────────────────────────────────────────────────────────────

const articleSel = `id, public_id, org_id, category_id, title, body, status,
	author_user_id, published_at, created_at, updated_at`

func scanArticle(row pgx.Row) (*Article, error) {
	a := &Article{}
	err := row.Scan(&a.ID, &a.PublicID, &a.OrgID, &a.CategoryID, &a.Title, &a.Body, &a.Status,
		&a.AuthorUserID, &a.PublishedAt, &a.CreatedAt, &a.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return a, nil
}

func (r *repoImpl) CreateArticle(ctx context.Context, a *Article) error {
	err := r.db.QueryRow(ctx,
		`INSERT INTO platform_kb_articles (org_id, category_id, title, body, author_user_id)
		 VALUES ($1,$2,$3,$4,$5) RETURNING id, public_id, status, created_at, updated_at`,
		a.OrgID, a.CategoryID, a.Title, a.Body, a.AuthorUserID,
	).Scan(&a.ID, &a.PublicID, &a.Status, &a.CreatedAt, &a.UpdatedAt)
	if err != nil {
		return fmt.Errorf("kb: CreateArticle: %w", err)
	}
	return nil
}

func (r *repoImpl) FindArticleByRef(ctx context.Context, orgID, ref string, includeUnpublished bool) (*Article, error) {
	q := `SELECT ` + articleSel + ` FROM platform_kb_articles
	       WHERE org_id=$1 AND (id::text=$2 OR public_id=$2)`
	if !includeUnpublished {
		q += ` AND status = 'published'`
	}
	return scanArticle(r.db.QueryRow(ctx, q, orgID, ref))
}

// articleWhere builds the predicate shared by FindArticles and CountArticles
// so a list and its own total cannot drift.
func articleWhere(orgID string, f ArticleFilter) (string, []any) {
	args := []any{orgID}
	clauses := []string{"org_id=$1"}
	add := func(clause string, val any) {
		args = append(args, val)
		clauses = append(clauses, fmt.Sprintf(clause, len(args)))
	}
	// The safe default: without IncludeUnpublished this ADDS a restriction
	// rather than removing one, so an unconfigured filter shows published
	// articles only.
	if !f.IncludeUnpublished {
		clauses = append(clauses, "status = 'published'")
	} else if f.Status != "" {
		add("status=$%d", f.Status)
	}
	if f.CategoryID != "" {
		add("category_id=$%d::uuid", f.CategoryID)
	}
	if f.Query != "" {
		// Full-text, matching the GIN expression index in migration 00112.
		// plainto_tsquery treats the input as literal words, so a user
		// typing "&" or "!" gets a search rather than a syntax error.
		add("to_tsvector('english', title || ' ' || body) @@ plainto_tsquery('english', $%d)", f.Query)
	}
	return " WHERE " + strings.Join(clauses, " AND "), args
}

func (r *repoImpl) FindArticles(ctx context.Context, orgID string, f ArticleFilter) ([]*Article, error) {
	f.Normalise()
	where, args := articleWhere(orgID, f)
	args = append(args, f.Limit, f.Offset)
	q := `SELECT ` + articleSel + ` FROM platform_kb_articles` + where +
		fmt.Sprintf(` ORDER BY COALESCE(published_at, created_at) DESC LIMIT $%d OFFSET $%d`,
			len(args)-1, len(args))
	rows, err := r.db.Query(ctx, q, args...)
	if err != nil {
		return nil, fmt.Errorf("kb: FindArticles: %w", err)
	}
	defer rows.Close()
	list := make([]*Article, 0)
	for rows.Next() {
		a, err := scanArticle(rows)
		if err != nil {
			return nil, err
		}
		list = append(list, a)
	}
	return list, rows.Err()
}

func (r *repoImpl) CountArticles(ctx context.Context, orgID string, f ArticleFilter) (int, error) {
	where, args := articleWhere(orgID, f)
	var n int
	if err := r.db.QueryRow(ctx, `SELECT COUNT(*) FROM platform_kb_articles`+where, args...).Scan(&n); err != nil {
		return 0, fmt.Errorf("kb: CountArticles: %w", err)
	}
	return n, nil
}

func (r *repoImpl) UpdateArticle(ctx context.Context, a *Article) error {
	ct, err := r.db.Exec(ctx,
		`UPDATE platform_kb_articles
		    SET category_id=$3, title=$4, body=$5, status=$6, published_at=$7, updated_at=NOW()
		  WHERE org_id=$1 AND id=$2::uuid`,
		a.OrgID, a.ID, a.CategoryID, a.Title, a.Body, a.Status, a.PublishedAt)
	if err != nil {
		return fmt.Errorf("kb: UpdateArticle: %w", err)
	}
	if ct.RowsAffected() == 0 {
		return ErrArticleNotFound
	}
	return nil
}
