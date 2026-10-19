// Package http exposes system first-initialization endpoints.
package http

import (
	"context"

	"github.com/labstack/echo/v5"

	"github.com/NSObjects/echo-admin/internal/modules/setup/usecase"
	"github.com/NSObjects/echo-admin/internal/platform/server/httpreq"
	"github.com/NSObjects/echo-admin/internal/platform/server/httpresp"
)

// SetupUsecase contains the setup workflows needed by the HTTP adapter.
type SetupUsecase interface {
	State(context.Context) (usecase.State, error)
	Submit(context.Context, usecase.SubmitInput) (usecase.State, error)
}

// Handler serves setup endpoints.
type Handler struct {
	usecase SetupUsecase
}

// New creates a setup HTTP handler.
func New(usecase SetupUsecase) *Handler {
	return &Handler{usecase: usecase}
}

// Register mounts setup routes on group.
func Register(group *echo.Group, handler *Handler) {
	group.GET("/setup/state", handler.State)
	group.POST("/setup", handler.Submit)
}

// State returns whether system first initialization has completed.
func (h *Handler) State(c *echo.Context) error {
	state, err := h.usecase.State(c.Request().Context())
	if err != nil {
		return err
	}
	return httpresp.OK(c, state)
}

// Submit completes system first initialization.
func (h *Handler) Submit(c *echo.Context) error {
	var req submitRequest
	if err := httpreq.BindAndValidate(c, &req); err != nil {
		return err
	}
	state, err := h.usecase.Submit(c.Request().Context(), usecase.SubmitInput{
		Username:    req.Username,
		DisplayName: req.DisplayName,
		Email:       req.Email,
		Password:    req.Password,
		SiteName:    req.SiteName,
	})
	if err != nil {
		return err
	}
	return httpresp.OK(c, state)
}
