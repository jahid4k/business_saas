// backend/internal/hrm/payslips/service.go
package payslips

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/shopspring/decimal"

	"github.com/expr-lang/expr"
	"github.com/jackc/pgx/v5/pgxpool"

	hrmsalary "github.com/mridha/businesssaas/internal/hrm/salary"
)

// Service defines business logic for the payroll engine.
type Service interface {
	ListRuns(ctx context.Context, orgID string) (*RunListResponse, error)
	GetRun(ctx context.Context, orgID, ref string) (*PayslipRun, error)
	CreateRun(ctx context.Context, orgID, createdBy string, req CreateRunRequest) (*PayslipRun, error)
	// ComputeRun runs the payroll formula engine for all employees in the org.
	// Prerequisite: attendance_period must be finalized (if attendance_period_id is set).
	ComputeRun(ctx context.Context, orgID, ref, computedBy string) (*PayslipRun, error)
	ApproveRun(ctx context.Context, orgID, ref, approvedBy string) (*PayslipRun, error)
	MarkPaid(ctx context.Context, orgID, ref, paidBy string) (*PayslipRun, error)
	CancelRun(ctx context.Context, orgID, ref string) (*PayslipRun, error)
	ListPayslips(ctx context.Context, orgID, runID, employeeID string) (*SlipListResponse, error)
	GetPayslip(ctx context.Context, orgID, ref string) (*Payslip, error)
}

type serviceImpl struct {
	repo Repository
	db   *pgxpool.Pool
}

func NewService(repo Repository, db *pgxpool.Pool) Service { return &serviceImpl{repo: repo, db: db} }

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

	existing, err := s.repo.FindRunByPeriod(ctx, orgID, req.Year, req.Month)
	if err != nil {
		return nil, fmt.Errorf("payslips: CreateRun: check existing: %w", err)
	}
	if existing != nil {
		return nil, ErrDuplicateRun
	}

	currency := "BDT"
	if req.Currency != nil && strings.TrimSpace(*req.Currency) != "" {
		currency = *req.Currency
	}

	run := &PayslipRun{
		OrgID: orgID, PeriodYear: req.Year, PeriodMonth: req.Month,
		Description: req.Description, Currency: currency,
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
func (s *serviceImpl) ComputeRun(ctx context.Context, orgID, ref, computedBy string) (*PayslipRun, error) {
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

	// D1 dependency check: attendance period must be finalized
	if run.AttendancePeriodID != nil {
		var attStatus string
		if err := s.db.QueryRow(ctx, `SELECT status FROM hrm_attendance_periods WHERE id=$1::uuid`, *run.AttendancePeriodID).Scan(&attStatus); err == nil {
			if attStatus != "finalized" && attStatus != "locked" {
				return nil, ErrAttendanceNotFinalized
			}
		}
	}

	// Mark as computing to prevent concurrent runs
	run.Status = RunComputing
	if err := s.repo.UpdateRun(ctx, run); err != nil {
		return nil, fmt.Errorf("payslips: ComputeRun: mark computing: %w", err)
	}

	// Fetch rounding policy
	var roundingScale int32 = 2
	var roundingMode string = "half_up"
	_ = s.db.QueryRow(ctx, `SELECT money_rounding_scale, money_rounding_mode FROM organizations WHERE id=$1::uuid`, orgID).Scan(&roundingScale, &roundingMode)

	// Load active employees
	type empRow struct {
		ID            string
		HireDate      string
		StructureID   *string
		StructureName *string
		BasicPay      decimal.Decimal
	}
	rows, err := s.db.Query(ctx,
		`SELECT e.id::text, COALESCE(to_char(e.hire_date,'YYYY-MM-DD'),''),
		es.structure_id::text, ess.name, COALESCE(es.basic_pay,0)
		FROM hrm_employees e
		LEFT JOIN LATERAL (
		    SELECT structure_id, basic_pay FROM hrm_employee_salary_records
		    WHERE employee_id=e.id AND effective_date <= make_date($2,$3,1)
		    ORDER BY effective_date DESC LIMIT 1
		) es ON TRUE
		LEFT JOIN hrm_salary_structures ess ON ess.id=es.structure_id
		WHERE e.org_id=$1 AND e.status IN ('active','resigned')`,
		orgID, run.PeriodYear, run.PeriodMonth)
	if err != nil {
		run.Status = RunDraft
		_ = s.repo.UpdateRun(ctx, run)
		return nil, fmt.Errorf("payslips: ComputeRun: load employees: %w", err)
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

	var totalGross, totalDeductions, totalNet decimal.Decimal

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
		}
		var components []compRow
		if emp.StructureID != nil {
			cRows, err := s.db.Query(ctx,
				`SELECT c.id::text, c.name, c.component_type, c.calc_method, c.fixed_value,
				sc.override_value, c.formula_expression, c.slab_config, sc.display_order
				FROM hrm_salary_structure_components sc
				JOIN hrm_salary_components c ON c.id=sc.component_id
				WHERE sc.structure_id=$1::uuid
				ORDER BY sc.display_order`,
				*emp.StructureID)
			if err == nil {
				defer cRows.Close()
				for cRows.Next() {
					var cr compRow
					_ = cRows.Scan(&cr.CompID, &cr.CompName, &cr.CompType, &cr.CalcMethod, &cr.FixedValue, &cr.OverrideValue, &cr.Formula, &cr.SlabConfigRaw, &cr.DisplayOrder)
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

		for _, comp := range components {
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
				amount = gross.Mul(effectiveFixed).Div(decimal.NewFromInt(100))
			case "formula":
				if comp.Formula != nil {
					env["GROSS"] = gross.InexactFloat64()
					if result, err := s.evalFormula(*comp.Formula, env); err == nil {
						amount = decimal.NewFromFloat(result)
					}
				}
			case "slab":
				if comp.SlabConfigRaw != nil {
					env["GROSS"] = gross.InexactFloat64()
					var cfg hrmsalary.SlabConfig
					if err := json.Unmarshal(comp.SlabConfigRaw, &cfg); err == nil {
						if base, ok := env[cfg.BaseVariable].(float64); ok {
							amount = decimal.NewFromFloat(ComputeSlab(base, &cfg))
						}
					}
				}
			default: // manual — default 0; HR enters manually
				amount = decimal.Zero
			}

			amount = roundDecimal(amount, roundingScale, roundingMode)

			if amount.IsNegative() {
				amount = decimal.Zero
			} // safety: no negative line items

			var compIDStr *string
			if comp.CompID != nil {
				compIDStr = comp.CompID
			}
			var formulaUsed *string
			if comp.Formula != nil {
				formulaUsed = comp.Formula
			}

			lines = append(lines, &PayslipLine{
				OrgID:          orgID,
				ComponentID:    compIDStr,
				ComponentName:  comp.CompName,
				ComponentType:  comp.CompType,
				CalcMethod:     comp.CalcMethod,
				FormulaUsed:    formulaUsed,
				ComputedAmount: amount,
				DisplayOrder:   comp.DisplayOrder,
			})

			switch comp.CompType {
			case "earning":
				gross = gross.Add(amount)
			case "deduction":
				deductions = deductions.Add(amount)
			}
		}

		netPay := gross.Sub(deductions)
		if netPay.IsNegative() {
			netPay = decimal.Zero
		}

		// Create payslip
		slip := &Payslip{
			OrgID: orgID, EmployeeID: emp.ID,
			PayslipRunID: run.ID,
			PeriodYear:   run.PeriodYear, PeriodMonth: run.PeriodMonth,
			SalaryStructureID: emp.StructureID, SalaryStructureName: emp.StructureName,
			GrossPay: gross, TotalDeductions: deductions, NetPay: netPay,
			BasicPay: emp.BasicPay,
			WorkDays: workDays, PresentDays: presentDays, AbsentDays: absentDays,
			LeaveDays: leaveDays, HolidayDays: holidayDays, OvertimeHours: otHours,
			Currency: run.Currency,
			Status:   SlipComputed,
		}
		if err := s.repo.CreatePayslip(ctx, slip); err != nil {
			continue
		}

		// Attach payslip_id to lines and persist
		for _, l := range lines {
			l.PayslipID = slip.ID
		}
		if len(lines) > 0 {
			_ = s.repo.CreatePayslipLines(ctx, lines)
		}

		totalGross = totalGross.Add(gross)
		totalDeductions = totalDeductions.Add(deductions)
		totalNet = totalNet.Add(netPay)
	}

	now := time.Now()
	run.Status = RunComputed
	run.TotalEmployees = len(employees)
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
func ComputeSlab(base float64, cfg *hrmsalary.SlabConfig) float64 {
	if cfg == nil || len(cfg.Slabs) == 0 || base <= 0 {
		return 0
	}

	slabs := make([]hrmsalary.Slab, len(cfg.Slabs))
	copy(slabs, cfg.Slabs)
	sort.Slice(slabs, func(i, j int) bool {
		if slabs[i].UpTo == nil { return false } // no-upper-bound slab always sorts last
		if slabs[j].UpTo == nil { return true }
		return slabs[i].UpTo.LessThan(*slabs[j].UpTo)
	})

	total, lower := 0.0, 0.0
	for _, sl := range slabs {
		upper := base // uncapped (UpTo == nil) slab absorbs whatever remains
		if sl.UpTo != nil {
			upper = sl.UpTo.InexactFloat64()
		}
		if upper > base {
			upper = base
		}
		if upper > lower {
			total += (upper - lower) * sl.Rate.InexactFloat64()
		}
		if sl.UpTo == nil || upper >= base {
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

func (s *serviceImpl) ListPayslips(ctx context.Context, orgID, runID, employeeID string) (*SlipListResponse, error) {
	list, err := s.repo.FindPayslips(ctx, orgID, runID, employeeID)
	if err != nil {
		return nil, fmt.Errorf("payslips: ListPayslips: %w", err)
	}
	if list == nil {
		list = []*Payslip{}
	}
	return &SlipListResponse{Payslips: list, Total: len(list)}, nil
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
