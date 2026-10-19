package mysql

import (
	"errors"

	drivermysql "github.com/go-sql-driver/mysql"
	"gorm.io/gorm"

	"github.com/NSObjects/echo-admin/internal/platform/apperr"
)

// IsDuplicateKey reports whether err is a MySQL duplicate-key error (1062).
// Module MySQL adapters use it to detect uniqueness races, e.g. upsert
// contention, without keeping private copies of the error-number check.
func IsDuplicateKey(err error) bool {
	var mysqlErr *drivermysql.MySQLError
	return errors.As(err, &mysqlErr) && mysqlErr.Number == 1062
}

// MapReadError translates a GORM read error into its application semantics:
// record-not-found becomes a not-found application error naming the resource,
// everything else stays a database failure attributed to the operation.
// Module MySQL adapters share this mapping instead of keeping private copies.
func MapReadError(err error, resource, operation string) error {
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return apperr.NewNotFound(resource)
	}
	return apperr.WrapDatabase(err, operation)
}

// MapWriteError translates a GORM write error into its application semantics:
// MySQL duplicate-key error 1062 becomes a conflict carrying conflictMessage,
// everything else stays a database failure attributed to the operation.
func MapWriteError(err error, conflictMessage, operation string) error {
	if IsDuplicateKey(err) {
		return apperr.NewConflict(conflictMessage)
	}
	return apperr.WrapDatabase(err, operation)
}
