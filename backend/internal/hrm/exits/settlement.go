// backend/internal/hrm/exits/settlement.go
package exits

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/shopspring/decimal"

	"github.com/mridha/businesssaas/internal/hrm/expenses"
	"github.com/mridha/businesssaas/internal/hrm/leave"
	"github.com/mridha/businesssaas/internal/hrm/loans"
	"github.com/mridha/businesssaas/internal/hrm/payslips"
)

// ── Consumer-owned settlement sources ────────────────────────────────────────
//
// Three narrow interfaces, each naming the PROVIDER's own types so the
// provider satisfies it structurally with no adapter — the corrected
// certifications.SkillGranter precedent, as used for
// assets.HandoverAcknowledger, expenses.ReimbursementCreator,
// email.TicketRaiser and recruitment.RehireChecker.
//
// All three are NIL-SAFE. A deployment without leave, loans or expenses wired
// produces no line from that source rather than failing the settlement — the
// same contract payslips' own six sources keep.

// LeaveEncashmentSource supplies encashment DAYS this org has already
// recorded, per leave type, with the rate basis its policy configured.
//
// The split is deliberate and predates this phase: hrm/leave records days and
// never money (Phase 2 decision #3), and encashment_rate_basis has been
// stored since then with the note that "a future F&F phase reads this". Leave
// owns how many days; F&F owns what a day is worth.
type LeaveEncashmentSource interface {
	EncashmentsForSettlement(ctx context.Context, orgID, employeeID string) ([]*leave.EncashmentSummary, error)
}

// LoanSettlementSource supplies a leaver's outstanding loan balances and
// forecloses them.
//
// ForecloseForSettlement is separate from the ordinary ForecloseLoan because
// the caller differs in kind: a human settling a loan early supplies the
// amount, but a settlement must never trust a supplied figure — it computes
// the outstanding and forecloses exactly that.
type LoanSettlementSource interface {
	OutstandingLoansForEmployee(ctx context.Context, orgID, employeeID string) ([]*loans.OutstandingLoan, error)
	ForecloseForSettlement(ctx context.Context, orgID, loanID, foreclosedBy string, amount decimal.Decimal) error
}

// AdvanceSettlementSource supplies unsettled travel advances and recovers
// them.
type AdvanceSettlementSource interface {
	OutstandingAdvancesForEmployee(ctx context.Context, orgID, employeeID string) ([]*expenses.Advance, error)
	RecoverAdvanceForSettlement(ctx context.Context, orgID, advanceID string, amount decimal.Decimal) error
}

// This file satisfies payslips.FnFSource. The direction is deliberate:
// hrm/exits imports hrm/payslips, never the reverse — payslips is the
// CONSUMER and owns the interface, exactly as it does for BonusSource,
// LoanSource, ReimbursementSource, StatutorySource and BenefitsSource.

// SettlementForRun answers which employee a run_type='fnf' run settles and
// what one-off credits and debits apply on top of their prorated final
// salary.
//
// Returning nil means no exit is attached to this run — payslips reports that
// rather than computing an empty settlement, because an F&F run that pays
// nobody is always a mistake.
func (s *serviceImpl) SettlementForRun(ctx context.Context, orgID, runID string) (*payslips.FnFSettlement, error) {
	exit, err := s.repo.FindExitByFnFRun(ctx, orgID, runID)
	if err != nil {
		return nil, fmt.Errorf("exits: SettlementForRun: %w", err)
	}
	if exit == nil {
		return nil, nil
	}

	tenure, err := s.repo.EmployeeTenure(ctx, orgID, exit.EmployeeID)
	if err != nil {
		return nil, fmt.Errorf("exits: SettlementForRun: %w", err)
	}
	if tenure == nil {
		return nil, fmt.Errorf("exits: SettlementForRun: employee %s not found", exit.EmployeeID)
	}

	// The currency the settlement is actually paid in, which is what an
	// advance's own currency must match before it can be recovered.
	runCurrency, err := s.repo.FnFRunCurrency(ctx, orgID, runID)
	if err != nil {
		return nil, err
	}

	rows, err := s.buildSettlementLines(ctx, orgID, exit, tenure, runCurrency)
	if err != nil {
		return nil, err
	}

	// Record the audit trail before payroll runs, so a settlement can be
	// explained even if the compute later aborts. These rows carry no
	// payslip_line_id yet; MarkSettled stamps it once the lines are real.
	if err := s.repo.InsertSettlementLines(ctx, exit.ID, rows); err != nil {
		return nil, err
	}

	lines := make([]payslips.SettlementLine, 0, len(rows))
	for _, r := range rows {
		sourceID := ""
		if r.SourceID != nil {
			sourceID = *r.SourceID
		}
		lines = append(lines, payslips.SettlementLine{
			SourceType: r.SourceType, SourceID: sourceID,
			Description: r.Description, Amount: r.Amount, IsCredit: r.IsCredit,
		})
	}
	return &payslips.FnFSettlement{
		ExitID: exit.ID, EmployeeID: exit.EmployeeID, Lines: lines,
	}, nil
}

// buildSettlementLines assembles every credit and debit for one exit.
//
// The DAILY RATE is computed ONCE here and reused by every line that needs
// it. Two different rates for the same person in one settlement is
// indefensible to whoever has to read the payslip, and deriving it separately
// per source is exactly how that happens.
func (s *serviceImpl) buildSettlementLines(ctx context.Context, orgID string, exit *Exit, tenure *EmployeeTenure, runCurrency string) ([]SettlementLineRow, error) {
	// ⚠ Nearly unreachable since 11B-2 — a payroll run now always carries a
	// resolved currency — but a settlement must never assemble in a currency
	// nobody chose, so the fallback resolves through the same chain instead
	// of naming one.
	if strings.TrimSpace(runCurrency) == "" {
		runCurrency = s.resolveCurrency(ctx, orgID, nil)
	}
	rows := make([]SettlementLineRow, 0, 4)

	ruleRow, err := s.repo.GratuityRuleAsOf(ctx, orgID, exit.LastWorkingDate)
	if err != nil {
		return nil, fmt.Errorf("exits: buildSettlementLines: %w", err)
	}
	rule := ruleRow.ToRule()

	// The divisor belongs to the gratuity rule because that is where the
	// policy is recorded. With no rule configured, fall back to the common
	// statutory 30 — stated here rather than left implicit, since it silently
	// decides what a notice shortfall costs.
	divisor := decimal.NewFromInt(30)
	if rule != nil && rule.MonthlyDivisor.IsPositive() {
		divisor = rule.MonthlyDivisor
	}
	dailyRate := DailyRate(tenure.MonthlyPay, divisor)

	// ── Credit: gratuity ──
	grat := ComputeGratuity(rule, tenure.MonthlyPay, tenure.HireDate, exit.LastWorkingDate, tenure.IsForCause)
	if grat.Amount.IsPositive() {
		rows = append(rows, SettlementLineRow{
			SourceType: "gratuity",
			Description: fmt.Sprintf("Gratuity — %d completed years at %s days per year",
				grat.CompletedYears, rule.DaysPerYear.String()),
			Amount: grat.Amount, IsCredit: true, Currency: "BDT",
		})
	}

	// ── Debit: notice shortfall ──
	//
	// 9A stores DAYS, not money, precisely so the rate is applied here at
	// settlement time rather than frozen months earlier at exit creation.
	if exit.NoticeShortfallDays > 0 && dailyRate.IsPositive() {
		amount := dailyRate.Mul(decimal.NewFromInt(int64(exit.NoticeShortfallDays)))
		rows = append(rows, SettlementLineRow{
			SourceType:  "notice_shortfall",
			Description: fmt.Sprintf("Notice shortfall recovery — %d days", exit.NoticeShortfallDays),
			Amount:      amount, IsCredit: false, Currency: "BDT",
		})
	}

	// ── Credit: leave encashment ──
	//
	// hrm/leave has recorded the DAYS; this is where they become money.
	// Pricing is per policy: 'basic_pay' uses the monthly basic, 'gross_pay'
	// the same figure this run computed as gross. A leave type whose policy
	// says 'fixed' is SKIPPED with a reason — see below.
	if s.leaveSource != nil {
		encashments, err := s.leaveSource.EncashmentsForSettlement(ctx, orgID, exit.EmployeeID)
		if err != nil {
			return nil, fmt.Errorf("exits: buildSettlementLines: leave encashment: %w", err)
		}
		for _, e := range encashments {
			if e.Days <= 0 {
				continue
			}
			rate, reason := encashmentDailyRate(e, tenure.MonthlyPay, divisor)
			if reason != "" {
				// Recorded, not silently dropped: HR needs to see that these
				// days were owed and why nothing was paid for them.
				days := decimal.NewFromFloat(e.Days)
				id := e.LeaveTypeID
				rows = append(rows, SettlementLineRow{
					SourceType: "leave_encashment", SourceID: &id,
					Description: fmt.Sprintf("%s — %s days encashed, NOT PAID: %s",
						e.LeaveTypeName, days.String(), reason),
					Amount: decimal.Zero, IsCredit: true, Currency: "BDT",
				})
				continue
			}
			amount := rate.Mul(decimal.NewFromFloat(e.Days))
			if !amount.IsPositive() {
				continue
			}
			id := e.LeaveTypeID
			rows = append(rows, SettlementLineRow{
				SourceType: "leave_encashment", SourceID: &id,
				Description: fmt.Sprintf("%s encashment — %s days", e.LeaveTypeName,
					decimal.NewFromFloat(e.Days).String()),
				Amount: amount, IsCredit: true, Currency: "BDT",
			})
		}
	}

	// ── Debit: loan foreclosure ──
	//
	// The FULL outstanding, not just the installment due. 7C deliberately
	// left auto-foreclosure unbuilt because it needed the negative-net guard
	// redesigned around an F&F module that did not exist; both now do.
	//
	// ⚠ The ordinary per-installment recovery is SKIPPED for fnf runs
	// (payslips.computePayslips gates it on run type). Without that, the
	// installment due this period would be charged twice — once by recovery
	// and once inside this balance — and capped by two different rules.
	if s.loanSource != nil {
		outstanding, err := s.loanSource.OutstandingLoansForEmployee(ctx, orgID, exit.EmployeeID)
		if err != nil {
			return nil, fmt.Errorf("exits: buildSettlementLines: loans: %w", err)
		}
		for _, l := range outstanding {
			if !l.Outstanding.IsPositive() {
				continue
			}
			id := l.LoanID
			rows = append(rows, SettlementLineRow{
				SourceType: "loan_foreclosure", SourceID: &id,
				Description: fmt.Sprintf("Loan foreclosure (%s) — full outstanding balance", l.LoanType),
				Amount:      l.Outstanding, IsCredit: false, Currency: "BDT",
			})
		}
	}

	// ── Debit: unsettled travel advances ──
	//
	// A same-currency advance is recovered directly. A FOREIGN-currency
	// advance is converted only when a real recorded rate exists as of the
	// last working date (11B-1); with no rate it keeps the behaviour this
	// slice shipped with — reported on the trail at zero with the reason, so
	// HR settles it deliberately.
	//
	// ⚠ THE NO-RATE PATH IS NOT A FALLBACK TO PARITY, AND MUST NEVER BECOME
	// ONE. Treating an unconvertible balance as 1:1 charges a departing
	// person a number nobody computed, and they have no way to see that it
	// happened. Refusing to convert is the honest failure; converting at an
	// invented rate is a silent one.
	if s.advanceSource != nil {
		advances, err := s.advanceSource.OutstandingAdvancesForEmployee(ctx, orgID, exit.EmployeeID)
		if err != nil {
			return nil, fmt.Errorf("exits: buildSettlementLines: advances: %w", err)
		}
		for _, a := range advances {
			out := a.Outstanding()
			if !out.IsPositive() {
				continue
			}
			id := a.ID
			if !strings.EqualFold(a.Currency, runCurrency) {
				conv, err := s.convertAdvance(ctx, orgID, out, a.Currency, runCurrency, exit.LastWorkingDate)
				if err != nil {
					return nil, err
				}
				if conv == nil {
					rows = append(rows, SettlementLineRow{
						SourceType: "travel_advance", SourceID: &id,
						Description: fmt.Sprintf(
							"Travel advance %s %s outstanding — NOT RECOVERED: no exchange rate to %s, settle manually",
							a.Currency, out.String(), runCurrency),
						Amount: decimal.Zero, IsCredit: false, Currency: a.Currency,
					})
					continue
				}
				// ⚠ All five audit fields travel together: the original
				// amount and currency, the rate, its date, and the converted
				// figure. Amount/Currency stay the CONVERTED pair so payslip
				// assembly is untouched.
				rows = append(rows, SettlementLineRow{
					SourceType: "travel_advance", SourceID: &id,
					Description: fmt.Sprintf(
						"Travel advance recovery — %s %s converted at %s (rate of %s)",
						a.Currency, out.String(), conv.Rate.String(),
						conv.RateDate.Format("2006-01-02")),
					Amount: conv.Converted, IsCredit: false, Currency: runCurrency,
					OriginalAmount: &conv.Original, OriginalCurrency: &conv.OriginalCurrency,
					ExchangeRate: &conv.Rate, ExchangeRateDate: &conv.RateDate,
				})
				continue
			}
			rows = append(rows, SettlementLineRow{
				SourceType: "travel_advance", SourceID: &id,
				Description: "Travel advance recovery — unsettled balance",
				Amount:      out, IsCredit: false, Currency: a.Currency,
			})
		}
	}

	// ── Debit: unresolved clearance dues ──
	//
	// One line per item rather than a single total: "clearance dues 52,000"
	// is not something a departing employee can check, and each of these is
	// a specific claim by a specific department that they may want to
	// dispute individually.
	items, err := s.repo.FindClearanceItems(ctx, orgID, exit.ID)
	if err != nil {
		return nil, fmt.Errorf("exits: buildSettlementLines: %w", err)
	}
	for _, it := range items {
		if it.IsResolved || !it.BlockingAmount.IsPositive() {
			continue
		}
		id := it.ID
		rows = append(rows, SettlementLineRow{
			SourceType: "clearance_due", SourceID: &id,
			Description: fmt.Sprintf("%s — %s", it.Department, it.Description),
			Amount:      it.BlockingAmount, IsCredit: false, Currency: it.Currency,
		})
	}

	return rows, nil
}

// encashmentDailyRate prices one encashed day under the policy's configured
// basis. A non-empty reason means NO money should be paid, and says why.
//
// ⚠ 'fixed' cannot be honoured: hrm_leave_policies stores the BASIS but has
// no column for the fixed amount itself, so there is nothing to pay. Guessing
// a figure, or quietly pricing it at zero with no explanation, would both be
// worse than saying so — this is a real gap in the Phase 2 schema, not an
// oversight here.
func encashmentDailyRate(e *leave.EncashmentSummary, monthlyBasic, divisor decimal.Decimal) (decimal.Decimal, string) {
	if e.RateBasis == nil {
		return decimal.Zero, "the leave policy has no encashment rate basis configured"
	}
	switch *e.RateBasis {
	case leave.EncashmentBasisBasicPay, leave.EncashmentBasisGrossPay:
		// Both resolve against the monthly figure this run already loaded, so
		// encashment and the notice shortfall cannot disagree about what a
		// day is worth.
		return DailyRate(monthlyBasic, divisor), ""
	case leave.EncashmentBasisFixed:
		return decimal.Zero, "the policy uses a fixed encashment rate, which hrm_leave_policies has no column to store"
	default:
		return decimal.Zero, fmt.Sprintf("unrecognised encashment rate basis %q", *e.RateBasis)
	}
}

// MarkSettled stamps which payslip line each settlement line became.
//
// Called by ComputeRun only after every payslip and line is persisted — never
// from abortCompute. That contract is what stops a source being marked
// consumed with no payslip behind it, which for a foreclosed loan would be
// money written off that nobody ever collected.
func (s *serviceImpl) MarkSettled(ctx context.Context, runID string, applied []payslips.AppliedSettlementLine) error {
	if len(applied) == 0 {
		return nil
	}
	// The org is recovered from the run rather than passed: payslips knows
	// the run, and threading an orgID through the interface would let the two
	// disagree.
	exit, err := s.repo.FindExitByFnFRunAnyOrg(ctx, runID)
	if err != nil {
		return fmt.Errorf("exits: MarkSettled: %w", err)
	}
	if exit == nil {
		return fmt.Errorf("exits: MarkSettled: no exit attached to run %s", runID)
	}
	for _, a := range applied {
		if err := s.repo.LinkSettlementLineToPayslip(ctx, exit.ID, a.SourceType, a.SourceID, a.LineID); err != nil {
			return err
		}
		// Mark the SOURCE consumed, not just the trail. Linking a line only
		// records that the money was charged; without this the loan stays
		// active and the advance stays outstanding, and the next process to
		// look would charge them again.
		//
		// Reached only after every payslip and line is persisted — never from
		// abortCompute — so a loan can never be closed against a settlement
		// that was rolled back.
		if err := s.consumeSettlementSource(ctx, exit.OrgID, a); err != nil {
			return err
		}
	}
	return nil
}

// consumeSettlementSource closes out whatever produced one settlement line.
//
// Failures here are RETURNED, not swallowed: ComputeRun treats a MarkSettled
// error as a full compute failure and aborts the run, which is right —
// payslips committed with a loan still active would let the same balance be
// recovered twice.
func (s *serviceImpl) consumeSettlementSource(ctx context.Context, orgID string, a payslips.AppliedSettlementLine) error {
	if a.SourceID == "" || !a.Amount.IsPositive() {
		// Gratuity and notice shortfall have no source row to close, and a
		// zero-amount line is one that was reported but deliberately NOT
		// charged (an unpriceable encashment, a foreign-currency advance) —
		// consuming either would be wrong.
		return nil
	}
	switch a.SourceType {
	case "loan_foreclosure":
		if s.loanSource == nil {
			return nil
		}
		// The empty actor is not data loss: hrm_loans has no foreclosed_by
		// column, and the provider's own ForecloseLoan ignores the argument
		// too. Who foreclosed is recorded where it is actually answerable —
		// on the payroll run that approved the settlement.
		if err := s.loanSource.ForecloseForSettlement(ctx, orgID, a.SourceID, "", a.Amount); err != nil {
			return fmt.Errorf("exits: consumeSettlementSource: foreclose loan %s: %w", a.SourceID, err)
		}
	case "travel_advance":
		if s.advanceSource == nil {
			return nil
		}
		if err := s.advanceSource.RecoverAdvanceForSettlement(ctx, orgID, a.SourceID, a.Amount); err != nil {
			return fmt.Errorf("exits: consumeSettlementSource: recover advance %s: %w", a.SourceID, err)
		}
	}
	// leave_encashment and clearance_due need no source write: the leave
	// ledger is append-only and already records the days, and a clearance
	// item's own resolution is HR's decision, not a payroll side effect.
	return nil
}

// ClearanceComplete reports whether every blocking clearance item is
// resolved. Read by the payroll layer to gate F&F APPROVAL — never
// computation, so HR can see the settlement figure while clearance is open.
func (s *serviceImpl) ClearanceComplete(ctx context.Context, orgID, runID string) (bool, error) {
	exit, err := s.repo.FindExitByFnFRun(ctx, orgID, runID)
	if err != nil {
		return false, fmt.Errorf("exits: ClearanceComplete: %w", err)
	}
	if exit == nil {
		// No exit attached is not "clearance incomplete" — it is a different
		// problem, reported by payslips as ErrNoExitForFnFRun at compute time.
		return true, nil
	}
	items, err := s.repo.FindClearanceItems(ctx, orgID, exit.ID)
	if err != nil {
		return false, fmt.Errorf("exits: ClearanceComplete: %w", err)
	}
	for _, it := range items {
		if !it.IsResolved && it.BlockingAmount.IsPositive() {
			return false, nil
		}
	}
	return true, nil
}

// AttachFnFRun links a payroll run to the exit it settles, which is what
// makes SettlementForRun able to answer at all.
func (s *serviceImpl) AttachFnFRun(ctx context.Context, orgID string, caller Caller, ref, runID string) (*Exit, error) {
	if !caller.CanManage {
		return nil, ErrAccessDenied
	}
	e, err := s.loadExit(ctx, orgID, caller, ref)
	if err != nil {
		return nil, err
	}
	if e.Status.IsTerminal() {
		return nil, ErrWrongStatus
	}
	e.FnFPayslipRunID = &runID
	if e.Status == StatusInitiated || e.Status == StatusInClearance {
		e.Status = StatusPendingSettlement
	}
	if err := s.repo.UpdateExit(ctx, e); err != nil {
		return nil, err
	}
	return e, nil
}

// ── Gratuity rules ───────────────────────────────────────────────────────────

// CreateGratuityRuleRequest creates an effective-dated gratuity rule. A
// revision is a NEW row with a later effective_date, never an edit — that is
// what stops a change altering settlements already computed.
type CreateGratuityRuleRequest struct {
	Name string `json:"name"`
	// Decimals arrive as strings: a JSON number cannot represent every
	// decimal exactly, and these figures multiply into somebody's payout.
	MinYearsOfService   string  `json:"min_years_of_service"`
	DaysPerYear         string  `json:"days_per_year"`
	BaseComponent       *string `json:"base_component"`
	MonthlyDivisor      *string `json:"monthly_divisor"`
	ForfeitOnMisconduct *bool   `json:"forfeit_on_misconduct"`
	EffectiveDate       string  `json:"effective_date"`
}

func (s *serviceImpl) ListGratuityRules(ctx context.Context, orgID string, caller Caller) ([]*GratuityRuleRow, error) {
	return s.repo.ListGratuityRules(ctx, orgID)
}

func (s *serviceImpl) CreateGratuityRule(ctx context.Context, orgID string, caller Caller, req CreateGratuityRuleRequest) (*GratuityRuleRow, error) {
	name := strings.TrimSpace(req.Name)
	if name == "" {
		return nil, ErrNameRequired
	}
	minYears, err := decimal.NewFromString(strings.TrimSpace(req.MinYearsOfService))
	if err != nil || minYears.IsNegative() {
		return nil, ErrInvalidGratuityRule
	}
	daysPerYear, err := decimal.NewFromString(strings.TrimSpace(req.DaysPerYear))
	if err != nil || !daysPerYear.IsPositive() {
		return nil, ErrInvalidGratuityRule
	}
	effective, err := time.Parse("2006-01-02", strings.TrimSpace(req.EffectiveDate))
	if err != nil {
		return nil, ErrInvalidGratuityRule
	}

	divisor := decimal.NewFromInt(30)
	if req.MonthlyDivisor != nil && strings.TrimSpace(*req.MonthlyDivisor) != "" {
		divisor, err = decimal.NewFromString(strings.TrimSpace(*req.MonthlyDivisor))
		if err != nil || !divisor.IsPositive() {
			return nil, ErrInvalidGratuityRule
		}
	}
	base := "basic"
	if req.BaseComponent != nil && strings.TrimSpace(*req.BaseComponent) != "" {
		base = strings.ToLower(strings.TrimSpace(*req.BaseComponent))
		if base != "basic" && base != "gross" {
			return nil, ErrInvalidGratuityRule
		}
	}

	g := &GratuityRuleRow{
		OrgID: orgID, Name: name,
		MinYearsOfService: minYears, DaysPerYear: daysPerYear,
		BaseComponent: base, MonthlyDivisor: divisor,
		EffectiveDate: effective, CreatedBy: caller.UserID,
	}
	if req.ForfeitOnMisconduct != nil {
		g.ForfeitOnMisconduct = *req.ForfeitOnMisconduct
	}
	if err := s.repo.CreateGratuityRule(ctx, g); err != nil {
		return nil, err
	}
	return g, nil
}

// ListSettlementLines returns the audit trail explaining one settlement.
func (s *serviceImpl) ListSettlementLines(ctx context.Context, orgID string, caller Caller, ref string) ([]*SettlementLineRow, error) {
	e, err := s.loadExit(ctx, orgID, caller, ref)
	if err != nil {
		return nil, err
	}
	return s.repo.FindSettlementLines(ctx, orgID, e.ID)
}

// advanceConversion is the five-field record for one converted advance.
type advanceConversion struct {
	Original         decimal.Decimal
	OriginalCurrency string
	Rate             decimal.Decimal
	RateDate         time.Time
	Converted        decimal.Decimal
}

// convertAdvance resolves the rate that applied on the last working date and
// applies it.
//
// ⚠ Returns (nil, nil) — meaning "not convertible" — when no rate source is
// wired or no rate was recorded for that pair by that date. The caller then
// reports the advance at zero with an explanation, exactly as it did before
// the FX table existed. There is deliberately no branch that returns a rate
// of 1.
//
// ⚠ The rate is resolved AS OF THE LAST WORKING DATE, not today. A rate
// recorded after somebody left must not reprice their settlement, and a
// settlement re-assembled months later must produce the same figure it did
// the first time.
func (s *serviceImpl) convertAdvance(ctx context.Context, orgID string, amount decimal.Decimal, from, to string, asOf time.Time) (*advanceConversion, error) {
	if s.rateSource == nil {
		return nil, nil
	}
	rate, rateDate, ok, err := s.rateSource.RateAsOfPrimitive(ctx, orgID, from, to, asOf)
	if err != nil {
		return nil, fmt.Errorf("exits: convertAdvance: %w", err)
	}
	if !ok || !rate.IsPositive() {
		return nil, nil
	}
	return &advanceConversion{
		Original:         amount,
		OriginalCurrency: strings.ToUpper(strings.TrimSpace(from)),
		Rate:             rate,
		RateDate:         rateDate,
		// Only the RESULT rounds to money scale; the rate is stored as
		// resolved.
		Converted: amount.Mul(rate).Round(2),
	}, nil
}
