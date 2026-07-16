package social

import (
	"context"
	"fmt"

	"github.com/mridha/businesssaas/internal/crm/leads"
)

type Service interface {
	ProcessWebhook(ctx context.Context, platform string, payload map[string]any) error

	ListOrgSocials(ctx context.Context, orgID string) ([]*SocialIntegration, error)
	CreateOrgSocial(ctx context.Context, orgID, platform, pageID string) (*SocialIntegration, error)
	DeleteOrgSocial(ctx context.Context, orgID, id string) error
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

func (s *serviceImpl) ProcessWebhook(ctx context.Context, platform string, payload map[string]any) error {
	// Extract page_id (this varies heavily by platform, e.g. Facebook sends it inside `entry[0].id`)
	// For this mock implementation as per Task.md, we'll try a generic or flat extraction.
	pageID, _ := payload["page_id"].(string)
	if pageID == "" {
		// Mock logic: try to extract from Meta format `entry[0].id`
		if entries, ok := payload["entry"].([]any); ok && len(entries) > 0 {
			if entryMap, ok := entries[0].(map[string]any); ok {
				if id, ok := entryMap["id"].(string); ok {
					pageID = id
				}
			}
		}
	}

	if pageID == "" {
		_ = s.repo.CreateLog(ctx, &SocialLeadLog{
			Platform:     platform,
			PageID:       "unknown",
			RawPayload:   payload,
			Processed:    false,
			ErrorMessage: stringPtr("Missing page_id in payload"),
		})
		return nil // Return 200 so webhook doesn't retry
	}

	orgID, err := s.repo.GetOrgByPageID(ctx, platform, pageID)
	if err != nil {
		return fmt.Errorf("social: GetOrgByPageID: %w", err)
	}

	logRecord := &SocialLeadLog{
		Platform:   platform,
		PageID:     pageID,
		RawPayload: payload,
	}

	if orgID != "" {
		logRecord.OrgID = &orgID
	}

	if orgID == "" {
		logRecord.Processed = false
		logRecord.ErrorMessage = stringPtr("No active organization found for page_id")
		_ = s.repo.CreateLog(ctx, logRecord)
		return nil
	}

	// Extract standard lead fields from mock payload
	// In production, we'd hit Facebook Graph API to fetch lead details using leadgen_id
	// But for this mockup/stub, we map flat values if provided.
	firstName, _ := payload["first_name"].(string)
	lastName, _ := payload["last_name"].(string)
	email, _ := payload["email"].(string)
	phone, _ := payload["phone"].(string)

	if firstName == "" {
		firstName = "Social Lead"
	}

	source := fmt.Sprintf("%s_lead_ad", platform)

	req := leads.CreateLeadRequest{
		FirstName:     firstName,
		CaptureSource: &source,
		CaptureMetadata: map[string]any{
			"page_id": pageID,
		},
	}

	if lastName != "" {
		req.LastName = &lastName
	}
	if email != "" {
		req.Email = &email
	}
	if phone != "" {
		req.Phone = &phone
	}

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

func stringPtr(s string) *string {
	return &s
}

func (s *serviceImpl) ListOrgSocials(ctx context.Context, orgID string) ([]*SocialIntegration, error) {
	return s.repo.ListOrgSocials(ctx, orgID)
}

func (s *serviceImpl) CreateOrgSocial(ctx context.Context, orgID, platform, pageID string) (*SocialIntegration, error) {
	return s.repo.CreateOrgSocial(ctx, orgID, platform, pageID)
}

func (s *serviceImpl) DeleteOrgSocial(ctx context.Context, orgID, id string) error {
	return s.repo.DeleteOrgSocial(ctx, orgID, id)
}
