// Package usecase coordinates system setting and dictionary workflows.
package usecase

import (
	"context"
	"time"

	accessusecase "github.com/NSObjects/echo-admin/internal/modules/access/usecase"
	"github.com/NSObjects/echo-admin/internal/modules/settings/domain"
)

// Store persists system settings and dictionaries.
type Store interface {
	ListConfigs(context.Context) ([]domain.SystemConfig, error)
	UpsertConfig(context.Context, domain.SystemConfig) (domain.SystemConfig, error)
	DeleteConfig(context.Context, string) error
	ListParams(context.Context) ([]domain.SystemParam, error)
	FindParamByID(context.Context, int64) (domain.SystemParam, error)
	FindParamByKey(context.Context, string) (domain.SystemParam, error)
	CreateParam(context.Context, domain.SystemParam) (domain.SystemParam, error)
	UpdateParam(context.Context, domain.SystemParam) (domain.SystemParam, error)
	DeleteParam(context.Context, int64) error
	DeleteParams(context.Context, []int64) error
	ListDictionaries(context.Context) ([]domain.Dictionary, error)
	CreateDictionary(context.Context, domain.Dictionary) (domain.Dictionary, error)
	UpdateDictionary(context.Context, domain.Dictionary) (domain.Dictionary, error)
	DeleteDictionary(context.Context, string) error
	AddDictionaryItem(context.Context, string, domain.DictionaryItem) (domain.Dictionary, error)
	UpdateDictionaryItem(context.Context, string, domain.DictionaryItem) (domain.Dictionary, error)
	FindDictionaryItem(context.Context, string, int64) (domain.DictionaryItem, error)
	DeleteDictionaryItem(context.Context, string, int64) (domain.Dictionary, error)
	ListVersions(context.Context) ([]domain.SystemVersion, error)
	FindVersionByID(context.Context, int64) (domain.SystemVersion, error)
	CreateVersion(context.Context, domain.SystemVersion) (domain.SystemVersion, error)
	UpdateVersion(context.Context, domain.SystemVersion) (domain.SystemVersion, error)
	DeleteVersion(context.Context, int64) error
	DeleteVersions(context.Context, []int64) error
}

// VersionCatalog exports access-owned resources for version bundles.
type VersionCatalog interface {
	ExportVersionMenus(context.Context, []int64) ([]accessusecase.MenuTreeInput, error)
}

// ImportRunner executes version and dictionary imports inside one database
// transaction so a failed import leaves stored menus, dictionaries, and
// version records unchanged.
type ImportRunner interface {
	RunImport(context.Context, func(context.Context, ImportTransaction) error) error
}

// ImportTransaction contains the write capabilities required inside an import
// transaction. Implementations must apply every write to the same
// transaction-scoped storage.
type ImportTransaction interface {
	ImportMenus(context.Context, []accessusecase.MenuTreeInput) error
	ReplaceDictionary(context.Context, domain.Dictionary) error
	CreateVersion(context.Context, domain.SystemVersion) (domain.SystemVersion, error)
}

// Usecase coordinates system setting and dictionary rules.
type Usecase struct {
	store   Store
	catalog VersionCatalog
	imports ImportRunner
}

// Option customizes settings usecase dependencies.
type Option func(*Usecase)

// WithVersionCatalog installs the cross-module catalog used by version bundles.
func WithVersionCatalog(catalog VersionCatalog) Option {
	return func(u *Usecase) {
		u.catalog = catalog
	}
}

// WithImportRunner installs the transactional runner used by import workflows.
func WithImportRunner(runner ImportRunner) Option {
	return func(u *Usecase) {
		u.imports = runner
	}
}

// New creates a settings usecase.
func New(store Store, opts ...Option) *Usecase {
	u := &Usecase{store: store}
	for _, opt := range opts {
		opt(u)
	}
	return u
}

// ConfigInput carries a system config value update.
type ConfigInput struct {
	Key    string
	Name   string
	Value  string
	Public bool
}

// ParamInput carries mutable system parameter fields.
type ParamInput struct {
	Name  string
	Key   string
	Value string
	Desc  string
}

// UpdateParamInput carries mutable parameter fields with the target id.
type UpdateParamInput struct {
	ID    int64
	Name  string
	Key   string
	Value string
	Desc  string
}

// ParamListInput carries pagination and filters for parameter lists.
type ParamListInput struct {
	Page     int
	PageSize int
	Name     string
	Key      string
}

// ParamListOutput is a paginated parameter result.
type ParamListOutput struct {
	Items    []SystemParam
	Page     int
	PageSize int
	Total    int
}

// DictionaryInput carries dictionary creation fields.
type DictionaryInput struct {
	Code string
	Name string
}

// UpdateDictionaryInput carries mutable dictionary fields.
type UpdateDictionaryInput struct {
	Code string
	Name string
}

// DictionaryItemInput carries dictionary item fields.
type DictionaryItemInput struct {
	ID       int64
	ParentID int64
	Label    string
	Value    string
	Extend   string
	Sort     int
	Active   bool
}

// VersionInput carries release record fields.
type VersionInput struct {
	Version     string
	Name        string
	Description string
	Data        string
	PublishedAt time.Time
}

// ExportVersionInput selects resources to pack into a version bundle.
type ExportVersionInput struct {
	Version       string
	Name          string
	Description   string
	MenuIDs       []int64
	DictionaryIDs []int64
}

// VersionBundle is the portable JSON shape for version export and import.
type VersionBundle struct {
	Version      VersionInfo                   `json:"version"`
	Menus        []accessusecase.MenuTreeInput `json:"menus"`
	Dictionaries []VersionDictionary           `json:"dictionaries"`
}

// DictionaryBundle is the portable JSON shape for dictionary export and import.
type DictionaryBundle struct {
	ExportTime   string              `json:"export_time"`
	Dictionaries []VersionDictionary `json:"dictionaries"`
}

// VersionInfo stores version metadata inside an exported bundle.
type VersionInfo struct {
	Name        string `json:"name"`
	Code        string `json:"code"`
	Description string `json:"description"`
	ExportTime  string `json:"export_time"`
}

// VersionDictionary is a dictionary stored in a version bundle.
type VersionDictionary struct {
	Code  string                  `json:"code"`
	Name  string                  `json:"name"`
	Items []VersionDictionaryItem `json:"items"`
}

// VersionDictionaryItem is a dictionary item stored in a version bundle.
type VersionDictionaryItem struct {
	ParentID int64  `json:"parent_id,omitempty"`
	Label    string `json:"label"`
	Value    string `json:"value"`
	Extend   string `json:"extend,omitempty"`
	Sort     int    `json:"sort"`
	Active   bool   `json:"active"`
	Level    int    `json:"level,omitempty"`
	Path     string `json:"path,omitempty"`
}

// UpdateVersionInput carries mutable release record fields.
type UpdateVersionInput struct {
	ID          int64
	Version     string
	Name        string
	Description string
	Data        string
	PublishedAt time.Time
}

// SystemConfig is the adapter-facing config DTO.
type SystemConfig struct {
	Key       string    `json:"key"`
	Name      string    `json:"name"`
	Value     string    `json:"value"`
	Public    bool      `json:"public"`
	UpdatedAt time.Time `json:"updated_at"`
}

// SystemParam is the adapter-facing parameter DTO.
type SystemParam struct {
	ID        int64     `json:"id"`
	Name      string    `json:"name"`
	Key       string    `json:"key"`
	Value     string    `json:"value"`
	Desc      string    `json:"desc"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// Dictionary is the adapter-facing dictionary DTO.
type Dictionary struct {
	ID        int64            `json:"id"`
	Code      string           `json:"code"`
	Name      string           `json:"name"`
	Items     []DictionaryItem `json:"items"`
	CreatedAt time.Time        `json:"created_at"`
	UpdatedAt time.Time        `json:"updated_at"`
}

// DictionaryItem is the adapter-facing dictionary item DTO.
type DictionaryItem struct {
	ID       int64            `json:"id"`
	ParentID int64            `json:"parent_id"`
	Label    string           `json:"label"`
	Value    string           `json:"value"`
	Extend   string           `json:"extend"`
	Sort     int              `json:"sort"`
	Active   bool             `json:"active"`
	Level    int              `json:"level"`
	Path     string           `json:"path"`
	Children []DictionaryItem `json:"children"`
}

// SystemVersion is the adapter-facing release record DTO.
type SystemVersion struct {
	ID          int64     `json:"id"`
	Version     string    `json:"version"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	Data        string    `json:"-"`
	PublishedAt time.Time `json:"published_at"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}
