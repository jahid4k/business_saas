// backend/internal/tests/unit/platform/tickets/sla_test.go
// The pausable SLA clock. Every SLA figure a user sees derives from this
// arithmetic, so it is pure and tested before anything calls it — the
// ComputeSlab / ApplyIncrease / Amortize / BookValue / SettleAgainstAdvance
// precedent.
package tickets_test

import (
	"testing"
	"time"

	"github.com/mridha/businesssaas/internal/platform/tickets"
)

var base = time.Date(2026, 3, 1, 9, 0, 0, 0, time.UTC)

func at(minutes int) time.Time { return base.Add(time.Duration(minutes) * time.Minute) }

func pause(minutes int) tickets.SLAEvent {
	return tickets.SLAEvent{EventType: "pause", OccurredAt: at(minutes)}
}
func resume(minutes int) tickets.SLAEvent {
	return tickets.SLAEvent{EventType: "resume", OccurredAt: at(minutes)}
}

func TestPausedDuration(t *testing.T) {
	cases := []struct {
		name    string
		events  []tickets.SLAEvent
		now     time.Time
		wantMin int
	}{
		{"no events means nothing paused", nil, at(120), 0},
		{"one closed pause", []tickets.SLAEvent{pause(30), resume(50)}, at(120), 20},
		{
			// The case a single counter cannot audit: paused twice.
			"two closed pauses accumulate",
			[]tickets.SLAEvent{pause(10), resume(25), pause(60), resume(90)},
			at(120), 45,
		},
		{
			"a trailing pause is still running, so it counts up to now",
			[]tickets.SLAEvent{pause(30)},
			at(100), 70,
		},
		{
			"closed pause plus a still-open one",
			[]tickets.SLAEvent{pause(10), resume(20), pause(80)},
			at(100), 30,
		},
		{
			"a second pause while already paused adds nothing",
			[]tickets.SLAEvent{pause(10), pause(30), resume(50)},
			at(120), 40, // measured from the FIRST pause
		},
		{
			"a resume with no matching pause stops nothing",
			[]tickets.SLAEvent{resume(20)},
			at(120), 0,
		},
		{
			"a stray resume before any pause does not corrupt a later interval",
			[]tickets.SLAEvent{resume(5), pause(30), resume(50)},
			at(120), 20,
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := tickets.PausedDuration(c.events, c.now)
			want := time.Duration(c.wantMin) * time.Minute
			if got != want {
				t.Errorf("PausedDuration = %v, want %v", got, want)
			}
		})
	}

	t.Run("an out-of-order resume contributes zero, never negative", func(t *testing.T) {
		// Clock skew or bad data. A negative interval would SUBTRACT from
		// paused time, inflating elapsed time and masking a breach.
		events := []tickets.SLAEvent{
			{EventType: "pause", OccurredAt: at(60)},
			{EventType: "resume", OccurredAt: at(30)}, // earlier than its pause
		}
		if got := tickets.PausedDuration(events, at(120)); got != 0 {
			t.Errorf("PausedDuration = %v, want 0 for an inverted interval", got)
		}
	})
}

func TestIsPaused(t *testing.T) {
	cases := []struct {
		name   string
		events []tickets.SLAEvent
		want   bool
	}{
		{"no events", nil, false},
		{"open pause", []tickets.SLAEvent{pause(10)}, true},
		{"paused then resumed", []tickets.SLAEvent{pause(10), resume(20)}, false},
		{"resumed then paused again", []tickets.SLAEvent{pause(10), resume(20), pause(30)}, true},
		{"fully cycled twice", []tickets.SLAEvent{pause(10), resume(20), pause(30), resume(40)}, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := tickets.IsPaused(c.events); got != c.want {
				t.Errorf("IsPaused = %v, want %v", got, c.want)
			}
		})
	}
}

func TestElapsedSLA_ExcludesPausedTime(t *testing.T) {
	t.Run("no pauses means elapsed is plain wall clock", func(t *testing.T) {
		got := tickets.ElapsedSLA(base, at(120), nil)
		if got != 120*time.Minute {
			t.Errorf("elapsed = %v, want 120m", got)
		}
	})

	t.Run("a 30-minute pause is excluded", func(t *testing.T) {
		// 120 minutes wall clock, 30 of them paused -> 90 working minutes.
		got := tickets.ElapsedSLA(base, at(120), []tickets.SLAEvent{pause(30), resume(60)})
		if got != 90*time.Minute {
			t.Errorf("elapsed = %v, want 90m", got)
		}
	})

	t.Run("two pauses are both excluded", func(t *testing.T) {
		got := tickets.ElapsedSLA(base, at(200), []tickets.SLAEvent{
			pause(10), resume(30), pause(100), resume(140),
		})
		// 200 wall - (20 + 40) = 140.
		if got != 140*time.Minute {
			t.Errorf("elapsed = %v, want 140m", got)
		}
	})

	t.Run("a still-paused ticket stops accruing", func(t *testing.T) {
		// Paused at minute 30 and never resumed: elapsed freezes at 30
		// however long we wait.
		events := []tickets.SLAEvent{pause(30)}
		atHour := tickets.ElapsedSLA(base, at(60), events)
		atDay := tickets.ElapsedSLA(base, at(1440), events)
		if atHour != 30*time.Minute {
			t.Errorf("elapsed at +60m = %v, want 30m", atHour)
		}
		if atDay != 30*time.Minute {
			t.Errorf("elapsed at +1440m = %v, want 30m — a paused clock must not keep running", atDay)
		}
	})

	t.Run("elapsed never goes negative", func(t *testing.T) {
		if got := tickets.ElapsedSLA(base, base, nil); got != 0 {
			t.Errorf("elapsed = %v, want 0", got)
		}
		if got := tickets.ElapsedSLA(at(60), base, nil); got != 0 {
			t.Errorf("elapsed with until before createdAt = %v, want 0", got)
		}
	})
}

func TestEvaluateSLA(t *testing.T) {
	t.Run("within target", func(t *testing.T) {
		st := tickets.EvaluateSLA(base, at(30), 60, nil)
		if st.Breached {
			t.Error("breached at 30m against a 60m target")
		}
		if st.Remaining != 30*time.Minute {
			t.Errorf("remaining = %v, want 30m", st.Remaining)
		}
	})

	t.Run("breached", func(t *testing.T) {
		st := tickets.EvaluateSLA(base, at(90), 60, nil)
		if !st.Breached {
			t.Error("not breached at 90m against a 60m target")
		}
		if st.Remaining != 0 {
			t.Errorf("remaining = %v, want 0 once breached", st.Remaining)
		}
	})

	t.Run("exactly at target is breached", func(t *testing.T) {
		st := tickets.EvaluateSLA(base, at(60), 60, nil)
		if !st.Breached {
			t.Error("60m elapsed against a 60m target should be breached")
		}
	})

	// THE point of a pausable clock: a pause is what keeps a ticket inside
	// its SLA when the delay was not the agent's.
	t.Run("a pause rescues a ticket that would otherwise have breached", func(t *testing.T) {
		// 90 minutes wall clock against a 60-minute target — a breach — but
		// 40 of those minutes were spent waiting on the requester.
		events := []tickets.SLAEvent{pause(20), resume(60)}
		st := tickets.EvaluateSLA(base, at(90), 60, events)
		if st.Breached {
			t.Errorf("breached despite a 40m pause: elapsed = %v", st.Elapsed)
		}
		if st.Elapsed != 50*time.Minute {
			t.Errorf("elapsed = %v, want 50m (90 wall - 40 paused)", st.Elapsed)
		}
		if st.Remaining != 10*time.Minute {
			t.Errorf("remaining = %v, want 10m", st.Remaining)
		}
	})

	t.Run("no policy configured is never a breach", func(t *testing.T) {
		// Reporting every ticket in an org with no SLA policy as breached
		// would make the signal useless.
		for _, target := range []int{0, -30} {
			st := tickets.EvaluateSLA(base, at(10000), target, nil)
			if st.Breached {
				t.Errorf("target %d: breached with no policy configured", target)
			}
			if st.Target != 0 {
				t.Errorf("target %d: Target = %v, want 0", target, st.Target)
			}
		}
	})

	t.Run("Paused is reported alongside the measurement", func(t *testing.T) {
		st := tickets.EvaluateSLA(base, at(90), 60, []tickets.SLAEvent{pause(20)})
		if !st.Paused {
			t.Error("Paused = false for a ticket with an open pause")
		}
	})
}

func TestEvaluateSLA_MinuteFieldsTrackTheDurations(t *testing.T) {
	// The serialised minute fields are derived from the durations, never
	// assigned separately, so the two can never disagree on the wire.
	cases := []struct {
		name             string
		until            time.Time
		target           int
		events           []tickets.SLAEvent
		wantElapsedMin   int
		wantTargetMin    int
		wantRemainingMin int
	}{
		{"plain", at(30), 60, nil, 30, 60, 30},
		{"breached floors remaining at zero", at(90), 60, nil, 90, 60, 0},
		{"no policy leaves every field zero", at(90), 0, nil, 90, 0, 0},
		{"a pause is excluded from the minute count", at(90), 60,
			[]tickets.SLAEvent{pause(20), resume(60)}, 50, 60, 10},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			st := tickets.EvaluateSLA(base, c.until, c.target, c.events)
			if st.ElapsedMinutes != c.wantElapsedMin {
				t.Errorf("ElapsedMinutes = %d, want %d", st.ElapsedMinutes, c.wantElapsedMin)
			}
			if st.TargetMinutes != c.wantTargetMin {
				t.Errorf("TargetMinutes = %d, want %d", st.TargetMinutes, c.wantTargetMin)
			}
			if st.RemainingMinutes != c.wantRemainingMin {
				t.Errorf("RemainingMinutes = %d, want %d", st.RemainingMinutes, c.wantRemainingMin)
			}
			if st.ElapsedMinutes != int(st.Elapsed/time.Minute) {
				t.Error("ElapsedMinutes and Elapsed disagree")
			}
		})
	}
}
