// Package apperr defines framework-free application errors.
package apperr

// Kind describes the caller-visible class of an application error without
// depending on HTTP.
type Kind string

// Application error kinds.
const (
	KindOK               Kind = "ok"
	KindBadRequest       Kind = "bad_request"
	KindValidation       Kind = "validation"
	KindUnauthorized     Kind = "unauthorized"
	KindForbidden        Kind = "forbidden"
	KindNotFound         Kind = "not_found"
	KindMethodNotAllowed Kind = "method_not_allowed"
	KindConflict         Kind = "conflict"
	KindInternal         Kind = "internal"
)

// Category identifies the operational area associated with an error.
type Category string

// Categories are package-private spellings: Category values surface only
// through Definition and Info, and no caller needs the constants themselves.
const (
	categorySystem     Category = "system"
	categoryDatabase   Category = "database"
	categoryAuth       Category = "auth"
	categoryPermission Category = "permission"
	categoryValidation Category = "validation"
	categoryBusiness   Category = "business"
)

// Application error codes are assigned explicitly so the value of every live
// code is part of the JSON `code` contract. The numbering is intentionally
// non-contiguous: retired, never-shipped codes left gaps behind.
const (
	ErrSuccess    = 100001
	ErrUnknown    = 100002
	ErrValidation = 100004

	ErrDatabase = 100101

	ErrPermissionDenied = 100207
	ErrAccountDisabled  = 100209
	ErrTooManyAttempts  = 100210

	ErrBadRequest          = 100400
	ErrUnauthorized        = 100401
	ErrForbidden           = 100403
	ErrNotFound            = 100404
	ErrMethodNotAllowed    = 100405
	ErrConflict            = 100409
	ErrSystemUninitialized = 100410
	ErrInternalServer      = 100500
)

// Definition is the registered meaning of an application error code.
type Definition struct {
	Code     int
	Kind     Kind
	Category Category
	Message  string
}

var definitions = map[int]Definition{
	ErrSuccess:             {Code: ErrSuccess, Kind: KindOK, Category: categorySystem, Message: "OK"},
	ErrUnknown:             {Code: ErrUnknown, Kind: KindInternal, Category: categorySystem, Message: "Internal server error"},
	ErrValidation:          {Code: ErrValidation, Kind: KindValidation, Category: categoryValidation, Message: "Validation failed"},
	ErrDatabase:            {Code: ErrDatabase, Kind: KindInternal, Category: categoryDatabase, Message: "Database error"},
	ErrBadRequest:          {Code: ErrBadRequest, Kind: KindBadRequest, Category: categoryValidation, Message: "Bad request"},
	ErrUnauthorized:        {Code: ErrUnauthorized, Kind: KindUnauthorized, Category: categoryAuth, Message: "Unauthorized"},
	ErrForbidden:           {Code: ErrForbidden, Kind: KindForbidden, Category: categoryPermission, Message: "Forbidden"},
	ErrNotFound:            {Code: ErrNotFound, Kind: KindNotFound, Category: categoryBusiness, Message: "Not found"},
	ErrMethodNotAllowed:    {Code: ErrMethodNotAllowed, Kind: KindMethodNotAllowed, Category: categoryValidation, Message: "Method not allowed"},
	ErrConflict:            {Code: ErrConflict, Kind: KindConflict, Category: categoryBusiness, Message: "Conflict"},
	ErrSystemUninitialized: {Code: ErrSystemUninitialized, Kind: KindConflict, Category: categorySystem, Message: "system is not initialized"},
	ErrInternalServer:      {Code: ErrInternalServer, Kind: KindInternal, Category: categorySystem, Message: "Internal server error"},
	ErrPermissionDenied:    {Code: ErrPermissionDenied, Kind: KindForbidden, Category: categoryPermission, Message: "Permission denied"},
	ErrAccountDisabled:     {Code: ErrAccountDisabled, Kind: KindForbidden, Category: categoryPermission, Message: "Account is disabled"},
	ErrTooManyAttempts:     {Code: ErrTooManyAttempts, Kind: KindForbidden, Category: categoryPermission, Message: "Too many login attempts"},
}

// lookup returns the registered definition for code.
func lookup(code int) (Definition, bool) {
	def, ok := definitions[code]
	return def, ok
}

func definitionFor(code int) Definition {
	if def, ok := lookup(code); ok {
		return def
	}
	return definitions[ErrUnknown]
}
