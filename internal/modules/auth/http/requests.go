package authhttp

type loginRequest struct {
	Username string `json:"username" validate:"required,max=64"`
	Password string `json:"password" validate:"required,max=72"`
}

type switchRoleRequest struct {
	RoleID int64 `json:"role_id" validate:"required,gt=0"`
}

type changePasswordRequest struct {
	CurrentPassword string `json:"current_password" validate:"required,max=72"`
	NewPassword     string `json:"new_password" validate:"required,min=8,max=72"`
}

type updateProfileRequest struct {
	DisplayName string `json:"display_name" validate:"required,max=80"`
	Email       string `json:"email" validate:"omitempty,email,max=160"`
}
