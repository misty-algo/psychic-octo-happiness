# Echo Admin Web

Echo Admin 的中后台前端，基于 Umi Max、React、Ant Design 和 ProComponents。

## 开发命令

```sh
npm install
npm run dev
npm run lint
npm run test
npm run build
```

开发服务会把 `/api` 代理到 `ECHO_ADMIN_WEB_API_TARGET`，默认是 `http://127.0.0.1:9322`。后端接口路径以 `/api` 开头，登录后浏览器通过 HttpOnly cookie 携带 Login Session，前端不会保存登录凭证到 `localStorage`。生产部署仍按同源 `/api` 访问，unsafe method 会从 `csrf_token` cookie 读取并发送 `X-CSRF-Token`。

Utoo 的 persistent cache 使用项目级 `.turbopack/lock`。如果本地 dev server 正在运行，构建验证可使用 `ECHO_ADMIN_WEB_DISABLE_UTOOPACK_CACHE=1 npm run build` 禁用 persistent cache，避免停止当前 dev server。若 CI 或沙箱环境不允许 utoopack 创建子进程，可使用 `ECHO_ADMIN_WEB_DISABLE_UTOOPACK=1 npm run build` 走默认 webpack 构建。

登录态中的当前角色由后端 `/api/auth/me` 返回。切换角色时调用 `/api/auth/role`，后端更新当前 Login Session，前端按新角色的菜单、按钮权限、数据权限和默认入口刷新界面。角色编辑以功能权限为单一授权面：提交时由权限 token 派生菜单 `menu_ids`、API `api_ids` 和按钮 `button_ids`，自助路由（当前用户、登出、改密、切角色、上传文件访问）对所有角色默认授权。工作台展示当前用户、已授权能力、后台菜单、应用信息和 capability 状态。受管 API 路由目录只读查看 deployment-owned 路由，不提供新增、编辑、删除；角色对路由的放行由角色编辑的功能权限自动派生，页面不再提供单独的角色授权入口。版本管理页面维护发布记录，支持选择菜单和字典导出版本 JSON；导入包不能包含 API 路由定义。API Token 页面创建 token 后只展示一次明文，后续列表只显示 token 前缀、状态、归属和使用时间。系统参数、数据字典、文件和日志页面继续提供各自的管理能力。

## 目录

- `src/pages`：后台页面，包括工作台、管理员、角色、API、API Token、菜单、配置、参数、版本、字典、文件和日志。
- `src/services/admin.ts`：后台 API client 和 DTO 类型。
- `src/requestErrorConfig.ts`：统一请求错误处理、cookie credentials 和 CSRF header。
- `config/routes.ts`：前端路由。
- `config/config.ts`：Umi Max 配置。

## 约定

- 不使用 OpenAPI 生成器作为默认开发路径。
- 新 API 先在对应后端 business module 定义清楚，再在 `src/services/admin.ts` 增加显式方法。
- 页面只做表单、列表、状态和 DTO 转换，不承载核心业务规则。
- 路由可见性使用后端菜单控制，`hidden` 菜单不会进入侧边栏；菜单通过绑定权限 token 决定可见性，角色勾选对应功能权限后自动看到该菜单，菜单页不再提供单独的角色授权入口；页面写操作按钮使用 `resource:action` 权限 token 控制，并随菜单按钮种子一起管理。
- 构建产物 `dist`、Umi 生成目录和 Utoo cache 不提交。
