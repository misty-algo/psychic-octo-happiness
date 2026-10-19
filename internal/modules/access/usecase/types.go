// Package usecase coordinates access management and runtime authorization.
package usecase

import (
	"context"
	"time"

	"github.com/NSObjects/echo-admin/internal/modules/access/domain"
	"github.com/NSObjects/echo-admin/internal/platform/pagination"
)

// Store persists access roles, menus, and managed API routes.
type Store interface {
	FindRoleByID(context.Context, int64) (domain.Role, error)
	FindRoleByCode(context.Context, string) (domain.Role, error)
	ListAllRoles(context.Context) ([]domain.Role, error)
	CreateRole(context.Context, domain.Role) (domain.Role, error)
	UpdateRole(context.Context, domain.Role) (domain.Role, error)
	DeleteRole(context.Context, int64) error
	FindAPIByID(context.Context, int64) (domain.API, error)
	FindAPIByRoute(context.Context, string, string) (domain.API, error)
	ListAPIs(context.Context) ([]domain.API, error)
	FindMenuByID(context.Context, int64) (domain.Menu, error)
	ListMenus(context.Context) ([]domain.Menu, error)
	CreateMenu(context.Context, domain.Menu) (domain.Menu, error)
	UpdateMenu(context.Context, domain.Menu) (domain.Menu, error)
	DeleteMenu(context.Context, int64) error
}

// AdminRoleReader reads the current administrator role assignment without exposing identity storage.
type AdminRoleReader interface {
	AdminRoleState(context.Context, int64) (AdminRoleState, error)
	RoleAssigned(context.Context, int64) (bool, error)
}

// AdminRoleState is the identity snapshot needed for role delegation and
// runtime authorization.
type AdminRoleState struct {
	RoleIDs      []int64
	ActiveRoleID int64
	Active       bool
}

// Usecase coordinates access management and runtime authorization rules.
type Usecase struct {
	store  Store
	admins AdminRoleReader
}

// AuthorizationSubject identifies the administrator and active role whose
// current grants must be evaluated.
type AuthorizationSubject struct {
	AdminID      int64
	ActiveRoleID int64
}

// AuthorizationView is the active Administration Authorization state used by
// current-user responses. It is embedded into auth's /auth/me JSON contract,
// so the snake_case tags below ARE the wire contract consumed by the browser
// client; removing them serializes Go field names and breaks the frontend.
type AuthorizationView struct {
	ActiveRole  Role     `json:"active_role"`
	Roles       []Role   `json:"roles"`
	Permissions []string `json:"permissions"`
	Menus       []Menu   `json:"menus"`
	DefaultPath string   `json:"default_path"`
}

// New creates an access usecase.
func New(store Store, admins AdminRoleReader) *Usecase {
	return &Usecase{store: store, admins: admins}
}

// WithStore returns a shallow copy of the usecase bound to store. The
// composition root uses it to run menu imports on a transaction-scoped store;
// the copy shares no mutable state with the receiver.
func (u *Usecase) WithStore(store Store) *Usecase {
	bound := *u
	bound.store = store
	return &bound
}

// RoleInput carries mutable role fields.
type RoleInput struct {
	ParentID    int64
	Code        string
	Name        string
	Permissions []string
	MenuIDs     []int64
	APIIDs      []int64
	ButtonIDs   []int64
	DataRoleIDs []int64
	DefaultPath string
	Active      bool
}

// UpdateRoleInput carries partial role updates.
type UpdateRoleInput struct {
	ID          int64
	ParentID    *int64
	Name        *string
	Permissions []string
	MenuIDs     []int64
	APIIDs      []int64
	ButtonIDs   []int64
	DataRoleIDs []int64
	DefaultPath *string
	Active      *bool
}

// CopyRoleInput carries the new identity fields for a copied role.
type CopyRoleInput struct {
	SourceID    int64
	ParentID    *int64
	Code        string
	Name        string
	DefaultPath *string
	Active      *bool
}

// MenuInput carries mutable menu fields. JSON tags shape the version-bundle
// contract; ParentID is tree-derived on import and meaningless on export, so
// it never serializes.
type MenuInput struct {
	ParentID   int64             `json:"-"`
	Name       string            `json:"name"`
	Path       string            `json:"path"`
	Icon       string            `json:"icon"`
	Hidden     bool              `json:"hidden"`
	Component  string            `json:"component"`
	Meta       MenuMetaInput     `json:"meta"`
	Permission string            `json:"permission"`
	Sort       int               `json:"sort"`
	Active     bool              `json:"active"`
	Buttons    []MenuButtonInput `json:"buttons"`
}

// MenuMetaInput carries router metadata for one menu.
type MenuMetaInput struct {
	ActiveName     string `json:"active_name"`
	KeepAlive      bool   `json:"keep_alive"`
	DefaultMenu    bool   `json:"default_menu"`
	CloseTab       bool   `json:"close_tab"`
	TransitionType string `json:"transition_type"`
}

// MenuButtonInput carries one page-level operation key attached to a menu.
// Button identity is path-derived during import (upsert by name), so the
// database ID never travels through a version bundle.
type MenuButtonInput struct {
	ID          int64  `json:"-"`
	Name        string `json:"name"`
	Description string `json:"description"`
}

// MenuTreeInput is the single menu exchange shape: version bundles serialize
// it directly and imports consume it directly. Parent links are derived from
// the tree itself: roots attach to the top level and children attach to their
// freshly saved parent, so stale parent references inside a bundle never
// survive an import.
type MenuTreeInput struct {
	MenuInput
	Children []MenuTreeInput `json:"children,omitempty"`
}

// UpdateMenuInput carries mutable menu updates: the target ID plus one
// complete menu field set. ParentID lives on the embedded MenuInput so it has
// exactly one meaning.
type UpdateMenuInput struct {
	ID int64
	MenuInput
}

// ListInput carries pagination for role lists.
type ListInput struct {
	Page     int
	PageSize int
}

// ListFilter is the validated store-facing pagination window.
type ListFilter struct {
	pagination.Window
}

// RoleListOutput is a paginated role result.
type RoleListOutput struct {
	Items    []Role
	Page     int
	PageSize int
	Total    int
}

// APIListOutput is a paginated API result.
type APIListOutput struct {
	Items    []API
	Page     int
	PageSize int
	Total    int
}

// Role is the adapter-facing role DTO.
type Role struct {
	ID          int64     `json:"id"`
	ParentID    int64     `json:"parent_id"`
	Code        string    `json:"code"`
	Name        string    `json:"name"`
	Permissions []string  `json:"permissions"`
	MenuIDs     []int64   `json:"menu_ids"`
	APIIDs      []int64   `json:"api_ids"`
	ButtonIDs   []int64   `json:"button_ids"`
	DataRoleIDs []int64   `json:"data_role_ids"`
	DefaultPath string    `json:"default_path"`
	Active      bool      `json:"active"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// Menu is the adapter-facing menu DTO.
type Menu struct {
	ID         int64     `json:"id"`
	ParentID   int64     `json:"parent_id"`
	Name       string    `json:"name"`
	Path       string    `json:"path"`
	Icon       string    `json:"icon"`
	Hidden     bool      `json:"hidden"`
	Component  string    `json:"component"`
	Meta       MenuMeta  `json:"meta"`
	Permission string    `json:"permission"`
	Sort       int       `json:"sort"`
	Active     bool      `json:"active"`
	Buttons    []Button  `json:"buttons"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
}

// MenuMeta is the adapter-facing router metadata DTO.
type MenuMeta struct {
	ActiveName     string `json:"active_name"`
	KeepAlive      bool   `json:"keep_alive"`
	DefaultMenu    bool   `json:"default_menu"`
	CloseTab       bool   `json:"close_tab"`
	TransitionType string `json:"transition_type"`
}

// Button is the adapter-facing menu button DTO.
type Button struct {
	ID          int64     `json:"id"`
	MenuID      int64     `json:"menu_id"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// API is the adapter-facing API route DTO.
type API struct {
	ID          int64     `json:"id"`
	Method      string    `json:"method"`
	Path        string    `json:"path"`
	Description string    `json:"description"`
	Group       string    `json:"group"`
	Permission  string    `json:"permission"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// PermissionDefinition is the adapter-facing permission metadata DTO.
type PermissionDefinition struct {
	Token    string `json:"token"`
	Resource string `json:"resource"`
	Action   string `json:"action"`
	Name     string `json:"name"`
}
