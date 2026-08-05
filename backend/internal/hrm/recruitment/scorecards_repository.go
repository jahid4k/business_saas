// backend/internal/hrm/recruitment/scorecards_repository.go
package recruitment

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
)

// ScorecardRepository is embedded into Repository — see repository.go.
type ScorecardRepository interface {
	FindScorecards(ctx context.Context, interviewID string) ([]*Scorecard, error)
	FindScorecard(ctx context.Context, interviewID, panelistEmployeeID string) (*Scorecard, error)
	// UpsertScorecardDraft inserts or updates the caller's own scorecard row.
	// The WHERE submitted_at IS NULL clause on the DO UPDATE branch is a
	// second, DB-level guard against the immutable-once-submitted rule (the
	// service checks first; this closes the race between check and write).
	// Returns ErrScorecardAlreadySubmitted if the guard blocks the write.
	UpsertScorecardDraft(ctx context.Context, sc *Scorecard) error
	// SubmitScorecard sets submitted_at — the service has already verified
	// the row exists and is not yet submitted; ErrNoRows here means a
	// concurrent submit won the race.
	SubmitScorecard(ctx context.Context, interviewID, panelistEmployeeID string) (*Scorecard, error)
}

const scorecardCols = `id, public_id, interview_id, panelist_employee_id, overall_rating, technical_score,
	communication_score, culture_fit_score, recommendation, strengths, concerns, submitted_at, created_at, updated_at`

func scanScorecard(row interface{ Scan(...any) error }, sc *Scorecard) error {
	return row.Scan(
		&sc.ID, &sc.PublicID, &sc.InterviewID, &sc.PanelistEmployeeID, &sc.OverallRating, &sc.TechnicalScore,
		&sc.CommunicationScore, &sc.CultureFitScore, &sc.Recommendation, &sc.Strengths, &sc.Concerns,
		&sc.SubmittedAt, &sc.CreatedAt, &sc.UpdatedAt,
	)
}

func (r *repoImpl) FindScorecards(ctx context.Context, interviewID string) ([]*Scorecard, error) {
	q := `SELECT ` + scorecardCols + ` FROM hrm_interview_scorecards WHERE interview_id = $1 ORDER BY created_at ASC`
	rows, err := r.db.Query(ctx, q, interviewID)
	if err != nil {
		return nil, fmt.Errorf("recruitment: FindScorecards: %w", err)
	}
	defer rows.Close()
	list := make([]*Scorecard, 0)
	for rows.Next() {
		sc := &Scorecard{}
		if err := scanScorecard(rows, sc); err != nil {
			return nil, fmt.Errorf("recruitment: FindScorecards: scan: %w", err)
		}
		list = append(list, sc)
	}
	return list, rows.Err()
}

func (r *repoImpl) FindScorecard(ctx context.Context, interviewID, panelistEmployeeID string) (*Scorecard, error) {
	q := `SELECT ` + scorecardCols + ` FROM hrm_interview_scorecards WHERE interview_id = $1 AND panelist_employee_id = $2`
	sc := &Scorecard{}
	err := scanScorecard(r.db.QueryRow(ctx, q, interviewID, panelistEmployeeID), sc)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("recruitment: FindScorecard: %w", err)
	}
	return sc, nil
}

func (r *repoImpl) UpsertScorecardDraft(ctx context.Context, sc *Scorecard) error {
	err := r.db.QueryRow(ctx,
		`INSERT INTO hrm_interview_scorecards
		    (interview_id, panelist_employee_id, overall_rating, technical_score, communication_score, culture_fit_score, recommendation, strengths, concerns)
		 VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9)
		 ON CONFLICT (interview_id, panelist_employee_id) DO UPDATE SET
		    overall_rating = EXCLUDED.overall_rating,
		    technical_score = EXCLUDED.technical_score,
		    communication_score = EXCLUDED.communication_score,
		    culture_fit_score = EXCLUDED.culture_fit_score,
		    recommendation = EXCLUDED.recommendation,
		    strengths = EXCLUDED.strengths,
		    concerns = EXCLUDED.concerns,
		    updated_at = NOW()
		 WHERE hrm_interview_scorecards.submitted_at IS NULL
		 RETURNING id, public_id, submitted_at, created_at, updated_at`,
		sc.InterviewID, sc.PanelistEmployeeID, sc.OverallRating, sc.TechnicalScore, sc.CommunicationScore,
		sc.CultureFitScore, sc.Recommendation, sc.Strengths, sc.Concerns,
	).Scan(&sc.ID, &sc.PublicID, &sc.SubmittedAt, &sc.CreatedAt, &sc.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrScorecardAlreadySubmitted
	}
	if err != nil {
		return fmt.Errorf("recruitment: UpsertScorecardDraft: %w", err)
	}
	return nil
}

func (r *repoImpl) SubmitScorecard(ctx context.Context, interviewID, panelistEmployeeID string) (*Scorecard, error) {
	sc := &Scorecard{}
	err := scanScorecard(r.db.QueryRow(ctx,
		`UPDATE hrm_interview_scorecards SET submitted_at = NOW(), updated_at = NOW()
		 WHERE interview_id = $1 AND panelist_employee_id = $2 AND submitted_at IS NULL
		 RETURNING `+scorecardCols,
		interviewID, panelistEmployeeID,
	), sc)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrScorecardAlreadySubmitted
	}
	if err != nil {
		return nil, fmt.Errorf("recruitment: SubmitScorecard: %w", err)
	}
	return sc, nil
}
