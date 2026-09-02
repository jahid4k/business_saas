// backend/internal/tests/unit/hrm/orgchart/cycle_test.go
// Cycle detection for the reporting hierarchy. Pure, and tested before
// anything calls it — the Amortize / BookValue / EvaluateSLA /
// NoticeShortfallDays / ComputeGratuity precedent.
//
// A cycle here is not a cosmetic problem: scope.Predicate's view_team tier
// walks this same hierarchy to decide who may read whose payroll, so a loop
// makes an authorization query non-terminating.
package orgchart_test

import (
	"testing"

	"github.com/mridha/businesssaas/internal/hrm/orgchart"
)

func TestWouldCreateCycle(t *testing.T) {
	// A chain: e1 -> e2 -> e3 -> e4 (e4 at the top).
	chain := map[string]string{
		"e1": "e2",
		"e2": "e3",
		"e3": "e4",
	}

	cases := []struct {
		name              string
		edges             map[string]string
		employee, manager string
		want              bool
	}{
		{
			"an ordinary new report is fine",
			chain, "e9", "e2", false,
		},
		{
			"re-parenting within the chain, downward, is fine",
			chain, "e1", "e4", false,
		},
		{
			// The obvious case a parent-only check would also catch.
			"DIRECT swap: making a subordinate your manager",
			chain, "e2", "e1", true,
		},
		{
			// ⚠ THE CASE A PARENT-ONLY CHECK MISSES. e3 already sits two
			// levels above e1, so pointing e3 at e1 closes e1->e2->e3->e1.
			"INDIRECT cycle across three levels",
			chain, "e3", "e1", true,
		},
		{
			"indirect cycle across four levels",
			chain, "e4", "e1", true,
		},
		{
			"managing yourself",
			chain, "e1", "e1", true,
		},
		{
			"empty ids are not a cycle",
			chain, "", "e1", false,
		},
		{
			"a manager with no chain above them",
			map[string]string{}, "e1", "e2", false,
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := orgchart.WouldCreateCycle(c.edges, c.employee, c.manager); got != c.want {
				t.Errorf("WouldCreateCycle(%s -> %s) = %v, want %v",
					c.employee, c.manager, got, c.want)
			}
		})
	}
}

// TestWouldCreateCycle_RefusesAgainstAlreadyBrokenData — if a loop somehow
// reached the table (a direct database edit, a restored backup), the walk
// cannot prove a new edge is safe. Refusing is right: adding to a chart that
// is already broken makes it harder to repair, and an uncapped walk would
// spin forever inside a request.
func TestWouldCreateCycle_RefusesAgainstAlreadyBrokenData(t *testing.T) {
	broken := map[string]string{
		"a": "b",
		"b": "c",
		"c": "a", // pre-existing loop, not involving the new employee at all
	}
	if !orgchart.WouldCreateCycle(broken, "newbie", "a") {
		t.Error("accepted an edge into a pre-existing loop — the walk cannot terminate, " +
			"so it cannot have established that the edge is safe")
	}
}

// TestWouldCreateCycle_DepthCapTerminates proves a chain longer than the cap
// cannot hang a request.
func TestWouldCreateCycle_DepthCapTerminates(t *testing.T) {
	deep := map[string]string{}
	prev := "top"
	for i := 0; i < orgchart.MaxChainDepth+50; i++ {
		id := "n" + string(rune('A'+i%26)) + string(rune('0'+i/26))
		deep[id] = prev
		prev = id
	}
	// Must return, and must not claim safety it could not establish.
	if !orgchart.WouldCreateCycle(deep, "subject", prev) {
		t.Error("a chain deeper than MaxChainDepth was reported safe — the walk never " +
			"reached the top, so nothing was proved")
	}
}

func TestChainToTop(t *testing.T) {
	chain := map[string]string{"e1": "e2", "e2": "e3", "e3": "e4"}

	got := orgchart.ChainToTop(chain, "e1")
	want := []string{"e2", "e3", "e4"}
	if len(got) != len(want) {
		t.Fatalf("chain = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("chain[%d] = %s, want %s", i, got[i], want[i])
		}
	}

	if top := orgchart.ChainToTop(chain, "e4"); len(top) != 0 {
		t.Errorf("chain above the top person = %v, want empty", top)
	}

	// A loop returns what it walked rather than nothing — somebody diagnosing
	// a broken chart needs to see the path.
	loop := map[string]string{"a": "b", "b": "c", "c": "a"}
	if walked := orgchart.ChainToTop(loop, "a"); len(walked) == 0 {
		t.Error("a looping chain returned nothing; the partial path is the diagnostic")
	}
}
