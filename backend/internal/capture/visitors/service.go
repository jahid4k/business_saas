package visitors

import (
	"context"
	"fmt"

	"github.com/mridha/businesssaas/internal/crm/leads"
)

type Service interface {
	IdentifyVisitor(ctx context.Context, orgID, ip, userAgent string, req IdentifyRequest) error
	ListVisitors(ctx context.Context, orgID string) ([]*WebsiteVisitor, error)
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

func (s *serviceImpl) IdentifyVisitor(ctx context.Context, orgID, ip, userAgent string, req IdentifyRequest) error {
	visitor, err := s.repo.FindVisitorBySession(ctx, orgID, req.SessionID)
	if err != nil {
		return fmt.Errorf("visitors: IdentifyVisitor: find: %w", err)
	}

	isNew := false
	if visitor == nil {
		isNew = true
		visitor = &WebsiteVisitor{
			OrgID:     orgID,
			SessionID: req.SessionID,
			IPAddress: &ip,
			UserAgent: &userAgent,
		}
	} else {
		// Update IP/UserAgent if they changed
		visitor.IPAddress = &ip
		visitor.UserAgent = &userAgent
	}

	// Mock IP Enrichment (in reality we'd call Clearbit, 6sense, or ipinfo here)
	// Task says "If an IP is identified (mocked for now), create a Lead"
	// Let's pretend if they pass traits["email"], we create a lead.
	if req.Traits != nil {
		email, okEmail := req.Traits["email"].(string)
		name, _ := req.Traits["name"].(string)
		company, okCompany := req.Traits["company"].(string)

		if visitor.LinkedLeadID == nil && (okEmail || okCompany) {
			if name == "" {
				name = "Website Visitor"
			}
			source := "website_visitor"
			leadReq := leads.CreateLeadRequest{
				FirstName:     name,
				CaptureSource: &source,
			}
			if okEmail {
				leadReq.Email = &email
			}
			if okCompany {
				leadReq.CompanyName = &company
				visitor.CompanyName = &company
			}

			// Create lead
			lead, err := s.leadsSvc.CreateLead(ctx, orgID, "", leadReq)
			if err == nil && lead != nil {
				visitor.LinkedLeadID = &lead.ID
			}
		}
	}

	if isNew {
		if err := s.repo.CreateVisitor(ctx, visitor); err != nil {
			return fmt.Errorf("visitors: CreateVisitor: %w", err)
		}
	} else {
		if err := s.repo.UpdateVisitor(ctx, visitor); err != nil {
			return fmt.Errorf("visitors: UpdateVisitor: %w", err)
		}
	}

	// Log pageview
	pv := &VisitorPageview{
		VisitorID: visitor.ID,
		URL:       req.URL,
		Title:     req.Title,
		Referrer:  req.Referrer,
	}
	if err := s.repo.CreatePageview(ctx, pv); err != nil {
		return fmt.Errorf("visitors: CreatePageview: %w", err)
	}

	return nil
}

func (s *serviceImpl) ListVisitors(ctx context.Context, orgID string) ([]*WebsiteVisitor, error) {
	return s.repo.ListVisitors(ctx, orgID)
}
