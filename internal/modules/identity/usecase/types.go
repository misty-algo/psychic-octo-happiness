// Package usecase coordinates administrator management workflows.
package usecase

import (
	"context"
	"time"

	"github.com/NSObjects/echo-admin/internal/modules/identity/domain"
	"github.com/NSObjects/echo-admin/internal/platform/pagination"
)

// Store persists administrators for the identity usecase.
type Store interface {
	FindByUsername(context.Context, string) (domain.Admin, error)
	FindByID(context.Context, int64) (domain.Admin, error)
	ListAll(context.Context) ([]domain.Admin, error)
	Create(context.Context, domain.Admin) (domain.Admin, error)
	Update(context.Context, domain.Admin) (domain.Admin, error)
	Delete(context.Context, int64) error
}

// RoleScope authorizes role assignments without exposing access storage details.
type RoleScope interface {
	AssignableRoleIDs(context.Context) ([]int64, error)
	VisibleRoleIDs(context.Context) ([]int64, error)
	EnsureAssignableRoles(context.Context, []int64) error
}

// LoginSessionRevoker revokes browser login sessions after identity security
// events without exposing auth storage to the identity module.
type LoginSessionRevoker interface {
	RevokeLoginSessions(context.Context, int64) error
}

// Usecase coordinates administrator management rules.
type Usecase struct {
	store    Store
	roles    RoleScope
	sessions LoginSessionRevoker
}

// New creates an identity usecase.
func New(store Store, roles RoleScope, sessions LoginSessionRevoker) *Usecase {
	return &Usecase{store: store, roles: roles, sessions: sessions}
}

// AdminInput carries mutable administrator fields.
type AdminInput struct {
	Username     string
	DisplayName  string
	Email        string
	Password     string
	RoleIDs      []int64
	ActiveRoleID int64
	Active       bool
}

// UpdateAdminInput carries partial administrator updates.
type UpdateAdminInput struct {
	ID           int64
	DisplayName  *string
	Email        *string
	Password     *string
	RoleIDs      []int64
	ActiveRoleID *int64
	Active       *bool
}

// RoleAdminsInput carries the full administrator assignment for one role.
type RoleAdminsInput struct {
	RoleID   int64
	AdminIDs []int64
}

// ListInput carries pagination for administrator lists.
type ListInput struct {
	Page     int
	PageSize int
}

// ListFilter is the validated store-facing pagination window.
type ListFilter struct {
	pagination.Window
}

// ListOutput is a paginated administrator result.
type ListOutput struct {
	Items    []Admin
	Page     int
	PageSize int
	Total    int
}

// Admin is the adapter-facing administrator DTO.
type Admin struct {
	ID           int64     `json:"id"`
	Username     string    `json:"username"`
	DisplayName  string    `json:"display_name"`
	Email        string    `json:"email"`
	RoleIDs      []int64   `json:"role_ids"`
	ActiveRoleID int64     `json:"active_role_id"`
	Active       bool      `json:"active"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}
