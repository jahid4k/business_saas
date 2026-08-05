// backend/internal/hrm/recruitment/repository.go
package recruitment

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Repository defines the data access interface for the recruitment module.
// Composed of the sub-feature interfaces defined in pipelines_repository.go,
// candidates_repository.go, and applications_repository.go — the
// internal/hrm/leave precedent for a module too large for the 5-file shape.
type Repository interface {
	PipelineRepository
	CandidateRepository
	ApplicationRepository
	InterviewRepository
	ScorecardRepository
	OfferRepository
	ReferralRepository

	// BeginTx opens a new database transaction for use by callers that need
	// to span writes across sub-feature repositories — the crm/leads
	// BeginTx precedent. HireApplication is the only current caller: it
	// spans an application update, a requisition update, and (via
	// EmployeeCreator.CreateEmployeeTx) an employee insert in another
	// package, all in one commit.
	BeginTx(ctx context.Context) (pgx.Tx, error)

	// Requisitions
	FindRequisitions(ctx context.Context, orgID string, filter RequisitionListFilter) ([]*Requisition, error)
	CountRequisitions(ctx context.Context, orgID string, filter RequisitionListFilter) (int, error)
	FindRequisitionByRef(ctx context.Context, orgID, ref string) (*Requisition, error)
	CreateRequisition(ctx context.Context, r *Requisition) error
	UpdateRequisition(ctx context.Context, r *Requisition) error
	SetRequisitionApprovalInstance(ctx context.Context, id, instanceID string, status RequisitionStatus) error
	UpdateRequisitionStatus(ctx context.Context, id string, status RequisitionStatus) error
	CloseRequisition(ctx context.Context, id string, reason string) error
	// IncrementRequisitionFilledCountTx is called by HireApplication inside
	// its own transaction — see BeginTx's doc comment.
	IncrementRequisitionFilledCountTx(ctx context.Context, tx pgx.Tx, requisitionID string) error

	// Postings
	FindPostings(ctx context.Context, orgID string, filter PostingListFilter) ([]*Posting, error)
	CountPostings(ctx context.Context, orgID string, filter PostingListFilter) (int, error)
	FindPostingByRef(ctx context.Context, orgID, ref string) (*Posting, error)
	CreatePosting(ctx context.Context, p *Posting) error
	UpdatePosting(ctx context.Context, p *Posting) error
	DeletePosting(ctx context.Context, orgID, postingID string) error
	SetPostingStatus(ctx context.Context, id string, status PostingStatus, publishedAt, closedAt *string) error
	SlugExists(ctx context.Context, orgID, slug, excludeID string) (bool, error)
}

type repoImpl struct{ db *pgxpool.Pool }

func NewRepository(db *pgxpool.Pool) Repository { return &repoImpl{db: db} }

func (r *repoImpl) BeginTx(ctx context.Context) (pgx.Tx, error) {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("recruitment: BeginTx: %w", err)
	}
	return tx, nil
}

// ============================================================
// Requisitions
// ============================================================

const requisitionCols = `id, public_id, org_id, title, department_id, position_id, hiring_manager_id,
	employment_type, openings, filled_count, location, salary_min, salary_max, salary_currency,
	justification, target_start_date, status, approval_instance_id, closed_at, close_reason,
	created_by, created_at, updated_at`

func scanRequisition(row interface{ Scan(...any) error }, r *Requisition) error {
	return row.Scan(
		&r.ID, &r.PublicID, &r.OrgID, &r.Title, &r.DepartmentID, &r.PositionID, &r.HiringManagerID,
		&r.EmploymentType, &r.Openings, &r.FilledCount, &r.Location, &r.SalaryMin, &r.SalaryMax, &r.SalaryCurrency,
		&r.Justification, &r.TargetStartDate, &r.Status, &r.ApprovalInstanceID, &r.ClosedAt, &r.CloseReason,
		&r.CreatedBy, &r.CreatedAt, &r.UpdatedAt,
	)
}

func buildRequisitionsWhere(orgID string, filter RequisitionListFilter) (string, []any) {
	clauses := []string{"org_id = $1"}
	args := []any{orgID}
	if filter.Status != "" {
		args = append(args, filter.Status)
		clauses = append(clauses, fmt.Sprintf("status = $%d", len(args)))
	}
	return strings.Join(clauses, " AND "), args
}

func (r *repoImpl) FindRequisitions(ctx context.Context, orgID string, filter RequisitionListFilter) ([]*Requisition, error) {
	where, args := buildRequisitionsWhere(orgID, filter)
	args = append(args, filter.Limit, filter.Offset)
	q := fmt.Sprintf(`SELECT %s FROM hrm_job_requisitions WHERE %s ORDER BY created_at DESC LIMIT $%d OFFSET $%d`,
		requisitionCols, where, len(args)-1, len(args))
	rows, err := r.db.Query(ctx, q, args...)
	if err != nil {
		return nil, fmt.Errorf("recruitment: FindRequisitions: %w", err)
	}
	defer rows.Close()
	list := make([]*Requisition, 0)
	for rows.Next() {
		req := &Requisition{}
		if err := scanRequisition(rows, req); err != nil {
			return nil, fmt.Errorf("recruitment: FindRequisitions: scan: %w", err)
		}
		list = append(list, req)
	}
	return list, rows.Err()
}

func (r *repoImpl) CountRequisitions(ctx context.Context, orgID string, filter RequisitionListFilter) (int, error) {
	where, args := buildRequisitionsWhere(orgID, filter)
	var count int
	if err := r.db.QueryRow(ctx, fmt.Sprintf(`SELECT COUNT(*) FROM hrm_job_requisitions WHERE %s`, where), args...).Scan(&count); err != nil {
		return 0, fmt.Errorf("recruitment: CountRequisitions: %w", err)
	}
	return count, nil
}

func (r *repoImpl) FindRequisitionByRef(ctx context.Context, orgID, ref string) (*Requisition, error) {
	q := `SELECT ` + requisitionCols + ` FROM hrm_job_requisitions WHERE org_id = $1 AND (id::text = $2 OR public_id = $2)`
	req := &Requisition{}
	err := scanRequisition(r.db.QueryRow(ctx, q, orgID, ref), req)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("recruitment: FindRequisitionByRef: %w", err)
	}
	return req, nil
}

func (r *repoImpl) CreateRequisition(ctx context.Context, req *Requisition) error {
	return r.db.QueryRow(ctx,
		`INSERT INTO hrm_job_requisitions
		    (org_id, title, department_id, position_id, hiring_manager_id, employment_type, openings,
		     location, salary_min, salary_max, salary_currency, justification, target_start_date,
		     status, created_by)
		 VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15)
		 RETURNING id, public_id, created_at, updated_at`,
		req.OrgID, req.Title, req.DepartmentID, req.PositionID, req.HiringManagerID, req.EmploymentType, req.Openings,
		req.Location, req.SalaryMin, req.SalaryMax, req.SalaryCurrency, req.Justification, req.TargetStartDate,
		req.Status, req.CreatedBy,
	).Scan(&req.ID, &req.PublicID, &req.CreatedAt, &req.UpdatedAt)
}

func (r *repoImpl) UpdateRequisition(ctx context.Context, req *Requisition) error {
	err := r.db.QueryRow(ctx,
		`UPDATE hrm_job_requisitions SET
		    title=$1, department_id=$2, position_id=$3, hiring_manager_id=$4, employment_type=$5,
		    openings=$6, location=$7, salary_min=$8, salary_max=$9, salary_currency=$10,
		    justification=$11, target_start_date=$12, updated_at=NOW()
		 WHERE id=$13 AND org_id=$14 RETURNING updated_at`,
		req.Title, req.DepartmentID, req.PositionID, req.HiringManagerID, req.EmploymentType,
		req.Openings, req.Location, req.SalaryMin, req.SalaryMax, req.SalaryCurrency,
		req.Justification, req.TargetStartDate, req.ID, req.OrgID,
	).Scan(&req.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrRequisitionNotFound
	}
	return err
}

func (r *repoImpl) SetRequisitionApprovalInstance(ctx context.Context, id, instanceID string, status RequisitionStatus) error {
	_, err := r.db.Exec(ctx,
		`UPDATE hrm_job_requisitions SET approval_instance_id=$1, status=$2, updated_at=NOW() WHERE id=$3`,
		instanceID, status, id,
	)
	return err
}

func (r *repoImpl) UpdateRequisitionStatus(ctx context.Context, id string, status RequisitionStatus) error {
	_, err := r.db.Exec(ctx, `UPDATE hrm_job_requisitions SET status=$1, updated_at=NOW() WHERE id=$2`, status, id)
	return err
}

func (r *repoImpl) CloseRequisition(ctx context.Context, id string, reason string) error {
	_, err := r.db.Exec(ctx,
		`UPDATE hrm_job_requisitions SET status='closed', closed_at=NOW(), close_reason=$1, updated_at=NOW() WHERE id=$2`,
		reason, id,
	)
	return err
}

func (r *repoImpl) IncrementRequisitionFilledCountTx(ctx context.Context, tx pgx.Tx, requisitionID string) error {
	cmd, err := tx.Exec(ctx, `UPDATE hrm_job_requisitions SET filled_count = filled_count + 1, updated_at = NOW() WHERE id = $1`, requisitionID)
	if err != nil {
		return fmt.Errorf("recruitment: IncrementRequisitionFilledCountTx: %w", err)
	}
	if cmd.RowsAffected() == 0 {
		return ErrRequisitionNotFound
	}
	return nil
}

// ============================================================
// Postings
// ============================================================

const postingCols = `id, public_id, org_id, requisition_id, pipeline_id, title, description_markdown,
	public_slug, location, is_remote, employment_type, status, published_at, closed_at,
	created_by, created_at, updated_at`

func scanPosting(row interface{ Scan(...any) error }, p *Posting) error {
	return row.Scan(
		&p.ID, &p.PublicID, &p.OrgID, &p.RequisitionID, &p.PipelineID, &p.Title, &p.DescriptionMarkdown,
		&p.PublicSlug, &p.Location, &p.IsRemote, &p.EmploymentType, &p.Status, &p.PublishedAt, &p.ClosedAt,
		&p.CreatedBy, &p.CreatedAt, &p.UpdatedAt,
	)
}

func buildPostingsWhere(orgID string, filter PostingListFilter) (string, []any) {
	clauses := []string{"org_id = $1"}
	args := []any{orgID}
	if filter.Status != "" {
		args = append(args, filter.Status)
		clauses = append(clauses, fmt.Sprintf("status = $%d", len(args)))
	}
	if filter.RequisitionID != "" {
		args = append(args, filter.RequisitionID)
		clauses = append(clauses, fmt.Sprintf("requisition_id = $%d", len(args)))
	}
	return strings.Join(clauses, " AND "), args
}

func (r *repoImpl) FindPostings(ctx context.Context, orgID string, filter PostingListFilter) ([]*Posting, error) {
	where, args := buildPostingsWhere(orgID, filter)
	args = append(args, filter.Limit, filter.Offset)
	q := fmt.Sprintf(`SELECT %s FROM hrm_job_postings WHERE %s ORDER BY created_at DESC LIMIT $%d OFFSET $%d`,
		postingCols, where, len(args)-1, len(args))
	rows, err := r.db.Query(ctx, q, args...)
	if err != nil {
		return nil, fmt.Errorf("recruitment: FindPostings: %w", err)
	}
	defer rows.Close()
	list := make([]*Posting, 0)
	for rows.Next() {
		p := &Posting{}
		if err := scanPosting(rows, p); err != nil {
			return nil, fmt.Errorf("recruitment: FindPostings: scan: %w", err)
		}
		list = append(list, p)
	}
	return list, rows.Err()
}

func (r *repoImpl) CountPostings(ctx context.Context, orgID string, filter PostingListFilter) (int, error) {
	where, args := buildPostingsWhere(orgID, filter)
	var count int
	if err := r.db.QueryRow(ctx, fmt.Sprintf(`SELECT COUNT(*) FROM hrm_job_postings WHERE %s`, where), args...).Scan(&count); err != nil {
		return 0, fmt.Errorf("recruitment: CountPostings: %w", err)
	}
	return count, nil
}

func (r *repoImpl) FindPostingByRef(ctx context.Context, orgID, ref string) (*Posting, error) {
	q := `SELECT ` + postingCols + ` FROM hrm_job_postings WHERE org_id = $1 AND (id::text = $2 OR public_id = $2)`
	p := &Posting{}
	err := scanPosting(r.db.QueryRow(ctx, q, orgID, ref), p)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("recruitment: FindPostingByRef: %w", err)
	}
	return p, nil
}

func (r *repoImpl) CreatePosting(ctx context.Context, p *Posting) error {
	return r.db.QueryRow(ctx,
		`INSERT INTO hrm_job_postings
		    (org_id, requisition_id, pipeline_id, title, description_markdown, public_slug,
		     location, is_remote, employment_type, status, created_by)
		 VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11)
		 RETURNING id, public_id, created_at, updated_at`,
		p.OrgID, p.RequisitionID, p.PipelineID, p.Title, p.DescriptionMarkdown, p.PublicSlug,
		p.Location, p.IsRemote, p.EmploymentType, p.Status, p.CreatedBy,
	).Scan(&p.ID, &p.PublicID, &p.CreatedAt, &p.UpdatedAt)
}

func (r *repoImpl) UpdatePosting(ctx context.Context, p *Posting) error {
	err := r.db.QueryRow(ctx,
		`UPDATE hrm_job_postings SET
		    title=$1, description_markdown=$2, public_slug=$3, location=$4, is_remote=$5,
		    employment_type=$6, updated_at=NOW()
		 WHERE id=$7 AND org_id=$8 RETURNING updated_at`,
		p.Title, p.DescriptionMarkdown, p.PublicSlug, p.Location, p.IsRemote, p.EmploymentType, p.ID, p.OrgID,
	).Scan(&p.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrPostingNotFound
	}
	return err
}

func (r *repoImpl) DeletePosting(ctx context.Context, orgID, postingID string) error {
	cmd, err := r.db.Exec(ctx, `DELETE FROM hrm_job_postings WHERE org_id=$1 AND id=$2`, orgID, postingID)
	if err != nil {
		return fmt.Errorf("recruitment: DeletePosting: %w", err)
	}
	if cmd.RowsAffected() == 0 {
		return ErrPostingNotFound
	}
	return nil
}

func (r *repoImpl) SetPostingStatus(ctx context.Context, id string, status PostingStatus, publishedAt, closedAt *string) error {
	_, err := r.db.Exec(ctx,
		`UPDATE hrm_job_postings SET status=$1,
		    published_at = COALESCE($2::timestamptz, published_at),
		    closed_at = COALESCE($3::timestamptz, closed_at),
		    updated_at=NOW()
		 WHERE id=$4`,
		status, publishedAt, closedAt, id,
	)
	return err
}

// SlugExists checks per-org slug uniqueness. excludeID may be "" (create
// path, nothing to exclude) — id::text <> $3 is used rather than id <> $3
// specifically so an empty string never fails a UUID cast.
func (r *repoImpl) SlugExists(ctx context.Context, orgID, slug, excludeID string) (bool, error) {
	var exists bool
	err := r.db.QueryRow(ctx,
		`SELECT EXISTS(SELECT 1 FROM hrm_job_postings WHERE org_id=$1 AND LOWER(public_slug)=LOWER($2) AND id::text <> $3)`,
		orgID, slug, excludeID,
	).Scan(&exists)
	return exists, err
}
