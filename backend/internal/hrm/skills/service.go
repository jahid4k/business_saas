// backend/internal/hrm/skills/service.go
package skills

import (
	"context"
	"fmt"
	"strings"
	"time"
)

type Service interface {
	// ── Taxonomy ────────────────────────────────────────────────────────
	ListSkills(ctx context.Context, orgID string, f SkillListFilter) (*SkillListResponse, error)
	GetSkill(ctx context.Context, orgID, ref string) (*Skill, error)
	CreateSkill(ctx context.Context, orgID, createdBy string, req CreateSkillRequest) (*Skill, error)
	UpdateSkill(ctx context.Context, orgID, ref string, req UpdateSkillRequest) (*Skill, error)
	DeleteSkill(ctx context.Context, orgID, ref string) error

	// ── Employee skills ─────────────────────────────────────────────────
	ListEmployeeSkills(ctx context.Context, orgID string, caller Caller, f EmployeeSkillListFilter) (*EmployeeSkillListResponse, error)
	GrantSkill(ctx context.Context, orgID string, caller Caller, req GrantSkillRequest) (*EmployeeSkill, error)
	UpdateEmployeeSkill(ctx context.Context, orgID, ref string, caller Caller, req UpdateEmployeeSkillRequest) (*EmployeeSkill, error)
	RevokeSkill(ctx context.Context, orgID, ref string, caller Caller) error

	// GrantFromSource records a skill acquired by completing a course or
	// earning a certification. Called SERVICE-TO-SERVICE, never from a route:
	// the source ids are resolved by the caller from its own domain, exactly
	// like the form engine's instantiate.
	GrantFromSource(ctx context.Context, orgID, employeeID, skillID string, source Source, sourceID string) (*EmployeeSkill, error)
}

type serviceImpl struct {
	repo    Repository
	records RecordAuthorizer
}

func NewService(repo Repository, records RecordAuthorizer) Service {
	return &serviceImpl{repo: repo, records: records}
}

// ── Taxonomy ─────────────────────────────────────────────────────────────────
//
// The taxonomy is org-level and NOT scope-filtered: it carries no employee_id,
// so scope.Predicate — which hard-codes FROM hrm_employees — has nothing to
// filter on. A catalogue nobody can read is useless.

func (s *serviceImpl) ListSkills(ctx context.Context, orgID string, f SkillListFilter) (*SkillListResponse, error) {
	f.Normalise()
	list, err := s.repo.FindSkills(ctx, orgID, f)
	if err != nil {
		return nil, err
	}
	total, err := s.repo.CountSkills(ctx, orgID, f)
	if err != nil {
		return nil, err
	}
	return &SkillListResponse{Skills: list, Total: total, Limit: f.Limit, Offset: f.Offset}, nil
}

func (s *serviceImpl) GetSkill(ctx context.Context, orgID, ref string) (*Skill, error) {
	sk, err := s.repo.FindSkillByRef(ctx, orgID, ref)
	if err != nil {
		return nil, err
	}
	if sk == nil {
		return nil, ErrSkillNotFound
	}
	return sk, nil
}

func (s *serviceImpl) CreateSkill(ctx context.Context, orgID, createdBy string, req CreateSkillRequest) (*Skill, error) {
	name := strings.TrimSpace(req.Name)
	if name == "" {
		return nil, ErrNameRequired
	}
	taken, err := s.repo.SkillNameExists(ctx, orgID, name, "")
	if err != nil {
		return nil, err
	}
	if taken {
		return nil, ErrNameTaken
	}

	sk := &Skill{
		OrgID: orgID, Name: name,
		Description: nilIfBlank(req.Description), Category: nilIfBlank(req.Category),
		CreatedBy: createdBy,
	}
	if err := s.repo.CreateSkill(ctx, sk); err != nil {
		return nil, err
	}
	return sk, nil
}

func (s *serviceImpl) UpdateSkill(ctx context.Context, orgID, ref string, req UpdateSkillRequest) (*Skill, error) {
	sk, err := s.GetSkill(ctx, orgID, ref)
	if err != nil {
		return nil, err
	}
	if req.Name != nil {
		name := strings.TrimSpace(*req.Name)
		if name == "" {
			return nil, ErrNameRequired
		}
		taken, err := s.repo.SkillNameExists(ctx, orgID, name, sk.ID)
		if err != nil {
			return nil, err
		}
		if taken {
			return nil, ErrNameTaken
		}
		sk.Name = name
	}
	if req.Description != nil {
		sk.Description = nilIfBlank(req.Description)
	}
	if req.Category != nil {
		sk.Category = nilIfBlank(req.Category)
	}
	if req.IsActive != nil {
		sk.IsActive = *req.IsActive
	}
	if err := s.repo.UpdateSkill(ctx, sk); err != nil {
		return nil, err
	}
	return sk, nil
}

// DeleteSkill refuses when anybody holds it.
//
// hrm_employee_skills.skill_id is ON DELETE CASCADE, so without this check
// deleting a taxonomy entry would silently erase every employee's record of
// having it — a data loss with no error. Deactivating is the intended way to
// retire a skill, which is what is_active is for.
func (s *serviceImpl) DeleteSkill(ctx context.Context, orgID, ref string) error {
	sk, err := s.GetSkill(ctx, orgID, ref)
	if err != nil {
		return err
	}
	inUse, err := s.repo.SkillInUse(ctx, sk.ID)
	if err != nil {
		return err
	}
	if inUse {
		return ErrSkillInUse
	}
	return s.repo.DeleteSkill(ctx, orgID, sk.ID)
}

// ── Employee skills ──────────────────────────────────────────────────────────

func (s *serviceImpl) ListEmployeeSkills(ctx context.Context, orgID string, caller Caller, f EmployeeSkillListFilter) (*EmployeeSkillListResponse, error) {
	f.Normalise()
	f.Scope = caller.Tier
	f.CallerUserID = caller.UserID

	list, err := s.repo.FindEmployeeSkills(ctx, orgID, f)
	if err != nil {
		return nil, err
	}
	total, err := s.repo.CountEmployeeSkills(ctx, orgID, f)
	if err != nil {
		return nil, err
	}
	return &EmployeeSkillListResponse{Skills: list, Total: total, Limit: f.Limit, Offset: f.Offset}, nil
}

// authorizeWrite narrows a write. hrm.skills.manage is unscoped at the route,
// so this record check is the only thing stopping a view_team manager
// recording skills for somebody outside their reporting line.
func (s *serviceImpl) authorizeWrite(ctx context.Context, orgID, employeeID string, caller Caller) error {
	if !caller.CanManage {
		return ErrAccessDenied
	}
	ok, err := s.records.AuthorizeRecordAccess(ctx, caller.Tier, orgID, caller.UserID, employeeID)
	if err != nil {
		return fmt.Errorf("skills: authorize write: %w", err)
	}
	if !ok {
		return ErrAccessDenied
	}
	return nil
}

func (s *serviceImpl) GrantSkill(ctx context.Context, orgID string, caller Caller, req GrantSkillRequest) (*EmployeeSkill, error) {
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

	sk, err := s.GetSkill(ctx, orgID, strings.TrimSpace(req.SkillID))
	if err != nil {
		return nil, err
	}
	if !sk.IsActive {
		return nil, ErrSkillInactive
	}

	var prof *Proficiency
	if req.Proficiency != nil && strings.TrimSpace(*req.Proficiency) != "" {
		p := Proficiency(strings.TrimSpace(*req.Proficiency))
		if !p.IsValid() {
			return nil, ErrInvalidProfic
		}
		prof = &p
	}
	acquired, err := parseDate(req.AcquiredOn)
	if err != nil {
		return nil, err
	}

	// uq_hrm_eskl_employee_skill is the guarantee; this is the message.
	held, err := s.repo.HasSkill(ctx, orgID, employeeID, sk.ID)
	if err != nil {
		return nil, err
	}
	if held {
		return nil, ErrAlreadyGranted
	}

	es := &EmployeeSkill{
		OrgID: orgID, EmployeeID: employeeID, SkillID: sk.ID,
		Proficiency: prof, Source: SourceManual,
		AcquiredOn: acquired, Notes: nilIfBlank(req.Notes),
	}
	if caller.UserID != "" {
		es.CreatedBy = &caller.UserID
	}
	if err := s.repo.GrantSkill(ctx, es); err != nil {
		return nil, err
	}
	return es, nil
}

// GrantFromSource is the service-to-service entry point used when completing a
// course or earning a certification grants a skill.
//
// It is deliberately NOT reachable from a route: the source id is resolved by
// the CALLING module from its own domain, so a client can never assert "this
// course granted me that skill". Same reasoning as the form engine having no
// generic instantiate route.
//
// It is idempotent — re-running a completion must not fail. An employee who
// already holds the skill keeps their existing record, including a manually
// set proficiency somebody may have curated.
func (s *serviceImpl) GrantFromSource(ctx context.Context, orgID, employeeID, skillID string, source Source, sourceID string) (*EmployeeSkill, error) {
	if !source.IsValid() {
		return nil, fmt.Errorf("skills: invalid source %q", source)
	}
	held, err := s.repo.HasSkill(ctx, orgID, employeeID, skillID)
	if err != nil {
		return nil, err
	}
	if held {
		// Already recorded. Returning nil, nil would make the caller branch on
		// a nil result; returning the existing row keeps the call total.
		f := EmployeeSkillListFilter{EmployeeID: employeeID, SkillID: skillID, Limit: 1}
		f.Normalise()
		existing, err := s.repo.FindEmployeeSkills(ctx, orgID, f)
		if err != nil {
			return nil, err
		}
		if len(existing) > 0 {
			return existing[0], nil
		}
		return nil, nil
	}

	now := time.Now()
	es := &EmployeeSkill{
		OrgID: orgID, EmployeeID: employeeID, SkillID: skillID,
		Source: source, AcquiredOn: &now,
	}
	switch source {
	case SourceCourse:
		es.SourceEnrollmentID = &sourceID
	case SourceCertification:
		es.SourceCertificationID = &sourceID
	}
	if err := s.repo.GrantSkill(ctx, es); err != nil {
		return nil, err
	}
	return es, nil
}

func (s *serviceImpl) UpdateEmployeeSkill(ctx context.Context, orgID, ref string, caller Caller, req UpdateEmployeeSkillRequest) (*EmployeeSkill, error) {
	es, err := s.loadEmployeeSkill(ctx, orgID, ref, caller)
	if err != nil {
		return nil, err
	}
	if err := s.authorizeWrite(ctx, orgID, es.EmployeeID, caller); err != nil {
		return nil, err
	}

	if req.Proficiency != nil {
		if strings.TrimSpace(*req.Proficiency) == "" {
			es.Proficiency = nil
		} else {
			p := Proficiency(strings.TrimSpace(*req.Proficiency))
			if !p.IsValid() {
				return nil, ErrInvalidProfic
			}
			es.Proficiency = &p
		}
	}
	if req.AcquiredOn != nil {
		acquired, err := parseDate(req.AcquiredOn)
		if err != nil {
			return nil, err
		}
		es.AcquiredOn = acquired
	}
	if req.Notes != nil {
		es.Notes = nilIfBlank(req.Notes)
	}

	// Note what is absent: source and its ids. A skill granted by passing a
	// course must not be re-labelled as manually asserted — the provenance is
	// the reason the distinction exists.
	if err := s.repo.UpdateEmployeeSkill(ctx, es); err != nil {
		return nil, err
	}
	return es, nil
}

func (s *serviceImpl) RevokeSkill(ctx context.Context, orgID, ref string, caller Caller) error {
	es, err := s.loadEmployeeSkill(ctx, orgID, ref, caller)
	if err != nil {
		return err
	}
	if err := s.authorizeWrite(ctx, orgID, es.EmployeeID, caller); err != nil {
		return err
	}
	return s.repo.RevokeSkill(ctx, orgID, es.ID)
}

// loadEmployeeSkill fetches org-scoped, then narrows by the caller's tier —
// the fetch-by-id half of the control the list path applies in SQL.
func (s *serviceImpl) loadEmployeeSkill(ctx context.Context, orgID, ref string, caller Caller) (*EmployeeSkill, error) {
	es, err := s.repo.FindEmployeeSkillByRef(ctx, orgID, ref)
	if err != nil {
		return nil, err
	}
	if es == nil {
		return nil, ErrEmployeeSkillNotFound
	}
	ok, err := s.records.AuthorizeRecordAccess(ctx, caller.Tier, orgID, caller.UserID, es.EmployeeID)
	if err != nil {
		return nil, fmt.Errorf("skills: authorize: %w", err)
	}
	if !ok {
		return nil, ErrAccessDenied
	}
	return es, nil
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

func parseDate(s *string) (*time.Time, error) {
	if s == nil || strings.TrimSpace(*s) == "" {
		return nil, nil
	}
	t, err := time.Parse(dateLayout, strings.TrimSpace(*s))
	if err != nil {
		return nil, ErrInvalidDate
	}
	return &t, nil
}
