// backend/internal/crm/deals/service_lead_conversion.go
//
// This file adds CreateDealFromLeadTx to the deals service so it satisfies
// the updated leads.DealCreator interface used by atomic lead conversion.
//
// HOW TO APPLY:
// 1. Add CreateDealFromLeadTx to the Service interface in service.go
// 2. Add CreateDealTx to the Repository interface in repository.go
// 3. Copy the implementations below into their respective files.
//
// The existing CreateDealFromLead (non-Tx) is kept for backwards compatibility
// but should no longer be called by lead conversion.
package deals

import (
	"context"

	"github.com/jackc/pgx/v5"
)

// ============================================================
// Add to Service interface in service.go:
// ============================================================
//
//   // CreateDealFromLeadTx implements leads.DealCreator using an existing tx.
//   CreateDealFromLeadTx(ctx context.Context, tx pgx.Tx, orgID, userID string, lead *leads.Lead, req leads.ConvertLeadRequest) (string, error)

// CreateDealFromLeadTx inserts a deal inside the supplied transaction.
// Called by leads.Service.ConvertLead so the deal, contact, and lead-status
// update are all committed or rolled back together.
// func (s *serviceImpl) CreateDealFromLeadTx(ctx context.Context, tx pgx.Tx, orgID, userID string, lead *leads.Lead, req leads.ConvertLeadRequest) (string, error) {
// 	title := lead.FirstName
// 	if req.DealTitle != nil && *req.DealTitle != "" {
// 		title = *req.DealTitle
// 	}
// 	value := 0.0
// 	if req.DealValue != nil {
// 		value = *req.DealValue
// 	}
// 	d := &Deal{
// 		OrgID:      orgID,
// 		Title:      title,
// 		Value:      value,
// 		Currency:   "USD",
// 		PipelineID: *req.PipelineID,
// 		StageID:    *req.StageID,
// 		Status:     DealStatusOpen,
// 		OwnerID:    lead.OwnerID,
// 		CreatedBy:  userID,
// 	}
// 	if err := s.repo.CreateDealTx(ctx, tx, d); err != nil {
// 		return "", fmt.Errorf("deals: CreateDealFromLeadTx: %w", err)
// 	}
// 	return d.ID, nil
// }

// ============================================================
// Add to Repository interface in repository.go:
// ============================================================
//
//   CreateDealTx(ctx context.Context, tx pgx.Tx, d *Deal) error

// CreateDealTx inserts a deal within an existing pgx.Tx.
// Add this method to the repoImpl in repository.go.
func createDealTxImpl(ctx context.Context, tx pgx.Tx, d *Deal) error {
	const q = `
		INSERT INTO crm_deals
		    (org_id, title, value, currency, pipeline_id, stage_id,
		     contact_id, company_id, status, close_date, owner_id, created_by)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12)
		RETURNING id, public_id, created_at, updated_at`

	return tx.QueryRow(ctx, q,
		d.OrgID, d.Title, d.Value, d.Currency, d.PipelineID, d.StageID,
		d.ContactID, d.CompanyID, d.Status, d.CloseDate, d.OwnerID, d.CreatedBy,
	).Scan(&d.ID, &d.PublicID, &d.CreatedAt, &d.UpdatedAt)
}
