// backend/internal/tests/integration/fx_test.go
// Phase 11B-1: exchange rates, and the two carried gaps they close.
//
// ⚠ THE RULE THIS SLICE EXISTS TO ENFORCE: never store converted-only. Every
// converted figure keeps original_amount + original_currency + rate +
// rate_date + converted_amount. A stored conversion with no rate cannot be
// audited, cannot be recomputed when a rate is corrected, and cannot be
// explained to the person whose settlement it reduced.
//
// ⚠ AND THE RULE THAT MATTERS MORE: with NO rate available, nothing converts
// at parity. Phase 9B refused to convert precisely because guessing charges a
// departing person a number nobody computed.
//
// Gate: INTEGRATION=1
package integration

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	hrmexpenses "github.com/mridha/businesssaas/internal/hrm/expenses"
	"github.com/mridha/businesssaas/internal/hrm/fx"
	"github.com/shopspring/decimal"
)

type fxFixture struct {
	orgID    string
	statusID string
	ownerID  string
}

func fxAdmin(userID string) fx.Caller { return fx.Caller{UserID: userID, CanManage: true} }

func seedFxFixture(t *testing.T, env *testEnv) *fxFixture {
	t.Helper()
	orgID, statusID, ownerID := seedScopeTestOrg(t, env)
	return &fxFixture{orgID: orgID, statusID: statusID, ownerID: ownerID}
}

func recordRate(t *testing.T, env *testEnv, fxf *fxFixture, from, to, rate string, on time.Time) {
	t.Helper()
	if _, err := env.hrmFxSvc.RecordRate(context.Background(), fxf.orgID, fxAdmin(fxf.ownerID),
		fx.RecordRateRequest{
			FromCurrency: from, ToCurrency: to, Rate: rate,
			RateDate: on.Format("2006-01-02"),
		}); err != nil {
		t.Fatalf("record rate %s->%s @%s: %v", from, to, rate, err)
	}
}

// ============================================================
// The effective-dated lookup
// ============================================================

// TestIntegration_FX_RateIsResolvedAsOfTheDateNotToday — a rate recorded
// after a claim was submitted must never reprice it, or a rate correction
// would silently rewrite settled figures.
func TestIntegration_FX_RateIsResolvedAsOfTheDateNotToday(t *testing.T) {
	env := newTestEnv(t)
	ctx := context.Background()
	f := seedFxFixture(t, env)
	today := time.Now().Truncate(24 * time.Hour)

	recordRate(t, env, f, "EUR", "BDT", "100", today.AddDate(0, 0, -30))
	recordRate(t, env, f, "EUR", "BDT", "110", today.AddDate(0, 0, -10))
	recordRate(t, env, f, "EUR", "BDT", "120", today.AddDate(0, 0, 5)) // future

	// As of 20 days ago, only the 30-day-old rate had happened.
	got, err := env.hrmFxSvc.RateAsOf(ctx, f.orgID, "EUR", "BDT", today.AddDate(0, 0, -20))
	if err != nil || got == nil {
		t.Fatalf("RateAsOf: %v (%v)", err, got)
	}
	if !got.Rate.Equal(decimal.RequireFromString("100")) {
		t.Errorf("rate as of 20 days ago = %s, want 100 — a later rate was applied retroactively",
			got.Rate)
	}

	// Today, the 10-day-old rate applies — NOT the future one.
	got, err = env.hrmFxSvc.RateAsOf(ctx, f.orgID, "EUR", "BDT", today)
	if err != nil || got == nil {
		t.Fatalf("RateAsOf today: %v", err)
	}
	if !got.Rate.Equal(decimal.RequireFromString("110")) {
		t.Errorf("rate today = %s, want 110 — a rate dated in the future was used", got.Rate)
	}

	// Before any rate existed there is no rate, and that is an answer.
	got, err = env.hrmFxSvc.RateAsOf(ctx, f.orgID, "EUR", "BDT", today.AddDate(0, 0, -60))
	if err != nil {
		t.Fatalf("RateAsOf before any rate: %v", err)
	}
	if got != nil {
		t.Errorf("a rate of %s was found before any was recorded", got.Rate)
	}
}

// TestIntegration_FX_InvertedRateSaysSo — an inverted rate is a DERIVED
// number, and a reader checking a conversion needs to know that.
func TestIntegration_FX_InvertedRateSaysSo(t *testing.T) {
	env := newTestEnv(t)
	ctx := context.Background()
	f := seedFxFixture(t, env)
	today := time.Now().Truncate(24 * time.Hour)
	recordRate(t, env, f, "EUR", "BDT", "109", today)

	direct, err := env.hrmFxSvc.RateAsOf(ctx, f.orgID, "EUR", "BDT", today)
	if err != nil || direct == nil {
		t.Fatalf("direct: %v", err)
	}
	if direct.Direction != fx.DirectionDirect {
		t.Errorf("direction = %q, want direct", direct.Direction)
	}

	inv, err := env.hrmFxSvc.RateAsOf(ctx, f.orgID, "BDT", "EUR", today)
	if err != nil || inv == nil {
		t.Fatalf("inverse: %v", err)
	}
	if inv.Direction != fx.DirectionInverted {
		t.Errorf("direction = %q, want inverted — a derived rate must not pass as a recorded one",
			inv.Direction)
	}
	if !inv.Rate.Equal(decimal.RequireFromString("0.00917431")) {
		t.Errorf("inverted rate = %s, want 0.00917431", inv.Rate)
	}
	if inv.SourceRateID != direct.SourceRateID {
		t.Error("the inverted rate does not point back at the row it was derived from")
	}
}

// TestIntegration_FX_SameCurrencyIsNotAConversion — recording a rate of 1
// would put a fabricated lookup into an audit trail whose purpose is to say
// where a number came from.
func TestIntegration_FX_SameCurrencyIsNotAConversion(t *testing.T) {
	env := newTestEnv(t)
	ctx := context.Background()
	f := seedFxFixture(t, env)
	today := time.Now().Truncate(24 * time.Hour)

	if _, err := env.hrmFxSvc.RecordRate(ctx, f.orgID, fxAdmin(f.ownerID), fx.RecordRateRequest{
		FromCurrency: "USD", ToCurrency: "USD", Rate: "1", RateDate: today.Format("2006-01-02"),
	}); !errors.Is(err, fx.ErrSameCurrency) {
		t.Errorf("recording a USD->USD rate returned %v, want ErrSameCurrency", err)
	}
	if _, err := env.hrmFxSvc.RateAsOf(ctx, f.orgID, "USD", "usd", today); !errors.Is(err, fx.ErrSameCurrency) {
		t.Errorf("resolving USD->USD returned %v, want ErrSameCurrency", err)
	}
}

// TestIntegration_FX_DuplicateAndInvalidRatesAreRefused
func TestIntegration_FX_DuplicateAndInvalidRatesAreRefused(t *testing.T) {
	env := newTestEnv(t)
	ctx := context.Background()
	f := seedFxFixture(t, env)
	today := time.Now().Truncate(24 * time.Hour)
	caller := fxAdmin(f.ownerID)
	dateStr := today.Format("2006-01-02")

	recordRate(t, env, f, "EUR", "BDT", "109", today)
	// Two rates for one pair on one day is two answers to a question that
	// has one, and the effective-dated lookup would pick arbitrarily.
	if _, err := env.hrmFxSvc.RecordRate(ctx, f.orgID, caller, fx.RecordRateRequest{
		FromCurrency: "EUR", ToCurrency: "BDT", Rate: "110", RateDate: dateStr,
	}); !errors.Is(err, fx.ErrDuplicateRate) {
		t.Errorf("a duplicate pair+date returned %v, want ErrDuplicateRate", err)
	}
	for _, bad := range []string{"0", "-1", "abc"} {
		if _, err := env.hrmFxSvc.RecordRate(ctx, f.orgID, caller, fx.RecordRateRequest{
			FromCurrency: "GBP", ToCurrency: "BDT", Rate: bad, RateDate: dateStr,
		}); !errors.Is(err, fx.ErrInvalidRate) {
			t.Errorf("rate %q returned %v, want ErrInvalidRate", bad, err)
		}
	}
	// Manage is required to record; reading is open.
	if _, err := env.hrmFxSvc.RecordRate(ctx, f.orgID, fx.Caller{UserID: f.ownerID},
		fx.RecordRateRequest{FromCurrency: "GBP", ToCurrency: "BDT", Rate: "150", RateDate: dateStr}); !errors.Is(err, fx.ErrAccessDenied) {
		t.Errorf("recording without manage returned %v, want ErrAccessDenied", err)
	}
	if _, err := env.hrmFxSvc.ListRates(ctx, f.orgID, nil, nil, 10); err != nil {
		t.Errorf("listing rates was refused: %v", err)
	}
}

// TestIntegration_FX_RatePrecisionSurvivesTheDatabase — NUMERIC(18,8), not
// (15,2). A rate rounded to money scale turns a real balance into nothing.
func TestIntegration_FX_RatePrecisionSurvivesTheDatabase(t *testing.T) {
	env := newTestEnv(t)
	ctx := context.Background()
	f := seedFxFixture(t, env)
	today := time.Now().Truncate(24 * time.Hour)

	recordRate(t, env, f, "BTC", "USD", "0.00000123", today)
	got, err := env.hrmFxSvc.RateAsOf(ctx, f.orgID, "BTC", "USD", today)
	if err != nil || got == nil {
		t.Fatalf("RateAsOf: %v", err)
	}
	if !got.Rate.Equal(decimal.RequireFromString("0.00000123")) {
		t.Fatalf("rate came back as %s, want 0.00000123 — the database rounded a rate", got.Rate)
	}
	res, err := env.hrmFxSvc.ConvertAsOf(ctx, f.orgID,
		decimal.RequireFromString("1000000000"), "BTC", "USD", today)
	if err != nil {
		t.Fatalf("ConvertAsOf: %v", err)
	}
	if !res.Available || !res.Conversion.ConvertedAmount.Equal(decimal.RequireFromString("1230")) {
		t.Errorf("1,000,000,000 x 0.00000123 = %v, want 1230 — rounding the rate first gives 0",
			res.Conversion)
	}
}

// ============================================================
// Gap 1 (8B): expense lines get a real rate source
// ============================================================

// TestIntegration_FX_ExpenseLineResolvesItsOwnRate closes the 8B gap: the
// rate was previously whatever the claimant typed, defaulting to 1.
func TestIntegration_FX_ExpenseLineResolvesItsOwnRate(t *testing.T) {
	env := newTestEnv(t)
	ctx := context.Background()
	f := seedFxFixture(t, env)
	today := time.Now().Truncate(24 * time.Hour)
	spent := today.AddDate(0, 0, -3)

	setOrgDefaults(t, env, f.orgID, "BD", "BDT", "Asia/Dhaka")
	recordRate(t, env, f, "EUR", "BDT", "109.5", spent)

	emp := seedEmployee(t, env, f.orgID, f.statusID, f.ownerID, "", "Traveller", nil)
	claim := seedExpenseClaim(t, env, f.orgID, emp, f.ownerID)

	line, err := env.hrmExpensesSvc.AddLine(ctx, f.orgID, claim, expenseLineReq("100", "EUR", spent, nil))
	if err != nil {
		t.Fatalf("AddLine: %v", err)
	}

	// ⚠ All five audit fields.
	if !line.Amount.Equal(decimal.RequireFromString("100")) {
		t.Errorf("original amount = %s, want 100", line.Amount)
	}
	if line.Currency != "EUR" {
		t.Errorf("original currency = %q, want EUR", line.Currency)
	}
	if !line.ExchangeRate.Equal(decimal.RequireFromString("109.5")) {
		t.Errorf("rate = %s, want 109.5 — the line did not resolve a rate from the table",
			line.ExchangeRate)
	}
	if line.ExchangeRateDate == nil {
		t.Fatal("no rate DATE stored — a stored rate that cannot be checked against the table " +
			"it came from is the converted-only case with extra steps")
	}
	if !line.ExchangeRateDate.Truncate(24 * time.Hour).Equal(spent) {
		t.Errorf("rate date = %s, want %s", line.ExchangeRateDate.Format("2006-01-02"),
			spent.Format("2006-01-02"))
	}
	if !line.BaseAmount.Equal(decimal.RequireFromString("10950")) {
		t.Errorf("base amount = %s, want 10950", line.BaseAmount)
	}
}

// TestIntegration_FX_CallerSuppliedRateStillWins — an org with a contractual
// or corporate-card rate must be able to state it, and a table lookup must
// not silently reprice their claim.
func TestIntegration_FX_CallerSuppliedRateStillWins(t *testing.T) {
	env := newTestEnv(t)
	ctx := context.Background()
	f := seedFxFixture(t, env)
	today := time.Now().Truncate(24 * time.Hour)
	spent := today.AddDate(0, 0, -3)

	setOrgDefaults(t, env, f.orgID, "BD", "BDT", "Asia/Dhaka")
	recordRate(t, env, f, "EUR", "BDT", "109.5", spent)
	emp := seedEmployee(t, env, f.orgID, f.statusID, f.ownerID, "", "Traveller", nil)
	claim := seedExpenseClaim(t, env, f.orgID, emp, f.ownerID)

	stated := "115"
	line, err := env.hrmExpensesSvc.AddLine(ctx, f.orgID, claim, expenseLineReq("100", "EUR", spent, &stated))
	if err != nil {
		t.Fatalf("AddLine: %v", err)
	}
	if !line.ExchangeRate.Equal(decimal.RequireFromString("115")) {
		t.Errorf("rate = %s, want the caller's 115 — a table lookup overrode a stated rate",
			line.ExchangeRate)
	}
	if !line.BaseAmount.Equal(decimal.RequireFromString("11500")) {
		t.Errorf("base = %s, want 11500", line.BaseAmount)
	}
}

// TestIntegration_FX_SameCurrencyLineRecordsNoConversion — a BDT line in a
// BDT org has not been through an FX lookup, and must not look as though it
// has.
func TestIntegration_FX_SameCurrencyLineRecordsNoConversion(t *testing.T) {
	env := newTestEnv(t)
	ctx := context.Background()
	f := seedFxFixture(t, env)
	spent := time.Now().Truncate(24*time.Hour).AddDate(0, 0, -1)
	setOrgDefaults(t, env, f.orgID, "BD", "BDT", "Asia/Dhaka")
	emp := seedEmployee(t, env, f.orgID, f.statusID, f.ownerID, "", "Local", nil)
	claim := seedExpenseClaim(t, env, f.orgID, emp, f.ownerID)

	line, err := env.hrmExpensesSvc.AddLine(ctx, f.orgID, claim, expenseLineReq("500", "BDT", spent, nil))
	if err != nil {
		t.Fatalf("AddLine: %v", err)
	}
	if line.ExchangeRateDate != nil {
		t.Errorf("a same-currency line recorded a rate date of %s — that is a fabricated lookup "+
			"in an audit trail", line.ExchangeRateDate.Format("2006-01-02"))
	}
	if !line.ExchangeRate.Equal(decimal.RequireFromString("1")) {
		t.Errorf("rate = %s, want 1", line.ExchangeRate)
	}
	if !line.BaseAmount.Equal(decimal.RequireFromString("500")) {
		t.Errorf("base = %s, want 500", line.BaseAmount)
	}
}

// TestIntegration_FX_NoRateLeavesTheLineUnconverted — with no rate recorded,
// the line behaves exactly as it did before 11B-1 existed.
func TestIntegration_FX_NoRateLeavesTheLineUnconverted(t *testing.T) {
	env := newTestEnv(t)
	ctx := context.Background()
	f := seedFxFixture(t, env)
	spent := time.Now().Truncate(24*time.Hour).AddDate(0, 0, -1)
	setOrgDefaults(t, env, f.orgID, "BD", "BDT", "Asia/Dhaka")
	emp := seedEmployee(t, env, f.orgID, f.statusID, f.ownerID, "", "Traveller", nil)
	claim := seedExpenseClaim(t, env, f.orgID, emp, f.ownerID)

	line, err := env.hrmExpensesSvc.AddLine(ctx, f.orgID, claim, expenseLineReq("100", "EUR", spent, nil))
	if err != nil {
		t.Fatalf("AddLine: %v", err)
	}
	if line.ExchangeRateDate != nil {
		t.Error("a rate date was stored though no rate exists for EUR->BDT")
	}
	if !line.ExchangeRate.Equal(decimal.RequireFromString("1")) {
		t.Errorf("rate = %s, want 1 (the pre-11B behaviour)", line.ExchangeRate)
	}
	if !line.BaseAmount.Equal(decimal.RequireFromString("100")) {
		t.Errorf("base = %s, want 100 unconverted", line.BaseAmount)
	}
}

// ============================================================
// Gap 2 (9B): the foreign travel advance in F&F
// ============================================================

// TestIntegration_FX_ForeignAdvanceConvertsWhenARateExists closes the 9B gap.
func TestIntegration_FX_ForeignAdvanceConvertsWhenARateExists(t *testing.T) {
	env := newTestEnv(t)
	ctx := context.Background()
	f := seedFxFixture(t, env)
	lwd := time.Now().Truncate(24*time.Hour).AddDate(0, 0, -5)
	recordRate(t, env, f, "EUR", "BDT", "109", lwd.AddDate(0, 0, -1))

	got, err := env.hrmFxSvc.ConvertAsOf(ctx, f.orgID,
		decimal.RequireFromString("500"), "EUR", "BDT", lwd)
	if err != nil {
		t.Fatalf("ConvertAsOf: %v", err)
	}
	if !got.Available {
		t.Fatal("no rate found though one was recorded before the last working date")
	}
	c := got.Conversion
	// ⚠ All five fields, checked individually — this is the record a
	// departing person would need to dispute the figure.
	if !c.OriginalAmount.Equal(decimal.RequireFromString("500")) ||
		c.OriginalCurrency != "EUR" ||
		!c.Rate.Equal(decimal.RequireFromString("109")) ||
		c.RateDate.IsZero() ||
		!c.ConvertedAmount.Equal(decimal.RequireFromString("54500")) {
		t.Errorf("conversion = %+v, want 500 EUR @109 on a real date = 54500", c)
	}
	if c.TargetCurrency != "BDT" {
		t.Errorf("target = %q, want BDT", c.TargetCurrency)
	}
}

// TestIntegration_FX_NoRateNeverConvertsAtParity is the most important test
// in the slice.
//
// Phase 9B refused to convert foreign advances because "converting a
// foreign-currency advance would mean inventing a rate and mis-charging a
// departing person real money". 11B-1 must give it a rate WITHOUT weakening
// that refusal: with no rate, nothing converts, and certainly not at 1:1.
func TestIntegration_FX_NoRateNeverConvertsAtParity(t *testing.T) {
	env := newTestEnv(t)
	ctx := context.Background()
	f := seedFxFixture(t, env)
	lwd := time.Now().Truncate(24 * time.Hour)

	// No rate recorded at all.
	got, err := env.hrmFxSvc.ConvertAsOf(ctx, f.orgID,
		decimal.RequireFromString("500"), "EUR", "BDT", lwd)
	if err != nil {
		t.Fatalf("ConvertAsOf: %v", err)
	}
	if got.Available {
		t.Fatalf("a conversion was produced with no rate recorded: %+v", got.Conversion)
	}
	if got.Conversion != nil {
		t.Errorf("an unavailable result still carried a conversion: %+v — a caller could use it",
			got.Conversion)
	}

	// A rate recorded AFTER the date must not be reached back for.
	recordRate(t, env, f, "EUR", "BDT", "109", lwd.AddDate(0, 0, 10))
	got, err = env.hrmFxSvc.ConvertAsOf(ctx, f.orgID,
		decimal.RequireFromString("500"), "EUR", "BDT", lwd)
	if err != nil {
		t.Fatalf("ConvertAsOf after future rate: %v", err)
	}
	if got.Available {
		t.Errorf("a rate dated 10 days after the settlement was used to price it: %+v",
			got.Conversion)
	}
}

// TestIntegration_FX_SettlementLineStoresAllFiveFields asserts the audit set
// lands in the DATABASE, where chk_hrm_esl_conversion_complete enforces
// all-or-nothing.
func TestIntegration_FX_SettlementLineStoresAllFiveFields(t *testing.T) {
	env := newTestEnv(t)
	ctx := context.Background()
	f := seedFxFixture(t, env)
	lwd := time.Now().Truncate(24*time.Hour).AddDate(0, 0, -5)

	emp := seedEmployee(t, env, f.orgID, f.statusID, f.ownerID, "", "Departing", nil)
	var exitID string
	if err := env.db.QueryRow(ctx,
		`INSERT INTO hrm_exits (org_id, employee_id, source_type, source_id, last_working_date,
		     status, created_by)
		 VALUES ($1,$2,'resignation',gen_random_uuid(),$3,'completed',$4) RETURNING id`,
		f.orgID, emp, lwd, f.ownerID).Scan(&exitID); err != nil {
		t.Fatalf("seed exit: %v", err)
	}

	// ⚠ A half-recorded conversion must be refused by the database, not just
	// by Go. This is the all-or-nothing CHECK.
	_, err := env.db.Exec(ctx,
		`INSERT INTO hrm_exit_settlement_lines
		   (exit_id, source_type, description, amount, is_credit, currency, original_amount)
		 VALUES ($1,'travel_advance','half a conversion',100,false,'BDT',50)`, exitID)
	if err == nil {
		t.Error("the database accepted an original_amount with no rate or rate_date — a " +
			"half-recorded conversion is the converted-only case wearing four columns")
	} else if !strings.Contains(err.Error(), "chk_hrm_esl_conversion_complete") {
		t.Errorf("refused for the wrong reason: %v", err)
	}

	// The complete set is accepted.
	if _, err := env.db.Exec(ctx,
		`INSERT INTO hrm_exit_settlement_lines
		   (exit_id, source_type, description, amount, is_credit, currency,
		    original_amount, original_currency, exchange_rate, exchange_rate_date)
		 VALUES ($1,'travel_advance','converted advance',54500,false,'BDT',500,'EUR',109,$2)`,
		exitID, lwd); err != nil {
		t.Fatalf("a complete conversion was refused: %v", err)
	}

	var orig decimal.Decimal
	var origCur string
	var rate decimal.Decimal
	var rateDate time.Time
	if err := env.db.QueryRow(ctx,
		`SELECT original_amount, original_currency, exchange_rate, exchange_rate_date
		   FROM hrm_exit_settlement_lines WHERE exit_id=$1 AND original_amount IS NOT NULL`,
		exitID).Scan(&orig, &origCur, &rate, &rateDate); err != nil {
		t.Fatalf("read back: %v", err)
	}
	if !orig.Equal(decimal.RequireFromString("500")) || origCur != "EUR" ||
		!rate.Equal(decimal.RequireFromString("109")) || rateDate.IsZero() {
		t.Errorf("stored set = %s %s @%s on %s", orig, origCur, rate, rateDate)
	}
}

// TestIntegration_FX_SingleCurrencyOrgIsUnaffected is the regression guard.
// An org that never touches a foreign currency must behave identically.
func TestIntegration_FX_SingleCurrencyOrgIsUnaffected(t *testing.T) {
	env := newTestEnv(t)
	ctx := context.Background()
	f := seedFxFixture(t, env)
	spent := time.Now().Truncate(24*time.Hour).AddDate(0, 0, -1)
	emp := seedEmployee(t, env, f.orgID, f.statusID, f.ownerID, "", "Local", nil)
	claim := seedExpenseClaim(t, env, f.orgID, emp, f.ownerID)

	// No org currency configured, no rates recorded — the state of every
	// organization in this database.
	line, err := env.hrmExpensesSvc.AddLine(ctx, f.orgID, claim, expenseLineReq("250.50", "USD", spent, nil))
	if err != nil {
		t.Fatalf("AddLine: %v", err)
	}
	if !line.BaseAmount.Equal(decimal.RequireFromString("250.50")) {
		t.Errorf("base = %s, want 250.50 unconverted", line.BaseAmount)
	}
	if line.ExchangeRateDate != nil {
		t.Error("a rate date was stored for an org with no currency and no rates")
	}
	if rates, err := env.hrmFxSvc.ListRates(ctx, f.orgID, nil, nil, 10); err != nil || len(rates) != 0 {
		t.Errorf("rates = %v (err %v), want none", rates, err)
	}
}

// ── helpers ──────────────────────────────────────────────────────────────────

func seedExpenseClaim(t *testing.T, env *testEnv, orgID, employeeID, createdBy string) string {
	t.Helper()
	var id string
	if err := env.db.QueryRow(context.Background(),
		`INSERT INTO hrm_expense_claims (org_id, employee_id, title, created_by)
		 VALUES ($1,$2,$3,$4) RETURNING id`,
		orgID, employeeID, fmt.Sprintf("Trip %d", time.Now().UnixNano()), createdBy).Scan(&id); err != nil {
		t.Fatalf("seed expense claim: %v", err)
	}
	return id
}

// expenseLineReq builds a line request. rate == nil exercises the 11B-1
// lookup path; a non-nil rate exercises the caller-supplied path that still
// takes precedence.
func expenseLineReq(amount, currency string, spent time.Time, rate *string) hrmexpenses.CreateLineRequest {
	amt := amount
	cur := currency
	return hrmexpenses.CreateLineRequest{
		Category:     "ground_transport",
		Description:  strPtr("Taxi"),
		ExpenseDate:  spent.Format("2006-01-02"),
		Amount:       &amt,
		Currency:     &cur,
		ExchangeRate: rate,
	}
}

// ============================================================
// Gap 2, end to end through the real F&F settlement
// ============================================================

// TestIntegration_FX_FnFForeignAdvanceIsRecoveredWhenARateExists is the 9B
// gap closed through the ACTUAL settlement path, not just the FX service.
//
// Its counterpart —
// TestIntegration_FnFSources_ForeignCurrencyAdvanceIsReportedNotGuessed —
// records no rate and must still report the advance at zero. The two together
// are the whole claim: a rate converts, and its absence still refuses.
func TestIntegration_FX_FnFForeignAdvanceIsRecoveredWhenARateExists(t *testing.T) {
	env := newTestEnv(t)
	ctx := context.Background()
	fxf := seedFnF(t, env, "40000")
	addComponent(t, env, fxf.payrollFixture, "Basic Pay", "earning", "pct_of_basic", "100", nil, 1)

	var runCurrency string
	var lwd time.Time
	if err := env.db.QueryRow(ctx,
		`SELECT r.currency, e.last_working_date
		   FROM hrm_payslip_runs r JOIN hrm_exits e ON e.id = $2
		  WHERE r.id = $1`, fxf.runID, fxf.exitID).Scan(&runCurrency, &lwd); err != nil {
		t.Fatalf("read run currency and last working date: %v", err)
	}
	foreign := "EUR"
	if runCurrency == "EUR" {
		foreign = "JPY"
	}

	// A rate recorded BEFORE the last working date, so the effective-dated
	// lookup finds it.
	if _, err := env.hrmFxSvc.RecordRate(ctx, fxf.orgID, fxAdmin(fxf.ownerID), fx.RecordRateRequest{
		FromCurrency: foreign, ToCurrency: runCurrency, Rate: "100",
		RateDate: lwd.AddDate(0, 0, -1).Format("2006-01-02"),
	}); err != nil {
		t.Fatalf("record rate: %v", err)
	}

	advanceID := seedFnFAdvance(t, env, fxf, "500.00", foreign)
	if _, err := env.hrmPayslipsSvc.ComputeRun(ctx, fxf.orgID, fxf.runID, fxf.ownerID); err != nil {
		t.Fatalf("compute: %v", err)
	}

	line := settlementLineFor(t, env, fxf, "travel_advance")
	if line == nil {
		t.Fatal("the advance line vanished")
	}
	// 500 foreign x 100 = 50,000 in the run currency.
	if !line.Amount.Equal(decimal.RequireFromString("50000")) {
		t.Errorf("recovered amount = %s, want 50000 — the advance was not converted", line.Amount)
	}
	if line.Currency != runCurrency {
		t.Errorf("line currency = %q, want the run's %q — amount and currency stay the "+
			"CONVERTED pair", line.Currency, runCurrency)
	}
	if strings.Contains(line.Description, "NOT RECOVERED") {
		t.Errorf("description still says NOT RECOVERED though a rate exists: %q", line.Description)
	}

	// ⚠ All five audit fields on the stored line.
	if line.OriginalAmount == nil || !line.OriginalAmount.Equal(decimal.RequireFromString("500")) {
		t.Errorf("original_amount = %v, want 500", line.OriginalAmount)
	}
	if line.OriginalCurrency == nil || *line.OriginalCurrency != foreign {
		t.Errorf("original_currency = %v, want %s", line.OriginalCurrency, foreign)
	}
	if line.ExchangeRate == nil || !line.ExchangeRate.Equal(decimal.RequireFromString("100")) {
		t.Errorf("exchange_rate = %v, want 100", line.ExchangeRate)
	}
	if line.ExchangeRateDate == nil {
		t.Error("no exchange_rate_date — the rate cannot be checked against the table it came from")
	}
	// The description has to state the rate, because this is the line a
	// departing person would dispute.
	if !strings.Contains(line.Description, "100") {
		t.Errorf("description %q does not state the rate applied", line.Description)
	}

	// And the advance IS now marked settled, unlike the no-rate case.
	var settled decimal.Decimal
	if err := env.db.QueryRow(ctx,
		`SELECT settled_amount FROM hrm_travel_advances WHERE id=$1`, advanceID).Scan(&settled); err != nil {
		t.Fatalf("read advance: %v", err)
	}
	if settled.IsZero() {
		t.Error("the advance was charged for but not marked settled — it would be recovered twice")
	}
}
