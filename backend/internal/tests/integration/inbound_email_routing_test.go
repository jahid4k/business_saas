// backend/internal/tests/integration/inbound_email_routing_test.go
// The inbound-email pipeline. Phase 8D generalizes ProcessInboundWebhook from
// a single-consumer lead pipeline into a router, which is the riskiest change
// in Phase 8 — it modifies something that already works in production.
//
// TestIntegration_InboundEmail_LeadCaptureStillWorks was written and made to
// pass BEFORE any of that change, and must stay green through all of it. If
// it ever goes red, the routing work has regressed lead capture and nothing
// else in this file matters.
// Gate: INTEGRATION=1
package integration

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	captureemail "github.com/mridha/businesssaas/internal/capture/email"
	"github.com/mridha/businesssaas/internal/platform/tickets"
)

// seedInboundAddress registers an inbound address for an org and returns it.
// An empty destination exercises the default path — the same thing an older
// client that does not send the field would do.
func seedInboundAddress(t *testing.T, env *testEnv, orgID, prefix string, dest captureemail.Destination) string {
	t.Helper()
	addr := fmt.Sprintf("%s-%d@inbound-test.local", prefix, time.Now().UnixNano())
	if _, err := env.emailSvc.CreateOrgEmail(context.Background(), orgID, addr, dest); err != nil {
		t.Fatalf("create org inbound address: %v", err)
	}
	return addr
}

func inboundPayload(to, from, subject, body string) map[string]any {
	return map[string]any{
		"to": to, "from": from, "subject": subject, "text": body,
	}
}

// countLeads returns how many leads the org holds.
func countLeads(t *testing.T, env *testEnv, orgID string) int {
	t.Helper()
	var n int
	if err := env.db.QueryRow(context.Background(),
		`SELECT COUNT(*) FROM crm_leads WHERE org_id = $1`, orgID).Scan(&n); err != nil {
		t.Fatalf("count leads: %v", err)
	}
	return n
}

// TestIntegration_InboundEmail_LeadCaptureStillWorks is the regression guard
// for the whole slice. An inbound email to an ordinary capture address must
// produce a lead, exactly as it did before routing existed.
func TestIntegration_InboundEmail_LeadCaptureStillWorks(t *testing.T) {
	env := newTestEnv(t)
	ctx := context.Background()
	orgID, _, _ := seedScopeTestOrg(t, env)
	// Deliberately created with NO destination — the pre-8D shape.
	addr := seedInboundAddress(t, env, orgID, "sales", "")

	before := countLeads(t, env, orgID)
	if err := env.emailSvc.ProcessInboundWebhook(ctx,
		inboundPayload(addr, "Jane Prospect <jane@prospect.example>", "Pricing enquiry",
			"How much for 40 seats?")); err != nil {
		t.Fatalf("ProcessInboundWebhook: %v", err)
	}
	if got := countLeads(t, env, orgID) - before; got != 1 {
		// ProcessInboundWebhook swallows every failure into the log row by
		// design (the webhook provider must get a 200), so the reason lives
		// there. Surfacing it here matters: this test is the tripwire for the
		// whole slice, and "0 leads" alone tells whoever trips it nothing.
		t.Fatalf("created %d leads, want 1 — lead capture has regressed; log says: %s",
			got, lastLogError(t, env, addr))
	}

	var email, source string
	if err := env.db.QueryRow(ctx,
		`SELECT email, capture_source FROM crm_leads WHERE org_id = $1 ORDER BY created_at DESC LIMIT 1`,
		orgID).Scan(&email, &source); err != nil {
		t.Fatalf("read lead: %v", err)
	}
	if email != "jane@prospect.example" {
		t.Errorf("lead email = %q, want the sender's address extracted from the display-name form", email)
	}
	if source != "email" {
		t.Errorf("capture_source = %q, want \"email\"", source)
	}

	// The log row is how an operator debugs a webhook, so it must record the
	// success as well as the failures.
	var processed bool
	var logged *string
	if err := env.db.QueryRow(ctx,
		`SELECT processed, error_message FROM inbound_email_logs
		  WHERE org_id = $1 ORDER BY created_at DESC LIMIT 1`, orgID).Scan(&processed, &logged); err != nil {
		t.Fatalf("read log: %v", err)
	}
	if !processed || logged != nil {
		t.Errorf("log says processed=%v error=%v, want processed with no error", processed, logged)
	}
}

// TestIntegration_InboundEmail_UnknownAddressIsLoggedNotErrored pins the
// webhook contract: an unroutable email must be recorded and swallowed, not
// returned as an error, or the provider retries it forever.
func TestIntegration_InboundEmail_UnknownAddressIsLoggedNotErrored(t *testing.T) {
	env := newTestEnv(t)
	ctx := context.Background()

	unknown := fmt.Sprintf("nobody-%d@inbound-test.local", time.Now().UnixNano())
	if err := env.emailSvc.ProcessInboundWebhook(ctx,
		inboundPayload(unknown, "someone@example.com", "Hello", "Anyone there?")); err != nil {
		t.Fatalf("ProcessInboundWebhook returned an error for an unknown address: %v", err)
	}

	var processed bool
	var msg *string
	if err := env.db.QueryRow(ctx,
		`SELECT processed, error_message FROM inbound_email_logs
		  WHERE to_address = $1 ORDER BY created_at DESC LIMIT 1`, unknown).Scan(&processed, &msg); err != nil {
		t.Fatalf("read log: %v", err)
	}
	if processed {
		t.Error("an email to an unregistered address was marked processed")
	}
	if msg == nil {
		t.Error("no error_message recorded — an operator has nothing to debug from")
	}
	t.Cleanup(func() {
		_, _ = env.db.Exec(ctx, `DELETE FROM inbound_email_logs WHERE to_address = $1`, unknown)
	})
}

// lastLogError reads the failure ProcessInboundWebhook recorded rather than
// returned.
func lastLogError(t *testing.T, env *testEnv, toAddress string) string {
	t.Helper()
	var processed bool
	var msg *string
	err := env.db.QueryRow(context.Background(),
		`SELECT processed, error_message FROM inbound_email_logs
		  WHERE to_address = $1 ORDER BY created_at DESC LIMIT 1`, toAddress).Scan(&processed, &msg)
	if err != nil {
		return fmt.Sprintf("(no log row: %v)", err)
	}
	if msg == nil {
		return fmt.Sprintf("(processed=%v, no error recorded)", processed)
	}
	return *msg
}

// ============================================================
// The additive ticket branch
// ============================================================

// TestIntegration_InboundEmail_SupportAddressRaisesATicket is the feature.
// The SAME webhook call that makes a lead at a sales@ address makes a ticket
// at a support@ one — the receiving address decides, nothing about the
// message does.
func TestIntegration_InboundEmail_SupportAddressRaisesATicket(t *testing.T) {
	env := newTestEnv(t)
	ctx := context.Background()
	fx := seedTicketFixture(t, env)
	addr := seedInboundAddress(t, env, fx.orgID, "support", captureemail.DestinationTicket)

	// The sender must be an employee with a platform account, which the
	// fixture's member already is.
	var senderEmail string
	if err := env.db.QueryRow(ctx, `SELECT email FROM users WHERE id = $1`, fx.memberID).Scan(&senderEmail); err != nil {
		t.Fatalf("read sender email: %v", err)
	}

	leadsBefore := countLeads(t, env, fx.orgID)
	if err := env.emailSvc.ProcessInboundWebhook(ctx,
		inboundPayload(addr, senderEmail, "VPN keeps dropping", "Disconnects every ten minutes.")); err != nil {
		t.Fatalf("ProcessInboundWebhook: %v", err)
	}

	var subject, status, requesterUser, requesterID string
	var description *string
	if err := env.db.QueryRow(ctx,
		`SELECT subject, status, requester_user_id::text, requester_id::text, description
		   FROM platform_tickets WHERE org_id = $1 ORDER BY created_at DESC LIMIT 1`,
		fx.orgID).Scan(&subject, &status, &requesterUser, &requesterID, &description); err != nil {
		t.Fatalf("read ticket (log says: %s): %v", lastLogError(t, env, addr), err)
	}
	if subject != "VPN keeps dropping" {
		t.Errorf("subject = %q, want the email's subject", subject)
	}
	if description == nil || *description != "Disconnects every ten minutes." {
		t.Errorf("description = %v, want the email body", description)
	}
	// The ticket is the SENDER's, not a system user's — it must appear in
	// their own list and be commentable by them.
	if requesterUser != fx.memberID {
		t.Errorf("requester_user_id = %s, want the sender %s", requesterUser, fx.memberID)
	}
	if requesterID != fx.employeeID {
		t.Errorf("requester_id = %s, want the sender's employee id %s", requesterID, fx.employeeID)
	}
	if status != "open" {
		t.Errorf("status = %q, want open", status)
	}

	// The other branch must not have fired.
	if got := countLeads(t, env, fx.orgID) - leadsBefore; got != 0 {
		t.Errorf("a ticket-destination email also created %d leads", got)
	}

	// And the requester can actually see it through the ticket service.
	res, err := env.ticketsSvc.List(ctx, fx.orgID, fx.memberID, tickets.ListFilter{})
	if err != nil {
		t.Fatalf("list as requester: %v", err)
	}
	if res.Total < 1 {
		t.Error("the emailed-in ticket is not visible to the person who sent the email")
	}
}

// TestIntegration_InboundEmail_UnknownSenderGetsNoTicket proves the branch
// refuses rather than improvises. Attaching a stranger's email to a fallback
// employee would put words in that person's mouth — in an HR helpdesk that
// may carry a grievance, that is the worst available failure.
func TestIntegration_InboundEmail_UnknownSenderGetsNoTicket(t *testing.T) {
	env := newTestEnv(t)
	ctx := context.Background()
	fx := seedTicketFixture(t, env)
	addr := seedInboundAddress(t, env, fx.orgID, "support", captureemail.DestinationTicket)

	before := countTickets(t, env, fx.orgID)
	if err := env.emailSvc.ProcessInboundWebhook(ctx,
		inboundPayload(addr, "stranger@nowhere.example", "Let me in", "Hello")); err != nil {
		t.Fatalf("ProcessInboundWebhook returned an error rather than logging: %v", err)
	}
	if got := countTickets(t, env, fx.orgID) - before; got != 0 {
		t.Errorf("created %d tickets for a sender who is not an employee", got)
	}

	msg := lastLogError(t, env, addr)
	if !strings.Contains(msg, "not an employee") {
		t.Errorf("log says %q — an operator cannot tell why the ticket was refused", msg)
	}
}

// TestIntegration_InboundEmail_DestinationDefaultsToLead pins the property
// that makes migration 00112 a no-op for existing installs: the column
// default, the service default, and the router's fallback must all agree on
// 'lead'. If any one of them drifts, every pre-8D address silently changes
// behaviour.
func TestIntegration_InboundEmail_DestinationDefaultsToLead(t *testing.T) {
	env := newTestEnv(t)
	ctx := context.Background()
	orgID, _, _ := seedScopeTestOrg(t, env)

	// Inserted the way a pre-8D row exists: no destination named at all.
	addr := fmt.Sprintf("legacy-%d@inbound-test.local", time.Now().UnixNano())
	if _, err := env.db.Exec(ctx,
		`INSERT INTO org_inbound_emails (org_id, address, is_active) VALUES ($1, $2, TRUE)`,
		orgID, addr); err != nil {
		t.Fatalf("insert legacy address: %v", err)
	}

	var dest string
	if err := env.db.QueryRow(ctx,
		`SELECT destination FROM org_inbound_emails WHERE address = $1`, addr).Scan(&dest); err != nil {
		t.Fatalf("read destination: %v", err)
	}
	if dest != "lead" {
		t.Fatalf("column default is %q, want \"lead\" — existing addresses would change behaviour", dest)
	}

	before := countLeads(t, env, orgID)
	if err := env.emailSvc.ProcessInboundWebhook(ctx,
		inboundPayload(addr, "old@customer.example", "Still working?", "Body")); err != nil {
		t.Fatalf("ProcessInboundWebhook: %v", err)
	}
	if got := countLeads(t, env, orgID) - before; got != 1 {
		t.Errorf("a legacy address created %d leads, want 1; log says: %s", got, lastLogError(t, env, addr))
	}

	// The service default must agree with the column default.
	created, err := env.emailSvc.CreateOrgEmail(ctx, orgID,
		fmt.Sprintf("unspecified-%d@inbound-test.local", time.Now().UnixNano()), "")
	if err != nil {
		t.Fatalf("CreateOrgEmail with no destination: %v", err)
	}
	if created.Destination != captureemail.DestinationLead {
		t.Errorf("service default is %q, want lead", created.Destination)
	}
}

// countTickets returns how many tickets the org holds.
func countTickets(t *testing.T, env *testEnv, orgID string) int {
	t.Helper()
	var n int
	if err := env.db.QueryRow(context.Background(),
		`SELECT COUNT(*) FROM platform_tickets WHERE org_id = $1`, orgID).Scan(&n); err != nil {
		t.Fatalf("count tickets: %v", err)
	}
	return n
}
