// backend/internal/hrm/payslips/service.go
package payslips

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/shopspring/decimal"

	"github.com/expr-lang/expr"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	hrmsalary "github.com/mridha/businesssaas/internal/hrm/salary"
)

// Service defines business logic for the payroll engine.
type Service interface {
	// SetBaseCurrencySource attaches the legal-entity layer so a run's
	// currency is resolved rather than assumed (11B-2).
	SetBaseCurrencySource(src BaseCurrencySource)

	ListRuns(ctx context.Context, orgID string) (*RunListResponse, error)
	GetRun(ctx context.Context, orgID, ref string) (*PayslipRun, error)
	CreateRun(ctx context.Context, orgID, createdBy string, req CreateRunRequest) (*PayslipRun, error)
	// ComputeRun runs the payroll formula engine for all employees in the org.
	// Prerequisite: attendance_period must be finalized (if attendance_period_id is set).
	ComputeRun(ctx context.Context, orgID, ref, computedBy string) (*PayslipRun, error)
	// PreviewRun is the mandatory dry run — same arithmetic as ComputeRun,
	// persisting nothing.
	PreviewRun(ctx context.Context, orgID, ref string) (*RunPreview, error)
	ApproveRun(ctx context.Context, orgID, ref, approvedBy string) (*PayslipRun, error)
	MarkPaid(ctx context.Context, orgID, ref, paidBy string) (*PayslipRun, error)
	CancelRun(ctx context.Context, orgID, ref string) (*PayslipRun, error)
	ListPayslips(ctx context.Context, orgID string, filter SlipListFilter) (*SlipListResponse, error)
	GetPayslip(ctx context.Context, orgID, ref string) (*Payslip, error)
}

type serviceImpl struct {
	repo Repository
	db   *pgxpool.Pool
	// bonusSource feeds a run_type='bonus' run its lines. Nil is valid — see
	// BonusSource's doc comment in model.go.
	bonusSource BonusSource
	// loanSource / reimbursementSource / statutorySource / benefitsSource feed
	// their respective lines into every OTHER run type's normal per-employee
	// computation. Nil is valid for all four — see their doc comments in
	// model.go.
	loanSource          LoanSource
	reimbursementSource ReimbursementSource
	statutorySource     StatutorySource
	benefitsSource      BenefitsSource
	// fnfSource feeds a run_type='fnf' run its exit-specific credits and
	// debits, and tells it WHICH employee is being settled. Nil is valid —
	// see FnFSource's doc comment in model.go.
	fnfSource FnFSource
	// entityCurrency is OPTIONAL (11B-2). Nil keeps the historical BDT
	// default for a run whose currency the caller did not name.
	entityCurrency BaseCurrencySource
}

func NewService(
	repo Repository, db *pgxpool.Pool,
	bonusSource BonusSource, loanSource LoanSource, reimbursementSource ReimbursementSource,
	statutorySource StatutorySource, benefitsSource BenefitsSource,
	fnfSource FnFSource,
) Service {
	return &serviceImpl{
		repo: repo, db: db, bonusSource: bonusSource,
		statutorySource: statutorySource, benefitsSource: benefitsSource,
		loanSource: loanSource, reimbursementSource: reimbursementSource,
		fnfSource: fnfSource,
	}
}

func (s *serviceImpl) ListRuns(ctx context.Context, orgID string) (*RunListResponse, error) {
	list, err := s.repo.FindRuns(ctx, orgID)
	if err != nil {
		return nil, fmt.Errorf("payslips: ListRuns: %w", err)
	}
	if list == nil {
		list = []*PayslipRun{}
	}
	return &RunListResponse{Runs: list, Total: len(list)}, nil
}

func (s *serviceImpl) GetRun(ctx context.Context, orgID, ref string) (*PayslipRun, error) {
	r, err := s.repo.FindRunByRef(ctx, orgID, ref)
	if err != nil {
		return nil, fmt.Errorf("payslips: GetRun: %w", err)
	}
	if r == nil {
		return nil, ErrNotFound
	}
	return r, nil
}

func (s *serviceImpl) CreateRun(ctx context.Context, orgID, createdBy string, req CreateRunRequest) (*PayslipRun, error) {
	if req.Year == 0 {
		return nil, ErrYearRequired
	}
	if req.Month == 0 {
		return nil, ErrMonthRequired
	}
	if req.Month < 1 || req.Month > 12 {
		return nil, ErrInvalidMonth
	}

	runType := RunTypeRegular
	if req.RunType != nil && strings.TrimSpace(*req.RunType) != "" {
		runType = RunType(strings.TrimSpace(*req.RunType))
		if !runType.IsValid() {
			return nil, ErrInvalidRunType
		}
	}

	// Only a regular run is capped at one per org per month. Off-cycle, bonus,
	// arrears and FnF runs are legitimately repeatable within a period — a
	// leaver's final settlement cannot wait for next month. The partial index
	// uq_hrm_pr_org_month_regular is the guarantee; this is the friendly
	// message, and both read RunType.IsUniquePerPeriod so they cannot drift.
	if runType.IsUniquePerPeriod() {
		existing, err := s.repo.FindRunByPeriod(ctx, orgID, req.Year, req.Month, runType, req.LegalEntityID)
		if err != nil {
			return nil, fmt.Errorf("payslips: CreateRun: check existing: %w", err)
		}
		if existing != nil {
			return nil, ErrDuplicateRun
		}
	}

	// ⚠ THE CURRENCY IS RESOLVED, NOT HARDCODED (11B-2). This read
	// `currency := "BDT"` since Phase 7, so a US organization creating a run
	// without naming a currency got Bangladeshi Taka on every payslip.
	//
	// Resolution order: what the caller asked for → a currency a LEGAL ENTITY
	// declared → BDT as the last resort.
	//
	// ⚠ organizations.currency is deliberately NOT in that chain. It is
	// NOT NULL DEFAULT 'USD', so every organization carries USD whether or
	// not anyone chose it, and reading it here would silently relabel every
	// existing organization's payslips from BDT to USD. An org fixes its
	// currency by declaring a legal entity, which is an act rather than a
	// default. See entities.DeclaredCurrency.
	currency := ""
	if req.Currency != nil && strings.TrimSpace(*req.Currency) != "" {
		currency = strings.ToUpper(strings.TrimSpace(*req.Currency))
	}
	if currency == "" && s.entityCurrency != nil {
		resolved, err := s.entityCurrency.DeclaredCurrency(ctx, orgID, req.LegalEntityID)
		if err != nil {
			return nil, fmt.Errorf("payslips: CreateRun: resolve currency: %w", err)
		}
		currency = strings.ToUpper(strings.TrimSpace(resolved))
	}
	if currency == "" {
		currency = "BDT"
	}

	run := &PayslipRun{
		OrgID: orgID, PeriodYear: req.Year, PeriodMonth: req.Month,
		RunType:     runType,
		Description: req.Description, Currency: currency,
		LegalEntityID:      req.LegalEntityID,
		AttendancePeriodID: req.AttendancePeriodID,
		Status:             RunDraft, CreatedBy: createdBy,
	}
	if err := s.repo.CreateRun(ctx, run); err != nil {
		if strings.Contains(err.Error(), "unique") {
			return nil, ErrDuplicateRun
		}
		return nil, fmt.Errorf("payslips: CreateRun: %w", err)
	}
	return run, nil
}

// ComputeRun is the core payroll engine.
// For each active employee:
//  1. Load salary structure + components (ordered by display_order)
//  2. Get basic_pay from hrm_employee_salary_records
//  3. Get attendance summary for the period (from D1 if linked)
//  4. Compute each component using the A1 formula engine
//  5. Insert payslip + lines
//
// ComputeRun computes a run and PERSISTS the result, moving it to 'computed'.
func (s *serviceImpl) ComputeRun(ctx context.Context, orgID, ref, computedBy string) (*PayslipRun, error) {
	run, err := s.loadRunForCompute(ctx, orgID, ref)
	if err != nil {
		return nil, err
	}

	// Mark as computing to prevent concurrent runs.
	run.Status = RunComputing
	if err := s.repo.UpdateRun(ctx, run); err != nil {
		return nil, fmt.Errorf("payslips: ComputeRun: mark computing: %w", err)
	}

	results, err := s.computePayslips(ctx, orgID, run)
	if err != nil {
		return nil, s.abortCompute(ctx, run, err)
	}

	// Persistence failures abort the whole run. Previously a failed
	// CreatePayslip was answered with `continue` and a failed
	// CreatePayslipLines with `_ =`, which meant an employee could silently
	// end up unpaid, or paid with a gross and a net but no lines explaining
	// either. The run's own header hid it: TotalEmployees counted every
	// employee while the money totals counted only the ones that saved, so a
	// partially-written run looked complete and merely disagreed with itself.
	//
	// A payroll run is all-or-nothing. Anything less than every payslip
	// written is a failed run, and the caller has to be told.
	var totalGross, totalDeductions, totalNet decimal.Decimal
	var paidBonusLines []PaidBonusLine
	var recoveryApplications []RecoveryApplication
	var paidReimbursementLines []PaidReimbursementLine
	var appliedSettlementLines []AppliedSettlementLine
	for _, res := range results {
		if err := s.repo.CreatePayslip(ctx, res.Slip); err != nil {
			return nil, s.abortCompute(ctx, run,
				fmt.Errorf("payslips: ComputeRun: create payslip for employee %s: %w",
					res.Slip.EmployeeID, err))
		}
		for _, l := range res.Lines {
			l.PayslipID = res.Slip.ID
		}
		if len(res.Lines) > 0 {
			if err := s.repo.CreatePayslipLines(ctx, res.Lines); err != nil {
				return nil, s.abortCompute(ctx, run,
					fmt.Errorf("payslips: ComputeRun: create lines for employee %s: %w",
						res.Slip.EmployeeID, err))
			}
		}
		// SourceBonusIDs[i] names the bonus that produced res.Lines[i] — the
		// two slices are built in lockstep by computeBonusPayslips, and
		// res.Lines[i].ID is only real now that CreatePayslipLines has run.
		for i, bonusID := range res.SourceBonusIDs {
			paidBonusLines = append(paidBonusLines, PaidBonusLine{BonusID: bonusID, LineID: res.Lines[i].ID})
		}
		// Loan recovery / reimbursement lines carry their own correlation
		// directly on the line (set in computePayslips) — read it now that
		// CreatePayslipLines has assigned the real ID onto the same pointer.
		for _, l := range res.Lines {
			if l.sourceLoanScheduleID != "" {
				recoveryApplications = append(recoveryApplications, RecoveryApplication{
					ScheduleID: l.sourceLoanScheduleID, LineID: l.ID, AmountApplied: l.ComputedAmount,
				})
			}
			if l.sourceReimbursementID != "" {
				paidReimbursementLines = append(paidReimbursementLines, PaidReimbursementLine{
					ReimbursementID: l.sourceReimbursementID, LineID: l.ID,
				})
			}
			if l.sourceSettlementType != "" {
				appliedSettlementLines = append(appliedSettlementLines, AppliedSettlementLine{
					SourceType: l.sourceSettlementType, SourceID: l.sourceSettlementID,
					LineID: l.ID, Amount: l.ComputedAmount,
				})
			}
		}
		totalGross = totalGross.Add(res.Slip.GrossPay)
		totalDeductions = totalDeductions.Add(res.Slip.TotalDeductions)
		totalNet = totalNet.Add(res.Slip.NetPay)
	}

	// Bookkeeping the bonus/loan/reimbursement engines need, all treated with
	// the same all-or-nothing discipline as the payslip writes above: if any
	// fails, the run aborts and the underlying records stay exactly as they
	// were for the next attempt, rather than leaving payslips committed with
	// money payable or recoverable a second time.
	if len(paidBonusLines) > 0 && s.bonusSource != nil {
		if err := s.bonusSource.MarkBonusesPaid(ctx, run.ID, paidBonusLines); err != nil {
			return nil, s.abortCompute(ctx, run,
				fmt.Errorf("payslips: ComputeRun: mark bonuses paid: %w", err))
		}
	}
	if len(recoveryApplications) > 0 && s.loanSource != nil {
		if err := s.loanSource.RecordRecoveries(ctx, run.ID, recoveryApplications); err != nil {
			return nil, s.abortCompute(ctx, run,
				fmt.Errorf("payslips: ComputeRun: record loan recoveries: %w", err))
		}
	}
	if len(paidReimbursementLines) > 0 && s.reimbursementSource != nil {
		if err := s.reimbursementSource.MarkReimbursementsPaid(ctx, run.ID, paidReimbursementLines); err != nil {
			return nil, s.abortCompute(ctx, run,
				fmt.Errorf("payslips: ComputeRun: mark reimbursements paid: %w", err))
		}
	}
	// Same all-or-nothing discipline: a loan marked foreclosed or an advance
	// marked recovered with no payslip behind it is money written off that
	// nobody ever collected.
	if len(appliedSettlementLines) > 0 && s.fnfSource != nil {
		if err := s.fnfSource.MarkSettled(ctx, run.ID, appliedSettlementLines); err != nil {
			return nil, s.abortCompute(ctx, run,
				fmt.Errorf("payslips: ComputeRun: mark settlement applied: %w", err))
		}
	}

	now := time.Now()
	run.Status = RunComputed
	run.TotalEmployees = len(results)
	run.TotalGrossPay = totalGross
	run.TotalDeductions = totalDeductions
	run.TotalNetPay = totalNet
	run.ComputedAt = &now
	run.ComputedBy = &computedBy
	if err := s.repo.UpdateRun(ctx, run); err != nil {
		return nil, fmt.Errorf("payslips: ComputeRun: finalize run: %w", err)
	}
	return run, nil
}

// PreviewRun is the mandatory dry run. It computes exactly what ComputeRun
// would and PERSISTS NOTHING — no payslips, no lines, no status change.
//
// It shares computePayslips with the real thing rather than reimplementing a
// "preview mode", so a preview cannot disagree with what approval later
// commits. It is also the only compute path that may run against an
// already-computed run: checking the numbers before approving is the entire
// point, and re-previewing must never disturb what is on record.
func (s *serviceImpl) PreviewRun(ctx context.Context, orgID, ref string) (*RunPreview, error) {
	run, err := s.repo.FindRunByRef(ctx, orgID, ref)
	if err != nil {
		return nil, fmt.Errorf("payslips: PreviewRun: %w", err)
	}
	if run == nil {
		return nil, ErrNotFound
	}
	if run.Status == RunCancelled {
		return nil, ErrWrongStatus
	}
	if err := s.assertAttendanceReady(ctx, run); err != nil {
		return nil, err
	}

	results, err := s.computePayslips(ctx, orgID, run)
	if err != nil {
		return nil, err
	}

	preview := &RunPreview{
		RunID: run.ID, RunType: run.RunType,
		PeriodYear: run.PeriodYear, PeriodMonth: run.PeriodMonth,
		Currency: run.Currency, Payslips: make([]*Payslip, 0, len(results)),
	}
	for _, res := range results {
		preview.TotalGrossPay = preview.TotalGrossPay.Add(res.Slip.GrossPay)
		preview.TotalDeductions = preview.TotalDeductions.Add(res.Slip.TotalDeductions)
		preview.TotalNetPay = preview.TotalNetPay.Add(res.Slip.NetPay)
		// Surfacing this is the reason a dry run exists: a negative net blocks
		// approval, and finding that out here beats finding out at approval.
		if res.Slip.NetPay.IsNegative() {
			preview.NegativeNetCount++
		}
		preview.Payslips = append(preview.Payslips, res.Slip)
	}
	preview.TotalEmployees = len(results)
	return preview, nil
}

// abortCompute unwinds a failed computation and returns the causing error.
//
// Two things must happen, in this order. Any payslips already written are
// removed, because an aborted run returns to 'draft' and may be recomputed —
// leaving them would give the retry a second set of payslips alongside the
// first. Then the run comes out of 'computing', which it can otherwise never
// leave: every entry point refuses that status, so a run stranded there is
// dead and its period can never be paid.
//
// Cleanup failures are joined onto the original error rather than replacing
// it. The first error is why the run failed; a failed cleanup is a second
// problem, and losing either one costs someone their pay.
func (s *serviceImpl) abortCompute(ctx context.Context, run *PayslipRun, cause error) error {
	if delErr := s.repo.DeletePayslipsByRun(ctx, run.ID); delErr != nil {
		cause = errors.Join(cause, fmt.Errorf(
			"payslips: abortCompute: run %s left with partial payslips: %w", run.ID, delErr))
	}
	run.Status = RunDraft
	if updErr := s.repo.UpdateRun(ctx, run); updErr != nil {
		cause = errors.Join(cause, fmt.Errorf(
			"payslips: abortCompute: run %s stranded in computing: %w", run.ID, updErr))
	}
	return cause
}

// loadRunForCompute resolves a run and refuses the states a fresh computation
// must not overwrite.
func (s *serviceImpl) loadRunForCompute(ctx context.Context, orgID, ref string) (*PayslipRun, error) {
	run, err := s.repo.FindRunByRef(ctx, orgID, ref)
	if err != nil {
		return nil, fmt.Errorf("payslips: ComputeRun: %w", err)
	}
	if run == nil {
		return nil, ErrNotFound
	}
	if run.Status == RunComputed || run.Status == RunApproved || run.Status == RunPaid {
		return nil, ErrAlreadyComputed
	}
	if run.Status == RunCancelled {
		return nil, ErrWrongStatus
	}
	if err := s.assertAttendanceReady(ctx, run); err != nil {
		return nil, err
	}
	return run, nil
}

// assertAttendanceReady enforces the D1 dependency: a linked attendance period
// must be finalized before its numbers drive anyone's pay.
func (s *serviceImpl) assertAttendanceReady(ctx context.Context, run *PayslipRun) error {
	if run.AttendancePeriodID == nil {
		return nil
	}
	var attStatus string
	if err := s.db.QueryRow(ctx,
		`SELECT status FROM hrm_attendance_periods WHERE id=$1::uuid`,
		*run.AttendancePeriodID).Scan(&attStatus); err == nil {
		if attStatus != "finalized" && attStatus != "locked" {
			return ErrAttendanceNotFinalized
		}
	}
	return nil
}

// computedPayslip is one employee's result before anything is written.
// Preview returns these directly; ComputeRun persists them.
type computedPayslip struct {
	Slip  *Payslip
	Lines []*PayslipLine
	// SourceBonusIDs is set only by computeBonusPayslips, one entry per
	// entry in Lines at the SAME index — ComputeRun zips them together after
	// CreatePayslipLines assigns real line IDs, to call
	// bonusSource.MarkBonusesPaid. Empty for every other run type.
	SourceBonusIDs []string
}

// computePayslips runs the whole engine for a run and returns the results
// WITHOUT persisting anything and WITHOUT touching the run's status.
//
// Splitting it out is what makes a dry run possible: preview and the real
// compute share this exact arithmetic, so a preview cannot disagree with what
// approval would later commit. Duplicating the calculation for a "preview
// mode" would have created two engines that drift.
// empRow is one employee's payroll inputs. Package-level rather than local to
// computePayslips because the F&F path (loadFnFEmployee) produces the same
// shape by a different query, and the per-employee computation downstream must
// not be able to tell which produced it.
type empRow struct {
	ID            string
	HireDate      string
	StructureID   *string
	StructureName *string
	BasicPay      decimal.Decimal
}

func (s *serviceImpl) computePayslips(ctx context.Context, orgID string, run *PayslipRun) ([]computedPayslip, error) {
	// A bonus run pays ONLY approved bonuses for its period — running the
	// normal salary-structure computation here would double-pay everyone's
	// regular basic pay alongside their bonus. See computeBonusPayslips.
	if run.RunType == RunTypeBonus {
		return s.computeBonusPayslips(ctx, orgID, run)
	}

	// Fetch rounding policy
	var roundingScale int32 = 2
	var roundingMode string = "half_up"
	_ = s.db.QueryRow(ctx, `SELECT money_rounding_scale, money_rounding_mode FROM organizations WHERE id=$1::uuid`, orgID).Scan(&roundingScale, &roundingMode)

	// Load active employees (empRow is package-level — see its declaration —
	// so the F&F path can produce the same shape).
	// Who gets paid.
	//
	// This filter previously read `e.status IN ('active','resigned')`, which was
	// broken twice over. hrm_employees.status was dropped by migration 00053 in
	// favour of status_id, so the query failed 42703 and payroll could not
	// compute AT ALL. And 'resigned' was never a valid value even on the
	// original 00021 CHECK ('active','inactive','on_leave','terminated') — it
	// only became a real status NAME (inside the 'terminated' category) when
	// 00053 seeded it. So the old filter matched active employees at best.
	//
	// The rule now, stated explicitly because it decides who gets money:
	//   • active and on_leave employees are paid — being on leave does not stop
	//     pay, and treating those categories differently silently unpays people.
	//   • a leaver is paid when their termination_date falls on or after the
	//     period start, because they worked part of the period and are owed it.
	//     Dropping them is how a mid-month resignation silently goes unpaid.
	// Category, not status name: names are org-customisable, categories are the
	// fixed CHECK-constrained vocabulary, so this cannot be broken by a rename.
	rows, err := s.db.Query(ctx,
		`SELECT e.id::text, COALESCE(to_char(e.hire_date,'YYYY-MM-DD'),''),
		es.structure_id::text, ess.name, COALESCE(es.basic_pay,0)
		FROM hrm_employees e
		JOIN hrm_employee_statuses est ON est.id = e.status_id
		LEFT JOIN LATERAL (
		    SELECT structure_id, basic_pay FROM hrm_employee_salary_records
		    WHERE employee_id=e.id AND effective_date <= make_date($2,$3,1)
		    ORDER BY effective_date DESC LIMIT 1
		) es ON TRUE
		LEFT JOIN hrm_salary_structures ess ON ess.id=es.structure_id
		WHERE e.org_id=$1
		  AND (
		      est.category IN ('active','on_leave')
		      OR (est.category = 'terminated'
		          AND e.termination_date IS NOT NULL
		          AND e.termination_date >= make_date($2,$3,1))
		  )
		  -- ⚠ Entity narrowing (11B-2). $4 NULL means the WHOLE
		  -- ORGANIZATION, which is every run that already exists — the
		  -- predicate short-circuits to TRUE and the query is unchanged.
		  -- Writing this as a plain equality would have emptied the payroll
		  -- run of every organization in this database, none of which has
		  -- entities.
		  AND ($4::uuid IS NULL OR e.legal_entity_id = $4::uuid)`,
		orgID, run.PeriodYear, run.PeriodMonth, run.LegalEntityID)
	if err != nil {
		return nil, fmt.Errorf("payslips: computePayslips: load employees: %w", err)
	}
	defer rows.Close()

	var employees []empRow
	for rows.Next() {
		var e empRow
		if err := rows.Scan(&e.ID, &e.HireDate, &e.StructureID, &e.StructureName, &e.BasicPay); err != nil {
			continue
		}
		employees = append(employees, e)
	}

	// ── F&F: narrow the employee set to the one leaver being settled ──
	//
	// An F&F run settles ONE person, and it must ignore the org-wide
	// eligibility filter above entirely. That filter pays a terminated
	// employee only when their termination_date falls on or after the period
	// start — which is exactly the person an F&F run exists to pay, months
	// after they left. Running the filter for a settlement would silently
	// produce an empty run.
	//
	// Everything after this point is unchanged: the ordinary per-employee
	// computation gives the leaver their prorated final salary, statutory
	// deductions, benefits and reimbursements, and the settlement's own
	// credits and debits are appended alongside — the ADDS-ON shape loans and
	// reimbursements already use, NOT the REPLACES shape bonus uses.
	var fnfSettlement *FnFSettlement
	if run.RunType == RunTypeFnF {
		if s.fnfSource == nil {
			// No exit engine wired — an F&F run computes to nothing rather
			// than panicking. See FnFSource's doc comment in model.go.
			return nil, nil
		}
		fnfSettlement, err = s.fnfSource.SettlementForRun(ctx, orgID, run.ID)
		if err != nil {
			return nil, fmt.Errorf("payslips: computePayslips: fnf settlement: %w", err)
		}
		if fnfSettlement == nil {
			return nil, ErrNoExitForFnFRun
		}
		employees, err = s.loadFnFEmployee(ctx, orgID, run, fnfSettlement.EmployeeID)
		if err != nil {
			return nil, err
		}
	}

	results := make([]computedPayslip, 0, len(employees))

	for _, emp := range employees {
		// Compute tenure
		tenureYears := 0.0
		if emp.HireDate != "" {
			if hd, err := time.Parse("2006-01-02", emp.HireDate); err == nil {
				tenureYears = time.Since(hd).Hours() / (24 * 365.25)
			}
		}

		// Get attendance summary
		presentDays, absentDays, leaveDays, holidayDays := 0, 0, 0, 0
		var otHours float64
		workDays := 0 // org-level working days (from attendance period if available)
		if run.AttendancePeriodID != nil {
			_ = s.db.QueryRow(ctx,
				`SELECT
				COUNT(*) FILTER (WHERE day_type IN ('present','late','half_day','work_from_home')),
				COUNT(*) FILTER (WHERE day_type='absent'),
				COUNT(*) FILTER (WHERE day_type='on_leave'),
				COUNT(*) FILTER (WHERE day_type='holiday'),
				COALESCE(SUM(overtime_hours),0)
				FROM hrm_attendance_records
				WHERE org_id=$1 AND employee_id=$2::uuid AND status='approved'
				AND EXTRACT(YEAR FROM attendance_date)=$3 AND EXTRACT(MONTH FROM attendance_date)=$4`,
				orgID, emp.ID, run.PeriodYear, run.PeriodMonth,
			).Scan(&presentDays, &absentDays, &leaveDays, &holidayDays, &otHours)
			_ = s.db.QueryRow(ctx, `SELECT total_work_days FROM hrm_attendance_periods WHERE id=$1::uuid`, *run.AttendancePeriodID).Scan(&workDays)
		}

		// Load salary structure components
		type compRow struct {
			CompID        *string
			CompName      string
			CompType      string
			CalcMethod    string
			FixedValue    decimal.Decimal
			OverrideValue *decimal.Decimal
			Formula       *string
			SlabConfigRaw []byte
			DisplayOrder  int
			// IsTaxable feeds TAXABLE_GROSS — the sum of only the earning
			// components flagged taxable (00023), which 7D's statutory engine
			// reads. Nothing needed this distinction before 7D, which is why
			// it was not already tracked here.
			IsTaxable bool
		}
		var components []compRow
		if emp.StructureID != nil {
			cRows, err := s.db.Query(ctx,
				`SELECT c.id::text, c.name, c.component_type, c.calc_method, c.fixed_value,
				sc.override_value, c.formula_expression, c.slab_config, sc.display_order, c.is_taxable
				FROM hrm_salary_structure_components sc
				JOIN hrm_salary_components c ON c.id=sc.component_id
				WHERE sc.structure_id=$1::uuid
				ORDER BY sc.display_order`,
				*emp.StructureID)
			if err == nil {
				defer cRows.Close()
				for cRows.Next() {
					var cr compRow
					_ = cRows.Scan(&cr.CompID, &cr.CompName, &cr.CompType, &cr.CalcMethod, &cr.FixedValue, &cr.OverrideValue, &cr.Formula, &cr.SlabConfigRaw, &cr.DisplayOrder, &cr.IsTaxable)
					components = append(components, cr)
				}
			}
		}

		// Formula evaluation environment (matches A1 formula_variables documentation)
		env := map[string]interface{}{
			"BASIC":        emp.BasicPay.InexactFloat64(),
			"GROSS":        0.0, // updated after each earning
			"PRESENT_DAYS": float64(presentDays),
			"WORK_DAYS":    float64(workDays),
			"TENURE_YEARS": tenureYears,
		}

		var lines []*PayslipLine
		gross, deductions := decimal.Zero, decimal.Zero

		// GROSS HAS ONE VALUE PER PAYSLIP, NOT A RUNNING TOTAL.
		//
		// This used to be a single loop that added each earning to `gross` as it
		// went, while pct_of_gross / formula / slab read that half-finished
		// figure. A component evaluated third saw only the first two components'
		// earnings, so REORDERING display_order SILENTLY CHANGED EVERYONE'S PAY —
		// and display_order is an ordinary admin-editable field.
		//
		// The fix is staged evaluation. Every component inside a stage sees the
		// same fixed inputs, so order within a stage cannot affect any result:
		//
		//   stage 1  earnings that do NOT reference GROSS  -> establishes gross
		//   stage 2  earnings that DO reference GROSS      -> added to gross
		//   -- statutory base and statutory land HERE in 7D --
		//   stage 3  deductions and employer contributions -> see the FINAL gross
		//   -- loan recovery lands HERE in 7C --
		//   net = gross - deductions
		//
		// That is the build plan's "earnings -> gross -> statutory base ->
		// statutory -> other deductions -> loan recovery -> net". The two
		// missing stages are no-ops today; because every stage reads only
		// inputs fixed before it runs, adding them cannot re-derive anything
		// computed above.
		//
		// Amounts are collected by component index and lines are emitted in the
		// original display order afterwards, so presentation order is unchanged.
		amounts := make([]decimal.Decimal, len(components))

		computeAmount := func(comp compRow, grossForCalc decimal.Decimal) decimal.Decimal {
			var amount decimal.Decimal
			effectiveFixed := comp.FixedValue
			if comp.OverrideValue != nil {
				effectiveFixed = *comp.OverrideValue
			}

			switch comp.CalcMethod {
			case "fixed":
				amount = effectiveFixed
			case "pct_of_basic":
				amount = emp.BasicPay.Mul(effectiveFixed).Div(decimal.NewFromInt(100))
			case "pct_of_gross":
				amount = grossForCalc.Mul(effectiveFixed).Div(decimal.NewFromInt(100))
			case "formula":
				if comp.Formula != nil {
					env["GROSS"] = grossForCalc.InexactFloat64()
					if result, err := s.evalFormula(*comp.Formula, env); err == nil {
						// ⚠ THE ONE REMAINING float64 BOUNDARY IN PAYROLL MONEY.
						// expr-lang evaluates user-authored formulas in float64
						// (expr.AsFloat64), so a formula component's result is
						// inexact before it ever gets here. Making formulas
						// exact means replacing the evaluator, which is its own
						// piece of work — this conversion is the single, named
						// place that imprecision enters, and it is rounded
						// immediately below.
						amount = decimal.NewFromFloat(result)
					}
				}
			case "slab":
				if comp.SlabConfigRaw != nil {
					var cfg hrmsalary.SlabConfig
					if err := json.Unmarshal(comp.SlabConfigRaw, &cfg); err == nil {
						// Resolved from the DECIMAL sources, never from the
						// float formula env. Routing the base through that map
						// is what made the arithmetic lossy before it even
						// reached ComputeSlab.
						if base, ok := slabBase(cfg.BaseVariable, emp.BasicPay, grossForCalc,
							presentDays, workDays, tenureYears); ok {
							amount = ComputeSlab(base, &cfg)
						}
					}
				}
			default: // manual — default 0; HR enters manually
				amount = decimal.Zero
			}

			// Negative amounts are NOT clamped. A formula that legitimately
			// produces a negative adjustment (a correction, a clawback) must
			// survive; zeroing it made the money disappear with no trace.
			return roundDecimal(amount, roundingScale, roundingMode)
		}

		// taxableGross sums only the earning components flagged is_taxable
		// (00023) — 7D's statutory engine reads this, never GrossPay itself,
		// because gross also includes non-taxable earnings.
		taxableGross := decimal.Zero

		// Stage 1 — earnings independent of gross. These define gross.
		for i, comp := range components {
			if comp.CompType != "earning" || ReferencesGross(comp.CalcMethod, comp.Formula, comp.SlabConfigRaw) {
				continue
			}
			amounts[i] = computeAmount(comp, decimal.Zero)
			gross = gross.Add(amounts[i])
			if comp.IsTaxable {
				taxableGross = taxableGross.Add(amounts[i])
			}
		}

		// Stage 2 — earnings expressed as a share of gross. Each is evaluated
		// against the stage-1 total, so two such components cannot influence
		// each other regardless of their order.
		stageOneGross := gross
		for i, comp := range components {
			if comp.CompType != "earning" || !ReferencesGross(comp.CalcMethod, comp.Formula, comp.SlabConfigRaw) {
				continue
			}
			amounts[i] = computeAmount(comp, stageOneGross)
			gross = gross.Add(amounts[i])
			if comp.IsTaxable {
				taxableGross = taxableGross.Add(amounts[i])
			}
		}

		// Stage 3 — everything else, against the FINAL gross.
		for i, comp := range components {
			if comp.CompType == "earning" {
				continue
			}
			amounts[i] = computeAmount(comp, gross)
			if comp.CompType == "deduction" {
				deductions = deductions.Add(amounts[i])
			}
			// employer_contribution is an employer cost: it is recorded as a
			// line but affects neither gross nor the employee's deductions.
		}

		// Emit lines in the original display order.
		for i, comp := range components {
			var compIDStr *string
			if comp.CompID != nil {
				compIDStr = comp.CompID
			}
			var formulaUsed *string
			if comp.Formula != nil {
				formulaUsed = comp.Formula
			}

			// line_type is what the line DOES; component_type stays as the
			// snapshot of what the component WAS. For component-derived lines
			// the two agree, and an employer contribution is recorded as an
			// earning-shaped line flagged separately. Lines with no component
			// behind them — loan recovery, statutory, arrears — are produced by
			// later slices and set their own type directly.
			lineType := LineEarning
			if comp.CompType == "deduction" {
				lineType = LineDeduction
			}

			lines = append(lines, &PayslipLine{
				OrgID:                  orgID,
				ComponentID:            compIDStr,
				ComponentName:          comp.CompName,
				ComponentType:          comp.CompType,
				CalcMethod:             comp.CalcMethod,
				FormulaUsed:            formulaUsed,
				ComputedAmount:         amounts[i],
				LineType:               lineType,
				IsEmployerContribution: comp.CompType == "employer_contribution",
				DisplayOrder:           comp.DisplayOrder,
			})
		}

		// ── Statutory (7D) ───────────────────────────────────────────────
		// Placed after the salary-structure deductions above (Stage 3) rather
		// than literally between "earnings→gross" and "other deductions" as
		// the build plan's prose ordering reads: statutory rules are computed
		// from a wholly separate table (hrm_statutory_rules), feed nothing
		// Stage 3 needs, and reordering Stage 3 itself — already covered by
		// r25's reordering-safety tests — was not worth the risk for a
		// placement that produces an identical total either way. It still
		// runs before reimbursements/loan recovery, matching "statutory
		// sits between other deductions and loan_recovery" (r27).
		statutoryTotal := decimal.Zero
		if s.statutorySource != nil {
			statLines, err := s.statutorySource.ComputeForEmployee(ctx, orgID, emp.ID, run.PeriodYear, run.PeriodMonth, gross, emp.BasicPay, taxableGross)
			if err != nil {
				return nil, fmt.Errorf("payslips: computePayslips: statutory for employee %s: %w", emp.ID, err)
			}
			for i, sl := range statLines {
				lines = append(lines, &PayslipLine{
					OrgID: orgID, ComponentName: sl.Description, ComponentType: "deduction",
					CalcMethod: "manual", ComputedAmount: sl.Amount,
					LineType: LineStatutory, IsEmployerContribution: sl.IsEmployerContribution,
					DisplayOrder: len(lines) + i + 1,
				})
				// An employer contribution is an employer cost: it is
				// recorded as a line but affects neither gross nor the
				// employee's deductions — the same rule Stage 3 already
				// applies to employer_contribution-typed components.
				if !sl.IsEmployerContribution {
					statutoryTotal = statutoryTotal.Add(sl.Amount)
				}
			}
		}
		deductions = deductions.Add(statutoryTotal)

		// ── Benefits (7D) ────────────────────────────────────────────────
		// The employee's own recurring per-period cost of their active
		// enrollments. The employer's share (employer_cost_snapshot) is
		// tracked on the enrollment but produces no line here — no consumer
		// reads an employer-cost payslip line today (migration 00104).
		if s.benefitsSource != nil {
			benefitLines, err := s.benefitsSource.PendingDeductionsForEmployee(ctx, orgID, emp.ID, run.PeriodYear, run.PeriodMonth)
			if err != nil {
				return nil, fmt.Errorf("payslips: computePayslips: benefits for employee %s: %w", emp.ID, err)
			}
			for i, bl := range benefitLines {
				lines = append(lines, &PayslipLine{
					OrgID: orgID, ComponentName: bl.Description, ComponentType: "deduction",
					CalcMethod: "manual", ComputedAmount: bl.Amount,
					LineType: LineDeduction, DisplayOrder: len(lines) + i + 1,
				})
				deductions = deductions.Add(bl.Amount)
			}
		}

		// ── Reimbursements (7C) ────────────────────────────────────────────
		// Additive, and deliberately NOT folded into GrossPay: a
		// reimbursement repays an expense the employee already incurred out
		// of pocket, not earned income, so it should not inflate the figure
		// any future statutory (7D) engine would treat as taxable. It still
		// reduces TotalDeductions' counterpart headroom check below in the
		// right direction — more reimbursement means more room before a
		// loan recovery would drive net negative.
		reimbursementTotal := decimal.Zero
		if s.reimbursementSource != nil {
			pending, err := s.reimbursementSource.PendingForEmployee(ctx, orgID, emp.ID, run.PeriodYear, run.PeriodMonth)
			if err != nil {
				return nil, fmt.Errorf("payslips: computePayslips: reimbursements for employee %s: %w", emp.ID, err)
			}
			for i, r := range pending {
				reimbursementTotal = reimbursementTotal.Add(r.Amount)
				line := &PayslipLine{
					OrgID: orgID, ComponentName: r.Description, ComponentType: "earning",
					CalcMethod: "manual", ComputedAmount: r.Amount,
					LineType: LineReimbursement, DisplayOrder: len(lines) + i + 1,
				}
				line.sourceReimbursementID = r.ReimbursementID
				lines = append(lines, line)
			}
		}

		// ── Loan recovery (7C) — LAST stage before net, per 7A's ordering ──
		// Capped so recovery never drives net negative: headroom is net
		// BEFORE loan recovery (gross - deductions + reimbursements), and
		// each pending installment (oldest first) takes min(what it owes,
		// remaining headroom). A shortfall is simply not recovered this
		// run — the schedule row (hrm/loans) stays partially_recovered and
		// is picked up again next run, never written off silently.
		// ⚠ SKIPPED for an F&F run. A settlement forecloses the whole
		// outstanding balance as a single line (see FnFSource), and running
		// this as well would charge the installment due this period TWICE —
		// once here and once inside that balance. The headroom cap below is
		// also wrong for a settlement: it exists to stop recovery driving net
		// negative, which is precisely what an F&F run is allowed to do.
		loanRecoveryTotal := decimal.Zero
		if s.loanSource != nil && run.RunType != RunTypeFnF {
			pending, err := s.loanSource.PendingInstallmentsForEmployee(ctx, orgID, emp.ID, run.PeriodYear, run.PeriodMonth)
			if err != nil {
				return nil, fmt.Errorf("payslips: computePayslips: loan installments for employee %s: %w", emp.ID, err)
			}
			headroom := gross.Sub(deductions).Add(reimbursementTotal)
			for i, inst := range pending {
				if !headroom.IsPositive() {
					break
				}
				applied := inst.AmountDue
				if applied.GreaterThan(headroom) {
					applied = headroom
				}
				headroom = headroom.Sub(applied)
				loanRecoveryTotal = loanRecoveryTotal.Add(applied)
				line := &PayslipLine{
					OrgID: orgID, ComponentName: inst.Description, ComponentType: "deduction",
					CalcMethod: "manual", ComputedAmount: applied,
					LineType: LineLoanRecovery, DisplayOrder: len(lines) + i + 1,
				}
				line.sourceLoanScheduleID = inst.ScheduleID
				lines = append(lines, line)
			}
		}

		// ── F&F settlement lines ──
		//
		// Appended alongside loan recovery and reimbursements, in the same
		// additive shape, so a settlement is an ordinary payslip with extra
		// lines rather than a parallel calculator. Credits raise gross,
		// debits raise deductions, and BOTH are stored positive — direction
		// lives on the line's own type, never on the sign of the amount.
		fnfCreditTotal, fnfDebitTotal := decimal.Zero, decimal.Zero
		if fnfSettlement != nil {
			for _, sl := range fnfSettlement.Lines {
				if sl.Amount.IsZero() {
					// A zero line is noise on a document somebody has to read
					// and possibly dispute. Skipping it is not hiding
					// anything: hrm_exit_settlement_lines still records that
					// the source was evaluated and came to nothing.
					continue
				}
				lineType, componentType := LineDeduction, "deduction"
				if sl.IsCredit {
					lineType, componentType = LineEarning, "earning"
					fnfCreditTotal = fnfCreditTotal.Add(sl.Amount)
				} else {
					fnfDebitTotal = fnfDebitTotal.Add(sl.Amount)
				}
				line := &PayslipLine{
					OrgID: orgID, ComponentName: sl.Description, ComponentType: componentType,
					CalcMethod: "manual", ComputedAmount: sl.Amount,
					LineType: lineType, DisplayOrder: len(lines) + 1,
				}
				line.sourceSettlementType = sl.SourceType
				line.sourceSettlementID = sl.SourceID
				lines = append(lines, line)
			}
			gross = gross.Add(fnfCreditTotal)
		}

		// The true net, negative or not. Clamping this to zero was the worst of
		// the three defects: deductions exceeding gross produced a payslip
		// reporting net = 0 whose own line items did not add up, with nothing
		// anywhere recording that money had gone missing. ApproveRun is the
		// guard — it refuses to approve a run containing a negative payslip.
		// (Loan recovery above already keeps this run's OWN contribution from
		// causing that; a negative net from ordinary salary-structure
		// deductions alone is still exactly the case the guard exists for.)
		// fnfDebitTotal is NOT capped against available credits, unlike loan
		// recovery above. That is the point of an F&F run: what a leaver owes
		// can legitimately exceed what they are due, and the negative net
		// that results is a receivable to collect, not a data problem to
		// suppress. ApproveRun permits a negative net for run_type='fnf' and
		// for no other run type.
		totalDeductions := deductions.Add(loanRecoveryTotal).Add(fnfDebitTotal)
		netPay := gross.Sub(deductions).Add(reimbursementTotal).Sub(loanRecoveryTotal).Sub(fnfDebitTotal)

		// Create payslip
		slip := &Payslip{
			OrgID: orgID, EmployeeID: emp.ID,
			PayslipRunID: run.ID,
			PeriodYear:   run.PeriodYear, PeriodMonth: run.PeriodMonth,
			SalaryStructureID: emp.StructureID, SalaryStructureName: emp.StructureName,
			GrossPay: gross, TotalDeductions: totalDeductions, NetPay: netPay,
			BasicPay: emp.BasicPay,
			WorkDays: workDays, PresentDays: presentDays, AbsentDays: absentDays,
			LeaveDays: leaveDays, HolidayDays: holidayDays, OvertimeHours: otHours,
			Currency: run.Currency,
			Status:   SlipComputed,
		}
		results = append(results, computedPayslip{Slip: slip, Lines: lines})
	}

	return results, nil
}

// computeBonusPayslips builds one payslip per employee holding at least one
// approved-but-unpaid bonus for the run's period, one earning line per
// underlying bonus.
//
// Deduction-free by design, not by omission: statutory withholding on a
// bonus payout is Phase 7D scope, which does not exist yet. Paying a bonus
// net-of-nothing today is an honest description of what this run type does
// pending that engine, not a placeholder silently pretending otherwise —
// exactly the distinction r25's negative-net fix drew between a real
// zero and a masked one.
// loadFnFEmployee loads the single leaver an F&F run settles, BY ID and
// without the org-wide eligibility filter.
//
// The SELECT list mirrors the eligibility query exactly — same salary-record
// LATERAL, same COALESCE — so the per-employee computation downstream cannot
// tell which path produced its input. What it deliberately omits is the
// status/termination_date predicate: a settlement is run for somebody who has
// already left, which that predicate exists to exclude from ordinary payroll.
func (s *serviceImpl) loadFnFEmployee(ctx context.Context, orgID string, run *PayslipRun, employeeID string) ([]empRow, error) {
	var e empRow
	err := s.db.QueryRow(ctx,
		`SELECT e.id::text, COALESCE(to_char(e.hire_date,'YYYY-MM-DD'),''),
		 es.structure_id::text, ess.name, COALESCE(es.basic_pay,0)
		 FROM hrm_employees e
		 LEFT JOIN LATERAL (
		     SELECT structure_id, basic_pay FROM hrm_employee_salary_records
		     WHERE employee_id=e.id AND effective_date <= make_date($3,$4,1)
		     ORDER BY effective_date DESC LIMIT 1
		 ) es ON TRUE
		 LEFT JOIN hrm_salary_structures ess ON ess.id=es.structure_id
		 WHERE e.org_id=$1 AND e.id=$2::uuid`,
		orgID, employeeID, run.PeriodYear, run.PeriodMonth,
	).Scan(&e.ID, &e.HireDate, &e.StructureID, &e.StructureName, &e.BasicPay)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrFnFEmployeeNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("payslips: loadFnFEmployee: %w", err)
	}
	return []empRow{e}, nil
}

func (s *serviceImpl) computeBonusPayslips(ctx context.Context, orgID string, run *PayslipRun) ([]computedPayslip, error) {
	if s.bonusSource == nil {
		// No bonus engine wired — a bonus run computes to nothing, not a
		// panic. See BonusSource's doc comment in model.go.
		return nil, nil
	}
	pending, err := s.bonusSource.PendingBonusesForPeriod(ctx, orgID, run.PeriodYear, run.PeriodMonth)
	if err != nil {
		return nil, fmt.Errorf("payslips: computeBonusPayslips: %w", err)
	}

	byEmployee := make(map[string][]PendingBonus)
	order := make([]string, 0)
	for _, b := range pending {
		if _, seen := byEmployee[b.EmployeeID]; !seen {
			order = append(order, b.EmployeeID)
		}
		byEmployee[b.EmployeeID] = append(byEmployee[b.EmployeeID], b)
	}

	results := make([]computedPayslip, 0, len(order))
	for _, empID := range order {
		bonuses := byEmployee[empID]
		gross := decimal.Zero
		lines := make([]*PayslipLine, 0, len(bonuses))
		sourceIDs := make([]string, 0, len(bonuses))
		for i, b := range bonuses {
			gross = gross.Add(b.Amount)
			sourceIDs = append(sourceIDs, b.BonusID)
			lines = append(lines, &PayslipLine{
				OrgID: orgID, ComponentName: b.Description, ComponentType: "earning",
				CalcMethod: "manual", ComputedAmount: b.Amount,
				LineType: LineEarning, DisplayOrder: i + 1,
			})
		}
		slip := &Payslip{
			OrgID: orgID, EmployeeID: empID, PayslipRunID: run.ID,
			PeriodYear: run.PeriodYear, PeriodMonth: run.PeriodMonth,
			GrossPay: gross, TotalDeductions: decimal.Zero, NetPay: gross,
			BasicPay: decimal.Zero, Currency: run.Currency, Status: SlipComputed,
		}
		results = append(results, computedPayslip{Slip: slip, Lines: lines, SourceBonusIDs: sourceIDs})
	}
	return results, nil
}

// ComputeSlab evaluates a progressive (marginal) bracket calculation against
// base — the same model as progressive income tax, where each slab's rate
// applies only to the portion of base that falls within that bracket, not to
// the whole amount. This matches the design intent documented on
// hrm_salary_components.slab_config (migration 00023) and on
// salary.SlabConfig: "progressive bracket calculation (e.g. income tax)".
//
// Slabs are sorted ascending by UpTo before evaluation (nil sorts last, since
// it means "no upper bound") — slab validation at the Setup layer only
// guarantees exactly one nil UpTo as the final entry, not that the input
// slice itself is already in order.
//
// Exported (rather than a package-private helper) specifically so it can be
// unit tested directly — ComputeRun as a whole talks straight to *pgxpool.Pool
// and isn't unit-testable without a live database.
// slabBase resolves a slab table's base_variable to its exact decimal value.
//
// It mirrors the formula environment's variable set exactly, so a slab table
// that worked before still works — but it reads BASIC and GROSS from their
// decimal sources rather than from that float64 map. The day counts are whole
// numbers and exact either way; TENURE_YEARS is a float by nature and is the
// only variable here that cannot be represented exactly, which is fine because
// no money is denominated in it.
//
// An unrecognised variable returns false and the component computes nothing —
// the same outcome the old `env[name].(float64)` type assertion produced for a
// name that was not in the map.
func slabBase(name string, basic, gross decimal.Decimal, presentDays, workDays int, tenureYears float64) (decimal.Decimal, bool) {
	switch strings.ToUpper(strings.TrimSpace(name)) {
	case "BASIC":
		return basic, true
	case "GROSS":
		return gross, true
	case "PRESENT_DAYS":
		return decimal.NewFromInt(int64(presentDays)), true
	case "WORK_DAYS":
		return decimal.NewFromInt(int64(workDays)), true
	case "TENURE_YEARS":
		return decimal.NewFromFloat(tenureYears), true
	default:
		return decimal.Zero, false
	}
}

// ⚠ THE ARITHMETIC HERE IS decimal, NOT float64, AND THAT IS LOAD-BEARING.
// This walk previously ran in float64 with its decimal inputs converted via
// InexactFloat64(). Both ends of the calculation were already exact — only the
// middle was lossy — and it produced genuinely wrong money: scanning 28,572
// ordinary salary bases against a three-bracket table, 42 of them came out
// ONE PAISA different after rounding (base 1030.10 gave 51.50 where the exact
// answer is 51.51). This is the statutory deduction on every payslip, so a
// paisa is not a rounding curiosity, it is a wrong figure on somebody's pay.
func ComputeSlab(base decimal.Decimal, cfg *hrmsalary.SlabConfig) decimal.Decimal {
	if cfg == nil || len(cfg.Slabs) == 0 || !base.IsPositive() {
		return decimal.Zero
	}

	slabs := make([]hrmsalary.Slab, len(cfg.Slabs))
	copy(slabs, cfg.Slabs)
	sort.Slice(slabs, func(i, j int) bool {
		if slabs[i].UpTo == nil {
			return false
		} // no-upper-bound slab always sorts last
		if slabs[j].UpTo == nil {
			return true
		}
		return slabs[i].UpTo.LessThan(*slabs[j].UpTo)
	})

	total, lower := decimal.Zero, decimal.Zero
	for _, sl := range slabs {
		upper := base // uncapped (UpTo == nil) slab absorbs whatever remains
		if sl.UpTo != nil {
			upper = *sl.UpTo
		}
		if upper.GreaterThan(base) {
			upper = base
		}
		if upper.GreaterThan(lower) {
			total = total.Add(upper.Sub(lower).Mul(sl.Rate))
		}
		if sl.UpTo == nil || upper.GreaterThanOrEqual(base) {
			break
		}
		lower = upper
	}
	return total
}

// evalFormula evaluates an expr-lang expression in the given environment.
func (s *serviceImpl) evalFormula(expression string, env map[string]interface{}) (float64, error) {
	program, err := expr.Compile(expression, expr.Env(env), expr.AsFloat64())
	if err != nil {
		return 0, fmt.Errorf("compile: %w", err)
	}
	result, err := expr.Run(program, env)
	if err != nil {
		return 0, fmt.Errorf("eval: %w", err)
	}
	if v, ok := result.(float64); ok {
		return v, nil
	}
	return 0, fmt.Errorf("formula did not return a number")
}

// ReferencesGross reports whether a component's value depends on GROSS, which
// is what decides the stage it is evaluated in.
//
// Getting this wrong changes pay, so it takes primitives rather than the
// caller's local row type and is tested directly. A component wrongly judged
// gross-independent is evaluated against zero; one wrongly judged dependent is
// excluded from the gross it should have contributed to.
func ReferencesGross(calcMethod string, formula *string, slabConfigRaw []byte) bool {
	switch calcMethod {
	case "pct_of_gross":
		return true
	case "formula":
		// Textual, because the expression is evaluated against an env map whose
		// GROSS key is set per stage. A formula naming GROSS must not be
		// evaluated before gross exists.
		return formula != nil && strings.Contains(*formula, "GROSS")
	case "slab":
		if slabConfigRaw == nil {
			return false
		}
		var cfg hrmsalary.SlabConfig
		if err := json.Unmarshal(slabConfigRaw, &cfg); err != nil {
			return false
		}
		return cfg.BaseVariable == "GROSS"
	}
	return false
}

func roundDecimal(d decimal.Decimal, scale int32, mode string) decimal.Decimal {
	switch mode {
	case "half_up":
		return d.Round(scale)
	case "half_even":
		return d.RoundBank(scale)
	case "down":
		return d.RoundDown(scale)
	case "up":
		return d.RoundUp(scale)
	case "ceiling":
		return d.RoundCeil(scale)
	case "floor":
		return d.RoundFloor(scale)
	default:
		return d.Round(scale)
	}
}

func (s *serviceImpl) ApproveRun(ctx context.Context, orgID, ref, approvedBy string) (*PayslipRun, error) {
	run, err := s.repo.FindRunByRef(ctx, orgID, ref)
	if err != nil {
		return nil, fmt.Errorf("payslips: ApproveRun: %w", err)
	}
	if run == nil {
		return nil, ErrNotFound
	}
	if run.Status != RunComputed {
		return nil, ErrNotComputed
	}

	// The negative-net guard.
	//
	// ComputeRun stores the true net, negative included — it no longer clamps
	// to zero and pretends the shortfall did not happen. That honesty has to
	// stop somewhere, and approval is the right place: a run where deductions
	// exceed someone's gross is a data problem to resolve, never something to
	// pay out. Refusing here also means the offending payslips are still on
	// record for whoever has to fix them.
	negatives, err := s.repo.CountNegativeNetPayslips(ctx, run.ID)
	if err != nil {
		return nil, fmt.Errorf("payslips: ApproveRun: negative-net check: %w", err)
	}
	// An F&F run is the ONE exception, and it is an exception to the
	// CONCLUSION, not to the reasoning above. For ordinary payroll a negative
	// net means the inputs are wrong and paying it out would be indefensible.
	// For a settlement it means the leaver owes the company more than the
	// company owes them — a receivable to collect, which refusing to approve
	// would strand along with the rest of the settlement. Every other run
	// type keeps the guard exactly as r25 wrote it.
	if negatives > 0 && run.RunType != RunTypeFnF {
		return nil, fmt.Errorf("%w: %d payslip(s) in this run", ErrNegativeNetPay, negatives)
	}

	// Clearance gates the MONEY LEAVING, not the arithmetic. An F&F run can
	// be computed and inspected with clearance still open — that is how HR
	// answers "what will I actually receive" — but it cannot be approved
	// until every department with an outstanding claim has settled or waived
	// it. Once approved the figure is fixed, and a late claim has nowhere
	// left to go.
	if run.RunType == RunTypeFnF && s.fnfSource != nil {
		cleared, err := s.fnfSource.ClearanceComplete(ctx, orgID, run.ID)
		if err != nil {
			return nil, fmt.Errorf("payslips: ApproveRun: clearance check: %w", err)
		}
		if !cleared {
			return nil, ErrClearancePending
		}
	}

	now := time.Now()
	run.Status = RunApproved
	run.ApprovedAt = &now
	run.ApprovedBy = &approvedBy
	if err := s.repo.UpdateRun(ctx, run); err != nil {
		return nil, fmt.Errorf("payslips: ApproveRun: %w", err)
	}
	return run, nil
}

func (s *serviceImpl) MarkPaid(ctx context.Context, orgID, ref, paidBy string) (*PayslipRun, error) {
	run, err := s.repo.FindRunByRef(ctx, orgID, ref)
	if err != nil {
		return nil, fmt.Errorf("payslips: MarkPaid: %w", err)
	}
	if run == nil {
		return nil, ErrNotFound
	}
	if run.Status != RunApproved {
		return nil, ErrNotApproved
	}
	now := time.Now()
	run.Status = RunPaid
	run.PaidAt = &now
	run.PaidBy = &paidBy
	if err := s.repo.UpdateRun(ctx, run); err != nil {
		return nil, fmt.Errorf("payslips: MarkPaid: %w", err)
	}
	return run, nil
}

func (s *serviceImpl) CancelRun(ctx context.Context, orgID, ref string) (*PayslipRun, error) {
	run, err := s.repo.FindRunByRef(ctx, orgID, ref)
	if err != nil {
		return nil, fmt.Errorf("payslips: CancelRun: %w", err)
	}
	if run == nil {
		return nil, ErrNotFound
	}
	if run.Status == RunPaid || run.Status == RunCancelled {
		return nil, ErrWrongStatus
	}
	run.Status = RunCancelled
	if err := s.repo.UpdateRun(ctx, run); err != nil {
		return nil, fmt.Errorf("payslips: CancelRun: %w", err)
	}
	return run, nil
}

func (s *serviceImpl) ListPayslips(ctx context.Context, orgID string, filter SlipListFilter) (*SlipListResponse, error) {
	filter.Normalise()
	list, err := s.repo.FindPayslips(ctx, orgID, filter)
	if err != nil {
		return nil, fmt.Errorf("payslips: ListPayslips: %w", err)
	}
	if list == nil {
		list = []*Payslip{}
	}
	total, err := s.repo.CountPayslips(ctx, orgID, filter)
	if err != nil {
		return nil, fmt.Errorf("payslips: ListPayslips: count: %w", err)
	}
	return &SlipListResponse{Payslips: list, Total: total, Limit: filter.Limit, Offset: filter.Offset}, nil
}

func (s *serviceImpl) GetPayslip(ctx context.Context, orgID, ref string) (*Payslip, error) {
	p, err := s.repo.FindPayslipByRef(ctx, orgID, ref)
	if err != nil {
		return nil, fmt.Errorf("payslips: GetPayslip: %w", err)
	}
	if p == nil {
		return nil, ErrPayslipNotFound
	}
	// Load lines
	lines, err := s.repo.LoadPayslipLines(ctx, p.ID)
	if err == nil {
		p.Lines = lines
	}
	return p, nil
}

// SetBaseCurrencySource attaches the legal-entity layer (11B-2).
func (s *serviceImpl) SetBaseCurrencySource(src BaseCurrencySource) { s.entityCurrency = src }
