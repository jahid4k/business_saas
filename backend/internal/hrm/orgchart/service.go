// backend/internal/hrm/orgchart/service.go
package orgchart

import (
	"context"
	"fmt"
	"strings"
	"time"
)

// Caller carries identity and the manage authority the route gate cannot
// express on its own.
type Caller struct {
	UserID    string
	CanManage bool
}

// Service is the org chart's business layer.
//
// The chart is NOT scope-tiered: a chart whose shape depends on who is
// looking is a subtree, not a chart, and succession and analytics both need
// the whole graph. What is sensitive is the salary and appraisal data hanging
// off each node, and that stays behind its own already-tiered resources.
type Service interface {
	// Relationships
	CreateRelationship(ctx context.Context, orgID string, caller Caller, req CreateRelationshipRequest) (*Relationship, error)
	ListRelationships(ctx context.Context, orgID string, caller Caller, employeeID string, activeOnly bool) ([]*Relationship, error)
	EndRelationship(ctx context.Context, orgID string, caller Caller, ref string, req EndRelationshipRequest) (*Relationship, error)

	// Chart
	GetChart(ctx context.Context, orgID string, caller Caller) ([]*ChartNode, error)
	// ManagementChain returns the solid-line chain above an employee, nearest
	// first — what an approval router or an escalation walks.
	ManagementChain(ctx context.Context, orgID string, caller Caller, employeeID string) ([]string, error)

	// Seats
	CreateSeat(ctx context.Context, orgID string, caller Caller, req CreateSeatRequest) (*Seat, error)
	ListSeats(ctx context.Context, orgID string, caller Caller, positionID string, vacantOnly bool) ([]*Seat, error)
	AssignSeat(ctx context.Context, orgID string, caller Caller, ref string, req AssignSeatRequest) (*Seat, error)
}

type serviceImpl struct{ repo Repository }

func NewService(repo Repository) Service { return &serviceImpl{repo: repo} }

// CreateRelationship validates the line and refuses a cycle BEFORE inserting.
//
// ⚠ The cycle check is an AUTHORIZATION safety check, not data tidiness.
// scope.Predicate's view_team tier walks this same hierarchy with a recursive
// CTE; a loop makes that query non-terminating, so every scope-tiered
// permission in the product would hang. Refusing here is the cheap place.
func (s *serviceImpl) CreateRelationship(ctx context.Context, orgID string, caller Caller, req CreateRelationshipRequest) (*Relationship, error) {
	if !caller.CanManage {
		return nil, ErrAccessDenied
	}
	employeeID := strings.TrimSpace(req.EmployeeID)
	managerID := strings.TrimSpace(req.ManagerID)
	if employeeID == "" || managerID == "" {
		return nil, ErrEmployeeNotFound
	}
	if employeeID == managerID {
		return nil, ErrSelfManagement
	}

	relType := Solid
	if strings.TrimSpace(req.RelationshipType) != "" {
		relType = RelationshipType(strings.TrimSpace(req.RelationshipType))
		if !relType.IsValid() {
			return nil, ErrInvalidType
		}
	}

	// Both ends must exist in THIS org. The FKs enforce existence but not
	// tenancy, so a cross-tenant employee id would otherwise be insertable.
	for _, id := range []string{employeeID, managerID} {
		ok, err := s.repo.EmployeeExists(ctx, orgID, id)
		if err != nil {
			return nil, err
		}
		if !ok {
			return nil, ErrEmployeeNotFound
		}
	}

	if relType.GrantsDataAccess() {
		// One active solid manager per employee, checked here so the caller
		// gets ErrDuplicateSolid rather than a raw 23505 from
		// uq_hrm_rrel_active_solid.
		existing, err := s.repo.FindActiveSolid(ctx, orgID, employeeID)
		if err != nil {
			return nil, err
		}
		if existing != nil {
			return nil, ErrDuplicateSolid
		}

		// Only SOLID lines are cycle-checked. Matrix lines may legitimately
		// loop — two people can lead each other's projects — and refusing
		// those would reject valid org charts.
		edges, err := s.repo.SolidEdges(ctx, orgID)
		if err != nil {
			return nil, err
		}
		if WouldCreateCycle(edges, employeeID, managerID) {
			return nil, ErrWouldCreateCycle
		}
	}

	effectiveFrom := time.Now()
	if req.EffectiveFrom != nil && strings.TrimSpace(*req.EffectiveFrom) != "" {
		d, err := time.Parse("2006-01-02", strings.TrimSpace(*req.EffectiveFrom))
		if err != nil {
			return nil, fmt.Errorf("orgchart: CreateRelationship: effective_from must be YYYY-MM-DD: %w", err)
		}
		effectiveFrom = d
	}

	rel := &Relationship{
		OrgID: orgID, EmployeeID: employeeID, ManagerID: managerID,
		RelationshipType: relType, EffectiveFrom: effectiveFrom,
		Note: req.Note, CreatedBy: caller.UserID,
	}
	if err := s.repo.CreateRelationship(ctx, rel); err != nil {
		return nil, err
	}
	return rel, nil
}

func (s *serviceImpl) ListRelationships(ctx context.Context, orgID string, caller Caller, employeeID string, activeOnly bool) ([]*Relationship, error) {
	return s.repo.ListRelationships(ctx, orgID, strings.TrimSpace(employeeID), activeOnly)
}

func (s *serviceImpl) EndRelationship(ctx context.Context, orgID string, caller Caller, ref string, req EndRelationshipRequest) (*Relationship, error) {
	if !caller.CanManage {
		return nil, ErrAccessDenied
	}
	rel, err := s.repo.FindRelationshipByRef(ctx, orgID, ref)
	if err != nil {
		return nil, fmt.Errorf("orgchart: EndRelationship: %w", err)
	}
	if rel == nil {
		return nil, ErrRelationshipNotFound
	}
	if !rel.IsActive() {
		return nil, ErrAlreadyEnded
	}

	effectiveTo := time.Now()
	if req.EffectiveTo != nil && strings.TrimSpace(*req.EffectiveTo) != "" {
		d, err := time.Parse("2006-01-02", strings.TrimSpace(*req.EffectiveTo))
		if err != nil {
			return nil, fmt.Errorf("orgchart: EndRelationship: effective_to must be YYYY-MM-DD: %w", err)
		}
		effectiveTo = d
	}
	if err := s.repo.EndRelationship(ctx, orgID, rel.ID, effectiveTo); err != nil {
		return nil, err
	}
	return s.repo.FindRelationshipByRef(ctx, orgID, rel.ID)
}

func (s *serviceImpl) GetChart(ctx context.Context, orgID string, caller Caller) ([]*ChartNode, error) {
	nodes, err := s.repo.ChartNodes(ctx, orgID)
	if err != nil {
		return nil, err
	}
	// Matrix lines are attached separately from ChildIDs so no consumer can
	// mistake a dotted line for one that confers data access.
	matrix, err := s.repo.ListRelationships(ctx, orgID, "", true)
	if err != nil {
		return nil, err
	}
	byEmployee := map[string][]*Relationship{}
	for _, m := range matrix {
		if m.RelationshipType.GrantsDataAccess() {
			continue
		}
		byEmployee[m.EmployeeID] = append(byEmployee[m.EmployeeID], m)
	}
	for _, n := range nodes {
		n.MatrixLines = byEmployee[n.EmployeeID]
	}
	return nodes, nil
}

func (s *serviceImpl) ManagementChain(ctx context.Context, orgID string, caller Caller, employeeID string) ([]string, error) {
	edges, err := s.repo.SolidEdges(ctx, orgID)
	if err != nil {
		return nil, err
	}
	return ChainToTop(edges, strings.TrimSpace(employeeID)), nil
}

// ── Seats ────────────────────────────────────────────────────────────────────

func (s *serviceImpl) CreateSeat(ctx context.Context, orgID string, caller Caller, req CreateSeatRequest) (*Seat, error) {
	if !caller.CanManage {
		return nil, ErrAccessDenied
	}
	positionID := strings.TrimSpace(req.PositionID)
	if positionID == "" {
		return nil, ErrSeatNotFound
	}
	if req.EmployeeID != nil && strings.TrimSpace(*req.EmployeeID) != "" {
		ok, err := s.repo.EmployeeExists(ctx, orgID, *req.EmployeeID)
		if err != nil {
			return nil, err
		}
		if !ok {
			return nil, ErrEmployeeNotFound
		}
	}
	seat := &Seat{
		OrgID: orgID, PositionID: positionID, EmployeeID: req.EmployeeID,
		SeatLabel: req.SeatLabel, CreatedBy: caller.UserID,
	}
	if err := s.repo.CreateSeat(ctx, seat); err != nil {
		return nil, err
	}
	return seat, nil
}

func (s *serviceImpl) ListSeats(ctx context.Context, orgID string, caller Caller, positionID string, vacantOnly bool) ([]*Seat, error) {
	return s.repo.ListSeats(ctx, orgID, strings.TrimSpace(positionID), vacantOnly)
}

// AssignSeat sets or clears a seat's occupant.
//
// A nil employee VACATES the seat rather than deleting it — the seat is
// headcount, and keeping it is what makes the vacancy visible to whatever
// eventually raises a requisition against it.
func (s *serviceImpl) AssignSeat(ctx context.Context, orgID string, caller Caller, ref string, req AssignSeatRequest) (*Seat, error) {
	if !caller.CanManage {
		return nil, ErrAccessDenied
	}
	seat, err := s.repo.FindSeatByRef(ctx, orgID, ref)
	if err != nil {
		return nil, fmt.Errorf("orgchart: AssignSeat: %w", err)
	}
	if seat == nil {
		return nil, ErrSeatNotFound
	}
	if req.EmployeeID != nil && strings.TrimSpace(*req.EmployeeID) != "" {
		ok, err := s.repo.EmployeeExists(ctx, orgID, *req.EmployeeID)
		if err != nil {
			return nil, err
		}
		if !ok {
			return nil, ErrEmployeeNotFound
		}
		seat.EmployeeID = req.EmployeeID
	} else {
		seat.EmployeeID = nil
	}
	if err := s.repo.UpdateSeat(ctx, seat); err != nil {
		return nil, err
	}
	return seat, nil
}
