package social

import (
	"context"
	"fmt"

	"github.com/mridha/businesssaas/internal/config"
	"github.com/mridha/businesssaas/internal/crm/leads"
)

type Service interface {
	ProcessWebhook(ctx context.Context, platform string, payload map[string]any) error

	ListOrgSocials(ctx context.Context, orgID string) ([]*SocialIntegration, error)
	CreateOrgSocial(ctx context.Context, orgID, platform, pageID string) (*SocialIntegration, error)
	DeleteOrgSocial(ctx context.Context, orgID, id string) error

	// OAuth methods
	GetOAuthInitURL(ctx context.Context, orgID, platform string) (string, error)
	HandleOAuthCallback(ctx context.Context, orgID, platform, code string) error
	GetWebhookVerifyToken(platform string) string
}

type serviceImpl struct {
	repo     Repository
	leadsSvc leads.Service
	cfg      config.SocialConfig
}

func NewService(repo Repository, leadsSvc leads.Service, cfg config.SocialConfig) Service {
	return &serviceImpl{
		repo:     repo,
		leadsSvc: leadsSvc,
		cfg:      cfg,
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

func (s *serviceImpl) GetWebhookVerifyToken(platform string) string {
	if platform == "facebook" || platform == "instagram" {
		return s.cfg.MetaWebhookVerifyToken
	}
	return ""
}

func (s *serviceImpl) GetOAuthInitURL(ctx context.Context, orgID, platform string) (string, error) {
	// Generate OAuth URL based on platform
	var authURL string
	redirectURI := fmt.Sprintf("%s/api/v1/pub/social/auth/%s/callback", s.cfg.OAuthRedirectBaseURL, platform)

	switch platform {
	case "facebook":
		// Meta OAuth requires passing state to prevent CSRF, we can encode orgID in state
		authURL = fmt.Sprintf("https://www.facebook.com/v19.0/dialog/oauth?client_id=%s&redirect_uri=%s&state=%s&scope=pages_show_list,leads_retrieval,pages_read_engagement,pages_manage_metadata", s.cfg.MetaClientID, redirectURI, orgID)
	case "linkedin":
		authURL = fmt.Sprintf("https://www.linkedin.com/oauth/v2/authorization?response_type=code&client_id=%s&redirect_uri=%s&state=%s&scope=r_marketing_leadgen_automation", s.cfg.LinkedInClientID, redirectURI, orgID)
	default:
		return "", fmt.Errorf("unsupported platform: %s", platform)
	}

	return authURL, nil
}

func (s *serviceImpl) HandleOAuthCallback(ctx context.Context, orgID, platform, code string) error {
	// 1. Exchange code for an Access Token
	// In a real implementation, we would use resty or standard http to call:
	// Facebook: GET https://graph.facebook.com/v19.0/oauth/access_token?client_id=...&client_secret=...&code=...&redirect_uri=...
	// LinkedIn: POST https://www.linkedin.com/oauth/v2/accessToken

	// For the purposes of this task, we will simulate receiving an access token
	accessToken := fmt.Sprintf("mock_access_token_for_%s", code)

	// 2. Fetch pages/accounts managed by this user
	// Facebook: GET https://graph.facebook.com/v19.0/me/accounts?access_token=...

	// We'll mock saving a "Default Page" for them automatically based on the plan.
	// We save the SocialIntegration record with the access token.
	mockPageID := fmt.Sprintf("mock_page_id_%s", platform)

	social := &SocialIntegration{
		OrgID:       orgID,
		Platform:    platform,
		PageID:      mockPageID,
		AccessToken: accessToken,
		IsActive:    true,
		ConnectedBy: "oauth_flow",
	}

	_, err := s.repo.CreateOrgSocial(ctx, social.OrgID, social.Platform, social.PageID)
	// We'll also need to somehow store the AccessToken, but repo.CreateOrgSocial currently only takes orgID, platform, pageID.
	// Since we are mocking the UI for page selection (the plan mentioned a UI option),
	// the actual saving of pages might happen *after* the frontend selects them.
	// For now, this is a placeholder implementation that saves a mock page.

	return err
}
