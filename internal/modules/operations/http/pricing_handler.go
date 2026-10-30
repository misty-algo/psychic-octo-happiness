// A05 计价规则、A06 派单策略、A09 拼车/特价的 HTTP 适配。
//
// 这三块都是「配置即线上行为」，写操作一律走审计录制：谁在什么时候把全城的
// 计价或派单改成了什么样，事后必须能查到。
package operationshttp

import (
	"strconv"
	"time"

	"github.com/labstack/echo/v5"

	"github.com/NSObjects/echo-admin/internal/modules/operations/domain"
	"github.com/NSObjects/echo-admin/internal/modules/operations/usecase"
	"github.com/NSObjects/echo-admin/internal/platform/apperr"
	"github.com/NSObjects/echo-admin/internal/platform/requestctx"
	"github.com/NSObjects/echo-admin/internal/platform/server/httpreq"
	"github.com/NSObjects/echo-admin/internal/platform/server/httpresp"
)

// ---- A05 计价规则 ----

// createPricingRuleRequest 是新建计价草稿的请求。
type createPricingRuleRequest struct {
	CityCode   string `json:"city_code" validate:"required,max=20"`
	Area       string `json:"area" validate:"omitempty,max=40"`
	CarType    string `json:"car_type" validate:"required,max=30"`
	StartFen   int64  `json:"start_fen" validate:"gte=0"`
	IncludeM   int64  `json:"include_m" validate:"gte=0"`
	PerKmFen   int64  `json:"per_km_fen" validate:"gte=0"`
	PerMinFen  int64  `json:"per_min_fen" validate:"gte=0"`
	ServiceBp  int64  `json:"service_bp" validate:"gte=0,lte=10000"`
	CommBp     int64  `json:"comm_bp" validate:"gte=0,lte=10000"`
	SurgeBp    int64  `json:"surge_bp" validate:"gt=0,lte=100000"`
	DiscountBp int64  `json:"discount_bp" validate:"gte=0,lte=10000"`
	CancelFen  int64  `json:"cancel_fen" validate:"gte=0"`
	FixedFen   int64  `json:"fixed_fen" validate:"gte=0"`
	Priority   int    `json:"priority" validate:"gte=0"`
	ValidFrom  string `json:"valid_from" validate:"required"`
	ValidTo    string `json:"valid_to" validate:"required"`
}

// pricingPreviewRequest 是样例预览请求：可以带一份待发草稿，也可以指定样例。
type pricingPreviewRequest struct {
	RuleID  string              `json:"rule_id"`
	Rule    *domain.PricingRule `json:"rule"`
	Samples []domain.FareInput  `json:"samples"`
}

// CreatePricingRule creates one draft pricing rule.
func (h *Handler) CreatePricingRule(c *echo.Context) error {
	operatorID, err := requestctx.RequireUserID(c.Request().Context())
	if err != nil {
		return err
	}
	var req createPricingRuleRequest
	if err := httpreq.BindAndValidate(c, &req); err != nil {
		return err
	}
	rule, opErr := h.pricing.CreateRule(c.Request().Context(), usecase.CreatePricingRuleInput{
		CityCode: req.CityCode, Area: req.Area, CarType: req.CarType,
		StartFen: req.StartFen, IncludeM: req.IncludeM, PerKmFen: req.PerKmFen, PerMinFen: req.PerMinFen,
		ServiceBp: req.ServiceBp, CommBp: req.CommBp, SurgeBp: req.SurgeBp, DiscountBp: req.DiscountBp,
		CancelFen: req.CancelFen, FixedFen: req.FixedFen, Priority: req.Priority,
		ValidFrom: req.ValidFrom, ValidTo: req.ValidTo,
		OperatorID: operatorID, RequestID: requestctx.GetRequestID(c.Request().Context()),
	})
	detail := "新建计价规则草稿"
	if opErr == nil {
		detail = "新建计价规则草稿：" + rule.RuleID + "（" + rule.CityCode + "/" + rule.CarType + "）"
	}
	if auditErr := h.audit.Record(c, "create_pricing_rule", "ops_pricing", req.CityCode, detail, opErr); auditErr != nil {
		return auditErr
	}
	if opErr != nil {
		return opErr
	}
	return httpresp.Created(c, rule)
}

// ListPricingRules returns one page of pricing rules.
func (h *Handler) ListPricingRules(c *echo.Context) error {
	page, pageSize, err := httpreq.Pagination(c)
	if err != nil {
		return err
	}
	items, total, err := h.pricing.ListRules(c.Request().Context(), usecase.PricingRuleFilter{
		CityCode: c.QueryParam("city_code"),
		CarType:  c.QueryParam("car_type"),
		State:    c.QueryParam("state"),
		Keyword:  c.QueryParam("keyword"),
		Skip:     (page - 1) * pageSize,
		Limit:    pageSize,
	})
	if err != nil {
		return err
	}
	return httpresp.Paginated(c, items, page, pageSize, int(total))
}

// PricingRuleDetail returns one pricing rule.
func (h *Handler) PricingRuleDetail(c *echo.Context) error {
	rule, err := h.pricing.RuleDetail(c.Request().Context(), c.Param("rule_id"))
	if err != nil {
		return err
	}
	return httpresp.OK(c, rule)
}

// PublishPricingRule publishes one draft rule. 发布会改变线上算钱结果，必须留痕。
func (h *Handler) PublishPricingRule(c *echo.Context) error {
	ruleID := c.Param("rule_id")
	rule, opErr := h.pricing.PublishRule(c.Request().Context(), ruleID)
	detail := "发布计价规则 " + ruleID
	if opErr == nil {
		detail = "发布计价规则：" + rule.RuleID + " V" + strconv.FormatInt(rule.Version, 10)
	}
	if auditErr := h.audit.Record(c, "publish_pricing_rule", "ops_pricing", ruleID, detail, opErr); auditErr != nil {
		return auditErr
	}
	if opErr != nil {
		return opErr
	}
	return httpresp.OK(c, rule)
}

// DisablePricingRule disables one published rule.
func (h *Handler) DisablePricingRule(c *echo.Context) error {
	ruleID := c.Param("rule_id")
	rule, opErr := h.pricing.DisableRule(c.Request().Context(), ruleID)
	detail := "停用计价规则 " + ruleID
	if opErr == nil {
		detail = "停用计价规则：" + rule.RuleID
	}
	if auditErr := h.audit.Record(c, "disable_pricing_rule", "ops_pricing", ruleID, detail, opErr); auditErr != nil {
		return auditErr
	}
	if opErr != nil {
		return opErr
	}
	return httpresp.OK(c, rule)
}

// PreviewPricingFare previews the fare split of samples before publishing.
func (h *Handler) PreviewPricingFare(c *echo.Context) error {
	var req pricingPreviewRequest
	if err := httpreq.BindAndValidate(c, &req); err != nil {
		return err
	}
	ruleID := req.RuleID
	if ruleID == "" {
		ruleID = c.Param("rule_id")
	}
	var rule *domain.PricingRule
	if ruleID != "" {
		stored, err := h.pricing.RuleDetail(c.Request().Context(), ruleID)
		if err != nil {
			return err
		}
		rule = &stored
	} else {
		rule = req.Rule
	}
	samples, err := h.pricing.Preview(c.Request().Context(), usecase.PricingPreviewInput{
		Rule: rule, Samples: req.Samples,
	})
	if err != nil {
		return err
	}
	return httpresp.OK(c, samples)
}

// EffectivePricingRule returns the effective rule of one city and car type.
func (h *Handler) EffectivePricingRule(c *echo.Context) error {
	at := time.Now().UTC()
	if raw := c.QueryParam("at"); raw != "" {
		parsed, err := time.Parse(time.RFC3339, raw)
		if err != nil {
			return apperr.WrapBadRequest(err, "时间格式不合法，请使用 RFC3339")
		}
		at = parsed.UTC()
	}
	rule, err := h.pricing.EffectiveRule(c.Request().Context(), c.QueryParam("city_code"), c.QueryParam("car_type"), at)
	if err != nil {
		return err
	}
	return httpresp.OK(c, rule)
}

// ---- A06 派单策略 ----

// createDispatchRequest 是新建派单策略草稿的请求。
type createDispatchRequest struct {
	IncomeBp      int  `json:"income_bp" validate:"gte=0"`
	DistanceBp    int  `json:"distance_bp" validate:"gte=0"`
	RatingBp      int  `json:"rating_bp" validate:"gte=0"`
	OnlineSecBp   int  `json:"online_sec_bp" validate:"gte=0"`
	SuccessRateBp int  `json:"success_rate_bp" validate:"gte=0"`
	RadiusM       int  `json:"radius_m" validate:"required,gt=0"`
	InviteSec     int  `json:"invite_sec" validate:"required,gt=0"`
	TotalSec      int  `json:"total_sec" validate:"required,gt=0"`
	ListenSec     int  `json:"listen_sec" validate:"required,gt=0"`
	GrabSec       int  `json:"grab_sec" validate:"required,gt=0"`
	RegionSwitch  bool `json:"region_switch"`
	ExtraWindow   int  `json:"extra_window_sec" validate:"required,gt=0"`
	MaxReturn     int  `json:"max_accepted_return" validate:"required,gt=0"`
}

// CreateDispatchStrategy creates one draft dispatch strategy.
func (h *Handler) CreateDispatchStrategy(c *echo.Context) error {
	operatorID, err := requestctx.RequireUserID(c.Request().Context())
	if err != nil {
		return err
	}
	var req createDispatchRequest
	if err := httpreq.BindAndValidate(c, &req); err != nil {
		return err
	}
	strategy, opErr := h.dispatch.CreateStrategy(c.Request().Context(), usecase.CreateDispatchInput{
		Weights: domain.DispatchWeights{
			IncomeBp: req.IncomeBp, DistanceBp: req.DistanceBp, RatingBp: req.RatingBp,
			OnlineSecBp: req.OnlineSecBp, SuccessRateBp: req.SuccessRateBp,
		},
		RadiusM: req.RadiusM, InviteSec: req.InviteSec, TotalSec: req.TotalSec,
		ListenSec: req.ListenSec, GrabSec: req.GrabSec, RegionSwitch: req.RegionSwitch,
		Redispatch: domain.DispatchRedispatch{ExtraWindowSec: req.ExtraWindow, MaxAcceptedReturn: req.MaxReturn},
		OperatorID: operatorID, RequestID: requestctx.GetRequestID(c.Request().Context()),
	})
	detail := "新建派单策略草稿"
	if opErr == nil {
		detail = "新建派单策略草稿：" + strategy.StrategyID
	}
	if auditErr := h.audit.Record(c, "create_dispatch_strategy", "ops_dispatch", "dispatch", detail, opErr); auditErr != nil {
		return auditErr
	}
	if opErr != nil {
		return opErr
	}
	return httpresp.Created(c, strategy)
}

// ListDispatchStrategies returns one page of dispatch strategies.
func (h *Handler) ListDispatchStrategies(c *echo.Context) error {
	page, pageSize, err := httpreq.Pagination(c)
	if err != nil {
		return err
	}
	items, total, err := h.dispatch.ListStrategies(c.Request().Context(), usecase.DispatchFilter{
		State: c.QueryParam("state"),
		Skip:  (page - 1) * pageSize,
		Limit: pageSize,
	})
	if err != nil {
		return err
	}
	return httpresp.Paginated(c, items, page, pageSize, int(total))
}

// PublishDispatchStrategy publishes one draft strategy.
func (h *Handler) PublishDispatchStrategy(c *echo.Context) error {
	id := c.Param("strategy_id")
	strategy, opErr := h.dispatch.PublishStrategy(c.Request().Context(), id)
	detail := "发布派单策略 " + id
	if opErr == nil {
		detail = "发布派单策略：" + strategy.StrategyID
	}
	if auditErr := h.audit.Record(c, "publish_dispatch_strategy", "ops_dispatch", id, detail, opErr); auditErr != nil {
		return auditErr
	}
	if opErr != nil {
		return opErr
	}
	return httpresp.OK(c, strategy)
}

// PreviewDispatchRanking previews how candidates are ranked under one strategy.
func (h *Handler) PreviewDispatchRanking(c *echo.Context) error {
	var req struct {
		Candidates []domain.DispatchCandidate `json:"candidates"`
	}
	// 候选可以不传：不传就用最近真实派单记录里的司机做预览。
	if err := c.Bind(&req); err != nil {
		req.Candidates = nil
	}
	ranked, err := h.dispatch.PreviewRanking(c.Request().Context(), c.Param("strategy_id"), req.Candidates)
	if err != nil {
		return err
	}
	return httpresp.OK(c, ranked)
}

// ---- A09 拼车与特价 ----

// createPoolConfigRequest 是拼车配置草稿请求。
type createPoolConfigRequest struct {
	MatchSec  int      `json:"match_sec" validate:"required,gt=0,lte=180"`
	GroupMax  int      `json:"group_max" validate:"required,gt=0,lte=2"`
	SeatMax   int      `json:"seat_max" validate:"required,gt=0,lte=4"`
	CityCodes []string `json:"city_codes"`
}

// createSpecialPlanRequest 是特价固定价方案请求。
type createSpecialPlanRequest struct {
	Name      string `json:"name" validate:"required,min=2,max=60"`
	CityCode  string `json:"city_code" validate:"required,max=20"`
	CarType   string `json:"car_type" validate:"required,max=30"`
	FixedFen  int64  `json:"fixed_fen" validate:"required,gt=0"`
	Seats     int    `json:"seats" validate:"gte=0,lte=6"`
	ValidFrom string `json:"valid_from" validate:"required"`
	ValidTo   string `json:"valid_to" validate:"required"`
}

// CreatePoolConfig creates one draft pool config.
func (h *Handler) CreatePoolConfig(c *echo.Context) error {
	operatorID, err := requestctx.RequireUserID(c.Request().Context())
	if err != nil {
		return err
	}
	var req createPoolConfigRequest
	if err := httpreq.BindAndValidate(c, &req); err != nil {
		return err
	}
	config, opErr := h.pool.CreateConfig(c.Request().Context(), usecase.CreatePoolInput{
		MatchSec: req.MatchSec, GroupMax: req.GroupMax, SeatMax: req.SeatMax, CityCodes: req.CityCodes,
		OperatorID: operatorID, RequestID: requestctx.GetRequestID(c.Request().Context()),
	})
	detail := "新建拼车配置草稿"
	if opErr == nil {
		detail = "新建拼车配置草稿：" + config.ConfigID
	}
	if auditErr := h.audit.Record(c, "create_pool_config", "ops_pool", "pool", detail, opErr); auditErr != nil {
		return auditErr
	}
	if opErr != nil {
		return opErr
	}
	return httpresp.Created(c, config)
}

// ListPoolConfigs returns one page of pool configs.
func (h *Handler) ListPoolConfigs(c *echo.Context) error {
	page, pageSize, err := httpreq.Pagination(c)
	if err != nil {
		return err
	}
	items, total, err := h.pool.ListConfigs(c.Request().Context(), usecase.PoolFilter{
		State: c.QueryParam("state"),
		Skip:  (page - 1) * pageSize,
		Limit: pageSize,
	})
	if err != nil {
		return err
	}
	return httpresp.Paginated(c, items, page, pageSize, int(total))
}

// PublishPoolConfig publishes one draft pool config.
func (h *Handler) PublishPoolConfig(c *echo.Context) error {
	id := c.Param("config_id")
	config, opErr := h.pool.PublishConfig(c.Request().Context(), id)
	detail := "发布拼车配置 " + id
	if opErr == nil {
		detail = "发布拼车配置：" + config.ConfigID
	}
	if auditErr := h.audit.Record(c, "publish_pool_config", "ops_pool", id, detail, opErr); auditErr != nil {
		return auditErr
	}
	if opErr != nil {
		return opErr
	}
	return httpresp.OK(c, config)
}

// CreateSpecialPlan creates one special price plan.
func (h *Handler) CreateSpecialPlan(c *echo.Context) error {
	operatorID, err := requestctx.RequireUserID(c.Request().Context())
	if err != nil {
		return err
	}
	var req createSpecialPlanRequest
	if err := httpreq.BindAndValidate(c, &req); err != nil {
		return err
	}
	plan, opErr := h.pool.CreatePlan(c.Request().Context(), usecase.CreatePlanInput{
		Name: req.Name, CityCode: req.CityCode, CarType: req.CarType,
		FixedFen: req.FixedFen, Seats: req.Seats, ValidFrom: req.ValidFrom, ValidTo: req.ValidTo,
		OperatorID: operatorID, RequestID: requestctx.GetRequestID(c.Request().Context()),
	})
	detail := "新建特价方案"
	if opErr == nil {
		detail = "新建特价方案：" + plan.PlanID + "（" + plan.Name + "）"
	}
	if auditErr := h.audit.Record(c, "create_special_plan", "ops_pool", req.CityCode, detail, opErr); auditErr != nil {
		return auditErr
	}
	if opErr != nil {
		return opErr
	}
	return httpresp.Created(c, plan)
}

// ListSpecialPlans returns one page of special price plans.
func (h *Handler) ListSpecialPlans(c *echo.Context) error {
	page, pageSize, err := httpreq.Pagination(c)
	if err != nil {
		return err
	}
	items, total, err := h.pool.ListPlans(c.Request().Context(), usecase.PoolFilter{
		State: c.QueryParam("state"),
		Skip:  (page - 1) * pageSize,
		Limit: pageSize,
	})
	if err != nil {
		return err
	}
	return httpresp.Paginated(c, items, page, pageSize, int(total))
}

// PublishSpecialPlan publishes one special price plan.
func (h *Handler) PublishSpecialPlan(c *echo.Context) error {
	id := c.Param("plan_id")
	plan, opErr := h.pool.PublishPlan(c.Request().Context(), id)
	detail := "发布特价方案 " + id
	if opErr == nil {
		detail = "发布特价方案：" + plan.PlanID
	}
	if auditErr := h.audit.Record(c, "publish_special_plan", "ops_pool", id, detail, opErr); auditErr != nil {
		return auditErr
	}
	if opErr != nil {
		return opErr
	}
	return httpresp.OK(c, plan)
}

// PoolStatsRead returns the carpool metrics: match rate, shared rate, unmatched.
func (h *Handler) PoolStatsRead(c *echo.Context) error {
	stats, err := h.pool.Stats(c.Request().Context())
	if err != nil {
		return err
	}
	return httpresp.OK(c, stats)
}
