package apikeys

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
)

type Service interface {
	GenerateKey(ctx context.Context, orgID, callerID string, req CreateKeyRequest) (*CreateKeyResponse, error)
	ValidateKey(ctx context.Context, rawKey string) (*OrgAPIKey, error)
	ListKeys(ctx context.Context, orgID string) ([]*OrgAPIKey, error)
	RevokeKey(ctx context.Context, orgID, keyID string) error
	UpdateLastUsed(ctx context.Context, keyID string) error
}

type service struct {
	repo Repository
}

func NewService(repo Repository) Service {
	return &service{repo: repo}
}

func (s *service) GenerateKey(ctx context.Context, orgID, callerID string, req CreateKeyRequest) (*CreateKeyResponse, error) {
	// Generate random 32 bytes for the key
	randomBytes := make([]byte, 32)
	if _, err := rand.Read(randomBytes); err != nil {
		return nil, fmt.Errorf("failed to generate random key: %w", err)
	}

	// Create raw key: e.g. bs_live_ + hex(32 bytes)
	rawKey := "bs_live_" + hex.EncodeToString(randomBytes)

	// Hash the raw key using SHA-256 (allows us to do exact match lookups in DB)
	hash := sha256.Sum256([]byte(rawKey))
	keyHash := hex.EncodeToString(hash[:])

	// Prefix is the first 16 chars of the raw key for UI identification
	keyPrefix := rawKey[:16]

	key := &OrgAPIKey{
		OrgID:          orgID,
		Name:           req.Name,
		KeyPrefix:      keyPrefix,
		KeyHash:        keyHash,
		Scopes:         req.Scopes,
		AllowedOrigins: req.AllowedOrigins,
		IsActive:       true,
		ExpiresAt:      req.ExpiresAt,
		CreatedBy:      callerID,
	}

	if err := s.repo.Create(ctx, key); err != nil {
		return nil, fmt.Errorf("failed to save api key: %w", err)
	}

	return &CreateKeyResponse{
		Key:    key,
		RawKey: rawKey,
	}, nil
}

func (s *service) ValidateKey(ctx context.Context, rawKey string) (*OrgAPIKey, error) {
	hash := sha256.Sum256([]byte(rawKey))
	keyHash := hex.EncodeToString(hash[:])

	key, err := s.repo.FindByHash(ctx, keyHash)
	if err != nil {
		return nil, err
	}

	if !key.IsActive {
		return nil, ErrKeyRevoked
	}

	// TODO: Check if expired, though the DB has expires_at

	// Fire and forget updating last used at. Since we don't want to block, 
	// we could do this async or caller can do it async. 
	// The plan says the middleware does this without blocking.

	return key, nil
}

func (s *service) ListKeys(ctx context.Context, orgID string) ([]*OrgAPIKey, error) {
	return s.repo.FindByOrgID(ctx, orgID)
}

func (s *service) RevokeKey(ctx context.Context, orgID, keyID string) error {
	return s.repo.Revoke(ctx, orgID, keyID)
}

func (s *service) UpdateLastUsed(ctx context.Context, keyID string) error {
	return s.repo.UpdateLastUsed(ctx, keyID)
}
