package email

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/mridha/businesssaas/internal/crm/leads"
	"github.com/mridha/businesssaas/internal/platform/tickets"
)

// TicketRaiser is the narrow slice of the ticket engine this package needs.
// Declared HERE, by the consumer, and naming tickets' own request and result
// types so tickets.Service satisfies it structurally with no adapter — the
// corrected certifications.SkillGranter precedent, also used for
// assets.HandoverAcknowledger and expenses.ReimbursementCreator.
//
// The direction matters: capture/email imports platform/tickets, never the
// reverse. platform/tickets must not know that inbound email exists.
type TicketRaiser interface {
	Create(ctx context.Context, orgID, callerUserID string, req tickets.CreateTicketRequest) (*tickets.Ticket, error)
}

type Service interface {
	ProcessInboundWebhook(ctx context.Context, payload map[string]any) error

	// Settings
	ListOrgEmails(ctx context.Context, orgID string) ([]*OrgInboundEmail, error)
	CreateOrgEmail(ctx context.Context, orgID string, address string, destination Destination) (*OrgInboundEmail, error)
	DeleteOrgEmail(ctx context.Context, orgID string, id string) error
}

type serviceImpl struct {
	repo       Repository
	leadsSvc   leads.Service
	ticketsSvc TicketRaiser
}

// NewService takes ticketsSvc as a nil-safe dependency: an install with no
// ticket engine wired still routes leads, and a 'ticket' address there is
// logged as unroutable rather than panicking inside a webhook handler.
func NewService(repo Repository, leadsSvc leads.Service, ticketsSvc TicketRaiser) Service {
	return &serviceImpl{
		repo:       repo,
		leadsSvc:   leadsSvc,
		ticketsSvc: ticketsSvc,
	}
}

// ErrNoTicketEngine is recorded when a 'ticket' address is hit on an install
// with no ticket engine wired.
var ErrNoTicketEngine = errors.New("no ticket engine is wired for this deployment")

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

	// Resolve the address to an org AND a destination. Before Phase 8D this
	// was an org-only lookup, because lead creation was the only consumer.
	route, err := s.repo.GetRouteByAddress(ctx, to)
	if err != nil {
		return fmt.Errorf("email: GetRouteByAddress: %w", err)
	}

	logRecord := &InboundEmailLog{
		ToAddress:   to,
		FromAddress: from,
		Subject:     &subject,
		RawPayload:  payload,
	}

	if route == nil {
		logRecord.Processed = false
		logRecord.ErrorMessage = stringPtr("No active organization found for address")
		_ = s.repo.CreateLog(ctx, logRecord)
		return nil
	}
	logRecord.OrgID = &route.OrgID

	// THE ROUTER. Every failure below is recorded on the log row and
	// swallowed, because the webhook provider must get a 200 or it retries
	// the same message forever. inbound_email_logs.error_message is the only
	// place an operator can see what went wrong.
	switch route.Destination {
	case DestinationTicket:
		err = s.raiseTicket(ctx, route.OrgID, from, subject, text)
	default:
		// DestinationLead, and anything unrecognised. Falling back to the
		// pre-8D behaviour is deliberate: a destination this build does not
		// understand must not silently drop the message.
		err = s.captureLead(ctx, route.OrgID, from, subject, text)
	}

	if err != nil {
		logRecord.Processed = false
		logRecord.ErrorMessage = stringPtr(err.Error())
	} else {
		logRecord.Processed = true
	}

	_ = s.repo.CreateLog(ctx, logRecord)
	return nil
}

// captureLead is the original, unchanged behaviour of this pipeline, moved
// into its own branch. Nothing about it was altered by the routing work.
func (s *serviceImpl) captureLead(ctx context.Context, orgID, from, subject, text string) error {
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

	// Create the lead (empty userID since it's system-generated — created_by
	// is nullable as of migration 00112 for exactly this reason).
	_, err := s.leadsSvc.CreateLead(ctx, orgID, "", req)
	return err
}

// raiseTicket is the additive branch. It refuses rather than improvises: a
// ticket needs a real requester, and platform_tickets.requester_id and
// .requester_user_id are both NOT NULL. Attaching a stranger's email to some
// fallback employee would put words in that person's mouth — in an HR
// helpdesk that may carry a grievance, that is the worst possible failure.
func (s *serviceImpl) raiseTicket(ctx context.Context, orgID, from, subject, text string) error {
	if s.ticketsSvc == nil {
		return ErrNoTicketEngine
	}
	requester, err := s.repo.FindEmployeeRequester(ctx, orgID, from)
	if err != nil {
		return fmt.Errorf("email: FindEmployeeRequester: %w", err)
	}
	if requester == nil {
		return fmt.Errorf("email: sender %s is not an employee of this organization, so no ticket requester could be resolved", from)
	}

	if strings.TrimSpace(subject) == "" {
		subject = "(no subject)"
	}
	var body *string
	if strings.TrimSpace(text) != "" {
		body = &text
	}

	// Created AS the requester, not as a system user: the ticket is theirs,
	// it must appear in their own list, and they must be able to comment on
	// it. tickets.Service resolves permissions from this caller id.
	_, err = s.ticketsSvc.Create(ctx, orgID, requester.UserID, tickets.CreateTicketRequest{
		RequesterID: requester.EmployeeID,
		Subject:     subject,
		Description: body,
	})
	return err
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

func (s *serviceImpl) CreateOrgEmail(ctx context.Context, orgID string, address string, destination Destination) (*OrgInboundEmail, error) {
	// An unspecified destination is 'lead', matching the column default, so
	// an older client that does not send the field keeps its behaviour.
	if destination == "" {
		destination = DestinationLead
	}
	if !destination.IsValid() {
		return nil, fmt.Errorf("email: CreateOrgEmail: unrecognised destination %q", destination)
	}
	return s.repo.CreateOrgEmail(ctx, orgID, address, destination)
}

func (s *serviceImpl) DeleteOrgEmail(ctx context.Context, orgID string, id string) error {
	return s.repo.DeleteOrgEmail(ctx, orgID, id)
}
