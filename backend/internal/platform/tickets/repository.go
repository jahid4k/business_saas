// backend/internal/platform/tickets/repository.go
package tickets

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Repository is the data access interface for the ticket engine.
//
// TENANT ISOLATION: comments and SLA events have no org_id of their own —
// every query reaches them by JOINing through platform_tickets and filtering
// its org_id, the platform_checklist_instance_items precedent.
//
// ⚠ THE TWO COMMENT READ PATHS ARE THE CONFIDENTIALITY MECHANISM.
// FindPublicComments and FindAllComments differ by one WHERE clause, and
// that difference is the whole of internal-comment protection. A single
// FindComments returning everything plus a caller-side filter is the version
// that eventually leaks, because the filter is one forgotten branch away
// from being skipped. The requester's path must never have an internal
// comment in memory at all. Same structural shape as 5C's 360-feedback
// anonymity and 6A's quiz answer keys.
type Repository interface {
	// Categories
	FindCategories(ctx context.Context, orgID string, activeOnly bool) ([]*Category, error)
	FindCategoryByRef(ctx context.Context, orgID, ref string) (*Category, error)
	CreateCategory(ctx context.Context, c *Category) error
	UpdateCategory(ctx context.Context, c *Category) error

	// SLA policies
	FindPolicies(ctx context.Context, orgID string) ([]*SLAPolicy, error)
	FindPolicyByRef(ctx context.Context, orgID, ref string) (*SLAPolicy, error)
	CreatePolicy(ctx context.Context, p *SLAPolicy) error
	UpdatePolicy(ctx context.Context, p *SLAPolicy) error
	// ResolvePolicy picks the policy governing a new ticket: the one matching
	// both category and priority if it exists, otherwise the org-wide default
	// (NULL category_id) for that priority. Returns nil when the org has
	// configured neither — an org with no SLA policy is a valid state, and
	// tickets there simply have no target rather than being reported as
	// permanently breached.
	ResolvePolicy(ctx context.Context, orgID string, categoryID *string, priority Priority) (*SLAPolicy, error)

	// Tickets
	CreateTicket(ctx context.Context, t *Ticket) error
	FindTicketByRef(ctx context.Context, orgID, ref string) (*Ticket, error)
	FindTickets(ctx context.Context, orgID string, f ListFilter) ([]*Ticket, error)
	CountTickets(ctx context.Context, orgID string, f ListFilter) (int, error)
	UpdateTicket(ctx context.Context, t *Ticket) error

	// Comments — two read paths, see the interface doc comment.
	CreateComment(ctx context.Context, orgID string, c *Comment) error
	// FindPublicComments returns ONLY is_internal = FALSE rows. This is the
	// requester's path.
	FindPublicComments(ctx context.Context, orgID, ticketID string) ([]*Comment, error)
	// FindAllComments returns internal comments too. Agents only.
	FindAllComments(ctx context.Context, orgID, ticketID string) ([]*Comment, error)

	// SLA events — append-only.
	CreateSLAEvent(ctx context.Context, orgID, ticketID, eventType string, reason *string, actorID string) error
	FindSLAEvents(ctx context.Context, orgID, ticketID string) ([]SLAEvent, error)
}

type repoImpl struct{ db *pgxpool.Pool }

func NewRepository(db *pgxpool.Pool) Repository { return &repoImpl{db: db} }

// ── Categories ───────────────────────────────────────────────────────────────

const categorySel = `id, public_id, org_id, name, description, is_sensitive,
	restricted_role, is_active, created_by, created_at, updated_at`

func scanCategory(row pgx.Row) (*Category, error) {
	c := &Category{}
	err := row.Scan(&c.ID, &c.PublicID, &c.OrgID, &c.Name, &c.Description, &c.IsSensitive,
		&c.RestrictedRole, &c.IsActive, &c.CreatedBy, &c.CreatedAt, &c.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return c, nil
}

func (r *repoImpl) FindCategories(ctx context.Context, orgID string, activeOnly bool) ([]*Category, error) {
	q := `SELECT ` + categorySel + ` FROM platform_ticket_categories WHERE org_id=$1`
	if activeOnly {
		q += ` AND is_active=TRUE`
	}
	q += ` ORDER BY name`
	rows, err := r.db.Query(ctx, q, orgID)
	if err != nil {
		return nil, fmt.Errorf("tickets: FindCategories: %w", err)
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
		`SELECT `+categorySel+` FROM platform_ticket_categories
		  WHERE org_id=$1 AND (id::text=$2 OR public_id=$2)`, orgID, ref))
}

func (r *repoImpl) CreateCategory(ctx context.Context, c *Category) error {
	err := r.db.QueryRow(ctx,
		`INSERT INTO platform_ticket_categories (org_id, name, description, is_sensitive, restricted_role, created_by)
		 VALUES ($1,$2,$3,$4,$5,$6) RETURNING id, public_id, is_active, created_at, updated_at`,
		c.OrgID, c.Name, c.Description, c.IsSensitive, c.RestrictedRole, c.CreatedBy,
	).Scan(&c.ID, &c.PublicID, &c.IsActive, &c.CreatedAt, &c.UpdatedAt)
	if err != nil {
		return fmt.Errorf("tickets: CreateCategory: %w", err)
	}
	return nil
}

func (r *repoImpl) UpdateCategory(ctx context.Context, c *Category) error {
	ct, err := r.db.Exec(ctx,
		`UPDATE platform_ticket_categories
		    SET name=$3, description=$4, is_sensitive=$5, restricted_role=$6, is_active=$7, updated_at=NOW()
		  WHERE org_id=$1 AND id=$2::uuid`,
		c.OrgID, c.ID, c.Name, c.Description, c.IsSensitive, c.RestrictedRole, c.IsActive)
	if err != nil {
		return fmt.Errorf("tickets: UpdateCategory: %w", err)
	}
	if ct.RowsAffected() == 0 {
		return ErrCategoryNotFound
	}
	return nil
}

// ── SLA policies ─────────────────────────────────────────────────────────────

const policySel = `id, public_id, org_id, category_id, priority,
	first_response_minutes, resolution_minutes, created_by, created_at, updated_at`

func scanPolicy(row pgx.Row) (*SLAPolicy, error) {
	p := &SLAPolicy{}
	err := row.Scan(&p.ID, &p.PublicID, &p.OrgID, &p.CategoryID, &p.Priority,
		&p.FirstResponseMinutes, &p.ResolutionMinutes, &p.CreatedBy, &p.CreatedAt, &p.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return p, nil
}

func (r *repoImpl) FindPolicies(ctx context.Context, orgID string) ([]*SLAPolicy, error) {
	rows, err := r.db.Query(ctx,
		`SELECT `+policySel+` FROM platform_sla_policies WHERE org_id=$1
		  ORDER BY category_id NULLS FIRST, priority`, orgID)
	if err != nil {
		return nil, fmt.Errorf("tickets: FindPolicies: %w", err)
	}
	defer rows.Close()
	list := make([]*SLAPolicy, 0)
	for rows.Next() {
		p, err := scanPolicy(rows)
		if err != nil {
			return nil, err
		}
		list = append(list, p)
	}
	return list, rows.Err()
}

func (r *repoImpl) FindPolicyByRef(ctx context.Context, orgID, ref string) (*SLAPolicy, error) {
	return scanPolicy(r.db.QueryRow(ctx,
		`SELECT `+policySel+` FROM platform_sla_policies
		  WHERE org_id=$1 AND (id::text=$2 OR public_id=$2)`, orgID, ref))
}

func (r *repoImpl) CreatePolicy(ctx context.Context, p *SLAPolicy) error {
	err := r.db.QueryRow(ctx,
		`INSERT INTO platform_sla_policies (org_id, category_id, priority, first_response_minutes, resolution_minutes, created_by)
		 VALUES ($1,$2,$3,$4,$5,$6) RETURNING id, public_id, created_at, updated_at`,
		p.OrgID, p.CategoryID, p.Priority, p.FirstResponseMinutes, p.ResolutionMinutes, p.CreatedBy,
	).Scan(&p.ID, &p.PublicID, &p.CreatedAt, &p.UpdatedAt)
	if err != nil {
		return fmt.Errorf("tickets: CreatePolicy: %w", err)
	}
	return nil
}

func (r *repoImpl) UpdatePolicy(ctx context.Context, p *SLAPolicy) error {
	ct, err := r.db.Exec(ctx,
		`UPDATE platform_sla_policies
		    SET first_response_minutes=$3, resolution_minutes=$4, updated_at=NOW()
		  WHERE org_id=$1 AND id=$2::uuid`,
		p.OrgID, p.ID, p.FirstResponseMinutes, p.ResolutionMinutes)
	if err != nil {
		return fmt.Errorf("tickets: UpdatePolicy: %w", err)
	}
	if ct.RowsAffected() == 0 {
		return ErrPolicyNotFound
	}
	return nil
}

func (r *repoImpl) ResolvePolicy(ctx context.Context, orgID string, categoryID *string, priority Priority) (*SLAPolicy, error) {
	// Category-specific wins over the org-wide default; ORDER BY puts the
	// non-NULL category_id first and LIMIT 1 takes it. A ticket with no
	// category can only ever match the default row.
	return scanPolicy(r.db.QueryRow(ctx,
		`SELECT `+policySel+` FROM platform_sla_policies
		  WHERE org_id=$1 AND priority=$3
		    AND (category_id IS NULL OR ($2::uuid IS NOT NULL AND category_id=$2::uuid))
		  ORDER BY category_id NULLS LAST
		  LIMIT 1`,
		orgID, categoryID, priority))
}

// ── Tickets ──────────────────────────────────────────────────────────────────

const ticketSel = `id, public_id, org_id, requester_type, requester_id, requester_user_id,
	category_id, subject, description, priority, status, assignee_user_id, sla_policy_id,
	first_response_at, resolved_at, closed_at, converted_to_type, converted_to_id, converted_at,
	created_at, updated_at`

func scanTicket(row pgx.Row) (*Ticket, error) {
	t := &Ticket{}
	err := row.Scan(&t.ID, &t.PublicID, &t.OrgID, &t.RequesterType, &t.RequesterID, &t.RequesterUserID,
		&t.CategoryID, &t.Subject, &t.Description, &t.Priority, &t.Status, &t.AssigneeUserID, &t.SLAPolicyID,
		&t.FirstResponseAt, &t.ResolvedAt, &t.ClosedAt, &t.ConvertedToType, &t.ConvertedToID, &t.ConvertedAt,
		&t.CreatedAt, &t.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return t, nil
}

func (r *repoImpl) CreateTicket(ctx context.Context, t *Ticket) error {
	err := r.db.QueryRow(ctx,
		`INSERT INTO platform_tickets
		   (org_id, requester_type, requester_id, requester_user_id, category_id,
		    subject, description, priority, sla_policy_id)
		 VALUES ($1,$2,$3::uuid,$4::uuid,$5,$6,$7,$8,$9)
		 RETURNING id, public_id, status, created_at, updated_at`,
		t.OrgID, t.RequesterType, t.RequesterID, t.RequesterUserID, t.CategoryID,
		t.Subject, t.Description, t.Priority, t.SLAPolicyID,
	).Scan(&t.ID, &t.PublicID, &t.Status, &t.CreatedAt, &t.UpdatedAt)
	if err != nil {
		return fmt.Errorf("tickets: CreateTicket: %w", err)
	}
	return nil
}

func (r *repoImpl) FindTicketByRef(ctx context.Context, orgID, ref string) (*Ticket, error) {
	return scanTicket(r.db.QueryRow(ctx,
		`SELECT `+ticketSel+` FROM platform_tickets
		  WHERE org_id=$1 AND (id::text=$2 OR public_id=$2)`, orgID, ref))
}

// ticketWhere builds the shared predicate for FindTickets and CountTickets so
// the two can never drift — a list whose total counts rows the list itself
// excludes is a paging bug that only shows up under load.
func ticketWhere(orgID string, f ListFilter) (string, []any) {
	args := []any{orgID}
	clauses := []string{"org_id=$1"}
	add := func(clause string, val any) {
		args = append(args, val)
		clauses = append(clauses, fmt.Sprintf(clause, len(args)))
	}
	if f.Status != "" {
		add("status=$%d", f.Status)
	}
	if f.Priority != "" {
		add("priority=$%d", f.Priority)
	}
	if f.CategoryID != "" {
		add("category_id=$%d::uuid", f.CategoryID)
	}
	if f.AssigneeUserID != "" {
		add("assignee_user_id=$%d::uuid", f.AssigneeUserID)
	}
	// The visibility narrowing. Without .view_all a caller sees the tickets
	// they raised plus the ones assigned to them, and nothing else. This is
	// the platform stand-in for hrm/scope, which cannot be used here because
	// scope.Predicate hard-codes FROM hrm_employees.
	if !f.CanViewAll {
		args = append(args, f.ViewerUserID)
		clauses = append(clauses, fmt.Sprintf(
			"(requester_user_id=$%d::uuid OR assignee_user_id=$%d::uuid)", len(args), len(args)))
	}
	return " WHERE " + strings.Join(clauses, " AND "), args
}

func (r *repoImpl) FindTickets(ctx context.Context, orgID string, f ListFilter) ([]*Ticket, error) {
	f.Normalise()
	where, args := ticketWhere(orgID, f)
	args = append(args, f.Limit, f.Offset)
	q := `SELECT ` + ticketSel + ` FROM platform_tickets` + where +
		fmt.Sprintf(` ORDER BY created_at DESC LIMIT $%d OFFSET $%d`, len(args)-1, len(args))
	rows, err := r.db.Query(ctx, q, args...)
	if err != nil {
		return nil, fmt.Errorf("tickets: FindTickets: %w", err)
	}
	defer rows.Close()
	list := make([]*Ticket, 0)
	for rows.Next() {
		t, err := scanTicket(rows)
		if err != nil {
			return nil, err
		}
		list = append(list, t)
	}
	return list, rows.Err()
}

func (r *repoImpl) CountTickets(ctx context.Context, orgID string, f ListFilter) (int, error) {
	where, args := ticketWhere(orgID, f)
	var n int
	if err := r.db.QueryRow(ctx, `SELECT COUNT(*) FROM platform_tickets`+where, args...).Scan(&n); err != nil {
		return 0, fmt.Errorf("tickets: CountTickets: %w", err)
	}
	return n, nil
}

func (r *repoImpl) UpdateTicket(ctx context.Context, t *Ticket) error {
	ct, err := r.db.Exec(ctx,
		`UPDATE platform_tickets
		    SET category_id=$3, subject=$4, description=$5, priority=$6, status=$7,
		        assignee_user_id=$8, sla_policy_id=$9, first_response_at=$10,
		        resolved_at=$11, closed_at=$12,
		        converted_to_type=$13, converted_to_id=$14, converted_at=$15,
		        updated_at=NOW()
		  WHERE org_id=$1 AND id=$2::uuid`,
		t.OrgID, t.ID, t.CategoryID, t.Subject, t.Description, t.Priority, t.Status,
		t.AssigneeUserID, t.SLAPolicyID, t.FirstResponseAt, t.ResolvedAt, t.ClosedAt,
		t.ConvertedToType, t.ConvertedToID, t.ConvertedAt)
	if err != nil {
		return fmt.Errorf("tickets: UpdateTicket: %w", err)
	}
	if ct.RowsAffected() == 0 {
		return ErrTicketNotFound
	}
	return nil
}

// ── Comments ─────────────────────────────────────────────────────────────────

const commentSel = `c.id, c.public_id, c.ticket_id, c.author_user_id, c.body,
	c.is_internal, c.created_at, c.updated_at`

func scanComment(row pgx.Row) (*Comment, error) {
	c := &Comment{}
	err := row.Scan(&c.ID, &c.PublicID, &c.TicketID, &c.AuthorUserID, &c.Body,
		&c.IsInternal, &c.CreatedAt, &c.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return c, nil
}

func (r *repoImpl) CreateComment(ctx context.Context, orgID string, c *Comment) error {
	// The SELECT in the VALUES clause is the tenant check: a ticket id from
	// another org yields no row and the insert affects nothing, rather than
	// silently attaching a comment across a tenant boundary.
	err := r.db.QueryRow(ctx,
		`INSERT INTO platform_ticket_comments (ticket_id, author_user_id, body, is_internal)
		 SELECT t.id, $3::uuid, $4, $5 FROM platform_tickets t
		  WHERE t.id=$2::uuid AND t.org_id=$1
		 RETURNING id, public_id, created_at, updated_at`,
		orgID, c.TicketID, c.AuthorUserID, c.Body, c.IsInternal,
	).Scan(&c.ID, &c.PublicID, &c.CreatedAt, &c.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrTicketNotFound
	}
	if err != nil {
		return fmt.Errorf("tickets: CreateComment: %w", err)
	}
	return nil
}

func (r *repoImpl) findComments(ctx context.Context, orgID, ticketID string, internalToo bool) ([]*Comment, error) {
	q := `SELECT ` + commentSel + ` FROM platform_ticket_comments c
	       JOIN platform_tickets t ON t.id = c.ticket_id
	      WHERE t.org_id=$1 AND c.ticket_id=$2::uuid`
	if !internalToo {
		q += ` AND c.is_internal = FALSE`
	}
	q += ` ORDER BY c.created_at`
	rows, err := r.db.Query(ctx, q, orgID, ticketID)
	if err != nil {
		return nil, fmt.Errorf("tickets: findComments: %w", err)
	}
	defer rows.Close()
	list := make([]*Comment, 0)
	for rows.Next() {
		c, err := scanComment(rows)
		if err != nil {
			return nil, err
		}
		list = append(list, c)
	}
	return list, rows.Err()
}

func (r *repoImpl) FindPublicComments(ctx context.Context, orgID, ticketID string) ([]*Comment, error) {
	return r.findComments(ctx, orgID, ticketID, false)
}

func (r *repoImpl) FindAllComments(ctx context.Context, orgID, ticketID string) ([]*Comment, error) {
	return r.findComments(ctx, orgID, ticketID, true)
}

// ── SLA events ───────────────────────────────────────────────────────────────

func (r *repoImpl) CreateSLAEvent(ctx context.Context, orgID, ticketID, eventType string, reason *string, actorID string) error {
	// Same tenant-guarded INSERT ... SELECT shape as CreateComment.
	ct, err := r.db.Exec(ctx,
		`INSERT INTO platform_ticket_sla_events (ticket_id, event_type, reason, actor_id)
		 SELECT t.id, $3, $4, $5::uuid FROM platform_tickets t
		  WHERE t.id=$2::uuid AND t.org_id=$1`,
		orgID, ticketID, eventType, reason, actorID)
	if err != nil {
		return fmt.Errorf("tickets: CreateSLAEvent: %w", err)
	}
	if ct.RowsAffected() == 0 {
		return ErrTicketNotFound
	}
	return nil
}

func (r *repoImpl) FindSLAEvents(ctx context.Context, orgID, ticketID string) ([]SLAEvent, error) {
	rows, err := r.db.Query(ctx,
		`SELECT e.event_type, e.occurred_at FROM platform_ticket_sla_events e
		   JOIN platform_tickets t ON t.id = e.ticket_id
		  WHERE t.org_id=$1 AND e.ticket_id=$2::uuid
		  ORDER BY e.occurred_at`,
		orgID, ticketID)
	if err != nil {
		return nil, fmt.Errorf("tickets: FindSLAEvents: %w", err)
	}
	defer rows.Close()
	list := make([]SLAEvent, 0)
	for rows.Next() {
		var ev SLAEvent
		var occurred time.Time
		if err := rows.Scan(&ev.EventType, &occurred); err != nil {
			return nil, err
		}
		ev.OccurredAt = occurred
		list = append(list, ev)
	}
	return list, rows.Err()
}
