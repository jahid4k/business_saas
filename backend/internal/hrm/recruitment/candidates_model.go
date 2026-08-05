// backend/internal/hrm/recruitment/candidates_model.go
package recruitment

import (
	"errors"
	"time"
)

type CandidateSource string

const (
	CandidateSourceCareersPage CandidateSource = "careers_page"
	CandidateSourceReferral    CandidateSource = "referral"
	CandidateSourceAgency      CandidateSource = "agency"
	CandidateSourceSourced     CandidateSource = "sourced"
	CandidateSourceDirect      CandidateSource = "direct"
	CandidateSourceImport      CandidateSource = "import"
	CandidateSourceOther       CandidateSource = "other"
)

func (s CandidateSource) IsValid() bool {
	switch s {
	case CandidateSourceCareersPage, CandidateSourceReferral, CandidateSourceAgency,
		CandidateSourceSourced, CandidateSourceDirect, CandidateSourceImport, CandidateSourceOther:
		return true
	}
	return false
}

// Candidate is a person. Distinct from Application — one candidate may apply
// to many postings. Resume fields point at a content-addressed file outside
// ./uploads; see candidates_service.go for the upload/download path.
type Candidate struct {
	ID                   string          `db:"id"                       json:"id"`
	PublicID             string          `db:"public_id"                json:"public_id"`
	OrgID                string          `db:"org_id"                   json:"org_id"`
	FirstName            string          `db:"first_name"               json:"first_name"`
	LastName             *string         `db:"last_name"                json:"last_name,omitempty"`
	Email                *string         `db:"email"                    json:"email,omitempty"`
	Phone                *string         `db:"phone"                    json:"phone,omitempty"`
	Headline             *string         `db:"headline"                 json:"headline,omitempty"`
	Location             *string         `db:"location"                 json:"location,omitempty"`
	LinkedInURL          *string         `db:"linkedin_url"              json:"linkedin_url,omitempty"`
	PortfolioURL         *string         `db:"portfolio_url"             json:"portfolio_url,omitempty"`
	Source               CandidateSource `db:"source"              json:"source"`
	ReferredByEmployeeID *string         `db:"referred_by_employee_id"  json:"referred_by_employee_id,omitempty"`
	ResumeFilePath       *string         `db:"resume_file_path"         json:"-"`
	ResumeFileName       *string         `db:"resume_file_name"         json:"resume_file_name,omitempty"`
	ResumeMimeType       *string         `db:"resume_mime_type"         json:"resume_mime_type,omitempty"`
	ResumeSizeBytes      *int64          `db:"resume_size_bytes"        json:"resume_size_bytes,omitempty"`
	ResumeSHA256         *string         `db:"resume_sha256"            json:"-"`
	Notes                *string         `db:"notes"                    json:"notes,omitempty"`
	PurgeAfter           *time.Time      `db:"purge_after"              json:"purge_after,omitempty"`
	CreatedBy            *string         `db:"created_by"               json:"created_by,omitempty"`
	CreatedAt            time.Time       `db:"created_at"                json:"created_at"`
	UpdatedAt            time.Time       `db:"updated_at"                json:"updated_at"`
}

// HasResume reports whether a resume file is on record — the handler uses
// this to 404 a download request before touching the filesystem.
func (c *Candidate) HasResume() bool {
	return c.ResumeFilePath != nil && *c.ResumeFilePath != ""
}

type CreateCandidateRequest struct {
	FirstName            string  `json:"first_name"`
	LastName             *string `json:"last_name"`
	Email                *string `json:"email"`
	Phone                *string `json:"phone"`
	Headline             *string `json:"headline"`
	Location             *string `json:"location"`
	LinkedInURL          *string `json:"linkedin_url"`
	PortfolioURL         *string `json:"portfolio_url"`
	Source               *string `json:"source"`
	ReferredByEmployeeID *string `json:"referred_by_employee_id"`
	Notes                *string `json:"notes"`
}

type UpdateCandidateRequest struct {
	FirstName    *string `json:"first_name"`
	LastName     *string `json:"last_name"`
	Email        *string `json:"email"`
	Phone        *string `json:"phone"`
	Headline     *string `json:"headline"`
	Location     *string `json:"location"`
	LinkedInURL  *string `json:"linkedin_url"`
	PortfolioURL *string `json:"portfolio_url"`
	Notes        *string `json:"notes"`
}

type CandidateListFilter struct {
	Search string
	Limit  int
	Offset int
}

func (f *CandidateListFilter) Normalise() {
	if f.Limit <= 0 {
		f.Limit = DefaultLimit
	}
	if f.Limit > MaxLimit {
		f.Limit = MaxLimit
	}
	if f.Offset < 0 {
		f.Offset = 0
	}
}

type CandidateListResponse struct {
	Candidates []*Candidate `json:"candidates"`
	Total      int          `json:"total"`
	Limit      int          `json:"limit"`
	Offset     int          `json:"offset"`
}

var (
	ErrCandidateNotFound      = errors.New("candidate not found")
	ErrFirstNameRequired      = errors.New("first_name is required")
	ErrCandidateEmailExists   = errors.New("a candidate with this email already exists in this organization")
	ErrInvalidCandidateSource = errors.New("invalid source value")
	ErrNoResumeOnFile         = errors.New("no resume on file for this candidate")
	ErrInvalidResumeType      = errors.New("only PDF resumes are accepted")
	ErrResumeTooLarge         = errors.New("resume file exceeds the size limit")
)
