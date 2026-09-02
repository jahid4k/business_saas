// backend/internal/hrm/recruitment/offers_service.go
package recruitment

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/mridha/businesssaas/internal/hrm/approvals"
)

// OfferService is embedded into Service — see service.go.
type OfferService interface {
	ListOffers(ctx context.Context, orgID, applicationID string) ([]*Offer, error)
	GetOffer(ctx context.Context, orgID, ref string) (*Offer, error)
	CreateOffer(ctx context.Context, orgID, applicationRef, createdBy string, req CreateOfferRequest) (*Offer, error)
	UpdateOffer(ctx context.Context, orgID, ref string, req UpdateOfferRequest) (*Offer, error)
	SubmitOffer(ctx context.Context, orgID, ref, submittedBy string) (*Offer, error)
	SendOffer(ctx context.Context, orgID, ref string) (*Offer, error)
	AcceptOffer(ctx context.Context, orgID, ref string) (*Offer, error)
	DeclineOffer(ctx context.Context, orgID, ref string) (*Offer, error)
	RescindOffer(ctx context.Context, orgID, ref string) (*Offer, error)
	// HandleOfferApprovalDecision implements approvals.EntityCallback —
	// registered via approvalsSvc.RegisterCallback("offer", ...) in main.go.
	HandleOfferApprovalDecision(ctx context.Context, orgID, entityID string, approved bool) error
}

func (s *serviceImpl) ListOffers(ctx context.Context, orgID, applicationID string) ([]*Offer, error) {
	app, err := s.repo.FindApplicationByRef(ctx, orgID, applicationID)
	if err != nil {
		return nil, fmt.Errorf("recruitment: ListOffers: %w", err)
	}
	if app == nil {
		return nil, ErrApplicationNotFound
	}
	list, err := s.repo.FindOffers(ctx, orgID, app.ID)
	if err != nil {
		return nil, fmt.Errorf("recruitment: ListOffers: %w", err)
	}
	if list == nil {
		list = []*Offer{}
	}
	return list, nil
}

func (s *serviceImpl) GetOffer(ctx context.Context, orgID, ref string) (*Offer, error) {
	o, err := s.repo.FindOfferByRef(ctx, orgID, ref)
	if err != nil {
		return nil, fmt.Errorf("recruitment: GetOffer: %w", err)
	}
	if o == nil {
		return nil, ErrOfferNotFound
	}
	return o, nil
}

func parseOptionalDate(v *string) (*time.Time, error) {
	if v == nil || strings.TrimSpace(*v) == "" {
		return nil, nil
	}
	d, err := time.Parse(dateLayout, strings.TrimSpace(*v))
	if err != nil {
		return nil, err
	}
	return &d, nil
}

func parseOptionalRFC3339(v *string) (*time.Time, error) {
	if v == nil || strings.TrimSpace(*v) == "" {
		return nil, nil
	}
	d, err := time.Parse(time.RFC3339, strings.TrimSpace(*v))
	if err != nil {
		return nil, err
	}
	return &d, nil
}

func (s *serviceImpl) CreateOffer(ctx context.Context, orgID, applicationRef, createdBy string, req CreateOfferRequest) (*Offer, error) {
	app, err := s.repo.FindApplicationByRef(ctx, orgID, applicationRef)
	if err != nil {
		return nil, fmt.Errorf("recruitment: CreateOffer: %w", err)
	}
	if app == nil {
		return nil, ErrApplicationNotFound
	}

	requisitionRef := strings.TrimSpace(req.RequisitionID)
	if requisitionRef == "" {
		return nil, ErrOfferRequisitionRequired
	}
	requisition, err := s.repo.FindRequisitionByRef(ctx, orgID, requisitionRef)
	if err != nil {
		return nil, fmt.Errorf("recruitment: CreateOffer: requisition: %w", err)
	}
	if requisition == nil {
		return nil, ErrRequisitionNotFound
	}

	currency := "USD"
	if req.SalaryCurrency != nil && strings.TrimSpace(*req.SalaryCurrency) != "" {
		currency = strings.ToUpper(strings.TrimSpace(*req.SalaryCurrency))
	}

	startDate, err := parseOptionalDate(req.StartDate)
	if err != nil {
		return nil, fmt.Errorf("recruitment: CreateOffer: start_date: %w", err)
	}
	expiresAt, err := parseOptionalRFC3339(req.ExpiresAt)
	if err != nil {
		return nil, fmt.Errorf("recruitment: CreateOffer: expires_at: %w", err)
	}

	o := &Offer{
		OrgID: orgID, ApplicationID: app.ID, RequisitionID: requisition.ID,
		BaseSalary: req.BaseSalary, SalaryCurrency: currency, SigningBonus: req.SigningBonus,
		EquityDetails: req.EquityDetails, StartDate: startDate, ExpiresAt: expiresAt,
		Status: OfferStatusDraft, CreatedBy: createdBy,
	}
	if err := s.repo.CreateOffer(ctx, o); err != nil {
		return nil, fmt.Errorf("recruitment: CreateOffer: %w", err)
	}
	return o, nil
}

func (s *serviceImpl) UpdateOffer(ctx context.Context, orgID, ref string, req UpdateOfferRequest) (*Offer, error) {
	o, err := s.repo.FindOfferByRef(ctx, orgID, ref)
	if err != nil {
		return nil, fmt.Errorf("recruitment: UpdateOffer: %w", err)
	}
	if o == nil {
		return nil, ErrOfferNotFound
	}
	if o.Status != OfferStatusDraft {
		return nil, ErrOfferWrongStatus
	}

	if req.BaseSalary != nil {
		o.BaseSalary = req.BaseSalary
	}
	if req.SalaryCurrency != nil && strings.TrimSpace(*req.SalaryCurrency) != "" {
		o.SalaryCurrency = strings.ToUpper(strings.TrimSpace(*req.SalaryCurrency))
	}
	if req.SigningBonus != nil {
		o.SigningBonus = req.SigningBonus
	}
	if req.EquityDetails != nil {
		o.EquityDetails = req.EquityDetails
	}
	if req.StartDate != nil {
		d, err := parseOptionalDate(req.StartDate)
		if err != nil {
			return nil, fmt.Errorf("recruitment: UpdateOffer: start_date: %w", err)
		}
		o.StartDate = d
	}
	if req.ExpiresAt != nil {
		d, err := parseOptionalRFC3339(req.ExpiresAt)
		if err != nil {
			return nil, fmt.Errorf("recruitment: UpdateOffer: expires_at: %w", err)
		}
		o.ExpiresAt = d
	}

	if err := s.repo.UpdateOffer(ctx, o); err != nil {
		return nil, fmt.Errorf("recruitment: UpdateOffer: %w", err)
	}
	return o, nil
}

// SubmitOffer mirrors SubmitRequisition's approval contract exactly:
// FindDefault → if a template exists, CreateInstance + persist
// approval_instance_id + status=pending_approval; if none exists, auto-
// approve — the established fallback across every approval-gated HRM
// workflow.
func (s *serviceImpl) SubmitOffer(ctx context.Context, orgID, ref, submittedBy string) (*Offer, error) {
	o, err := s.repo.FindOfferByRef(ctx, orgID, ref)
	if err != nil {
		return nil, fmt.Errorf("recruitment: SubmitOffer: %w", err)
	}
	if o == nil {
		return nil, ErrOfferNotFound
	}
	if o.Status != OfferStatusDraft {
		return nil, ErrOfferWrongStatus
	}

	tmpl, tErr := s.approvalsSvc.FindDefault(ctx, orgID, approvals.ActionTypeOffer)
	if tErr == nil && tmpl != nil {
		inst, iErr := s.approvalsSvc.CreateInstance(ctx, orgID, approvals.CreateInstanceRequest{
			TemplateID: tmpl.ID, EntityType: "offer", EntityID: o.ID, RequestedBy: submittedBy,
		})
		if iErr != nil {
			return nil, fmt.Errorf("recruitment: SubmitOffer: creating approval instance: %w", iErr)
		}
		if err := s.repo.SetOfferApprovalInstance(ctx, o.ID, inst.ID, OfferStatusPendingApproval); err != nil {
			return nil, fmt.Errorf("recruitment: SubmitOffer: %w", err)
		}
		o.ApprovalInstanceID = &inst.ID
		o.Status = OfferStatusPendingApproval
		return o, nil
	}

	if err := s.repo.UpdateOfferStatus(ctx, o.ID, OfferStatusApproved); err != nil {
		return nil, fmt.Errorf("recruitment: SubmitOffer: %w", err)
	}
	o.Status = OfferStatusApproved
	return o, nil
}

// HandleOfferApprovalDecision reacts to the offer's approval instance
// completing. Idempotency guard mirrors HandleApprovalDecision (requisitions).
func (s *serviceImpl) HandleOfferApprovalDecision(ctx context.Context, orgID, entityID string, approved bool) error {
	o, err := s.repo.FindOfferByRef(ctx, orgID, entityID)
	if err != nil {
		return fmt.Errorf("recruitment: HandleOfferApprovalDecision: %w", err)
	}
	if o == nil {
		return ErrOfferNotFound
	}
	if o.Status != OfferStatusPendingApproval {
		return nil
	}
	status := OfferStatusApproved
	if !approved {
		status = OfferStatusRejected
	}
	if err := s.repo.UpdateOfferStatus(ctx, o.ID, status); err != nil {
		return fmt.Errorf("recruitment: HandleOfferApprovalDecision: %w", err)
	}
	return nil
}

func (s *serviceImpl) SendOffer(ctx context.Context, orgID, ref string) (*Offer, error) {
	o, err := s.repo.FindOfferByRef(ctx, orgID, ref)
	if err != nil {
		return nil, fmt.Errorf("recruitment: SendOffer: %w", err)
	}
	if o == nil {
		return nil, ErrOfferNotFound
	}
	if o.Status != OfferStatusApproved {
		return nil, ErrOfferWrongStatus
	}
	if err := s.repo.UpdateOfferStatus(ctx, o.ID, OfferStatusSent); err != nil {
		return nil, fmt.Errorf("recruitment: SendOffer: %w", err)
	}
	o.Status = OfferStatusSent
	return o, nil
}

// AcceptOffer only records acceptance. It does NOT auto-move the
// application's pipeline stage or auto-trigger hire conversion — the
// recruiter still calls MoveApplication and HireApplication explicitly (the
// same "no implicit state machine" boundary as interview outcomes).
func (s *serviceImpl) AcceptOffer(ctx context.Context, orgID, ref string) (*Offer, error) {
	o, err := s.repo.FindOfferByRef(ctx, orgID, ref)
	if err != nil {
		return nil, fmt.Errorf("recruitment: AcceptOffer: %w", err)
	}
	if o == nil {
		return nil, ErrOfferNotFound
	}
	if o.Status != OfferStatusSent {
		return nil, ErrOfferWrongStatus
	}
	if err := s.repo.UpdateOfferStatus(ctx, o.ID, OfferStatusAccepted); err != nil {
		return nil, fmt.Errorf("recruitment: AcceptOffer: %w", err)
	}
	o.Status = OfferStatusAccepted
	return o, nil
}

func (s *serviceImpl) DeclineOffer(ctx context.Context, orgID, ref string) (*Offer, error) {
	o, err := s.repo.FindOfferByRef(ctx, orgID, ref)
	if err != nil {
		return nil, fmt.Errorf("recruitment: DeclineOffer: %w", err)
	}
	if o == nil {
		return nil, ErrOfferNotFound
	}
	if o.Status != OfferStatusSent {
		return nil, ErrOfferWrongStatus
	}
	if err := s.repo.UpdateOfferStatus(ctx, o.ID, OfferStatusDeclined); err != nil {
		return nil, fmt.Errorf("recruitment: DeclineOffer: %w", err)
	}
	o.Status = OfferStatusDeclined
	return o, nil
}

func (s *serviceImpl) RescindOffer(ctx context.Context, orgID, ref string) (*Offer, error) {
	o, err := s.repo.FindOfferByRef(ctx, orgID, ref)
	if err != nil {
		return nil, fmt.Errorf("recruitment: RescindOffer: %w", err)
	}
	if o == nil {
		return nil, ErrOfferNotFound
	}
	switch o.Status {
	case OfferStatusAccepted, OfferStatusDeclined, OfferStatusRescinded, OfferStatusExpired, OfferStatusRejected:
		return nil, ErrOfferWrongStatus
	}
	if err := s.repo.UpdateOfferStatus(ctx, o.ID, OfferStatusRescinded); err != nil {
		return nil, fmt.Errorf("recruitment: RescindOffer: %w", err)
	}
	o.Status = OfferStatusRescinded
	return o, nil
}
