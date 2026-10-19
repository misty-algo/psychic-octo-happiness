// Package apitokenhttp adapts API token HTTP requests to the token usecase.
package apitokenhttp

import (
	"strconv"

	"github.com/labstack/echo/v5"

	"github.com/NSObjects/echo-admin/internal/modules/apitoken/usecase"
	"github.com/NSObjects/echo-admin/internal/modules/audit/oprec"
	"github.com/NSObjects/echo-admin/internal/platform/server/httpreq"
	"github.com/NSObjects/echo-admin/internal/platform/server/httpresp"
)

// Handler adapts API token HTTP requests to the token usecase.
type Handler struct {
	usecase *usecase.Usecase
	audit   *oprec.Recorder
}

// New creates an API token HTTP handler.
func New(uc *usecase.Usecase, audit *oprec.Recorder) *Handler {
	return &Handler{usecase: uc, audit: audit}
}

// Register mounts API token routes on group.
func Register(group *echo.Group, handler *Handler) {
	group.GET("/api-tokens", handler.ListTokens)
	group.POST("/api-tokens", handler.CreateToken)
	group.PATCH("/api-tokens/:id", handler.UpdateToken)
	group.DELETE("/api-tokens/:id", handler.DeleteToken)
}

// ListTokens returns token metadata without raw secrets or hashes.
func (h *Handler) ListTokens(c *echo.Context) error {
	input, err := listInput(c)
	if err != nil {
		return err
	}
	output, err := h.usecase.ListTokens(c.Request().Context(), input)
	if err != nil {
		return err
	}
	return httpresp.Paginated(c, output.Items, output.Page, output.PageSize, output.Total)
}

// CreateToken creates an API token and returns the raw secret once.
func (h *Handler) CreateToken(c *echo.Context) error {
	var req tokenRequest
	if err := httpreq.BindAndValidate(c, &req); err != nil {
		return err
	}
	created, err := h.usecase.CreateToken(c.Request().Context(), tokenInputFromRequest(req))
	err = h.audit.Record(c, "create", "api_token", strconv.FormatInt(created.Token.ID, 10), "created api token", err)
	if err != nil {
		return err
	}
	return httpresp.Created(c, created)
}

// UpdateToken updates token metadata without changing the secret.
func (h *Handler) UpdateToken(c *echo.Context) error {
	id, err := httpreq.PathID(c, "id", "api token")
	if err != nil {
		return err
	}
	var req tokenRequest
	if bindErr := httpreq.BindAndValidate(c, &req); bindErr != nil {
		return bindErr
	}
	token, err := h.usecase.UpdateToken(c.Request().Context(), usecase.UpdateTokenInput{
		ID:          id,
		Name:        req.Name,
		Description: req.Description,
		Active:      req.Active,
		ExpiresAt:   req.ExpiresAt,
	})
	err = h.audit.Record(c, "update", "api_token", strconv.FormatInt(token.ID, 10), "updated api token", err)
	if err != nil {
		return err
	}
	return httpresp.OK(c, token)
}

// DeleteToken revokes an API token.
func (h *Handler) DeleteToken(c *echo.Context) error {
	id, err := httpreq.PathID(c, "id", "api token")
	if err != nil {
		return err
	}
	err = h.usecase.DeleteToken(c.Request().Context(), id)
	err = h.audit.Record(c, "delete", "api_token", strconv.FormatInt(id, 10), "revoked api token", err)
	if err != nil {
		return err
	}
	return httpresp.OK(c, deletedResponse{ID: id})
}

func listInput(c *echo.Context) (usecase.ListInput, error) {
	page, pageSize, err := httpreq.Pagination(c)
	if err != nil {
		return usecase.ListInput{}, err
	}
	adminID, err := httpreq.QueryInt64(c, "admin_id", 0)
	if err != nil {
		return usecase.ListInput{}, err
	}
	var active *bool
	if c.QueryParam("active") != "" {
		value, err := httpreq.QueryBool(c, "active", false)
		if err != nil {
			return usecase.ListInput{}, err
		}
		active = &value
	}
	return usecase.ListInput{Page: page, PageSize: pageSize, AdminID: adminID, Active: active}, nil
}

func tokenInputFromRequest(req tokenRequest) usecase.TokenInput {
	return usecase.TokenInput{
		AdminID:     req.AdminID,
		RoleID:      req.RoleID,
		Name:        req.Name,
		Description: req.Description,
		Active:      req.Active,
		Days:        req.Days,
		ExpiresAt:   req.ExpiresAt,
	}
}

type deletedResponse struct {
	ID int64 `json:"id"`
}
