// A09 拼车与特价配置：匹配参数、拼车组约束与特价固定价方案。
//
// 拼车的商业承诺是「提交时看到的价就是最终价」：拼成按拼车价，拼未成由平台
// 承担优惠差额，乘客价格不变。所以这里的配置一旦改动，直接影响平台补贴成本，
// 必须版本化并留痕。
package domain

import (
	"errors"
	"time"
)

// 配置状态。
const (
	PoolConfigStateDraft     = "draft"
	PoolConfigStatePublished = "published"
	PoolConfigStateDisabled  = "disabled"
)

// PoolStateLabel 状态中文名。
var PoolStateLabel = map[string]string{
	PoolConfigStateDraft:     "草稿",
	PoolConfigStatePublished: "已发布",
	PoolConfigStateDisabled:  "已停用",
}

// 拼车硬约束：每单最多等待 60 秒封组；匹配加首次派单最多 180 秒。
const (
	DefaultMatchSec      = 60
	DefaultGroupMaxOrder = 2
	DefaultGroupMaxSeat  = 4
	MaxDispatchTotalSec  = 180
)

// 拼车/特价配置错误。
var (
	ErrPoolConfigNotFound    = errors.New("拼车配置不存在")
	ErrPoolMatchSecInvalid   = errors.New("匹配等待时长必须在 1-180 秒之间")
	ErrPoolGroupMaxInvalid   = errors.New("拼车组最多 2 单 4 座")
	ErrPoolPlanNotFound      = errors.New("特价方案不存在")
	ErrPoolPlanNameRequired  = errors.New("特价方案必须填写名称")
	ErrPoolPlanFixedInvalid  = errors.New("特价固定价必须大于 0")
	ErrPoolPlanCityRequired  = errors.New("特价方案必须指定城市与车型")
	ErrPoolPlanWindowInvalid = errors.New("特价方案生效区间不合法")
	ErrPoolDraftOnly         = errors.New("只有草稿状态可以发布")
)

// PoolConfig 是拼车匹配参数。
type PoolConfig struct {
	ConfigID   string    `json:"config_id"`
	Version    int64     `json:"version"`
	MatchSec   int       `json:"match_sec"`    // 第一单等待封组时长
	GroupMax   int       `json:"group_max"`    // 每组最多订单数
	SeatMax    int       `json:"seat_max"`     // 每组最多座位数
	CityCodes  []string  `json:"city_codes"`   // 适用城市
	State      string    `json:"state"`
	Operator   string    `json:"operator,omitempty"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
}

// Validate 校验拼车参数。
func (c PoolConfig) Validate() error {
	if c.MatchSec <= 0 || c.MatchSec > MaxDispatchTotalSec {
		return ErrPoolMatchSecInvalid
	}
	if c.GroupMax <= 0 || c.GroupMax > DefaultGroupMaxOrder {
		return ErrPoolGroupMaxInvalid
	}
	if c.SeatMax <= 0 || c.SeatMax > DefaultGroupMaxSeat {
		return ErrPoolGroupMaxInvalid
	}
	return nil
}

// Mutable 只有草稿能改。
func (c PoolConfig) Mutable() bool {
	return c.State == PoolConfigStateDraft
}

// SpecialPricePlan 是一个特价固定价方案：固定价是承诺价，不重复扣活动差价。
type SpecialPricePlan struct {
	PlanID    string    `json:"plan_id"`
	Name      string    `json:"name"`
	CityCode  string    `json:"city_code"`
	CarType   string    `json:"car_type"`
	FixedFen  int64     `json:"fixed_fen"`
	Seats     int       `json:"seats"`
	ValidFrom time.Time `json:"valid_from"`
	ValidTo   time.Time `json:"valid_to"`
	State     string    `json:"state"`
	Operator  string    `json:"operator,omitempty"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// Validate 校验特价方案。
func (p SpecialPricePlan) Validate() error {
	if p.Name == "" {
		return ErrPoolPlanNameRequired
	}
	if p.CityCode == "" || p.CarType == "" {
		return ErrPoolPlanCityRequired
	}
	if p.FixedFen <= 0 {
		return ErrPoolPlanFixedInvalid
	}
	if !p.ValidFrom.IsZero() && !p.ValidTo.IsZero() && !p.ValidTo.After(p.ValidFrom) {
		return ErrPoolPlanWindowInvalid
	}
	return nil
}

// PoolStats 是拼车口径的三个统计：匹配成功率、实际共乘率、拼未成数。
// 三者独立统计，不能互相推导，否则运营看不出是匹配差还是拼成率低。
type PoolStats struct {
	PoolOrderCount   int64   `json:"pool_order_count"`   // 拼车订单数
	GroupedCount     int64   `json:"grouped_count"`      // 成功成组数
	MatchedCount     int64   `json:"matched_count"`      // 匹配成功（有司机接单并出发）
	SharedCount      int64   `json:"shared_count"`       // 实际共乘（≥2 名不同乘客同车出发）
	UnmatchedCount   int64   `json:"unmatched_count"`    // 拼未成数
	MatchSuccessRate float64 `json:"match_success_rate"` // 匹配成功率
	SharedRate       float64 `json:"shared_rate"`        // 实际共乘率
}
