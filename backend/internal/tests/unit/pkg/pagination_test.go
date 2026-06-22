// backend/internal/tests/unit/pkg/pagination_test.go
package pkg

import (
	"testing"

	"github.com/mridha/businesssaas/pkg/pagination"
)

func TestPaginationParams_Page_FirstPage(t *testing.T) {
	p := pagination.Params{Limit: 50, Offset: 0}
	if got := p.Page(); got != 1 {
		t.Errorf("Page() = %d, want 1", got)
	}
}

func TestPaginationParams_Page_SecondPage(t *testing.T) {
	p := pagination.Params{Limit: 50, Offset: 50}
	if got := p.Page(); got != 2 {
		t.Errorf("Page() = %d, want 2", got)
	}
}

func TestPaginationParams_Page_ThirdPage(t *testing.T) {
	p := pagination.Params{Limit: 10, Offset: 20}
	if got := p.Page(); got != 3 {
		t.Errorf("Page() = %d, want 3", got)
	}
}

func TestPaginationParams_Page_ZeroLimit(t *testing.T) {
	p := pagination.Params{Limit: 0, Offset: 0}
	if got := p.Page(); got != 1 {
		t.Errorf("Page() with zero limit = %d, want 1 (guard against div-by-zero)", got)
	}
}

func TestNewMeta_Fields(t *testing.T) {
	p := pagination.Params{Limit: 25, Offset: 50}
	m := pagination.NewMeta(p, 200)
	if m.Total != 200 {
		t.Errorf("Meta.Total = %d, want 200", m.Total)
	}
	if m.Limit != 25 {
		t.Errorf("Meta.Limit = %d, want 25", m.Limit)
	}
	if m.Offset != 50 {
		t.Errorf("Meta.Offset = %d, want 50", m.Offset)
	}
}

func TestPaginationConstants(t *testing.T) {
	if pagination.DefaultLimit <= 0 {
		t.Errorf("DefaultLimit must be positive, got %d", pagination.DefaultLimit)
	}
	if pagination.MaxLimit < pagination.DefaultLimit {
		t.Errorf("MaxLimit (%d) must be >= DefaultLimit (%d)", pagination.MaxLimit, pagination.DefaultLimit)
	}
}
