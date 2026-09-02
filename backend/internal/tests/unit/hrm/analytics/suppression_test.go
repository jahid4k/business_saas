// backend/internal/tests/unit/hrm/analytics/suppression_test.go
package analytics_test

import (
	"testing"

	"github.com/mridha/businesssaas/internal/hrm/analytics"
)

func disclosed(d analytics.Distribution) map[string]int {
	out := map[string]int{}
	for _, g := range d.Groups {
		if !g.Suppressed && g.Count != nil {
			out[g.Key] = *g.Count
		}
	}
	return out
}

// TestSuppress_SuppressedGroupCannotBeDifferencedOut is the plan's stated
// verification and the reason the rule is more than a threshold check.
//
// With a published total, one hidden group is a subtraction. The test walks
// every shape that arithmetic could exploit.
func TestSuppress_SuppressedGroupCannotBeDifferencedOut(t *testing.T) {
	cases := []struct {
		name   string
		groups []analytics.Group
	}{
		{"one tiny group among large ones", []analytics.Group{
			{Key: "male", Count: 60}, {Key: "female", Count: 35}, {Key: "other", Count: 2},
		}},
		{"two tiny groups", []analytics.Group{
			{Key: "male", Count: 60}, {Key: "female", Count: 35},
			{Key: "other", Count: 2}, {Key: "prefer_not_to_say", Count: 1},
		}},
		{"a tiny group and a small disclosed one", []analytics.Group{
			{Key: "male", Count: 90}, {Key: "female", Count: 5}, {Key: "other", Count: 1},
		}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			d := analytics.Suppress(c.groups, 5)

			if d.Total != nil {
				t.Fatalf("the total (%d) was published alongside %d suppressed group(s) — "+
					"one subtraction recovers the hidden count", *d.Total, d.SuppressedGroups)
			}
			if d.SuppressedGroups < 2 {
				t.Errorf("%d group(s) suppressed; at least 2 are needed or the hole is "+
					"determinable", d.SuppressedGroups)
			}
			if d.Note == "" {
				t.Error("suppression happened with no explanation — an unexplained hole in a " +
					"chart reads as a bug and gets 'fixed'")
			}
			// No suppressed cell may leak a count through the wire field.
			for _, g := range d.Groups {
				if g.Suppressed && g.Count != nil {
					t.Errorf("suppressed group %q still carries count %d", g.Key, *g.Count)
				}
			}
		})
	}
}

// TestSuppress_CleanDistributionIsReportedWhole — suppression that fires when
// it does not need to makes the whole instrument useless and teaches people
// to route around it.
func TestSuppress_CleanDistributionIsReportedWhole(t *testing.T) {
	d := analytics.Suppress([]analytics.Group{
		{Key: "male", Count: 60}, {Key: "female", Count: 35}, {Key: "other", Count: 12},
	}, 5)
	if d.SuppressedGroups != 0 {
		t.Fatalf("%d groups suppressed on a distribution where every group clears the threshold",
			d.SuppressedGroups)
	}
	if d.Total == nil || *d.Total != 107 {
		t.Errorf("total = %v, want 107", d.Total)
	}
	got := disclosed(d)
	if got["male"] != 60 || got["female"] != 35 || got["other"] != 12 {
		t.Errorf("disclosed counts %v do not match the input", got)
	}
	if d.Note != "" {
		t.Errorf("a clean distribution carried a suppression note: %q", d.Note)
	}
}

// TestSuppress_ExactlyAtThresholdIsDisclosed — the threshold is a floor, not
// a strict inequality, and drifting it by one either discloses a group that
// should be hidden or hides one that need not be.
func TestSuppress_ExactlyAtThresholdIsDisclosed(t *testing.T) {
	d := analytics.Suppress([]analytics.Group{
		{Key: "a", Count: 50}, {Key: "b", Count: 30}, {Key: "c", Count: 5},
	}, 5)
	if d.SuppressedGroups != 0 {
		t.Errorf("a group of exactly 5 was suppressed at threshold 5 — the threshold is a floor")
	}
	d = analytics.Suppress([]analytics.Group{
		{Key: "a", Count: 50}, {Key: "b", Count: 30}, {Key: "c", Count: 4},
	}, 5)
	if d.SuppressedGroups < 2 {
		t.Errorf("a group of 4 at threshold 5 produced %d suppressions", d.SuppressedGroups)
	}
}

// TestSuppress_LastGroupStandingIsAlsoSuppressed — one disclosed group with a
// withheld total still says "everyone here is X", which is the disclosure the
// whole mechanism exists to prevent.
func TestSuppress_LastGroupStandingIsAlsoSuppressed(t *testing.T) {
	d := analytics.Suppress([]analytics.Group{
		{Key: "male", Count: 40}, {Key: "female", Count: 2}, {Key: "other", Count: 1},
	}, 5)
	for _, g := range d.Groups {
		if !g.Suppressed {
			t.Errorf("group %q remained disclosed while every other group was hidden — "+
				"that alone discloses the composition", g.Key)
		}
	}
	if d.Total != nil {
		t.Error("the total was published")
	}
}

// TestSuppress_ThresholdFloorCannotBeTalkedDown — a caller passing 1 or 0 is
// asking to disclose an individual, and the function refuses rather than
// obeying. The schema CHECK enforces the same floor; this is the code half.
func TestSuppress_ThresholdFloorCannotBeTalkedDown(t *testing.T) {
	for _, threshold := range []int{-5, 0, 1} {
		d := analytics.Suppress([]analytics.Group{
			{Key: "a", Count: 50}, {Key: "b", Count: 30}, {Key: "c", Count: 1},
		}, threshold)
		if d.Threshold < analytics.MinThreshold {
			t.Errorf("threshold %d was accepted as %d, want a floor of %d",
				threshold, d.Threshold, analytics.MinThreshold)
		}
		if d.SuppressedGroups == 0 {
			t.Errorf("threshold %d disclosed a group of 1", threshold)
		}
	}
}

// TestSuppress_PreservesInputOrder — the caller decides the order groups are
// drawn in; re-sorting them by size would itself hint at which is smallest.
func TestSuppress_PreservesInputOrder(t *testing.T) {
	in := []analytics.Group{
		{Key: "male", Count: 60}, {Key: "other", Count: 12}, {Key: "female", Count: 35},
	}
	d := analytics.Suppress(in, 5)
	want := []string{"male", "other", "female"}
	for i, g := range d.Groups {
		if g.Key != want[i] {
			t.Errorf("group %d is %q, want %q — input order must survive", i, g.Key, want[i])
		}
	}
	// And the input slice itself must not have been reordered underneath the
	// caller.
	if in[1].Key != "other" {
		t.Error("Suppress reordered the caller's slice in place")
	}
}
