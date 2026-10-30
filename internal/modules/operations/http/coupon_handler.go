// A05 营销中心的 HTTP 接口：券模板、券批次、发放、活动与对账导出。
package operationshttp

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/labstack/echo/v5"

	"github.com/NSObjects/echo-admin/internal/modules/operations/domain"
	"github.com/NSObjects/echo-admin/internal/modules/operations/usecase"
	"github.com/NSObjects/echo-admin/internal/platform/requestctx"
	"github.com/NSObjects/echo-admin/internal/platform/server/httpreq"
	"github.com/NSObjects/echo-admin/internal/platform/server/httpresp"
)

// ---- 请求体 ----

// createCouponTemplateRequest 是新建券模板的请求。金额单位是分。
type createCouponTemplateRequest struct {
	TemplateNo      string   `json:"template_no" validate:"required,min=2,max=64"`
	Name            string   `json:"name" validate:"required,min=2,max=60"`
	CouponType      string   `json:"coupon_type" validate:"omitempty,oneof=cash discount new_user"`
	FaceValueFen    int64    `json:"face_value_fen" validate:"omitempty,gte=0"`
	ThresholdFen    int64    `json:"threshold_fen" validate:"omitempty,gte=0"`
	DiscountRate    int      `json:"discount_rate" validate:"omitempty,gte=1,lte=99"`
	MaxDiscountFen  int64    `json:"max_discount_fen" validate:"omitempty,gte=0"`
	Bearer          string   `json:"bearer" validate:"omitempty,oneof=platform driver share"`
	CityCodes       []string `json:"city_codes"`
	ProductCodes    []string `json:"product_codes"`
	ValidStart      string   `json:"valid_start" validate:"required"`
	ValidEnd        string   `json:"valid_end" validate:"required"`
	Stock           int64    `json:"stock" validate:"required,gt=0"`
	PerLimit        int64    `json:"per_limit" validate:"required,gt=0"`
	Stackable       bool     `json:"stackable"`
	MutualTemplates []string `json:"mutual_templates"`
	Description     string   `json:"description" validate:"omitempty,max=200"`
}

// templateCommandRequest 是券模板命令：publish 发布、stop 停售。
type templateCommandRequest struct {
	Command string `json:"command" validate:"required,oneof=publish stop"`
}

// createCouponBatchRequest 是新建券批次的请求。
type createCouponBatchRequest struct {
	BatchNo      string `json:"batch_no" validate:"required,min=2,max=24"`
	TemplateNo   string `json:"template_no" validate:"required"`
	TotalCount   int64  `json:"total_count" validate:"required,gt=0"`
	Channel      string `json:"channel" validate:"omitempty,oneof=new_user activity manual invite compensate"`
	BudgetFen    int64  `json:"budget_fen" validate:"omitempty,gte=0"`
	StartAt      string `json:"start_at" validate:"required"`
	EndAt        string `json:"end_at" validate:"required"`
	CampaignCode string `json:"campaign_code" validate:"omitempty,max=64"`
}

// batchCommandRequest 是券批次命令：stop 停止、finish 结束。
type batchCommandRequest struct {
	Command string `json:"command" validate:"required,oneof=stop finish"`
}

// issueCouponsRequest 是发放请求。发给谁必须明确列出。
type issueCouponsRequest struct {
	PassengerIDs []string `json:"passenger_ids" validate:"required,min=1,max=1000,dive,required"`
}

// subsidyConfigRequest 是补贴活动的结构化规则（A08）。
type subsidyConfigRequest struct {
	TriggerType  string `json:"trigger_type" validate:"required,oneof=per_order time_period task"`
	AmountFen    int64  `json:"amount_fen" validate:"required,gt=0"`
	MaxPerDriver int64  `json:"max_per_driver" validate:"required,gt=0"`
	DailyCapFen  int64  `json:"daily_cap_fen" validate:"omitempty,gte=0"`
	TaskType     string `json:"task_type" validate:"omitempty,max=40"`
	NeedEnroll   bool   `json:"need_enroll"`
}

// createCampaignRequest 是新建活动的请求。
type createCampaignRequest struct {
	Code            string                `json:"code" validate:"required,min=2,max=64"`
	Name            string                `json:"name" validate:"required,min=2,max=60"`
	Type            string                `json:"type" validate:"required,oneof=new_user_gift invite recharge sign_in task_center driver_bonus flash_sale rush_subsidy period_subsidy"`
	TargetAudience  string                `json:"target_audience" validate:"omitempty,oneof=passenger driver all"`
	Rules           string                `json:"rules" validate:"omitempty,max=500"`
	CouponTemplates []string              `json:"coupon_templates"`
	RewardAmountFen int64                 `json:"reward_amount_fen" validate:"omitempty,gte=0"`
	SubsidyConfig   *subsidyConfigRequest `json:"subsidy_config"`
	CityCodes       []string              `json:"city_codes"`
	StartAt         string                `json:"start_at" validate:"required"`
	EndAt           string                `json:"end_at" validate:"required"`
	BudgetFen       int64                 `json:"budget_fen" validate:"omitempty,gte=0"`
}

// grantSubsidyRequest 是登记一笔补贴发放的请求（A08）。
type grantSubsidyRequest struct {
	DriverID string `json:"driver_id" validate:"required,max=64"`
	OrderNo  string `json:"order_no" validate:"omitempty,max=40"`
	BizType  string `json:"biz_type" validate:"required,oneof=time_subsidy complete_task peak_bonus other"`
	Reason   string `json:"reason" validate:"required,min=2,max=200"`
}

// campaignCommandRequest 是活动命令：start 开始、pause 暂停、end 结束。
type campaignCommandRequest struct {
	Command string `json:"command" validate:"required,oneof=start pause end"`
}

// ---- 券模板 ----

// ListCouponTemplates returns one page of coupon templates.
func (h *Handler) ListCouponTemplates(c *echo.Context) error {
	page, pageSize, err := httpreq.Pagination(c)
	if err != nil {
		return err
	}
	output, err := h.marketing.ListCouponTemplates(c.Request().Context(), usecase.ListMarketingInput{
		Keyword:  c.QueryParam("keyword"),
		State:    c.QueryParam("state"),
		Type:     c.QueryParam("type"),
		Page:     page,
		PageSize: pageSize,
	})
	if err != nil {
		return err
	}
	return httpresp.Paginated(c, output.Items, page, pageSize, int(output.Total))
}

// CouponTemplateDetail returns one coupon template.
func (h *Handler) CouponTemplateDetail(c *echo.Context) error {
	output, err := h.marketing.CouponTemplateDetail(c.Request().Context(), c.Param("template_no"))
	if err != nil {
		return err
	}
	return httpresp.OK(c, output)
}

// CreateCouponTemplate creates a draft coupon template.
func (h *Handler) CreateCouponTemplate(c *echo.Context) error {
	operatorID, err := requestctx.RequireUserID(c.Request().Context())
	if err != nil {
		return err
	}
	var req createCouponTemplateRequest
	if err := httpreq.BindAndValidate(c, &req); err != nil {
		return err
	}
	output, opErr := h.marketing.CreateCouponTemplate(c.Request().Context(), usecase.CreateCouponTemplateInput{
		TemplateNo:      req.TemplateNo,
		Name:            req.Name,
		CouponType:      req.CouponType,
		FaceValueFen:    req.FaceValueFen,
		ThresholdFen:    req.ThresholdFen,
		DiscountRate:    req.DiscountRate,
		MaxDiscountFen:  req.MaxDiscountFen,
		Bearer:          req.Bearer,
		CityCodes:       req.CityCodes,
		ProductCodes:    req.ProductCodes,
		ValidStart:      req.ValidStart,
		ValidEnd:        req.ValidEnd,
		Stock:           req.Stock,
		PerLimit:        req.PerLimit,
		Stackable:       req.Stackable,
		MutualTemplates: req.MutualTemplates,
		Description:     req.Description,
		OperatorID:      operatorID,
		RequestID:       requestctx.GetRequestID(c.Request().Context()),
	})
	detail := "创建券模板"
	if opErr == nil {
		detail = "创建券模板：" + output.TemplateNo + " " + output.Name
	}
	if auditErr := h.audit.Record(c, "create_coupon_template", "ops_marketing", output.TemplateNo, detail, opErr); auditErr != nil {
		return auditErr
	}
	if opErr != nil {
		return opErr
	}
	return httpresp.OK(c, output)
}

// ExecuteTemplateCommand publishes or stops a coupon template.
func (h *Handler) ExecuteTemplateCommand(c *echo.Context) error {
	operatorID, err := requestctx.RequireUserID(c.Request().Context())
	if err != nil {
		return err
	}
	var req templateCommandRequest
	if err := httpreq.BindAndValidate(c, &req); err != nil {
		return err
	}
	templateNo := c.Param("template_no")
	output, opErr := h.marketing.ExecuteTemplateCommand(c.Request().Context(), templateNo, req.Command,
		operatorID, requestctx.GetRequestID(c.Request().Context()))
	detail := "券模板命令"
	if opErr == nil {
		detail = fmt.Sprintf("券模板命令：%s → %s", req.Command, output.State)
	}
	if auditErr := h.audit.Record(c, "coupon_template_command", "ops_marketing", templateNo, detail, opErr); auditErr != nil {
		return auditErr
	}
	if opErr != nil {
		return opErr
	}
	return httpresp.OK(c, output)
}

// ---- 券批次 ----

// ListCouponBatches returns one page of coupon batches.
func (h *Handler) ListCouponBatches(c *echo.Context) error {
	page, pageSize, err := httpreq.Pagination(c)
	if err != nil {
		return err
	}
	output, err := h.marketing.ListCouponBatches(c.Request().Context(), usecase.ListMarketingInput{
		TemplateNo:   c.QueryParam("template_no"),
		Channel:      c.QueryParam("channel"),
		Status:       c.QueryParam("status"),
		CampaignCode: c.QueryParam("campaign_code"),
		Page:         page,
		PageSize:     pageSize,
	})
	if err != nil {
		return err
	}
	return httpresp.Paginated(c, output.Items, page, pageSize, int(output.Total))
}

// CreateCouponBatch creates an issue batch whose total caps the issuance.
func (h *Handler) CreateCouponBatch(c *echo.Context) error {
	operatorID, err := requestctx.RequireUserID(c.Request().Context())
	if err != nil {
		return err
	}
	var req createCouponBatchRequest
	if err := httpreq.BindAndValidate(c, &req); err != nil {
		return err
	}
	output, opErr := h.marketing.CreateCouponBatch(c.Request().Context(), usecase.CreateCouponBatchInput{
		BatchNo:      req.BatchNo,
		TemplateNo:   req.TemplateNo,
		TotalCount:   req.TotalCount,
		Channel:      req.Channel,
		BudgetFen:    req.BudgetFen,
		StartAt:      req.StartAt,
		EndAt:        req.EndAt,
		CampaignCode: req.CampaignCode,
		OperatorID:   operatorID,
		RequestID:    requestctx.GetRequestID(c.Request().Context()),
	})
	detail := "创建券批次"
	if opErr == nil {
		detail = fmt.Sprintf("创建券批次：%s 总量=%d", output.BatchNo, output.TotalCount)
	}
	if auditErr := h.audit.Record(c, "create_coupon_batch", "ops_marketing", req.BatchNo, detail, opErr); auditErr != nil {
		return auditErr
	}
	if opErr != nil {
		return opErr
	}
	return httpresp.OK(c, output)
}

// IssueCoupons issues coupons to the named passengers.
func (h *Handler) IssueCoupons(c *echo.Context) error {
	operatorID, err := requestctx.RequireUserID(c.Request().Context())
	if err != nil {
		return err
	}
	var req issueCouponsRequest
	if err := httpreq.BindAndValidate(c, &req); err != nil {
		return err
	}
	batchNo := c.Param("batch_no")
	output, opErr := h.marketing.IssueCoupons(c.Request().Context(), batchNo, usecase.IssueCouponsInput{
		PassengerIDs: req.PassengerIDs,
		OperatorID:   operatorID,
		RequestID:    requestctx.GetRequestID(c.Request().Context()),
	})
	detail := "发放优惠券"
	if opErr == nil {
		detail = fmt.Sprintf("发放优惠券：%s 发出 %d 张，挡下 %d 人", batchNo, output.Issued, len(output.Skipped))
	} else {
		detail = fmt.Sprintf("发放优惠券：%s 失败（%v）", batchNo, opErr)
	}
	if auditErr := h.audit.Record(c, "issue_coupons", "ops_marketing", batchNo, detail, opErr); auditErr != nil {
		return auditErr
	}
	if opErr != nil {
		return opErr
	}
	return httpresp.OK(c, output)
}

// ExecuteBatchCommand stops or finishes a coupon batch.
func (h *Handler) ExecuteBatchCommand(c *echo.Context) error {
	operatorID, err := requestctx.RequireUserID(c.Request().Context())
	if err != nil {
		return err
	}
	var req batchCommandRequest
	if err := httpreq.BindAndValidate(c, &req); err != nil {
		return err
	}
	batchNo := c.Param("batch_no")
	output, opErr := h.marketing.ExecuteBatchCommand(c.Request().Context(), batchNo, req.Command,
		operatorID, requestctx.GetRequestID(c.Request().Context()))
	detail := "券批次命令"
	if opErr == nil {
		detail = fmt.Sprintf("券批次命令：%s → %s", req.Command, output.Status)
	}
	if auditErr := h.audit.Record(c, "coupon_batch_command", "ops_marketing", batchNo, detail, opErr); auditErr != nil {
		return auditErr
	}
	if opErr != nil {
		return opErr
	}
	return httpresp.OK(c, output)
}

// ListIssuedCoupons returns one page of issued coupons.
func (h *Handler) ListIssuedCoupons(c *echo.Context) error {
	page, pageSize, err := httpreq.Pagination(c)
	if err != nil {
		return err
	}
	output, err := h.marketing.ListIssuedCoupons(c.Request().Context(), usecase.ListMarketingInput{
		BatchNo:     c.QueryParam("batch_no"),
		TemplateNo:  c.QueryParam("template_no"),
		PassengerID: c.QueryParam("passenger_id"),
		Phone:       c.QueryParam("phone"),
		State:       c.QueryParam("state"),
		Page:        page,
		PageSize:    pageSize,
	})
	if err != nil {
		return err
	}
	return httpresp.Paginated(c, output.Items, page, pageSize, int(output.Total))
}

// ---- 活动 ----

// ListCampaigns returns one page of campaigns.
func (h *Handler) ListCampaigns(c *echo.Context) error {
	page, pageSize, err := httpreq.Pagination(c)
	if err != nil {
		return err
	}
	output, err := h.marketing.ListCampaigns(c.Request().Context(), usecase.ListMarketingInput{
		Keyword:  c.QueryParam("keyword"),
		Status:   c.QueryParam("status"),
		Type:     c.QueryParam("type"),
		Page:     page,
		PageSize: pageSize,
	})
	if err != nil {
		return err
	}
	return httpresp.Paginated(c, output.Items, page, pageSize, int(output.Total))
}

// CreateCampaign creates a draft campaign.
func (h *Handler) CreateCampaign(c *echo.Context) error {
	operatorID, err := requestctx.RequireUserID(c.Request().Context())
	if err != nil {
		return err
	}
	var req createCampaignRequest
	if err := httpreq.BindAndValidate(c, &req); err != nil {
		return err
	}
	output, opErr := h.marketing.CreateCampaign(c.Request().Context(), usecase.CreateCampaignInput{
		Code:            req.Code,
		Name:            req.Name,
		Type:            req.Type,
		TargetAudience:  req.TargetAudience,
		Rules:           req.Rules,
		CouponTemplates: req.CouponTemplates,
		RewardAmountFen: req.RewardAmountFen,
		SubsidyConfig:   convertSubsidyConfig(req.SubsidyConfig),
		CityCodes:       req.CityCodes,
		StartAt:         req.StartAt,
		EndAt:           req.EndAt,
		BudgetFen:       req.BudgetFen,
		OperatorID:      operatorID,
		RequestID:       requestctx.GetRequestID(c.Request().Context()),
	})
	detail := "创建营销活动"
	if opErr == nil {
		detail = "创建营销活动：" + output.Code + " " + output.Name
	}
	if auditErr := h.audit.Record(c, "create_campaign", "ops_marketing", req.Code, detail, opErr); auditErr != nil {
		return auditErr
	}
	if opErr != nil {
		return opErr
	}
	return httpresp.OK(c, output)
}

// ExecuteCampaignCommand starts, pauses or ends a campaign.
func (h *Handler) ExecuteCampaignCommand(c *echo.Context) error {
	operatorID, err := requestctx.RequireUserID(c.Request().Context())
	if err != nil {
		return err
	}
	var req campaignCommandRequest
	if err := httpreq.BindAndValidate(c, &req); err != nil {
		return err
	}
	code := c.Param("code")
	output, opErr := h.marketing.ExecuteCampaignCommand(c.Request().Context(), code, req.Command,
		operatorID, requestctx.GetRequestID(c.Request().Context()))
	detail := "活动命令"
	if opErr == nil {
		detail = fmt.Sprintf("活动命令：%s → %s", req.Command, output.Status)
	}
	if auditErr := h.audit.Record(c, "campaign_command", "ops_marketing", code, detail, opErr); auditErr != nil {
		return auditErr
	}
	if opErr != nil {
		return opErr
	}
	return httpresp.OK(c, output)
}

// CampaignSettlement returns the reconciliation summary of one campaign.
func (h *Handler) CampaignSettlement(c *echo.Context) error {
	output, err := h.marketing.CampaignSettlement(c.Request().Context(), c.Param("code"))
	if err != nil {
		return err
	}
	return httpresp.OK(c, output)
}

// ExportCampaignSettlement streams the settlement as a CSV file for finance.
func (h *Handler) ExportCampaignSettlement(c *echo.Context) error {
	code := strings.TrimSpace(c.Param("code"))
	settlement, opErr := h.marketing.CampaignSettlement(c.Request().Context(), code)
	detail := "导出活动对账单"
	if opErr == nil {
		detail = fmt.Sprintf("导出活动对账单：%s 核销 %d 张", code, settlement.UsedCount)
	}
	if auditErr := h.audit.Record(c, "export_campaign_settlement", "ops_marketing", code, detail, opErr); auditErr != nil {
		return auditErr
	}
	if opErr != nil {
		return opErr
	}
	filename := fmt.Sprintf("campaign-settlement-%s.csv", code)
	c.Response().Header().Set("Content-Type", "text/csv; charset=utf-8")
	c.Response().Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%q", filename))
	return c.Blob(http.StatusOK, "text/csv; charset=utf-8", []byte(usecase.SettlementCSV(settlement)))
}

// ---- 补贴台账（A08）----

// convertSubsidyConfig 把请求里的补贴配置翻译成领域对象；没传就是 nil。
func convertSubsidyConfig(req *subsidyConfigRequest) *domain.SubsidyConfig {
	if req == nil {
		return nil
	}
	return &domain.SubsidyConfig{
		TriggerType:  req.TriggerType,
		AmountFen:    req.AmountFen,
		MaxPerDriver: req.MaxPerDriver,
		DailyCapFen:  req.DailyCapFen,
		TaskType:     req.TaskType,
		NeedEnroll:   req.NeedEnroll,
	}
}

// GrantSubsidy registers one subsidy grant into the campaign ledger.
func (h *Handler) GrantSubsidy(c *echo.Context) error {
	operatorID, err := requestctx.RequireUserID(c.Request().Context())
	if err != nil {
		return err
	}
	var req grantSubsidyRequest
	if err := httpreq.BindAndValidate(c, &req); err != nil {
		return err
	}
	code := c.Param("code")
	output, opErr := h.marketing.GrantSubsidy(c.Request().Context(), code, usecase.GrantSubsidyInput{
		DriverID:   req.DriverID,
		OrderNo:    req.OrderNo,
		BizType:    req.BizType,
		Reason:     req.Reason,
		OperatorID: operatorID,
		RequestID:  requestctx.GetRequestID(c.Request().Context()),
	})
	detail := "登记补贴发放"
	if opErr == nil {
		detail = fmt.Sprintf("登记补贴发放：%s → 司机 %s，%d分", code, output.Record.DriverID, output.Record.AmountFen)
	}
	if auditErr := h.audit.Record(c, "grant_subsidy", "ops_marketing", code, detail, opErr); auditErr != nil {
		return auditErr
	}
	if opErr != nil {
		return opErr
	}
	return httpresp.OK(c, output)
}

// ListSubsidyRecords returns one page of the campaign subsidy ledger.
func (h *Handler) ListSubsidyRecords(c *echo.Context) error {
	page, pageSize, err := httpreq.Pagination(c)
	if err != nil {
		return err
	}
	output, err := h.marketing.ListSubsidyRecords(c.Request().Context(), usecase.ListMarketingInput{
		CampaignCode: c.Param("code"),
		DriverID:     c.QueryParam("driver_id"),
		BizType:      c.QueryParam("biz_type"),
		Day:          c.QueryParam("day"),
		Page:         page,
		PageSize:     pageSize,
	})
	if err != nil {
		return err
	}
	return httpresp.Paginated(c, output.Items, page, pageSize, int(output.Total))
}

// voidCouponRequest 是作废一张券的请求：confirm 是前端二次确认的落点。
type voidCouponRequest struct {
	Reason  string `json:"reason" validate:"required,min=2,max=200"`
	Confirm bool   `json:"confirm"`
}

// VoidCoupon voids one available coupon.
func (h *Handler) VoidCoupon(c *echo.Context) error {
	operatorID, err := requestctx.RequireUserID(c.Request().Context())
	if err != nil {
		return err
	}
	var req voidCouponRequest
	if err := httpreq.BindAndValidate(c, &req); err != nil {
		return err
	}
	couponNo := c.Param("coupon_no")
	coupon, opErr := h.marketing.VoidCoupon(c.Request().Context(), couponNo, usecase.VoidCouponInput{
		Reason: req.Reason, Confirm: req.Confirm,
		OperatorID: operatorID, RequestID: requestctx.GetRequestID(c.Request().Context()),
	})
	detail := "作废优惠券 " + couponNo
	if opErr == nil {
		detail = "作废优惠券：" + coupon.CouponNo + "（" + req.Reason + "）"
	}
	if auditErr := h.audit.Record(c, "void_coupon", "ops_marketing", couponNo, detail, opErr); auditErr != nil {
		return auditErr
	}
	if opErr != nil {
		return opErr
	}
	return httpresp.OK(c, coupon)
}

// reverseSubsidyRequest 是追回一笔补贴的请求。
type reverseSubsidyRequest struct {
	Reason string `json:"reason" validate:"required,min=2,max=200"`
}

// ReverseSubsidy reverses one subsidy grant: fake orders or full refunds.
func (h *Handler) ReverseSubsidy(c *echo.Context) error {
	operatorID, err := requestctx.RequireUserID(c.Request().Context())
	if err != nil {
		return err
	}
	var req reverseSubsidyRequest
	if err := httpreq.BindAndValidate(c, &req); err != nil {
		return err
	}
	recordNo := c.Param("record_no")
	record, opErr := h.marketing.ReverseSubsidy(c.Request().Context(), c.Param("code"), recordNo, usecase.ReverseSubsidyInput{
		Reason: req.Reason, OperatorID: operatorID, RequestID: requestctx.GetRequestID(c.Request().Context()),
	})
	detail := "追回补贴 " + recordNo
	if opErr == nil {
		detail = "追回补贴：" + record.RecordNo + " " + fmt.Sprintf("%d", record.AmountFen) + "分（" + req.Reason + "）"
	}
	if auditErr := h.audit.Record(c, "reverse_subsidy", "ops_marketing", recordNo, detail, opErr); auditErr != nil {
		return auditErr
	}
	if opErr != nil {
		return opErr
	}
	return httpresp.OK(c, record)
}

// SubsidySummary returns the subsidy budget and grant progress of one campaign.
func (h *Handler) SubsidySummary(c *echo.Context) error {
	output, err := h.marketing.SubsidySummary(c.Request().Context(), c.Param("code"))
	if err != nil {
		return err
	}
	return httpresp.OK(c, output)
}
