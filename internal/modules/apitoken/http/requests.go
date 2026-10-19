package apitokenhttp

import "time"

type tokenRequest struct {
	AdminID     int64      `json:"admin_id" validate:"omitempty,gt=0"`
	RoleID      int64      `json:"role_id" validate:"omitempty,gt=0"`
	Name        string     `json:"name" validate:"required,max=80"`
	Description string     `json:"description" validate:"omitempty,max=240"`
	Active      bool       `json:"active"`
	Days        int        `json:"days" validate:"omitempty,min=1,max=365"`
	ExpiresAt   *time.Time `json:"expires_at"`
}
