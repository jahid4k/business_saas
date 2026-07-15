package email

import (
	"context"
	"fmt"
	"strings"

	"github.com/mridha/businesssaas/internal/crm/leads"
)

type Service interface {
	ProcessInboundWebhook(ctx context.Context, payload map[string]any) error

	// Settings
	ListOrgEmails(ctx context.Context, orgID string) ([]*OrgInboundEmail, error)
	CreateOrgEmail(ctx context.Context, orgID string, address string) (*OrgInboundEmail, error)
	DeleteOrgEmail(ctx context.Context, orgID string, id string) error
}

type serviceImpl struct {
	repo     Repository
	leadsSvc leads.Service
}

func NewService(repo Repository, leadsSvc leads.Service) Service {
	return &serviceImpl{
		repo:     repo,
		leadsSvc: leadsSvc,
	}
}

func (s *serviceImpl) ProcessInboundWebhook(ctx context.Context, payload map[string]any) error {
	// Standard fields from typical email webhook (e.g. SendGrid Inbound Parse)
	to, _ := payload["to"].(string)
	from, _ := payload["from"].(string)
	subject, _ := payload["subject"].(string)
	text, _ := payload["text"].(string)

	if to == "" || from == "" {
		// Log generic failure if missing required fields
		_ = s.repo.CreateLog(ctx, &InboundEmailLog{
			ToAddress:    to,
			FromAddress:  from,
			RawPayload:   payload,
			Processed:    false,
			ErrorMessage: stringPtr("Missing 'to' or 'from' in payload"),
		})
		return nil // Return 200 to webhook provider so they don't retry
	}

	// Clean addresses (e.g., "John Doe <john@doe.com>" -> "john@doe.com")
	to = extractEmail(to)
	from = extractEmail(from)

	// Lookup Org
	orgID, err := s.repo.GetOrgByAddress(ctx, to)
	if err != nil {
		return fmt.Errorf("email: GetOrgByAddress: %w", err)
	}

	logRecord := &InboundEmailLog{
		ToAddress:   to,
		FromAddress: from,
		Subject:     &subject,
		RawPayload:  payload,
	}

	if orgID != "" {
		logRecord.OrgID = &orgID
	}

	if orgID == "" {
		logRecord.Processed = false
		logRecord.ErrorMessage = stringPtr("No active organization found for address")
		_ = s.repo.CreateLog(ctx, logRecord)
		return nil
	}

	// Create Lead
	source := "email"
	namePart := strings.Split(from, "@")[0] // Fallback for name

	req := leads.CreateLeadRequest{
		FirstName:     namePart, // Since FirstName is required
		Email:         &from,
		CaptureSource: &source,
		CaptureMetadata: map[string]any{
			"subject": subject,
			"body":    text,
		},
	}

	// If there's an obvious first/last name, we could parse it, but this is a simple implementation.
	
	// Create the lead (empty userID since it's system-generated)
	_, err = s.leadsSvc.CreateLead(ctx, orgID, "", req)
	
	if err != nil {
		logRecord.Processed = false
		logRecord.ErrorMessage = stringPtr(err.Error())
	} else {
		logRecord.Processed = true
	}

	_ = s.repo.CreateLog(ctx, logRecord)
	return nil
}

func extractEmail(raw string) string {
	// Simple extractor for "Name <email@domain.com>" -> "email@domain.com"
	start := strings.Index(raw, "<")
	end := strings.Index(raw, ">")
	if start != -1 && end != -1 && end > start {
		return strings.TrimSpace(raw[start+1 : end])
	}
	return strings.TrimSpace(raw)
}

func stringPtr(s string) *string {
	return &s
}

func (s *serviceImpl) ListOrgEmails(ctx context.Context, orgID string) ([]*OrgInboundEmail, error) {
	return s.repo.ListOrgEmails(ctx, orgID)
}

func (s *serviceImpl) CreateOrgEmail(ctx context.Context, orgID string, address string) (*OrgInboundEmail, error) {
	return s.repo.CreateOrgEmail(ctx, orgID, address)
}

func (s *serviceImpl) DeleteOrgEmail(ctx context.Context, orgID string, id string) error {
	return s.repo.DeleteOrgEmail(ctx, orgID, id)
}
