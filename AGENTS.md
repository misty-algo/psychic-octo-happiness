## 项目方向

这是一个 module-first 的 Go API 模板，不是 generator-first、OpenAPI-first、layer-first 模板。

统一使用这些架构词：

- `business module`：业务模块，位于 `internal/modules/<module>`。
- `platform`：平台运行时代码，位于 `internal/platform`。
- `capability`：可复用基础设施能力，例如 MySQL、Redis、MongoDB、tracing、logging。
- `composition root`：启动装配层，位于 `internal/boot`。

不要把本项目改回 `service/biz/data` 分层心智。不要新增生成器主导的 API 开发路径。

## 修改前必须查看

修改代码前优先查看：

- `go.mod`
- `README.md`
- `Makefile`
- `.golangci.yml`
- 当前修改点附近代码
- 当前修改点附近测试

涉及启动、配置、HTTP runtime 或业务模块装配时，还必须查看：

- `internal/boot/README.md`
- `internal/platform/server/README.md`
- `configs/README.md`

## 业务模块约束

业务代码只放在 `internal/modules/<module>`，标准结构如下：

- `domain`：领域对象、构造函数、领域错误和不可变业务规则。
- `usecase`：用例输入输出、业务流程、usecase-owned outbound interface。
- `adapters/<adapter>`：具体外部系统或本地实现，例如 `memory`、`mysql`。
- `http`：Echo handler 和 `Register(group, handler)`。

新增业务模块时按这个顺序写：

1. `domain`
2. `usecase`
3. `adapters/memory`
4. `http`
5. `internal/boot/business.go` 装配
6. 行为测试

不要在 handler 中写核心业务逻辑。不要让 domain 依赖 Echo、DB、SDK、transport 类型或 platform runtime。

## 依赖和装配

`internal/boot` 是 composition root。它可以 import business adapters、platform infrastructure、server 和 configs。业务模块不要反向 import boot。

业务路由只能在 `internal/boot` 中通过 `NewModule`、`Provide`、`Route` 显式装配。不要在 `internal/platform/server` 中手写业务路由。

跨模块依赖禁止直接 import 对方 store 或 adapter。由消费方 usecase 定义小 interface，再在 `internal/boot` 写 adapter bridge。lookup interface 要表达消费方真正需要的语义，不要把重要状态压成裸 `bool`。

通用基础设施必须放在 `internal/platform/infrastructure` 或明确的 platform package。不要把通用能力命名成某个业务域，例如不要新增类似 `userstorage` 的平台抽象。

## interface 规则

不要机械创建 interface。

允许新增 interface 的情况：

- 使用方需要定义依赖边界。
- 有多个真实 adapter。
- 测试需要替换 DB、网络、文件系统、时钟、ID 生成器等不可控依赖。
- 需要隔离跨模块 lookup。

禁止：

- 每个 struct 自动配 interface。
- 为了 mock 而 mock。
- 为了未来可能有多个实现提前抽象。
- 在实现方旁边定义没有使用方语义的 interface。
- 创建巨大 interface。

interface 应优先定义在使用方。

## HTTP API 规则

HTTP adapter 只做：

- path/query/body 解析；
- validator 校验；
- request DTO 到 usecase input 的转换；
- 调用 usecase，并透传 `c.Request().Context()`；
- 用 `httpresp` 输出统一响应。

请求解析优先复用 `internal/platform/server/httpreq`。响应优先复用 `internal/platform/server/httpresp`。错误语义优先用 `internal/platform/apperr`。

新增 API 时必须补 HTTP adapter 行为测试。测试应覆盖至少一个成功路径和一个关键失败路径，不要只测 happy path。

## Store 和外部资源

usecase 定义自己的 store interface。adapter 实现 usecase 的 interface。

默认开发路径应有 `adapters/memory`，保证本地和测试开箱可跑。真实存储 adapter 复用 boot 已经加载的基础设施资源，例如 `*gorm.DB`。

外部 I/O、DB、RPC、长任务必须接收 `context.Context`，且 context 必须作为第一个参数。不要把 context 存进 struct。

## 错误处理

所有 error 必须处理。跨层返回错误时加上下文，并用 `%w` 保留原始错误。

不要同一层又 log 又 return 同一个 error。不要依赖完整错误字符串做业务判断。需要语义判断时使用 `errors.Is` / `errors.As` 或 `apperr.Parse`。

错误响应不要暴露 secret、token、密码、SQL 细节或内部实现。

## 并发和资源

新增 goroutine 前必须明确：

- 谁拥有它；
- 什么时候退出；
- 如何取消；
- 错误如何处理；
- 是否有数据竞争；
- 如何测试。

每个 goroutine 必须有退出路径。后台任务必须受 context 控制。共享可变状态必须有清晰所有权或锁保护。

文件、HTTP response body、database rows、transaction、stream、ticker、timer、external process 都必须正确关闭。

## 新项目规则

这是模板项目，优先清理坏设计，不要兼容坏设计。

除非用户明确要求，否则禁止：

- 保留旧 API 和新 API 双轨。
- 写 deprecated wrapper。
- 写 legacy / compat / fallback 分支。
- 同时支持旧配置和新配置。
- 同时支持旧字段和新字段。
- 为旧错误行为保留兼容逻辑。
- 为临时迁移写长期 adapter。
- 为未来不确定需求提前抽象。

发现坏设计时，优先删除、统一、重命名或重构，并同步更新调用方、测试和文档。

## 注释

导出的类型、函数、方法、常量、变量必须有注释。

以下场景必须写注释说明原因或约束：

- 非显然业务规则；
- 状态流转；
- 幂等逻辑；
- 金额、时间、精度、时区规则；
- goroutine、channel、mutex、worker、ticker；
- 安全相关逻辑；
- 数据库事务和一致性逻辑；
- 第三方 API 特殊行为。

不要写只复述代码的废话注释。

## 测试

行为变化必须有测试。bug 修复必须补测试。

优先表驱动测试，使用 `t.Run` 写清 case 名。测试失败信息必须包含 got 和 want。测试 helper 必须调用 `t.Helper()`。

优先 fake、in-memory 实现和真实小组件。不要为了测试引入复杂 mock 框架。

重点覆盖：

- 空输入、nil 输入、零值；
- 重复输入、非法输入、边界值；
- context cancellation、超时；
- 外部依赖失败；
- 事务回滚；
- 幂等；
- 并发改动的数据竞争风险。

## 文档

改动影响使用方式时，必须更新相关文档：

- `README.md`
- `configs/README.md`
- `internal/boot/README.md`
- `internal/platform/server/README.md`
- `Makefile`
- Compose、Docker、K8s 示例

文档必须描述当前事实，不要保留退休设计或历史兼容噪音。

## 验证

优先使用项目已有命令：

```sh
make test
make lint
make build
make verify
```

并发相关改动额外运行：

```sh
go test -race ./...
```

如果命令无法运行或已有基线失败，最终回复必须说明具体命令、失败文件和失败原因。不要把失败说成通过。

修改依赖后必须运行：

```sh
go mod tidy
```

完成前必须确认 `git diff --check` 通过，且没有无关生成物或构建产物留在工作区。

## 禁止事项

除非用户明确要求，否则禁止：

- 新增不必要依赖。
- 新增无必要 interface。
- 新增 `utils` / `helpers` / `common` 包。
- 新增全局 service locator。
- 新增 DI container。
- 新增无退出路径 goroutine。
- 新增反射 mapper。
- 新增不必要 generics。
- 引入大型框架。
- 引入 ORM 之外的新持久化框架。
- 忽略 error。
- 在业务逻辑中 panic。
- 在库代码中 `os.Exit`。
- 在测试中裸 `time.Sleep`。
- 在 handler 中写核心业务逻辑。
- 在 domain 中依赖 transport、DB 或 SDK 类型。
- 留下“以后再清理”的临时代码。

## 完成前自检

最终回复前检查：

- 是否有无关修改；
- 是否有不必要依赖；
- 是否有不必要 interface；
- 是否有兼容胶水；
- 是否有 ignored error；
- 是否有 context 丢失；
- 是否有 goroutine 泄漏；
- 是否有 data race 风险；
- 是否有资源未关闭；
- 是否有测试缺失；
- 是否需要补注释；
- 是否需要更新文档。

## Agent skills

### Issue tracker

Issue 和 PRD 记录在 GitHub Issues 中；仅 Issue 进入 triage，外部 PR 不作为需求入口。详见 `docs/agents/issue-tracker.md`。

### Triage labels

Triage 使用默认标签：`needs-triage`、`needs-info`、`ready-for-agent`、`ready-for-human`、`wontfix`。详见 `docs/agents/triage-labels.md`。

### Domain docs

本仓库使用 single-context domain-doc 布局：根目录使用 `CONTEXT.md`，架构决策位于 `docs/adr/`。详见 `docs/agents/domain.md`。

## UI 设计规则

* 整体风格保持现代 SaaS：简洁、克制、清晰、有留白。
* 优先保证信息层级和操作效率，不为了“丰富”增加 UI。
* 页面结构清晰：标题、描述、主要操作、内容各司其职。
* 页面标题简洁明确，不添加无意义的营销文案。
* 减少 Card 使用，没有明确分组需求时不要使用 Card。
* 禁止无意义的 Card 套 Card。
* Table 只展示必要信息，操作列保持简洁。
* 高频操作直接展示，低频操作收进更多菜单。
* Status 保持简单统一，避免滥用 Tag 和颜色。
* 一个区域通常只保留一个 Primary Action，其他操作降低视觉权重。
* 控制颜色数量，颜色主要用于操作、状态和反馈。
* 禁止大面积渐变、强阴影、过度圆角和装饰性背景。
* 禁止为了视觉效果大量添加 Icon、Tag、动画或装饰性文案。
* 保持充足留白，避免页面内容拥挤。
* 表单保持紧凑，不要为了填满页面而拉伸输入框。
* 新页面优先复用已有页面的布局、组件和视觉模式。
* 不要为了单个页面创造新的颜色、间距、圆角或视觉风格。
* 修改页面遵循最小改动原则，不要无关地重新设计其他区域。
* 页面完成后检查整体一致性，避免出现明显的“组件堆砌感”。
* 最终目标是现代、统一、耐看的 SaaS 中后台，而不是追求视觉效果本身。
