// A05 营销中心的配置键、默认值与错误映射。时限与限额一律读配置，不写死。
package usecase

import (
	"errors"
	"fmt"

	"github.com/NSObjects/echo-admin/internal/modules/operations/domain"
	"github.com/NSObjects/echo-admin/internal/platform/apperr"
)

// 配置键。
const (
	// ConfigCouponIssueRatePerMinute 单个批次每分钟最多发放多少张（限流降级阈值）。
	ConfigCouponIssueRatePerMinute = "coupon.issue.rate_per_minute"
	// ConfigCouponStockWarningRatio 库存低于总量的百分之多少时预警。
	ConfigCouponStockWarningRatio = "coupon.stock.warning_ratio"
)

// SSOT 默认值：每分钟最多发 600 张，库存低于 10% 预警。
const (
	DefaultIssueRatePerMinute  = 600
	DefaultStockWarningPercent = 10
)

// ErrMarketingCommandUnknown 是命令名不合法。
var ErrMarketingCommandUnknown = errors.New("unknown marketing command")

// mapMarketingError 把领域规则错误翻译成 HTTP 语义，同时保留原始哨兵便于测试。
func mapMarketingError(err error) error {
	switch {
	case err == nil:
		return nil
	case errors.Is(err, domain.ErrCouponTemplateNotFound):
		return apperr.WrapNotFound(err, "券模板不存在")
	case errors.Is(err, domain.ErrCouponBatchNotFound):
		return apperr.WrapNotFound(err, "券批次不存在")
	case errors.Is(err, domain.ErrCampaignNotFound):
		return apperr.WrapNotFound(err, "营销活动不存在")
	case errors.Is(err, domain.ErrInvalidTemplateNo):
		return apperr.WrapBadRequest(err, "券模板编号不合法")
	case errors.Is(err, domain.ErrInvalidBatchNo):
		return apperr.WrapBadRequest(err, "券批次号不合法")
	case errors.Is(err, domain.ErrInvalidCampaignCode):
		return apperr.WrapBadRequest(err, "活动编码不合法")
	case errors.Is(err, domain.ErrInvalidCouponName):
		return apperr.WrapBadRequest(err, "名称长度需在 2-60 字之间")
	case errors.Is(err, domain.ErrInvalidCouponAmount):
		return apperr.WrapBadRequest(err, "券面额不合法：满减券面额需大于 0 且不能超过使用门槛")
	case errors.Is(err, domain.ErrInvalidDiscountRate):
		return apperr.WrapBadRequest(err, "折扣率需在 1-99 之间")
	case errors.Is(err, domain.ErrInvalidCouponStock):
		return apperr.WrapBadRequest(err, "券库存不合法")
	case errors.Is(err, domain.ErrInvalidCouponPerLimit):
		return apperr.WrapBadRequest(err, "每人限领张数不合法，且不能超过库存")
	case errors.Is(err, domain.ErrInvalidCouponValidity):
		return apperr.WrapBadRequest(err, "券有效期不合法")
	case errors.Is(err, domain.ErrInvalidCouponType):
		return apperr.WrapBadRequest(err, "券类型只能是 cash/discount/new_user")
	case errors.Is(err, domain.ErrInvalidCouponBearer):
		return apperr.WrapBadRequest(err, "承担方只能是 platform/driver/share")
	case errors.Is(err, domain.ErrInvalidIssueCount):
		return apperr.WrapBadRequest(err, "发放数量不合法")
	case errors.Is(err, domain.ErrCouponTemplatePublished):
		return apperr.WrapConflict(err, "券模板已发布，不能重复发布")
	case errors.Is(err, domain.ErrCouponTemplateStopped):
		return apperr.WrapConflict(err, "券模板已停售，不能继续发放")
	case errors.Is(err, domain.ErrCouponTemplateDraft):
		return apperr.WrapConflict(err, "券模板还是草稿，请先发布")
	case errors.Is(err, domain.ErrCouponBatchNotRunning):
		return apperr.WrapConflict(err, "券批次不在发放中")
	case errors.Is(err, domain.ErrCouponBatchClosed):
		return apperr.WrapConflict(err, "券批次已结束或已停止")
	case errors.Is(err, domain.ErrCouponBatchWindow):
		return apperr.WrapConflict(err, "券批次不在发放时间窗内")
	case errors.Is(err, domain.ErrCouponBatchExhausted), errors.Is(err, domain.ErrCouponStockExhausted):
		return apperr.WrapConflict(err, "券已发完，不能超发")
	case errors.Is(err, domain.ErrCouponPerLimitReached):
		return apperr.WrapConflict(err, "该乘客已达每人限领张数")
	case errors.Is(err, domain.ErrCouponMutuallyExclusive):
		return apperr.WrapConflict(err, "该券与乘客已持有的券互斥，不能叠加")
	case errors.Is(err, domain.ErrCampaignCodeTaken):
		return apperr.WrapConflict(err, "活动编码已存在")
	case errors.Is(err, domain.ErrCampaignNotRunning):
		return apperr.WrapConflict(err, "活动不在进行中")
	case errors.Is(err, domain.ErrCampaignEnded):
		return apperr.WrapConflict(err, "活动已结束")
	case errors.Is(err, domain.ErrCampaignWindowInvalid):
		return apperr.WrapBadRequest(err, "活动时间窗不合法")
	case errors.Is(err, domain.ErrCampaignBudgetInvalid):
		return apperr.WrapBadRequest(err, "活动预算不合法")
	case errors.Is(err, domain.ErrSubsidyConfigInvalid):
		return apperr.WrapBadRequest(err, "补贴配置不合法：触发口径、金额、每人上限或日封顶有误")
	case errors.Is(err, domain.ErrSubsidyNotConfigured):
		return apperr.WrapBadRequest(err, "补贴类活动必须配置补贴规则")
	case errors.Is(err, domain.ErrSubsidyCampaignNotRun):
		return apperr.WrapConflict(err, "活动不在进行中，不能登记补贴")
	case errors.Is(err, domain.ErrSubsidyDriverRequired):
		return apperr.WrapBadRequest(err, "补贴对象司机不能为空")
	case errors.Is(err, domain.ErrSubsidyReasonRequired):
		return apperr.WrapBadRequest(err, "补贴事由不能为空")
	case errors.Is(err, domain.ErrSubsidyDriverLimit):
		return apperr.WrapConflict(err, "该司机已达活动补贴次数上限")
	case errors.Is(err, domain.ErrSubsidyDailyCapReached):
		return apperr.WrapConflict(err, "活动当日补贴已达封顶，明天再来")
	case errors.Is(err, domain.ErrSubsidyBudgetExceeded):
		return apperr.WrapConflict(err, "活动预算不足，不能继续发放补贴")

	// ---- A05 计价规则 ----
	case errors.Is(err, domain.ErrPricingRuleNotFound):
		return apperr.WrapNotFound(err, "计价规则不存在")
	case errors.Is(err, domain.ErrPricingCityRequired), errors.Is(err, domain.ErrPricingCarTypeRequired):
		return apperr.WrapBadRequest(err, "计价规则必须指定城市与车型")
	case errors.Is(err, domain.ErrPricingWindowInvalid):
		return apperr.WrapBadRequest(err, "生效区间不合法：开始时间必须早于结束时间")
	case errors.Is(err, domain.ErrPricingBpInvalid):
		return apperr.WrapBadRequest(err, "比例参数超出范围：溢价 1-100000，服务费/佣金/折扣 0-10000")
	case errors.Is(err, domain.ErrPricingStartFeeInvalid), errors.Is(err, domain.ErrPricingPerKmInvalid),
		errors.Is(err, domain.ErrPricingPerMinInvalid):
		return apperr.WrapBadRequest(err, "价格参数不能为负数")
	case errors.Is(err, domain.ErrPricingPublishedReadOnly):
		return apperr.WrapConflict(err, "已发布版本不能修改，请发新版本")
	case errors.Is(err, domain.ErrPricingDraftOnly):
		return apperr.WrapConflict(err, "只有草稿状态可以发布")
	case errors.Is(err, domain.ErrPricingOverlap):
		return apperr.WrapConflict(err, "同一城市车型区域、相同优先级的生效区间不能重叠")
	case errors.Is(err, domain.ErrPricingNoEffective):
		return apperr.WrapNotFound(err, "该城市车型当前没有生效中的计价版本，必须发版后才能接单")
	case errors.Is(err, domain.ErrPricingSampleRequired):
		return apperr.WrapBadRequest(err, "发布前必须预览分账样例且资金守恒")

	// ---- A06 派单策略 ----
	case errors.Is(err, domain.ErrDispatchNotFound):
		return apperr.WrapNotFound(err, "派单策略不存在")
	case errors.Is(err, domain.ErrDispatchWeightInvalid):
		return apperr.WrapBadRequest(err, "权重总和必须等于 10000，且不能全为 0")
	case errors.Is(err, domain.ErrDispatchRadiusInvalid):
		return apperr.WrapBadRequest(err, "候选半径必须大于 0")
	case errors.Is(err, domain.ErrDispatchWindowInvalid):
		return apperr.WrapBadRequest(err, "派单时限不合法：听单 + 抢单必须等于总窗口")
	case errors.Is(err, domain.ErrDispatchReassignBounds):
		return apperr.WrapBadRequest(err, "重派边界不合法：额外窗口与退回次数上限必须为正")
	case errors.Is(err, domain.ErrDispatchDraftOnly):
		return apperr.WrapConflict(err, "只有草稿状态可以发布")

	// ---- A09 拼车与特价 ----
	case errors.Is(err, domain.ErrPoolConfigNotFound):
		return apperr.WrapNotFound(err, "拼车配置不存在")
	case errors.Is(err, domain.ErrPoolMatchSecInvalid):
		return apperr.WrapBadRequest(err, "匹配等待时长必须在 1-180 秒之间")
	case errors.Is(err, domain.ErrPoolGroupMaxInvalid):
		return apperr.WrapBadRequest(err, "拼车组最多 2 单 4 座")
	case errors.Is(err, domain.ErrPoolPlanNotFound):
		return apperr.WrapNotFound(err, "特价方案不存在")
	case errors.Is(err, domain.ErrPoolPlanNameRequired):
		return apperr.WrapBadRequest(err, "特价方案必须填写名称")
	case errors.Is(err, domain.ErrPoolPlanFixedInvalid):
		return apperr.WrapBadRequest(err, "特价固定价必须大于 0")
	case errors.Is(err, domain.ErrPoolPlanCityRequired):
		return apperr.WrapBadRequest(err, "特价方案必须指定城市与车型")
	case errors.Is(err, domain.ErrPoolPlanWindowInvalid):
		return apperr.WrapBadRequest(err, "特价方案生效区间不合法")
	case errors.Is(err, domain.ErrPoolDraftOnly):
		return apperr.WrapConflict(err, "只有草稿状态可以发布")

	// ---- A07 券作废 ----
	case errors.Is(err, domain.ErrCouponNotFound):
		return apperr.WrapNotFound(err, "优惠券不存在")
	case errors.Is(err, domain.ErrCouponNotVoidable):
		return apperr.WrapConflict(err, "只有未使用的券可以作废")
	case errors.Is(err, domain.ErrCouponVoidReason):
		return apperr.WrapBadRequest(err, "作废券必须填写原因")
	case errors.Is(err, domain.ErrCouponConfirmNeeded):
		return apperr.WrapBadRequest(err, "作废券不可恢复，必须先二次确认")

	// ---- A08 补贴追回 ----
	case errors.Is(err, domain.ErrSubsidyRecordNotFound):
		return apperr.WrapNotFound(err, "补贴台账记录不存在")
	case errors.Is(err, domain.ErrSubsidyAlreadyReversed):
		return apperr.WrapConflict(err, "这笔补贴已经追回过了")
	case errors.Is(err, domain.ErrSubsidyReversalReason):
		return apperr.WrapBadRequest(err, "追回补贴必须填写原因")
	case errors.Is(err, ErrMarketingCommandUnknown):
		return apperr.WrapBadRequest(err, "不支持的命令")
	case errors.Is(err, domain.ErrIssueRateLimited):
		// 用限流专用业务码（100210）而不是冲突码：这是"稍后重试"不是"状态不对"。
		return apperr.Newf(apperr.ErrTooManyAttempts, "发放高峰，已限流降级，请稍后重试")
	default:
		return err
	}
}

// skipOf 由页码算出跳过条数，页码从 1 起。
func skipOf(page, pageSize int) int {
	size := limitOf(pageSize)
	if page <= 1 {
		return 0
	}
	return (page - 1) * size
}

// limitOf 约束每页条数，非法值收敛到默认 20，上限 100。
func limitOf(pageSize int) int {
	const defaultSize = 20
	if pageSize <= 0 {
		return defaultSize
	}
	if pageSize > maxPageSize {
		return maxPageSize
	}
	return pageSize
}

// percentOf 计算百分比，分母为 0 时返回 0，避免除零。
func percentOf(part, total int64) float64 {
	if total <= 0 {
		return 0
	}
	return float64(part) * 100 / float64(total)
}

// yuan 把分转成元字符串，用于导出与展示。
func yuan(fen int64) string {
	negative := fen < 0
	if negative {
		fen = -fen
	}
	value := fmt.Sprintf("%d.%02d", fen/100, fen%100)
	if negative {
		return "-" + value
	}
	return value
}
