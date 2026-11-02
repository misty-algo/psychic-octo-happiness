// Package operationshttp adapts operations user-management HTTP requests to
// the operations usecase. Every mutation and every full-phone read goes
// through the shared operation recorder so audit semantics stay uniform.
package operationshttp

import (
	"github.com/labstack/echo/v5"

	"github.com/NSObjects/echo-admin/internal/modules/audit/oprec"
	"github.com/NSObjects/echo-admin/internal/modules/operations/usecase"
	"github.com/NSObjects/echo-admin/internal/platform/requestctx"
	"github.com/NSObjects/echo-admin/internal/platform/server/httpreq"
	"github.com/NSObjects/echo-admin/internal/platform/server/httpresp"
)

// Handler adapts operations requests to the module usecases.
type Handler struct {
	usecase     *usecase.Usecase
	audits      *usecase.AuditUsecase
	orders      *usecase.OrderUsecase
	tickets     *usecase.TicketUsecase
	marketing   *usecase.MarketingUsecase
	pricing     *usecase.PricingUsecase
	dispatch    *usecase.DispatchUsecase
	pool        *usecase.PoolUsecase
	withdrawals *usecase.WithdrawalActionUsecase
	audit       *oprec.Recorder
}

// New creates an operations HTTP handler.
func New(uc *usecase.Usecase, audits *usecase.AuditUsecase, orders *usecase.OrderUsecase,
	tickets *usecase.TicketUsecase, marketing *usecase.MarketingUsecase,
	pricing *usecase.PricingUsecase, dispatch *usecase.DispatchUsecase,
	pool *usecase.PoolUsecase, withdrawals *usecase.WithdrawalActionUsecase, audit *oprec.Recorder,
) *Handler {
	return &Handler{
		usecase:     uc,
		audits:      audits,
		orders:      orders,
		tickets:     tickets,
		marketing:   marketing,
		pricing:     pricing,
		dispatch:    dispatch,
		pool:        pool,
		withdrawals: withdrawals,
		audit:       audit,
	}
}

// Register mounts operations routes on group.
func Register(group *echo.Group, handler *Handler) {
	group.GET("/ops/users", handler.ListUsers)
	group.GET("/ops/users/:id", handler.UserDetail)
	group.POST("/ops/users/:id/restrict", handler.RestrictUser)
	group.POST("/ops/users/:id/unrestrict", handler.UnrestrictUser)

	group.GET("/ops/driver-audits", handler.ListDriverAudits)
	group.GET("/ops/driver-audits/:application_id", handler.DriverAuditHistory)
	group.POST("/ops/driver-audits/:application_id/revisions/:revision/claim", handler.ClaimDriverAudit)
	group.POST("/ops/driver-audits/:application_id/revisions/:revision/decide", handler.DecideDriverAudit)

	group.GET("/ops/orders", handler.ListOrders)
	group.GET("/ops/orders/:order_no", handler.OrderDetail)
	group.POST("/ops/orders/:order_no/commands", handler.ExecuteOrderCommand)

	// 提现审批：读共享库 withdraw_requests，审批动作走打车网关由司机域记账。
	group.GET("/ops/withdrawals", handler.ListWithdrawals)
	group.POST("/ops/withdrawals/:id/approve", handler.ApproveWithdrawal)
	group.POST("/ops/withdrawals/:id/reject", handler.RejectWithdrawal)
	group.POST("/ops/withdrawals/:id/pay", handler.PayWithdrawal)

	group.GET("/ops/tickets", handler.ListTickets)
	group.GET("/ops/tickets/:ticket_no", handler.TicketDetail)
	group.POST("/ops/tickets/:ticket_no/commands", handler.ExecuteTicketCommand)
	group.POST("/ops/tickets/:ticket_no/appeal", handler.AppealTicket)

	// 营销中心：券模板、券批次、发放、活动与对账导出。
	group.GET("/ops/marketing/coupon-templates", handler.ListCouponTemplates)
	group.GET("/ops/marketing/coupon-templates/:template_no", handler.CouponTemplateDetail)
	group.POST("/ops/marketing/coupon-templates", handler.CreateCouponTemplate)
	group.POST("/ops/marketing/coupon-templates/:template_no/commands", handler.ExecuteTemplateCommand)

	group.GET("/ops/marketing/coupon-batches", handler.ListCouponBatches)
	group.POST("/ops/marketing/coupon-batches", handler.CreateCouponBatch)
	group.POST("/ops/marketing/coupon-batches/:batch_no/issue", handler.IssueCoupons)
	group.POST("/ops/marketing/coupon-batches/:batch_no/commands", handler.ExecuteBatchCommand)
	group.GET("/ops/marketing/coupons", handler.ListIssuedCoupons)
	// 投放领券中心：生成一批无主券码，乘客端先到先得领取。
	group.POST("/ops/marketing/coupons/publish", handler.PublishPublicCoupons)

	group.GET("/ops/marketing/campaigns", handler.ListCampaigns)
	group.POST("/ops/marketing/campaigns", handler.CreateCampaign)
	group.POST("/ops/marketing/campaigns/:code/commands", handler.ExecuteCampaignCommand)
	group.GET("/ops/marketing/campaigns/:code/settlement", handler.CampaignSettlement)
	group.GET("/ops/marketing/campaigns/:code/settlement/export", handler.ExportCampaignSettlement)

	// A08 补贴台账：登记发放、台账查询、补贴口径汇总。
	group.POST("/ops/marketing/campaigns/:code/subsidies", handler.GrantSubsidy)
	group.GET("/ops/marketing/campaigns/:code/subsidies", handler.ListSubsidyRecords)
	group.GET("/ops/marketing/campaigns/:code/subsidy-summary", handler.SubsidySummary)
	// A08 补贴追回：虚假订单或全额退款时冲正，预算同步退回。
	group.POST("/ops/marketing/campaigns/:code/subsidies/:record_no/reverse", handler.ReverseSubsidy)

	// A07 券状态机补齐：作废未使用的券（不可恢复，前端二次确认）。
	group.POST("/ops/marketing/coupons/:coupon_no/void", handler.VoidCoupon)

	// A05 计价规则：草稿、发布、停用、样例预览与生效版本查询。
	group.GET("/ops/pricing/rules", handler.ListPricingRules)
	group.POST("/ops/pricing/rules", handler.CreatePricingRule)
	group.GET("/ops/pricing/rules/effective", handler.EffectivePricingRule)
	group.GET("/ops/pricing/rules/:rule_id", handler.PricingRuleDetail)
	group.POST("/ops/pricing/rules/:rule_id/publish", handler.PublishPricingRule)
	group.POST("/ops/pricing/rules/:rule_id/disable", handler.DisablePricingRule)
	group.POST("/ops/pricing/preview", handler.PreviewPricingFare)

	// A06 派单策略：草稿、发布、候选排序预览。
	group.GET("/ops/dispatch/strategies", handler.ListDispatchStrategies)
	group.POST("/ops/dispatch/strategies", handler.CreateDispatchStrategy)
	group.POST("/ops/dispatch/strategies/:strategy_id/publish", handler.PublishDispatchStrategy)
	// 排序预览要带候选司机入参，用 POST 而不是 GET。
	group.POST("/ops/dispatch/strategies/:strategy_id/preview", handler.PreviewDispatchRanking)

	// A09 拼车与特价：匹配配置、特价固定价方案、拼车统计。
	group.GET("/ops/pool/configs", handler.ListPoolConfigs)
	group.POST("/ops/pool/configs", handler.CreatePoolConfig)
	group.POST("/ops/pool/configs/:config_id/publish", handler.PublishPoolConfig)
	group.GET("/ops/pool/plans", handler.ListSpecialPlans)
	group.POST("/ops/pool/plans", handler.CreateSpecialPlan)
	group.POST("/ops/pool/plans/:plan_id/publish", handler.PublishSpecialPlan)
	group.GET("/ops/pool/stats", handler.PoolStatsRead)
}

// ListUsers returns one page of accounts with effective restrictions.
func (h *Handler) ListUsers(c *echo.Context) error {
	page, pageSize, err := httpreq.Pagination(c)
	if err != nil {
		return err
	}
	output, err := h.usecase.ListUsers(c.Request().Context(), usecase.ListUsersInput{
		Role:     c.QueryParam("role"),
		Keyword:  c.QueryParam("keyword"),
		Page:     page,
		PageSize: pageSize,
	})
	if err != nil {
		return err
	}
	return httpresp.Paginated(c, output.Items, page, pageSize, int(output.Total))
}

// UserDetail returns the account with the full phone number. Unmasking is a
// sensitive read: it is audited by the shared recorder and by the business
// operation log written inside the usecase.
func (h *Handler) UserDetail(c *echo.Context) error {
	operatorID, err := requestctx.RequireUserID(c.Request().Context())
	if err != nil {
		return err
	}
	detail, opErr := h.usecase.UserDetail(c.Request().Context(), c.Param("id"), operatorID, requestctx.GetRequestID(c.Request().Context()))
	if auditErr := h.audit.Record(c, "view_full_phone", "ops_user", c.Param("id"), "查看用户完整手机号", opErr); auditErr != nil {
		return auditErr
	}
	if opErr != nil {
		return opErr
	}
	return httpresp.OK(c, detail)
}

// RestrictUser applies one business limit to an account.
func (h *Handler) RestrictUser(c *echo.Context) error {
	operatorID, err := requestctx.RequireUserID(c.Request().Context())
	if err != nil {
		return err
	}
	var req restrictRequest
	if err := httpreq.BindAndValidate(c, &req); err != nil {
		return err
	}
	restriction, opErr := h.usecase.RestrictUser(c.Request().Context(), usecase.RestrictInput{
		AccountID:  c.Param("id"),
		Type:       req.Type,
		Reason:     req.Reason,
		Scope:      req.Scope,
		OperatorID: operatorID,
		RequestID:  requestctx.GetRequestID(c.Request().Context()),
	})
	resourceID := c.Param("id")
	if restriction.ID != "" {
		resourceID = restriction.ID
	}
	if auditErr := h.audit.Record(c, "restrict", "ops_user", resourceID, "限制用户", opErr); auditErr != nil {
		return auditErr
	}
	if opErr != nil {
		return opErr
	}
	return httpresp.Created(c, restriction)
}

// UnrestrictUser revokes one ACTIVE restriction.
func (h *Handler) UnrestrictUser(c *echo.Context) error {
	operatorID, err := requestctx.RequireUserID(c.Request().Context())
	if err != nil {
		return err
	}
	var req unrestrictRequest
	if err := httpreq.BindAndValidate(c, &req); err != nil {
		return err
	}
	restriction, opErr := h.usecase.UnrestrictUser(c.Request().Context(), usecase.UnrestrictInput{
		AccountID:     c.Param("id"),
		RestrictionID: req.RestrictionID,
		Reason:        req.Reason,
		OperatorID:    operatorID,
		RequestID:     requestctx.GetRequestID(c.Request().Context()),
	})
	if auditErr := h.audit.Record(c, "unrestrict", "ops_user", c.Param("id"), "解除用户限制", opErr); auditErr != nil {
		return auditErr
	}
	if opErr != nil {
		return opErr
	}
	return httpresp.OK(c, restriction)
}
