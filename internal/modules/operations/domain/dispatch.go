// A06 派单策略配置：听单排序权重、候选半径、时限与重派边界。
//
// 派单是平台效率的核心旋钮，但旋钮拧错会直接表现为「叫不到车」。所以权重
// 必须总和 10000（不是随便填几个数），且保存前能预览候选排序结果——运营要
// 看到「谁会被先叫到」变了，才敢发版。
package domain

import (
	"errors"
	"time"
)

// 派单策略状态。
const (
	DispatchStateDraft     = "draft"
	DispatchStatePublished = "published"
	DispatchStateDisabled  = "disabled"
)

// DispatchStateLabel 状态中文名。
var DispatchStateLabel = map[string]string{
	DispatchStateDraft:     "草稿",
	DispatchStatePublished: "已发布",
	DispatchStateDisabled:  "已停用",
}

// 权重总和必须是 10000，这是文档硬约束（0-10000 整数归一）。
const TotalWeightBp = 10000

// 派单配置错误。
var (
	ErrDispatchNotFound       = errors.New("派单策略不存在")
	ErrDispatchWeightInvalid  = errors.New("权重总和必须等于 10000，且不能全为 0")
	ErrDispatchRadiusInvalid  = errors.New("候选半径必须大于 0")
	ErrDispatchWindowInvalid  = errors.New("派单时限不合法：听单 + 抢单必须等于总窗口")
	ErrDispatchReassignBounds = errors.New("重派边界不合法：额外窗口与退回次数上限必须为正")
	ErrDispatchDraftOnly      = errors.New("只有草稿状态可以发布")
)

// DispatchWeights 是听单排序的五项权重，单位万分比，总和 10000。
type DispatchWeights struct {
	IncomeBp      int `json:"income_bp"`       // 本月已结算收入（升序，越少越优先）
	DistanceBp    int `json:"distance_bp"`     // 距离
	RatingBp      int `json:"rating_bp"`       // 好评率
	OnlineSecBp   int `json:"online_sec_bp"`   // 在线时长
	SuccessRateBp int `json:"success_rate_bp"` // 成功率
}

// Sum 返回权重总和。
func (w DispatchWeights) Sum() int {
	return w.IncomeBp + w.DistanceBp + w.RatingBp + w.OnlineSecBp + w.SuccessRateBp
}

// DispatchRedispatch 是重派边界：退单后继承剩余预算，但窗口与次数都有限。
type DispatchRedispatch struct {
	ExtraWindowSec    int `json:"extra_window_sec"`     // 额外重派窗口（秒，最多一次）
	MaxAcceptedReturn int `json:"max_accepted_return"` // 已接单退回次数上限
}

// DispatchStrategy 是一份版本化的派单策略。
type DispatchStrategy struct {
	StrategyID  string             `json:"strategy_id"`
	Version     int64              `json:"version"`
	Weights     DispatchWeights    `json:"weights"`
	RadiusM     int                `json:"radius_m"`
	InviteSec   int                `json:"invite_sec"`
	TotalSec    int                `json:"total_sec"`
	ListenSec   int                `json:"listen_sec"`
	GrabSec     int                `json:"grab_sec"`
	RegionSwitch bool              `json:"region_switch"`
	Redispatch  DispatchRedispatch `json:"redispatch"`
	State       string             `json:"state"`
	Operator    string             `json:"operator,omitempty"`
	CreatedAt   time.Time          `json:"created_at"`
	UpdatedAt   time.Time          `json:"updated_at"`
	PublishedAt time.Time          `json:"published_at,omitempty"`
}

// Validate 校验策略取值：权重、半径、时限、重派边界。
func (s DispatchStrategy) Validate() error {
	if s.Weights.Sum() != TotalWeightBp {
		return ErrDispatchWeightInvalid
	}
	if s.Weights.Sum() == 0 {
		return ErrDispatchWeightInvalid
	}
	if s.RadiusM <= 0 {
		return ErrDispatchRadiusInvalid
	}
	if s.InviteSec <= 0 || s.TotalSec <= 0 || s.ListenSec <= 0 || s.GrabSec <= 0 {
		return ErrDispatchWindowInvalid
	}
	// 先听单后转抢单，两者之和必须等于总窗口，否则超时取消时机对不上。
	if s.ListenSec+s.GrabSec != s.TotalSec {
		return ErrDispatchWindowInvalid
	}
	if s.Redispatch.ExtraWindowSec <= 0 || s.Redispatch.MaxAcceptedReturn <= 0 {
		return ErrDispatchReassignBounds
	}
	return nil
}

// Mutable 只有草稿能改。
func (s DispatchStrategy) Mutable() bool {
	return s.State == DispatchStateDraft
}

// DispatchCandidate 是一个候选司机的原始指标，用于排序预览。
type DispatchCandidate struct {
	DriverID    string `json:"driver_id"`
	DriverName  string `json:"driver_name,omitempty"`
	IncomeFen   int64  `json:"income_fen"`   // 本月已结算收入
	DistanceM   int64  `json:"distance_m"`   // 距上车点
	Rating      int    `json:"rating"`       // 好评率（0-100）
	OnlineSec   int64  `json:"online_sec"`   // 今日在线时长
	SuccessRate int    `json:"success_rate"` // 成功率（0-100）
}

// RankedCandidate 是排序预览的一行：归一化得分与最终名次。
type RankedCandidate struct {
	Rank   int               `json:"rank"`
	Score  int               `json:"score"` // 0-10000
	Detail map[string]int    `json:"detail"`
	Driver DispatchCandidate `json:"driver"`
}

// RankCandidates 按策略给候选司机排序：先按本月已结算收入升序（收入少的先派，
// 保证司机之间机会均衡），收入相同再按权重归一化的综合评分降序。
func (s DispatchStrategy) RankCandidates(candidates []DispatchCandidate) []RankedCandidate {
	scored := make([]RankedCandidate, 0, len(candidates))
	for _, c := range candidates {
		detail := map[string]int{
			"income":       normalizedIncome(c, candidates),
			"distance":     c.Rating, // 占位，下面按维度覆盖
			"rating":       c.Rating,
			"online":       normalizedOnline(c, candidates),
			"success_rate": c.SuccessRate,
		}
		// 距离越近越好，归一化成 0-10000 的得分。
		detail["distance"] = normalizedDistance(c, candidates)
		score := (detail["income"]*s.Weights.IncomeBp +
			detail["distance"]*s.Weights.DistanceBp +
			detail["rating"]*s.Weights.RatingBp +
			detail["online"]*s.Weights.OnlineSecBp +
			detail["success_rate"]*s.Weights.SuccessRateBp) / 100
		scored = append(scored, RankedCandidate{Score: score, Detail: detail, Driver: c})
	}
	// 收入升序优先，同收入按综合评分降序。
	for i := 1; i < len(scored); i++ {
		for j := i; j > 0; j-- {
			prev, cur := scored[j-1], scored[j]
			better := cur.Driver.IncomeFen < prev.Driver.IncomeFen ||
				(cur.Driver.IncomeFen == prev.Driver.IncomeFen && cur.Score > prev.Score)
			if !better {
				break
			}
			scored[j-1], scored[j] = cur, prev
		}
	}
	for i := range scored {
		scored[i].Rank = i + 1
	}
	return scored
}

// normalizedIncome 收入越低得分越高（0-10000）。
func normalizedIncome(c DispatchCandidate, all []DispatchCandidate) int {
	max := int64(0)
	for _, o := range all {
		if o.IncomeFen > max {
			max = o.IncomeFen
		}
	}
	if max == 0 {
		return 10000
	}
	return int(10000 - c.IncomeFen*10000/max)
}

// normalizedDistance 距离越近得分越高（0-10000）。
func normalizedDistance(c DispatchCandidate, all []DispatchCandidate) int {
	max := int64(0)
	for _, o := range all {
		if o.DistanceM > max {
			max = o.DistanceM
		}
	}
	if max == 0 {
		return 10000
	}
	return int(10000 - c.DistanceM*10000/max)
}

// normalizedOnline 在线越久得分越高（0-10000）。
func normalizedOnline(c DispatchCandidate, all []DispatchCandidate) int {
	max := int64(0)
	for _, o := range all {
		if o.OnlineSec > max {
			max = o.OnlineSec
		}
	}
	if max == 0 {
		return 0
	}
	return int(c.OnlineSec * 10000 / max)
}
