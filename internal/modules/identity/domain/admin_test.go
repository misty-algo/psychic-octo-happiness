package domain

import (
	"errors"
	"testing"
)

func TestNewFirstAdministratorBuildsInstallationShape(t *testing.T) {
	hash := []byte("hashed-password")

	admin, err := NewFirstAdministrator("root", "系统管理员", "root@example.com", hash, 7)
	if err != nil {
		t.Fatalf("NewFirstAdministrator() error = %v, want nil", err)
	}
	if admin.ID != 0 {
		t.Fatalf("ID = %d, want 0 for a new row", admin.ID)
	}
	if len(admin.RoleIDs) != 1 || admin.RoleIDs[0] != 7 {
		t.Fatalf("RoleIDs = %v, want [7] (root role only)", admin.RoleIDs)
	}
	if admin.ActiveRoleID != 7 {
		t.Fatalf("ActiveRoleID = %d, want 7", admin.ActiveRoleID)
	}
	if !admin.Active {
		t.Fatal("Active = false, want true")
	}
	if !admin.CreatedAt.IsZero() || !admin.UpdatedAt.IsZero() {
		t.Fatalf("timestamps = %v/%v, want zero values", admin.CreatedAt, admin.UpdatedAt)
	}
}

func TestNewFirstAdministratorRejectsInvalidInput(t *testing.T) {
	hash := []byte("hashed-password")
	tests := []struct {
		name       string
		username   string
		password   []byte
		rootRoleID int64
		wantErr    error
	}{
		{name: "empty username", username: "", password: hash, rootRoleID: 7, wantErr: ErrInvalidUsername},
		{name: "missing password hash", username: "root", password: nil, rootRoleID: 7, wantErr: ErrInvalidPasswordHash},
		{name: "zero root role", username: "root", password: hash, rootRoleID: 0, wantErr: ErrInvalidActiveRole},
		{name: "negative root role", username: "root", password: hash, rootRoleID: -1, wantErr: ErrInvalidActiveRole},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := NewFirstAdministrator(tt.username, "系统管理员", "", tt.password, tt.rootRoleID)
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("NewFirstAdministrator() error = %v, want %v", err, tt.wantErr)
			}
		})
	}
}
