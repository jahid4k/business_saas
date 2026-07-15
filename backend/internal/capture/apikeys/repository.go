package apikeys

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Repository interface {
	Create(ctx context.Context, key *OrgAPIKey) error
	FindByHash(ctx context.Context, hash string) (*OrgAPIKey, error)
	FindByOrgID(ctx context.Context, orgID string) ([]*OrgAPIKey, error)
	Revoke(ctx context.Context, orgID, keyID string) error
	UpdateLastUsed(ctx context.Context, keyID string) error
}

type pgRepo struct {
	pool *pgxpool.Pool
}

func NewRepository(pool *pgxpool.Pool) Repository {
	return &pgRepo{pool: pool}
}

func (r *pgRepo) Create(ctx context.Context, key *OrgAPIKey) error {
	query := `
		INSERT INTO org_api_keys (
			org_id, name, key_prefix, key_hash, scopes, allowed_origins,
			is_active, expires_at, created_by
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7, $8, $9
		) RETURNING id, created_at
	`
	strScopes := make([]string, len(key.Scopes))
	for i, s := range key.Scopes {
		strScopes[i] = string(s)
	}

	return r.pool.QueryRow(ctx, query,
		key.OrgID, key.Name, key.KeyPrefix, key.KeyHash,
		strScopes, key.AllowedOrigins, key.IsActive,
		key.ExpiresAt, key.CreatedBy,
	).Scan(&key.ID, &key.CreatedAt)
}

func (r *pgRepo) FindByHash(ctx context.Context, hash string) (*OrgAPIKey, error) {
	query := `
		SELECT id, org_id, name, key_prefix, key_hash, scopes, allowed_origins,
		       is_active, last_used_at, expires_at, created_by, created_at
		FROM org_api_keys
		WHERE key_hash = $1
	`
	var k OrgAPIKey
	var strScopes []string
	err := r.pool.QueryRow(ctx, query, hash).Scan(
		&k.ID, &k.OrgID, &k.Name, &k.KeyPrefix, &k.KeyHash,
		&strScopes, &k.AllowedOrigins, &k.IsActive,
		&k.LastUsedAt, &k.ExpiresAt, &k.CreatedBy, &k.CreatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrKeyNotFound
		}
		return nil, err
	}
	for _, s := range strScopes {
		k.Scopes = append(k.Scopes, Scope(s))
	}
	return &k, nil
}

func (r *pgRepo) FindByOrgID(ctx context.Context, orgID string) ([]*OrgAPIKey, error) {
	query := `
		SELECT id, org_id, name, key_prefix, key_hash, scopes, allowed_origins,
		       is_active, last_used_at, expires_at, created_by, created_at
		FROM org_api_keys
		WHERE org_id = $1
		ORDER BY created_at DESC
	`
	rows, err := r.pool.Query(ctx, query, orgID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var keys []*OrgAPIKey
	for rows.Next() {
		var k OrgAPIKey
		var strScopes []string
		err := rows.Scan(
			&k.ID, &k.OrgID, &k.Name, &k.KeyPrefix, &k.KeyHash,
			&strScopes, &k.AllowedOrigins, &k.IsActive,
			&k.LastUsedAt, &k.ExpiresAt, &k.CreatedBy, &k.CreatedAt,
		)
		if err != nil {
			return nil, err
		}
		for _, s := range strScopes {
			k.Scopes = append(k.Scopes, Scope(s))
		}
		keys = append(keys, &k)
	}
	return keys, rows.Err()
}

func (r *pgRepo) Revoke(ctx context.Context, orgID, keyID string) error {
	query := `
		UPDATE org_api_keys
		SET is_active = false
		WHERE id = $1 AND org_id = $2
	`
	cmdTag, err := r.pool.Exec(ctx, query, keyID, orgID)
	if err != nil {
		return err
	}
	if cmdTag.RowsAffected() == 0 {
		return ErrKeyNotFound
	}
	return nil
}

func (r *pgRepo) UpdateLastUsed(ctx context.Context, keyID string) error {
	query := `
		UPDATE org_api_keys
		SET last_used_at = NOW()
		WHERE id = $1
	`
	_, err := r.pool.Exec(ctx, query, keyID)
	return err
}
