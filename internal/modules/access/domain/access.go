// Package domain contains role, permission, and menu business rules.
package domain

import (
	"errors"
	"strings"
	"time"
)

const (
	// RoleCodeSuperAdmin is the seeded root role with every platform grant.
	RoleCodeSuperAdmin = "super_admin"
	// DefaultRolePath is used when a role has not chosen a more specific home page.
	DefaultRolePath = "/dashboard"
)

// Permission names used by the back-office foundation modules.
const (
	PermissionAdminRead   = "admin:read"
	PermissionAdminCreate = "admin:create"
	PermissionAdminUpdate = "admin:update"
	PermissionAdminDelete = "admin:delete"

	PermissionRoleRead   = "role:read"
	PermissionRoleCreate = "role:create"
	PermissionRoleUpdate = "role:update"
	PermissionRoleDelete = "role:delete"

	PermissionMenuRead   = "menu:read"
	PermissionMenuCreate = "menu:create"
	PermissionMenuUpdate = "menu:update"
	PermissionMenuDelete = "menu:delete"

	PermissionAPIRead  = "api:read"
	PermissionAPIGrant = "api:grant"

	PermissionAPITokenRead   = "api_token:read"
	PermissionAPITokenCreate = "api_token:create"
	PermissionAPITokenUpdate = "api_token:update"
	PermissionAPITokenDelete = "api_token:delete"

	PermissionConfigRead   = "config:read"
	PermissionConfigUpdate = "config:update"
	PermissionConfigDelete = "config:delete"

	PermissionParamRead   = "param:read"
	PermissionParamCreate = "param:create"
	PermissionParamUpdate = "param:update"
	PermissionParamDelete = "param:delete"

	PermissionVersionRead   = "version:read"
	PermissionVersionCreate = "version:create"
	PermissionVersionUpdate = "version:update"
	PermissionVersionDelete = "version:delete"

	PermissionDictRead   = "dict:read"
	PermissionDictCreate = "dict:create"
	PermissionDictUpdate = "dict:update"
	PermissionDictDelete = "dict:delete"

	PermissionFileRead   = "file:read"
	PermissionFileUpload = "file:upload"
	PermissionFileUpdate = "file:update"
	PermissionFileDelete = "file:delete"

	PermissionFileCategoryCreate = "file_category:create"
	PermissionFileCategoryUpdate = "file_category:update"
	PermissionFileCategoryDelete = "file_category:delete"

	PermissionLogRead    = "log:read"
	PermissionLogDelete  = "log:delete"
	PermissionLogResolve = "log:resolve"

	PermissionOpsUserRead     = "ops_user:read"
	PermissionOpsUserDetail   = "ops_user:detail"
	PermissionOpsUserRestrict = "ops_user:restrict"

	PermissionOpsAuditRead   = "ops_audit:read"
	PermissionOpsAuditClaim  = "ops_audit:claim"
	PermissionOpsAuditDecide = "ops_audit:decide"

	PermissionOpsOrderRead    = "ops_order:read"
	PermissionOpsOrderCommand = "ops_order:command"

	PermissionOpsTicketRead   = "ops_ticket:read"
	PermissionOpsTicketHandle = "ops_ticket:command"
	PermissionOpsTicketSenior = "ops_ticket:senior"

	PermissionOpsMarketingRead   = "ops_marketing:read"
	PermissionOpsMarketingWrite  = "ops_marketing:write"
	PermissionOpsMarketingSettle = "ops_marketing:settle"

	// A05 计价规则：改价是高风险动作，发布单独一个权限，不与管理混用。
	PermissionOpsPricingRead   = "ops_pricing:read"
	PermissionOpsPricingWrite  = "ops_pricing:write"
	PermissionOpsPricingPublish = "ops_pricing:publish"

	// A06 派单策略。
	PermissionOpsDispatchRead  = "ops_dispatch:read"
	PermissionOpsDispatchWrite = "ops_dispatch:write"
)

// PermissionDefinition describes one grantable action exposed to role managers.
type PermissionDefinition struct {
	Token    string
	Resource string
	Action   string
	Name     string
}

// PermissionCatalog returns the canonical grant list used by seed and UI metadata.
func PermissionCatalog() []PermissionDefinition {
	return append([]PermissionDefinition(nil), permissionCatalog...)
}

// PermissionCatalogTokens returns the token projection of the permission
// catalog for seed and authorization upgrade writes. Each call builds a
// fresh slice so callers cannot mutate the catalog through it.
func PermissionCatalogTokens() []string {
	definitions := PermissionCatalog()
	tokens := make([]string, 0, len(definitions))
	for _, definition := range definitions {
		tokens = append(tokens, definition.Token)
	}
	return tokens
}

// Access validation errors.
var (
	ErrInvalidRoleID      = errors.New("invalid role id")
	ErrInvalidRoleParent  = errors.New("invalid role parent")
	ErrInvalidRoleCode    = errors.New("invalid role code")
	ErrInvalidRoleName    = errors.New("invalid role name")
	ErrRoleNeedsPerms     = errors.New("role needs at least one permission")
	ErrInvalidDefaultPath = errors.New("invalid default path")
	ErrInvalidMenuID      = errors.New("invalid menu id")
	ErrInvalidMenuName    = errors.New("invalid menu name")
	ErrInvalidMenuPath    = errors.New("invalid menu path")
	ErrInvalidComponent   = errors.New("invalid menu component")
	ErrInvalidMenuMeta    = errors.New("invalid menu meta")
	ErrInvalidPermission  = errors.New("invalid permission")
	ErrInvalidButtonID    = errors.New("invalid menu button id")
	ErrInvalidButtonName  = errors.New("invalid menu button name")
	ErrInvalidButtonDesc  = errors.New("invalid menu button description")
	ErrInvalidAPIID       = errors.New("invalid api id")
	ErrInvalidAPIPath     = errors.New("invalid api path")
	ErrInvalidAPIMethod   = errors.New("invalid api method")
	ErrInvalidAPIGroup    = errors.New("invalid api group")
	ErrInvalidAPIDesc     = errors.New("invalid api description")
)

// Role groups permissions, menu visibility, and delegation hierarchy for administrators.
type Role struct {
	ID          int64
	ParentID    int64
	Code        string
	Name        string
	Permissions []string
	MenuIDs     []int64
	APIIDs      []int64
	ButtonIDs   []int64
	DataRoleIDs []int64
	DefaultPath string
	Active      bool
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

// RestoreRole rebuilds a role from a trusted store representation.
func RestoreRole(id, parentID int64, code, name string, permissions []string, menuIDs, apiIDs, buttonIDs, dataRoleIDs []int64, defaultPath string, active bool, createdAt, updatedAt time.Time) (Role, error) {
	code = normalizeToken(code)
	name = strings.TrimSpace(name)
	defaultPath = normalizeDefaultPath(defaultPath)
	perms, permissionsOK := uniquePermissionTokens(permissions)

	if id < 0 {
		return Role{}, ErrInvalidRoleID
	}
	if parentID < 0 || (id > 0 && parentID == id) {
		return Role{}, ErrInvalidRoleParent
	}
	if code == "" || len(code) > 64 {
		return Role{}, ErrInvalidRoleCode
	}
	if name == "" || len(name) > 80 {
		return Role{}, ErrInvalidRoleName
	}
	if !permissionsOK {
		return Role{}, ErrInvalidPermission
	}
	if len(perms) == 0 {
		return Role{}, ErrRoleNeedsPerms
	}
	if !isSafePath(defaultPath) {
		return Role{}, ErrInvalidDefaultPath
	}
	return Role{
		ID:          id,
		ParentID:    parentID,
		Code:        code,
		Name:        name,
		Permissions: perms,
		MenuIDs:     uniquePositiveIDs(menuIDs),
		APIIDs:      uniquePositiveIDs(apiIDs),
		ButtonIDs:   uniquePositiveIDs(buttonIDs),
		DataRoleIDs: uniquePositiveIDs(dataRoleIDs),
		DefaultPath: defaultPath,
		Active:      active,
		CreatedAt:   createdAt,
		UpdatedAt:   updatedAt,
	}, nil
}

// IsSuperAdmin reports whether the role is the seeded root administrator role.
func (r Role) IsSuperAdmin() bool { return r.Code == RoleCodeSuperAdmin }

// HasAPI reports whether the role holds an explicit grant for one managed API.
func (r Role) HasAPI(apiID int64) bool {
	for _, id := range r.APIIDs {
		if id == apiID {
			return true
		}
	}
	return false
}

// Menu represents one visible navigation entry in the back-office UI.
type Menu struct {
	ID         int64
	ParentID   int64
	Name       string
	Path       string
	Icon       string
	Hidden     bool
	Component  string
	Meta       MenuMeta
	Permission string
	Sort       int
	Active     bool
	Buttons    []MenuButton
	CreatedAt  time.Time
	UpdatedAt  time.Time
}

// MenuMeta stores the router metadata gin-vue-admin keeps beside each menu.
type MenuMeta struct {
	ActiveName     string
	KeepAlive      bool
	DefaultMenu    bool
	CloseTab       bool
	TransitionType string
}

// MenuButton is a page-level operation key attached to one menu.
type MenuButton struct {
	ID          int64
	MenuID      int64
	Name        string
	Description string
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

// RestoreMenu rebuilds a menu from a trusted store representation.
func RestoreMenu(id, parentID int64, name, path, icon string, hidden bool, component string, meta MenuMeta, permission string, sort int, active bool, buttons []MenuButton, createdAt, updatedAt time.Time) (Menu, error) {
	name = strings.TrimSpace(name)
	path = strings.TrimSpace(path)
	icon = strings.TrimSpace(icon)
	component = strings.TrimSpace(component)
	meta = normalizeMenuMeta(meta)
	permission = normalizeToken(permission)
	menuID := id

	if err := validateMenuFields(id, parentID, name, path, component, meta, permission); err != nil {
		return Menu{}, err
	}
	buttons, err := normalizeMenuButtons(menuID, buttons)
	if err != nil {
		return Menu{}, err
	}
	return Menu{
		ID:         id,
		ParentID:   parentID,
		Name:       name,
		Path:       path,
		Icon:       icon,
		Hidden:     hidden,
		Component:  component,
		Meta:       meta,
		Permission: permission,
		Sort:       sort,
		Active:     active,
		Buttons:    buttons,
		CreatedAt:  createdAt,
		UpdatedAt:  updatedAt,
	}, nil
}

func validateMenuFields(id, parentID int64, name, path, component string, meta MenuMeta, permission string) error {
	if err := validateMenuIdentity(id, parentID); err != nil {
		return err
	}
	if err := validateMenuRoute(name, path, component, meta); err != nil {
		return err
	}
	if permission != "" && !isPermissionToken(permission) {
		return ErrInvalidPermission
	}
	return nil
}

func validateMenuIdentity(id, parentID int64) error {
	if id < 0 || parentID < 0 || (id > 0 && parentID == id) {
		return ErrInvalidMenuID
	}
	return nil
}

func validateMenuRoute(name, path, component string, meta MenuMeta) error {
	if name == "" || len(name) > 80 {
		return ErrInvalidMenuName
	}
	if !isSafePath(path) || len(path) > 160 {
		return ErrInvalidMenuPath
	}
	if component == "" || len(component) > 160 {
		return ErrInvalidComponent
	}
	if len(meta.ActiveName) > 160 || len(meta.TransitionType) > 80 {
		return ErrInvalidMenuMeta
	}
	return nil
}

// RestoreMenuButton rebuilds a page button from a trusted store representation.
func RestoreMenuButton(id, menuID int64, name, description string, createdAt, updatedAt time.Time) (MenuButton, error) {
	name = strings.TrimSpace(name)
	description = strings.TrimSpace(description)
	if id < 0 || menuID < 0 {
		return MenuButton{}, ErrInvalidButtonID
	}
	if name == "" || len(name) > 80 {
		return MenuButton{}, ErrInvalidButtonName
	}
	if len(description) > 120 {
		return MenuButton{}, ErrInvalidButtonDesc
	}
	return MenuButton{
		ID:          id,
		MenuID:      menuID,
		Name:        name,
		Description: description,
		CreatedAt:   createdAt,
		UpdatedAt:   updatedAt,
	}, nil
}

// API represents one backend route that can be assigned to a role.
type API struct {
	ID          int64
	Method      string
	Path        string
	Description string
	Group       string
	Permission  string
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

// RestoreAPI rebuilds an API route from a trusted store representation.
func RestoreAPI(id int64, method, path, description, group, permission string, createdAt, updatedAt time.Time) (API, error) {
	method = strings.ToUpper(strings.TrimSpace(method))
	path = strings.TrimSpace(path)
	description = strings.TrimSpace(description)
	group = strings.TrimSpace(group)
	permission = normalizeToken(permission)

	if id < 0 {
		return API{}, ErrInvalidAPIID
	}
	if !isHTTPMethod(method) {
		return API{}, ErrInvalidAPIMethod
	}
	if !isSafePath(path) || len(path) > 180 {
		return API{}, ErrInvalidAPIPath
	}
	if description == "" || len(description) > 120 {
		return API{}, ErrInvalidAPIDesc
	}
	if group == "" || len(group) > 80 {
		return API{}, ErrInvalidAPIGroup
	}
	if permission != "" && !isPermissionToken(permission) {
		return API{}, ErrInvalidPermission
	}
	return API{
		ID:          id,
		Method:      method,
		Path:        path,
		Description: description,
		Group:       group,
		Permission:  permission,
		CreatedAt:   createdAt,
		UpdatedAt:   updatedAt,
	}, nil
}

var permissionCatalog = []PermissionDefinition{
	{Token: PermissionAdminRead, Resource: "管理员", Action: "查看", Name: "查看管理员"},
	{Token: PermissionAdminCreate, Resource: "管理员", Action: "创建", Name: "创建管理员"},
	{Token: PermissionAdminUpdate, Resource: "管理员", Action: "更新", Name: "更新管理员"},
	{Token: PermissionAdminDelete, Resource: "管理员", Action: "删除", Name: "删除管理员"},
	{Token: PermissionRoleRead, Resource: "角色", Action: "查看", Name: "查看角色"},
	{Token: PermissionRoleCreate, Resource: "角色", Action: "创建", Name: "创建角色"},
	{Token: PermissionRoleUpdate, Resource: "角色", Action: "更新", Name: "更新角色"},
	{Token: PermissionRoleDelete, Resource: "角色", Action: "删除", Name: "删除角色"},
	{Token: PermissionMenuRead, Resource: "菜单", Action: "查看", Name: "查看菜单"},
	{Token: PermissionMenuCreate, Resource: "菜单", Action: "创建", Name: "创建菜单"},
	{Token: PermissionMenuUpdate, Resource: "菜单", Action: "更新", Name: "更新菜单"},
	{Token: PermissionMenuDelete, Resource: "菜单", Action: "删除", Name: "删除菜单"},
	{Token: PermissionAPIRead, Resource: "API", Action: "查看", Name: "查看API"},
	{Token: PermissionAPIGrant, Resource: "API", Action: "授权", Name: "授权API角色"},
	{Token: PermissionAPITokenRead, Resource: "API Token", Action: "查看", Name: "查看API Token"},
	{Token: PermissionAPITokenCreate, Resource: "API Token", Action: "创建", Name: "创建API Token"},
	{Token: PermissionAPITokenUpdate, Resource: "API Token", Action: "更新", Name: "更新API Token"},
	{Token: PermissionAPITokenDelete, Resource: "API Token", Action: "删除", Name: "删除API Token"},
	{Token: PermissionConfigRead, Resource: "系统配置", Action: "查看", Name: "查看系统配置"},
	{Token: PermissionConfigUpdate, Resource: "系统配置", Action: "更新", Name: "更新系统配置"},
	{Token: PermissionConfigDelete, Resource: "系统配置", Action: "删除", Name: "删除系统配置"},
	{Token: PermissionParamRead, Resource: "系统参数", Action: "查看", Name: "查看系统参数"},
	{Token: PermissionParamCreate, Resource: "系统参数", Action: "创建", Name: "创建系统参数"},
	{Token: PermissionParamUpdate, Resource: "系统参数", Action: "更新", Name: "更新系统参数"},
	{Token: PermissionParamDelete, Resource: "系统参数", Action: "删除", Name: "删除系统参数"},
	{Token: PermissionVersionRead, Resource: "版本管理", Action: "查看", Name: "查看版本管理"},
	{Token: PermissionVersionCreate, Resource: "版本管理", Action: "创建", Name: "创建版本记录"},
	{Token: PermissionVersionUpdate, Resource: "版本管理", Action: "更新", Name: "更新版本记录"},
	{Token: PermissionVersionDelete, Resource: "版本管理", Action: "删除", Name: "删除版本记录"},
	{Token: PermissionDictRead, Resource: "数据字典", Action: "查看", Name: "查看数据字典"},
	{Token: PermissionDictCreate, Resource: "数据字典", Action: "创建", Name: "创建数据字典"},
	{Token: PermissionDictUpdate, Resource: "数据字典", Action: "更新", Name: "更新数据字典"},
	{Token: PermissionDictDelete, Resource: "数据字典", Action: "删除", Name: "删除数据字典"},
	{Token: PermissionFileRead, Resource: "文件", Action: "查看", Name: "查看文件"},
	{Token: PermissionFileUpload, Resource: "文件", Action: "上传", Name: "上传文件"},
	{Token: PermissionFileUpdate, Resource: "文件", Action: "更新", Name: "更新文件"},
	{Token: PermissionFileDelete, Resource: "文件", Action: "删除", Name: "删除文件"},
	{Token: PermissionFileCategoryCreate, Resource: "文件分类", Action: "创建", Name: "创建文件分类"},
	{Token: PermissionFileCategoryUpdate, Resource: "文件分类", Action: "更新", Name: "更新文件分类"},
	{Token: PermissionFileCategoryDelete, Resource: "文件分类", Action: "删除", Name: "删除文件分类"},
	{Token: PermissionLogRead, Resource: "审计日志", Action: "查看", Name: "查看日志"},
	{Token: PermissionLogDelete, Resource: "审计日志", Action: "删除", Name: "删除日志"},
	{Token: PermissionLogResolve, Resource: "审计日志", Action: "处理", Name: "处理系统错误日志"},

	{Token: PermissionOpsUserRead, Resource: "运营用户", Action: "查看", Name: "查看运营用户列表"},
	{Token: PermissionOpsUserDetail, Resource: "运营用户", Action: "查看详情", Name: "查看用户完整信息"},
	{Token: PermissionOpsUserRestrict, Resource: "运营用户", Action: "限制", Name: "限制与解除用户"},

	{Token: PermissionOpsAuditRead, Resource: "司机车辆审核", Action: "查看", Name: "查看审核工作台与版本链"},
	{Token: PermissionOpsAuditClaim, Resource: "司机车辆审核", Action: "领取", Name: "领取审核版本"},
	{Token: PermissionOpsAuditDecide, Resource: "司机车辆审核", Action: "裁决", Name: "批准或驳回审核申请"},

	{Token: PermissionOpsOrderRead, Resource: "订单管理", Action: "查看", Name: "查看订单列表与穿透详情"},
	{Token: PermissionOpsOrderCommand, Resource: "订单管理", Action: "命令", Name: "执行订单业务命令"},

	{Token: PermissionOpsTicketRead, Resource: "投诉工单", Action: "查看", Name: "查看工单处理台与流转记录"},
	{Token: PermissionOpsTicketHandle, Resource: "投诉工单", Action: "处理", Name: "领单、回复、暂停续算、解决与关闭"},
	{Token: PermissionOpsTicketSenior, Resource: "投诉工单", Action: "申诉", Name: "高级客服处理申诉与误报撤销"},

	{Token: PermissionOpsMarketingRead, Resource: "营销中心", Action: "查看", Name: "查看券模板、券批次、发放记录与活动"},
	{Token: PermissionOpsMarketingWrite, Resource: "营销中心", Action: "管理", Name: "创建券模板与批次、发放优惠券、启停活动"},
	{Token: PermissionOpsMarketingSettle, Resource: "营销中心", Action: "对账", Name: "查看活动结算并导出财务对账单"},

	{Token: PermissionOpsPricingRead, Resource: "计价规则", Action: "查看", Name: "查看计价规则与分账样例预览"},
	{Token: PermissionOpsPricingWrite, Resource: "计价规则", Action: "管理", Name: "新建计价规则草稿"},
	{Token: PermissionOpsPricingPublish, Resource: "计价规则", Action: "发布", Name: "发布与停用计价版本"},

	{Token: PermissionOpsDispatchRead, Resource: "派单策略", Action: "查看", Name: "查看派单策略与候选排序预览"},
	{Token: PermissionOpsDispatchWrite, Resource: "派单策略", Action: "管理", Name: "新建与发布派单策略"},
}

func isHTTPMethod(value string) bool {
	switch value {
	case "GET", "POST", "PUT", "PATCH", "DELETE":
		return true
	default:
		return false
	}
}

func uniquePermissionTokens(values []string) ([]string, bool) {
	seen := make(map[string]struct{}, len(values))
	out := make([]string, 0, len(values))
	for _, value := range values {
		token := normalizeToken(value)
		if !isPermissionToken(token) {
			return nil, false
		}
		if _, ok := seen[token]; ok {
			continue
		}
		seen[token] = struct{}{}
		out = append(out, token)
	}
	return out, true
}

func isPermissionToken(value string) bool {
	if value == "" || len(value) > 80 {
		return false
	}
	parts := strings.Split(value, ":")
	return len(parts) == 2 && parts[0] != "" && parts[1] != ""
}

func isSafePath(value string) bool {
	return value != "" && strings.HasPrefix(value, "/") && !strings.HasPrefix(value, "//")
}

func normalizeMenuMeta(meta MenuMeta) MenuMeta {
	meta.ActiveName = strings.TrimSpace(meta.ActiveName)
	meta.TransitionType = strings.TrimSpace(meta.TransitionType)
	return meta
}

func normalizeMenuButtons(menuID int64, buttons []MenuButton) ([]MenuButton, error) {
	seenIDs := make(map[int64]struct{}, len(buttons))
	seenNames := make(map[string]struct{}, len(buttons))
	out := make([]MenuButton, 0, len(buttons))
	for _, button := range buttons {
		if button.MenuID == 0 {
			button.MenuID = menuID
		}
		normalized, err := RestoreMenuButton(button.ID, button.MenuID, button.Name, button.Description, button.CreatedAt, button.UpdatedAt)
		if err != nil {
			return nil, err
		}
		if normalized.MenuID != menuID && menuID > 0 {
			return nil, ErrInvalidButtonID
		}
		if normalized.ID > 0 {
			if _, ok := seenIDs[normalized.ID]; ok {
				continue
			}
			seenIDs[normalized.ID] = struct{}{}
		}
		if _, ok := seenNames[normalized.Name]; ok {
			continue
		}
		seenNames[normalized.Name] = struct{}{}
		out = append(out, normalized)
	}
	return out, nil
}

func normalizeDefaultPath(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return DefaultRolePath
	}
	return value
}

func normalizeToken(value string) string {
	return strings.ToLower(strings.TrimSpace(value))
}

func uniquePositiveIDs(ids []int64) []int64 {
	seen := make(map[int64]struct{}, len(ids))
	out := make([]int64, 0, len(ids))
	for _, id := range ids {
		if id <= 0 {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		out = append(out, id)
	}
	return out
}
