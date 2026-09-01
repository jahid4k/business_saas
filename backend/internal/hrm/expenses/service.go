// backend/internal/hrm/expenses/service.go
package expenses

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/shopspring/decimal"

	"github.com/mridha/businesssaas/internal/hrm/approvals"
	"github.com/mridha/businesssaas/internal/hrm/reimbursements"
)

type Service interface {
	// SetRateSource attaches the FX layer and the base-currency resolver
	// (11B-1). Optional: without them a line uses the caller's rate,
	// defaulting to 1, exactly as before.
	SetRateSource(rates RateSource, base BaseCurrencySource)

	// Config — hrm.expense_config, untiered.
	ListPolicies(ctx context.Context, orgID string) ([]*Policy, error)
	CreatePolicy(ctx context.Context, orgID, createdBy string, req CreatePolicyRequest) (*Policy, error)
	ListPerDiemRates(ctx context.Context, orgID string) ([]*PerDiemRate, error)
	CreatePerDiemRate(ctx context.Context, orgID, createdBy string, req CreatePerDiemRateRequest) (*PerDiemRate, error)
	ListMileageRates(ctx context.Context, orgID string) ([]*MileageRate, error)
	CreateMileageRate(ctx context.Context, orgID, createdBy string, req CreateMileageRateRequest) (*MileageRate, error)

	// Travel — hrm.travel, scope-tiered.
	ListTravel(ctx context.Context, orgID string, filter ListFilter) (*TravelListResponse, error)
	GetTravel(ctx context.Context, orgID, ref string) (*TravelRequest, error)
	// CreateTravel is self-service — the SERVICE resolves the caller's own
	// employeeID, so hrm.travel.submit cannot file a trip for someone else.
	CreateTravel(ctx context.Context, orgID, userID string, req CreateTravelRequest) (*TravelRequest, error)
	SubmitTravel(ctx context.Context, orgID, ref, submittedBy string) (*TravelRequest, error)
	AddItineraryItem(ctx context.Context, orgID, travelRef string, req CreateItineraryItemRequest) (*ItineraryItem, error)
	ListItinerary(ctx context.Context, orgID, travelRef string) ([]*ItineraryItem, error)
	HandleTravelApprovalDecision(ctx context.Context, orgID, entityID string, approved bool) error

	// Advances — hrm.travel.
	ListAdvances(ctx context.Context, orgID string, filter ListFilter) (*AdvanceListResponse, error)
	CreateAdvance(ctx context.Context, orgID, createdBy string, req CreateAdvanceRequest) (*Advance, error)
	DisburseAdvance(ctx context.Context, orgID, ref, disbursedBy string) (*Advance, error)

	// OutstandingAdvancesForEmployee and RecoverAdvanceForSettlement satisfy
	// exits.AdvanceSettlementSource. Declared on the interface so main.go can
	// pass this service where that source is wanted — satisfaction is
	// structural, so both must be visible here and not only on serviceImpl.
	OutstandingAdvancesForEmployee(ctx context.Context, orgID, employeeID string) ([]*Advance, error)
	RecoverAdvanceForSettlement(ctx context.Context, orgID, advanceID string, amount decimal.Decimal) error

	// Claims — hrm.expenses, scope-tiered.
	ListClaims(ctx context.Context, orgID string, filter ListFilter) (*ClaimListResponse, error)
	// GetClaim hydrates the DERIVED totals and the per-line violations.
	GetClaim(ctx context.Context, orgID, ref string) (*Claim, error)
	CreateClaim(ctx context.Context, orgID, userID string, req CreateClaimRequest) (*Claim, error)
	// AddLine converts to base currency and records any policy breach as a
	// WARNING — it never refuses the line.
	AddLine(ctx context.Context, orgID, claimRef string, req CreateLineRequest) (*Line, error)
	SubmitClaim(ctx context.Context, orgID, ref, submittedBy string) (*Claim, error)
	// ApproveLine is the LINE-LEVEL approval this module exists for.
	ApproveLine(ctx context.Context, orgID, claimRef, lineRef string, req ApproveLineRequest) (*Claim, error)
	HandleClaimApprovalDecision(ctx context.Context, orgID, entityID string, approved bool) error
	// SettleClaim divides the approved total against any advance and creates
	// the reimbursement payroll will actually pay. The 7C boundary.
	SettleClaim(ctx context.Context, orgID, ref, settledBy string) (*SettlementResult, error)
}

type serviceImpl struct {
	repo           Repository
	approvalsSvc   approvals.Service
	reimbursements ReimbursementCreator
	// rates and baseCurrency are OPTIONAL (11B-1). Both nil means this
	// package behaves exactly as it did before the FX table existed: the
	// caller's rate is used, defaulting to 1. Every deployment path that has
	// not wired them keeps working.
	rates        RateSource
	baseCurrency BaseCurrencySource
}

func NewService(repo Repository, approvalsSvc approvals.Service, reimbursementCreator ReimbursementCreator) Service {
	return &serviceImpl{repo: repo, approvalsSvc: approvalsSvc, reimbursements: reimbursementCreator}
}

// WithRateSource attaches the FX layer and the base-currency resolver.
//
// Separate from NewService so the six existing construction sites keep
// compiling unchanged, the SetBonusSource shape used throughout payslips.
func (s *serviceImpl) WithRateSource(rates RateSource, base BaseCurrencySource) {
	s.rates, s.baseCurrency = rates, base
}

// SetRateSource is the Service-level door to the same thing.
func (s *serviceImpl) SetRateSource(rates RateSource, base BaseCurrencySource) {
	s.WithRateSource(rates, base)
}

// ── Config ───────────────────────────────────────────────────────────────────

func (s *serviceImpl) ListPolicies(ctx context.Context, orgID string) ([]*Policy, error) {
	return s.repo.ListPolicies(ctx, orgID)
}

func (s *serviceImpl) CreatePolicy(ctx context.Context, orgID, createdBy string, req CreatePolicyRequest) (*Policy, error) {
	cat := LineCategory(strings.TrimSpace(req.Category))
	if !cat.IsValid() {
		return nil, ErrInvalidCategory
	}
	max, err := parseMoney(req.MaxAmount)
	if err != nil {
		return nil, err
	}
	eff, err := parseRequiredDate(req.EffectiveDate)
	if err != nil {
		return nil, err
	}
	p := &Policy{
		OrgID: orgID, Category: cat, MaxAmount: max,
		Currency: currencyOrDefault(req.Currency), EffectiveDate: *eff, CreatedBy: createdBy,
	}
	if err := s.repo.CreatePolicy(ctx, p); err != nil {
		return nil, fmt.Errorf("expenses: CreatePolicy: %w", err)
	}
	return p, nil
}

func (s *serviceImpl) ListPerDiemRates(ctx context.Context, orgID string) ([]*PerDiemRate, error) {
	return s.repo.ListPerDiemRates(ctx, orgID)
}

func (s *serviceImpl) CreatePerDiemRate(ctx context.Context, orgID, createdBy string, req CreatePerDiemRateRequest) (*PerDiemRate, error) {
	amt, err := parseMoney(req.DailyAmount)
	if err != nil {
		return nil, err
	}
	eff, err := parseRequiredDate(req.EffectiveDate)
	if err != nil {
		return nil, err
	}
	var country *string
	if req.CountryCode != nil && strings.TrimSpace(*req.CountryCode) != "" {
		c := strings.ToUpper(strings.TrimSpace(*req.CountryCode))
		if len(c) != 2 {
			return nil, fmt.Errorf("expenses: CreatePerDiemRate: country_code must be a 2-letter ISO code")
		}
		country = &c
	}
	r := &PerDiemRate{
		OrgID: orgID, CountryCode: country, DailyAmount: amt,
		Currency: currencyOrDefault(req.Currency), EffectiveDate: *eff, CreatedBy: createdBy,
	}
	if err := s.repo.CreatePerDiemRate(ctx, r); err != nil {
		return nil, fmt.Errorf("expenses: CreatePerDiemRate: %w", err)
	}
	return r, nil
}

func (s *serviceImpl) ListMileageRates(ctx context.Context, orgID string) ([]*MileageRate, error) {
	return s.repo.ListMileageRates(ctx, orgID)
}

func (s *serviceImpl) CreateMileageRate(ctx context.Context, orgID, createdBy string, req CreateMileageRateRequest) (*MileageRate, error) {
	rate, err := parseMoney(req.RatePerUnit)
	if err != nil {
		return nil, err
	}
	eff, err := parseRequiredDate(req.EffectiveDate)
	if err != nil {
		return nil, err
	}
	unit := "km"
	if req.Unit != nil && strings.TrimSpace(*req.Unit) != "" {
		unit = strings.ToLower(strings.TrimSpace(*req.Unit))
		if unit != "km" && unit != "mile" {
			return nil, fmt.Errorf("expenses: CreateMileageRate: unit must be km or mile")
		}
	}
	m := &MileageRate{
		OrgID: orgID, RatePerUnit: rate, Unit: unit,
		Currency: currencyOrDefault(req.Currency), EffectiveDate: *eff, CreatedBy: createdBy,
	}
	if err := s.repo.CreateMileageRate(ctx, m); err != nil {
		return nil, fmt.Errorf("expenses: CreateMileageRate: %w", err)
	}
	return m, nil
}

// ── Travel ───────────────────────────────────────────────────────────────────

func (s *serviceImpl) ListTravel(ctx context.Context, orgID string, filter ListFilter) (*TravelListResponse, error) {
	filter.Normalise()
	list, total, err := s.repo.ListTravel(ctx, orgID, filter)
	if err != nil {
		return nil, fmt.Errorf("expenses: ListTravel: %w", err)
	}
	return &TravelListResponse{Requests: list, Total: total, Limit: filter.Limit, Offset: filter.Offset}, nil
}

func (s *serviceImpl) GetTravel(ctx context.Context, orgID, ref string) (*TravelRequest, error) {
	t, err := s.repo.FindTravelByRef(ctx, orgID, ref)
	if err != nil {
		return nil, fmt.Errorf("expenses: GetTravel: %w", err)
	}
	if t == nil {
		return nil, ErrTravelNotFound
	}
	return t, nil
}

func (s *serviceImpl) CreateTravel(ctx context.Context, orgID, userID string, req CreateTravelRequest) (*TravelRequest, error) {
	employeeID, err := s.repo.FindEmployeeIDByUserID(ctx, orgID, userID)
	if err != nil {
		return nil, fmt.Errorf("expenses: CreateTravel: resolve caller employee: %w", err)
	}
	if employeeID == "" {
		return nil, fmt.Errorf("expenses: CreateTravel: caller has no employee record")
	}
	purpose := strings.TrimSpace(req.Purpose)
	dest := strings.TrimSpace(req.Destination)
	if purpose == "" || dest == "" {
		return nil, fmt.Errorf("expenses: CreateTravel: purpose and destination are required")
	}
	start, err := parseRequiredDate(req.StartDate)
	if err != nil {
		return nil, err
	}
	end, err := parseRequiredDate(req.EndDate)
	if err != nil {
		return nil, err
	}
	if end.Before(*start) {
		return nil, ErrInvalidDateRange
	}
	var country *string
	if req.DestinationCountry != nil && strings.TrimSpace(*req.DestinationCountry) != "" {
		c := strings.ToUpper(strings.TrimSpace(*req.DestinationCountry))
		country = &c
	}

	t := &TravelRequest{
		OrgID: orgID, EmployeeID: employeeID, Purpose: purpose, Destination: dest,
		DestinationCountry: country, StartDate: *start, EndDate: *end,
		Status: TravelDraft, Currency: currencyOrDefault(req.Currency), CreatedBy: userID,
	}
	if err := s.repo.CreateTravel(ctx, t); err != nil {
		return nil, fmt.Errorf("expenses: CreateTravel: %w", err)
	}
	return t, nil
}

func (s *serviceImpl) SubmitTravel(ctx context.Context, orgID, ref, submittedBy string) (*TravelRequest, error) {
	t, err := s.repo.FindTravelByRef(ctx, orgID, ref)
	if err != nil {
		return nil, fmt.Errorf("expenses: SubmitTravel: %w", err)
	}
	if t == nil {
		return nil, ErrTravelNotFound
	}
	if t.Status != TravelDraft {
		return nil, ErrWrongStatus
	}

	tmpl, tErr := s.approvalsSvc.FindDefault(ctx, orgID, approvals.ActionTypeTravelRequest)
	if tErr == nil && tmpl != nil {
		inst, iErr := s.approvalsSvc.CreateInstance(ctx, orgID, approvals.CreateInstanceRequest{
			TemplateID: tmpl.ID, EntityType: "travel_request", EntityID: t.ID, RequestedBy: submittedBy,
		})
		if iErr != nil {
			return nil, fmt.Errorf("expenses: SubmitTravel: creating approval instance: %w", iErr)
		}
		t.ApprovalInstanceID = &inst.ID
		t.Status = TravelPendingApproval
		if err := s.repo.UpdateTravel(ctx, t); err != nil {
			return nil, fmt.Errorf("expenses: SubmitTravel: %w", err)
		}
		return t, nil
	}

	// No approval template configured — unchanged fallback behavior.
	t.Status = TravelApproved
	if err := s.repo.UpdateTravel(ctx, t); err != nil {
		return nil, fmt.Errorf("expenses: SubmitTravel: %w", err)
	}
	return t, nil
}

func (s *serviceImpl) HandleTravelApprovalDecision(ctx context.Context, orgID, entityID string, approved bool) error {
	t, err := s.repo.FindTravelByRef(ctx, orgID, entityID)
	if err != nil {
		return fmt.Errorf("expenses: HandleTravelApprovalDecision: %w", err)
	}
	if t == nil {
		return ErrTravelNotFound
	}
	if t.Status != TravelPendingApproval {
		return nil
	}
	t.Status = TravelApproved
	if !approved {
		t.Status = TravelRejected
	}
	if err := s.repo.UpdateTravel(ctx, t); err != nil {
		return fmt.Errorf("expenses: HandleTravelApprovalDecision: %w", err)
	}
	return nil
}

func (s *serviceImpl) AddItineraryItem(ctx context.Context, orgID, travelRef string, req CreateItineraryItemRequest) (*ItineraryItem, error) {
	t, err := s.repo.FindTravelByRef(ctx, orgID, travelRef)
	if err != nil {
		return nil, fmt.Errorf("expenses: AddItineraryItem: %w", err)
	}
	if t == nil {
		return nil, ErrTravelNotFound
	}
	itemType := ItineraryItemType(strings.TrimSpace(req.ItemType))
	if !itemType.IsValid() {
		return nil, ErrInvalidItemType
	}
	cost, err := parseMoneyOrZero(req.EstimatedCost)
	if err != nil {
		return nil, err
	}
	starts, err := parseOptionalTimestamp(req.StartsAt)
	if err != nil {
		return nil, err
	}
	ends, err := parseOptionalTimestamp(req.EndsAt)
	if err != nil {
		return nil, err
	}
	if starts != nil && ends != nil && ends.Before(*starts) {
		return nil, ErrInvalidDateRange
	}
	order := 0
	if req.DisplayOrder != nil {
		order = *req.DisplayOrder
	}

	i := &ItineraryItem{
		TravelRequestID: t.ID, ItemType: itemType, Description: nilIfBlank(req.Description),
		FromLocation: nilIfBlank(req.FromLocation), ToLocation: nilIfBlank(req.ToLocation),
		StartsAt: starts, EndsAt: ends, BookingReference: nilIfBlank(req.BookingReference),
		EstimatedCost: cost, Currency: currencyOrDefault(req.Currency), DisplayOrder: order,
	}
	if err := s.repo.CreateItineraryItem(ctx, i); err != nil {
		return nil, fmt.Errorf("expenses: AddItineraryItem: %w", err)
	}
	return i, nil
}

func (s *serviceImpl) ListItinerary(ctx context.Context, orgID, travelRef string) ([]*ItineraryItem, error) {
	t, err := s.repo.FindTravelByRef(ctx, orgID, travelRef)
	if err != nil {
		return nil, fmt.Errorf("expenses: ListItinerary: %w", err)
	}
	if t == nil {
		return nil, ErrTravelNotFound
	}
	return s.repo.ListItinerary(ctx, t.ID)
}

// ── Advances ─────────────────────────────────────────────────────────────────

func (s *serviceImpl) ListAdvances(ctx context.Context, orgID string, filter ListFilter) (*AdvanceListResponse, error) {
	filter.Normalise()
	list, total, err := s.repo.ListAdvances(ctx, orgID, filter)
	if err != nil {
		return nil, fmt.Errorf("expenses: ListAdvances: %w", err)
	}
	return &AdvanceListResponse{Advances: list, Total: total, Limit: filter.Limit, Offset: filter.Offset}, nil
}

func (s *serviceImpl) CreateAdvance(ctx context.Context, orgID, createdBy string, req CreateAdvanceRequest) (*Advance, error) {
	amount, err := parseMoney(req.Amount)
	if err != nil {
		return nil, err
	}
	if !amount.IsPositive() {
		return nil, ErrInvalidAmount
	}
	employeeID := strings.TrimSpace(req.EmployeeID)
	if employeeID == "" {
		return nil, fmt.Errorf("expenses: CreateAdvance: employee_id is required")
	}
	var travelID *string
	if req.TravelRequestID != nil && strings.TrimSpace(*req.TravelRequestID) != "" {
		t, err := s.repo.FindTravelByRef(ctx, orgID, *req.TravelRequestID)
		if err != nil {
			return nil, fmt.Errorf("expenses: CreateAdvance: %w", err)
		}
		if t == nil {
			return nil, ErrTravelNotFound
		}
		travelID = &t.ID
	}

	a := &Advance{
		OrgID: orgID, EmployeeID: employeeID, TravelRequestID: travelID,
		Amount: amount, Currency: currencyOrDefault(req.Currency),
		SettledAmount: decimal.Zero, Status: AdvancePending, CreatedBy: createdBy,
	}
	if err := s.repo.CreateAdvance(ctx, a); err != nil {
		return nil, fmt.Errorf("expenses: CreateAdvance: %w", err)
	}
	return a, nil
}

// DisburseAdvance releases the funds — a distinct step from recording that an
// advance will be given, the hrm.loans.disburse precedent.
func (s *serviceImpl) DisburseAdvance(ctx context.Context, orgID, ref, disbursedBy string) (*Advance, error) {
	a, err := s.repo.FindAdvanceByRef(ctx, orgID, ref)
	if err != nil {
		return nil, fmt.Errorf("expenses: DisburseAdvance: %w", err)
	}
	if a == nil {
		return nil, ErrAdvanceNotFound
	}
	if a.Status != AdvancePending {
		return nil, ErrWrongStatus
	}
	now := time.Now()
	a.Status = AdvanceDisbursed
	a.DisbursedAt = &now
	a.DisbursedBy = &disbursedBy
	if err := s.repo.UpdateAdvance(ctx, a); err != nil {
		return nil, fmt.Errorf("expenses: DisburseAdvance: %w", err)
	}
	return a, nil
}

// ── Claims ───────────────────────────────────────────────────────────────────

func (s *serviceImpl) ListClaims(ctx context.Context, orgID string, filter ListFilter) (*ClaimListResponse, error) {
	filter.Normalise()
	list, total, err := s.repo.ListClaims(ctx, orgID, filter)
	if err != nil {
		return nil, fmt.Errorf("expenses: ListClaims: %w", err)
	}
	for _, c := range list {
		if err := s.hydrateTotals(ctx, c, false); err != nil {
			return nil, err
		}
	}
	return &ClaimListResponse{Claims: list, Total: total, Limit: filter.Limit, Offset: filter.Offset}, nil
}

func (s *serviceImpl) GetClaim(ctx context.Context, orgID, ref string) (*Claim, error) {
	c, err := s.repo.FindClaimByRef(ctx, orgID, ref)
	if err != nil {
		return nil, fmt.Errorf("expenses: GetClaim: %w", err)
	}
	if c == nil {
		return nil, ErrClaimNotFound
	}
	if err := s.hydrateTotals(ctx, c, true); err != nil {
		return nil, err
	}
	return c, nil
}

// hydrateTotals computes the claim's totals from its lines on every read —
// there are no stored total columns. withLines also attaches the lines
// themselves and their policy warnings.
func (s *serviceImpl) hydrateTotals(ctx context.Context, c *Claim, withLines bool) error {
	lines, err := s.repo.ListLines(ctx, c.ID)
	if err != nil {
		return fmt.Errorf("expenses: totals for claim %s: %w", c.ID, err)
	}
	bases := make([]decimal.Decimal, len(lines))
	approved := make([]*decimal.Decimal, len(lines))
	for i, l := range lines {
		bases[i] = l.BaseAmount
		approved[i] = l.ApprovedAmount
	}
	t := SumClaim(bases, approved)
	c.TotalClaimed = &t.Claimed
	c.TotalApproved = &t.Approved
	c.UndecidedLines = &t.Undecided

	if withLines {
		violations, err := s.repo.ListViolationsByClaim(ctx, c.ID)
		if err != nil {
			return fmt.Errorf("expenses: violations for claim %s: %w", c.ID, err)
		}
		for _, l := range lines {
			l.Violations = violations[l.ID]
		}
		c.Lines = lines
	}
	return nil
}

func (s *serviceImpl) CreateClaim(ctx context.Context, orgID, userID string, req CreateClaimRequest) (*Claim, error) {
	employeeID, err := s.repo.FindEmployeeIDByUserID(ctx, orgID, userID)
	if err != nil {
		return nil, fmt.Errorf("expenses: CreateClaim: resolve caller employee: %w", err)
	}
	if employeeID == "" {
		return nil, fmt.Errorf("expenses: CreateClaim: caller has no employee record")
	}
	title := strings.TrimSpace(req.Title)
	if title == "" {
		return nil, fmt.Errorf("expenses: CreateClaim: title is required")
	}

	var travelID, advanceID *string
	if req.TravelRequestID != nil && strings.TrimSpace(*req.TravelRequestID) != "" {
		t, err := s.repo.FindTravelByRef(ctx, orgID, *req.TravelRequestID)
		if err != nil {
			return nil, fmt.Errorf("expenses: CreateClaim: %w", err)
		}
		if t == nil {
			return nil, ErrTravelNotFound
		}
		travelID = &t.ID
	}
	if req.AdvanceID != nil && strings.TrimSpace(*req.AdvanceID) != "" {
		a, err := s.repo.FindAdvanceByRef(ctx, orgID, *req.AdvanceID)
		if err != nil {
			return nil, fmt.Errorf("expenses: CreateClaim: %w", err)
		}
		if a == nil {
			return nil, ErrAdvanceNotFound
		}
		advanceID = &a.ID
	}

	c := &Claim{
		OrgID: orgID, EmployeeID: employeeID, TravelRequestID: travelID, AdvanceID: advanceID,
		Title: title, Description: nilIfBlank(req.Description),
		BaseCurrency: currencyOrDefault(req.BaseCurrency), Status: ClaimDraft, CreatedBy: userID,
	}
	if err := s.repo.CreateClaim(ctx, c); err != nil {
		return nil, fmt.Errorf("expenses: CreateClaim: %w", err)
	}
	zero := decimal.Zero
	none := 0
	c.TotalClaimed, c.TotalApproved, c.UndecidedLines = &zero, &zero, &none
	return c, nil
}

// AddLine converts the amount to base currency using a rate snapshotted NOW,
// then records any policy breach as a WARNING. It never refuses a line for
// being over policy — the build plan is explicit that violations are
// warnings, not hard blocks, and an unclaimable over-cap taxi fare helps
// nobody.
func (s *serviceImpl) AddLine(ctx context.Context, orgID, claimRef string, req CreateLineRequest) (*Line, error) {
	c, err := s.repo.FindClaimByRef(ctx, orgID, claimRef)
	if err != nil {
		return nil, fmt.Errorf("expenses: AddLine: %w", err)
	}
	if c == nil {
		return nil, ErrClaimNotFound
	}
	if c.Status != ClaimDraft {
		return nil, ErrWrongStatus
	}
	cat := LineCategory(strings.TrimSpace(req.Category))
	if !cat.IsValid() {
		return nil, ErrInvalidCategory
	}
	spent, err := parseRequiredDate(req.ExpenseDate)
	if err != nil {
		return nil, err
	}
	// ⚠ A CALLER-SUPPLIED RATE STILL WINS. An organization with a
	// contractual or corporate-card rate must be able to state it, and
	// overriding that with a table lookup would silently reprice their claim.
	// The lookup is the FALLBACK, which is the change 11B-1 makes: before it,
	// the caller's number was the only path and its absence meant 1.
	rate := decimal.NewFromInt(1)
	var rateDate *time.Time
	callerSuppliedRate := false
	if req.ExchangeRate != nil && strings.TrimSpace(*req.ExchangeRate) != "" {
		r, err := decimal.NewFromString(strings.TrimSpace(*req.ExchangeRate))
		if err != nil || !r.IsPositive() {
			return nil, ErrInvalidExchangeRate
		}
		rate = r
		callerSuppliedRate = true
	}

	amount, err := parseMoneyOrZero(req.Amount)
	if err != nil {
		return nil, err
	}
	line := &Line{
		ClaimID: c.ID, Category: cat, Description: nilIfBlank(req.Description),
		ExpenseDate: *spent, Currency: currencyOrDefault(req.Currency),
		ExchangeRate: rate, ReceiptURL: nilIfBlank(req.ReceiptURL),
	}
	if req.DisplayOrder != nil {
		line.DisplayOrder = *req.DisplayOrder
	}

	// A mileage line derives its own amount from the rate in force on the
	// expense date — effective-dated, so a rate raised later cannot change it.
	if cat == CategoryMileage && req.MileageDistance != nil && strings.TrimSpace(*req.MileageDistance) != "" {
		dist, err := decimal.NewFromString(strings.TrimSpace(*req.MileageDistance))
		if err != nil || dist.IsNegative() {
			return nil, ErrInvalidAmount
		}
		mr, err := s.repo.FindMileageRateAsOf(ctx, orgID, *spent)
		if err != nil {
			return nil, fmt.Errorf("expenses: AddLine: %w", err)
		}
		if mr != nil {
			line.MileageDistance = &dist
			line.MileageRateID = &mr.ID
			amount = MileageAmount(dist, mr.RatePerUnit)
			line.Currency = mr.Currency
		}
	}

	// The rate lookup happens HERE, after the mileage branch above, because
	// a mileage line takes its currency from the mileage rate rather than
	// from the request — looking up earlier would price it against the wrong
	// pair.
	if !callerSuppliedRate {
		resolvedRate, resolvedDate, err := s.lookupRate(ctx, orgID, line.Currency, *spent)
		if err != nil {
			return nil, err
		}
		if resolvedDate != nil {
			rate, rateDate = resolvedRate, resolvedDate
		}
	}
	line.ExchangeRate = rate
	line.ExchangeRateDate = rateDate

	line.Amount = amount
	line.BaseAmount = ConvertToBase(amount, rate)

	if err := s.repo.CreateLine(ctx, line); err != nil {
		return nil, fmt.Errorf("expenses: AddLine: %w", err)
	}

	// Policy check — AFTER the line is safely persisted, because a breach
	// must never cost the employee their line.
	if err := s.recordPolicyViolation(ctx, orgID, line, *spent); err != nil {
		return nil, err
	}
	return line, nil
}

// recordPolicyViolation writes a warning row when a line exceeds the cap in
// force on its expense date. Snapshots the cap so the warning still reads
// correctly after the policy is re-priced or deleted.
func (s *serviceImpl) recordPolicyViolation(ctx context.Context, orgID string, line *Line, asOf time.Time) error {
	policy, err := s.repo.FindPolicyAsOf(ctx, orgID, line.Category, asOf)
	if err != nil {
		return fmt.Errorf("expenses: policy check: %w", err)
	}
	if policy == nil || !line.BaseAmount.GreaterThan(policy.MaxAmount) {
		return nil
	}
	v := &PolicyViolation{
		LineID: line.ID, PolicyID: &policy.ID, Category: string(line.Category),
		MaxAmount: policy.MaxAmount, ActualAmount: line.BaseAmount,
		Message: fmt.Sprintf("%s of %s exceeds the %s policy cap of %s",
			line.Category, line.BaseAmount, policy.Currency, policy.MaxAmount),
	}
	if err := s.repo.CreateViolation(ctx, v); err != nil {
		return fmt.Errorf("expenses: record violation: %w", err)
	}
	line.Violations = append(line.Violations, v)
	return nil
}

func (s *serviceImpl) SubmitClaim(ctx context.Context, orgID, ref, submittedBy string) (*Claim, error) {
	c, err := s.repo.FindClaimByRef(ctx, orgID, ref)
	if err != nil {
		return nil, fmt.Errorf("expenses: SubmitClaim: %w", err)
	}
	if c == nil {
		return nil, ErrClaimNotFound
	}
	if c.Status != ClaimDraft {
		return nil, ErrWrongStatus
	}
	lines, err := s.repo.ListLines(ctx, c.ID)
	if err != nil {
		return nil, fmt.Errorf("expenses: SubmitClaim: %w", err)
	}
	if len(lines) == 0 {
		return nil, ErrClaimHasNoLines
	}

	now := time.Now()
	tmpl, tErr := s.approvalsSvc.FindDefault(ctx, orgID, approvals.ActionTypeExpenseClaim)
	if tErr == nil && tmpl != nil {
		inst, iErr := s.approvalsSvc.CreateInstance(ctx, orgID, approvals.CreateInstanceRequest{
			TemplateID: tmpl.ID, EntityType: "expense_claim", EntityID: c.ID, RequestedBy: submittedBy,
		})
		if iErr != nil {
			return nil, fmt.Errorf("expenses: SubmitClaim: creating approval instance: %w", iErr)
		}
		c.ApprovalInstanceID = &inst.ID
		c.Status = ClaimPendingApproval
		c.SubmittedAt = &now
		if err := s.repo.UpdateClaim(ctx, c); err != nil {
			return nil, fmt.Errorf("expenses: SubmitClaim: %w", err)
		}
		return s.GetClaim(ctx, orgID, c.ID)
	}

	// No approval template configured — the claim still needs its lines
	// decided before it can settle, so it lands pending, not approved.
	c.Status = ClaimPendingApproval
	c.SubmittedAt = &now
	if err := s.repo.UpdateClaim(ctx, c); err != nil {
		return nil, fmt.Errorf("expenses: SubmitClaim: %w", err)
	}
	return s.GetClaim(ctx, orgID, c.ID)
}

// ApproveLine sets ONE line's approved amount. This is the line-level
// approval the whole module is shaped around: the claim's totals move
// because its lines did, never independently.
func (s *serviceImpl) ApproveLine(ctx context.Context, orgID, claimRef, lineRef string, req ApproveLineRequest) (*Claim, error) {
	c, err := s.repo.FindClaimByRef(ctx, orgID, claimRef)
	if err != nil {
		return nil, fmt.Errorf("expenses: ApproveLine: %w", err)
	}
	if c == nil {
		return nil, ErrClaimNotFound
	}
	// Revisable until the money actually moves. Locking at 'approved' would
	// leave an approver who mistyped the LAST line with no remedy at all —
	// deciding the final line flips the claim to approved, which would then
	// refuse every correction. Only 'paid' is final, because only then has a
	// reimbursement been created and handed to payroll.
	if c.Status != ClaimPendingApproval && c.Status != ClaimPartiallyApproved && c.Status != ClaimApproved {
		return nil, ErrWrongStatus
	}
	line, err := s.repo.FindLineByRef(ctx, lineRef)
	if err != nil {
		return nil, fmt.Errorf("expenses: ApproveLine: %w", err)
	}
	if line == nil || line.ClaimID != c.ID {
		return nil, ErrLineNotFound
	}

	approved, err := parseMoney(req.ApprovedAmount)
	if err != nil {
		return nil, err
	}
	// An approver may REDUCE a line, never inflate it past what was spent.
	// The CHECK enforces it too; this is the friendly message.
	if approved.GreaterThan(line.BaseAmount) {
		return nil, ErrApprovedExceedsClaimed
	}
	if err := s.repo.SetLineApprovedAmount(ctx, line.ID, approved); err != nil {
		return nil, err
	}

	// The claim's status follows from its lines: fully decided means
	// approved, anything left undecided means partially approved.
	if err := s.hydrateTotals(ctx, c, false); err != nil {
		return nil, err
	}
	next := ClaimApproved
	if c.UndecidedLines != nil && *c.UndecidedLines > 0 {
		next = ClaimPartiallyApproved
	}
	if c.Status != next {
		c.Status = next
		if err := s.repo.UpdateClaim(ctx, c); err != nil {
			return nil, fmt.Errorf("expenses: ApproveLine: %w", err)
		}
	}
	return s.GetClaim(ctx, orgID, c.ID)
}

func (s *serviceImpl) HandleClaimApprovalDecision(ctx context.Context, orgID, entityID string, approved bool) error {
	c, err := s.repo.FindClaimByRef(ctx, orgID, entityID)
	if err != nil {
		return fmt.Errorf("expenses: HandleClaimApprovalDecision: %w", err)
	}
	if c == nil {
		return ErrClaimNotFound
	}
	if c.Status != ClaimPendingApproval && c.Status != ClaimPartiallyApproved {
		return nil
	}
	now := time.Now()
	c.DecidedAt = &now
	if !approved {
		c.Status = ClaimRejected
		if err := s.repo.UpdateClaim(ctx, c); err != nil {
			return fmt.Errorf("expenses: HandleClaimApprovalDecision: %w", err)
		}
		return nil
	}

	// An approval decision does NOT overrule the per-line review. If lines
	// are still undecided, approving the instance leaves the claim
	// partially_approved — SettleClaim then refuses until every line has an
	// answer, rather than quietly paying out an unreviewed line as zero.
	if err := s.hydrateTotals(ctx, c, false); err != nil {
		return err
	}
	c.Status = ClaimApproved
	if c.UndecidedLines != nil && *c.UndecidedLines > 0 {
		c.Status = ClaimPartiallyApproved
	}
	if err := s.repo.UpdateClaim(ctx, c); err != nil {
		return fmt.Errorf("expenses: HandleClaimApprovalDecision: %w", err)
	}
	return nil
}

// SettleClaim is the 7C boundary in one method: divide the approved total
// against any outstanding advance, then create the reimbursement that
// payroll actually pays. hrm/payslips is not touched — 7C already built
// that seam and this rides it.
func (s *serviceImpl) SettleClaim(ctx context.Context, orgID, ref, settledBy string) (*SettlementResult, error) {
	c, err := s.repo.FindClaimByRef(ctx, orgID, ref)
	if err != nil {
		return nil, fmt.Errorf("expenses: SettleClaim: %w", err)
	}
	if c == nil {
		return nil, ErrClaimNotFound
	}
	if c.Status == ClaimPaid {
		return nil, ErrAlreadySettled
	}
	if c.Status != ClaimApproved {
		return nil, ErrWrongStatus
	}
	if err := s.hydrateTotals(ctx, c, false); err != nil {
		return nil, err
	}
	// Refuse rather than treat an unreviewed line as zero — silently paying
	// out less than was claimed is the failure mode this guards.
	if c.UndecidedLines != nil && *c.UndecidedLines > 0 {
		return nil, ErrLinesUndecided
	}

	outstanding := decimal.Zero
	var advance *Advance
	if c.AdvanceID != nil {
		advance, err = s.repo.FindAdvanceByRef(ctx, orgID, *c.AdvanceID)
		if err != nil {
			return nil, fmt.Errorf("expenses: SettleClaim: %w", err)
		}
		if advance != nil {
			outstanding = advance.Outstanding()
		}
	}

	settlement := SettleAgainstAdvance(*c.TotalApproved, outstanding)

	// Consume the advance by what this claim actually used.
	if advance != nil && settlement.AppliedToAdvance.IsPositive() {
		advance.SettledAmount = advance.SettledAmount.Add(settlement.AppliedToAdvance)
		if advance.SettledAmount.GreaterThanOrEqual(advance.Amount) {
			advance.Status = AdvanceSettled
		}
		if err := s.repo.UpdateAdvance(ctx, advance); err != nil {
			return nil, fmt.Errorf("expenses: SettleClaim: %w", err)
		}
	}

	result := &SettlementResult{
		Outcome: settlement.Outcome, Applied: settlement.AppliedToAdvance,
		Payable: settlement.PayableToEmployee, Recoverable: settlement.RecoverableFromEmployee,
	}

	// Only a positive payable becomes a reimbursement. In the exact and
	// employee-owes cases there is nothing for payroll to pay, so no
	// reimbursement is created — an empty one would show up in a payroll run
	// as a zero line nobody can explain.
	if settlement.PayableToEmployee.IsPositive() && s.reimbursements != nil {
		desc := "Expense claim: " + c.Title
		r, err := s.reimbursements.Create(ctx, orgID, settledBy, reimbursements.CreateRequest{
			EmployeeID:  c.EmployeeID,
			Category:    string(reimbursements.CategoryTravel),
			Description: &desc,
			Amount:      settlement.PayableToEmployee.String(),
			Currency:    &c.BaseCurrency,
		})
		if err != nil {
			return nil, fmt.Errorf("expenses: SettleClaim: create reimbursement: %w", err)
		}
		c.ReimbursementID = &r.ID
		result.ReimbursementID = &r.ID
	}

	c.Status = ClaimPaid
	if err := s.repo.UpdateClaim(ctx, c); err != nil {
		return nil, fmt.Errorf("expenses: SettleClaim: %w", err)
	}

	settled, err := s.GetClaim(ctx, orgID, c.ID)
	if err != nil {
		return nil, err
	}
	result.Claim = settled
	return result, nil
}

// ── Shared helpers ───────────────────────────────────────────────────────────

const (
	dateLayout      = "2006-01-02"
	timestampLayout = time.RFC3339
)

func parseMoney(v string) (decimal.Decimal, error) {
	d, err := decimal.NewFromString(strings.TrimSpace(v))
	if err != nil || d.IsNegative() {
		return decimal.Zero, ErrInvalidAmount
	}
	return d, nil
}

func parseMoneyOrZero(v *string) (decimal.Decimal, error) {
	if v == nil || strings.TrimSpace(*v) == "" {
		return decimal.Zero, nil
	}
	return parseMoney(*v)
}

func parseRequiredDate(v string) (*time.Time, error) {
	v = strings.TrimSpace(v)
	if v == "" {
		return nil, fmt.Errorf("expenses: date is required")
	}
	d, err := time.Parse(dateLayout, v)
	if err != nil {
		return nil, fmt.Errorf("expenses: invalid date %q: %w", v, err)
	}
	return &d, nil
}

func parseOptionalTimestamp(v *string) (*time.Time, error) {
	if v == nil || strings.TrimSpace(*v) == "" {
		return nil, nil
	}
	t, err := time.Parse(timestampLayout, strings.TrimSpace(*v))
	if err != nil {
		return nil, fmt.Errorf("expenses: invalid timestamp %q: %w", *v, err)
	}
	return &t, nil
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

// OutstandingAdvancesForEmployee reports advances a leaver never accounted
// for, so the F&F settlement can recover them.
func (s *serviceImpl) OutstandingAdvancesForEmployee(ctx context.Context, orgID, employeeID string) ([]*Advance, error) {
	out, err := s.repo.OutstandingAdvancesForEmployee(ctx, orgID, employeeID)
	if err != nil {
		return nil, fmt.Errorf("expenses: OutstandingAdvancesForEmployee: %w", err)
	}
	return out, nil
}

// RecoverAdvanceForSettlement marks an advance recovered through payroll.
//
// It advances settled_amount rather than overwriting it, so an advance a
// claim had already partly settled is not silently reset — the same
// discipline SettleClaim uses. Status flips to 'settled' only once nothing
// remains outstanding.
func (s *serviceImpl) RecoverAdvanceForSettlement(ctx context.Context, orgID, advanceID string, amount decimal.Decimal) error {
	if !amount.IsPositive() {
		return nil
	}
	a, err := s.repo.FindAdvanceByRef(ctx, orgID, advanceID)
	if err != nil {
		return fmt.Errorf("expenses: RecoverAdvanceForSettlement: %w", err)
	}
	if a == nil {
		return ErrAdvanceNotFound
	}
	a.SettledAmount = a.SettledAmount.Add(amount)
	if a.SettledAmount.GreaterThanOrEqual(a.Amount) {
		// Never record more settled than was ever advanced: a recovery
		// larger than the balance would make Outstanding() go negative and
		// read as the company owing the leaver money it never advanced.
		a.SettledAmount = a.Amount
		a.Status = AdvanceSettled
	}
	if err := s.repo.UpdateAdvance(ctx, a); err != nil {
		return fmt.Errorf("expenses: RecoverAdvanceForSettlement: %w", err)
	}
	return nil
}

// lookupRate resolves the rate to convert a line's currency into the
// organization's base currency on the expense date.
//
// ⚠ IT DEGRADES TO "NO CONVERSION" AT EVERY STEP, NEVER TO A GUESS. No FX
// source wired, no base currency configured, the line already in base
// currency, or no rate recorded for that pair on that date — each returns a
// nil date, and AddLine then leaves the rate at 1 exactly as it did before
// 11B-1 existed. The only way this function changes a stored figure is by
// finding a real rate somebody recorded.
func (s *serviceImpl) lookupRate(ctx context.Context, orgID, lineCurrency string, spent time.Time) (decimal.Decimal, *time.Time, error) {
	one := decimal.NewFromInt(1)
	if s.rates == nil || s.baseCurrency == nil {
		return one, nil, nil
	}
	base, err := s.baseCurrency.BaseCurrency(ctx, orgID, nil)
	if err != nil {
		return one, nil, fmt.Errorf("expenses: lookupRate: base currency: %w", err)
	}
	base = strings.ToUpper(strings.TrimSpace(base))
	line := strings.ToUpper(strings.TrimSpace(lineCurrency))
	// ⚠ A currency converted to itself is not a conversion. Recording a rate
	// of 1 with today's date would put a fabricated lookup into an audit
	// trail whose entire purpose is to say where a number came from.
	if base == "" || line == "" || base == line {
		return one, nil, nil
	}
	rate, rateDate, ok, err := s.rates.RateAsOfPrimitive(ctx, orgID, line, base, spent)
	if err != nil {
		return one, nil, fmt.Errorf("expenses: lookupRate: %w", err)
	}
	if !ok || !rate.IsPositive() {
		return one, nil, nil
	}
	return rate, &rateDate, nil
}
