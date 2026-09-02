// backend/internal/hrm/leave/balances_handler.go
package leave

import (
	"errors"
	"log/slog"
	"strconv"

	"github.com/gofiber/fiber/v3"

	"github.com/mridha/businesssaas/internal/middleware"
	"github.com/mridha/businesssaas/pkg/logger"
	"github.com/mridha/businesssaas/pkg/response"
)

// ─────────────────────────────────────────────────────────
// Policy handlers — flat, config-side, matching leave types' own shape
// ─────────────────────────────────────────────────────────

// ListPolicies godoc
//
//	@Summary		List leave balance policies
//	@Description	Returns every accrual/carry-forward/encashment policy configured for the org.
//	@Description	A leave type with no policy here has balance tracking disabled — it behaves
//	@Description	exactly as it did before Phase 2, with zero balance enforcement.
//	@Description
//	@Description	**Required permission:** `hrm.leave.view`
//	@Tags			HRM / Leave
//	@Produce		json
//	@Security		BearerAuth
//	@Param			orgId	path		string	true	"Organization ID"
//	@Success		200		{object}	response.OK{data=PolicyListResponse}
//	@Failure		401		{object}	response.Error
//	@Failure		403		{object}	response.Error
//	@Router			/organizations/{orgId}/hrm/leave/policies [get]
func (h *Handler) ListPolicies(c fiber.Ctx) error {
	log := logger.FromCtx(c)
	orgID, ok := middleware.OrganizationIDFromCtx(c)
	if !ok {
		return response.BadRequest(c, "NO_ORGANIZATION_CONTEXT", "Organization context is required")
	}
	result, err := h.service.ListPolicies(c.Context(), orgID)
	if err != nil {
		log.Error("leave: ListPolicies error", slog.Any("error", err))
		return response.InternalServerError(c)
	}
	return response.OK(c, result, "OK")
}

// CreatePolicy godoc
//
//	@Summary		Create a leave balance policy
//	@Description	Activates balance tracking for a leave type. Synchronously backfills
//	@Description	historical usage from any pre-existing approved requests for this leave
//	@Description	type (dated at each request's original approval date) — accrual itself
//	@Description	starts fresh from today, so the balance may start low or negative until
//	@Description	organic accrual catches up. Only one active policy is allowed per leave type.
//	@Description
//	@Description	**Required permission:** `hrm.leave.create`
//	@Tags			HRM / Leave
//	@Accept			json
//	@Produce		json
//	@Security		BearerAuth
//	@Param			orgId	path		string					true	"Organization ID"
//	@Param			body	body		CreatePolicyRequest		true	"Policy configuration"
//	@Success		201		{object}	response.Created{data=object{policy=LeavePolicy}}
//	@Failure		400		{object}	response.Error
//	@Failure		401		{object}	response.Error
//	@Failure		403		{object}	response.Error
//	@Failure		409		{object}	response.Error	"LEAVE_POLICY_ALREADY_EXISTS"
//	@Router			/organizations/{orgId}/hrm/leave/policies [post]
func (h *Handler) CreatePolicy(c fiber.Ctx) error {
	userID, ok := middleware.UserIDFromCtx(c)
	if !ok {
		return response.Unauthorized(c, "UNAUTHORIZED", "Authentication required")
	}
	orgID, ok := middleware.OrganizationIDFromCtx(c)
	if !ok {
		return response.BadRequest(c, "NO_ORGANIZATION_CONTEXT", "Organization context is required")
	}
	var req CreatePolicyRequest
	if err := c.Bind().JSON(&req); err != nil {
		return response.BadRequest(c, "INVALID_BODY", "Invalid request body")
	}
	p, err := h.service.CreatePolicy(c.Context(), orgID, userID, req)
	if err != nil {
		return h.balanceError(c, err)
	}
	return response.Created(c, fiber.Map{"policy": p}, "Leave policy created")
}

// GetPolicy godoc
//
//	@Summary		Get a leave balance policy
//	@Description
//	@Description	**Required permission:** `hrm.leave.view`
//	@Tags			HRM / Leave
//	@Produce		json
//	@Security		BearerAuth
//	@Param			orgId		path		string	true	"Organization ID"
//	@Param			policyId	path		string	true	"Policy public ID (lvp_*)"
//	@Success		200			{object}	response.OK{data=object{policy=LeavePolicy}}
//	@Failure		404			{object}	response.Error
//	@Router			/organizations/{orgId}/hrm/leave/policies/{policyId} [get]
func (h *Handler) GetPolicy(c fiber.Ctx) error {
	orgID, ok := middleware.OrganizationIDFromCtx(c)
	if !ok {
		return response.BadRequest(c, "NO_ORGANIZATION_CONTEXT", "Organization context is required")
	}
	p, err := h.service.GetPolicy(c.Context(), orgID, c.Params("policyId"))
	if err != nil {
		return h.balanceError(c, err)
	}
	return response.OK(c, fiber.Map{"policy": p}, "OK")
}

// UpdatePolicy godoc
//
//	@Summary		Update a leave balance policy
//	@Description	Never re-triggers backfill — that only happens once, at policy creation.
//	@Description
//	@Description	**Required permission:** `hrm.leave.update`
//	@Tags			HRM / Leave
//	@Accept			json
//	@Produce		json
//	@Security		BearerAuth
//	@Param			orgId		path		string					true	"Organization ID"
//	@Param			policyId	path		string					true	"Policy public ID"
//	@Param			body		body		UpdatePolicyRequest		true	"Fields to update"
//	@Success		200			{object}	response.OK{data=object{policy=LeavePolicy}}
//	@Failure		400			{object}	response.Error
//	@Failure		404			{object}	response.Error
//	@Router			/organizations/{orgId}/hrm/leave/policies/{policyId} [patch]
func (h *Handler) UpdatePolicy(c fiber.Ctx) error {
	orgID, ok := middleware.OrganizationIDFromCtx(c)
	if !ok {
		return response.BadRequest(c, "NO_ORGANIZATION_CONTEXT", "Organization context is required")
	}
	var req UpdatePolicyRequest
	if err := c.Bind().JSON(&req); err != nil {
		return response.BadRequest(c, "INVALID_BODY", "Invalid request body")
	}
	p, err := h.service.UpdatePolicy(c.Context(), orgID, c.Params("policyId"), req)
	if err != nil {
		return h.balanceError(c, err)
	}
	return response.OK(c, fiber.Map{"policy": p}, "Leave policy updated")
}

// ─────────────────────────────────────────────────────────
// Balance/ledger handlers — employee-scoped, nested under
// /hrm/employees/:employeeId/leave, matching salary's per-employee shape.
// Reads reuse the hrm.leave.view_own/view_team/view_all tiers already
// seeded for leave requests — no new scope permissions needed (see
// resolveEmployeeAccess).
// ─────────────────────────────────────────────────────────

// resolveEmployeeAccess resolves the caller's scope tier for "hrm.leave" and
// checks it authorizes access to employeeID's records. Returns a non-nil
// error only when a response has already been written (Unauthorized,
// InternalServerError, or Forbidden) — callers should return it directly.
func (h *Handler) resolveEmployeeAccess(c fiber.Ctx, orgID, userID, employeeID string) error {
	log := logger.FromCtx(c)
	scopeTier, err := h.authz.ResolveScope(c.Context(), userID, orgID, "hrm.leave")
	if err != nil {
		log.Error("leave: resolveEmployeeAccess", slog.Any("error", err))
		return response.InternalServerError(c)
	}
	allowed, err := h.scopeResolver.AuthorizeRecordAccess(c.Context(), scopeTier, orgID, userID, employeeID)
	if err != nil {
		log.Error("leave: resolveEmployeeAccess", slog.Any("error", err))
		return response.InternalServerError(c)
	}
	if !allowed {
		return response.Forbidden(c, "RECORD_ACCESS_DENIED", "You do not have access to this record")
	}
	return nil
}

// ListBalances godoc
//
//	@Summary		List an employee's current leave balances
//	@Description	Returns one entry per leave type — HasPolicy=false means balance tracking
//	@Description	isn't opted in for that type (no policy configured), Balance/HasPolicy
//	@Description	otherwise reflect the checkpoint+delta read (see CurrentBalance).
//	@Description
//	@Description	**Required permission:** `hrm.leave.view` (scoped by view_own/view_team/view_all)
//	@Tags			HRM / Leave
//	@Produce		json
//	@Security		BearerAuth
//	@Param			orgId		path		string	true	"Organization ID"
//	@Param			employeeId	path		string	true	"Employee UUID"
//	@Success		200			{object}	response.OK{data=object{balances=[]CurrentBalance}}
//	@Failure		401			{object}	response.Error
//	@Failure		403			{object}	response.Error	"RECORD_ACCESS_DENIED"
//	@Router			/organizations/{orgId}/hrm/employees/{employeeId}/leave/balances [get]
func (h *Handler) ListBalances(c fiber.Ctx) error {
	log := logger.FromCtx(c)
	userID, ok := middleware.UserIDFromCtx(c)
	if !ok {
		return response.Unauthorized(c, "UNAUTHORIZED", "Authentication required")
	}
	orgID, ok := middleware.OrganizationIDFromCtx(c)
	if !ok {
		return response.BadRequest(c, "NO_ORGANIZATION_CONTEXT", "Organization context is required")
	}
	employeeID := c.Params("employeeId")
	if respErr := h.resolveEmployeeAccess(c, orgID, userID, employeeID); respErr != nil {
		return respErr
	}
	list, err := h.service.ListCurrentBalances(c.Context(), orgID, employeeID)
	if err != nil {
		log.Error("leave: ListBalances error", slog.Any("error", err))
		return response.InternalServerError(c)
	}
	return response.OK(c, fiber.Map{"balances": list}, "OK")
}

// GetBalance godoc
//
//	@Summary		Get an employee's current balance for one leave type
//	@Description
//	@Description	**Required permission:** `hrm.leave.view` (scoped by view_own/view_team/view_all)
//	@Tags			HRM / Leave
//	@Produce		json
//	@Security		BearerAuth
//	@Param			orgId		path		string	true	"Organization ID"
//	@Param			employeeId	path		string	true	"Employee UUID"
//	@Param			leaveTypeId	path		string	true	"Leave type ID or public ID"
//	@Success		200			{object}	response.OK{data=object{balance=CurrentBalance}}
//	@Failure		401			{object}	response.Error
//	@Failure		403			{object}	response.Error	"RECORD_ACCESS_DENIED"
//	@Failure		404			{object}	response.Error	"LEAVE_TYPE_NOT_FOUND"
//	@Router			/organizations/{orgId}/hrm/employees/{employeeId}/leave/balances/{leaveTypeId} [get]
func (h *Handler) GetBalance(c fiber.Ctx) error {
	userID, ok := middleware.UserIDFromCtx(c)
	if !ok {
		return response.Unauthorized(c, "UNAUTHORIZED", "Authentication required")
	}
	orgID, ok := middleware.OrganizationIDFromCtx(c)
	if !ok {
		return response.BadRequest(c, "NO_ORGANIZATION_CONTEXT", "Organization context is required")
	}
	employeeID := c.Params("employeeId")
	if respErr := h.resolveEmployeeAccess(c, orgID, userID, employeeID); respErr != nil {
		return respErr
	}
	cb, err := h.service.GetCurrentBalance(c.Context(), orgID, employeeID, c.Params("leaveTypeId"))
	if err != nil {
		return h.balanceError(c, err)
	}
	return response.OK(c, fiber.Map{"balance": cb}, "OK")
}

// GetBalanceHistory godoc
//
//	@Summary		Get an employee's monthly balance snapshot history for one leave type
//	@Description	Each row is an immutable monthly snapshot — see hrm_leave_balances.
//	@Description
//	@Description	**Required permission:** `hrm.leave.view` (scoped by view_own/view_team/view_all)
//	@Tags			HRM / Leave
//	@Produce		json
//	@Security		BearerAuth
//	@Param			orgId		path		string	true	"Organization ID"
//	@Param			employeeId	path		string	true	"Employee UUID"
//	@Param			leaveTypeId	path		string	true	"Leave type ID or public ID"
//	@Param			limit		query		int		false	"Default 50, max 200"
//	@Param			offset		query		int		false	"Default 0"
//	@Success		200			{object}	response.OK{data=BalanceHistoryResponse}
//	@Failure		401			{object}	response.Error
//	@Failure		403			{object}	response.Error	"RECORD_ACCESS_DENIED"
//	@Router			/organizations/{orgId}/hrm/employees/{employeeId}/leave/balances/{leaveTypeId}/history [get]
func (h *Handler) GetBalanceHistory(c fiber.Ctx) error {
	userID, ok := middleware.UserIDFromCtx(c)
	if !ok {
		return response.Unauthorized(c, "UNAUTHORIZED", "Authentication required")
	}
	orgID, ok := middleware.OrganizationIDFromCtx(c)
	if !ok {
		return response.BadRequest(c, "NO_ORGANIZATION_CONTEXT", "Organization context is required")
	}
	employeeID := c.Params("employeeId")
	if respErr := h.resolveEmployeeAccess(c, orgID, userID, employeeID); respErr != nil {
		return respErr
	}
	limit, _ := strconv.Atoi(c.Query("limit", ""))
	offset, _ := strconv.Atoi(c.Query("offset", ""))
	result, err := h.service.ListBalanceHistory(c.Context(), orgID, employeeID, c.Params("leaveTypeId"), limit, offset)
	if err != nil {
		return h.balanceError(c, err)
	}
	return response.OK(c, result, "OK")
}

// ListTransactions godoc
//
//	@Summary		List an employee's leave ledger entries
//	@Description	The append-only source of truth — every accrual, usage, usage_reversal,
//	@Description	encashment, carry_forward, forfeiture, and adjustment ever posted.
//	@Description
//	@Description	**Required permission:** `hrm.leave.view` (scoped by view_own/view_team/view_all)
//	@Tags			HRM / Leave
//	@Produce		json
//	@Security		BearerAuth
//	@Param			orgId			path		string	true	"Organization ID"
//	@Param			employeeId		path		string	true	"Employee UUID"
//	@Param			leave_type_id	query		string	false	"Filter by leave type"
//	@Param			type			query		string	false	"Filter by transaction_type"
//	@Param			limit			query		int		false	"Default 50, max 200"
//	@Param			offset			query		int		false	"Default 0"
//	@Success		200				{object}	response.OK{data=TransactionListResponse}
//	@Failure		401				{object}	response.Error
//	@Failure		403				{object}	response.Error	"RECORD_ACCESS_DENIED"
//	@Router			/organizations/{orgId}/hrm/employees/{employeeId}/leave/transactions [get]
func (h *Handler) ListTransactions(c fiber.Ctx) error {
	log := logger.FromCtx(c)
	userID, ok := middleware.UserIDFromCtx(c)
	if !ok {
		return response.Unauthorized(c, "UNAUTHORIZED", "Authentication required")
	}
	orgID, ok := middleware.OrganizationIDFromCtx(c)
	if !ok {
		return response.BadRequest(c, "NO_ORGANIZATION_CONTEXT", "Organization context is required")
	}
	employeeID := c.Params("employeeId")
	if respErr := h.resolveEmployeeAccess(c, orgID, userID, employeeID); respErr != nil {
		return respErr
	}

	filter := TransactionFilter{
		LeaveTypeID:     c.Query("leave_type_id"),
		TransactionType: TransactionType(c.Query("type")),
	}
	if limit, err := strconv.Atoi(c.Query("limit", "")); err == nil {
		filter.Limit = limit
	}
	if offset, err := strconv.Atoi(c.Query("offset", "")); err == nil {
		filter.Offset = offset
	}

	result, err := h.service.ListTransactions(c.Context(), orgID, employeeID, filter)
	if err != nil {
		log.Error("leave: ListTransactions error", slog.Any("error", err))
		return response.InternalServerError(c)
	}
	return response.OK(c, result, "OK")
}

// AdjustBalance godoc
//
//	@Summary		Manually correct an employee's leave balance
//	@Description	Days is signed — positive credits the balance, negative debits it. A note
//	@Description	is mandatory: every manual correction must explain itself in the ledger.
//	@Description	Skips record-scoping — same as ApproveRequest/RejectRequest, an
//	@Description	owner/admin-only action that already carries ScopeAll.
//	@Description
//	@Description	**Required permission:** `hrm.leave.adjust_balance` (owner/admin only)
//	@Tags			HRM / Leave
//	@Accept			json
//	@Produce		json
//	@Security		BearerAuth
//	@Param			orgId		path		string					true	"Organization ID"
//	@Param			employeeId	path		string					true	"Employee UUID"
//	@Param			leaveTypeId	path		string					true	"Leave type ID or public ID"
//	@Param			body		body		PostAdjustmentRequest	true	"Signed days + mandatory note"
//	@Success		201			{object}	response.Created{data=object{transaction=LeaveTransaction}}
//	@Failure		400			{object}	response.Error	"ADJUSTMENT_NOTE_REQUIRED or ADJUSTMENT_DAYS_ZERO"
//	@Failure		409			{object}	response.Error	"NO_ACTIVE_POLICY"
//	@Router			/organizations/{orgId}/hrm/employees/{employeeId}/leave/balances/{leaveTypeId}/adjust [post]
func (h *Handler) AdjustBalance(c fiber.Ctx) error {
	userID, ok := middleware.UserIDFromCtx(c)
	if !ok {
		return response.Unauthorized(c, "UNAUTHORIZED", "Authentication required")
	}
	orgID, ok := middleware.OrganizationIDFromCtx(c)
	if !ok {
		return response.BadRequest(c, "NO_ORGANIZATION_CONTEXT", "Organization context is required")
	}
	var req PostAdjustmentRequest
	if err := c.Bind().JSON(&req); err != nil {
		return response.BadRequest(c, "INVALID_BODY", "Invalid request body")
	}
	t, err := h.service.PostAdjustment(c.Context(), orgID, c.Params("employeeId"), c.Params("leaveTypeId"), userID, req)
	if err != nil {
		return h.balanceError(c, err)
	}
	return response.Created(c, fiber.Map{"transaction": t}, "Balance adjusted")
}

// EncashBalance godoc
//
//	@Summary		Record a leave encashment
//	@Description	Reduces the balance and logs the days encashed. Does NOT compute a
//	@Description	currency amount — encashment_rate_basis on the policy is config a future
//	@Description	F&F (Full & Final settlement) phase reads; this phase never evaluates it.
//	@Description	The leave type's policy must have encashable=true.
//	@Description
//	@Description	**Required permission:** `hrm.leave.encash` (owner/admin only)
//	@Tags			HRM / Leave
//	@Accept			json
//	@Produce		json
//	@Security		BearerAuth
//	@Param			orgId		path		string					true	"Organization ID"
//	@Param			employeeId	path		string					true	"Employee UUID"
//	@Param			leaveTypeId	path		string					true	"Leave type ID or public ID"
//	@Param			body		body		PostEncashmentRequest	true	"Days to encash"
//	@Success		201			{object}	response.Created{data=object{transaction=LeaveTransaction}}
//	@Failure		400			{object}	response.Error	"ENCASHMENT_DAYS_INVALID"
//	@Failure		409			{object}	response.Error	"ENCASHMENT_NOT_ALLOWED or NO_ACTIVE_POLICY"
//	@Router			/organizations/{orgId}/hrm/employees/{employeeId}/leave/balances/{leaveTypeId}/encash [post]
func (h *Handler) EncashBalance(c fiber.Ctx) error {
	userID, ok := middleware.UserIDFromCtx(c)
	if !ok {
		return response.Unauthorized(c, "UNAUTHORIZED", "Authentication required")
	}
	orgID, ok := middleware.OrganizationIDFromCtx(c)
	if !ok {
		return response.BadRequest(c, "NO_ORGANIZATION_CONTEXT", "Organization context is required")
	}
	var req PostEncashmentRequest
	if err := c.Bind().JSON(&req); err != nil {
		return response.BadRequest(c, "INVALID_BODY", "Invalid request body")
	}
	t, err := h.service.PostEncashment(c.Context(), orgID, c.Params("employeeId"), c.Params("leaveTypeId"), userID, req)
	if err != nil {
		return h.balanceError(c, err)
	}
	return response.Created(c, fiber.Map{"transaction": t}, "Leave encashed")
}

// ─────────────────────────────────────────────────────────
// Error mapper
// ─────────────────────────────────────────────────────────

func (h *Handler) balanceError(c fiber.Ctx, err error) error {
	log := logger.FromCtx(c)
	switch {
	case errors.Is(err, ErrLeaveTypeNotFound):
		return response.NotFound(c, "LEAVE_TYPE_NOT_FOUND", "Leave type not found")
	case errors.Is(err, ErrLeaveTypeIDRequired):
		return response.BadRequest(c, "LEAVE_TYPE_ID_REQUIRED", "leave_type_id is required")
	case errors.Is(err, ErrPolicyNotFound):
		return response.NotFound(c, "LEAVE_POLICY_NOT_FOUND", "Leave policy not found")
	case errors.Is(err, ErrPolicyAlreadyExists):
		return response.Conflict(c, "LEAVE_POLICY_ALREADY_EXISTS", "An active leave policy already exists for this leave type")
	case errors.Is(err, ErrInvalidAccrualMethod):
		return response.BadRequest(c, "INVALID_ACCRUAL_METHOD", "accrual_method must be one of: monthly, annual, on_joining")
	case errors.Is(err, ErrInvalidAccrualRate):
		return response.BadRequest(c, "INVALID_ACCRUAL_RATE", "accrual_rate must be zero or greater")
	case errors.Is(err, ErrInvalidCarryForwardCap):
		return response.BadRequest(c, "INVALID_CARRY_FORWARD_CAP", "carry_forward_cap must be zero or greater when set")
	case errors.Is(err, ErrInvalidEncashmentBasis):
		return response.BadRequest(c, "INVALID_ENCASHMENT_BASIS", "encashment_rate_basis must be one of: basic_pay, gross_pay, fixed")
	case errors.Is(err, ErrAdjustmentNoteRequired):
		return response.BadRequest(c, "ADJUSTMENT_NOTE_REQUIRED", "A note is required for a manual balance adjustment")
	case errors.Is(err, ErrAdjustmentDaysZero):
		return response.BadRequest(c, "ADJUSTMENT_DAYS_ZERO", "Adjustment days must not be zero")
	case errors.Is(err, ErrEncashmentNotAllowed):
		return response.Conflict(c, "ENCASHMENT_NOT_ALLOWED", "This leave type's policy does not allow encashment")
	case errors.Is(err, ErrEncashmentDaysInvalid):
		return response.BadRequest(c, "ENCASHMENT_DAYS_INVALID", "Encashment days must be greater than zero")
	case errors.Is(err, ErrNoActivePolicy):
		return response.Conflict(c, "NO_ACTIVE_POLICY", "No active leave policy exists for this leave type")
	default:
		log.Error("leave balance error", slog.Any("error", err))
		return response.InternalServerError(c)
	}
}
