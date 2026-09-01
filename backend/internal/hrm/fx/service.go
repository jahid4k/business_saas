// backend/internal/hrm/fx/service.go
package fx

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/shopspring/decimal"
)

// Caller carries the manage authority the route gate established.
type Caller struct {
	UserID    string
	CanManage bool
}

// Service is the FX business layer.
//
// ⚠ RateAsOf and ConvertAsOf are what OTHER PACKAGES consume, through their
// own narrow interfaces. internal/hrm/expenses and internal/hrm/exits each
// declare a RateSource of their own that this type satisfies structurally —
// neither imports this package's Service, and there is no adapter.
type Service interface {
	RecordRate(ctx context.Context, orgID string, caller Caller, req RecordRateRequest) (*ExchangeRate, error)
	ListRates(ctx context.Context, orgID string, from, to *string, limit int) ([]*ExchangeRate, error)

	// RateAsOf resolves the rate that applied on a date, or reports that
	// none did.
	RateAsOf(ctx context.Context, orgID, from, to string, asOf time.Time) (*ResolvedRate, error)
	// ConvertAsOf resolves a rate and applies it in one step.
	ConvertAsOf(ctx context.Context, orgID string, amount decimal.Decimal, from, to string, asOf time.Time) (*ConversionResult, error)

	// RateAsOfPrimitive is RateAsOf in primitives, and exists so consumers
	// can declare a one-method interface this type satisfies structurally.
	//
	// ⚠ ok=false means no rate exists — a normal answer. It is never an
	// invitation to substitute 1.
	RateAsOfPrimitive(ctx context.Context, orgID, from, to string, asOf time.Time) (
		rate decimal.Decimal, rateDate time.Time, ok bool, err error)
}

type serviceImpl struct{ repo Repository }

func NewService(repo Repository) Service { return &serviceImpl{repo: repo} }

func (s *serviceImpl) RecordRate(ctx context.Context, orgID string, caller Caller, req RecordRateRequest) (*ExchangeRate, error) {
	if !caller.CanManage {
		return nil, ErrAccessDenied
	}
	from, to := NormaliseCurrency(req.FromCurrency), NormaliseCurrency(req.ToCurrency)
	if !ValidCurrency(from) || !ValidCurrency(to) {
		return nil, ErrInvalidCurrency
	}
	if from == to {
		return nil, ErrSameCurrency
	}
	rate, err := decimal.NewFromString(strings.TrimSpace(req.Rate))
	if err != nil {
		return nil, ErrInvalidRate
	}
	if rate.LessThanOrEqual(decimal.Zero) {
		return nil, ErrInvalidRate
	}
	rateDate, err := time.Parse("2006-01-02", strings.TrimSpace(req.RateDate))
	if err != nil {
		return nil, fmt.Errorf("fx: RecordRate: rate_date must be YYYY-MM-DD: %w", err)
	}
	source := "manual"
	if req.Source != nil && strings.TrimSpace(*req.Source) != "" {
		source = strings.ToLower(strings.TrimSpace(*req.Source))
		if source != "manual" && source != "import" {
			return nil, ErrInvalidSource
		}
	}

	e := &ExchangeRate{
		OrgID: orgID, FromCurrency: from, ToCurrency: to, Rate: rate,
		RateDate: rateDate, Source: source, Note: req.Note, CreatedBy: caller.UserID,
	}
	if err := s.repo.CreateRate(ctx, e); err != nil {
		return nil, err
	}
	return e, nil
}

func (s *serviceImpl) ListRates(ctx context.Context, orgID string, from, to *string, limit int) ([]*ExchangeRate, error) {
	return s.repo.ListRates(ctx, orgID, from, to, limit)
}

// RateAsOf resolves the rate that applied on a date.
//
// It tries the direct pair first, then the inverse. ⚠ An inverted rate is a
// DERIVED number and the result says so — a reader checking why a conversion
// produced its figure needs to know whether the rate was recorded as written
// or computed from its reciprocal.
//
// ⚠ Returns (nil, nil) when no rate exists in either direction. That is a
// normal outcome the caller must handle, not an error to swallow. Every
// caller in this codebase is required to degrade visibly rather than assume
// parity.
func (s *serviceImpl) RateAsOf(ctx context.Context, orgID, from, to string, asOf time.Time) (*ResolvedRate, error) {
	from, to = NormaliseCurrency(from), NormaliseCurrency(to)
	if !ValidCurrency(from) || !ValidCurrency(to) {
		return nil, ErrInvalidCurrency
	}
	// A currency converted to itself is not a conversion and must not
	// manufacture a rate of 1 in an audit trail.
	if from == to {
		return nil, ErrSameCurrency
	}

	direct, err := s.repo.FindDirectAsOf(ctx, orgID, from, to, asOf)
	if err != nil {
		return nil, err
	}
	if direct != nil {
		return &ResolvedRate{
			Rate: direct.Rate, RateDate: direct.RateDate,
			Direction: DirectionDirect, SourceRateID: direct.ID,
		}, nil
	}

	reverse, err := s.repo.FindDirectAsOf(ctx, orgID, to, from, asOf)
	if err != nil {
		return nil, err
	}
	if reverse == nil {
		return nil, nil
	}
	inverted, err := Invert(reverse.Rate)
	if err != nil {
		return nil, err
	}
	return &ResolvedRate{
		Rate: inverted, RateDate: reverse.RateDate,
		Direction: DirectionInverted, SourceRateID: reverse.ID,
	}, nil
}

// ConvertAsOf resolves a rate and applies it.
//
// ⚠ When no rate exists it returns Available=false with NO conversion — never
// a conversion at parity. Falling back to 1 is the mis-charge Phase 9B
// refused to make, and doing it here would push that mistake into every
// consumer at once.
func (s *serviceImpl) ConvertAsOf(ctx context.Context, orgID string, amount decimal.Decimal, from, to string, asOf time.Time) (*ConversionResult, error) {
	if SameCurrency(from, to) {
		return nil, ErrSameCurrency
	}
	resolved, err := s.RateAsOf(ctx, orgID, from, to, asOf)
	if err != nil {
		return nil, err
	}
	if resolved == nil {
		return &ConversionResult{Available: false}, nil
	}
	conv, err := Convert(amount, from, to, resolved.Rate, resolved.RateDate)
	if err != nil {
		return nil, err
	}
	return &ConversionResult{Available: true, Conversion: &conv, Resolved: resolved}, nil
}

// RateAsOfPrimitive adapts RateAsOf to primitives for consumer-owned narrow
// interfaces (expenses.RateSource, exits.RateSource). Same resolution, same
// refusals; only the shape differs.
func (s *serviceImpl) RateAsOfPrimitive(ctx context.Context, orgID, from, to string, asOf time.Time) (decimal.Decimal, time.Time, bool, error) {
	resolved, err := s.RateAsOf(ctx, orgID, from, to, asOf)
	if err != nil {
		// A same-currency request is not a failure, it is "no conversion
		// needed" — reported as unavailable so a caller cannot treat it as
		// a rate of 1 it may record.
		if errors.Is(err, ErrSameCurrency) {
			return decimal.Zero, time.Time{}, false, nil
		}
		return decimal.Zero, time.Time{}, false, err
	}
	if resolved == nil {
		return decimal.Zero, time.Time{}, false, nil
	}
	return resolved.Rate, resolved.RateDate, true, nil
}
