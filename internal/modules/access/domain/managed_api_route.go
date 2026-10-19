package domain

// ManagedAPIRouteDefinition is deployment-owned authorization metadata for
// one exact HTTP method and registered Echo route pattern.
type ManagedAPIRouteDefinition struct {
	Method      string
	Pattern     string
	Description string
	Group       string
	// Permission is Administration Authorization display metadata for Casbin
	// permission views, menus, buttons, and grant catalogs. Route enforcement
	// reads role API grants only and never consults this token.
	Permission string
}

var managedAPIRouteCatalog = [...]ManagedAPIRouteDefinition{
	{Method: "POST", Pattern: "/api/auth/logout", Description: "服务端退出当前登录会话", Group: "auth"},
	{Method: "POST", Pattern: "/api/auth/logout-others", Description: "撤销其他登录会话", Group: "auth"},
	{Method: "POST", Pattern: "/api/auth/password", Description: "当前管理员修改密码", Group: "auth"},
	{Method: "POST", Pattern: "/api/auth/role", Description: "切换当前角色", Group: "auth"},
	{Method: "GET", Pattern: "/api/auth/me", Description: "当前管理员", Group: "auth"},
	{Method: "PATCH", Pattern: "/api/auth/me", Description: "更新当前管理员资料", Group: "auth"},
	{Method: "GET", Pattern: "/api/admins", Description: "管理员列表", Group: "admin", Permission: PermissionAdminRead},
	{Method: "POST", Pattern: "/api/admins", Description: "创建管理员", Group: "admin", Permission: PermissionAdminCreate},
	{Method: "PATCH", Pattern: "/api/admins/:id", Description: "更新管理员", Group: "admin", Permission: PermissionAdminUpdate},
	{Method: "DELETE", Pattern: "/api/admins/:id", Description: "删除管理员", Group: "admin", Permission: PermissionAdminDelete},
	{Method: "GET", Pattern: "/api/roles", Description: "角色列表", Group: "role", Permission: PermissionRoleRead},
	{Method: "POST", Pattern: "/api/roles", Description: "创建角色", Group: "role", Permission: PermissionRoleCreate},
	{Method: "PATCH", Pattern: "/api/roles/:id", Description: "更新角色", Group: "role", Permission: PermissionRoleUpdate},
	{Method: "DELETE", Pattern: "/api/roles/:id", Description: "删除角色", Group: "role", Permission: PermissionRoleDelete},
	{Method: "POST", Pattern: "/api/roles/:id/copy", Description: "复制角色", Group: "role", Permission: PermissionRoleCreate},
	{Method: "GET", Pattern: "/api/roles/:id/admins", Description: "角色关联管理员", Group: "role", Permission: PermissionRoleRead},
	{Method: "PUT", Pattern: "/api/roles/:id/admins", Description: "更新角色关联管理员", Group: "role", Permission: PermissionRoleUpdate},
	{Method: "GET", Pattern: "/api/permissions", Description: "权限目录元数据", Group: "access", Permission: PermissionRoleRead},
	{Method: "GET", Pattern: "/api/apis", Description: "API列表", Group: "api", Permission: PermissionAPIRead},
	{Method: "GET", Pattern: "/api/apis/groups", Description: "API分组", Group: "api", Permission: PermissionAPIRead},
	{Method: "GET", Pattern: "/api/apis/:id", Description: "API详情", Group: "api", Permission: PermissionAPIRead},
	{Method: "GET", Pattern: "/api/api-tokens", Description: "API Token列表", Group: "api_token", Permission: PermissionAPITokenRead},
	{Method: "POST", Pattern: "/api/api-tokens", Description: "创建API Token", Group: "api_token", Permission: PermissionAPITokenCreate},
	{Method: "PATCH", Pattern: "/api/api-tokens/:id", Description: "更新API Token", Group: "api_token", Permission: PermissionAPITokenUpdate},
	{Method: "DELETE", Pattern: "/api/api-tokens/:id", Description: "删除API Token", Group: "api_token", Permission: PermissionAPITokenDelete},
	{Method: "GET", Pattern: "/api/menus", Description: "菜单列表", Group: "menu", Permission: PermissionMenuRead},
	{Method: "POST", Pattern: "/api/menus", Description: "创建菜单", Group: "menu", Permission: PermissionMenuCreate},
	{Method: "GET", Pattern: "/api/menus/:id", Description: "菜单详情", Group: "menu", Permission: PermissionMenuRead},
	{Method: "PATCH", Pattern: "/api/menus/:id", Description: "更新菜单", Group: "menu", Permission: PermissionMenuUpdate},
	{Method: "DELETE", Pattern: "/api/menus/:id", Description: "删除菜单", Group: "menu", Permission: PermissionMenuDelete},
	{Method: "GET", Pattern: "/api/system/configs", Description: "系统配置列表", Group: "config", Permission: PermissionConfigRead},
	{Method: "PUT", Pattern: "/api/system/configs/:key", Description: "创建或更新系统配置", Group: "config", Permission: PermissionConfigUpdate},
	{Method: "DELETE", Pattern: "/api/system/configs/:key", Description: "删除系统配置", Group: "config", Permission: PermissionConfigDelete},
	{Method: "GET", Pattern: "/api/system/params", Description: "系统参数列表", Group: "param", Permission: PermissionParamRead},
	{Method: "POST", Pattern: "/api/system/params", Description: "创建系统参数", Group: "param", Permission: PermissionParamCreate},
	{Method: "POST", Pattern: "/api/system/params/batch-delete", Description: "批量删除系统参数", Group: "param", Permission: PermissionParamDelete},
	{Method: "GET", Pattern: "/api/system/params/key/:key", Description: "按键获取系统参数", Group: "param", Permission: PermissionParamRead},
	{Method: "GET", Pattern: "/api/system/params/:id", Description: "系统参数详情", Group: "param", Permission: PermissionParamRead},
	{Method: "PATCH", Pattern: "/api/system/params/:id", Description: "更新系统参数", Group: "param", Permission: PermissionParamUpdate},
	{Method: "DELETE", Pattern: "/api/system/params/:id", Description: "删除系统参数", Group: "param", Permission: PermissionParamDelete},
	{Method: "GET", Pattern: "/api/system/versions", Description: "版本记录列表", Group: "version", Permission: PermissionVersionRead},
	{Method: "POST", Pattern: "/api/system/versions", Description: "创建版本记录", Group: "version", Permission: PermissionVersionCreate},
	{Method: "POST", Pattern: "/api/system/versions/export", Description: "导出版本包", Group: "version", Permission: PermissionVersionCreate},
	{Method: "POST", Pattern: "/api/system/versions/import", Description: "导入版本包", Group: "version", Permission: PermissionVersionCreate},
	{Method: "POST", Pattern: "/api/system/versions/batch-delete", Description: "批量删除版本记录", Group: "version", Permission: PermissionVersionDelete},
	{Method: "GET", Pattern: "/api/system/versions/:id", Description: "版本记录详情", Group: "version", Permission: PermissionVersionRead},
	{Method: "GET", Pattern: "/api/system/versions/:id/download", Description: "下载版本记录JSON", Group: "version", Permission: PermissionVersionRead},
	{Method: "PATCH", Pattern: "/api/system/versions/:id", Description: "更新版本记录", Group: "version", Permission: PermissionVersionUpdate},
	{Method: "DELETE", Pattern: "/api/system/versions/:id", Description: "删除版本记录", Group: "version", Permission: PermissionVersionDelete},
	{Method: "GET", Pattern: "/api/dictionaries", Description: "字典列表", Group: "dictionary", Permission: PermissionDictRead},
	{Method: "POST", Pattern: "/api/dictionaries", Description: "创建字典", Group: "dictionary", Permission: PermissionDictCreate},
	{Method: "GET", Pattern: "/api/dictionaries/export", Description: "导出字典", Group: "dictionary", Permission: PermissionDictRead},
	{Method: "POST", Pattern: "/api/dictionaries/import", Description: "导入字典", Group: "dictionary", Permission: PermissionDictCreate},
	{Method: "PATCH", Pattern: "/api/dictionaries/:code", Description: "更新字典", Group: "dictionary", Permission: PermissionDictUpdate},
	{Method: "DELETE", Pattern: "/api/dictionaries/:code", Description: "删除字典", Group: "dictionary", Permission: PermissionDictDelete},
	{Method: "POST", Pattern: "/api/dictionaries/:code/items", Description: "新增字典项", Group: "dictionary", Permission: PermissionDictCreate},
	{Method: "PATCH", Pattern: "/api/dictionaries/:code/items/:item_id", Description: "更新字典项", Group: "dictionary", Permission: PermissionDictUpdate},
	{Method: "DELETE", Pattern: "/api/dictionaries/:code/items/:item_id", Description: "删除字典项", Group: "dictionary", Permission: PermissionDictDelete},
	{Method: "GET", Pattern: "/api/file-categories", Description: "文件分类列表", Group: "file", Permission: PermissionFileRead},
	{Method: "POST", Pattern: "/api/file-categories", Description: "创建文件分类", Group: "file", Permission: PermissionFileCategoryCreate},
	{Method: "PATCH", Pattern: "/api/file-categories/:id", Description: "更新文件分类", Group: "file", Permission: PermissionFileCategoryUpdate},
	{Method: "DELETE", Pattern: "/api/file-categories/:id", Description: "删除文件分类", Group: "file", Permission: PermissionFileCategoryDelete},
	{Method: "GET", Pattern: "/api/files", Description: "文件列表", Group: "file", Permission: PermissionFileRead},
	{Method: "POST", Pattern: "/api/files", Description: "上传文件", Group: "file", Permission: PermissionFileUpload},
	{Method: "POST", Pattern: "/api/files/import-url", Description: "导入文件URL", Group: "file", Permission: PermissionFileUpload},
	{Method: "PATCH", Pattern: "/api/files/:id/name", Description: "重命名文件", Group: "file", Permission: PermissionFileUpdate},
	{Method: "DELETE", Pattern: "/api/files/:id", Description: "删除文件", Group: "file", Permission: PermissionFileDelete},
	{Method: "GET", Pattern: "/api/uploads/*", Description: "上传文件静态访问", Group: "file"},
	{Method: "GET", Pattern: "/api/logs/operations", Description: "操作日志", Group: "log", Permission: PermissionLogRead},
	{Method: "GET", Pattern: "/api/logs/operations/:id", Description: "操作日志详情", Group: "log", Permission: PermissionLogRead},
	{Method: "DELETE", Pattern: "/api/logs/operations/:id", Description: "删除操作日志", Group: "log", Permission: PermissionLogDelete},
	{Method: "POST", Pattern: "/api/logs/operations/batch-delete", Description: "批量删除操作日志", Group: "log", Permission: PermissionLogDelete},
	{Method: "GET", Pattern: "/api/logs/logins", Description: "登录日志", Group: "log", Permission: PermissionLogRead},
	{Method: "GET", Pattern: "/api/logs/logins/:id", Description: "登录日志详情", Group: "log", Permission: PermissionLogRead},
	{Method: "DELETE", Pattern: "/api/logs/logins/:id", Description: "删除登录日志", Group: "log", Permission: PermissionLogDelete},
	{Method: "POST", Pattern: "/api/logs/logins/batch-delete", Description: "批量删除登录日志", Group: "log", Permission: PermissionLogDelete},
	{Method: "GET", Pattern: "/api/logs/errors", Description: "系统错误日志", Group: "log", Permission: PermissionLogRead},
	{Method: "GET", Pattern: "/api/logs/errors/:id", Description: "系统错误日志详情", Group: "log", Permission: PermissionLogRead},
	{Method: "POST", Pattern: "/api/logs/errors/:id/resolve", Description: "处理系统错误日志", Group: "log", Permission: PermissionLogResolve},
	{Method: "DELETE", Pattern: "/api/logs/errors/:id/resolve", Description: "取消处理系统错误日志", Group: "log", Permission: PermissionLogResolve},
	{Method: "DELETE", Pattern: "/api/logs/errors/:id", Description: "删除系统错误日志", Group: "log", Permission: PermissionLogDelete},
	{Method: "POST", Pattern: "/api/logs/errors/batch-delete", Description: "批量删除系统错误日志", Group: "log", Permission: PermissionLogDelete},
}

// ManagedAPIRouteCatalog returns the authoritative deployment definition.
// A fresh slice prevents callers from mutating the catalog shared by setup,
// explicit authorization upgrades, and boot coverage tests.
func ManagedAPIRouteCatalog() []ManagedAPIRouteDefinition {
	definitions := make([]ManagedAPIRouteDefinition, len(managedAPIRouteCatalog))
	copy(definitions, managedAPIRouteCatalog[:])
	return definitions
}
