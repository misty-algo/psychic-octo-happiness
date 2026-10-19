// Package mysqljson provides small scanner/value types for MySQL JSON columns.
package mysqljson

import (
	"database/sql/driver"
	"encoding/json"
	"errors"
	"fmt"
)

// Strings stores a string slice in a MySQL JSON column.
type Strings []string

// Value serializes Strings for database writes.
func (s Strings) Value() (driver.Value, error) {
	data, err := json.Marshal([]string(s))
	if err != nil {
		return nil, fmt.Errorf("marshal string json: %w", err)
	}
	return string(data), nil
}

// Scan loads Strings from a MySQL JSON column.
func (s *Strings) Scan(value any) error {
	if s == nil {
		return errors.New("mysqljson.Strings scan target is nil")
	}
	var out []string
	if err := scanJSON(value, &out); err != nil {
		return err
	}
	*s = out
	return nil
}

// GormDataType tells GORM to keep Strings as a JSON column.
func (Strings) GormDataType() string {
	return "json"
}

// Int64s stores an int64 slice in a MySQL JSON column.
type Int64s []int64

// Value serializes Int64s for database writes.
func (s Int64s) Value() (driver.Value, error) {
	data, err := json.Marshal([]int64(s))
	if err != nil {
		return nil, fmt.Errorf("marshal int64 json: %w", err)
	}
	return string(data), nil
}

// Scan loads Int64s from a MySQL JSON column.
func (s *Int64s) Scan(value any) error {
	if s == nil {
		return errors.New("mysqljson.Int64s scan target is nil")
	}
	var out []int64
	if err := scanJSON(value, &out); err != nil {
		return err
	}
	*s = out
	return nil
}

// GormDataType tells GORM to keep Int64s as a JSON column.
func (Int64s) GormDataType() string {
	return "json"
}

func scanJSON(value, target any) error {
	switch typed := value.(type) {
	case nil:
		return nil
	case []byte:
		return unmarshalJSON(typed, target)
	case string:
		return unmarshalJSON([]byte(typed), target)
	default:
		return fmt.Errorf("scan json from %T", value)
	}
}

func unmarshalJSON(data []byte, target any) error {
	if len(data) == 0 {
		return nil
	}
	if err := json.Unmarshal(data, target); err != nil {
		return fmt.Errorf("unmarshal json: %w", err)
	}
	return nil
}
