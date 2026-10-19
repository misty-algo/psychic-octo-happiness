# Middlewares Package 说明

## 概述

Middlewares 包提供 API 模板的通用中间件能力，包括请求元数据、API Token、Login Session、CSRF、错误恢复、请求日志、压缩和可选 CORS。

## 中间件列表

### 1. 请求元数据中间件 (`request_context.go`)

**功能**: 把 request id、trace id、span id 等请求元数据写入标准 `context.Context`。

**边界约定**:
- 该中间件只搬运请求元数据，不做认证授权。
- request id、trace id、span id 只接受长度受限的可见 ASCII 值；非法 request id 会重新生成，非法 trace/span id 会丢弃。
- 该中间件不读取 `X-User-ID` 这类客户端身份 header；真实用户身份必须由认证边界验证后写入 context。
- usecase 层通过 `internal/platform/requestctx` 读取元数据，不依赖 Echo context。

### 2. Login Session 中间件 (`login_session.go`)

**功能**: 浏览器登录会话认证（只读侧）

**配置**:
```go
type LoginSessionConfig struct {
    CookieName    string
    Exemptions    []RouteExemption
    Authenticator LoginSessionAuthenticator
}
```

**特性**:
- 激活语义：config 非 nil 即安装；Authenticator 或 CookieName 缺失时构造报错，不存在静默直通
- 本层不拥有 cookie 契约：cookie 名字、属性与写侧 helper 由 auth 模块声明（`authhttp.LoginSessionCookieName` 等），composition root 把名字注入 `CookieName`——读写在同一声明下
- 豁免由 composition root 注入，按精确 method + 注册路由 pattern 匹配（`MatchRouteExemption` 单点执行，业务侧 skipper 共用）；本层不携带路由策略，不支持 path-only 或通配豁免
- 从配置的 HttpOnly cookie 读取 opaque token
- 通过 boot 注入的 authenticator 校验会话，不 import 业务存储
- 验证成功后写入 `requestctx.UserID`、`requestctx.RoleID` 和 `requestctx.LoginSessionID`
- API Token 已经认证的请求会跳过浏览器会话认证
- CSRF 配置（skipper、token lookup、cookie 属性）由 auth 模块拥有（`authhttp.CSRFMiddlewareConfig`），boot 经 `WithCSRFConfig` 注入，本层只负责安装；无 Login Session 则不安装 CSRF

### 3. 错误处理中间件 (`error.go`)

**功能**: 统一的错误处理和恢复

**特性**:
- 自动 panic 恢复
- 统一的错误响应格式
- 错误日志记录默认只记录 URL path，不记录 query string
- 支持业务错误码转换

**边界约定**:
- `internal/platform/apperr` 是错误码、错误类别和对外安全消息的唯一来源，并且不依赖 HTTP。
- `ErrorHandlerWithRecorder` 是 HTTP 错误边界，负责把 Echo 错误、panic 和未知错误归一化为应用错误，并记录结构化日志；内部错误同时经注入的 recorder 持久化。
- `internal/platform/server/httpresp.APIError` 负责把 `apperr.Info` 映射为 HTTP 状态码和 JSON 响应。
- Usecase 层直接返回错误，Adapter 层包装外部系统错误，业务模块只包装业务语义错误或透传已编码错误。
- 业务错误可以返回具体、安全的 `message`；内部错误对外始终返回安全文案，原始错误和诊断上下文只进入日志 `detail`。

### 4. 中间件配置 (`config.go`)

**功能**: 统一的中间件配置管理。请求日志使用 boot 阶段安装的 zerolog logger，并把 request id、trace id、span id、user id 等字段挂到当前请求 context。

**配置**:
```go
type MiddlewareConfig struct {
    EnableRecovery       bool
    EnableRequestContext bool
    EnableLogger         bool
    EnableGzip           bool
    EnableCORS           bool
    CORS                 middleware.CORSConfig
    APIKey               *APIKeyConfig
    InstallationGate     *InstallationGateConfig
    LoginSession         *LoginSessionConfig
    CSRF                 middleware.CSRFConfig
}
```

**激活语义**:
- `Enable*` 布尔承载真实运行配置（来自静态配置）。
- API Token、安装门、Login Session 的激活只有一个事实源：config 指针非 nil——composition root 只在依赖注入时构造指针，nil 即不安装。
- CSRF 由 Login Session 派生：无 Login Session 则无 CSRF 义务，`config.LoginSession == nil` 时不安装 CSRF。
- 安装顺序是载荷性的（Gate → APIKey → LoginSession → CSRF），每一级写入下一级读取的 requestctx 事实；顺序由装配级测试锁定。
- `ApplyMiddlewares` 收到 nil config 返回错误，不提供默认值。

**使用示例**:
```go
config := &MiddlewareConfig{
    EnableRecovery:       true,
    EnableRequestContext: true,
    EnableLogger:         true,
    EnableGzip:           true,
    EnableCORS:           true,
    CORS: middleware.CORSConfig{
        AllowOrigins: []string{"https://app.example.com"},
    },
    LoginSession: &LoginSessionConfig{
        CookieName: LoginSessionCookieName,
        Exemptions: []RouteExemption{
            {Method: http.MethodGet, Path: "/api/health"},
            {Method: http.MethodGet, Path: "/api/info"},
            {Method: http.MethodGet, Path: "/api/ready"},
        },
        Authenticator: loginSessionAuthenticator,
    },
    CSRF: CSRFConfig(nil, true),
}

ApplyMiddlewares(e, config)
```

`server.New` 使用保守默认值：不启用 CORS。项目真的需要跨域时，在静态配置或环境变量里显式打开 `http.cors.enabled`，并确认允许的 origin，避免模板默认放大浏览器访问面。

## 扩展性

权限系统、租户边界、审计等业务相关中间件应由具体项目按需接入，避免 API 模板默认绑定特定授权实现。
