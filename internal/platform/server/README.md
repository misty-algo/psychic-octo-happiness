# Server

`internal/platform/server` 只负责 Echo HTTP runtime：

- 创建 Echo 实例；
- 注册基础 middleware；
- 注册 `/api/health`、`/api/info`、`/api/ready`、`/api/capabilities`（health 和 ready 同时注册 GET 与 HEAD 形式，服务 HEAD 探活）；
- 提供业务路由入口 `API()`；
- 处理 HTTP 错误响应；
- 管理启动和优雅关闭。

`/api/info` 只返回静态配置里的 `app.name`、`app.version` 和当前时间。

它不 import 业务模块。业务路由只在 `internal/boot` 里显式组装。System First Initialization gate 通过 `WithInstallationStateReader` 注入；server 只判断当前安装是否完成并拦截未豁免路由，初始化前可达的路由名单由 boot 经 `WithPreInitRoutes` 注入，Login Session 的免认证名单经 `WithUnauthenticatedRoutes` 注入，server 不携带任何路由策略，也不知道 setup 表结构。API Token 认证通过 `WithAPIKeyVerifier` 传入一个小接口；server 只读取 `X-API-Token`、写入 request context，不知道 token 表、哈希策略或业务模块。浏览器 Login Session 认证通过 `WithLoginSessionAuthenticator` 注入；server 只读取 HttpOnly cookie 中的 opaque token、写入 request context，不知道会话表结构。内部错误记录通过 `WithSystemErrorRecorder` 注入，server 只在统一错误边界产生诊断事件，不知道审计表结构。

## API

```go
srv, err := server.New(cfg)
if err != nil {
	return err
}

if err := srv.Run(ctx); err != nil {
	return err
}
```

`server.New` 只接收静态配置值，不接收动态配置 store。业务路由不要在这里手写注册；
在 `internal/boot` 里用 `boot.NewModule`、`boot.Provide` 和 `boot.Route` 声明
adapter、usecase、handler 和 route，让 boot 持有同一个 `do` dependency graph。
