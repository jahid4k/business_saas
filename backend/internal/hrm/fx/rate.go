// backend/internal/hrm/fx/rate.go
package fx

import (
	"errors"
	"strings"
	"time"

	"github.com/shopspring/decimal"
)

var (
	ErrNoRate          = errors.New("fx: no exchange rate available for this pair as of this date")
	ErrInvalidRate     = errors.New("fx: a rate must be greater than zero")
	ErrInvalidCurrency = errors.New("fx: currency must be a three-letter ISO 4217 code")
	ErrSameCurrency    = errors.New("fx: a rate cannot convert a currency to itself")
	ErrInvalidDate     = errors.New("fx: rate_date must be a real date")
)

// MoneyScale is the number of decimal places a converted amount is rounded
// to. The RESULT of a conversion is money; the rate is not.
const MoneyScale = 2

// Conversion is the complete record of one currency conversion.
//
// ⚠ ALL FIVE FIELDS TRAVEL TOGETHER, ALWAYS. A converted figure stored
// without its rate and rate date cannot be audited, cannot be recomputed when
// a rate is corrected, and cannot be explained to the person whose settlement
// it reduced. Every table that stores a converted amount in this system
// stores this whole shape — hrm_expense_lines and hrm_exit_settlement_lines
// both carry a CHECK making it all-or-nothing.
type Conversion struct {
	OriginalAmount   decimal.Decimal `json:"original_amount"`
	OriginalCurrency string          `json:"original_currency"`
	Rate             decimal.Decimal `json:"exchange_rate"`
	RateDate         time.Time       `json:"exchange_rate_date"`
	ConvertedAmount  decimal.Decimal `json:"converted_amount"`
	TargetCurrency   string          `json:"target_currency"`
}

// Convert applies a rate and returns the complete five-field record.
//
// ⚠ THE RESULT ROUNDS TO 2 PLACES; THE RATE NEVER ROUNDS. A rate of
// 0.00000123 multiplied by a large balance is a real amount, and rounding the
// rate to 0.00 first turns it into nothing. Only money rounds.
//
// ⚠ A ZERO OR NEGATIVE RATE IS REFUSED, NOT TREATED AS 1. Falling back to 1
// is the exact mis-charge that made Phase 9B refuse to convert at all: it
// silently reprices a foreign balance as though the currencies were at par,
// and the person it charges has no way to see that it happened.
func Convert(amount decimal.Decimal, from, to string, rate decimal.Decimal, rateDate time.Time) (Conversion, error) {
	from, to = NormaliseCurrency(from), NormaliseCurrency(to)
	if !ValidCurrency(from) || !ValidCurrency(to) {
		return Conversion{}, ErrInvalidCurrency
	}
	if rate.LessThanOrEqual(decimal.Zero) {
		return Conversion{}, ErrInvalidRate
	}
	if rateDate.IsZero() {
		return Conversion{}, ErrInvalidDate
	}
	return Conversion{
		OriginalAmount:   amount,
		OriginalCurrency: from,
		Rate:             rate,
		RateDate:         rateDate,
		ConvertedAmount:  amount.Mul(rate).Round(MoneyScale),
		TargetCurrency:   to,
	}, nil
}

// SameCurrency reports whether a conversion is needed at all.
//
// ⚠ Converting a currency to itself is NOT a conversion, and callers must not
// record one. Writing a rate of 1 with today's date would put a fabricated
// "rate" into an audit trail whose whole purpose is to say where a number
// came from — and it would make every same-currency line look like it had
// been through an FX lookup that never happened.
func SameCurrency(a, b string) bool {
	return NormaliseCurrency(a) == NormaliseCurrency(b)
}

// Invert produces the reciprocal rate.
//
// ⚠ It does NOT round. An inverted rate is still a rate, and rounding it to
// money scale here would corrupt every amount it later multiplies. The
// division keeps enough places that a round trip stays faithful to the money
// scale, which is what InvertRoundTripsToMoneyScale pins.
func Invert(rate decimal.Decimal) (decimal.Decimal, error) {
	if rate.LessThanOrEqual(decimal.Zero) {
		return decimal.Zero, ErrInvalidRate
	}
	return decimal.NewFromInt(1).DivRound(rate, RateScale), nil
}

// RateScale is the working precision for rates. It matches
// hrm_exchange_rates.rate NUMERIC(18,8) so an inverted or stored rate never
// loses places between Go and the database.
const RateScale = 8

// NormaliseCurrency uppercases and trims an ISO 4217 code so "eur" from an
// import and "EUR" from the UI cannot become two different currencies.
func NormaliseCurrency(c string) string {
	return strings.ToUpper(strings.TrimSpace(c))
}

// ValidCurrency checks the shape of an ISO 4217 code. It deliberately does
// not check membership of a currency list: a hardcoded list goes stale, and
// refusing an org's real currency because this file has not heard of it is
// worse than accepting a typo that will fail at the rate lookup anyway.
func ValidCurrency(c string) bool {
	if len(c) != 3 {
		return false
	}
	for _, r := range c {
		if r < 'A' || r > 'Z' {
			return false
		}
	}
	return true
}
