package pagination

import (
	"math"
	"testing"

	"github.com/NSObjects/echo-admin/internal/platform/apperr"
)

func TestNormalizeAppliesDefaults(t *testing.T) {
	got, err := Normalize(0, 0, DefaultOptions)
	if err != nil {
		t.Fatalf("Normalize() error = %v", err)
	}
	if got.Page != 1 || got.PageSize != DefaultPageSize || got.Offset != 0 || got.Limit != DefaultPageSize {
		t.Fatalf("Normalize() = %#v, want first page with default size", got)
	}
}

func TestNormalizeRejectsOverflowingOffset(t *testing.T) {
	_, err := Normalize(math.MaxInt, MaxPageSize, DefaultOptions)
	if want := apperr.NewBadRequest("invalid pagination"); err.Error() != want.Error() {
		t.Fatalf("Normalize() error = %v, want bad request invalid pagination", err)
	}
}

func TestNormalizeRejectsInvalidPolicy(t *testing.T) {
	_, err := Normalize(1, 0, Options{DefaultPageSize: 0, MaxPageSize: MaxPageSize})
	if want := apperr.NewBadRequest("invalid pagination"); err.Error() != want.Error() {
		t.Fatalf("Normalize() error = %v, want bad request invalid pagination", err)
	}
}

func TestBoundsClampsOverflowingLimit(t *testing.T) {
	start, end, ok, err := Bounds(5, 4, math.MaxInt)
	if err != nil {
		t.Fatalf("Bounds() error = %v", err)
	}
	if !ok || start != 4 || end != 5 {
		t.Fatalf("Bounds() = %d %d %v, want 4 5 true", start, end, ok)
	}
}

func TestBoundsRejectsInvalidInput(t *testing.T) {
	_, _, _, err := Bounds(5, -1, 10)
	if want := apperr.NewBadRequest("invalid pagination"); err.Error() != want.Error() {
		t.Fatalf("Bounds() error = %v, want bad request invalid pagination", err)
	}
}
