// backend/internal/hrm/scope/predicate.go
package scope

import (
	"fmt"

	"github.com/mridha/businesssaas/internal/authz"
)

// DefaultMaxDepth is the Phase 1 "direct reports only" default for ScopeTeam.
// Widening to multi-level reporting chains later is a call-site constant
// change, not new infrastructure — the recursive CTE in Predicate already
// supports arbitrary depth.
const DefaultMaxDepth = 1

// Predicate returns a SQL boolean fragment — safe to AND directly into a
// WHERE clause, no leading "AND" — that is TRUE when `column` (an
// employee-id-typed column: "employee_id" on every HRM record table, or "id"
// on hrm_employees itself) is within callerUserID's visibility at tier.
//
// argOffset is the number of $-placeholders already consumed by the caller's
// query; Predicate's own placeholders start at argOffset+1. Returns the
// fragment and the args to append, in that order.
//
// ScopeTeam is inclusive of ScopeOwn (a manager sees their own records too,
// via UNION ALL) — callers never need to OR the two tiers together manually.
func Predicate(tier authz.Scope, column string, argOffset int, orgID, callerUserID string, maxDepth int) (string, []any) {
	switch tier {
	case authz.ScopeAll:
		return "TRUE", nil

	case authz.ScopeOwn:
		a, b := argOffset+1, argOffset+2
		frag := fmt.Sprintf(
			"%s = (SELECT id FROM hrm_employees WHERE org_id = $%d AND user_id = $%d)",
			column, a, b,
		)
		return frag, []any{orgID, callerUserID}

	case authz.ScopeTeam:
		a, b, c := argOffset+1, argOffset+2, argOffset+3
		// The path guard seeds with the caller's own id (not just the subordinate
		// chain) — without that, a cycle that loops back to the caller themself
		// (legal today: only direct self-reference is blocked at the app layer)
		// would not be caught by "he.id <> ALL(s.path)", since the caller was
		// never a member of the subordinate path being tracked.
		frag := fmt.Sprintf(`%s IN (
			WITH RECURSIVE caller AS (
				SELECT id FROM hrm_employees WHERE org_id = $%d AND user_id = $%d
			), subordinates AS (
				SELECT he.id, ARRAY[caller.id, he.id] AS path, 1 AS depth
				FROM hrm_employees he, caller
				WHERE he.org_id = $%d
				  AND he.manager_id = caller.id
				UNION ALL
				SELECT he.id, s.path || he.id, s.depth + 1
				FROM hrm_employees he
				JOIN subordinates s ON he.manager_id = s.id
				WHERE he.org_id = $%d
				  AND s.depth < $%d
				  AND he.id <> ALL(s.path)
			)
			SELECT id FROM subordinates
			UNION ALL
			SELECT id FROM caller
		)`, column, a, b, a, a, c)
		return frag, []any{orgID, callerUserID, maxDepth}

	default: // authz.ScopeNone
		return "FALSE", nil
	}
}
