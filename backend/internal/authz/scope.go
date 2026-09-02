// backend/internal/authz/scope.go
package authz

// Scope is the resource-level visibility tier a caller holds for a given
// resource, on top of the flat module/action RBAC that Can already enforces.
type Scope int

const (
	ScopeNone Scope = iota
	ScopeOwn
	ScopeTeam
	ScopeAll
)

func (s Scope) String() string {
	switch s {
	case ScopeOwn:
		return "own"
	case ScopeTeam:
		return "team"
	case ScopeAll:
		return "all"
	default:
		return "none"
	}
}
