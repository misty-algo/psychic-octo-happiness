# 多因子计价规则与地图天气展示设计

- 日期：2026-11-06
- 状态：已定稿，待实施
- 涉及仓库：
  - 管理端 `psychic-octo-happiness`（`main`，Go Echo + React/Umi）
  - 乘客/司机端 `bug-free-octo-potato`（`dev`，Go 多服务 + 原生 JS + Leaflet）
- 输入文档：
  - `计价规则表_按城市分表_v2.xlsx`（17 sheet：目录 + 16 城，每城 4 车型 × 6 时段 × 5 日历）
  - `计价规则_MySQL_完整.sql`（`ride_fare` 库 11 张表，16 城 / 64 规则包 / 768 时段价 / 17 因子 / 272 城市取值）
  - `计价查询示例.sql`（一次计价的完整口径与对账示例）

## 1. 目标

按 Excel/SQL 文档实现计价，价格由**天气、节假日、车型、高峰期、低峰期、行程拥挤、城市**共同决定；接入天气预报并在乘客端与司机端地图展示当前城市天气。先在**本地克隆的数据**上开发与测试，不改动仓库里的数据库连接配置。

## 2. 现状诊断（为什么必须改架构）

1. 乘客端真实估价走 `bug-free-octo-potato/internal/passenger/service/order.go:400-433` 的 `builtinPricing.Quote`，金额是代码常量（起步 1300 分、每公里 260 分、时长 = 距离/8），`cmd/passenger/main.go:102-127` 未调用 `WithPricingProvider`（定义在 `internal/passenger/service/auth.go:196-202`），因此改文档或改数据库都不会改变价格。
2. 管理端 A05 计价规则（`web/src/pages/OperationsPricing.tsx` → `internal/modules/operations/adapters/mongo/pricing_store.go` 的 `pricing_rules` 集合，camelCase）只做版本/分账预览，`readEffectivePricingRule` 全仓仅被前端告警条调用，**未接到乘客端估价**。
3. `bug-free-octo-potato/internal/shop/model/mongo_pricing.go:8-46` 存在同名 `pricing_rules` 集合但用 snake_case，与管理端**同名集合两套 schema**。
4. 第三套规则：`internal/gateway/handler/spec_contract_handler.go:664-675` 内存预置规则与试算公式，同样不落库、不接估价。
5. 天气/节假日代码两仓均无实现；仅 `internal/shop/model/mongo_pricing.go:117-150` 的 `surge_rules.WeatherTrigger` 字段预留，全仓无读取方。
6. 可复用资产：`internal/passenger/repo/poi_nominatim.go:34-50` 提供第三方 HTTP 客户端范式（超时、缓存、备用源、降级）；两端 Leaflet 地图分别由 `web/public/assets/passenger.js:1550` 与 `web/public/assets/driver.js:1014` 初始化，`driver.js:167-172` 的 `.sheet`/`renderOverlay()` 与自定义 `L.Control` 是天气浮层的落点。

## 3. 已确认决策

| 决策点 | 结论 |
| --- | --- |
| 唯一规则源 | **MySQL `ride_fare` 11 张表**，Excel/SQL 即事实来源，不维护第二套 |
| 天气数据源 | **Open-Meteo（免 key）+ 本地 Mock 可切换**，配置项 `weather.provider`，默认 mock |
| 节假日/供需 | **接实时数据**：第三方节假日日历 API + 实时空车/订单数计算供需比 |
| 管理端 A05 | **要能编辑**规则包与因子，处理版本与生效区间 |
| 配置切换 | **不改仓库配置文件**；本地测试通过启动环境变量覆盖指向 `127.0.0.1:3306` / `27018` |
| 地图天气 | **乘客端与司机端都要** |

## 4. 架构与数据流

```
乘客下单 / 游客看价
  → gateway passenger_handler.Estimate / QuotePrice
  → passenger service Estimate / QuotePrice
  → PricingProvider.Quote(QuoteInput)
  → ride_fare 规则仓储（MySQL）
       ① city + class → ACTIVE fare_rule_pack（version / included / K 上下限 / 封顶）
       ② calendar_code → calendar_type.uses_price_band → 选 WEEKDAY/WEEKEND 价表
       ③ start_min → time_band（PEAK/OFFPEAK/NIGHT/FLAT）→ fare_time_price
       ④ F_base = 起步 + 超额里程 + 超额时长 + 远途阶梯
       ⑤ 动态因子：组内取 |K−1| 最大、组间叠加 → clamp [k_min_bp, k_max_bp]
       ⑥ surge 封顶 + 节假日固定服务费 + 固定费 − 优惠 + 代收费 = total
  → fare_order_snapshot（一单一行，hit_factors_json 记录本次命中因子）
  → EstimateDTO 返回前端
```

并行数据供给（均通过 Provider 接口 + mock 实现，超时/缓存/失败降级）：

- `HolidayCalendarProvider` → `calendar_code`
- `WeatherProvider` → 天气码 → W0–W4 档位
- `SupplySnapshotProvider` → 供需比 R′ → `SUPPLY_R06…R30` 档位
- `CongestionProvider` → 实际时长 / 自由流时长 → `CONGESTION_1/2/3`（按文档保持 `NOOP`，只写入 `hit_factors_json`）

## 5. 组件设计

### 5.1 计价引擎（`bug-free-octo-potato`）

- 扩展 `PricingProvider.Quote`：现有签名只有 `(ctx, carType, 起点, 终点, couponID)`，无城市/时刻/天气/日历/供需入参。改为 `Quote(ctx, QuoteInput)`，`QuoteInput{CityCode, CarClass, StartAt, DistanceM, DurationS, CalendarCode, WeatherLevel, SupplyRatioBP, CongestionLevel, CouponID}`，同步修改两个 handler、gRPC proto 与前端请求。
- 车型映射：现有 `carType int`（`CarTypeCarpool/Premium`）映射到 `EXPRESS/COMFORT/PREMIUM/LUXURY`。
- 新增 `ride_fare` MySQL 适配器（复用项目已有 MySQL 依赖与 `configs/config.yaml` 的 DSN 结构），实现规则查询；金额一律整数分、系数万分比、里程米、时长秒、时段半开区间 `[start_min, end_min)`，禁止用 UTC 判日（用城市时区 `Asia/Shanghai`）。
- 因子裁决：`mutual_group` 组内取 `|k_bp − 10000|` 最大者，组间加法叠加，`clamp` 到规则包的 `[k_min_bp, k_max_bp]`，动态加价金额受 `surge_cap_cent` 封顶；`W4` 因 `need_manual_review=1` 只记录不自动拉满。
- 下线 `builtinPricing` 与 `spec_contract_handler.go` 内存规则，避免双轨；`pricing_rules` Mongo 集合（两套 schema）退役。
- 参考对账场景：`BJ / EXPRESS / WEEKDAY / 08:15 / 10 km / 38 min` → 起步 1650 + 里程 1757 + 时长 1768 + 远途 0 = **5175 分 = 51.75 元**。

### 5.2 实时 Provider

- 节假日：`holiday.api_base` 可配置，默认调用第三方日历 API，24 小时缓存；失败降级到内置 2026 年兜底表。返回 `WEEKDAY/WEEKEND/HOLIDAY/HOLIDAY_EVE/MAKEUP_WORKDAY`。
- 供需：本地实测 `driver_profiles`（`status`、`city_code`）与 `passenger_orders`（`state=PENDING`）可用，`dispatch_queues` 为空。R′ = 近 N 分钟待派订单数 ÷ 可接单司机数（`status=idle`），映射 `SUPPLY_R06/R08/R10/R12/R16/R25/R30`；司机数为 0 时取封顶档并告警。
- **城市码映射缺口**：`ride_fare.city_code` 为 `BJ/SH` 字母码，业务侧 `city_code` 为 `510100` 行政区划码。需在 `ride_fare.city` 增加 `adcode` 并建立映射，否则实时统计无法匹配城市。
- 拥堵：`实际时长 ÷ 自由流时长`，自由流时长由 `distanceM / 基准速度` 得出；按文档 `CONGESTION_*` 为 `NOOP`，仅落快照供展示与复算。
- 所有 Provider 有超时、缓存、失败降级；任一失败退回默认档，不阻塞估价。

### 5.3 天气客户端

- `weather.Provider` 接口：`Current(ctx, lat, lng, city) (Weather, error)`。
- `OpenMeteoProvider`：`api.open-meteo.com/v1/forecast` 的 current 字段（温度、天气码、风速、降水），3 秒超时、10 分钟缓存、失败降级；无需 API key。
- `MockProvider`：本地固定天气，默认启用，便于离线测试。
- 天气码映射：W0 晴 / W1 多云阴 / W2 小雨雾 / W3 中雨雪大风 / W4 暴雨暴雪极端。

### 5.4 地图天气卡（两端）

- 新增 `GET /api/v1/weather/current?city=&lat=&lng=`，后端缓存，前端 10 分钟刷新；失败显示"天气暂不可用"，不阻塞叫车。
- 乘客端 `passenger.js`、司机端 `driver.js` 各加一个 `L.Control` 天气卡：城市、图标、温度、天气描述、风力、更新时间。沿用现有配色与控件风格，不新增视觉体系。

### 5.5 管理端 A05 可编辑

- 复用现有 `draft → published → disabled` 状态机与"3 个分账样例资金守恒"校验（`internal/modules/operations/usecase/pricing.go`）。
- 新增 `UpdatePricingRule`（**仅草稿可改**）、版本递增（改已生效规则生成新版本而非覆盖）、生效区间与重叠校验、时段价与因子档位编辑。
- 现有 store 是 Mongo `pricing_rules`（`pricing_store.go:125-249` 只有 Create/Publish/Disable），需新增指向 MySQL `ride_fare` 的适配器，编辑 `fare_rule_pack` / `fare_time_price` / `fare_distance_tier` / `holiday_service_fee` / `surge_factor_rule`。
- 前端 `OperationsPricing.tsx` 增加编辑表单与「时段价 / 动态因子」页签；`pricing_rules` Mongo 集合退役。

## 6. 配置与本地测试

- 仓库内 `configs/config.toml`、`configs/config.yaml` **保持原样**，新增 `weather.*`、`holiday.*`、`supply.*` 配置键但默认值不改变现有行为。
- 本地测试通过启动时环境变量覆盖（`MONGODB_URI` / `MONGODB_DATABASE`、`ECHO_ADMIN_MYSQL_*` 等）指向 `127.0.0.1:3306` 与 `127.0.0.1:27018`，使用 `桌面\online-data-backup-2026-11-06` 克隆的数据；`local-pre-import/` 是导入前的本地回退快照。
- 不连接线上数据库进行开发或测试。

## 7. 错误处理与测试

- 所有金额用整数分，禁止浮点参与计费；时区用城市时区判日，禁止 UTC。
- Provider 超时/异常 → 降级默认档 + 结构化日志 + `hit_factors_json` 标注降级来源，绝不把错误透传成下单失败。
- 表驱动测试覆盖：空/零输入、边界时段（`00:00`、`23:59`）、跨夜时段、节假日与调休映射、K 上下限钳制、加价封顶、司机数为 0 的供需、天气 W4 人工审核、优惠券叠加、历史版本复算。
- 必测对账用例：`BJ/EXPRESS/WEEKDAY/08:15/10km/38min = 5175 分`；以及 16 城 × 4 车型 × 6 时段的规则包查询不为空。
- 新增 HTTP adapter 行为测试（成功路径 + 关键失败路径），SQL 示例作为验收基准。

## 8. 风险与未决项

- 第三方节假日日历 API 的具体供应商未指定，实现为**可配置 URL + 本地兜底**，不绑定厂商；上线前需确定供应商与可用性 SLA。
- 供需统计依赖司机心跳状态实时性；当前本地快照里司机状态只有 `busy/offline`，无 `idle`，测试时需用 mock 构造在线司机。
- `ride_fare.city_code` 与业务行政区划码映射需要补齐 16 城 `adcode`。
- 管理端从 Mongo 迁到 MySQL 时，`pricing_rules` 集合退役需同步清理项目 B 的 snake_case 模型与引用，避免遗留双轨。
- `fare_order_snapshot.order_id` 为 BIGINT，与现有订单号（字符串如 `HXZ...`）不一致，接入时需确定映射规则。

## 9. 实施顺序

1. 补齐 `ride_fare` 元数据：`city.adcode`、`holiday_calendar` 表与 2026 节假日数据。
2. 项目 B：天气包（Provider + OpenMeteo + Mock + 配置）与 `/weather/current` 接口。
3. 项目 B：`ride_fare` MySQL 适配器与计价引擎（含 `QuoteInput` 扩展、因子裁决、快照）。
4. 项目 B：实时节假日/供需/拥堵 Provider 与降级。
5. 两端地图天气卡 + 联调验证。
6. 管理端：MySQL 规则适配器、编辑 API、A05 前端编辑与因子页签。
7. 全量测试（Go + 前端）、16 城计抽核对账、本地端到端验证。
