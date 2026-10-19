// Package pagination normalizes bounded offset pagination.
package pagination

import (
	"math"

	"github.com/NSObjects/echo-admin/internal/platform/apperr"
)

// DefaultPageSize is the page size applied when a list request omits
// page_size. It is the single source of the platform-wide pagination policy.
const DefaultPageSize = 20

// MaxPageSize is the largest page size any list endpoint accepts.
const MaxPageSize = 100

const defaultPage = 1

// DefaultOptions is the pagination policy shared by every list endpoint;
// pass different Options only when an endpoint genuinely needs its own policy.
var DefaultOptions = Options{DefaultPageSize: DefaultPageSize, MaxPageSize: MaxPageSize}

// Options defines the pagination policy for one list endpoint.
type Options struct {
	DefaultPageSize int
	MaxPageSize     int
}

// Window is a validated page request translated into store-facing bounds.
type Window struct {
	Page     int
	PageSize int
	Offset   int
	Limit    int
}

// invalid reports invalid or overflowing pagination input as the single
// bad-request error every caller renders identically.
func invalid() error {
	return apperr.NewBadRequest("invalid pagination")
}

// Normalize applies defaults, validates bounds, and rejects offset overflow.
func Normalize(page, pageSize int, options Options) (Window, error) {
	if page == 0 {
		page = defaultPage
	}
	if pageSize == 0 {
		pageSize = options.DefaultPageSize
	}
	if page < 1 ||
		pageSize < 1 ||
		options.DefaultPageSize < 1 ||
		options.MaxPageSize < options.DefaultPageSize ||
		pageSize > options.MaxPageSize {
		return Window{}, invalid()
	}

	pageIndex := page - 1
	if pageIndex > math.MaxInt/pageSize {
		return Window{}, invalid()
	}

	return Window{
		Page:     page,
		PageSize: pageSize,
		Offset:   pageIndex * pageSize,
		Limit:    pageSize,
	}, nil
}

// Bounds converts validated offset pagination into safe slice indexes.
func Bounds(total, offset, limit int) (start, end int, ok bool, err error) {
	if total < 0 || offset < 0 || limit < 1 {
		return 0, 0, false, invalid()
	}
	if offset >= total {
		return 0, 0, false, nil
	}
	if limit > math.MaxInt-offset {
		return offset, total, true, nil
	}
	end = offset + limit
	if end > total {
		end = total
	}
	return offset, end, true, nil
}
