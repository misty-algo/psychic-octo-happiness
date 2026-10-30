// A08 补贴活动：结构化补贴规则、补贴台账与预算/日封顶约束。
// 补贴类活动发的是平台承担的钱（落补贴台账），与 A07 的券发放走两条账。
package domain

import (
	"errors"
	"strings"
	"time"
)

// 补贴类活动类型：类型落在补贴族里时必须携带结构化补贴配置。
func IsSubsidyCampaignType(t string) bool {
	switch t {
	case "rush_subsidy", "period_subsidy", "driver_bonus":
		return true
	}
	return false
}

// 补贴触发口径。
const (
	SubsidyTriggerPerOrder   = "per_order"   // 每单固定补贴（一口价差额等）
	SubsidyTriggerTimePeriod = "time_period" // 时段出车/在线补贴
	SubsidyTriggerTask       = "task"        // 冲单任务达成奖
)

// 补贴台账业务类型：对齐数据字典 driver_reward_penalties.bizType 的口径。
const (
	SubsidyBizTimeSubsidy  = "time_subsidy"
	SubsidyBizCompleteTask = "complete_task"
	SubsidyBizPeakBonus    = "peak_bonus"
	SubsidyBizOther        = "other"
)

// ValidSubsidyBizType 校验台账业务类型是否在字典口径内。
func ValidSubsidyBizType(t string) bool {
	switch t {
	case SubsidyBizTimeSubsidy, SubsidyBizCompleteTask, SubsidyBizPeakBonus, SubsidyBizOther:
		return true
	}
	return false
}

// 补贴域错误。
var (
	ErrSubsidyConfigInvalid   = errors.New("subsidy config is invalid")
	ErrSubsidyNotConfigured   = errors.New("campaign has no subsidy config")
	ErrSubsidyCampaignNotRun  = errors.New("campaign is not running for subsidy grant")
	ErrSubsidyDriverRequired  = errors.New("subsidy driver is required")
	ErrSubsidyDriverLimit     = errors.New("driver has reached the subsidy limit")
	ErrSubsidyDailyCapReached = errors.New("campaign daily subsidy cap reached")
	ErrSubsidyBudgetExceeded  = errors.New("campaign subsidy budget exceeded")
	ErrSubsidyReasonRequired  = errors.New("subsidy reason is required")
	// ErrSubsidyAlreadyReversed 一笔补贴不能追回两次，重复点击要挡住。
	ErrSubsidyAlreadyReversed = errors.New("subsidy record already reversed")
	ErrSubsidyReversalReason  = errors.New("追回补贴必须填写原因")
)

// ErrSubsidyRecordNotFound 补贴台账记录不存在。
var ErrSubsidyRecordNotFound = errors.New("补贴台账记录不存在")

// 预算告警阈值：用到 80% 就要提醒运营补预算，100% 停止发放。
const SubsidyBudgetWarnPct = 80

// BudgetUsagePct 返回预算使用百分比。
func BudgetUsagePct(usedFen, budgetFen int64) float64 {
	if budgetFen <= 0 {
		return 0
	}
	return float64(usedFen) / float64(budgetFen) * 100
}

// BudgetWarn 判断是否需要告警：使用率达到 80%。
func BudgetWarn(usedFen, budgetFen int64) bool {
	return budgetFen > 0 && BudgetUsagePct(usedFen, budgetFen) >= SubsidyBudgetWarnPct
}

// SubsidyConfig 是补贴活动的结构化规则。补贴金额由配置决定，
// 登记时不允许现场改金额，保证"配置即规则"。
type SubsidyConfig struct {
	TriggerType  string `json:"trigger_type"`           // per_order / time_period / task
	AmountFen    int64  `json:"amount_fen"`             // 单次补贴金额（分）
	MaxPerDriver int64  `json:"max_per_driver"`         // 每个司机累计次数上限
	DailyCapFen  int64  `json:"daily_cap_fen"`          // 活动单日发放封顶（分，0 表示不封顶）
	TaskType     string `json:"task_type,omitempty"`    // 字典 16.6 campaigns.taskType，触发口径为 task 时必填
	NeedEnroll   bool   `json:"need_enroll"`            // 是否需要司机报名（字典 16.6 campaigns.needEnroll）
}

// Validate 校验补贴配置本身是否可执行。
func (c SubsidyConfig) Validate() error {
	switch c.TriggerType {
	case SubsidyTriggerPerOrder, SubsidyTriggerTimePeriod, SubsidyTriggerTask:
	default:
		return ErrSubsidyConfigInvalid
	}
	if c.AmountFen <= 0 {
		return ErrSubsidyConfigInvalid
	}
	if c.MaxPerDriver <= 0 {
		return ErrSubsidyConfigInvalid
	}
	// 单日封顶至少要够发一笔，否则这活动今天就注定失败，属于配置事故。
	if c.DailyCapFen > 0 && c.DailyCapFen < c.AmountFen {
		return ErrSubsidyConfigInvalid
	}
	if c.TriggerType == SubsidyTriggerTask && strings.TrimSpace(c.TaskType) == "" {
		return ErrSubsidyConfigInvalid
	}
	return nil
}

// SubsidyRecord 是一笔补贴台账。谁发、发给谁、多少钱、为什么发，一行说清。
type SubsidyRecord struct {
	RecordNo     string    `json:"record_no"`
	CampaignCode string    `json:"campaign_code"`
	CampaignName string    `json:"campaign_name"`
	DriverID     string    `json:"driver_id"`
	OrderNo      string    `json:"order_no,omitempty"`
	AmountFen    int64     `json:"amount_fen"`
	BizType      string    `json:"biz_type"`
	Reason       string    `json:"reason"`
	Day          string    `json:"day"` // 补贴归属日 yyyy-MM-dd，日封顶的统计键
	Operator     string    `json:"operator"`
	CreatedAt    time.Time `json:"created_at"`
	// 追回：虚假订单或全额退款时冲正。台账只追加不修改，冲正用反向记录表达，
	// 原记录打上追回标记，预算同步退回。
	Reversed      bool      `json:"reversed"`
	ReversedAt    time.Time `json:"reversed_at,omitempty"`
	ReversedBy    string    `json:"reversed_by,omitempty"`
	ReverseReason string    `json:"reverse_reason,omitempty"`
}

// Voidable 判断这笔补贴还能不能被追回：已追回或金额非正都不能再追。
func (r SubsidyRecord) Voidable() bool {
	return !r.Reversed && r.AmountFen > 0 && r.RecordNo != ""
}

// NewSubsidyRecord 构造一笔补贴台账并做基础校验。
func NewSubsidyRecord(recordNo, campaignCode, campaignName, driverID, orderNo string,
	amountFen int64, bizType, reason, day string, now time.Time) (SubsidyRecord, error) {
	if strings.TrimSpace(driverID) == "" {
		return SubsidyRecord{}, ErrSubsidyDriverRequired
	}
	if strings.TrimSpace(reason) == "" {
		return SubsidyRecord{}, ErrSubsidyReasonRequired
	}
	if amountFen <= 0 {
		return SubsidyRecord{}, ErrSubsidyConfigInvalid
	}
	if !ValidSubsidyBizType(bizType) {
		return SubsidyRecord{}, ErrSubsidyConfigInvalid
	}
	return SubsidyRecord{
		RecordNo:     recordNo,
		CampaignCode: campaignCode,
		CampaignName: campaignName,
		DriverID:     strings.TrimSpace(driverID),
		OrderNo:      strings.TrimSpace(orderNo),
		AmountFen:    amountFen,
		BizType:      bizType,
		Reason:       strings.TrimSpace(reason),
		Day:          day,
		Operator:     "",
		CreatedAt:    now,
	}, nil
}

// CampaignSubsidySummary 是补贴口径的汇总：预算、累计发放、当日进度与触达人数。
type CampaignSubsidySummary struct {
	CampaignCode    string `json:"campaign_code"`
	CampaignName    string `json:"campaign_name"`
	Status          string `json:"status"`
	TriggerType     string `json:"trigger_type"`
	AmountFen       int64  `json:"amount_fen"`
	MaxPerDriver    int64  `json:"max_per_driver"`
	DailyCapFen     int64  `json:"daily_cap_fen"`
	BudgetFen       int64  `json:"budget_fen"`
	UsedBudgetFen   int64  `json:"used_budget_fen"`
	BudgetRemainFen int64  `json:"budget_remain_fen"`
	GrantedTotalFen int64  `json:"granted_total_fen"`
	RecordCount     int64  `json:"record_count"`
	DriverCount     int64  `json:"driver_count"`
	TodayGrantedFen int64  `json:"today_granted_fen"`
	Today           string `json:"today"`
	// BudgetWarn 预算使用率达到 80% 时置位，前端要显式提醒补预算。
	BudgetWarn     bool    `json:"budget_warn"`
	BudgetUsagePct float64 `json:"budget_usage_pct"`
}
