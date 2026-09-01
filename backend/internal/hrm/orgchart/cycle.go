// backend/internal/hrm/orgchart/cycle.go
package orgchart

// MaxChainDepth caps every upward walk.
//
// It is not a business rule about how tall an org may be — it is a guard
// against data that is ALREADY cyclic. If a bad row ever reaches the table
// (a direct database edit, a restored backup), an uncapped walk spins
// forever inside a request. scope.Predicate's own view_team CTE carries the
// same kind of cap for the same reason.
const MaxChainDepth = 64

// WouldCreateCycle reports whether making managerID the manager of
// employeeID would close a loop in the solid-line hierarchy.
//
// edges maps employeeID -> current solid-line managerID. Only solid lines
// belong here: dotted, functional and project relationships are matrix
// reporting and are ALLOWED to form loops — two people can legitimately lead
// each other's projects. Feeding those in would refuse valid org charts.
//
// The walk goes UPWARD from the proposed manager. If it reaches the employee,
// the employee is already somewhere above their proposed manager and the new
// edge would close the ring.
//
// ⚠ A parent-only check is not enough. Refusing A->B when B->A exists catches
// the direct swap and misses A->B->C->A entirely, which is the shape that
// actually occurs after a couple of reorganisations — and which turns every
// consumer of the chart into an infinite loop.
func WouldCreateCycle(edges map[string]string, employeeID, managerID string) bool {
	if employeeID == "" || managerID == "" {
		return false
	}
	// The degenerate case: managing yourself. The database CHECK refuses this
	// too, but callers should get a clean answer rather than a constraint
	// violation.
	if employeeID == managerID {
		return true
	}

	seen := map[string]bool{employeeID: true}
	current := managerID
	for depth := 0; depth < MaxChainDepth; depth++ {
		if current == employeeID {
			return true
		}
		if seen[current] {
			// A pre-existing loop that does NOT pass through this employee.
			// Refusing here is deliberate: the walk cannot prove the new edge
			// is safe, and adding to a chart that is already broken makes it
			// harder to repair.
			return true
		}
		seen[current] = true

		next, ok := edges[current]
		if !ok || next == "" {
			return false // reached the top of the chain cleanly
		}
		current = next
	}
	// Hit the depth cap without terminating: treat as cyclic rather than
	// silently allowing an edge whose safety was never established.
	return true
}

// ChainToTop returns the management chain above employeeID, nearest first,
// stopping at the top or at MaxChainDepth.
//
// Returns the chain it managed to walk even when it detects a loop — a caller
// diagnosing a broken chart needs to see the path, not an empty slice.
func ChainToTop(edges map[string]string, employeeID string) []string {
	chain := make([]string, 0, 8)
	seen := map[string]bool{employeeID: true}
	current, ok := edges[employeeID]
	for depth := 0; ok && current != "" && depth < MaxChainDepth; depth++ {
		if seen[current] {
			break // loop — return what we have
		}
		seen[current] = true
		chain = append(chain, current)
		current, ok = edges[current]
	}
	return chain
}
