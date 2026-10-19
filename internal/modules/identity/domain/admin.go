// Package domain contains the administrator management business rules.
package domain

import (
	"errors"
	"strings"
	"time"
)

// Administrator validation errors.
var (
	ErrInvalidAdminID      = errors.New("invalid admin id")
	ErrInvalidUsername     = errors.New("invalid username")
	ErrInvalidDisplayName  = errors.New("invalid display name")
	ErrInvalidPasswordHash = errors.New("invalid password hash")
	ErrAdminNeedsRole      = errors.New("admin needs at least one role")
	ErrInvalidActiveRole   = errors.New("active role is not assigned")
)

// Admin is a validated back-office user that can sign in and operate management APIs.
type Admin struct {
	ID           int64
	Username     string
	DisplayName  string
	Email        string
	PasswordHash []byte
	RoleIDs      []int64
	ActiveRoleID int64
	Active       bool
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

// NewAdmin validates and creates an admin before it is persisted.
func NewAdmin(username, displayName, email string, passwordHash []byte, roleIDs []int64, activeRoleID int64) (Admin, error) {
	return RestoreAdmin(0, username, displayName, email, passwordHash, roleIDs, activeRoleID, true, time.Time{}, time.Time{})
}

// NewFirstAdministrator builds the administrator created by System First
// Initialization: a fresh active administrator whose only role is the Root
// Role. Input normalization and password hashing belong to the caller; this
// constructor owns the installation-time shape only.
func NewFirstAdministrator(username, displayName, email string, passwordHash []byte, rootRoleID int64) (Admin, error) {
	if rootRoleID <= 0 {
		return Admin{}, ErrInvalidActiveRole
	}
	return RestoreAdmin(0, username, displayName, email, passwordHash, []int64{rootRoleID}, rootRoleID, true, time.Time{}, time.Time{})
}

// RestoreAdmin rebuilds an admin from a trusted store representation.
func RestoreAdmin(id int64, username, displayName, email string, passwordHash []byte, roleIDs []int64, activeRoleID int64, active bool, createdAt, updatedAt time.Time) (Admin, error) {
	username = normalizeToken(username)
	displayName = strings.TrimSpace(displayName)
	email = strings.TrimSpace(email)

	if id < 0 {
		return Admin{}, ErrInvalidAdminID
	}
	if username == "" || len(username) > 64 {
		return Admin{}, ErrInvalidUsername
	}
	if displayName == "" || len(displayName) > 80 {
		return Admin{}, ErrInvalidDisplayName
	}
	if len(passwordHash) == 0 {
		return Admin{}, ErrInvalidPasswordHash
	}
	copiedRoles := uniquePositiveIDs(roleIDs)
	if len(copiedRoles) == 0 {
		return Admin{}, ErrAdminNeedsRole
	}
	activeRoleID = normalizeActiveRoleID(copiedRoles, activeRoleID)
	if activeRoleID == 0 {
		return Admin{}, ErrInvalidActiveRole
	}
	return Admin{
		ID:           id,
		Username:     username,
		DisplayName:  displayName,
		Email:        email,
		PasswordHash: append([]byte(nil), passwordHash...),
		RoleIDs:      copiedRoles,
		ActiveRoleID: activeRoleID,
		Active:       active,
		CreatedAt:    createdAt,
		UpdatedAt:    updatedAt,
	}, nil
}

// HasRole reports whether the admin is assigned roleID.
func (a Admin) HasRole(roleID int64) bool {
	for _, assignedID := range a.RoleIDs {
		if assignedID == roleID {
			return true
		}
	}
	return false
}

// UpdateProfile changes mutable admin fields while preserving identity and password.
func (a Admin) UpdateProfile(displayName, email string, roleIDs []int64, activeRoleID int64, active bool) (Admin, error) {
	return RestoreAdmin(a.ID, a.Username, displayName, email, a.PasswordHash, roleIDs, activeRoleID, active, a.CreatedAt, time.Time{})
}

// ReplacePassword changes the admin password hash.
func (a Admin) ReplacePassword(passwordHash []byte) (Admin, error) {
	return RestoreAdmin(a.ID, a.Username, a.DisplayName, a.Email, passwordHash, a.RoleIDs, a.ActiveRoleID, a.Active, a.CreatedAt, time.Time{})
}

// SwitchActiveRole changes the role that authorization should use for new tokens.
func (a Admin) SwitchActiveRole(roleID int64) (Admin, error) {
	if !a.HasRole(roleID) {
		return Admin{}, ErrInvalidActiveRole
	}
	return RestoreAdmin(a.ID, a.Username, a.DisplayName, a.Email, a.PasswordHash, a.RoleIDs, roleID, a.Active, a.CreatedAt, time.Time{})
}

func normalizeToken(value string) string {
	return strings.ToLower(strings.TrimSpace(value))
}

func normalizeActiveRoleID(roleIDs []int64, activeRoleID int64) int64 {
	if activeRoleID <= 0 && len(roleIDs) > 0 {
		return roleIDs[0]
	}
	for _, roleID := range roleIDs {
		if roleID == activeRoleID {
			return activeRoleID
		}
	}
	return 0
}

func uniquePositiveIDs(ids []int64) []int64 {
	seen := make(map[int64]struct{}, len(ids))
	out := make([]int64, 0, len(ids))
	for _, id := range ids {
		if id <= 0 {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		out = append(out, id)
	}
	return out
}
