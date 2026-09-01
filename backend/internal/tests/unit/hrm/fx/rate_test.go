// backend/internal/tests/unit/hrm/fx/rate_test.go
package fx_test

import (
	"errors"
	"testing"
	"time"

	"github.com/mridha/businesssaas/internal/hrm/fx"
	"github.com/shopspring/decimal"
)

func d(s string) decimal.Decimal { return decimal.RequireFromString(s) }
func day(s string) time.Time {
	t, err := time.Parse("2006-01-02", s)
	if err != nil {
		panic(err)
	}
	return t
}

// TestConvert_RoundsTheResultNeverTheRate is the central money rule.
//
// A rate is not money. Rounding 0.00000123 to two places gives 0.00 and turns
// a real balance into nothing; rounding 1.0857 to 1.09 misprices every line it
// touches. Only the RESULT of a conversion is money.
func TestConvert_RoundsTheResultNeverTheRate(t *testing.T) {
	cases := []struct {
		name          string
		amount, rate  string
		wantConverted string
	}{
		{
			name: "a tiny rate is not rounded away",
			// 1,000,000,000 x 0.00000123 = 1230.00 exactly. If the rate were
			// rounded to 2 places first it would be 0.00 and the answer 0.
			amount: "1000000000", rate: "0.00000123", wantConverted: "1230",
		},
		{
			name:   "a four-place rate is applied in full",
			amount: "1000", rate: "1.0857", wantConverted: "1085.7",
		},
		{
			name:   "rounding a 1.0857 rate to 1.09 would give 1090 — it must not",
			amount: "10000", rate: "1.0857", wantConverted: "10857",
		},
		{
			name:   "the result rounds half-up to paisa",
			amount: "3", rate: "1.005", wantConverted: "3.02",
		},
		{
			name:   "a large balance at a high rate stays exact",
			amount: "123456.78", rate: "109.00000000", wantConverted: "13456789.02",
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			conv, err := fx.Convert(d(c.amount), "EUR", "BDT", d(c.rate), day("2026-01-15"))
			if err != nil {
				t.Fatalf("Convert: %v", err)
			}
			if !conv.ConvertedAmount.Equal(d(c.wantConverted)) {
				t.Errorf("converted = %s, want %s", conv.ConvertedAmount, c.wantConverted)
			}
			// ⚠ The rate must come back exactly as supplied.
			if !conv.Rate.Equal(d(c.rate)) {
				t.Errorf("rate came back as %s, want %s — the rate must never be rounded",
					conv.Rate, c.rate)
			}
		})
	}
}

// TestConvert_RecordsAllFiveFields — a converted figure without its rate and
// rate date cannot be audited, recomputed after a rate correction, or
// explained to the person whose settlement it reduced.
func TestConvert_RecordsAllFiveFields(t *testing.T) {
	conv, err := fx.Convert(d("100"), "eur", "bdt", d("109.5"), day("2026-03-01"))
	if err != nil {
		t.Fatalf("Convert: %v", err)
	}
	if !conv.OriginalAmount.Equal(d("100")) {
		t.Errorf("original amount = %s, want 100", conv.OriginalAmount)
	}
	if conv.OriginalCurrency != "EUR" {
		t.Errorf("original currency = %q, want EUR (normalised from 'eur')", conv.OriginalCurrency)
	}
	if !conv.Rate.Equal(d("109.5")) {
		t.Errorf("rate = %s, want 109.5", conv.Rate)
	}
	if !conv.RateDate.Equal(day("2026-03-01")) {
		t.Errorf("rate date = %s, want 2026-03-01", conv.RateDate.Format("2006-01-02"))
	}
	if !conv.ConvertedAmount.Equal(d("10950")) {
		t.Errorf("converted = %s, want 10950", conv.ConvertedAmount)
	}
	if conv.TargetCurrency != "BDT" {
		t.Errorf("target currency = %q, want BDT", conv.TargetCurrency)
	}
}

// TestConvert_RefusesRatherThanFallingBackToOne is the rule Phase 9B's
// refusal was protecting.
//
// A rate of 1 silently reprices a foreign balance as though the currencies
// were at par. The person it charges has no way to see that it happened,
// which is why an absent or invalid rate must be an error the caller has to
// handle, not a default it can ignore.
func TestConvert_RefusesRatherThanFallingBackToOne(t *testing.T) {
	for _, bad := range []string{"0", "-1", "-0.5"} {
		conv, err := fx.Convert(d("100"), "EUR", "BDT", d(bad), day("2026-01-01"))
		if !errors.Is(err, fx.ErrInvalidRate) {
			t.Errorf("rate %s returned %v, want ErrInvalidRate", bad, err)
		}
		if !conv.ConvertedAmount.IsZero() {
			t.Errorf("rate %s produced a converted amount of %s — a refusal must produce "+
				"nothing, not a figure somebody might use", bad, conv.ConvertedAmount)
		}
	}
	if _, err := fx.Convert(d("100"), "EUR", "BDT", d("1.09"), time.Time{}); !errors.Is(err, fx.ErrInvalidDate) {
		t.Errorf("a zero rate date returned %v, want ErrInvalidDate — a rate with no date "+
			"cannot be checked against the table it came from", err)
	}
	for _, bad := range []string{"EU", "EURO", "e u r", ""} {
		if _, err := fx.Convert(d("100"), bad, "BDT", d("1.09"), day("2026-01-01")); !errors.Is(err, fx.ErrInvalidCurrency) {
			t.Errorf("currency %q returned %v, want ErrInvalidCurrency", bad, err)
		}
	}
}

// TestSameCurrency_IsNotAConversion — recording a rate of 1 for USD→USD would
// put a fabricated rate into an audit trail whose whole purpose is to say
// where a number came from.
func TestSameCurrency_IsNotAConversion(t *testing.T) {
	for _, c := range [][2]string{{"USD", "USD"}, {"usd", "USD"}, {" EUR ", "eur"}} {
		if !fx.SameCurrency(c[0], c[1]) {
			t.Errorf("SameCurrency(%q,%q) = false, want true", c[0], c[1])
		}
	}
	if fx.SameCurrency("USD", "EUR") {
		t.Error("SameCurrency(USD,EUR) = true")
	}
}

// TestInvert_DoesNotRoundToMoneyScale — an inverted rate is still a rate.
func TestInvert_DoesNotRoundToMoneyScale(t *testing.T) {
	// 1/109 = 0.00917431..., which rounds to 0.01 at money scale — a 9%
	// error on every amount it touches.
	inv, err := fx.Invert(d("109"))
	if err != nil {
		t.Fatalf("Invert: %v", err)
	}
	if inv.Equal(d("0.01")) {
		t.Fatal("the inverted rate was rounded to money scale")
	}
	if !inv.Equal(d("0.00917431")) {
		t.Errorf("1/109 = %s, want 0.00917431 at rate scale", inv)
	}
	if _, err := fx.Invert(decimal.Zero); !errors.Is(err, fx.ErrInvalidRate) {
		t.Errorf("inverting zero returned %v, want ErrInvalidRate", err)
	}
}

// TestInvert_RoundTripsToMoneyScale — converting out and back must land on
// the original amount once rounded to money, or the rate scale is too small.
func TestInvert_RoundTripsToMoneyScale(t *testing.T) {
	for _, rate := range []string{"109", "1.0857", "0.75", "3"} {
		fwd := d(rate)
		back, err := fx.Invert(fwd)
		if err != nil {
			t.Fatalf("Invert(%s): %v", rate, err)
		}
		original := d("1000")
		out, _ := fx.Convert(original, "EUR", "BDT", fwd, day("2026-01-01"))
		home, _ := fx.Convert(out.ConvertedAmount, "BDT", "EUR", back, day("2026-01-01"))
		if home.ConvertedAmount.Sub(original).Abs().GreaterThan(d("0.01")) {
			t.Errorf("rate %s: 1000 -> %s -> %s, drifted more than a paisa on the round trip",
				rate, out.ConvertedAmount, home.ConvertedAmount)
		}
	}
}

func TestValidCurrency(t *testing.T) {
	for _, ok := range []string{"USD", "EUR", "BDT", "XYZ"} {
		if !fx.ValidCurrency(ok) {
			t.Errorf("%q rejected", ok)
		}
	}
	for _, bad := range []string{"", "US", "USDD", "us d", "12A", "usd"} {
		if fx.ValidCurrency(bad) {
			t.Errorf("%q accepted", bad)
		}
	}
}
