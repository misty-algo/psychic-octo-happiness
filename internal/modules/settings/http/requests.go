package settingshttp

import "time"

type configRequest struct {
	Name   string `json:"name" validate:"required,max=120"`
	Value  string `json:"value" validate:"max=4000"`
	Public bool   `json:"public"`
}

type paramRequest struct {
	Name  string `json:"name" validate:"required,max=120"`
	Key   string `json:"key" validate:"required,max=80"`
	Value string `json:"value" validate:"required,max=4000"`
	Desc  string `json:"desc" validate:"max=4000"`
}

type dictionaryRequest struct {
	Code string `json:"code" validate:"required,max=80"`
	Name string `json:"name" validate:"required,max=120"`
}

type updateDictionaryRequest struct {
	Name string `json:"name" validate:"required,max=120"`
}

type dictionaryItemRequest struct {
	ParentID int64  `json:"parent_id" validate:"omitempty,min=0"`
	Label    string `json:"label" validate:"required,max=120"`
	Value    string `json:"value" validate:"required,max=120"`
	Extend   string `json:"extend" validate:"max=4000"`
	Sort     int    `json:"sort"`
	Active   bool   `json:"active"`
}

type versionRequest struct {
	Version     string    `json:"version" validate:"required,max=80"`
	Name        string    `json:"name" validate:"required,max=120"`
	Description string    `json:"description" validate:"max=4000"`
	PublishedAt time.Time `json:"published_at"`
}

type exportVersionRequest struct {
	Version       string  `json:"version" validate:"required,max=80"`
	Name          string  `json:"name" validate:"required,max=120"`
	Description   string  `json:"description" validate:"max=4000"`
	MenuIDs       []int64 `json:"menu_ids" validate:"omitempty,dive,gt=0"`
	DictionaryIDs []int64 `json:"dictionary_ids" validate:"omitempty,dive,gt=0"`
}
