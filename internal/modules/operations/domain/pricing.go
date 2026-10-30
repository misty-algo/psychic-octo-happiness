// A05 计价规则管理：版本化的城市/车型价格配置与分账公式。
//
// 价格是平台最容易出事故的配置：一旦改错，全城订单都算错钱。这里的约束是
// 「生效版本不可原地修改，只能发新版本并指定生效区间」，历史订单按各自快照
// 结算，新单按当前生效版本计价。发布前必须能算出样例分账，运营要看到
// T/Q/H/K/F/P/D/N 八个数字对得上，才敢点发布。
package domain

import (
	"errors"
	"time"
)

// 计价规则状态：草稿可改，已发布只读，停用是不再参与生效选择。
const (
	PricingRuleStateDraft     = "draft"
	PricingRuleStatePublished = "published"
	PricingRuleStateDisabled  = "disabled"
)

// 计价规则各状态的中文名，管理端直接展示。
var PricingRuleStateLabel = map[string]string{
	PricingRuleStateDraft:     "草稿",
	PricingRuleStatePublished: "已发布",
	PricingRuleStateDisabled:  "已停用",
}

// 万分比上限：溢价最多 10 倍，服务费/佣金最多 100%，折扣最多 100%。
const (
	MaxSurgeBp    = 100000
	MaxServiceBp  = 10000
	MaxCommBp     = 10000
	MaxDiscountBp = 10000
)

// 计价错误。
var (
	ErrPricingRuleNotFound      = errors.New("计价规则不存在")
	ErrPricingCityRequired      = errors.New("计价规则必须指定城市")
	ErrPricingWindowInvalid     = errors.New("生效区间不合法：开始时间必须早于结束时间")
	ErrPricingStartFeeInvalid   = errors.New("起步价不能为负数")
	ErrPricingPerKmInvalid      = errors.New("每公里价不能为负数")
	ErrPricingPerMinInvalid     = errors.New("每分钟价不能为负数")
	ErrPricingBpInvalid         = errors.New("比例参数超出允许范围：溢价/服务费/佣金/折扣均为万分比")
	ErrPricingPublishedReadOnly = errors.New("已发布版本不能修改，请发新版本")
	ErrPricingDraftOnly         = errors.New("只有草稿状态可以发布")
	ErrPricingOverlap           = errors.New("同一城市车型区域、相同优先级的生效区间不能重叠")
	ErrPricingNoEffective       = errors.New("该城市车型当前没有生效中的计价版本")
	ErrPricingSampleRequired    = errors.New("发布前必须至少预览三个分账样例")
)

// PricingRule 是一条版本化的计价配置。
type PricingRule struct {
	RuleID     string    `json:"rule_id"`
	Version    int64     `json:"version"`
	CityCode   string    `json:"city_code"`
	Area       string    `json:"area,omitempty"`     // 区域（可空表示全城）
	CarType    string    `json:"car_type"`           // 车型/业务类型
	StartFen   int64     `json:"start_fen"`          // 起步价（分）
	IncludeM   int64     `json:"include_m"`          // 起步包含里程（米）
	PerKmFen   int64     `json:"per_km_fen"`         // 每公里价（分）
	PerMinFen  int64     `json:"per_min_fen"`        // 每分钟价（分）
	ServiceBp  int64     `json:"service_bp"`         // 乘客服务费（万分比）
	CommBp     int64     `json:"comm_bp"`            // 司机佣金（万分比）
	SurgeBp    int64     `json:"surge_bp"`           // 溢价系数（万分比，10000 为不溢价）
	DiscountBp int64     `json:"discount_bp"`        // 拼车折扣（万分比，10000 为不打折）
	CancelFen  int64     `json:"cancel_fen"`         // 取消费（分）
	FixedFen   int64     `json:"fixed_fen"`          // 固定价（分，0 表示按里程时长计价）
	Priority   int       `json:"priority"`           // 区域优先级，重叠时取高者
	ValidFrom  time.Time `json:"valid_from"`
	ValidTo    time.Time `json:"valid_to"`
	State      string    `json:"state"`
	Operator   string    `json:"operator,omitempty"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
}

// Validate 校验一条规则自身的取值是否合法，不涉及版本重叠。
func (r PricingRule) Validate() error {
	if r.CityCode == "" {
		return ErrPricingCityRequired
	}
	if r.CarType == "" {
		return ErrPricingCarTypeRequired
	}
	if !r.ValidFrom.IsZero() && !r.ValidTo.IsZero() && !r.ValidTo.After(r.ValidFrom) {
		return ErrPricingWindowInvalid
	}
	if r.StartFen < 0 {
		return ErrPricingStartFeeInvalid
	}
	if r.PerKmFen < 0 {
		return ErrPricingPerKmInvalid
	}
	if r.PerMinFen < 0 {
		return ErrPricingPerMinInvalid
	}
	if r.SurgeBp <= 0 || r.SurgeBp > MaxSurgeBp {
		return ErrPricingBpInvalid
	}
	if r.ServiceBp < 0 || r.ServiceBp > MaxServiceBp {
		return ErrPricingBpInvalid
	}
	if r.CommBp < 0 || r.CommBp > MaxCommBp {
		return ErrPricingBpInvalid
	}
	// 折扣只在拼车时生效，允许为 0（表示不配置），但不能超过 10000。
	if r.DiscountBp < 0 || r.DiscountBp > MaxDiscountBp {
		return ErrPricingBpInvalid
	}
	if r.CancelFen < 0 || r.FixedFen < 0 {
		return ErrPricingStartFeeInvalid
	}
	return nil
}

// Mutable 判断这条规则还能不能改：已发布版本只能停用或发新版本。
func (r PricingRule) Mutable() bool {
	return r.State == PricingRuleStateDraft
}

// Overlaps 判断两条规则是否在同一城市车型区域、同一优先级上时间重叠。
// 区域重叠时按优先级选一，所以只有优先级相同才禁止重叠。
func (r PricingRule) Overlaps(other PricingRule) bool {
	if r.RuleID != "" && r.RuleID == other.RuleID {
		return false
	}
	if r.CityCode != other.CityCode || r.CarType != other.CarType || r.Area != other.Area {
		return false
	}
	if r.Priority != other.Priority {
		return false
	}
	return r.ValidFrom.Before(other.ValidTo) && other.ValidFrom.Before(r.ValidTo)
}

// EffectiveAt 判断规则在指定时刻是否生效中。
func (r PricingRule) EffectiveAt(at time.Time) bool {
	if r.State != PricingRuleStatePublished {
		return false
	}
	if !r.ValidFrom.IsZero() && at.Before(r.ValidFrom) {
		return false
	}
	if !r.ValidTo.IsZero() && !at.Before(r.ValidTo) {
		return false
	}
	return true
}

// ---- 分账公式（文档 4.1，全平台唯一口径）----
//
//	T = start + Round((dist - incl) * perKm, 1000) + Round(dur * perMin, 60)
//	Q = Round(T * surgeBp, 10000)（拼车再乘 discountBp）
//	H = Round(Q * serviceBp, 10000)（乘客服务费）
//	K = Round(Q * commBp, 10000)（司机佣金）
//	F = Q + H（原车费）
//	P = F - Cp - Cd（乘客应付）
//	D = Q - K - Cd（司机应得）
//	N = H + K - Cp（平台净额）
//	资金守恒：P = D + N + G
//
// Round(n, d) 在文档里写作「舍入到 d 的倍数」，实际含义是先按更细的粒度
// 计算再取整到分：里程项的 d=1000 是把「米×分每公里」换算成公里后取整，
// 时长项的 d=60 是把「秒×分每分钟」换算成分钟后取整。

// roundDiv 做四舍五入的整数除法：roundDiv(n, d) = round(n / d)。
func roundDiv(n, d int64) int64 {
	if d <= 0 {
		return n
	}
	if n >= 0 {
		return (n + d/2) / d
	}
	return -((-n + d/2) / d)
}

// FareInput 是一次计价样例的输入。
type FareInput struct {
	Name       string `json:"name,omitempty"` // 样例名，空则按顺序命名
	DistM      int64  `json:"dist_m"`         // 里程（米）
	DurS       int64 `json:"dur_s"`        // 时长（秒）
	CouponFen  int64 `json:"coupon_fen"`   // 乘客券优惠 Cp
	SubsidyFen int64 `json:"subsidy_fen"`  // 平台补贴/优惠差额 Cd
	Pooled     bool  `json:"pooled"`       // 是否拼车（拼车走折扣）
}

// FareBreakdown 是一次计价的分账结果，八个数字都要能给运营看。
type FareBreakdown struct {
	TFen int64 `json:"t_fen"` // 里程时长费
	QFen int64 `json:"q_fen"` // 溢价后车费
	HFen int64 `json:"h_fen"` // 乘客服务费
	KFen int64 `json:"k_fen"` // 司机佣金
	FFen int64 `json:"f_fen"` // 原车费
	PFen int64 `json:"p_fen"` // 乘客应付
	DFen int64 `json:"d_fen"` // 司机应得
	NFen int64 `json:"n_fen"` // 平台净额
	GFen int64 `json:"g_fen"` // 补贴差额项，守恒时恒为 0
}

// ComputeFare 按规则与输入算一笔账。固定价优先：配了固定价就不再按里程时长算。
func (r PricingRule) ComputeFare(in FareInput) FareBreakdown {
	var t int64
	if r.FixedFen > 0 {
		// 特价/固定价方案：直接取固定价，不再叠加里程时长。
		t = r.FixedFen
	} else {
		extra := in.DistM - r.IncludeM
		if extra < 0 {
			extra = 0
		}
		t = r.StartFen + roundDiv(extra*r.PerKmFen, 1000) + roundDiv(in.DurS*r.PerMinFen, 60)
	}

	q := roundDiv(t*r.SurgeBp, 10000)
	if in.Pooled && r.DiscountBp > 0 {
		q = roundDiv(q*r.DiscountBp, 10000)
	}
	h := roundDiv(q*r.ServiceBp, 10000)
	k := roundDiv(q*r.CommBp, 10000)
	f := q + h
	p := f - in.CouponFen - in.SubsidyFen
	d := q - k - in.SubsidyFen
	n := h + k - in.CouponFen
	// 守恒校验：P = D + N + G，理想情况 G = 0。
	g := p - d - n
	return FareBreakdown{TFen: t, QFen: q, HFen: h, KFen: k, FFen: f, PFen: p, DFen: d, NFen: n, GFen: g}
}

// FareSample 是一个发布前的分账样例，供运营核对。
type FareSample struct {
	Name  string        `json:"name"`
	Input FareInput     `json:"input"`
	Fare  FareBreakdown `json:"fare"`
}

// ErrPricingCarTypeRequired 车型必须指定，否则无法判断生效范围。
var ErrPricingCarTypeRequired = errors.New("计价规则必须指定车型/业务类型")
