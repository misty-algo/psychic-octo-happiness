// Package accesshttp adapts role and menu HTTP requests to the access usecase.
package accesshttp

import (
	"strconv"

	"github.com/labstack/echo/v5"

	"github.com/NSObjects/echo-admin/internal/modules/access/usecase"
	"github.com/NSObjects/echo-admin/internal/modules/audit/oprec"
	"github.com/NSObjects/echo-admin/internal/platform/server/httpreq"
	"github.com/NSObjects/echo-admin/internal/platform/server/httpresp"
)

// Handler adapts role and menu HTTP requests to the access usecase.
type Handler struct {
	usecase *usecase.Usecase
	audit   *oprec.Recorder
}

// New creates an access HTTP handler.
func New(uc *usecase.Usecase, audit *oprec.Recorder) *Handler {
	return &Handler{usecase: uc, audit: audit}
}

// Register mounts role and menu routes on group.
func Register(group *echo.Group, handler *Handler) {
	group.GET("/permissions", handler.ListPermissions)
	group.GET("/roles", handler.ListRoles)
	group.POST("/roles", handler.CreateRole)
	group.PATCH("/roles/:id", handler.UpdateRole)
	group.DELETE("/roles/:id", handler.DeleteRole)
	group.POST("/roles/:id/copy", handler.CopyRole)
	group.GET("/apis", handler.ListAPIs)
	group.GET("/apis/groups", handler.ListAPIGroups)
	group.GET("/apis/:id", handler.ReadAPI)
	group.GET("/menus", handler.ListMenus)
	group.POST("/menus", handler.CreateMenu)
	group.GET("/menus/:id", handler.ReadMenu)
	group.PATCH("/menus/:id", handler.UpdateMenu)
	group.DELETE("/menus/:id", handler.DeleteMenu)
}

// ListPermissions returns grant metadata.
func (h *Handler) ListPermissions(c *echo.Context) error {
	// The catalog feeds role editors. Route authorization keeps disabled
	// identities, unassigned API tokens, and non-role-management users from
	// enumerating the grant surface.
	permissions, err := h.usecase.ListPermissions(c.Request().Context())
	if err != nil {
		return err
	}
	return httpresp.OK(c, permissions)
}

// ListRoles returns roles.
func (h *Handler) ListRoles(c *echo.Context) error {
	input, err := listInput(c)
	if err != nil {
		return err
	}
	output, err := h.usecase.ListRoles(c.Request().Context(), input)
	if err != nil {
		return err
	}
	return httpresp.Paginated(c, output.Items, output.Page, output.PageSize, output.Total)
}

// CreateRole creates a role.
func (h *Handler) CreateRole(c *echo.Context) error {
	var req createRoleRequest
	if err := httpreq.BindAndValidate(c, &req); err != nil {
		return err
	}
	role, opErr := h.usecase.CreateRole(c.Request().Context(), roleInputFromRequest(req))
	if err := h.audit.Record(c, "create", "role", strconv.FormatInt(role.ID, 10), "created role", opErr); err != nil {
		return err
	}
	if opErr != nil {
		return opErr
	}
	return httpresp.Created(c, role)
}

// UpdateRole updates a role.
func (h *Handler) UpdateRole(c *echo.Context) error {
	input, err := updateRoleInput(c)
	if err != nil {
		return err
	}
	role, opErr := h.usecase.UpdateRole(c.Request().Context(), input)
	if err := h.audit.Record(c, "update", "role", strconv.FormatInt(role.ID, 10), "updated role", opErr); err != nil {
		return err
	}
	if opErr != nil {
		return opErr
	}
	return httpresp.OK(c, role)
}

// DeleteRole deletes a role.
func (h *Handler) DeleteRole(c *echo.Context) error {
	id, err := httpreq.PathID(c, "id", "role")
	if err != nil {
		return err
	}
	opErr := h.usecase.DeleteRole(c.Request().Context(), id)
	if err := h.audit.Record(c, "delete", "role", strconv.FormatInt(id, 10), "deleted role", opErr); err != nil {
		return err
	}
	if opErr != nil {
		return opErr
	}
	return httpresp.OK(c, deletedResponse{ID: id})
}

// CopyRole copies a role with its grants.
func (h *Handler) CopyRole(c *echo.Context) error {
	id, err := httpreq.PathID(c, "id", "role")
	if err != nil {
		return err
	}
	var req copyRoleRequest
	if bindErr := httpreq.BindAndValidate(c, &req); bindErr != nil {
		return bindErr
	}
	role, opErr := h.usecase.CopyRole(c.Request().Context(), usecase.CopyRoleInput{
		SourceID:    id,
		ParentID:    req.ParentID,
		Code:        req.Code,
		Name:        req.Name,
		DefaultPath: req.DefaultPath,
		Active:      req.Active,
	})
	if err := h.audit.Record(c, "copy", "role", strconv.FormatInt(role.ID, 10), "copied role", opErr); err != nil {
		return err
	}
	if opErr != nil {
		return opErr
	}
	return httpresp.Created(c, role)
}

// ListAPIs returns API routes.
func (h *Handler) ListAPIs(c *echo.Context) error {
	input, err := listInput(c)
	if err != nil {
		return err
	}
	output, err := h.usecase.ListAPIs(c.Request().Context(), input)
	if err != nil {
		return err
	}
	return httpresp.Paginated(c, output.Items, output.Page, output.PageSize, output.Total)
}

// ListAPIGroups returns API group names.
func (h *Handler) ListAPIGroups(c *echo.Context) error {
	groups, err := h.usecase.APIGroups(c.Request().Context())
	if err != nil {
		return err
	}
	return httpresp.OK(c, apiGroupsResponse{Groups: groups})
}

// ReadAPI returns one API route.
func (h *Handler) ReadAPI(c *echo.Context) error {
	id, err := httpreq.PathID(c, "id", "api")
	if err != nil {
		return err
	}
	api, err := h.usecase.FindAPI(c.Request().Context(), id)
	if err != nil {
		return err
	}
	return httpresp.OK(c, api)
}

// ListMenus returns menus.
func (h *Handler) ListMenus(c *echo.Context) error {
	menus, err := h.usecase.ListMenus(c.Request().Context())
	if err != nil {
		return err
	}
	return httpresp.OK(c, menus)
}

// ReadMenu returns one menu.
func (h *Handler) ReadMenu(c *echo.Context) error {
	id, err := httpreq.PathID(c, "id", "menu")
	if err != nil {
		return err
	}
	menu, err := h.usecase.FindMenu(c.Request().Context(), id)
	if err != nil {
		return err
	}
	return httpresp.OK(c, menu)
}

// CreateMenu creates a menu.
func (h *Handler) CreateMenu(c *echo.Context) error {
	var req menuRequest
	if err := httpreq.BindAndValidate(c, &req); err != nil {
		return err
	}
	menu, opErr := h.usecase.CreateMenu(c.Request().Context(), menuInputFromRequest(req))
	if err := h.audit.Record(c, "create", "menu", strconv.FormatInt(menu.ID, 10), "created menu", opErr); err != nil {
		return err
	}
	if opErr != nil {
		return opErr
	}
	return httpresp.Created(c, menu)
}

// UpdateMenu updates a menu.
func (h *Handler) UpdateMenu(c *echo.Context) error {
	input, err := updateMenuInput(c)
	if err != nil {
		return err
	}
	menu, opErr := h.usecase.UpdateMenu(c.Request().Context(), input)
	if err := h.audit.Record(c, "update", "menu", strconv.FormatInt(menu.ID, 10), "updated menu", opErr); err != nil {
		return err
	}
	if opErr != nil {
		return opErr
	}
	return httpresp.OK(c, menu)
}

// DeleteMenu deletes a menu.
func (h *Handler) DeleteMenu(c *echo.Context) error {
	id, err := httpreq.PathID(c, "id", "menu")
	if err != nil {
		return err
	}
	opErr := h.usecase.DeleteMenu(c.Request().Context(), id)
	if err := h.audit.Record(c, "delete", "menu", strconv.FormatInt(id, 10), "deleted menu", opErr); err != nil {
		return err
	}
	if opErr != nil {
		return opErr
	}
	return httpresp.OK(c, deletedResponse{ID: id})
}

func listInput(c *echo.Context) (usecase.ListInput, error) {
	page, pageSize, err := httpreq.Pagination(c)
	if err != nil {
		return usecase.ListInput{}, err
	}
	return usecase.ListInput{Page: page, PageSize: pageSize}, nil
}

func roleInputFromRequest(req createRoleRequest) usecase.RoleInput {
	return usecase.RoleInput{
		ParentID:    req.ParentID,
		Code:        req.Code,
		Name:        req.Name,
		Permissions: req.Permissions,
		MenuIDs:     req.MenuIDs,
		APIIDs:      req.APIIDs,
		ButtonIDs:   req.ButtonIDs,
		DataRoleIDs: req.DataRoleIDs,
		DefaultPath: req.DefaultPath,
		Active:      req.Active,
	}
}

func updateRoleInput(c *echo.Context) (usecase.UpdateRoleInput, error) {
	id, err := httpreq.PathID(c, "id", "role")
	if err != nil {
		return usecase.UpdateRoleInput{}, err
	}
	var req updateRoleRequest
	if bindErr := httpreq.BindAndValidate(c, &req); bindErr != nil {
		return usecase.UpdateRoleInput{}, bindErr
	}
	return usecase.UpdateRoleInput{
		ID:          id,
		ParentID:    req.ParentID,
		Name:        req.Name,
		Permissions: req.Permissions,
		MenuIDs:     req.MenuIDs,
		APIIDs:      req.APIIDs,
		ButtonIDs:   req.ButtonIDs,
		DataRoleIDs: req.DataRoleIDs,
		DefaultPath: req.DefaultPath,
		Active:      req.Active,
	}, nil
}

func menuInputFromRequest(req menuRequest) usecase.MenuInput {
	return usecase.MenuInput{
		ParentID:  req.ParentID,
		Name:      req.Name,
		Path:      req.Path,
		Icon:      req.Icon,
		Hidden:    req.Hidden,
		Component: req.Component,
		Meta: usecase.MenuMetaInput{
			ActiveName:     req.Meta.ActiveName,
			KeepAlive:      req.Meta.KeepAlive,
			DefaultMenu:    req.Meta.DefaultMenu,
			CloseTab:       req.Meta.CloseTab,
			TransitionType: req.Meta.TransitionType,
		},
		Permission: req.Permission,
		Sort:       req.Sort,
		Active:     req.Active,
		Buttons:    buttonInputsFromRequest(req.Buttons),
	}
}

func updateMenuInput(c *echo.Context) (usecase.UpdateMenuInput, error) {
	id, err := httpreq.PathID(c, "id", "menu")
	if err != nil {
		return usecase.UpdateMenuInput{}, err
	}
	var req menuRequest
	if bindErr := httpreq.BindAndValidate(c, &req); bindErr != nil {
		return usecase.UpdateMenuInput{}, bindErr
	}
	return usecase.UpdateMenuInput{ID: id, MenuInput: menuInputFromRequest(req)}, nil
}

func buttonInputsFromRequest(buttons []menuButtonRequest) []usecase.MenuButtonInput {
	out := make([]usecase.MenuButtonInput, 0, len(buttons))
	for _, button := range buttons {
		out = append(out, usecase.MenuButtonInput{
			ID:          button.ID,
			Name:        button.Name,
			Description: button.Description,
		})
	}
	return out
}

type deletedResponse struct {
	ID int64 `json:"id"`
}

type apiGroupsResponse struct {
	Groups []string `json:"groups"`
}
