package accesshttp

type createRoleRequest struct {
	ParentID    int64    `json:"parent_id" validate:"gte=0"`
	Code        string   `json:"code" validate:"required,max=64"`
	Name        string   `json:"name" validate:"required,max=80"`
	Permissions []string `json:"permissions" validate:"required,min=1,dive,required,max=80"`
	MenuIDs     []int64  `json:"menu_ids" validate:"omitempty,dive,gt=0"`
	APIIDs      []int64  `json:"api_ids" validate:"omitempty,dive,gt=0"`
	ButtonIDs   []int64  `json:"button_ids" validate:"omitempty,dive,gt=0"`
	DataRoleIDs []int64  `json:"data_role_ids" validate:"omitempty,dive,gt=0"`
	DefaultPath string   `json:"default_path" validate:"omitempty,max=160"`
	Active      bool     `json:"active"`
}

type updateRoleRequest struct {
	ParentID    *int64   `json:"parent_id" validate:"omitempty,gte=0"`
	Name        *string  `json:"name" validate:"omitempty,max=80"`
	Permissions []string `json:"permissions" validate:"omitempty,dive,required,max=80"`
	MenuIDs     []int64  `json:"menu_ids" validate:"omitempty,dive,gt=0"`
	APIIDs      []int64  `json:"api_ids" validate:"omitempty,dive,gt=0"`
	ButtonIDs   []int64  `json:"button_ids" validate:"omitempty,dive,gt=0"`
	DataRoleIDs []int64  `json:"data_role_ids" validate:"omitempty,dive,gt=0"`
	DefaultPath *string  `json:"default_path" validate:"omitempty,max=160"`
	Active      *bool    `json:"active"`
}

type copyRoleRequest struct {
	ParentID    *int64  `json:"parent_id" validate:"omitempty,gte=0"`
	Code        string  `json:"code" validate:"required,max=64"`
	Name        string  `json:"name" validate:"required,max=80"`
	DefaultPath *string `json:"default_path" validate:"omitempty,max=160"`
	Active      *bool   `json:"active"`
}

type menuRequest struct {
	ParentID   int64               `json:"parent_id" validate:"gte=0"`
	Name       string              `json:"name" validate:"required,max=80"`
	Path       string              `json:"path" validate:"required,max=160"`
	Icon       string              `json:"icon" validate:"omitempty,max=80"`
	Hidden     bool                `json:"hidden"`
	Component  string              `json:"component" validate:"required,max=160"`
	Meta       menuMetaRequest     `json:"meta"`
	Permission string              `json:"permission" validate:"omitempty,max=80"`
	Sort       int                 `json:"sort"`
	Active     bool                `json:"active"`
	Buttons    []menuButtonRequest `json:"buttons" validate:"omitempty,dive"`
}

type menuMetaRequest struct {
	ActiveName     string `json:"active_name" validate:"omitempty,max=160"`
	KeepAlive      bool   `json:"keep_alive"`
	DefaultMenu    bool   `json:"default_menu"`
	CloseTab       bool   `json:"close_tab"`
	TransitionType string `json:"transition_type" validate:"omitempty,max=80"`
}

type menuButtonRequest struct {
	ID          int64  `json:"id" validate:"omitempty,gte=0"`
	Name        string `json:"name" validate:"required,max=80"`
	Description string `json:"description" validate:"omitempty,max=120"`
}
