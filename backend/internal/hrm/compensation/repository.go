// backend/internal/hrm/compensation/repository.go
package compensation

import "github.com/jackc/pgx/v5/pgxpool"

// Repository composes the sub-feature interfaces declared in
// config_repository.go, revisions_repository.go and bonuses_repository.go.
type Repository interface {
	ConfigRepository
	RevisionRepository
	BonusRepository
}

type repoImpl struct{ db *pgxpool.Pool }

func NewRepository(db *pgxpool.Pool) Repository { return &repoImpl{db: db} }
