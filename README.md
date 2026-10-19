# echo-admin

`echo-admin` 是一个 module-first 的 Go 中后台基础框架。它提供登录认证、管理员管理、角色权限、菜单管理、API Token、系统配置、数据字典、文件上传和分类、操作日志、登录日志和系统错误日志，为后续业务模块提供统一后台管理能力。

项目后端使用 Go 1.26、Echo v5、Casbin 和 `samber/do`。前端位于 `web/`，使用 Umi Max、React、Ant Design 和 ProComponents。

## 架构边界

- `business module`：业务模块，位于 `internal/modules/<module>`。
- `platform`：平台运行时代码，位于 `internal/platform`。
- `capability`：可复用基础设施能力，例如 MySQL、Redis、MongoDB、tracing、logging。
- `composition root`：启动装配层，位于 `internal/boot`。

核心目录：

```text
.
├── cmd/                         # CLI 入口，默认读取 configs/config.toml
├── configs/                     # 静态配置示例和配置说明
├── internal/boot/               # composition root，负责装配资源、模块和路由
├── internal/modules/auth/       # 登录会话、当前用户和认证流程
├── internal/modules/identity/   # 管理员管理
├── internal/modules/access/     # 角色权限和菜单管理
├── internal/modules/apitoken/    # API Token 管理和 token 认证
├── internal/modules/settings/   # 系统配置和数据字典
├── internal/modules/fileasset/  # 文件上传元数据和分类
├── internal/modules/audit/      # 操作日志、登录日志和系统错误日志
├── internal/platform/           # HTTP runtime、配置、错误、请求响应、基础设施能力
├── web/                         # 前端中后台应用
├── k8s/                         # Kubernetes 示例
├── docker-compose.yaml          # 本地开发 Compose
├── Dockerfile                   # runtime image
└── Makefile                     # 构建、测试、验证命令
```

不要把本项目改回 `service/biz/data` 分层心智。业务路由只在 `internal/boot` 通过 `NewModule`、`Provide`、`Route` 显式装配，`internal/platform/server` 不 import 业务模块。

## 快速开始

后端依赖 MySQL 存储。先启动本地 MySQL：

```bash
docker compose up mysql
```

再启动 API：

```bash
export ECHO_ADMIN_MYSQL_PASSWORD="echo_admin_dev_password"
go run main.go --config configs/config.toml
```

默认监听 `:9322`。基础检查：

```bash
curl http://127.0.0.1:9322/api/health
curl http://127.0.0.1:9322/api/info
curl http://127.0.0.1:9322/api/ready
curl http://127.0.0.1:9322/api/capabilities
```

首次启动空库后，普通管理接口会保持未初始化状态。浏览器后台会先进入 `/setup`，由初始化页创建权限目录、API 目录、基础菜单、`super_admin` Root Role、系统配置、状态字典和首个管理员。这个管理员由初始化表单填写，并自动绑定最高权限 Root Role；初始化成功后再跳转到登录页。

```text
打开后台: http://127.0.0.1:8000/setup
填写: username / display_name / email(可选) / password / site_name(可选)
```

`super_admin` 是根角色，默认显式持有全部权限、全部基础菜单、全部 Managed API Route、全部按钮、全部数据角色和默认入口 `/dashboard`。管理员可以拥有多个角色；浏览器登录使用服务端 Login Session 和 HttpOnly cookie，当前生效角色保存在本次登录会话中。切换角色后后端更新当前登录会话，前端菜单、按钮权限和数据权限随 `/api/auth/me` 刷新。退出登录只撤销当前登录会话，`logout-others` 可撤销同一管理员的其他登录会话。

前端：

```bash
cd web
npm install
npm run dev
```

前端 dev server 默认把 `/api` 代理到 `http://127.0.0.1:9322`；如需连接其他后端，设置 `ECHO_ADMIN_WEB_API_TARGET`。生产部署按同源 `/api` 访问，浏览器登录后通过 cookie 携带登录会话，unsafe method 会自动带 `X-CSRF-Token`。机器客户端继续使用 `X-API-Token`。

## 配置

默认配置文件是 `configs/config.toml`。配置支持 TOML、YAML、JSON，未知配置字段会导致启动失败。环境变量前缀是 `ECHO_ADMIN_`。

浏览器后台默认使用 Login Session；服务端只把 opaque session token 写入 HttpOnly cookie，MySQL 只保存 SHA-256 哈希。状态变更请求使用 Echo CSRF middleware，前端从 `csrf_token` cookie 读取并发送 `X-CSRF-Token`。生产 HTTPS 部署应开启 secure cookie：

```toml
[http]
secure_cookies = true
```

数据库 host、port、database、username 和连接池等非敏感拓扑配置写在配置文件的 `[mysql]` 中；MySQL 密码可以写在私有配置文件，也可以由 `ECHO_ADMIN_MYSQL_PASSWORD` 注入。首次管理员密码不在静态配置中维护，由 `/setup` 初始化页提交，长度必须是 8 到 72 个字符。

可选 capability 默认关闭：

- `redis.enabled=false`
- `mongodb.enabled=false`
- `tracing.enabled=false`
- `http.cors.enabled=false`

MySQL 是后台基础能力的必需存储，示例配置默认 `mysql.enabled=true`。Redis、MongoDB、tracing 等可选资源启用后会进入 `/api/ready` 和 `/api/capabilities`。上传文件默认保存到 `admin.upload_dir`，本地示例值是 `uploads`。

完整配置说明见 `configs/README.md`。

## 本地 Compose

默认 `docker compose up` 启动 API 和 MySQL，公开端口默认绑定 `127.0.0.1`：

```bash
docker compose up
```

Redis、MongoDB 和 Jaeger 在 `resources` profile 下：

```bash
cp env.example .env
docker compose --profile resources up
```

`.env` 只给 Docker Compose 做本地开发端口和凭据替换；应用本身读取真实进程环境变量，不会自动加载 `.env` 文件。

资源服务使用 Compose 命名卷保存本地开发数据。需要重置本地资源状态时，显式运行 `docker compose down -v`。

## Foundation Modules

后台基础能力按边界拆成多个 business module：

```text
internal/modules/auth/           # 登录认证、当前用户和权限判断
internal/modules/identity/       # 管理员生命周期
internal/modules/access/         # 角色、权限和菜单
internal/modules/apitoken/       # API Token 管理和 token 认证
internal/modules/settings/       # 系统配置和数据字典
internal/modules/fileasset/      # 文件上传元数据和分类
internal/modules/audit/          # 操作日志、登录日志和系统错误日志
```

运行期只使用 MySQL adapter。各模块的 usecase 定义自己的 store interface，`internal/boot` 从已配置的 `*gorm.DB` 创建 concrete store，并负责跨模块装配，例如 auth 通过自己的小接口读取 identity/access，并通过 boot bridge 写入 audit。

`access` 中的授权判断基于 Casbin RBAC：管理员映射为 `user:<id>`，角色映射为 `role:<code>`，权限 token 必须是 `resource:action`，并在授权时映射为 Casbin 的 `{subject, object, action}`。当前生效角色决定本次请求的权限集合，已分配但未激活的其他角色不会参与授权。

`auth` 模块提供登录、当前用户、当前用户资料更新、角色切换、当前用户改密码和服务端退出登录，并通过消费方定义的小 interface 读取 `access` 生成的当前授权视图。浏览器登录态是 MySQL 持久化的 Login Session；cookie 中只保存 opaque token，表中只保存 token 哈希、当前角色、idle 过期时间、绝对过期时间和撤销信息。登录会话校验发生在 server middleware：过期、撤销、管理员不可用或角色不可用都会按未授权处理。当前用户改密码只撤销其他会话并保留当前会话；管理员被禁用、删除或被重置密码会撤销该管理员全部会话。API Token 是独立的机器客户端认证路径，不复用浏览器 cookie。

`access` 模块提供当前授权视图、精确路由授权、权限目录、Managed API Route Catalog、角色树、数据权限、菜单管理和菜单按钮管理。每次授权都会读取当前管理员及角色状态；当前授权视图在模块内部用 Casbin 计算权限并过滤菜单和按钮，精确路由授权只检查当前角色的显式 `api_ids` grant。Managed API Route 的 method、Echo 注册 pattern 和元数据由 `access` 代码目录唯一维护，System API Route 与 Bootstrap API Route 不进入该目录；System First Initialization 把目录持久化到 MySQL。后台对目录只读，不能创建、编辑、删除或通过版本包导入路由定义；角色对路由的放行在角色编辑时由功能权限派生，不再提供按路由或按菜单的手工授权端点。boot 在全部路由挂载后从 Echo route table 生成 manifest：System/Bootstrap 通过精确 method+pattern 豁免，其他 `/api` route 都是 Managed；存在 Managed route 却没有 Route Authorization 时 assembly 直接失败。默认部署测试要求 manifest 与代码目录双向完全一致。根角色同样必须显式持有完整目录 grant，不存在 `Public` 或 Root 绕过。角色通过 `parent_id` 形成委派树：根角色可以管理全部角色；普通角色只能查看自己和下级角色，只能把自己的下级角色分配给管理员，并且不能授予自己没有的权限、菜单、API、菜单按钮或数据角色。`data_role_ids` 是当前角色可见的管理员数据范围。菜单记录保存 `hidden`、`component`、`keep_alive`、`default_menu`、`close_tab`、`active_name` 和 `transition_type` 等路由元信息。

`settings` 模块提供系统配置、系统参数、数据字典和版本管理。系统配置支持创建、更新、删除，启动种子配置 `site_name` 不允许从后台删除。系统参数提供可分页参数表，支持名称/键筛选、按 ID 或 key 查询、创建、更新、单条删除和批量删除。数据字典支持字典项树形父子关系、扩展值、层级路径、防循环父级调整和父项删除保护。版本管理保存可审计的发布记录，支持选择菜单和字典导出版本包，以及导入版本包恢复菜单和字典；版本包明确拒绝 Managed API Route 内容，避免绕过 deployment-owned catalog。

`apitoken` 模块提供 API Token 列表、创建、更新、作废和请求认证。创建 token 时可指定目标管理员、目标角色和 1-365 天有效期；普通角色只能给自己当前身份签发，`super_admin` 可以给已启用且持有目标角色的管理员签发。明文只返回一次，MySQL 只保存 SHA-256 哈希和短前缀；后续列表和更新接口不会回显明文或哈希。服务端接受 `X-API-Token` 请求头，验证成功后把 token 绑定的管理员和角色写入 request context，后续仍由同一套 route 授权 middleware 校验 API 目录和角色 `api_ids`。停用或过期 token 会被拒绝，成功使用会更新 `last_used_at`；删除操作会作废 token，不物理删除记录。

`fileasset` 模块提供本地上传、外部 URL 元数据登记、文件重命名、文件删除和分类树管理。文件可以按分类筛选，上传和导入 URL 时可选择分类；删除分类只会把该分类下的文件归到未分类，不会删除文件资产。本地上传文件通过 `/api/uploads/*` 读取时同样经过统一 API route grant 拦截。

`audit` 模块保存操作日志、登录日志和系统错误日志，并支持单条和批量删除。系统错误日志由统一 HTTP 错误边界记录，只记录内部错误和 panic 恢复后的 5xx 响应；普通 4xx 业务错误不会写入系统错误表。错误记录包含安全响应码、请求路径、请求 ID、用户 ID 和诊断 detail，供超管在日志页排查；超管可以把系统错误标记为已处理、记录处理备注，也可以取消处理状态。

HTTP adapter 只做请求解析、validator 校验、DTO 转换、调用 usecase、统一响应。核心业务规则放在 `domain` 和 `usecase`。

## HTTP API

系统路由：

| Method | Path | 用途 |
| --- | --- | --- |
| `GET` | `/api/health` | 进程存活检查 |
| `GET` | `/api/info` | 应用名称、版本和当前时间 |
| `GET` | `/api/ready` | 整体 readiness |
| `GET` | `/api/capabilities` | capability 状态详情 |

后台基础路由：

| Method | Path | 用途 |
| --- | --- | --- |
| `POST` | `/api/auth/login` | 管理员登录 |
| `POST` | `/api/auth/role` | 切换当前登录会话的生效角色 |
| `POST` | `/api/auth/logout` | 服务端退出当前登录会话 |
| `POST` | `/api/auth/logout-others` | 撤销同一管理员的其他登录会话 |
| `POST` | `/api/auth/password` | 当前管理员修改密码并撤销其他登录会话 |
| `GET` | `/api/auth/me` | 当前管理员 |
| `PATCH` | `/api/auth/me` | 更新当前管理员资料 |
| `GET` | `/api/admins` | 管理员列表 |
| `POST` | `/api/admins` | 创建管理员 |
| `PATCH` | `/api/admins/:id` | 更新管理员 |
| `DELETE` | `/api/admins/:id` | 删除管理员 |
| `GET` | `/api/roles` | 角色列表 |
| `POST` | `/api/roles` | 创建角色 |
| `PATCH` | `/api/roles/:id` | 更新角色 |
| `DELETE` | `/api/roles/:id` | 删除角色 |
| `POST` | `/api/roles/:id/copy` | 复制角色 |
| `GET` | `/api/roles/:id/admins` | 角色关联管理员 |
| `PUT` | `/api/roles/:id/admins` | 覆盖角色关联管理员 |
| `GET` | `/api/permissions` | 权限目录元数据 |
| `GET` | `/api/apis` | API 列表 |
| `GET` | `/api/apis/groups` | API 分组 |
| `GET` | `/api/apis/:id` | API 详情 |
| `GET` | `/api/api-tokens` | API Token 列表 |
| `POST` | `/api/api-tokens` | 创建 API Token |
| `PATCH` | `/api/api-tokens/:id` | 更新 API Token |
| `DELETE` | `/api/api-tokens/:id` | 作废 API Token |
| `GET` | `/api/menus` | 菜单列表 |
| `POST` | `/api/menus` | 创建菜单 |
| `GET` | `/api/menus/:id` | 菜单详情 |
| `PATCH` | `/api/menus/:id` | 更新菜单 |
| `DELETE` | `/api/menus/:id` | 删除菜单 |
| `GET` | `/api/system/configs` | 系统配置列表 |
| `PUT` | `/api/system/configs/:key` | 创建或更新系统配置 |
| `DELETE` | `/api/system/configs/:key` | 删除系统配置 |
| `GET` | `/api/system/params` | 系统参数列表 |
| `POST` | `/api/system/params` | 创建系统参数 |
| `POST` | `/api/system/params/batch-delete` | 批量删除系统参数 |
| `GET` | `/api/system/params/key/:key` | 按键获取系统参数 |
| `GET` | `/api/system/params/:id` | 系统参数详情 |
| `PATCH` | `/api/system/params/:id` | 更新系统参数 |
| `DELETE` | `/api/system/params/:id` | 删除系统参数 |
| `GET` | `/api/system/versions` | 版本记录列表 |
| `POST` | `/api/system/versions` | 创建版本记录 |
| `POST` | `/api/system/versions/export` | 导出版本包 |
| `POST` | `/api/system/versions/import` | 导入版本包 |
| `POST` | `/api/system/versions/batch-delete` | 批量删除版本记录 |
| `GET` | `/api/system/versions/:id` | 版本记录详情 |
| `GET` | `/api/system/versions/:id/download` | 下载版本记录 JSON |
| `PATCH` | `/api/system/versions/:id` | 更新版本记录 |
| `DELETE` | `/api/system/versions/:id` | 删除版本记录 |
| `GET` | `/api/dictionaries` | 字典列表 |
| `POST` | `/api/dictionaries` | 创建字典 |
| `GET` | `/api/dictionaries/export` | 导出字典 JSON |
| `POST` | `/api/dictionaries/import` | 导入字典 JSON |
| `PATCH` | `/api/dictionaries/:code` | 更新字典 |
| `DELETE` | `/api/dictionaries/:code` | 删除字典 |
| `POST` | `/api/dictionaries/:code/items` | 新增字典项 |
| `PATCH` | `/api/dictionaries/:code/items/:item_id` | 更新字典项 |
| `DELETE` | `/api/dictionaries/:code/items/:item_id` | 删除字典项 |
| `GET` | `/api/file-categories` | 文件分类树 |
| `POST` | `/api/file-categories` | 创建文件分类 |
| `PATCH` | `/api/file-categories/:id` | 更新文件分类 |
| `DELETE` | `/api/file-categories/:id` | 删除文件分类 |
| `GET` | `/api/files` | 文件列表 |
| `POST` | `/api/files` | 上传文件 |
| `POST` | `/api/files/import-url` | 导入外部文件 URL |
| `PATCH` | `/api/files/:id/name` | 重命名文件 |
| `DELETE` | `/api/files/:id` | 删除文件 |
| `GET` | `/api/uploads/*` | 上传文件静态访问 |
| `GET` | `/api/logs/operations` | 操作日志 |
| `GET` | `/api/logs/operations/:id` | 操作日志详情 |
| `DELETE` | `/api/logs/operations/:id` | 删除操作日志 |
| `POST` | `/api/logs/operations/batch-delete` | 批量删除操作日志 |
| `GET` | `/api/logs/logins` | 登录日志 |
| `GET` | `/api/logs/logins/:id` | 登录日志详情 |
| `DELETE` | `/api/logs/logins/:id` | 删除登录日志 |
| `POST` | `/api/logs/logins/batch-delete` | 批量删除登录日志 |
| `GET` | `/api/logs/errors` | 系统错误日志 |
| `GET` | `/api/logs/errors/:id` | 系统错误日志详情 |
| `POST` | `/api/logs/errors/:id/resolve` | 标记系统错误已处理 |
| `DELETE` | `/api/logs/errors/:id/resolve` | 取消系统错误处理状态 |
| `DELETE` | `/api/logs/errors/:id` | 删除系统错误日志 |
| `POST` | `/api/logs/errors/batch-delete` | 批量删除系统错误日志 |

## 新业务模块

新增业务模块时，按这个顺序写：

1. `internal/modules/<module>/domain`
2. `internal/modules/<module>/usecase`
3. `internal/modules/<module>/adapters/mysql`
4. `internal/modules/<module>/http`
5. `internal/boot/business.go`
6. 行为测试

职责边界：

- `domain` 放领域对象、构造函数、领域错误和不可变业务规则。
- `usecase` 放输入输出、业务流程和 usecase-owned outbound interface。
- `adapters/<adapter>` 实现 usecase 定义的 interface。
- `http` 复用 `internal/platform/server/httpreq` 和 `internal/platform/server/httpresp`。
- `internal/boot` 负责选择 adapter、装配依赖、挂载路由。

跨模块依赖不要直接 import 对方 store 或 adapter。由消费方 usecase 定义小接口，再在 `internal/boot` 写 bridge。

## 验证

后端：

```bash
make test
make lint
make build
make verify
```

前端：

```bash
cd web
npm run lint
npm run test
npm run build
```

修改 Go 依赖后必须运行 `go mod tidy`。前端构建产物 `web/dist`、Umi 生成目录和 Utoo cache 不提交。

## 进一步阅读

- `AGENTS.md`：仓库级开发约束。
- `configs/README.md`：配置项和环境变量覆盖规则。
- `internal/boot/README.md`：composition root 和模块装配说明。
- `internal/platform/server/README.md`：HTTP runtime 边界说明。
- `web/README.md`：前端开发说明。
