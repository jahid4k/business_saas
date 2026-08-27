// backend/internal/hrm/payslips/model.go
package payslips

import (
	"context"
	"errors"
	"time"

	"github.com/shopspring/decimal"

	"github.com/mridha/businesssaas/internal/authz"
)

type RunStatus string

const (
	RunDraft     RunStatus = "draft"
	RunComputing RunStatus = "computing"
	RunComputed  RunStatus = "computed"
	RunApproved  RunStatus = "approved"
	RunPaid      RunStatus = "paid"
	RunCancelled RunStatus = "cancelled"
)

type SlipStatus string

const (
	SlipDraft    SlipStatus = "draft"
	SlipComputed SlipStatus = "computed"
	SlipApproved SlipStatus = "approved"
	SlipPaid     SlipStatus = "paid"
)

// PayslipRun is the monthly payroll batch for an organization.
type PayslipRun struct {
	ID                 string          `db:"id"                    json:"id"`
	PublicID           string          `db:"public_id"             json:"public_id"`
	OrgID              string          `db:"org_id"                json:"org_id"`
	PeriodYear         int             `db:"period_year"           json:"period_year"`
	PeriodMonth        int             `db:"period_month"          json:"period_month"`
	RunType            RunType         `db:"run_type"              json:"run_type"`
	Description        *string         `db:"description"           json:"description,omitempty"`
	Currency           string          `db:"currency"              json:"currency"`
	AttendancePeriodID *string         `db:"attendance_period_id"  json:"attendance_period_id,omitempty"`
	TotalEmployees     int             `db:"total_employees"       json:"total_employees"`
	TotalGrossPay      decimal.Decimal `db:"total_gross_pay"       json:"total_gross_pay"`
	TotalDeductions    decimal.Decimal `db:"total_deductions"      json:"total_deductions"`
	TotalNetPay        decimal.Decimal `db:"total_net_pay"         json:"total_net_pay"`
	Status             RunStatus       `db:"status"                json:"status"`
	ComputedAt         *time.Time      `db:"computed_at"           json:"computed_at,omitempty"`
	ComputedBy         *string         `db:"computed_by"           json:"computed_by,omitempty"`
	ApprovedAt         *time.Time      `db:"approved_at"           json:"approved_at,omitempty"`
	ApprovedBy         *string         `db:"approved_by"           json:"approved_by,omitempty"`
	PaidAt             *time.Time      `db:"paid_at"               json:"paid_at,omitempty"`
	PaidBy             *string         `db:"paid_by"               json:"paid_by,omitempty"`
	CreatedBy          string          `db:"created_by"            json:"created_by"`
	CreatedAt          time.Time       `db:"created_at"            json:"created_at"`
	UpdatedAt          time.Time       `db:"updated_at"            json:"updated_at"`
}

// Payslip is an individual employee payslip within a run.
type Payslip struct {
	ID                  string          `db:"id"                    json:"id"`
	PublicID            string          `db:"public_id"             json:"public_id"`
	OrgID               string          `db:"org_id"                json:"org_id"`
	EmployeeID          string          `db:"employee_id"           json:"employee_id"`
	PayslipRunID        string          `db:"payslip_run_id"        json:"payslip_run_id"`
	PeriodYear          int             `db:"period_year"           json:"period_year"`
	PeriodMonth         int             `db:"period_month"          json:"period_month"`
	SalaryStructureID   *string         `db:"salary_structure_id"   json:"salary_structure_id,omitempty"`
	SalaryStructureName *string         `db:"salary_structure_name" json:"salary_structure_name,omitempty"`
	GrossPay            decimal.Decimal `db:"gross_pay"             json:"gross_pay"`
	TotalDeductions     decimal.Decimal `db:"total_deductions"      json:"total_deductions"`
	NetPay              decimal.Decimal `db:"net_pay"               json:"net_pay"`
	BasicPay            decimal.Decimal `db:"basic_pay"             json:"basic_pay"`
	WorkDays            int             `db:"work_days"             json:"work_days"`
	PresentDays         int             `db:"present_days"          json:"present_days"`
	AbsentDays          int             `db:"absent_days"           json:"absent_days"`
	LeaveDays           int             `db:"leave_days"            json:"leave_days"`
	HolidayDays         int             `db:"holiday_days"          json:"holiday_days"`
	OvertimeHours       float64         `db:"overtime_hours"        json:"overtime_hours"`
	Currency            string          `db:"currency"              json:"currency"`
	Status              SlipStatus      `db:"status"                json:"status"`
	PaymentReference    *string         `db:"payment_reference"     json:"payment_reference,omitempty"`
	PaymentDate         *string         `db:"payment_date"          json:"payment_date,omitempty"`
	PaidAt              *time.Time      `db:"paid_at"               json:"paid_at,omitempty"`
	Lines               []*PayslipLine  `db:"-"               json:"lines,omitempty"`
	CreatedAt           time.Time       `db:"created_at"            json:"created_at"`
	UpdatedAt           time.Time       `db:"updated_at"            json:"updated_at"`
}

// PayslipLine is one salary component row within a payslip.
type PayslipLine struct {
	ID             string          `db:"id"              json:"id"`
	PayslipID      string          `db:"payslip_id"      json:"payslip_id"`
	OrgID          string          `db:"org_id"          json:"org_id"`
	ComponentID    *string         `db:"component_id"    json:"component_id,omitempty"`
	ComponentName  string          `db:"component_name"  json:"component_name"`
	ComponentType  string          `db:"component_type"  json:"component_type"`
	CalcMethod     string          `db:"calc_method"     json:"calc_method"`
	FormulaUsed    *string         `db:"formula_used"    json:"formula_used,omitempty"`
	ComputedAmount decimal.Decimal `db:"computed_amount" json:"computed_amount"`
	// LineType is what the line does; ComponentType above is what the component
	// was. Both are kept — see migration 00096.
	LineType               LineType `db:"line_type"                json:"line_type"`
	IsEmployerContribution bool     `db:"is_employer_contribution" json:"is_employer_contribution"`
	// SourcePeriodID names the run whose period an arrear recovers for.
	SourcePeriodID *string   `db:"source_period_id" json:"source_period_id,omitempty"`
	DisplayOrder   int       `db:"display_order"    json:"display_order"`
	CreatedAt      time.Time `db:"created_at"      json:"created_at"`

	// sourceLoanScheduleID / sourceReimbursementID correlate a compute-time
	// line back to the loan installment / reimbursement that produced it.
	// Package-private, never persisted (no db tag matters — unexported
	// fields are invisible to the SQL layer) and never serialized. Read only
	// by ComputeRun, immediately after CreatePayslipLines assigns the real
	// database ID onto this same pointer — no index bookkeeping required,
	// unlike computedPayslip.SourceBonusIDs (bonus payslips contain ONLY
	// bonus lines, so a parallel index-matched slice was simplest there;
	// here a line sits among ordinary salary-structure lines, so attaching
	// the correlation directly to the line it belongs to is what's robust).
	sourceLoanScheduleID  string
	sourceReimbursementID string
	// sourceSettlementType / sourceSettlementID do the same job for an F&F
	// settlement line, correlating it back to the loan, advance or clearance
	// item that produced it so MarkSettled can mark that source consumed.
	// A type as well as an id, because a settlement's sources live in four
	// different tables and an id alone is ambiguous between them.
	sourceSettlementType string
	sourceSettlementID   string
}

// CreateRunRequest creates a new payroll run.
// RunType distinguishes the monthly cycle from the runs that sit alongside it.
//
// Only 'regular' is capped at one per org per month — that is enforced by the
// partial unique index uq_hrm_pr_org_month_regular, not by application code.
// The others are deliberately repeatable within a period: a leaver's final
// settlement cannot wait for next month, and a bonus is not a substitute for
// payroll.
type RunType string

const (
	RunTypeRegular  RunType = "regular"
	RunTypeOffCycle RunType = "off_cycle"
	RunTypeBonus    RunType = "bonus"
	RunTypeArrears  RunType = "arrears"
	RunTypeFnF      RunType = "fnf"
)

func (t RunType) IsValid() bool {
	switch t {
	case RunTypeRegular, RunTypeOffCycle, RunTypeBonus, RunTypeArrears, RunTypeFnF:
		return true
	}
	return false
}

// IsUniquePerPeriod mirrors uq_hrm_pr_org_month_regular. The index is the
// guarantee; this is the shared definition the friendly duplicate check reads,
// so the two cannot drift apart.
func (t RunType) IsUniquePerPeriod() bool { return t == RunTypeRegular }

// LineType is what a payslip line DOES in the calculation.
//
// Deliberately distinct from PayslipLine.ComponentType, which snapshots what
// the COMPONENT was. They diverge as soon as a line has no component behind it
// — a loan recovery, a statutory deduction derived from a rule, an arrear
// recovered from an earlier period. See migration 00096.
type LineType string

const (
	LineEarning       LineType = "earning"
	LineDeduction     LineType = "deduction"
	LineArrear        LineType = "arrear"
	LineReimbursement LineType = "reimbursement"
	LineLoanRecovery  LineType = "loan_recovery"
	LineStatutory     LineType = "statutory"
)

func (t LineType) IsValid() bool {
	switch t {
	case LineEarning, LineDeduction, LineArrear,
		LineReimbursement, LineLoanRecovery, LineStatutory:
		return true
	}
	return false
}

// ReducesNet reports whether a line is subtracted from gross. Earnings and
// reimbursements add; everything else takes away. Keeping this on the type
// stops each new line type having to be remembered in the compute loop.
func (t LineType) ReducesNet() bool {
	switch t {
	case LineDeduction, LineLoanRecovery, LineStatutory:
		return true
	}
	return false
}

// RunPreview is a dry run's result: what ComputeRun WOULD produce, computed
// by the same code and persisted nowhere.
//
// NegativeNetCount is the field that earns the endpoint its keep — it is the
// condition that will block approval, surfaced before anyone commits the run
// rather than after.
type RunPreview struct {
	RunID       string  `json:"run_id"`
	RunType     RunType `json:"run_type"`
	PeriodYear  int     `json:"period_year"`
	PeriodMonth int     `json:"period_month"`
	Currency    string  `json:"currency"`

	TotalEmployees   int             `json:"total_employees"`
	TotalGrossPay    decimal.Decimal `json:"total_gross_pay"`
	TotalDeductions  decimal.Decimal `json:"total_deductions"`
	TotalNetPay      decimal.Decimal `json:"total_net_pay"`
	NegativeNetCount int             `json:"negative_net_count"`

	// Payslips are unsaved values — they carry no id, and nothing in the
	// database corresponds to them.
	Payslips []*Payslip `json:"payslips"`
}

type CreateRunRequest struct {
	Year               int     `json:"year"`
	Month              int     `json:"month"`
	RunType            *string `json:"run_type"` // defaults to 'regular'
	Description        *string `json:"description"`
	Currency           *string `json:"currency"`
	AttendancePeriodID *string `json:"attendance_period_id"` // optional D1 link
}

type RunListResponse struct {
	Runs  []*PayslipRun `json:"runs"`
	Total int           `json:"total"`
}

type SlipListResponse struct {
	Payslips []*Payslip `json:"payslips"`
	Total    int        `json:"total"`
	Limit    int        `json:"limit"`
	Offset   int        `json:"offset"`
}

// SlipListFilter narrows the payslip list query.
type SlipListFilter struct {
	RunID      string
	EmployeeID string
	Limit      int
	Offset     int

	// Scope and CallerUserID are set by the handler (from authzSvc.ResolveScope)
	// before calling Service.ListPayslips. Scope zero value (authz.ScopeNone)
	// means "no rows" — callers that intend no scoping must explicitly pass
	// authz.ScopeAll.
	Scope        authz.Scope
	CallerUserID string
}

const (
	DefaultLimit = 50
	MaxLimit     = 200
)

func (f *SlipListFilter) Normalise() {
	if f.Limit <= 0 {
		f.Limit = DefaultLimit
	}
	if f.Limit > MaxLimit {
		f.Limit = MaxLimit
	}
	if f.Offset < 0 {
		f.Offset = 0
	}
}

// PendingBonus is one bonus owed to an employee for a period, as reported by
// a BonusSource.
type PendingBonus struct {
	BonusID     string
	EmployeeID  string
	Description string // becomes the payslip line's component_name
	Amount      decimal.Decimal
}

// PaidBonusLine reports which payslip line a bonus was actually paid
// through, so a BonusSource can mark it paid against the real record.
type PaidBonusLine struct {
	BonusID string
	LineID  string
}

// BonusSource supplies approved-but-unpaid bonuses for a run_type='bonus'
// payroll run's period, and is told afterward which of them were persisted.
//
// Declared here, not in hrm/compensation, because payslips is the CONSUMER —
// the recruitment.EmployeeCreator / pip.TerminationCreator pattern: the
// orchestrator (whoever calls the interface) owns it, and the provider
// (hrm/compensation.Service) imports payslips to satisfy it with real types.
//
// A nil BonusSource is valid — hrm/compensation may not exist yet at wiring
// time in a given deployment shape — and simply makes a bonus run compute to
// zero payslips, not a panic. The platform/checklists.ChecklistHook /
// performance.FormEngine nil-hook precedent.
type BonusSource interface {
	PendingBonusesForPeriod(ctx context.Context, orgID string, year, month int) ([]PendingBonus, error)
	// MarkBonusesPaid is called ONLY after every payslip and line in the run
	// has been persisted successfully — never from inside abortCompute. A
	// failure here is treated as a full compute failure (see ComputeRun): it
	// triggers the same abort-and-cleanup path as a payslip write failing,
	// rather than leaving payslips committed with their bonuses still
	// 'approved' and payable a second time by the next run.
	MarkBonusesPaid(ctx context.Context, runID string, paid []PaidBonusLine) error
}

// PendingInstallment is one loan installment (or the remaining part of one,
// after a prior partial recovery) owed by an employee, as reported by a
// LoanSource. AmountDue is what remains, not the installment's original
// total_amount.
type PendingInstallment struct {
	LoanID      string
	ScheduleID  string
	Description string // becomes the payslip line's component_name
	AmountDue   decimal.Decimal
}

// RecoveryApplication reports how much of a pending installment was actually
// recovered in a run. This may be LESS than the installment's AmountDue —
// recovery is capped so it never drives an employee's net pay negative — in
// which case the shortfall stays owed and is picked up again by a later run.
type RecoveryApplication struct {
	ScheduleID    string
	LineID        string
	AmountApplied decimal.Decimal
}

// LoanSource supplies an employee's outstanding loan installments for a
// payroll period, oldest-due-first, and is told afterward how much of each
// was actually recovered.
//
// Declared here for the same reason as BonusSource: payslips is the
// CONSUMER, hrm/loans imports hrm/payslips to satisfy it. A nil LoanSource
// makes every run compute with zero loan recovery, not a panic.
type LoanSource interface {
	// PendingInstallmentsForEmployee returns installments due ON OR BEFORE
	// the given period, oldest first — a backlog from a prior run's
	// zero-net-pay capping is caught up before anything newer, not skipped.
	PendingInstallmentsForEmployee(ctx context.Context, orgID, employeeID string, year, month int) ([]PendingInstallment, error)
	// RecordRecoveries is called ONLY after every payslip and line in the run
	// has persisted successfully — the same all-or-nothing discipline as
	// MarkBonusesPaid: a failure here aborts the whole run rather than
	// leaving payslips committed with their recoveries unrecorded and
	// therefore recoverable a second time by the next run.
	RecordRecoveries(ctx context.Context, runID string, applications []RecoveryApplication) error
}

// PendingReimbursement is one approved, unpaid reimbursement owed to an
// employee, as reported by a ReimbursementSource.
type PendingReimbursement struct {
	ReimbursementID string
	Description     string
	Amount          decimal.Decimal
}

// PaidReimbursementLine reports which payslip line paid a reimbursement.
type PaidReimbursementLine struct {
	ReimbursementID string
	LineID          string
}

// ReimbursementSource supplies an employee's approved-but-unpaid
// reimbursements for a payroll period. Unlike BonusSource, this does not
// gate on run_type — reimbursements pay out alongside whatever else a
// regular/off_cycle/arrears/fnf run already computes for that employee, not
// through a dedicated run type of their own.
//
// Declared here for the same reason as BonusSource/LoanSource: payslips is
// the CONSUMER, hrm/reimbursements imports hrm/payslips to satisfy it. A nil
// ReimbursementSource makes every run pay zero reimbursements, not a panic.
type ReimbursementSource interface {
	PendingForEmployee(ctx context.Context, orgID, employeeID string, year, month int) ([]PendingReimbursement, error)
	// MarkReimbursementsPaid follows MarkBonusesPaid's failure discipline
	// exactly — called only after the run's payslips/lines have persisted,
	// and a failure aborts the whole run.
	MarkReimbursementsPaid(ctx context.Context, runID string, paid []PaidReimbursementLine) error
}

// StatutoryLine is one computed statutory deduction or employer contribution
// for an employee's payslip, as reported by a StatutorySource. Unlike
// bonuses/loans/reimbursements, there is no persisted "pending" record to
// mark paid afterward — a statutory amount is computed FRESH every run from
// the org's current rules, so StatutorySource has only one method.
type StatutoryLine struct {
	RuleID                 string
	Description            string
	Amount                 decimal.Decimal
	IsEmployerContribution bool
}

// StatutorySource computes an employee's statutory lines for a period, given
// the taxable/basic/gross bases computePayslips has already worked out —
// hrm/statutory owns the rule matching and slab evaluation, payslips owns
// nothing about what a country's rules look like.
//
// Declared here for the same reason as BonusSource/LoanSource/
// ReimbursementSource: payslips is the CONSUMER, hrm/statutory imports
// hrm/payslips (to reuse ComputeSlab) and satisfies this, not the reverse. A
// nil StatutorySource makes every run compute zero statutory lines.
type StatutorySource interface {
	ComputeForEmployee(ctx context.Context, orgID, employeeID string, year, month int, gross, basic, taxableGross decimal.Decimal) ([]StatutoryLine, error)
}

// PendingBenefitDeduction is one active benefit enrollment's recurring
// per-period employee cost, as reported by a BenefitsSource. Only the
// employee's own cost — the employer's share is tracked but produces no
// payslip line, see migration 00104's header.
type PendingBenefitDeduction struct {
	EnrollmentID string
	Description  string
	Amount       decimal.Decimal
}

// BenefitsSource supplies an employee's active benefit enrollment costs for
// a payroll period. Like reimbursements, this recurs every period an
// enrollment is active — there is nothing to "mark paid" the way a one-time
// bonus or reimbursement has, so no second method is needed.
//
// Declared here for the same reason as the other three sources: payslips is
// the CONSUMER, hrm/benefits imports hrm/payslips to satisfy it.
type BenefitsSource interface {
	PendingDeductionsForEmployee(ctx context.Context, orgID, employeeID string, year, month int) ([]PendingBenefitDeduction, error)
}

// ── F&F settlement (Phase 9B) ────────────────────────────────────────────────

// SettlementLine is one F&F credit or debit, as reported by an FnFSource.
//
// Amount is ALWAYS POSITIVE; direction lives in IsCredit. Storing debits as
// negatives means every reader has to know the sign convention, and the first
// one who does not produces a settlement that adds up backwards.
type SettlementLine struct {
	// SourceType names the origin: leave_encashment, gratuity,
	// notice_shortfall, loan_foreclosure, travel_advance, clearance_due.
	SourceType  string
	SourceID    string
	Description string
	Amount      decimal.Decimal
	IsCredit    bool
}

// FnFSettlement is everything an F&F run needs that ordinary payroll does not
// already know: WHO is being settled, and what one-off credits and debits
// apply on top of their final prorated salary.
type FnFSettlement struct {
	ExitID     string
	EmployeeID string
	Lines      []SettlementLine
}

// AppliedSettlementLine reports which settlement lines actually became
// payslip lines, so the provider can mark their sources consumed — a loan
// foreclosed, an advance recovered.
type AppliedSettlementLine struct {
	SourceType string
	SourceID   string
	LineID     string
	Amount     decimal.Decimal
}

// FnFSource supplies the exit-specific half of a run_type='fnf' run.
//
// ⚠ F&F IS THE 'ADDS-ON' SHAPE, NOT 'REPLACES'. computeBonusPayslips replaces
// the salary computation because a bonus run must not pay regular salary. An
// F&F run MUST pay prorated final salary — it is the largest credit in most
// settlements — so the ordinary per-employee computation runs unchanged with
// the EMPLOYEE SET narrowed to the leaver, and these lines are appended
// exactly as loan, reimbursement, statutory and benefit lines already are.
//
// Declared here for the same reason as the other five sources: payslips is
// the CONSUMER, hrm/exits imports hrm/payslips to satisfy it, never the
// reverse. A nil FnFSource makes an F&F run compute to nothing rather than
// panicking — the established nil-source precedent.
type FnFSource interface {
	// SettlementForRun answers which employee this run settles and what extra
	// credits and debits apply. A nil result means no exit is attached to the
	// run, which the caller reports rather than silently computing an empty
	// settlement — an F&F run that pays nobody is always a mistake.
	SettlementForRun(ctx context.Context, orgID, runID string) (*FnFSettlement, error)
	// MarkSettled is called ONLY after every payslip and line in the run has
	// been persisted successfully — never from inside abortCompute. Same
	// contract as BonusSource.MarkBonusesPaid and for the same reason: a loan
	// marked foreclosed with no payslip behind it is money written off that
	// nobody ever collected.
	MarkSettled(ctx context.Context, runID string, applied []AppliedSettlementLine) error
	// ClearanceComplete reports whether every blocking clearance item on the
	// attached exit is resolved. Checked at APPROVAL, never at computation:
	// HR must be able to compute a draft and see what a leaver will actually
	// receive while clearance is still open. Locking earlier leaves them with
	// no way to answer that — 8B's ApproveLine lesson.
	ClearanceComplete(ctx context.Context, orgID, runID string) (bool, error)
}

var (
	ErrNotFound               = errors.New("payslip run not found")
	ErrPayslipNotFound        = errors.New("payslip not found")
	ErrYearRequired           = errors.New("year is required")
	ErrMonthRequired          = errors.New("month is required (1-12)")
	ErrInvalidMonth           = errors.New("month must be between 1 and 12")
	ErrDuplicateRun           = errors.New("payslip run already exists for this period")
	ErrAttendanceNotFinalized = errors.New("attendance period must be finalized before computing payroll")
	ErrWrongStatus            = errors.New("action not allowed in current payslip run status")
	ErrAlreadyComputed        = errors.New("payslip run has already been computed")
	ErrNotComputed            = errors.New("payslip run must be computed before approving")
	ErrNotApproved            = errors.New("payslip run must be approved before marking as paid")
	// ErrNegativeNetPay blocks approval of a run containing a payslip whose
	// deductions exceed its gross. ComputeRun records the real figure rather
	// than clamping it to zero, so the shortfall is visible and fixable; this
	// is what stops it being paid out.
	ErrInvalidRunType = errors.New("run_type must be one of regular, off_cycle, bonus, arrears, fnf")
	ErrNegativeNetPay = errors.New("payroll run contains payslips with negative net pay and cannot be approved")

	// ErrNoExitForFnFRun fires when a run_type='fnf' run has no exit record
	// attached. Reported rather than computed as an empty run: an F&F run
	// that settles nobody is always a mistake, and silently producing zero
	// payslips would look like a successful settlement.
	ErrNoExitForFnFRun = errors.New("this full & final run has no exit record attached")
	// ErrFnFEmployeeNotFound fires when the exit names an employee who does
	// not exist in this org — the FK-free source_id's failure mode.
	ErrFnFEmployeeNotFound = errors.New("the employee this settlement names was not found")
	// ErrClearancePending blocks APPROVAL of an F&F run whose exit still has
	// unresolved blocking clearance items. Deliberately not checked at
	// computation: HR must be able to see the settlement figure while
	// clearance is still open, and locking earlier leaves them with no way to
	// answer "what will I actually receive" — 8B's ApproveLine lesson.
	ErrClearancePending = errors.New("clearance is incomplete: resolve all blocking items before approving the settlement")
)
