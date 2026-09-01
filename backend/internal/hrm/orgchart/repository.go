// backend/internal/hrm/orgchart/repository.go
package orgchart

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Repository is the data access interface for the org chart.
//
// TENANT ISOLATION: every method takes orgID and every query filters on it.
//
// ⚠ CreateRelationship and EndRelationship own a TRANSACTION that also writes
// hrm_employees.manager_id. That write is not incidental bookkeeping — it is
// what scope.Predicate's view_team tier resolves through, so a relationship
// committed without it would leave the product's authorization following a
// stale reporting line. The two must move together or not at all.
type Repository interface {
	// SolidEdges returns employee -> current solid-line manager for the whole
	// org, which is what cycle detection walks. Only solid lines: matrix
	// relationships may legitimately loop.
	SolidEdges(ctx context.Context, orgID string) (map[string]string, error)

	CreateRelationship(ctx context.Context, r *Relationship) error
	FindRelationshipByRef(ctx context.Context, orgID, ref string) (*Relationship, error)
	FindActiveSolid(ctx context.Context, orgID, employeeID string) (*Relationship, error)
	ListRelationships(ctx context.Context, orgID, employeeID string, activeOnly bool) ([]*Relationship, error)
	// EndRelationship stamps effective_to and, for a solid line, clears
	// hrm_employees.manager_id in the same transaction.
	EndRelationship(ctx context.Context, orgID, id string, effectiveTo time.Time) error

	EmployeeExists(ctx context.Context, orgID, employeeID string) (bool, error)
	// ChartNodes returns every employee with their denormalized manager, for
	// rendering. Reads manager_id, deliberately: the chart must show what
	// authorization actually follows, so a drift between table and column is
	// VISIBLE rather than hidden behind a prettier query.
	ChartNodes(ctx context.Context, orgID string) ([]*ChartNode, error)

	// Seats
	CreateSeat(ctx context.Context, s *Seat) error
	FindSeatByRef(ctx context.Context, orgID, ref string) (*Seat, error)
	ListSeats(ctx context.Context, orgID, positionID string, vacantOnly bool) ([]*Seat, error)
	UpdateSeat(ctx context.Context, s *Seat) error
}

type repoImpl struct{ db *pgxpool.Pool }

func NewRepository(db *pgxpool.Pool) Repository { return &repoImpl{db: db} }

const relSel = `id, public_id, org_id, employee_id, manager_id, relationship_type,
	effective_from, effective_to, note, created_by, created_at, updated_at`

func scanRelationship(row pgx.Row) (*Relationship, error) {
	r := &Relationship{}
	err := row.Scan(&r.ID, &r.PublicID, &r.OrgID, &r.EmployeeID, &r.ManagerID, &r.RelationshipType,
		&r.EffectiveFrom, &r.EffectiveTo, &r.Note, &r.CreatedBy, &r.CreatedAt, &r.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return r, nil
}

func (r *repoImpl) SolidEdges(ctx context.Context, orgID string) (map[string]string, error) {
	rows, err := r.db.Query(ctx,
		`SELECT employee_id::text, manager_id::text
		   FROM hrm_reporting_relationships
		  WHERE org_id=$1 AND relationship_type='solid' AND effective_to IS NULL`, orgID)
	if err != nil {
		return nil, fmt.Errorf("orgchart: SolidEdges: %w", err)
	}
	defer rows.Close()
	edges := map[string]string{}
	for rows.Next() {
		var employeeID, managerID string
		if err := rows.Scan(&employeeID, &managerID); err != nil {
			return nil, err
		}
		edges[employeeID] = managerID
	}
	return edges, rows.Err()
}

// CreateRelationship inserts the line and, for a solid one, writes
// hrm_employees.manager_id in the SAME transaction.
//
// ⚠ The manager_id write is the load-bearing half. scope.Predicate's
// view_team CTE joins on that column, so committing the relationship without
// it would leave every scope-tiered permission in the product following a
// reporting line that no longer exists.
func (r *repoImpl) CreateRelationship(ctx context.Context, rel *Relationship) error {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("orgchart: CreateRelationship: begin: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	if err := tx.QueryRow(ctx,
		`INSERT INTO hrm_reporting_relationships
		   (org_id, employee_id, manager_id, relationship_type, effective_from, note, created_by)
		 VALUES ($1,$2::uuid,$3::uuid,$4,$5,$6,$7)
		 RETURNING id, public_id, created_at, updated_at`,
		rel.OrgID, rel.EmployeeID, rel.ManagerID, rel.RelationshipType,
		rel.EffectiveFrom, rel.Note, rel.CreatedBy,
	).Scan(&rel.ID, &rel.PublicID, &rel.CreatedAt, &rel.UpdatedAt); err != nil {
		return fmt.Errorf("orgchart: CreateRelationship: %w", err)
	}

	if rel.RelationshipType.GrantsDataAccess() {
		if _, err := tx.Exec(ctx,
			`UPDATE hrm_employees SET manager_id=$3::uuid, updated_at=NOW()
			  WHERE org_id=$1 AND id=$2::uuid`,
			rel.OrgID, rel.EmployeeID, rel.ManagerID); err != nil {
			return fmt.Errorf("orgchart: CreateRelationship: sync manager_id: %w", err)
		}
	}
	return tx.Commit(ctx)
}

func (r *repoImpl) EndRelationship(ctx context.Context, orgID, id string, effectiveTo time.Time) error {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("orgchart: EndRelationship: begin: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var employeeID string
	var relType RelationshipType
	err = tx.QueryRow(ctx,
		`UPDATE hrm_reporting_relationships SET effective_to=$3, updated_at=NOW()
		  WHERE org_id=$1 AND id=$2::uuid AND effective_to IS NULL
		 RETURNING employee_id::text, relationship_type`,
		orgID, id, effectiveTo).Scan(&employeeID, &relType)
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrAlreadyEnded
	}
	if err != nil {
		return fmt.Errorf("orgchart: EndRelationship: %w", err)
	}

	// Ending a solid line clears manager_id. Leaving the column pointing at a
	// manager the table says is no longer theirs is exactly the drift this
	// design exists to prevent — and it would keep granting view_team access.
	if relType.GrantsDataAccess() {
		if _, err := tx.Exec(ctx,
			`UPDATE hrm_employees SET manager_id=NULL, updated_at=NOW()
			  WHERE org_id=$1 AND id=$2::uuid`, orgID, employeeID); err != nil {
			return fmt.Errorf("orgchart: EndRelationship: clear manager_id: %w", err)
		}
	}
	return tx.Commit(ctx)
}

func (r *repoImpl) FindRelationshipByRef(ctx context.Context, orgID, ref string) (*Relationship, error) {
	return scanRelationship(r.db.QueryRow(ctx,
		`SELECT `+relSel+` FROM hrm_reporting_relationships
		  WHERE org_id=$1 AND (id::text=$2 OR public_id=$2)`, orgID, ref))
}

func (r *repoImpl) FindActiveSolid(ctx context.Context, orgID, employeeID string) (*Relationship, error) {
	return scanRelationship(r.db.QueryRow(ctx,
		`SELECT `+relSel+` FROM hrm_reporting_relationships
		  WHERE org_id=$1 AND employee_id=$2::uuid
		    AND relationship_type='solid' AND effective_to IS NULL`, orgID, employeeID))
}

func (r *repoImpl) ListRelationships(ctx context.Context, orgID, employeeID string, activeOnly bool) ([]*Relationship, error) {
	q := `SELECT ` + relSel + ` FROM hrm_reporting_relationships WHERE org_id=$1`
	args := []any{orgID}
	if employeeID != "" {
		args = append(args, employeeID)
		q += fmt.Sprintf(` AND (employee_id=$%d::uuid OR manager_id=$%d::uuid)`, len(args), len(args))
	}
	if activeOnly {
		q += ` AND effective_to IS NULL`
	}
	q += ` ORDER BY effective_from DESC, created_at DESC`
	rows, err := r.db.Query(ctx, q, args...)
	if err != nil {
		return nil, fmt.Errorf("orgchart: ListRelationships: %w", err)
	}
	defer rows.Close()
	list := make([]*Relationship, 0)
	for rows.Next() {
		rel, err := scanRelationship(rows)
		if err != nil {
			return nil, err
		}
		list = append(list, rel)
	}
	return list, rows.Err()
}

func (r *repoImpl) EmployeeExists(ctx context.Context, orgID, employeeID string) (bool, error) {
	var n int
	if err := r.db.QueryRow(ctx,
		`SELECT COUNT(*) FROM hrm_employees WHERE org_id=$1 AND id=$2::uuid`,
		orgID, employeeID).Scan(&n); err != nil {
		return false, fmt.Errorf("orgchart: EmployeeExists: %w", err)
	}
	return n > 0, nil
}

// ChartNodes reads hrm_employees.manager_id rather than the relationships
// table, deliberately.
//
// The chart must show what AUTHORIZATION actually follows. Rendering from the
// relationships table would draw a correct-looking chart while view_team
// resolved through a stale column, hiding exactly the drift that matters.
func (r *repoImpl) ChartNodes(ctx context.Context, orgID string) ([]*ChartNode, error) {
	rows, err := r.db.Query(ctx,
		`SELECT e.id::text,
		        TRIM(COALESCE(e.first_name,'') || ' ' || COALESCE(e.last_name,'')),
		        e.manager_id::text, p.title
		   FROM hrm_employees e
		   LEFT JOIN hrm_positions p ON p.id = e.position_id
		  WHERE e.org_id=$1
		  ORDER BY e.first_name, e.last_name`, orgID)
	if err != nil {
		return nil, fmt.Errorf("orgchart: ChartNodes: %w", err)
	}
	defer rows.Close()

	nodes := make([]*ChartNode, 0)
	byID := map[string]*ChartNode{}
	for rows.Next() {
		n := &ChartNode{ChildIDs: []string{}}
		if err := rows.Scan(&n.EmployeeID, &n.DisplayName, &n.ManagerID, &n.PositionName); err != nil {
			return nil, err
		}
		nodes = append(nodes, n)
		byID[n.EmployeeID] = n
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	for _, n := range nodes {
		if n.ManagerID == nil {
			continue
		}
		if parent, ok := byID[*n.ManagerID]; ok {
			parent.ChildIDs = append(parent.ChildIDs, n.EmployeeID)
		}
	}
	return nodes, nil
}

// ── Seats ────────────────────────────────────────────────────────────────────

const seatSel = `id, public_id, org_id, position_id, employee_id, seat_label,
	is_active, created_by, created_at, updated_at`

func scanSeat(row pgx.Row) (*Seat, error) {
	s := &Seat{}
	err := row.Scan(&s.ID, &s.PublicID, &s.OrgID, &s.PositionID, &s.EmployeeID,
		&s.SeatLabel, &s.IsActive, &s.CreatedBy, &s.CreatedAt, &s.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return s, nil
}

func (r *repoImpl) CreateSeat(ctx context.Context, s *Seat) error {
	err := r.db.QueryRow(ctx,
		`INSERT INTO hrm_position_seats (org_id, position_id, employee_id, seat_label, created_by)
		 VALUES ($1,$2::uuid,$3::uuid,$4,$5)
		 RETURNING id, public_id, is_active, created_at, updated_at`,
		s.OrgID, s.PositionID, s.EmployeeID, s.SeatLabel, s.CreatedBy,
	).Scan(&s.ID, &s.PublicID, &s.IsActive, &s.CreatedAt, &s.UpdatedAt)
	if err != nil {
		return fmt.Errorf("orgchart: CreateSeat: %w", err)
	}
	return nil
}

func (r *repoImpl) FindSeatByRef(ctx context.Context, orgID, ref string) (*Seat, error) {
	return scanSeat(r.db.QueryRow(ctx,
		`SELECT `+seatSel+` FROM hrm_position_seats
		  WHERE org_id=$1 AND (id::text=$2 OR public_id=$2)`, orgID, ref))
}

func (r *repoImpl) ListSeats(ctx context.Context, orgID, positionID string, vacantOnly bool) ([]*Seat, error) {
	q := `SELECT ` + seatSel + ` FROM hrm_position_seats WHERE org_id=$1`
	args := []any{orgID}
	if positionID != "" {
		args = append(args, positionID)
		q += fmt.Sprintf(` AND position_id=$%d::uuid`, len(args))
	}
	if vacantOnly {
		q += ` AND employee_id IS NULL AND is_active = TRUE`
	}
	q += ` ORDER BY created_at`
	rows, err := r.db.Query(ctx, q, args...)
	if err != nil {
		return nil, fmt.Errorf("orgchart: ListSeats: %w", err)
	}
	defer rows.Close()
	list := make([]*Seat, 0)
	for rows.Next() {
		s, err := scanSeat(rows)
		if err != nil {
			return nil, err
		}
		list = append(list, s)
	}
	return list, rows.Err()
}

func (r *repoImpl) UpdateSeat(ctx context.Context, s *Seat) error {
	ct, err := r.db.Exec(ctx,
		`UPDATE hrm_position_seats SET employee_id=$3::uuid, seat_label=$4, is_active=$5, updated_at=NOW()
		  WHERE org_id=$1 AND id=$2::uuid`,
		s.OrgID, s.ID, s.EmployeeID, s.SeatLabel, s.IsActive)
	if err != nil {
		return fmt.Errorf("orgchart: UpdateSeat: %w", err)
	}
	if ct.RowsAffected() == 0 {
		return ErrSeatNotFound
	}
	return nil
}
