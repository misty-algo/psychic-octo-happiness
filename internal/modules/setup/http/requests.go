package http

type submitRequest struct {
	Username    string `json:"username" validate:"required,max=64"`
	DisplayName string `json:"display_name" validate:"required,max=80"`
	Email       string `json:"email" validate:"omitempty,email,max=160"`
	Password    string `json:"password" validate:"required,min=8,max=72"`
	SiteName    string `json:"site_name" validate:"omitempty,max=120"`
}
