// backend/internal/platform/tickets/sla.go
package tickets

import "time"

// SLAEvent is one pause or resume, as recorded in the append-only
// platform_ticket_sla_events ledger.
type SLAEvent struct {
	EventType  string // "pause" | "resume"
	OccurredAt time.Time
}

// PausedDuration folds a ticket's pause/resume ledger into the total time
// its SLA clock was stopped.
//
// Pure — no DB, no service receiver — so it is unit-testable directly: the
// payslips.ComputeSlab / compensation.ApplyIncrease / loans.Amortize /
// assets.BookValue / expenses.SettleAgainstAdvance precedent. Every SLA
// figure a user sees derives from this, so it is tested before anything
// calls it.
//
// Why a ledger and not a counter: one ticket is routinely paused and resumed
// several times ("waiting on the requester", twice). A mutable
// paused_minutes column would show the total but never how it was reached,
// and could not be audited or corrected without rewriting history. Same
// reasoning as 7C's hrm_loan_recovery_events.
//
// The ledger is assumed ordered by occurred_at (the repository orders it).
// Real-world messiness is handled rather than trusted:
//
//   - Consecutive pauses: the FIRST wins. A second pause while already
//     paused adds nothing, because the clock is already stopped.
//   - A resume with no matching pause: ignored. It stops nothing.
//   - A trailing pause with no resume: the ticket is paused RIGHT NOW, so
//     the open interval runs to `now`.
//   - A resume earlier than its pause (clock skew, bad data): contributes
//     zero rather than a negative interval, which would otherwise ADD time
//     to the SLA and silently mask a breach.
func PausedDuration(events []SLAEvent, now time.Time) time.Duration {
	var total time.Duration
	var pausedAt *time.Time

	for _, e := range events {
		switch e.EventType {
		case "pause":
			if pausedAt == nil {
				t := e.OccurredAt
				pausedAt = &t
			}
			// Already paused — a second pause changes nothing.
		case "resume":
			if pausedAt == nil {
				continue // resume without a pause stops nothing
			}
			if d := e.OccurredAt.Sub(*pausedAt); d > 0 {
				total += d
			}
			pausedAt = nil
		}
	}

	// Still paused: the open interval runs to now.
	if pausedAt != nil {
		if d := now.Sub(*pausedAt); d > 0 {
			total += d
		}
	}
	return total
}

// IsPaused reports whether the ledger ends in an unmatched pause.
func IsPaused(events []SLAEvent) bool {
	paused := false
	for _, e := range events {
		switch e.EventType {
		case "pause":
			paused = true
		case "resume":
			paused = false
		}
	}
	return paused
}

// ElapsedSLA is the working time a ticket has consumed: wall-clock since it
// opened, MINUS everything the pause ledger stopped.
//
// until is when the clock stops counting — the resolution time for a
// resolved ticket, or `now` for a live one. Passing a resolved ticket's
// resolution time is what keeps a closed ticket's elapsed figure stable
// instead of growing forever.
func ElapsedSLA(createdAt, until time.Time, events []SLAEvent) time.Duration {
	wall := until.Sub(createdAt)
	if wall <= 0 {
		return 0
	}
	elapsed := wall - PausedDuration(events, until)
	if elapsed < 0 {
		return 0
	}
	return elapsed
}

// SLAStatus is a ticket's standing against one target.
//
// The three durations are serialised as MINUTES, not as time.Duration.
// Marshalling a time.Duration puts a raw nanosecond count on the wire —
// 14400000000000 for four hours — which every client then has to know to
// divide. Minutes match the unit the policy is configured in, so what a
// client reads back is in the same terms an admin typed in.
type SLAStatus struct {
	// Elapsed is working time consumed, excluding paused periods.
	Elapsed time.Duration `json:"-"`
	// Target is the policy allowance. Zero means no policy applies, in which
	// case Breached is always false — no target cannot be missed.
	Target time.Duration `json:"-"`
	// Remaining is Target - Elapsed, floored at zero.
	Remaining time.Duration `json:"-"`

	ElapsedMinutes   int  `json:"elapsed_minutes"`
	TargetMinutes    int  `json:"target_minutes"`
	RemainingMinutes int  `json:"remaining_minutes"`
	Breached         bool `json:"breached"`
	// Paused reports whether the clock is stopped right now.
	Paused bool `json:"paused"`
}

// inMinutes fills the serialised fields from the duration ones. Kept as a
// derivation rather than two independent assignments so the pair can never
// disagree.
func (s *SLAStatus) inMinutes() {
	s.ElapsedMinutes = int(s.Elapsed / time.Minute)
	s.TargetMinutes = int(s.Target / time.Minute)
	s.RemainingMinutes = int(s.Remaining / time.Minute)
}

// EvaluateSLA measures elapsed working time against a target.
//
// A zero or negative target means "no policy configured", and is NEVER a
// breach — reporting every ticket in an org with no SLA policy as breached
// would make the whole signal useless.
func EvaluateSLA(createdAt, until time.Time, targetMinutes int, events []SLAEvent) SLAStatus {
	elapsed := ElapsedSLA(createdAt, until, events)
	st := SLAStatus{Elapsed: elapsed, Paused: IsPaused(events)}
	if targetMinutes <= 0 {
		st.inMinutes()
		return st
	}
	st.Target = time.Duration(targetMinutes) * time.Minute
	if elapsed >= st.Target {
		st.Breached = true
		st.inMinutes()
		return st
	}
	st.Remaining = st.Target - elapsed
	st.inMinutes()
	return st
}
