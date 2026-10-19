// Package domain contains system configuration and dictionary rules.
package domain

import (
	"errors"
	"strings"
	"time"
)

// System data validation errors.
var (
	ErrInvalidConfigKey   = errors.New("invalid config key")
	ErrInvalidConfigName  = errors.New("invalid config name")
	ErrInvalidDictID      = errors.New("invalid dictionary id")
	ErrInvalidDictCode    = errors.New("invalid dictionary code")
	ErrInvalidDictName    = errors.New("invalid dictionary name")
	ErrInvalidDictItemID  = errors.New("invalid dictionary item id")
	ErrInvalidDictParent  = errors.New("invalid dictionary item parent")
	ErrInvalidDictLabel   = errors.New("invalid dictionary item label")
	ErrInvalidDictValue   = errors.New("invalid dictionary item value")
	ErrInvalidDictExtend  = errors.New("invalid dictionary item extend")
	ErrInvalidDictLevel   = errors.New("invalid dictionary item level")
	ErrInvalidDictPath    = errors.New("invalid dictionary item path")
	ErrInvalidParamID     = errors.New("invalid system param id")
	ErrInvalidParamName   = errors.New("invalid system param name")
	ErrInvalidParamKey    = errors.New("invalid system param key")
	ErrInvalidParamValue  = errors.New("invalid system param value")
	ErrInvalidParamDesc   = errors.New("invalid system param description")
	ErrInvalidVersionID   = errors.New("invalid system version id")
	ErrInvalidVersion     = errors.New("invalid system version")
	ErrInvalidVersionName = errors.New("invalid system version name")
	ErrInvalidVersionDesc = errors.New("invalid system version description")
)

// SystemConfig is a validated key-value runtime setting managed from the back office.
type SystemConfig struct {
	Key       string
	Name      string
	Value     string
	Public    bool
	UpdatedAt time.Time
}

// RestoreSystemConfig rebuilds a config item from a trusted store representation.
func RestoreSystemConfig(key, name, value string, public bool, updatedAt time.Time) (SystemConfig, error) {
	key = normalizeToken(key)
	name = strings.TrimSpace(name)
	if key == "" || len(key) > 80 {
		return SystemConfig{}, ErrInvalidConfigKey
	}
	if name == "" || len(name) > 120 {
		return SystemConfig{}, ErrInvalidConfigName
	}
	return SystemConfig{Key: key, Name: name, Value: value, Public: public, UpdatedAt: updatedAt}, nil
}

// SystemParam is an operator-managed named key/value parameter.
type SystemParam struct {
	ID        int64
	Name      string
	Key       string
	Value     string
	Desc      string
	CreatedAt time.Time
	UpdatedAt time.Time
}

// RestoreSystemParam rebuilds a parameter from a trusted store representation.
func RestoreSystemParam(id int64, name, key, value, desc string, createdAt, updatedAt time.Time) (SystemParam, error) {
	name = strings.TrimSpace(name)
	key = normalizeToken(key)
	desc = strings.TrimSpace(desc)
	if id < 0 {
		return SystemParam{}, ErrInvalidParamID
	}
	if name == "" || len(name) > 120 {
		return SystemParam{}, ErrInvalidParamName
	}
	if key == "" || len(key) > 80 {
		return SystemParam{}, ErrInvalidParamKey
	}
	if value == "" || len(value) > 4000 {
		return SystemParam{}, ErrInvalidParamValue
	}
	if len(desc) > 4000 {
		return SystemParam{}, ErrInvalidParamDesc
	}
	return SystemParam{ID: id, Name: name, Key: key, Value: value, Desc: desc, CreatedAt: createdAt, UpdatedAt: updatedAt}, nil
}

// Dictionary is a validated group of controlled values.
type Dictionary struct {
	ID        int64
	Code      string
	Name      string
	Items     []DictionaryItem
	CreatedAt time.Time
	UpdatedAt time.Time
}

// RestoreDictionary rebuilds a dictionary from a trusted store representation.
func RestoreDictionary(id int64, code, name string, items []DictionaryItem, createdAt, updatedAt time.Time) (Dictionary, error) {
	code = normalizeToken(code)
	name = strings.TrimSpace(name)
	if id < 0 {
		return Dictionary{}, ErrInvalidDictID
	}
	if code == "" || len(code) > 80 {
		return Dictionary{}, ErrInvalidDictCode
	}
	if name == "" || len(name) > 120 {
		return Dictionary{}, ErrInvalidDictName
	}
	copiedItems := append([]DictionaryItem(nil), items...)
	return Dictionary{ID: id, Code: code, Name: name, Items: copiedItems, CreatedAt: createdAt, UpdatedAt: updatedAt}, nil
}

// DictionaryItem is a validated selectable value under a dictionary.
type DictionaryItem struct {
	ID       int64
	ParentID int64
	Label    string
	Value    string
	Extend   string
	Sort     int
	Active   bool
	Level    int
	Path     string
	Children []DictionaryItem
}

// RestoreDictionaryItem rebuilds a dictionary item from a trusted store representation.
func RestoreDictionaryItem(id, parentID int64, label, value, extend string, sort int, active bool, level int, path string, children []DictionaryItem) (DictionaryItem, error) {
	label = strings.TrimSpace(label)
	value = strings.TrimSpace(value)
	extend = strings.TrimSpace(extend)
	path = strings.TrimSpace(path)
	if id < 0 {
		return DictionaryItem{}, ErrInvalidDictItemID
	}
	if parentID < 0 || (id > 0 && id == parentID) {
		return DictionaryItem{}, ErrInvalidDictParent
	}
	if label == "" || len(label) > 120 {
		return DictionaryItem{}, ErrInvalidDictLabel
	}
	if value == "" || len(value) > 120 {
		return DictionaryItem{}, ErrInvalidDictValue
	}
	if len(extend) > 4000 {
		return DictionaryItem{}, ErrInvalidDictExtend
	}
	if level < 0 {
		return DictionaryItem{}, ErrInvalidDictLevel
	}
	if len(path) > 4000 {
		return DictionaryItem{}, ErrInvalidDictPath
	}
	return DictionaryItem{
		ID:       id,
		ParentID: parentID,
		Label:    label,
		Value:    value,
		Extend:   extend,
		Sort:     sort,
		Active:   active,
		Level:    level,
		Path:     path,
		Children: append([]DictionaryItem(nil), children...),
	}, nil
}

// SystemVersion is an operator-managed release note for the running back office.
type SystemVersion struct {
	ID          int64
	Version     string
	Name        string
	Description string
	Data        string
	PublishedAt time.Time
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

// RestoreSystemVersion rebuilds a release record from a trusted store representation.
func RestoreSystemVersion(id int64, version, name, description, data string, publishedAt, createdAt, updatedAt time.Time) (SystemVersion, error) {
	version = strings.TrimSpace(version)
	name = strings.TrimSpace(name)
	description = strings.TrimSpace(description)

	if id < 0 {
		return SystemVersion{}, ErrInvalidVersionID
	}
	if !isVersionLabel(version) {
		return SystemVersion{}, ErrInvalidVersion
	}
	if name == "" || len(name) > 120 {
		return SystemVersion{}, ErrInvalidVersionName
	}
	if len(description) > 4000 {
		return SystemVersion{}, ErrInvalidVersionDesc
	}
	return SystemVersion{
		ID:          id,
		Version:     version,
		Name:        name,
		Description: description,
		Data:        data,
		PublishedAt: publishedAt,
		CreatedAt:   createdAt,
		UpdatedAt:   updatedAt,
	}, nil
}

func normalizeToken(value string) string {
	return strings.ToLower(strings.TrimSpace(value))
}

func isVersionLabel(value string) bool {
	if value == "" || len(value) > 80 {
		return false
	}
	for _, r := range value {
		if r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' {
			continue
		}
		switch r {
		case '.', '-', '_', '+':
			continue
		default:
			return false
		}
	}
	return true
}
