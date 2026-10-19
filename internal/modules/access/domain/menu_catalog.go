// Menu and Root Role installation catalog. The catalog is deployment-owned
// baseline data: it changes only through explicit access-owned upgrade work,
// never through administrator CRUD.
package domain

import "time"

// MenuGroupComponent is the component assigned to first-level navigation
// groups that only lay out their children.
const MenuGroupComponent = "Layout"

// rootRoleName is the display name of the Root Role baseline.
const rootRoleName = "超级管理员"

// MenuButtonSeed describes one page-level operation button in the menu catalog.
type MenuButtonSeed struct {
	Name        string
	Description string
}

// MenuSeed describes one menu row in the installation catalog. ParentPath
// references another catalog entry's Path because no row IDs exist yet at
// installation time; empty means a first-level menu.
type MenuSeed struct {
	Name       string
	Path       string
	ParentPath string
	Icon       string
	Hidden     bool
	Component  string
	Meta       MenuMeta
	Permission string
	Sort       int
	Buttons    []MenuButtonSeed
}

var menuCatalog = []MenuSeed{
	{Name: "工作台", Path: "/dashboard", Icon: "dashboard", Component: "./Dashboard", Sort: 10},
	{Name: "组织权限", Path: "/access", Icon: "safety", Component: MenuGroupComponent, Sort: 20},
	{Name: "管理员管理", Path: "/admins", ParentPath: "/access", Icon: "user", Component: "./Admins", Permission: PermissionAdminRead, Sort: 21, Buttons: []MenuButtonSeed{
		{Name: "create", Description: "新增管理员"},
		{Name: "update", Description: "编辑管理员"},
		{Name: "delete", Description: "删除管理员"},
	}},
	{Name: "角色权限", Path: "/roles", ParentPath: "/access", Icon: "safety", Component: "./Roles", Permission: PermissionRoleRead, Sort: 22, Buttons: []MenuButtonSeed{
		{Name: "create", Description: "新增角色"},
		{Name: "update", Description: "编辑角色"},
		{Name: "delete", Description: "删除角色"},
		{Name: "copy", Description: "复制角色"},
		{Name: "members", Description: "授权角色成员"},
	}},
	{Name: "菜单管理", Path: "/menus", ParentPath: "/access", Icon: "menu", Component: "./Menus", Permission: PermissionMenuRead, Meta: MenuMeta{KeepAlive: true}, Sort: 23, Buttons: []MenuButtonSeed{
		{Name: "create", Description: "新增菜单"},
		{Name: "update", Description: "编辑菜单"},
		{Name: "delete", Description: "删除菜单"},
		{Name: "roles", Description: "授权菜单角色"},
	}},
	{Name: "受管API路由目录", Path: "/apis", ParentPath: "/access", Icon: "api", Component: "./APIs", Permission: PermissionAPIRead, Meta: MenuMeta{KeepAlive: true}, Sort: 24, Buttons: []MenuButtonSeed{
		{Name: "grant", Description: "授权API角色"},
	}},
	{Name: "API Token", Path: "/api-tokens", ParentPath: "/access", Icon: "key", Component: "./APITokens", Permission: PermissionAPITokenRead, Meta: MenuMeta{KeepAlive: true}, Sort: 25, Buttons: []MenuButtonSeed{
		{Name: "create", Description: "新增API Token"},
		{Name: "update", Description: "编辑API Token"},
		{Name: "delete", Description: "删除API Token"},
	}},
	{Name: "系统管理", Path: "/system", Icon: "setting", Component: MenuGroupComponent, Sort: 30},
	{Name: "系统配置", Path: "/configs", ParentPath: "/system", Icon: "setting", Component: "./Configs", Permission: PermissionConfigRead, Sort: 31, Buttons: []MenuButtonSeed{
		{Name: "update", Description: "更新配置"},
		{Name: "delete", Description: "删除配置"},
	}},
	{Name: "系统参数", Path: "/params", ParentPath: "/system", Icon: "control", Component: "./Params", Permission: PermissionParamRead, Meta: MenuMeta{KeepAlive: true}, Sort: 32, Buttons: []MenuButtonSeed{
		{Name: "create", Description: "新增参数"},
		{Name: "update", Description: "编辑参数"},
		{Name: "delete", Description: "删除参数"},
	}},
	{Name: "版本管理", Path: "/versions", ParentPath: "/system", Icon: "server", Component: "./Versions", Permission: PermissionVersionRead, Sort: 33, Buttons: []MenuButtonSeed{
		{Name: "create", Description: "新增版本记录"},
		{Name: "export", Description: "导出版本包"},
		{Name: "import", Description: "导入版本包"},
		{Name: "update", Description: "编辑版本记录"},
		{Name: "delete", Description: "删除版本记录"},
	}},
	{Name: "数据字典", Path: "/dictionaries", ParentPath: "/system", Icon: "profile", Component: "./Dictionaries", Permission: PermissionDictRead, Sort: 34, Buttons: []MenuButtonSeed{
		{Name: "create", Description: "新增字典"},
		{Name: "export", Description: "导出字典"},
		{Name: "import", Description: "导入字典"},
		{Name: "update", Description: "编辑字典"},
		{Name: "delete", Description: "删除字典"},
		{Name: "item_create", Description: "新增字典项"},
		{Name: "item_update", Description: "编辑字典项"},
		{Name: "item_delete", Description: "删除字典项"},
	}},
	{Name: "资源管理", Path: "/resources", Icon: "folder", Component: MenuGroupComponent, Sort: 40},
	{Name: "文件上传", Path: "/files", ParentPath: "/resources", Icon: "upload", Component: "./Files", Permission: PermissionFileRead, Sort: 41, Buttons: []MenuButtonSeed{
		{Name: "upload", Description: "上传文件"},
		{Name: "update", Description: "重命名文件"},
		{Name: "delete", Description: "删除文件"},
		{Name: "category_create", Description: "新增文件分类"},
		{Name: "category_update", Description: "编辑文件分类"},
		{Name: "category_delete", Description: "删除文件分类"},
	}},
	{Name: "运维审计", Path: "/audit", Icon: "fileSearch", Component: MenuGroupComponent, Sort: 50},
	{Name: "审计日志", Path: "/logs", ParentPath: "/audit", Icon: "fileSearch", Component: "./Logs", Permission: PermissionLogRead, Sort: 51, Buttons: []MenuButtonSeed{
		{Name: "resolve", Description: "处理系统错误"},
		{Name: "delete", Description: "删除日志"},
	}},
}

// MenuCatalog returns the installation menu catalog. Grants derived from it
// change only through explicit access-owned upgrade work.
func MenuCatalog() []MenuSeed {
	return append([]MenuSeed(nil), menuCatalog...)
}

// NewRootRole builds the Root Role authorization baseline: complete explicit
// grants over the installed menu, API, and button catalogs plus every existing
// role as data authority. The adapter appends the persisted row's own ID to
// the data-role grants afterwards because the database assigns that ID.
func NewRootRole(menuIDs, apiIDs, buttonIDs, roleIDs []int64, now time.Time) (Role, error) {
	return RestoreRole(0, 0, RoleCodeSuperAdmin, rootRoleName, PermissionCatalogTokens(), menuIDs, apiIDs, buttonIDs, roleIDs, DefaultRolePath, true, now, now)
}
