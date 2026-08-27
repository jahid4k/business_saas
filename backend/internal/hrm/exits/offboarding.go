// backend/internal/hrm/exits/offboarding.go
package exits

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/mridha/businesssaas/internal/platform/forms"
)

// ── Consumer-owned interfaces for offboarding (9C) ───────────────────────────

// MembershipSuspender suspends a departing employee's org membership.
// Satisfied structurally by authz.Service. Nil-safe.
type MembershipSuspender interface {
	SuspendMembership(ctx context.Context, orgID, userID string) error
}

// SessionRevoker kills a departing employee's live sessions. Satisfied
// structurally by auth.Service, whose LogoutAll already has exactly this
// shape — no new provider method was needed. Nil-safe.
type SessionRevoker interface {
	LogoutAll(ctx context.Context, userID string) error
}

// ── Exit interviews ──────────────────────────────────────────────────────────

// ScheduleInterview creates the interview record. It does NOT send it: the
// form is instantiated when the scheduled date arrives, because an interview
// answered while the employee is still on the payroll gets a different answer
// from one answered after they have left, and the honest one is the point.
func (s *serviceImpl) ScheduleInterview(ctx context.Context, orgID string, caller Caller, ref string, req ScheduleInterviewRequest) (*ExitInterview, error) {
	e, err := s.loadExit(ctx, orgID, caller, ref)
	if err != nil {
		return nil, err
	}
	if e.Status == StatusCancelled {
		return nil, ErrWrongStatus
	}
	existing, err := s.repo.FindInterviewByExit(ctx, orgID, e.ID)
	if err != nil {
		return nil, fmt.Errorf("exits: ScheduleInterview: %w", err)
	}
	if existing != nil {
		// Matches uq_hrm_exiv_exit. A second interview would split one
		// person's answers across two records and double-count them in any
		// aggregate.
		return nil, ErrInterviewExists
	}

	// Default: the day AFTER the last working date.
	scheduledFor := e.LastWorkingDate.AddDate(0, 0, 1)
	if req.ScheduledFor != nil && strings.TrimSpace(*req.ScheduledFor) != "" {
		d, err := time.Parse("2006-01-02", strings.TrimSpace(*req.ScheduledFor))
		if err != nil {
			return nil, fmt.Errorf("exits: ScheduleInterview: scheduled_for must be YYYY-MM-DD: %w", err)
		}
		scheduledFor = d
	}

	i := &ExitInterview{
		OrgID: orgID, ExitID: e.ID, ScheduledFor: scheduledFor, CreatedBy: caller.UserID,
	}
	if err := s.repo.CreateInterview(ctx, i); err != nil {
		return nil, err
	}
	return i, nil
}

// GetInterview returns the interview's LIFECYCLE — when it is due, whether it
// was sent, whether it was answered. It deliberately carries no responses;
// reading those needs hrm.exits.interview_view, which a manager does not
// hold. See ReadInterviewResponses.
func (s *serviceImpl) GetInterview(ctx context.Context, orgID string, caller Caller, ref string) (*ExitInterview, error) {
	e, err := s.loadExit(ctx, orgID, caller, ref)
	if err != nil {
		return nil, err
	}
	i, err := s.repo.FindInterviewByExit(ctx, orgID, e.ID)
	if err != nil {
		return nil, fmt.Errorf("exits: GetInterview: %w", err)
	}
	if i == nil {
		return nil, ErrInterviewNotFound
	}
	return i, nil
}

// SendInterview instantiates the form and marks the interview sent.
//
// Exported separately from the sweep so HR can send one by hand — but it
// still refuses before the scheduled date, because the timing IS the
// confidentiality mechanism here, not a convenience.
func (s *serviceImpl) SendInterview(ctx context.Context, orgID string, caller Caller, ref string) (*ExitInterview, error) {
	e, err := s.loadExit(ctx, orgID, caller, ref)
	if err != nil {
		return nil, err
	}
	i, err := s.repo.FindInterviewByExit(ctx, orgID, e.ID)
	if err != nil {
		return nil, fmt.Errorf("exits: SendInterview: %w", err)
	}
	if i == nil {
		return nil, ErrInterviewNotFound
	}
	if i.Status != InterviewScheduled {
		return nil, ErrWrongStatus
	}
	if truncateToDay(time.Now()).Before(truncateToDay(i.ScheduledFor)) {
		return nil, ErrInterviewNotDue
	}
	if err := s.sendInterview(ctx, i, e); err != nil {
		return nil, err
	}
	return i, nil
}

// sendInterview instantiates the org's default exit-interview form for the
// departing employee. Shared by the manual path and the sweep.
func (s *serviceImpl) sendInterview(ctx context.Context, i *ExitInterview, e *Exit) error {
	if s.forms == nil {
		return ErrNoInterviewTemplate
	}
	subject, err := s.repo.FindSubject(ctx, i.OrgID, e.EmployeeID)
	if err != nil {
		return fmt.Errorf("exits: sendInterview: %w", err)
	}
	if subject == nil {
		return ErrExitNotFound
	}

	instance, err := s.forms.InstantiateDefault(ctx, i.OrgID, forms.FormTypeExitInterview,
		forms.SubjectContext{
			SubjectType:  forms.SubjectEmployee,
			SubjectID:    subject.EmployeeID,
			SubjectLabel: subject.DisplayName,
			// The departing employee answers about themselves — subject and
			// respondent are the same person here, unlike an appraisal.
			RespondentUserID: subject.UserID,
			RespondentRole:   "self",
			CreatedBy:        i.CreatedBy,
		})
	if err != nil {
		return fmt.Errorf("exits: sendInterview: instantiate: %w", err)
	}
	if instance == nil {
		// No default template configured. Reported rather than silently
		// marking the interview sent with nothing behind it.
		return ErrNoInterviewTemplate
	}

	now := time.Now()
	i.FormInstanceID = &instance.Instance.ID
	i.Status = InterviewSent
	i.SentAt = &now
	return s.repo.UpdateInterview(ctx, i)
}

// RunInterviewSweep sends every interview whose scheduled date has arrived.
//
// Instance-wide, the benefits.activate_pending_enrollments shape. A single
// interview failing does not abort the sweep — one org with no template
// configured must not stop every other org's interviews going out.
func (s *serviceImpl) RunInterviewSweep(ctx context.Context, asOf time.Time) (int, error) {
	due, err := s.repo.FindDueInterviews(ctx, asOf, sweepBatchLimit)
	if err != nil {
		return 0, err
	}
	sent := 0
	for _, i := range due {
		e, err := s.repo.FindExitByRef(ctx, i.OrgID, i.ExitID)
		if err != nil || e == nil {
			slog.Warn("exits: interview sweep: exit not found",
				slog.String("interview_id", i.ID), slog.Any("error", err))
			continue
		}
		if err := s.sendInterview(ctx, i, e); err != nil {
			slog.Warn("exits: interview sweep: send failed",
				slog.String("interview_id", i.ID), slog.String("org_id", i.OrgID),
				slog.Any("error", err))
			continue
		}
		sent++
	}
	return sent, nil
}

// sweepBatchLimit caps how much either sweep does per run. Both are nightly
// and the backlog is normally tiny; the cap exists so a first run against a
// long-neglected deployment cannot hold a connection for minutes.
const sweepBatchLimit = 500

// ── Access revocation ────────────────────────────────────────────────────────

// RunAccessRevocationSweep suspends membership and kills live sessions for
// every exit whose last working date has passed.
//
// ⚠ IDEMPOTENT BY CONSTRUCTION. The query filters on access_revoked_at IS
// NULL, and the stamp is written after a successful revocation — so a
// revoked exit leaves the set permanently rather than being re-revoked every
// night. Both underlying operations are also naturally repeatable: suspending
// an already-suspended member and logging out an already-logged-out user are
// both no-ops.
//
// This is destructive but REVERSIBLE: the membership is suspended, not
// deleted, an admin can re-activate it, and every HR record is untouched.
func (s *serviceImpl) RunAccessRevocationSweep(ctx context.Context, asOf time.Time) (int, error) {
	due, err := s.repo.FindExitsDueForRevocation(ctx, asOf, sweepBatchLimit)
	if err != nil {
		return 0, err
	}
	revoked := 0
	for _, e := range due {
		if err := s.revokeAccessFor(ctx, e); err != nil {
			// One employee's failure must not stop the rest. A departure
			// whose access stays live is exactly what this exists to prevent,
			// so it is logged loudly and retried on the next run — the stamp
			// is only written on success.
			slog.Error("exits: access revocation failed",
				slog.String("exit_id", e.ID), slog.String("org_id", e.OrgID),
				slog.Any("error", err))
			continue
		}
		revoked++
	}
	return revoked, nil
}

// RevokeAccessNow is the manual path: a dismissal for cause does not wait for
// tonight's cron. Gated on hrm.exits.revoke_access.
func (s *serviceImpl) RevokeAccessNow(ctx context.Context, orgID string, caller Caller, ref string) (*Exit, error) {
	e, err := s.loadExit(ctx, orgID, caller, ref)
	if err != nil {
		return nil, err
	}
	if e.AccessRevokedAt != nil {
		// Already done. Returning the exit rather than an error keeps the
		// manual path as idempotent as the sweep.
		return e, nil
	}
	if err := s.revokeAccessFor(ctx, e); err != nil {
		return nil, err
	}
	return e, nil
}

// revokeAccessFor does the work for one exit and stamps it, in that order.
//
// The stamp is written LAST and only on success. Writing it first would mean
// a failure halfway leaves the exit marked revoked with access still live —
// the sweep would then skip it forever, which is the worst available outcome
// for a feature whose whole job is closing off access.
func (s *serviceImpl) revokeAccessFor(ctx context.Context, e *Exit) error {
	userID, err := s.repo.FindEmployeeUserID(ctx, e.OrgID, e.EmployeeID)
	if err != nil {
		return err
	}

	if userID != "" {
		if s.suspender != nil {
			if err := s.suspender.SuspendMembership(ctx, e.OrgID, userID); err != nil {
				return fmt.Errorf("suspend membership: %w", err)
			}
		}
		if s.sessions != nil {
			if err := s.sessions.LogoutAll(ctx, userID); err != nil {
				return fmt.Errorf("revoke sessions: %w", err)
			}
		}
	}
	// An employee with no platform account still gets stamped: there was
	// nothing to revoke, and leaving them unstamped would make the sweep
	// retry them every night forever.

	now := time.Now()
	e.AccessRevokedAt = &now
	return s.repo.UpdateExit(ctx, e)
}

// ReadInterviewResponses returns what the departing employee actually said.
//
// ⚠ THIS IS THE CONFIDENTIAL PATH, and it is the reason .interview_view
// exists as a separate permission from .interview. Scheduling an interview is
// administrative; reading it is not. A manager holds view_team over exits and
// can therefore see that an interview happened — they must not be able to see
// what was said about them, and no role below admin is granted the key.
//
// Separating READ from SCHEDULE at the permission layer is what makes that
// structural rather than a matter of who remembers to check. The 5C
// 360-feedback precedent: the protection is which query the caller is
// allowed to reach, not a field somebody strips afterwards.
func (s *serviceImpl) ReadInterviewResponses(ctx context.Context, orgID string, caller Caller, ref string) (*forms.InstanceWithResponses, error) {
	if !caller.CanViewInterviews {
		return nil, ErrAccessDenied
	}
	e, err := s.loadExit(ctx, orgID, caller, ref)
	if err != nil {
		return nil, err
	}
	i, err := s.repo.FindInterviewByExit(ctx, orgID, e.ID)
	if err != nil {
		return nil, fmt.Errorf("exits: ReadInterviewResponses: %w", err)
	}
	if i == nil {
		return nil, ErrInterviewNotFound
	}
	if i.FormInstanceID == nil {
		// Scheduled but never sent — there is nothing to read, which is not
		// the same as an empty set of answers.
		return nil, ErrInterviewNotFound
	}
	if s.forms == nil {
		return nil, ErrNoInterviewTemplate
	}
	return s.forms.GetInstance(ctx, orgID, *i.FormInstanceID)
}

// ── Document issuance ────────────────────────────────────────────────────────

// DocumentIssuanceEligibility reports whether each exit document may be
// issued yet.
//
// The relieving letter is the ONE place clearance and the settlement — which
// are tracked independently — have to agree, because it is the document that
// says the organization considers the person fully departed and owes nothing.
//
// The experience letter is deliberately NOT blocked. It states employment
// dates, which are true regardless of what is owed, and withholding it would
// punish somebody for a dispute about money by making them unemployable while
// it is resolved.
func (s *serviceImpl) DocumentIssuanceEligibility(ctx context.Context, orgID string, caller Caller, ref string) ([]*DocumentEligibility, error) {
	e, err := s.loadExit(ctx, orgID, caller, ref)
	if err != nil {
		return nil, err
	}

	items, err := s.repo.FindClearanceItems(ctx, orgID, e.ID)
	if err != nil {
		return nil, fmt.Errorf("exits: DocumentIssuanceEligibility: %w", err)
	}
	summary := summariseClearance(items)

	out := []*DocumentEligibility{{
		DocumentType: "experience_letter",
		Eligible:     true,
	}}

	relieving := &DocumentEligibility{DocumentType: "relieving_letter", Eligible: true}
	switch {
	case summary.BlockingItems > 0:
		relieving.Eligible = false
		relieving.Reason = fmt.Sprintf(
			"clearance is incomplete: %d item(s) with %s outstanding",
			summary.BlockingItems, summary.OutstandingDues.String())
	case e.FnFPayslipRunID == nil:
		relieving.Eligible = false
		relieving.Reason = "the final settlement has not been run"
	case e.Status != StatusSettled && e.Status != StatusCompleted:
		relieving.Eligible = false
		relieving.Reason = "the final settlement has not been approved"
	}
	out = append(out, relieving)
	return out, nil
}
