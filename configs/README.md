# 配置说明

项目默认只加载一个静态配置文件：

```go
cfg, err := configs.Load("configs/config.toml")
```

配置文件支持 TOML、YAML、JSON，格式由文件后缀识别。无后缀时按 TOML 解析，未知后缀会在启动时失败。未知配置字段也会在启动时失败。`configs.Load` 是规范化与校验的唯一入口：返回的 Config 即最终结果，server、logging 和各 infrastructure 资源直接信任它，不再重复校验。

环境变量可覆盖除 MySQL 拓扑外的全部配置项。覆盖键集由 `internal/platform/configs/config.go` 中 Config 各字段的 `mapstructure` tag 自动推导，没有需要手工登记的键表；新增配置字段即自动获得环境变量覆盖能力。MySQL 的 `host`、`port`、`database`、`username` 刻意保持仅限配置文件（见 `decode.go` 的 `envExcluded`），只有密码走环境变量。

## 当前配置项

```toml
[app]
name = "echo-admin"
version = "dev"

[system]
port = ":9322"
level = 1 # 1=debug, 2=online

[log]
format = "console" # console, json
output = "stdout" # stdout, stderr
caller = false

[http]
enable_recovery = true
enable_request_context = true
enable_request_log = true
enable_gzip = true
secure_cookies = false

[http.cors]
enabled = false
allow_origins = []
allow_methods = []
allow_headers = []
allow_credentials = false
expose_headers = []
max_age_seconds = 0

[admin]
upload_dir = "uploads"

[mysql]
enabled = true
host = "127.0.0.1"
port = 3306
database = "echo_admin"
username = "echo_admin"
password = ""
max_open_conns = 25
max_idle_conns = 5
conn_max_lifetime_seconds = 300
ping_timeout_seconds = 3

[redis]
enabled = false
address = ""

[mongodb]
enabled = false
uri = ""
database = ""

[tracing]
enabled = false
endpoint = ""
protocol = "grpc" # grpc, http
insecure = true
```

HTTP middleware、MySQL、Redis、MongoDB、Jaeger tracing 和 admin upload directory 都是 configured resources。后台基础能力运行期依赖 MySQL；启用 Redis、MongoDB、tracing 等可选外部依赖后也必须提供连接配置，并会进入 `/api/ready` 和 `/api/capabilities`。

业务代码放在 `internal/modules/<module>`，平台运行时代码放在 `internal/platform`。业务 adapter 复用 boot 已经加载的资源：usecase 定义 usecase-owned outbound interface，adapter 使用配置好的 MySQL/Redis/MongoDB client 实现它，然后业务 module 用 `boot.NewModule`、`boot.Provide` 和 `boot.Route` 声明 adapter、usecase、handler 和 route。

## 环境变量覆盖

配置文件是主配置入口。数据库 host、port、database、username、连接池等非敏感拓扑配置应写在配置文件中；MySQL 密码可以用环境变量覆盖，便于接入 Docker Compose、Kubernetes Secret 或 CI/CD secret。`env.example` 是面向运维的示例文档（混有 Docker Compose 的 `.env` 替换变量），不与推导键集一一对应；覆盖键的真理性来源是 Config 的 `mapstructure` tag。首次管理员密码不在静态配置中维护，由浏览器 `/setup` 初始化页提交。

```bash
export ECHO_ADMIN_APP_NAME=echo-admin
export ECHO_ADMIN_APP_VERSION=dev
export ECHO_ADMIN_SYSTEM_PORT=:9322
export ECHO_ADMIN_SYSTEM_LEVEL=1
export ECHO_ADMIN_LOG_FORMAT=console
export ECHO_ADMIN_LOG_OUTPUT=stdout
export ECHO_ADMIN_LOG_CALLER=false
export ECHO_ADMIN_HTTP_ENABLE_RECOVERY=true
export ECHO_ADMIN_HTTP_ENABLE_REQUEST_CONTEXT=true
export ECHO_ADMIN_HTTP_ENABLE_REQUEST_LOG=true
export ECHO_ADMIN_HTTP_ENABLE_GZIP=true
export ECHO_ADMIN_HTTP_SECURE_COOKIES=false
export ECHO_ADMIN_HTTP_CORS_ENABLED=false
export ECHO_ADMIN_HTTP_CORS_ALLOW_ORIGINS=
export ECHO_ADMIN_HTTP_CORS_ALLOW_METHODS=
export ECHO_ADMIN_HTTP_CORS_ALLOW_HEADERS=
export ECHO_ADMIN_HTTP_CORS_ALLOW_CREDENTIALS=false
export ECHO_ADMIN_HTTP_CORS_EXPOSE_HEADERS=
export ECHO_ADMIN_HTTP_CORS_MAX_AGE_SECONDS=0
export ECHO_ADMIN_ADMIN_UPLOAD_DIR=uploads
export ECHO_ADMIN_MYSQL_PASSWORD=replace-with-a-private-database-password
export ECHO_ADMIN_REDIS_ENABLED=false
export ECHO_ADMIN_REDIS_ADDRESS=
export ECHO_ADMIN_MONGODB_ENABLED=false
export ECHO_ADMIN_MONGODB_URI=
export ECHO_ADMIN_TRACING_ENABLED=false
export ECHO_ADMIN_TRACING_ENDPOINT=
export ECHO_ADMIN_TRACING_PROTOCOL=grpc
export ECHO_ADMIN_TRACING_INSECURE=true
```

浏览器后台登录使用服务端 Login Session 和 HttpOnly cookie，不需要 JWT secret。生产 HTTPS 部署应设置 `ECHO_ADMIN_HTTP_SECURE_COOKIES=true`，让 Login Session cookie 和 CSRF cookie 都带 `Secure` 属性。空库首次启动后访问浏览器后台会进入 `/setup`，由初始化页创建拥有最高权限 Root Role 的首个管理员。

`app.name` 和 `app.version` 会出现在 `/api/info` 响应中。`system.level` 只接受 `1` 或 `2`。`log.format` 只接受 `console` 或 `json`；`log.output` 只接受 `stdout` 或 `stderr`。`http.enable_*` 控制 HTTP middleware 安装（省略时默认开启），`http.secure_cookies` 控制浏览器登录会话相关 cookie 是否只允许 HTTPS 传输。

`http.cors.enabled=true` 时必须配置 `http.cors.allow_origins`。`http.cors.allow_credentials=true` 时 `allow_origins` 不能包含 `*`。环境变量中的列表项使用逗号分隔，例如 `ECHO_ADMIN_HTTP_CORS_ALLOW_ORIGINS=https://app.example.com,https://admin.example.com`。

`/api/health` 是进程存活，`/api/ready` 只返回整体 readiness，`/api/capabilities` 返回 logging、MySQL、Redis、MongoDB、Jaeger tracing 的 disabled/available/unavailable 状态。业务模块装配会直接使用 MySQL，`mysql.enabled=false` 只适合平台层单元测试，不适合默认业务运行。

`docker-compose.yaml` 只用于本地开发，公开端口默认绑定 `127.0.0.1`。默认启动 API 和 MySQL；Redis、MongoDB 和 Jaeger 在 `resources` profile 下，使用 `docker compose --profile resources up` 启动。资源服务使用 Compose 命名卷保存本地开发数据；需要重置 MySQL、Redis 和 MongoDB 时，显式运行 `docker compose down -v`。
