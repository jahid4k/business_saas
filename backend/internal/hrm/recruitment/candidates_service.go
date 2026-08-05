// backend/internal/hrm/recruitment/candidates_service.go
package recruitment

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

// Resumes are stored OUTSIDE ./uploads, which cmd/server/main.go serves
// fully unauthenticated via static.New("./uploads"). A resume is candidate
// PII; access goes through GetResumeFile + the handler's authenticated
// download route only.
const (
	resumeStorageDir     = "storage/resumes"
	resumeMaxUploadBytes = 10 * 1024 * 1024 // 10MB
)

// CandidateService is embedded into Service — see service.go.
type CandidateService interface {
	ListCandidates(ctx context.Context, orgID string, filter CandidateListFilter) (*CandidateListResponse, error)
	GetCandidate(ctx context.Context, orgID, ref string) (*Candidate, error)
	CreateCandidate(ctx context.Context, orgID string, createdBy *string, req CreateCandidateRequest) (*Candidate, error)
	UpdateCandidate(ctx context.Context, orgID, ref string, req UpdateCandidateRequest) (*Candidate, error)
	DeleteCandidate(ctx context.Context, orgID, ref string) error

	// UploadResume validates and stores raw as the candidate's resume.
	// PDF only in Phase 4A — see the migration/plan notes on why DOCX is
	// deliberately not accepted (http.DetectContentType sniffs it as
	// application/zip, and trusting the extension is the exact bug the
	// avatar upload's own comment says it fixed).
	UploadResume(ctx context.Context, orgID, candidateRef string, raw []byte, originalFileName string) (*Candidate, error)
	// GetResumeFile loads the candidate (verifying org_id) and returns the
	// on-disk path to stream. Tenant isolation must not depend on path
	// secrecy — the org check happens before any filesystem access.
	GetResumeFile(ctx context.Context, orgID, candidateRef string) (*Candidate, string, error)
}

func (s *serviceImpl) ListCandidates(ctx context.Context, orgID string, filter CandidateListFilter) (*CandidateListResponse, error) {
	filter.Normalise()
	list, err := s.repo.FindCandidates(ctx, orgID, filter)
	if err != nil {
		return nil, fmt.Errorf("recruitment: ListCandidates: %w", err)
	}
	if list == nil {
		list = []*Candidate{}
	}
	total, err := s.repo.CountCandidates(ctx, orgID, filter)
	if err != nil {
		return nil, fmt.Errorf("recruitment: ListCandidates: count: %w", err)
	}
	return &CandidateListResponse{Candidates: list, Total: total, Limit: filter.Limit, Offset: filter.Offset}, nil
}

func (s *serviceImpl) GetCandidate(ctx context.Context, orgID, ref string) (*Candidate, error) {
	c, err := s.repo.FindCandidateByRef(ctx, orgID, ref)
	if err != nil {
		return nil, fmt.Errorf("recruitment: GetCandidate: %w", err)
	}
	if c == nil {
		return nil, ErrCandidateNotFound
	}
	return c, nil
}

// CreateCandidate returns ErrCandidateEmailExists on a duplicate rather than
// silently returning the existing record — crm/leads.CreateLead does the
// latter and it is a documented defect (Fix Pass A item 5). A second
// application for an existing candidate is a separate, explicit
// POST .../applications with that candidate_id.
func (s *serviceImpl) CreateCandidate(ctx context.Context, orgID string, createdBy *string, req CreateCandidateRequest) (*Candidate, error) {
	firstName := strings.TrimSpace(req.FirstName)
	if firstName == "" {
		return nil, ErrFirstNameRequired
	}

	var email *string
	if req.Email != nil {
		normalised := strings.ToLower(strings.TrimSpace(*req.Email))
		if normalised != "" {
			existing, err := s.repo.FindCandidateByEmail(ctx, orgID, normalised)
			if err != nil {
				return nil, fmt.Errorf("recruitment: CreateCandidate: dedup check: %w", err)
			}
			if existing != nil {
				return nil, ErrCandidateEmailExists
			}
			email = &normalised
		}
	}

	source := CandidateSourceDirect
	if req.Source != nil {
		source = CandidateSource(strings.TrimSpace(*req.Source))
		if !source.IsValid() {
			return nil, ErrInvalidCandidateSource
		}
	}

	c := &Candidate{
		OrgID: orgID, FirstName: firstName, LastName: req.LastName, Email: email, Phone: req.Phone,
		Headline: req.Headline, Location: req.Location, LinkedInURL: req.LinkedInURL, PortfolioURL: req.PortfolioURL,
		Source: source, ReferredByEmployeeID: req.ReferredByEmployeeID, Notes: req.Notes, CreatedBy: createdBy,
	}
	if err := s.repo.CreateCandidate(ctx, c); err != nil {
		return nil, fmt.Errorf("recruitment: CreateCandidate: %w", err)
	}
	return c, nil
}

func (s *serviceImpl) UpdateCandidate(ctx context.Context, orgID, ref string, req UpdateCandidateRequest) (*Candidate, error) {
	c, err := s.repo.FindCandidateByRef(ctx, orgID, ref)
	if err != nil {
		return nil, fmt.Errorf("recruitment: UpdateCandidate: %w", err)
	}
	if c == nil {
		return nil, ErrCandidateNotFound
	}
	if req.FirstName != nil {
		fn := strings.TrimSpace(*req.FirstName)
		if fn == "" {
			return nil, ErrFirstNameRequired
		}
		c.FirstName = fn
	}
	if req.LastName != nil {
		c.LastName = req.LastName
	}
	if req.Email != nil {
		normalised := strings.ToLower(strings.TrimSpace(*req.Email))
		if normalised != "" {
			existing, err := s.repo.FindCandidateByEmail(ctx, orgID, normalised)
			if err != nil {
				return nil, fmt.Errorf("recruitment: UpdateCandidate: dedup check: %w", err)
			}
			if existing != nil && existing.ID != c.ID {
				return nil, ErrCandidateEmailExists
			}
			c.Email = &normalised
		} else {
			c.Email = nil
		}
	}
	if req.Phone != nil {
		c.Phone = req.Phone
	}
	if req.Headline != nil {
		c.Headline = req.Headline
	}
	if req.Location != nil {
		c.Location = req.Location
	}
	if req.LinkedInURL != nil {
		c.LinkedInURL = req.LinkedInURL
	}
	if req.PortfolioURL != nil {
		c.PortfolioURL = req.PortfolioURL
	}
	if req.Notes != nil {
		c.Notes = req.Notes
	}
	if err := s.repo.UpdateCandidate(ctx, c); err != nil {
		return nil, fmt.Errorf("recruitment: UpdateCandidate: %w", err)
	}
	return c, nil
}

func (s *serviceImpl) DeleteCandidate(ctx context.Context, orgID, ref string) error {
	c, err := s.repo.FindCandidateByRef(ctx, orgID, ref)
	if err != nil {
		return fmt.Errorf("recruitment: DeleteCandidate: %w", err)
	}
	if c == nil {
		return ErrCandidateNotFound
	}
	if err := s.repo.SoftDeleteCandidate(ctx, orgID, c.ID); err != nil {
		return fmt.Errorf("recruitment: DeleteCandidate: %w", err)
	}
	return nil
}

// UploadResume mirrors internal/user/avatar.go's Upload: sniff real content
// (never trust the filename extension), hash, write once, record on the row.
func (s *serviceImpl) UploadResume(ctx context.Context, orgID, candidateRef string, raw []byte, originalFileName string) (*Candidate, error) {
	if len(raw) > resumeMaxUploadBytes {
		return nil, ErrResumeTooLarge
	}
	// Content sniffing, not extension — a renamed non-PDF file must not pass.
	// PDF-only in 4A: DOCX sniffs as application/zip via DetectContentType,
	// and half-validating (trusting the extension for that one case) is
	// worse than not accepting it — see migration 00078's design notes.
	contentType := http.DetectContentType(raw)
	if contentType != "application/pdf" {
		return nil, ErrInvalidResumeType
	}

	c, err := s.repo.FindCandidateByRef(ctx, orgID, candidateRef)
	if err != nil {
		return nil, fmt.Errorf("recruitment: UploadResume: %w", err)
	}
	if c == nil {
		return nil, ErrCandidateNotFound
	}

	sum := sha256.Sum256(raw)
	hash := hex.EncodeToString(sum[:])

	if err := os.MkdirAll(resumeStorageDir, 0o755); err != nil {
		return nil, fmt.Errorf("recruitment: UploadResume: mkdir: %w", err)
	}
	diskPath := filepath.Join(resumeStorageDir, hash+".pdf")
	// Content-addressed: two candidates uploading byte-identical files share
	// one path. Skip the write if it already exists — do not overwrite (the
	// bytes are identical by construction of the hash) and do not risk a
	// concurrent-write race with another upload of the same file.
	if _, statErr := os.Stat(diskPath); errors.Is(statErr, os.ErrNotExist) {
		if err := os.WriteFile(diskPath, raw, 0o644); err != nil {
			return nil, fmt.Errorf("recruitment: UploadResume: write file: %w", err)
		}
	}

	fileName := strings.TrimSpace(originalFileName)
	if fileName == "" {
		fileName = hash + ".pdf"
	}

	if err := s.repo.SetCandidateResume(ctx, c.ID, diskPath, fileName, contentType, int64(len(raw)), hash); err != nil {
		return nil, fmt.Errorf("recruitment: UploadResume: %w", err)
	}
	c.ResumeFilePath = &diskPath
	c.ResumeFileName = &fileName
	c.ResumeMimeType = &contentType
	size := int64(len(raw))
	c.ResumeSizeBytes = &size
	c.ResumeSHA256 = &hash
	return c, nil
}

func (s *serviceImpl) GetResumeFile(ctx context.Context, orgID, candidateRef string) (*Candidate, string, error) {
	c, err := s.repo.FindCandidateByRef(ctx, orgID, candidateRef)
	if err != nil {
		return nil, "", fmt.Errorf("recruitment: GetResumeFile: %w", err)
	}
	if c == nil {
		return nil, "", ErrCandidateNotFound
	}
	if !c.HasResume() {
		return nil, "", ErrNoResumeOnFile
	}
	return c, *c.ResumeFilePath, nil
}
