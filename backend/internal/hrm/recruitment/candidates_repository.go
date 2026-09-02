// backend/internal/hrm/recruitment/candidates_repository.go
package recruitment

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"
)

// CandidateRepository is embedded into Repository — see repository.go.
type CandidateRepository interface {
	FindCandidates(ctx context.Context, orgID string, filter CandidateListFilter) ([]*Candidate, error)
	CountCandidates(ctx context.Context, orgID string, filter CandidateListFilter) (int, error)
	FindCandidateByRef(ctx context.Context, orgID, ref string) (*Candidate, error)
	FindCandidateByEmail(ctx context.Context, orgID, email string) (*Candidate, error)
	CreateCandidate(ctx context.Context, c *Candidate) error
	UpdateCandidate(ctx context.Context, c *Candidate) error
	SoftDeleteCandidate(ctx context.Context, orgID, candidateID string) error
	SetCandidateResume(ctx context.Context, candidateID string, filePath, fileName, mimeType string, sizeBytes int64, sha256 string) error
	// CountCandidatesByResumeSHA256 supports reference-counted deletion — a
	// future purge job must not unlink a file another candidate still
	// points at. Not called by anything in Phase 4A; exists so that job
	// needs no new query when it lands.
	CountCandidatesByResumeSHA256(ctx context.Context, sha256 string) (int, error)
}

const candidateCols = `id, public_id, org_id, first_name, last_name, email, phone, headline, location,
	linkedin_url, portfolio_url, source, referred_by_employee_id,
	resume_file_path, resume_file_name, resume_mime_type, resume_size_bytes, resume_sha256,
	notes, purge_after, created_by, created_at, updated_at`

func scanCandidate(row interface{ Scan(...any) error }, c *Candidate) error {
	return row.Scan(
		&c.ID, &c.PublicID, &c.OrgID, &c.FirstName, &c.LastName, &c.Email, &c.Phone, &c.Headline, &c.Location,
		&c.LinkedInURL, &c.PortfolioURL, &c.Source, &c.ReferredByEmployeeID,
		&c.ResumeFilePath, &c.ResumeFileName, &c.ResumeMimeType, &c.ResumeSizeBytes, &c.ResumeSHA256,
		&c.Notes, &c.PurgeAfter, &c.CreatedBy, &c.CreatedAt, &c.UpdatedAt,
	)
}

func (r *repoImpl) FindCandidates(ctx context.Context, orgID string, filter CandidateListFilter) ([]*Candidate, error) {
	clauses := []string{"org_id = $1", "deleted_at IS NULL"}
	args := []any{orgID}
	if filter.Search != "" {
		args = append(args, "%"+filter.Search+"%")
		clauses = append(clauses, fmt.Sprintf("(first_name ILIKE $%d OR last_name ILIKE $%d OR email ILIKE $%d)", len(args), len(args), len(args)))
	}
	where := strings.Join(clauses, " AND ")
	args = append(args, filter.Limit, filter.Offset)
	q := fmt.Sprintf(`SELECT %s FROM hrm_candidates WHERE %s ORDER BY created_at DESC LIMIT $%d OFFSET $%d`,
		candidateCols, where, len(args)-1, len(args))
	rows, err := r.db.Query(ctx, q, args...)
	if err != nil {
		return nil, fmt.Errorf("recruitment: FindCandidates: %w", err)
	}
	defer rows.Close()
	list := make([]*Candidate, 0)
	for rows.Next() {
		c := &Candidate{}
		if err := scanCandidate(rows, c); err != nil {
			return nil, fmt.Errorf("recruitment: FindCandidates: scan: %w", err)
		}
		list = append(list, c)
	}
	return list, rows.Err()
}

func (r *repoImpl) CountCandidates(ctx context.Context, orgID string, filter CandidateListFilter) (int, error) {
	clauses := []string{"org_id = $1", "deleted_at IS NULL"}
	args := []any{orgID}
	if filter.Search != "" {
		args = append(args, "%"+filter.Search+"%")
		clauses = append(clauses, fmt.Sprintf("(first_name ILIKE $%d OR last_name ILIKE $%d OR email ILIKE $%d)", len(args), len(args), len(args)))
	}
	var count int
	q := fmt.Sprintf(`SELECT COUNT(*) FROM hrm_candidates WHERE %s`, strings.Join(clauses, " AND "))
	if err := r.db.QueryRow(ctx, q, args...).Scan(&count); err != nil {
		return 0, fmt.Errorf("recruitment: CountCandidates: %w", err)
	}
	return count, nil
}

func (r *repoImpl) FindCandidateByRef(ctx context.Context, orgID, ref string) (*Candidate, error) {
	q := `SELECT ` + candidateCols + ` FROM hrm_candidates WHERE org_id = $1 AND (id::text = $2 OR public_id = $2) AND deleted_at IS NULL`
	c := &Candidate{}
	err := scanCandidate(r.db.QueryRow(ctx, q, orgID, ref), c)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("recruitment: FindCandidateByRef: %w", err)
	}
	return c, nil
}

func (r *repoImpl) FindCandidateByEmail(ctx context.Context, orgID, email string) (*Candidate, error) {
	q := `SELECT ` + candidateCols + ` FROM hrm_candidates WHERE org_id = $1 AND LOWER(email) = LOWER($2) AND deleted_at IS NULL`
	c := &Candidate{}
	err := scanCandidate(r.db.QueryRow(ctx, q, orgID, email), c)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("recruitment: FindCandidateByEmail: %w", err)
	}
	return c, nil
}

func (r *repoImpl) CreateCandidate(ctx context.Context, c *Candidate) error {
	return r.db.QueryRow(ctx,
		`INSERT INTO hrm_candidates
		    (org_id, first_name, last_name, email, phone, headline, location, linkedin_url, portfolio_url,
		     source, referred_by_employee_id, notes, created_by)
		 VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13)
		 RETURNING id, public_id, created_at, updated_at`,
		c.OrgID, c.FirstName, c.LastName, c.Email, c.Phone, c.Headline, c.Location, c.LinkedInURL, c.PortfolioURL,
		c.Source, c.ReferredByEmployeeID, c.Notes, c.CreatedBy,
	).Scan(&c.ID, &c.PublicID, &c.CreatedAt, &c.UpdatedAt)
}

func (r *repoImpl) UpdateCandidate(ctx context.Context, c *Candidate) error {
	err := r.db.QueryRow(ctx,
		`UPDATE hrm_candidates SET
		    first_name=$1, last_name=$2, email=$3, phone=$4, headline=$5, location=$6,
		    linkedin_url=$7, portfolio_url=$8, notes=$9, updated_at=NOW()
		 WHERE id=$10 AND org_id=$11 AND deleted_at IS NULL RETURNING updated_at`,
		c.FirstName, c.LastName, c.Email, c.Phone, c.Headline, c.Location,
		c.LinkedInURL, c.PortfolioURL, c.Notes, c.ID, c.OrgID,
	).Scan(&c.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrCandidateNotFound
	}
	return err
}

func (r *repoImpl) SoftDeleteCandidate(ctx context.Context, orgID, candidateID string) error {
	// Deliberately does NOT touch the resume file — files are content-
	// addressed and may be shared with another candidate. See migration
	// 00078's header and CountCandidatesByResumeSHA256 below.
	cmd, err := r.db.Exec(ctx,
		`UPDATE hrm_candidates SET deleted_at = NOW() WHERE org_id=$1 AND id=$2 AND deleted_at IS NULL`,
		orgID, candidateID,
	)
	if err != nil {
		return fmt.Errorf("recruitment: SoftDeleteCandidate: %w", err)
	}
	if cmd.RowsAffected() == 0 {
		return ErrCandidateNotFound
	}
	return nil
}

func (r *repoImpl) SetCandidateResume(ctx context.Context, candidateID string, filePath, fileName, mimeType string, sizeBytes int64, sha256 string) error {
	_, err := r.db.Exec(ctx,
		`UPDATE hrm_candidates SET
		    resume_file_path=$1, resume_file_name=$2, resume_mime_type=$3, resume_size_bytes=$4, resume_sha256=$5,
		    updated_at=NOW()
		 WHERE id=$6`,
		filePath, fileName, mimeType, sizeBytes, sha256, candidateID,
	)
	return err
}

func (r *repoImpl) CountCandidatesByResumeSHA256(ctx context.Context, sha256 string) (int, error) {
	var count int
	err := r.db.QueryRow(ctx, `SELECT COUNT(*) FROM hrm_candidates WHERE resume_sha256 = $1`, sha256).Scan(&count)
	return count, err
}
