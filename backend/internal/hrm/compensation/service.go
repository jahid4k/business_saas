// backend/internal/hrm/compensation/service.go
package compensation

import (
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/mridha/businesssaas/internal/hrm/approvals"
)

// Service is composed of the sub-feature interfaces declared in
// config_service.go, revisions_service.go and bonuses_service.go.
type Service interface {
	ConfigService
	RevisionService
	BonusService
}

type serviceImpl struct {
	repo         Repository
	db           *pgxpool.Pool
	approvalsSvc approvals.Service
}

func NewService(repo Repository, db *pgxpool.Pool, approvalsSvc approvals.Service) Service {
	return &serviceImpl{repo: repo, db: db, approvalsSvc: approvalsSvc}
}

// ── Shared helpers ───────────────────────────────────────────────────────────

const dateLayout = "2006-01-02"

// parseDate converts a required ISO 8601 date string. Unlike the performance
// package's parseDate, the compensation date fields (band/cycle/bonus
// effective and period dates) are all required, so a blank string is an
// error, not nil.
func parseDate(v string) (*time.Time, error) {
	v = strings.TrimSpace(v)
	if v == "" {
		return nil, fmt.Errorf("date is required")
	}
	d, err := time.Parse(dateLayout, v)
	if err != nil {
		return nil, fmt.Errorf("invalid date %q: %w", v, err)
	}
	return &d, nil
}

// nilIfBlank normalises an all-whitespace optional string to nil so blank
// input and absent input are stored identically — the performance package's
// helper of the same name.
func nilIfBlank(s *string) *string {
	if s == nil || strings.TrimSpace(*s) == "" {
		return nil
	}
	trimmed := strings.TrimSpace(*s)
	return &trimmed
}
