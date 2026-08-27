// backend/internal/hrm/assets/service.go
package assets

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/shopspring/decimal"

	acks "github.com/mridha/businesssaas/internal/hrm/acknowledgements"
	"github.com/mridha/businesssaas/internal/hrm/approvals"
)

// HandoverAcknowledger is the one-method slice of hrm/acknowledgements.Service
// this package uses when an asset changes hands.
//
// It names acknowledgements' own types rather than mirroring them behind
// local structs, so acknowledgements.Service satisfies it structurally with
// no adapter in main.go — the corrected certifications.SkillGranter
// precedent. The import direction is safe: hrm/acknowledgements is a shared
// sign-off primitive already consumed by warnings, documents, announcements
// and calendar events, and it imports nothing from this package.
//
// ⚠ acks.TypeAssetHandover must exist in BOTH the AckType enum and the
// hrm_acknowledgements CHECK. Migration 00086 and 00094 each widened the
// CHECK without the enum, leaving 'appraisal' and 'course_completion'
// unreachable through Create()'s IsValid() gate; 8A fixes that drift and adds
// 'asset_handover' to both.
//
// A nil Acknowledger is valid and simply skips sign-off — the
// platform/checklists.ChecklistHook nil-hook precedent. Handover still
// records an assignment; only the signature request is skipped.
type HandoverAcknowledger interface {
	Create(ctx context.Context, orgID, requestedBy string, req acks.CreateAcknowledgementRequest) (*acks.Acknowledgement, error)
}

type Service interface {
	// Categories + licences — hrm.asset_config.
	ListCategories(ctx context.Context, orgID string) ([]*Category, error)
	CreateCategory(ctx context.Context, orgID, createdBy string, req CreateCategoryRequest) (*Category, error)
	ListLicenses(ctx context.Context, orgID string) ([]*License, error)
	GetLicense(ctx context.Context, orgID, ref string) (*License, error)
	CreateLicense(ctx context.Context, orgID, createdBy string, req CreateLicenseRequest) (*License, error)
	AssignSeat(ctx context.Context, orgID, licenseRef, assignedBy string, req AssignSeatRequest) (*SeatAssignment, error)
	ReleaseSeat(ctx context.Context, orgID, licenseRef, employeeID string) error
	ListSeats(ctx context.Context, orgID, licenseRef string) ([]*SeatAssignment, error)

	// Assets — hrm.assets.
	ListAssets(ctx context.Context, orgID string, filter ListFilter) (*AssetListResponse, error)
	GetAsset(ctx context.Context, orgID, ref string) (*Asset, error)
	CreateAsset(ctx context.Context, orgID, createdBy string, req CreateAssetRequest) (*Asset, error)

	AssignAsset(ctx context.Context, orgID, assetRef, assignedBy string, req AssignAssetRequest) (*Assignment, error)
	ReturnAsset(ctx context.Context, orgID, assetRef, returnedBy string, req ReturnAssetRequest) (*Assignment, error)
	ListAssignments(ctx context.Context, orgID string, filter ListFilter) (*AssignmentListResponse, error)

	AddMaintenance(ctx context.Context, orgID, assetRef, createdBy string, req CreateMaintenanceRequest) (*MaintenanceLog, error)
	ListMaintenance(ctx context.Context, orgID, assetRef string) ([]*MaintenanceLog, error)

	ListRequests(ctx context.Context, orgID string, filter ListFilter) (*RequestListResponse, error)
	GetRequest(ctx context.Context, orgID, ref string) (*AssetRequest, error)
	// RequestAsset is the self-service path — the route gates on
	// hrm.assets.request, which cannot express "for yourself only" (the
	// hrm.goals.set_own / benefits.EnrollSelf precedent), so the SERVICE
	// resolves the caller's own employeeID from userID.
	RequestAsset(ctx context.Context, orgID, userID string, req CreateAssetRequestRequest) (*AssetRequest, error)
	SubmitRequest(ctx context.Context, orgID, ref, submittedBy string) (*AssetRequest, error)
	FulfillRequest(ctx context.Context, orgID, ref, fulfilledBy string, req FulfillRequestRequest) (*AssetRequest, error)
	// HandleApprovalDecision is registered via
	// approvalsSvc.RegisterCallback("asset_request", ...) in main.go.
	HandleApprovalDecision(ctx context.Context, orgID, entityID string, approved bool) error
}

type serviceImpl struct {
	repo         Repository
	approvalsSvc approvals.Service
	acks         HandoverAcknowledger
}

func NewService(repo Repository, approvalsSvc approvals.Service, acks HandoverAcknowledger) Service {
	return &serviceImpl{repo: repo, approvalsSvc: approvalsSvc, acks: acks}
}

// ── Categories ───────────────────────────────────────────────────────────────

func (s *serviceImpl) ListCategories(ctx context.Context, orgID string) ([]*Category, error) {
	return s.repo.ListCategories(ctx, orgID)
}

func (s *serviceImpl) CreateCategory(ctx context.Context, orgID, createdBy string, req CreateCategoryRequest) (*Category, error) {
	name := strings.TrimSpace(req.Name)
	if name == "" {
		return nil, fmt.Errorf("assets: CreateCategory: name is required")
	}
	if req.UsefulLifeMonths != nil && *req.UsefulLifeMonths <= 0 {
		return nil, fmt.Errorf("assets: CreateCategory: useful_life_months must be positive when set")
	}
	requiresReturn := true
	if req.RequiresReturn != nil {
		requiresReturn = *req.RequiresReturn
	}
	c := &Category{
		OrgID: orgID, Name: name, Description: nilIfBlank(req.Description),
		RequiresReturn: requiresReturn, UsefulLifeMonths: req.UsefulLifeMonths,
		IsActive: true, CreatedBy: createdBy,
	}
	if err := s.repo.CreateCategory(ctx, c); err != nil {
		return nil, fmt.Errorf("assets: CreateCategory: %w", err)
	}
	return c, nil
}

// ── Licences + seats ─────────────────────────────────────────────────────────

func (s *serviceImpl) ListLicenses(ctx context.Context, orgID string) ([]*License, error) {
	list, err := s.repo.ListLicenses(ctx, orgID)
	if err != nil {
		return nil, err
	}
	// seats_used is DERIVED on every read — there is no counter column.
	for _, l := range list {
		if err := s.attachSeatsUsed(ctx, l); err != nil {
			return nil, err
		}
	}
	return list, nil
}

func (s *serviceImpl) GetLicense(ctx context.Context, orgID, ref string) (*License, error) {
	l, err := s.repo.FindLicenseByRef(ctx, orgID, ref)
	if err != nil {
		return nil, fmt.Errorf("assets: GetLicense: %w", err)
	}
	if l == nil {
		return nil, ErrLicenseNotFound
	}
	if err := s.attachSeatsUsed(ctx, l); err != nil {
		return nil, err
	}
	return l, nil
}

func (s *serviceImpl) attachSeatsUsed(ctx context.Context, l *License) error {
	n, err := s.repo.CountActiveSeats(ctx, l.ID)
	if err != nil {
		return fmt.Errorf("assets: seats used for licence %s: %w", l.ID, err)
	}
	l.SeatsUsed = &n
	return nil
}

func (s *serviceImpl) CreateLicense(ctx context.Context, orgID, createdBy string, req CreateLicenseRequest) (*License, error) {
	name := strings.TrimSpace(req.Name)
	if name == "" {
		return nil, fmt.Errorf("assets: CreateLicense: name is required")
	}
	if req.SeatsTotal <= 0 {
		return nil, ErrInvalidSeatsTotal
	}
	cost, err := parseMoneyOrZero(req.CostPerSeat)
	if err != nil {
		return nil, err
	}
	renewal, err := parseOptionalDate(req.RenewalDate)
	if err != nil {
		return nil, err
	}
	l := &License{
		OrgID: orgID, Name: name, Vendor: nilIfBlank(req.Vendor),
		SeatsTotal: req.SeatsTotal, CostPerSeat: cost,
		Currency: currencyOrDefault(req.Currency), RenewalDate: renewal,
		IsActive: true, CreatedBy: createdBy,
	}
	if err := s.repo.CreateLicense(ctx, l); err != nil {
		return nil, fmt.Errorf("assets: CreateLicense: %w", err)
	}
	zero := 0
	l.SeatsUsed = &zero
	return l, nil
}

// AssignSeat refuses to oversubscribe a licence. seats_used is counted live
// rather than read off a counter, so the check cannot go stale.
func (s *serviceImpl) AssignSeat(ctx context.Context, orgID, licenseRef, assignedBy string, req AssignSeatRequest) (*SeatAssignment, error) {
	l, err := s.repo.FindLicenseByRef(ctx, orgID, licenseRef)
	if err != nil {
		return nil, fmt.Errorf("assets: AssignSeat: %w", err)
	}
	if l == nil {
		return nil, ErrLicenseNotFound
	}
	employeeID := strings.TrimSpace(req.EmployeeID)
	if employeeID == "" {
		return nil, fmt.Errorf("assets: AssignSeat: employee_id is required")
	}

	used, err := s.repo.CountActiveSeats(ctx, l.ID)
	if err != nil {
		return nil, err
	}
	if used >= l.SeatsTotal {
		return nil, ErrNoSeatsLeft
	}

	seat := &SeatAssignment{OrgID: orgID, LicenseID: l.ID, EmployeeID: employeeID, AssignedBy: assignedBy}
	if err := s.repo.AssignSeat(ctx, seat); err != nil {
		return nil, err // ErrSeatAlreadyHeld surfaces unwrapped
	}
	return seat, nil
}

func (s *serviceImpl) ReleaseSeat(ctx context.Context, orgID, licenseRef, employeeID string) error {
	l, err := s.repo.FindLicenseByRef(ctx, orgID, licenseRef)
	if err != nil {
		return fmt.Errorf("assets: ReleaseSeat: %w", err)
	}
	if l == nil {
		return ErrLicenseNotFound
	}
	return s.repo.ReleaseSeat(ctx, orgID, l.ID, employeeID)
}

func (s *serviceImpl) ListSeats(ctx context.Context, orgID, licenseRef string) ([]*SeatAssignment, error) {
	l, err := s.repo.FindLicenseByRef(ctx, orgID, licenseRef)
	if err != nil {
		return nil, fmt.Errorf("assets: ListSeats: %w", err)
	}
	if l == nil {
		return nil, ErrLicenseNotFound
	}
	return s.repo.ListSeats(ctx, orgID, l.ID)
}

// ── Assets ───────────────────────────────────────────────────────────────────

func (s *serviceImpl) ListAssets(ctx context.Context, orgID string, filter ListFilter) (*AssetListResponse, error) {
	filter.Normalise()
	list, total, err := s.repo.ListAssets(ctx, orgID, filter)
	if err != nil {
		return nil, fmt.Errorf("assets: ListAssets: %w", err)
	}
	for _, a := range list {
		if err := s.attachBookValue(ctx, orgID, a); err != nil {
			return nil, err
		}
	}
	return &AssetListResponse{Assets: list, Total: total, Limit: filter.Limit, Offset: filter.Offset}, nil
}

func (s *serviceImpl) GetAsset(ctx context.Context, orgID, ref string) (*Asset, error) {
	a, err := s.repo.FindAssetByRef(ctx, orgID, ref)
	if err != nil {
		return nil, fmt.Errorf("assets: GetAsset: %w", err)
	}
	if a == nil {
		return nil, ErrAssetNotFound
	}
	if err := s.attachBookValue(ctx, orgID, a); err != nil {
		return nil, err
	}
	return a, nil
}

// attachBookValue computes the depreciation stub on every read — the 00076
// computed-not-stored rule. Useful life comes from the asset's CATEGORY; an
// asset with no category is not depreciated.
func (s *serviceImpl) attachBookValue(ctx context.Context, orgID string, a *Asset) error {
	var life *int
	if a.CategoryID != nil {
		cat, err := s.repo.FindCategoryByRef(ctx, orgID, *a.CategoryID)
		if err != nil {
			return fmt.Errorf("assets: book value for asset %s: %w", a.ID, err)
		}
		if cat != nil {
			life = cat.UsefulLifeMonths
		}
	}
	bv := BookValue(a.PurchaseCost, a.PurchaseDate, life, time.Now())
	a.BookValue = &bv
	return nil
}

func (s *serviceImpl) CreateAsset(ctx context.Context, orgID, createdBy string, req CreateAssetRequest) (*Asset, error) {
	name := strings.TrimSpace(req.Name)
	if name == "" {
		return nil, fmt.Errorf("assets: CreateAsset: name is required")
	}
	cost, err := parseMoneyOrZero(req.PurchaseCost)
	if err != nil {
		return nil, err
	}
	purchased, err := parseOptionalDate(req.PurchaseDate)
	if err != nil {
		return nil, err
	}
	if req.CategoryID != nil && strings.TrimSpace(*req.CategoryID) != "" {
		cat, err := s.repo.FindCategoryByRef(ctx, orgID, *req.CategoryID)
		if err != nil {
			return nil, fmt.Errorf("assets: CreateAsset: %w", err)
		}
		if cat == nil {
			return nil, ErrCategoryNotFound
		}
		req.CategoryID = &cat.ID
	} else {
		req.CategoryID = nil
	}

	a := &Asset{
		OrgID: orgID, CategoryID: req.CategoryID, Name: name,
		AssetTag: nilIfBlank(req.AssetTag), SerialNumber: nilIfBlank(req.SerialNumber),
		PurchaseDate: purchased, PurchaseCost: cost, Currency: currencyOrDefault(req.Currency),
		Status: AssetAvailable, Notes: nilIfBlank(req.Notes), CreatedBy: createdBy,
	}
	if err := s.repo.CreateAsset(ctx, a); err != nil {
		return nil, fmt.Errorf("assets: CreateAsset: %w", err)
	}
	if err := s.attachBookValue(ctx, orgID, a); err != nil {
		return nil, err
	}
	return a, nil
}

// ── Assignments ──────────────────────────────────────────────────────────────

// AssignAsset hands an asset to an employee. The DB's uq_hrm_asgn_active is
// the real guarantee that an asset has at most one holder — this pre-check
// only turns the race's loser into a clean ErrAlreadyAssigned instead of a
// raw constraint violation.
func (s *serviceImpl) AssignAsset(ctx context.Context, orgID, assetRef, assignedBy string, req AssignAssetRequest) (*Assignment, error) {
	a, err := s.repo.FindAssetByRef(ctx, orgID, assetRef)
	if err != nil {
		return nil, fmt.Errorf("assets: AssignAsset: %w", err)
	}
	if a == nil {
		return nil, ErrAssetNotFound
	}
	if a.Status == AssetRetired || a.Status == AssetLost {
		return nil, ErrWrongStatus
	}
	employeeID := strings.TrimSpace(req.EmployeeID)
	if employeeID == "" {
		return nil, fmt.Errorf("assets: AssignAsset: employee_id is required")
	}
	var condOut *Condition
	if req.ConditionOut != nil && strings.TrimSpace(*req.ConditionOut) != "" {
		c := Condition(strings.TrimSpace(*req.ConditionOut))
		if !c.IsValidOut() {
			return nil, ErrInvalidCondition
		}
		condOut = &c
	}

	asgn := &Assignment{
		OrgID: orgID, AssetID: a.ID, EmployeeID: employeeID,
		AssignedBy: assignedBy, ConditionOut: condOut, Notes: nilIfBlank(req.Notes),
	}
	if err := s.repo.CreateAssignment(ctx, asgn); err != nil {
		return nil, err // ErrAlreadyAssigned surfaces unwrapped
	}
	if err := s.repo.UpdateAssetStatus(ctx, orgID, a.ID, AssetAssigned); err != nil {
		return nil, fmt.Errorf("assets: AssignAsset: %w", err)
	}

	// Handover sign-off. A failure here must NOT unwind the assignment: the
	// asset really is in the employee's hands, and losing that record to keep
	// a signature request atomic would be the worse outcome. Logged by the
	// caller, not swallowed silently — see the Acknowledger doc comment.
	if s.acks != nil {
		if _, err := s.acks.Create(ctx, orgID, assignedBy, acks.CreateAcknowledgementRequest{
			EmployeeID:          employeeID,
			AcknowledgeableType: acks.TypeAssetHandover,
			AcknowledgeableID:   a.ID,
			EntityTitle:         a.Name,
			SignatureRequired:   true,
		}); err != nil {
			return asgn, fmt.Errorf("assets: AssignAsset: assignment recorded but handover sign-off failed: %w", err)
		}
	}
	return asgn, nil
}

func (s *serviceImpl) ReturnAsset(ctx context.Context, orgID, assetRef, returnedBy string, req ReturnAssetRequest) (*Assignment, error) {
	a, err := s.repo.FindAssetByRef(ctx, orgID, assetRef)
	if err != nil {
		return nil, fmt.Errorf("assets: ReturnAsset: %w", err)
	}
	if a == nil {
		return nil, ErrAssetNotFound
	}
	cur, err := s.repo.FindCurrentAssignment(ctx, a.ID)
	if err != nil {
		return nil, fmt.Errorf("assets: ReturnAsset: %w", err)
	}
	if cur == nil {
		return nil, ErrNotAssigned
	}
	var condIn *Condition
	if req.ConditionIn != nil && strings.TrimSpace(*req.ConditionIn) != "" {
		c := Condition(strings.TrimSpace(*req.ConditionIn))
		if !c.IsValidIn() {
			return nil, ErrInvalidCondition
		}
		condIn = &c
	}
	if err := s.repo.ReturnAssignment(ctx, cur.ID, returnedBy, condIn, nilIfBlank(req.Notes)); err != nil {
		return nil, err
	}
	now := time.Now()
	// A damaged return goes to maintenance, not straight back to the pool.
	next := AssetAvailable
	if condIn != nil && *condIn == ConditionDamaged {
		next = AssetInMaintenance
	}
	if err := s.repo.UpdateAssetStatus(ctx, orgID, a.ID, next); err != nil {
		return nil, fmt.Errorf("assets: ReturnAsset: %w", err)
	}
	// Re-read the row we just closed so the caller sees returned_at /
	// condition_in as persisted, not as we hoped to write them.
	cur.ReturnedAt = &now
	cur.ReturnedBy = &returnedBy
	cur.ConditionIn = condIn
	return cur, nil
}

func (s *serviceImpl) ListAssignments(ctx context.Context, orgID string, filter ListFilter) (*AssignmentListResponse, error) {
	filter.Normalise()
	list, total, err := s.repo.ListAssignments(ctx, orgID, filter)
	if err != nil {
		return nil, fmt.Errorf("assets: ListAssignments: %w", err)
	}
	return &AssignmentListResponse{Assignments: list, Total: total, Limit: filter.Limit, Offset: filter.Offset}, nil
}

// ── Maintenance ──────────────────────────────────────────────────────────────

func (s *serviceImpl) AddMaintenance(ctx context.Context, orgID, assetRef, createdBy string, req CreateMaintenanceRequest) (*MaintenanceLog, error) {
	a, err := s.repo.FindAssetByRef(ctx, orgID, assetRef)
	if err != nil {
		return nil, fmt.Errorf("assets: AddMaintenance: %w", err)
	}
	if a == nil {
		return nil, ErrAssetNotFound
	}
	mt := MaintenanceType(strings.TrimSpace(req.MaintenanceType))
	if !mt.IsValid() {
		return nil, ErrInvalidMaintenanceType
	}
	cost, err := parseMoneyOrZero(req.Cost)
	if err != nil {
		return nil, err
	}
	performed, err := parseRequiredDate(req.PerformedAt)
	if err != nil {
		return nil, err
	}

	m := &MaintenanceLog{
		OrgID: orgID, AssetID: a.ID, MaintenanceType: mt,
		Description: nilIfBlank(req.Description), Cost: cost,
		Currency: currencyOrDefault(req.Currency), PerformedAt: *performed,
		Vendor: nilIfBlank(req.Vendor), CreatedBy: createdBy,
	}
	if err := s.repo.CreateMaintenance(ctx, m); err != nil {
		return nil, fmt.Errorf("assets: AddMaintenance: %w", err)
	}
	return m, nil
}

func (s *serviceImpl) ListMaintenance(ctx context.Context, orgID, assetRef string) ([]*MaintenanceLog, error) {
	a, err := s.repo.FindAssetByRef(ctx, orgID, assetRef)
	if err != nil {
		return nil, fmt.Errorf("assets: ListMaintenance: %w", err)
	}
	if a == nil {
		return nil, ErrAssetNotFound
	}
	return s.repo.ListMaintenance(ctx, orgID, a.ID)
}

// ── Requests ─────────────────────────────────────────────────────────────────

func (s *serviceImpl) ListRequests(ctx context.Context, orgID string, filter ListFilter) (*RequestListResponse, error) {
	filter.Normalise()
	list, total, err := s.repo.ListRequests(ctx, orgID, filter)
	if err != nil {
		return nil, fmt.Errorf("assets: ListRequests: %w", err)
	}
	return &RequestListResponse{Requests: list, Total: total, Limit: filter.Limit, Offset: filter.Offset}, nil
}

func (s *serviceImpl) GetRequest(ctx context.Context, orgID, ref string) (*AssetRequest, error) {
	rq, err := s.repo.FindRequestByRef(ctx, orgID, ref)
	if err != nil {
		return nil, fmt.Errorf("assets: GetRequest: %w", err)
	}
	if rq == nil {
		return nil, ErrRequestNotFound
	}
	return rq, nil
}

func (s *serviceImpl) RequestAsset(ctx context.Context, orgID, userID string, req CreateAssetRequestRequest) (*AssetRequest, error) {
	employeeID, err := s.repo.FindEmployeeIDByUserID(ctx, orgID, userID)
	if err != nil {
		return nil, fmt.Errorf("assets: RequestAsset: resolve caller employee: %w", err)
	}
	if employeeID == "" {
		return nil, fmt.Errorf("assets: RequestAsset: caller has no employee record")
	}
	if req.CategoryID != nil && strings.TrimSpace(*req.CategoryID) != "" {
		cat, err := s.repo.FindCategoryByRef(ctx, orgID, *req.CategoryID)
		if err != nil {
			return nil, fmt.Errorf("assets: RequestAsset: %w", err)
		}
		if cat == nil {
			return nil, ErrCategoryNotFound
		}
		req.CategoryID = &cat.ID
	} else {
		req.CategoryID = nil
	}

	rq := &AssetRequest{
		OrgID: orgID, EmployeeID: employeeID, CategoryID: req.CategoryID,
		Justification: nilIfBlank(req.Justification), Status: RequestDraft, CreatedBy: userID,
	}
	if err := s.repo.CreateRequest(ctx, rq); err != nil {
		return nil, fmt.Errorf("assets: RequestAsset: %w", err)
	}
	return rq, nil
}

func (s *serviceImpl) SubmitRequest(ctx context.Context, orgID, ref, submittedBy string) (*AssetRequest, error) {
	rq, err := s.repo.FindRequestByRef(ctx, orgID, ref)
	if err != nil {
		return nil, fmt.Errorf("assets: SubmitRequest: %w", err)
	}
	if rq == nil {
		return nil, ErrRequestNotFound
	}
	if rq.Status != RequestDraft {
		return nil, ErrWrongStatus
	}

	tmpl, tErr := s.approvalsSvc.FindDefault(ctx, orgID, approvals.ActionTypeAssetRequest)
	if tErr == nil && tmpl != nil {
		inst, iErr := s.approvalsSvc.CreateInstance(ctx, orgID, approvals.CreateInstanceRequest{
			TemplateID: tmpl.ID, EntityType: "asset_request", EntityID: rq.ID, RequestedBy: submittedBy,
		})
		if iErr != nil {
			return nil, fmt.Errorf("assets: SubmitRequest: creating approval instance: %w", iErr)
		}
		rq.ApprovalInstanceID = &inst.ID
		rq.Status = RequestPendingApproval
		if err := s.repo.UpdateRequest(ctx, rq); err != nil {
			return nil, fmt.Errorf("assets: SubmitRequest: %w", err)
		}
		return rq, nil
	}

	// No approval template configured — unchanged fallback behavior, the
	// promotions / compensation / loans precedent.
	rq.Status = RequestApproved
	if err := s.repo.UpdateRequest(ctx, rq); err != nil {
		return nil, fmt.Errorf("assets: SubmitRequest: %w", err)
	}
	return rq, nil
}

func (s *serviceImpl) HandleApprovalDecision(ctx context.Context, orgID, entityID string, approved bool) error {
	rq, err := s.repo.FindRequestByRef(ctx, orgID, entityID)
	if err != nil {
		return fmt.Errorf("assets: HandleApprovalDecision: %w", err)
	}
	if rq == nil {
		return ErrRequestNotFound
	}
	if rq.Status != RequestPendingApproval {
		return nil
	}
	rq.Status = RequestApproved
	if !approved {
		rq.Status = RequestRejected
	}
	if err := s.repo.UpdateRequest(ctx, rq); err != nil {
		return fmt.Errorf("assets: HandleApprovalDecision: %w", err)
	}
	return nil
}

// FulfillRequest satisfies an approved request by assigning a real asset —
// a DISTINCT step from the approval decision, the promotions.Apply /
// compensation.ApplyCycle / loans.DisburseLoan precedent: a decision and the
// thing it authorizes are never the same call.
func (s *serviceImpl) FulfillRequest(ctx context.Context, orgID, ref, fulfilledBy string, req FulfillRequestRequest) (*AssetRequest, error) {
	rq, err := s.repo.FindRequestByRef(ctx, orgID, ref)
	if err != nil {
		return nil, fmt.Errorf("assets: FulfillRequest: %w", err)
	}
	if rq == nil {
		return nil, ErrRequestNotFound
	}
	if rq.Status != RequestApproved {
		return nil, ErrWrongStatus
	}

	if _, err := s.AssignAsset(ctx, orgID, req.AssetID, fulfilledBy, AssignAssetRequest{
		EmployeeID: rq.EmployeeID,
	}); err != nil {
		return nil, err // ErrAlreadyAssigned / ErrAssetNotFound surface unwrapped
	}

	asset, err := s.repo.FindAssetByRef(ctx, orgID, req.AssetID)
	if err != nil {
		return nil, fmt.Errorf("assets: FulfillRequest: %w", err)
	}
	now := time.Now()
	rq.Status = RequestFulfilled
	rq.FulfilledAssetID = &asset.ID
	rq.FulfilledAt = &now
	if err := s.repo.UpdateRequest(ctx, rq); err != nil {
		return nil, fmt.Errorf("assets: FulfillRequest: %w", err)
	}
	return rq, nil
}

// ── Shared helpers ───────────────────────────────────────────────────────────

const dateLayout = "2006-01-02"

func parseMoneyOrZero(v *string) (decimal.Decimal, error) {
	if v == nil || strings.TrimSpace(*v) == "" {
		return decimal.Zero, nil
	}
	d, err := decimal.NewFromString(strings.TrimSpace(*v))
	if err != nil || d.IsNegative() {
		return decimal.Zero, ErrInvalidAmount
	}
	return d, nil
}

func parseOptionalDate(v *string) (*time.Time, error) {
	if v == nil || strings.TrimSpace(*v) == "" {
		return nil, nil
	}
	d, err := time.Parse(dateLayout, strings.TrimSpace(*v))
	if err != nil {
		return nil, fmt.Errorf("assets: invalid date %q: %w", *v, err)
	}
	return &d, nil
}

func parseRequiredDate(v string) (*time.Time, error) {
	v = strings.TrimSpace(v)
	if v == "" {
		return nil, fmt.Errorf("assets: date is required")
	}
	d, err := time.Parse(dateLayout, v)
	if err != nil {
		return nil, fmt.Errorf("assets: invalid date %q: %w", v, err)
	}
	return &d, nil
}

func currencyOrDefault(v *string) string {
	if v != nil && strings.TrimSpace(*v) != "" {
		return strings.ToUpper(strings.TrimSpace(*v))
	}
	return "USD"
}

func nilIfBlank(s *string) *string {
	if s == nil || strings.TrimSpace(*s) == "" {
		return nil
	}
	trimmed := strings.TrimSpace(*s)
	return &trimmed
}
