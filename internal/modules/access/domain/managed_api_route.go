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

	{Method: "GET", Pattern: "/api/ops/users", Description: "运营用户列表", Group: "ops", Permission: PermissionOpsUserRead},
	{Method: "GET", Pattern: "/api/ops/users/:id", Description: "用户完整信息（含手机号脱敏解除）", Group: "ops", Permission: PermissionOpsUserDetail},
	{Method: "POST", Pattern: "/api/ops/users/:id/restrict", Description: "限制用户", Group: "ops", Permission: PermissionOpsUserRestrict},
	{Method: "POST", Pattern: "/api/ops/users/:id/unrestrict", Description: "解除用户限制", Group: "ops", Permission: PermissionOpsUserRestrict},

	{Method: "GET", Pattern: "/api/ops/driver-audits", Description: "司机车辆审核工作台", Group: "ops", Permission: PermissionOpsAuditRead},
	{Method: "GET", Pattern: "/api/ops/driver-audits/:application_id", Description: "审核版本链", Group: "ops", Permission: PermissionOpsAuditRead},
	{Method: "POST", Pattern: "/api/ops/driver-audits/:application_id/revisions/:revision/claim", Description: "领取审核版本", Group: "ops", Permission: PermissionOpsAuditClaim},
	{Method: "POST", Pattern: "/api/ops/driver-audits/:application_id/revisions/:revision/decide", Description: "审核裁决", Group: "ops", Permission: PermissionOpsAuditDecide},

	{Method: "GET", Pattern: "/api/ops/orders", Description: "订单管理列表", Group: "ops", Permission: PermissionOpsOrderRead},
	{Method: "GET", Pattern: "/api/ops/orders/:order_no", Description: "订单穿透详情", Group: "ops", Permission: PermissionOpsOrderRead},
	{Method: "POST", Pattern: "/api/ops/orders/:order_no/commands", Description: "订单业务命令", Group: "ops", Permission: PermissionOpsOrderCommand},

	{Method: "GET", Pattern: "/api/ops/withdrawals", Description: "提现审批队列", Group: "ops", Permission: PermissionOpsOrderRead},
	{Method: "POST", Pattern: "/api/ops/withdrawals/:id/approve", Description: "通过提现申请", Group: "ops", Permission: PermissionOpsOrderCommand},
	{Method: "POST", Pattern: "/api/ops/withdrawals/:id/reject", Description: "驳回提现申请", Group: "ops", Permission: PermissionOpsOrderCommand},
	{Method: "POST", Pattern: "/api/ops/withdrawals/:id/pay", Description: "标记提现打款完成", Group: "ops", Permission: PermissionOpsOrderCommand},

	{Method: "GET", Pattern: "/api/ops/tickets", Description: "工单处理台列表", Group: "ops", Permission: PermissionOpsTicketRead},
	{Method: "GET", Pattern: "/api/ops/tickets/:ticket_no", Description: "工单详情", Group: "ops", Permission: PermissionOpsTicketRead},
	{Method: "POST", Pattern: "/api/ops/tickets/:ticket_no/commands", Description: "工单状态命令", Group: "ops", Permission: PermissionOpsTicketHandle},
	{Method: "POST", Pattern: "/api/ops/tickets/:ticket_no/appeal", Description: "工单申诉换人", Group: "ops", Permission: PermissionOpsTicketSenior},

	{Method: "GET", Pattern: "/api/ops/marketing/coupon-templates", Description: "券模板列表", Group: "ops", Permission: PermissionOpsMarketingRead},
	{Method: "GET", Pattern: "/api/ops/marketing/coupon-templates/:template_no", Description: "券模板详情", Group: "ops", Permission: PermissionOpsMarketingRead},
	{Method: "POST", Pattern: "/api/ops/marketing/coupon-templates", Description: "新建券模板", Group: "ops", Permission: PermissionOpsMarketingWrite},
	{Method: "POST", Pattern: "/api/ops/marketing/coupon-templates/:template_no/commands", Description: "券模板发布/停售", Group: "ops", Permission: PermissionOpsMarketingWrite},

	{Method: "GET", Pattern: "/api/ops/marketing/coupon-batches", Description: "券批次列表", Group: "ops", Permission: PermissionOpsMarketingRead},
	{Method: "POST", Pattern: "/api/ops/marketing/coupon-batches", Description: "新建券批次", Group: "ops", Permission: PermissionOpsMarketingWrite},
	{Method: "POST", Pattern: "/api/ops/marketing/coupon-batches/:batch_no/issue", Description: "发放优惠券", Group: "ops", Permission: PermissionOpsMarketingWrite},
	{Method: "POST", Pattern: "/api/ops/marketing/coupon-batches/:batch_no/commands", Description: "券批次停止/结束", Group: "ops", Permission: PermissionOpsMarketingWrite},
	{Method: "GET", Pattern: "/api/ops/marketing/coupons", Description: "已发放券查询", Group: "ops", Permission: PermissionOpsMarketingRead},
	{Method: "POST", Pattern: "/api/ops/marketing/coupons/publish", Description: "投放领券中心", Group: "ops", Permission: PermissionOpsMarketingWrite},

	{Method: "GET", Pattern: "/api/ops/marketing/campaigns", Description: "营销活动列表", Group: "ops", Permission: PermissionOpsMarketingRead},
	{Method: "POST", Pattern: "/api/ops/marketing/campaigns", Description: "新建营销活动", Group: "ops", Permission: PermissionOpsMarketingWrite},
	{Method: "POST", Pattern: "/api/ops/marketing/campaigns/:code/commands", Description: "活动启停", Group: "ops", Permission: PermissionOpsMarketingWrite},
	{Method: "GET", Pattern: "/api/ops/marketing/campaigns/:code/settlement", Description: "活动结算对账", Group: "ops", Permission: PermissionOpsMarketingSettle},
	{Method: "GET", Pattern: "/api/ops/marketing/campaigns/:code/settlement/export", Description: "活动对账单导出", Group: "ops", Permission: PermissionOpsMarketingSettle},

	{Method: "POST", Pattern: "/api/ops/marketing/campaigns/:code/subsidies", Description: "登记补贴发放", Group: "ops", Permission: PermissionOpsMarketingWrite},
	{Method: "GET", Pattern: "/api/ops/marketing/campaigns/:code/subsidies", Description: "补贴台账查询", Group: "ops", Permission: PermissionOpsMarketingRead},
	{Method: "GET", Pattern: "/api/ops/marketing/campaigns/:code/subsidy-summary", Description: "补贴口径汇总", Group: "ops", Permission: PermissionOpsMarketingRead},

	// A07 券作废与 A08 补贴追回
	{Method: "POST", Pattern: "/api/ops/marketing/coupons/:coupon_no/void", Description: "作废优惠券", Group: "ops", Permission: PermissionOpsMarketingWrite},
	{Method: "POST", Pattern: "/api/ops/marketing/campaigns/:code/subsidies/:record_no/reverse", Description: "追回补贴", Group: "ops", Permission: PermissionOpsMarketingWrite},

	// A05 计价规则管理
	{Method: "GET", Pattern: "/api/ops/pricing/rules", Description: "计价规则列表", Group: "ops", Permission: PermissionOpsPricingRead},
	{Method: "POST", Pattern: "/api/ops/pricing/rules", Description: "新建计价规则草稿", Group: "ops", Permission: PermissionOpsPricingWrite},
	{Method: "GET", Pattern: "/api/ops/pricing/rules/effective", Description: "查询生效计价版本", Group: "ops", Permission: PermissionOpsPricingRead},
	{Method: "GET", Pattern: "/api/ops/pricing/rules/:rule_id", Description: "计价规则详情", Group: "ops", Permission: PermissionOpsPricingRead},
	{Method: "POST", Pattern: "/api/ops/pricing/rules/:rule_id/publish", Description: "发布计价规则", Group: "ops", Permission: PermissionOpsPricingPublish},
	{Method: "POST", Pattern: "/api/ops/pricing/rules/:rule_id/disable", Description: "停用计价规则", Group: "ops", Permission: PermissionOpsPricingPublish},
	{Method: "POST", Pattern: "/api/ops/pricing/preview", Description: "计价分账样例预览", Group: "ops", Permission: PermissionOpsPricingRead},

	// A06 派单策略配置
	{Method: "GET", Pattern: "/api/ops/dispatch/strategies", Description: "派单策略列表", Group: "ops", Permission: PermissionOpsDispatchRead},
	{Method: "POST", Pattern: "/api/ops/dispatch/strategies", Description: "新建派单策略草稿", Group: "ops", Permission: PermissionOpsDispatchWrite},
	{Method: "POST", Pattern: "/api/ops/dispatch/strategies/:strategy_id/publish", Description: "发布派单策略", Group: "ops", Permission: PermissionOpsDispatchWrite},
	{Method: "POST", Pattern: "/api/ops/dispatch/strategies/:strategy_id/preview", Description: "派单候选排序预览", Group: "ops", Permission: PermissionOpsDispatchRead},

	// A09 拼车与特价配置
	{Method: "GET", Pattern: "/api/ops/pool/configs", Description: "拼车匹配配置列表", Group: "ops", Permission: PermissionOpsMarketingRead},
	{Method: "POST", Pattern: "/api/ops/pool/configs", Description: "新建拼车匹配配置", Group: "ops", Permission: PermissionOpsMarketingWrite},
	{Method: "POST", Pattern: "/api/ops/pool/configs/:config_id/publish", Description: "发布拼车匹配配置", Group: "ops", Permission: PermissionOpsMarketingWrite},
	{Method: "GET", Pattern: "/api/ops/pool/plans", Description: "特价方案列表", Group: "ops", Permission: PermissionOpsMarketingRead},
	{Method: "POST", Pattern: "/api/ops/pool/plans", Description: "新建特价方案", Group: "ops", Permission: PermissionOpsMarketingWrite},
	{Method: "POST", Pattern: "/api/ops/pool/plans/:plan_id/publish", Description: "发布特价方案", Group: "ops", Permission: PermissionOpsMarketingWrite},
	{Method: "GET", Pattern: "/api/ops/pool/stats", Description: "拼车统计口径", Group: "ops", Permission: PermissionOpsMarketingRead},
}

// ManagedAPIRouteCatalog returns the authoritative deployment definition.
// A fresh slice prevents callers from mutating the catalog shared by setup,
// explicit authorization upgrades, and boot coverage tests.
func ManagedAPIRouteCatalog() []ManagedAPIRouteDefinition {
	definitions := make([]ManagedAPIRouteDefinition, len(managedAPIRouteCatalog))
	copy(definitions, managedAPIRouteCatalog[:])
	return definitions
}
