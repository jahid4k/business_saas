// backend/internal/hrm/certifications/service.go
package certifications

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/mridha/businesssaas/internal/hrm/skills"
)

type Service interface {
	// ── Catalogue ───────────────────────────────────────────────────────
	List(ctx context.Context, orgID string, f CertificationListFilter) (*CertificationListResponse, error)
	Get(ctx context.Context, orgID, ref string) (*Certification, error)
	Create(ctx context.Context, orgID, createdBy string, req CreateCertificationRequest) (*Certification, error)
	Update(ctx context.Context, orgID, ref string, req UpdateCertificationRequest) (*Certification, error)
	Delete(ctx context.Context, orgID, ref string) error

	// ── Employee credentials ────────────────────────────────────────────
	ListEmployeeCertifications(ctx context.Context, orgID string, caller Caller, f EmployeeCertificationListFilter) (*EmployeeCertificationListResponse, error)
	Issue(ctx context.Context, orgID string, caller Caller, req IssueRequest) (*EmployeeCertification, error)
	UpdateEmployeeCertification(ctx context.Context, orgID, ref string, caller Caller, req UpdateEmployeeCertificationRequest) (*EmployeeCertification, error)
	Revoke(ctx context.Context, orgID, ref string, caller Caller, req RevokeRequest) (*EmployeeCertification, error)

	// SweepExpiries is the scheduler job. Registered in main.go, not exposed
	// as a route: it runs across every org in the instance.
	SweepExpiries(ctx context.Context) (SweepResult, error)
}

type serviceImpl struct {
	repo    Repository
	records RecordAuthorizer
	// skills is optional: a deployment without the taxonomy wired still issues
	// credentials, they just record no skill.
	skills SkillGranter
}

func NewService(repo Repository, records RecordAuthorizer, skillGranter SkillGranter) Service {
	return &serviceImpl{repo: repo, records: records, skills: skillGranter}
}

// ── Catalogue ────────────────────────────────────────────────────────────────

func (s *serviceImpl) List(ctx context.Context, orgID string, f CertificationListFilter) (*CertificationListResponse, error) {
	f.Normalise()
	list, err := s.repo.FindCertifications(ctx, orgID, f)
	if err != nil {
		return nil, err
	}
	total, err := s.repo.CountCertifications(ctx, orgID, f)
	if err != nil {
		return nil, err
	}
	return &CertificationListResponse{Certifications: list, Total: total, Limit: f.Limit, Offset: f.Offset}, nil
}

func (s *serviceImpl) Get(ctx context.Context, orgID, ref string) (*Certification, error) {
	c, err := s.repo.FindByRef(ctx, orgID, ref)
	if err != nil {
		return nil, err
	}
	if c == nil {
		return nil, ErrNotFound
	}
	return c, nil
}

func (s *serviceImpl) Create(ctx context.Context, orgID, createdBy string, req CreateCertificationRequest) (*Certification, error) {
	name := strings.TrimSpace(req.Name)
	if name == "" {
		return nil, ErrNameRequired
	}
	if req.ValidityMonths != nil && *req.ValidityMonths <= 0 {
		return nil, ErrInvalidValidity
	}
	taken, err := s.repo.NameExists(ctx, orgID, name, "")
	if err != nil {
		return nil, err
	}
	if taken {
		return nil, ErrNameTaken
	}

	c := &Certification{
		OrgID: orgID, Name: name,
		Description: nilIfBlank(req.Description), IssuingBody: nilIfBlank(req.IssuingBody),
		ValidityMonths: req.ValidityMonths,
		CourseID:       nilIfBlank(req.CourseID), SkillID: nilIfBlank(req.SkillID),
		CreatedBy: createdBy,
	}
	if err := s.repo.Create(ctx, c); err != nil {
		return nil, err
	}
	return c, nil
}

func (s *serviceImpl) Update(ctx context.Context, orgID, ref string, req UpdateCertificationRequest) (*Certification, error) {
	c, err := s.Get(ctx, orgID, ref)
	if err != nil {
		return nil, err
	}
	if req.Name != nil {
		name := strings.TrimSpace(*req.Name)
		if name == "" {
			return nil, ErrNameRequired
		}
		taken, err := s.repo.NameExists(ctx, orgID, name, c.ID)
		if err != nil {
			return nil, err
		}
		if taken {
			return nil, ErrNameTaken
		}
		c.Name = name
	}
	if req.Description != nil {
		c.Description = nilIfBlank(req.Description)
	}
	if req.IssuingBody != nil {
		c.IssuingBody = nilIfBlank(req.IssuingBody)
	}
	if req.ValidityMonths != nil {
		if *req.ValidityMonths <= 0 {
			return nil, ErrInvalidValidity
		}
		c.ValidityMonths = req.ValidityMonths
	}
	if req.CourseID != nil {
		c.CourseID = nilIfBlank(req.CourseID)
	}
	if req.SkillID != nil {
		c.SkillID = nilIfBlank(req.SkillID)
	}
	if req.IsActive != nil {
		c.IsActive = *req.IsActive
	}

	// Note what changing validity_months does NOT do: it never moves an
	// already-issued credential's expires_at. Those are frozen at issue, so a
	// policy change cannot retroactively lapse somebody's licence.
	if err := s.repo.Update(ctx, c); err != nil {
		return nil, err
	}
	return c, nil
}

func (s *serviceImpl) Delete(ctx context.Context, orgID, ref string) error {
	c, err := s.Get(ctx, orgID, ref)
	if err != nil {
		return err
	}
	inUse, err := s.repo.CertificationInUse(ctx, c.ID)
	if err != nil {
		return err
	}
	if inUse {
		return ErrCertInUse
	}
	return s.repo.Delete(ctx, orgID, c.ID)
}

// ── Employee credentials ─────────────────────────────────────────────────────

func (s *serviceImpl) ListEmployeeCertifications(ctx context.Context, orgID string, caller Caller, f EmployeeCertificationListFilter) (*EmployeeCertificationListResponse, error) {
	f.Normalise()
	f.Scope = caller.Tier
	f.CallerUserID = caller.UserID

	list, err := s.repo.FindEmployeeCertifications(ctx, orgID, f)
	if err != nil {
		return nil, err
	}
	total, err := s.repo.CountEmployeeCertifications(ctx, orgID, f)
	if err != nil {
		return nil, err
	}
	return &EmployeeCertificationListResponse{
		Certifications: list, Total: total, Limit: f.Limit, Offset: f.Offset,
	}, nil
}

// authorizeWrite narrows an issue or revoke. hrm.certifications.manage is
// unscoped at the route, so this record check is the only thing stopping a
// view_team manager issuing credentials outside their reporting line.
func (s *serviceImpl) authorizeWrite(ctx context.Context, orgID, employeeID string, caller Caller) error {
	if !caller.CanManage {
		return ErrAccessDenied
	}
	ok, err := s.records.AuthorizeRecordAccess(ctx, caller.Tier, orgID, caller.UserID, employeeID)
	if err != nil {
		return fmt.Errorf("certifications: authorize write: %w", err)
	}
	if !ok {
		return ErrAccessDenied
	}
	return nil
}

// Issue records a credential and, when the certification carries a skill,
// records that skill against the employee too.
//
// The expiry date is DERIVED from validity_months when not given explicitly,
// and frozen here. A later change to the catalogue's validity never moves an
// issued credential — see Update.
func (s *serviceImpl) Issue(ctx context.Context, orgID string, caller Caller, req IssueRequest) (*EmployeeCertification, error) {
	employeeID, err := s.repo.EmployeeExists(ctx, orgID, strings.TrimSpace(req.EmployeeID))
	if err != nil {
		return nil, err
	}
	if employeeID == "" {
		return nil, ErrEmployeeNotFound
	}
	if err := s.authorizeWrite(ctx, orgID, employeeID, caller); err != nil {
		return nil, err
	}

	cert, err := s.Get(ctx, orgID, strings.TrimSpace(req.CertificationID))
	if err != nil {
		return nil, err
	}
	if !cert.IsActive {
		return nil, ErrCertInactive
	}

	issuedOn, err := time.Parse(dateLayout, strings.TrimSpace(req.IssuedOn))
	if err != nil {
		return nil, ErrInvalidDate
	}

	var expiresAt *time.Time
	switch {
	case req.ExpiresAt != nil && strings.TrimSpace(*req.ExpiresAt) != "":
		parsed, err := time.Parse(dateLayout, strings.TrimSpace(*req.ExpiresAt))
		if err != nil {
			return nil, ErrInvalidDate
		}
		expiresAt = &parsed
	case cert.ValidityMonths != nil:
		// AddDate, not raw duration arithmetic: months are not a fixed number
		// of hours, and DST would drift a 24-month validity by a day.
		derived := issuedOn.AddDate(0, *cert.ValidityMonths, 0)
		expiresAt = &derived
	}
	// A NULL validity means the credential never expires, which stays
	// distinguishable from "expires today".
	if expiresAt != nil && expiresAt.Before(issuedOn) {
		return nil, ErrExpiryBeforeIssue
	}

	// uq_hrm_ecrt_employee_cert_live is the guarantee; this is the message.
	live, err := s.repo.HasLiveCredential(ctx, orgID, employeeID, cert.ID)
	if err != nil {
		return nil, err
	}
	if live {
		return nil, ErrAlreadyHeld
	}

	ec := &EmployeeCertification{
		OrgID: orgID, EmployeeID: employeeID, CertificationID: cert.ID,
		EnrollmentID: nilIfBlank(req.EnrollmentID), CredentialID: nilIfBlank(req.CredentialID),
		IssuedOn: issuedOn, ExpiresAt: expiresAt, Notes: nilIfBlank(req.Notes),
	}
	if caller.UserID != "" {
		ec.IssuedBy = &caller.UserID
	}
	if err := s.repo.Issue(ctx, ec); err != nil {
		return nil, err
	}

	// The in-phase consumer that justifies the skills taxonomy existing now.
	// Best-effort by design: a credential that issued successfully must not be
	// rolled back because the derived skill record failed, and GrantFromSource
	// is idempotent so a retry is safe.
	if cert.SkillID != nil && s.skills != nil {
		if _, err := s.skills.GrantFromSource(ctx, orgID, employeeID, *cert.SkillID,
			skills.SourceCertification, ec.ID); err != nil {
			return ec, fmt.Errorf("certifications: credential issued but skill not recorded: %w", err)
		}
	}
	return ec, nil
}

func (s *serviceImpl) UpdateEmployeeCertification(ctx context.Context, orgID, ref string, caller Caller, req UpdateEmployeeCertificationRequest) (*EmployeeCertification, error) {
	ec, err := s.loadEmployeeCert(ctx, orgID, ref, caller)
	if err != nil {
		return nil, err
	}
	if err := s.authorizeWrite(ctx, orgID, ec.EmployeeID, caller); err != nil {
		return nil, err
	}

	if req.CredentialID != nil {
		ec.CredentialID = nilIfBlank(req.CredentialID)
	}
	if req.Notes != nil {
		ec.Notes = nilIfBlank(req.Notes)
	}
	if req.ExpiresAt != nil {
		if strings.TrimSpace(*req.ExpiresAt) == "" {
			ec.ExpiresAt = nil
		} else {
			parsed, err := time.Parse(dateLayout, strings.TrimSpace(*req.ExpiresAt))
			if err != nil {
				return nil, ErrInvalidDate
			}
			if parsed.Before(ec.IssuedOn) {
				return nil, ErrExpiryBeforeIssue
			}
			ec.ExpiresAt = &parsed
		}
	}
	if err := s.repo.UpdateEmployeeCert(ctx, ec); err != nil {
		return nil, err
	}
	return ec, nil
}

func (s *serviceImpl) Revoke(ctx context.Context, orgID, ref string, caller Caller, req RevokeRequest) (*EmployeeCertification, error) {
	ec, err := s.loadEmployeeCert(ctx, orgID, ref, caller)
	if err != nil {
		return nil, err
	}
	if err := s.authorizeWrite(ctx, orgID, ec.EmployeeID, caller); err != nil {
		return nil, err
	}
	if ec.Status == StatusRevoked {
		return nil, ErrAlreadyRevoked
	}
	// Revoking frees the employee to be re-issued, which is what makes the
	// unique index partial rather than absolute.
	return s.repo.SetStatus(ctx, orgID, ec.ID, StatusRevoked)
}

func (s *serviceImpl) loadEmployeeCert(ctx context.Context, orgID, ref string, caller Caller) (*EmployeeCertification, error) {
	ec, err := s.repo.FindEmployeeCertByRef(ctx, orgID, ref)
	if err != nil {
		return nil, err
	}
	if ec == nil {
		return nil, ErrEmployeeCertNotFound
	}
	ok, err := s.records.AuthorizeRecordAccess(ctx, caller.Tier, orgID, caller.UserID, ec.EmployeeID)
	if err != nil {
		return nil, fmt.Errorf("certifications: authorize: %w", err)
	}
	if !ok {
		return nil, ErrAccessDenied
	}
	return ec, nil
}

// ── The expiry sweep ─────────────────────────────────────────────────────────

// SweepExpiries is the nightly job the build plan calls the highest-value
// feature in Phase 6.
//
// ORDER MATTERS. MarkExpiring runs first and only touches credentials still in
// the future; MarkExpired then catches anything already past. Reversing them
// would flip a credential that lapsed yesterday to 'expiring' — a warning
// about something that has already happened.
//
// It runs across EVERY org in the instance, the same shape as the leave
// accrual and absence sweep jobs, because the scheduler is instance-wide.
func (s *serviceImpl) SweepExpiries(ctx context.Context) (SweepResult, error) {
	var res SweepResult

	expiring, err := s.repo.MarkExpiring(ctx, ExpiryWindowDays)
	if err != nil {
		return res, err
	}
	res.MarkedExpiring = expiring

	expired, err := s.repo.MarkExpired(ctx)
	if err != nil {
		// The expiring pass already committed; reporting the partial count
		// alongside the error beats claiming nothing happened.
		return res, err
	}
	res.MarkedExpired = expired

	return res, nil
}

// ── Helpers ──────────────────────────────────────────────────────────────────

func nilIfBlank(s *string) *string {
	if s == nil {
		return nil
	}
	t := strings.TrimSpace(*s)
	if t == "" {
		return nil
	}
	return &t
}
