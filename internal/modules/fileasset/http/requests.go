package fileassethttp

type importURLRequest struct {
	Name       string `json:"name" validate:"omitempty,max=180"`
	URL        string `json:"url" validate:"required,max=2048"`
	CategoryID int64  `json:"category_id" validate:"omitempty,min=0"`
}

type renameFileRequest struct {
	Name string `json:"name" validate:"required,max=180"`
}

type categoryRequest struct {
	Name     string `json:"name" validate:"required,max=80"`
	ParentID int64  `json:"parent_id" validate:"omitempty,min=0"`
}
