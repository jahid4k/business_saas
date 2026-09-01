// backend/internal/hrm/analytics/suppression.go
package analytics

import "sort"

// Group is one cell of a distribution before suppression is applied.
type Group struct {
	Key   string `json:"key"`
	Count int    `json:"count"`
}

// DisclosedGroup is one cell AFTER suppression. A suppressed cell carries no
// count at all — not a zero, not a range. Count is a pointer so the absence
// is visible on the wire rather than serialising as 0, which a reader would
// take for "nobody".
type DisclosedGroup struct {
	Key        string `json:"key"`
	Count      *int   `json:"count"`
	Suppressed bool   `json:"suppressed"`
}

// Distribution is a suppressed breakdown.
//
// ⚠ Total is a POINTER and is nil whenever anything was suppressed. That is
// the whole point: a total plus every disclosed group is one subtraction away
// from the group that was hidden, so publishing both would make the
// suppression decorative.
type Distribution struct {
	Groups           []DisclosedGroup `json:"groups"`
	Total            *int             `json:"total"`
	Threshold        int              `json:"threshold"`
	SuppressedGroups int              `json:"suppressed_groups"`
	// Note explains the absence to whoever is looking at the chart, because
	// an unexplained hole reads as a bug and gets "fixed".
	Note string `json:"note,omitempty"`
}

// MinThreshold is the floor the schema also enforces
// (chk_hrm_metric_threshold). A threshold of 1 discloses an individual.
const MinThreshold = 2

// Suppress applies aggregate disclosure control to a distribution.
//
// ⚠ THIS RULE BINDS EVERY CALLER INCLUDING view_all HOLDERS. There is no
// permission parameter and no bypass argument, because there is no reader for
// whom "which of my colleagues is the one non-binary employee" becomes an
// appropriate question. hrm.analytics.view_dei decides whether somebody sees
// the breakdown at all; it never unlocks a cell.
//
// Three rules, each earning its place:
//
//  1. PRIMARY SUPPRESSION — any group below the threshold is hidden. The
//     obvious rule, and on its own almost useless.
//
//  2. SECONDARY SUPPRESSION — if exactly one group would be hidden, the
//     smallest disclosed group is hidden too. With one hole and a published
//     total the hole is arithmetic; with two, it is a range.
//
//  3. TOTAL WITHHELD — whenever anything is suppressed the total is not
//     reported. This is stronger than the usual cell-suppression practice
//     and is what the plan actually asks for: "the suppressed group must not
//     be inferable by differencing against the total." Rule 2 alone leaves a
//     narrow case (two hidden groups summing to 2 can only be 1 and 1), and
//     withholding the total closes it rather than special-casing it.
//
// A distribution with nothing to suppress is returned whole, total included.
func Suppress(groups []Group, threshold int) Distribution {
	if threshold < MinThreshold {
		threshold = MinThreshold
	}

	ordered := make([]Group, len(groups))
	copy(ordered, groups)
	sort.SliceStable(ordered, func(i, j int) bool { return ordered[i].Count < ordered[j].Count })

	hidden := map[string]bool{}
	for _, g := range ordered {
		if g.Count < threshold {
			hidden[g.Key] = true
		}
	}

	// Rule 2. ordered is ascending, so the first group not already hidden is
	// the smallest disclosed one.
	if len(hidden) == 1 {
		for _, g := range ordered {
			if !hidden[g.Key] {
				hidden[g.Key] = true
				break
			}
		}
	}

	out := Distribution{
		Groups:    make([]DisclosedGroup, 0, len(groups)),
		Threshold: threshold,
	}
	total := 0
	for _, g := range groups {
		total += g.Count
		if hidden[g.Key] {
			out.Groups = append(out.Groups, DisclosedGroup{Key: g.Key, Suppressed: true})
			continue
		}
		c := g.Count
		out.Groups = append(out.Groups, DisclosedGroup{Key: g.Key, Count: &c})
	}
	out.SuppressedGroups = len(hidden)

	// Rule 3.
	if len(hidden) == 0 {
		out.Total = &total
		return out
	}
	// ⚠ Suppressing everything is the correct outcome when a single group
	// would otherwise be left standing alone — one disclosed group plus a
	// withheld total still says "everyone in this organization is X".
	if len(hidden) == len(groups)-1 {
		for i := range out.Groups {
			out.Groups[i].Count = nil
			out.Groups[i].Suppressed = true
		}
		out.SuppressedGroups = len(groups)
	}
	out.Note = "Groups below the disclosure threshold are suppressed, and the total is " +
		"withheld so a suppressed group cannot be recovered by subtraction."
	return out
}
