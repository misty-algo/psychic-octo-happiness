package identityhttp

type createAdminRequest struct {
	Username     string  `json:"username" validate:"required,max=64"`
	DisplayName  string  `json:"display_name" validate:"required,max=80"`
	Email        string  `json:"email" validate:"omitempty,email,max=160"`
	Password     string  `json:"password" validate:"required,min=8,max=72"`
	RoleIDs      []int64 `json:"role_ids" validate:"required,min=1,dive,gt=0"`
	ActiveRoleID int64   `json:"active_role_id" validate:"omitempty,gt=0"`
	Active       bool    `json:"active"`
}

type updateAdminRequest struct {
	DisplayName  *string `json:"display_name" validate:"omitempty,max=80"`
	Email        *string `json:"email" validate:"omitempty,email,max=160"`
	Password     *string `json:"password" validate:"omitempty,min=8,max=72"`
	RoleIDs      []int64 `json:"role_ids" validate:"omitempty,dive,gt=0"`
	ActiveRoleID *int64  `json:"active_role_id" validate:"omitempty,gt=0"`
	Active       *bool   `json:"active"`
}

type adminIDsRequest struct {
	AdminIDs []int64 `json:"admin_ids" validate:"omitempty,dive,gt=0"`
}
