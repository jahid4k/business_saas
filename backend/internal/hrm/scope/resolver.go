// backend/internal/hrm/scope/resolver.go
package scope

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/mridha/businesssaas/internal/authz"
)

// Resolver checks single-record access — the "GET by ID" / path-scoped-by-
// :employeeId shape that Predicate's list-level WHERE-clause injection
// doesn't cover, since the record has already been resolved outside of any
// list query by the time this check runs.
type Resolver struct {
	db *pgxpool.Pool
}

func NewResolver(db *pgxpool.Pool) *Resolver {
	return &Resolver{db: db}
}

// AuthorizeRecordAccess reports whether callerUserID, holding tier, may see a
// record whose owning employee is recordEmployeeRef (accepts either an
// hrm_employees.id or public_id, same ref-matching convention as FindByRef
// elsewhere in this codebase).
func (r *Resolver) AuthorizeRecordAccess(ctx context.Context, tier authz.Scope, orgID, callerUserID, recordEmployeeRef string) (bool, error) {
	if tier == authz.ScopeAll {
		return true, nil
	}
	if tier == authz.ScopeNone {
		return false, nil
	}

	frag, predArgs := Predicate(tier, "id", 2, orgID, callerUserID, DefaultMaxDepth)
	q := fmt.Sprintf(
		`SELECT EXISTS(SELECT 1 FROM hrm_employees WHERE org_id = $1 AND (id::text = $2 OR public_id = $2) AND (%s))`,
		frag,
	)
	args := append([]any{orgID, recordEmployeeRef}, predArgs...)

	var ok bool
	if err := r.db.QueryRow(ctx, q, args...).Scan(&ok); err != nil {
		return false, fmt.Errorf("scope: AuthorizeRecordAccess: %w", err)
	}
	return ok, nil
}
