// Package domain 只放营销中心（优惠券与活动）的纯业务规则，不依赖存储与传输层。
package domain

import (
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"
)

// 券模板状态。库内已有模板的 state 就是中文（如"已发布"），这里沿用同一套取值，
// 否则读不到业务侧已经写进去的历史数据。
const (
	CouponTemplateStateDraft     = "草稿"
	CouponTemplateStatePublished = "已发布"
	CouponTemplateStateStopped   = "已停售"
)

// 券类型。库内已有模板只有满减字段（faceValueFen/thresholdFen），没有类型字段，
// 因此类型由表达式推导：折扣券带 discountRate，其余按满减处理。
const (
	CouponTypeCash     = "cash"     // 满减券
	CouponTypeDiscount = "discount" // 折扣券
	CouponTypeNewUser  = "new_user" // 新客券
)

// 券的承担方。库内已有模板用的是 bearer 字段，取值 platform/driver。
const (
	CouponBearerPlatform = "platform"
	CouponBearerDriver   = "driver"
	CouponBearerShare    = "share"
)

// 券批次状态。
const (
	CouponBatchStatePending  = "pending"
	CouponBatchStateRunning  = "running"
	CouponBatchStateStopped  = "stopped"
	CouponBatchStateFinished = "finished"
)

// 发放渠道。
const (
	CouponChannelNewUser    = "new_user"
	CouponChannelActivity   = "activity"
	CouponChannelManual     = "manual"
	CouponChannelInvite     = "invite"
	CouponChannelCompensate = "compensate"
)

// 已发放券状态。库内 coupons 的 state 取值就是 AVAILABLE，沿用并补齐其余状态。
const (
	CouponStateAvailable = "AVAILABLE"
	CouponStateUsed      = "USED"
	CouponStateExpired   = "EXPIRED"
	CouponStateFrozen    = "FROZEN"
	// VOID 作废：活动下线或误发时回收，只针对未锁定未使用的券，
	// 已锁定在订单里的券不能作废，否则会把用户正在用的钱抽走。
	CouponStateVoid = "VOID"
)

// 券作废错误。
var (
	ErrCouponNotFound       = errors.New("优惠券不存在")
	ErrCouponNotVoidable    = errors.New("只有未使用的券可以作废，已使用/已过期/已作废的券不能再作废")
	ErrCouponVoidReason     = errors.New("作废券必须填写原因")
	ErrCouponConfirmNeeded  = errors.New("作废券不可恢复，必须二次确认")
)

// 活动状态。
const (
	CampaignStateDraft   = "draft"
	CampaignStateRunning = "running"
	CampaignStatePaused  = "paused"
	CampaignStateEnded   = "ended"
)

// 活动类型。
const (
	CampaignTypeNewUserGift   = "new_user_gift"
	CampaignTypeInvite        = "invite"
	CampaignTypeRecharge      = "recharge"
	CampaignTypeSignIn        = "sign_in"
	CampaignTypeTaskCenter    = "task_center"
	CampaignTypeDriverBonus   = "driver_bonus"
	CampaignTypeFlashSale     = "flash_sale"
	CampaignTypeRushSubsidy   = "rush_subsidy"
	CampaignTypePeriodSubsidy = "period_subsidy"
)

// 营销中心的校验错误。
var (
	ErrCouponTemplateNotFound  = errors.New("coupon template not found")
	ErrCouponBatchNotFound     = errors.New("coupon batch not found")
	ErrCampaignNotFound        = errors.New("campaign not found")
	ErrInvalidTemplateNo       = errors.New("invalid coupon template no")
	ErrInvalidBatchNo          = errors.New("invalid coupon batch no")
	ErrInvalidCampaignCode     = errors.New("invalid campaign code")
	ErrInvalidCouponName       = errors.New("invalid coupon name")
	ErrInvalidCouponAmount     = errors.New("invalid coupon amount")
	ErrInvalidCouponStock      = errors.New("invalid coupon stock")
	ErrInvalidCouponPerLimit   = errors.New("invalid coupon per-limit")
	ErrInvalidCouponValidity   = errors.New("invalid coupon validity window")
	ErrInvalidCouponType       = errors.New("invalid coupon type")
	ErrInvalidCouponBearer     = errors.New("invalid coupon bearer")
	ErrInvalidDiscountRate     = errors.New("invalid discount rate")
	ErrInvalidIssueCount       = errors.New("invalid issue count")
	ErrCouponTemplatePublished = errors.New("coupon template already published")
	ErrCouponTemplateStopped   = errors.New("coupon template is stopped")
	ErrCouponTemplateDraft     = errors.New("coupon template is still a draft")
	ErrCouponBatchNotRunning   = errors.New("coupon batch is not running")
	ErrCouponBatchClosed       = errors.New("coupon batch is closed")
	ErrCouponBatchWindow       = errors.New("coupon batch outside its issue window")
	ErrCouponBatchExhausted    = errors.New("coupon batch exhausted")
	ErrCouponStockExhausted    = errors.New("coupon template stock exhausted")
	ErrCouponPerLimitReached   = errors.New("passenger reached the per-person limit")
	ErrCouponMutuallyExclusive = errors.New("coupon conflicts with a held coupon")
	ErrCampaignCodeTaken       = errors.New("campaign code already taken")
	ErrCampaignNotRunning      = errors.New("campaign is not running")
	ErrCampaignEnded           = errors.New("campaign already ended")
	ErrCampaignWindowInvalid   = errors.New("campaign window is invalid")
	ErrCampaignBudgetInvalid   = errors.New("campaign budget is invalid")
	ErrIssueRateLimited        = errors.New("issue rate limited")
)

// 文案与数值边界。
const (
	CouponNameMaxLen   = 60
	CouponNameMinLen   = 2
	CouponDescMaxLen   = 200
	BatchNoLen         = 24
	MaxIssuePerRequest = 1000
)

// CouponTemplate 是一张券的定义。金额一律是分，与库内 faceValueFen 一致。
type CouponTemplate struct {
	TemplateNo      string    `json:"template_no"`
	Name            string    `json:"name"`
	CouponType      string    `json:"coupon_type"`
	FaceValueFen    int64     `json:"face_value_fen"`
	ThresholdFen    int64     `json:"threshold_fen"`
	DiscountRate    int       `json:"discount_rate"`
	MaxDiscountFen  int64     `json:"max_discount_fen"`
	Bearer          string    `json:"bearer"`
	CityCodes       []string  `json:"city_codes"`
	ProductCodes    []string  `json:"product_codes"`
	ValidStart      time.Time `json:"valid_start"`
	ValidEnd        time.Time `json:"valid_end"`
	Stock           int64     `json:"stock"`
	IssuedCount     int64     `json:"issued_count"`
	UsedCount       int64     `json:"used_count"`
	PerLimit        int64     `json:"per_limit"`
	Stackable       bool      `json:"stackable"`
	MutualTemplates []string  `json:"mutual_templates"`
	Description     string    `json:"description"`
	State           string    `json:"state"`
	CreatedAt       time.Time `json:"created_at"`
	UpdatedAt       time.Time `json:"updated_at"`
}

// CouponBatch 是一次发行计划：总量、渠道、预算与时间窗。防超发以它为准。
type CouponBatch struct {
	BatchNo       string    `json:"batch_no"`
	TemplateNo    string    `json:"template_no"`
	TemplateName  string    `json:"template_name"`
	TotalCount    int64     `json:"total_count"`
	IssuedCount   int64     `json:"issued_count"`
	UsedCount     int64     `json:"used_count"`
	Channel       string    `json:"channel"`
	BudgetFen     int64     `json:"budget_fen"`
	UsedBudgetFen int64     `json:"used_budget_fen"`
	Status        string    `json:"status"`
	StartAt       time.Time `json:"start_at"`
	EndAt         time.Time `json:"end_at"`
	CampaignCode  string    `json:"campaign_code"`
	Operator      string    `json:"operator"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}

// IssuedCoupon 是发到用户手里的一张券。对应库内 coupons 集合。
type IssuedCoupon struct {
	CouponNo     string     `json:"coupon_no"`
	TemplateNo   string     `json:"template_no"`
	TemplateName string     `json:"template_name"`
	BatchNo      string     `json:"batch_no"`
	PassengerID  string     `json:"passenger_id"`
	Passenger    string     `json:"passenger_name"`
	PhoneMask    string     `json:"phone_mask"`
	State        string     `json:"state"`
	FaceValueFen int64      `json:"face_value_fen"`
	ThresholdFen int64      `json:"threshold_fen"`
	Bearer       string     `json:"bearer"`
	IssuedAt     time.Time  `json:"issued_at"`
	ExpireAt     time.Time  `json:"expire_at"`
	UsedAt       *time.Time `json:"used_at,omitempty"`
	OrderNo      string     `json:"order_no"`
	Channel      string     `json:"channel"`
}

// Campaign 是一场营销活动。
type Campaign struct {
	Code             string    `json:"code"`
	Name             string    `json:"name"`
	Type             string    `json:"type"`
	TargetAudience   string    `json:"target_audience"`
	Rules            string    `json:"rules"`
	CouponTemplates  []string       `json:"coupon_templates"`
	RewardAmountFen  int64          `json:"reward_amount_fen"`
	SubsidyConfig    *SubsidyConfig `json:"subsidy_config,omitempty"`
	CityCodes        []string       `json:"city_codes"`
	StartAt          time.Time `json:"start_at"`
	EndAt            time.Time `json:"end_at"`
	Status           string    `json:"status"`
	ParticipantCount int64     `json:"participant_count"`
	BudgetFen        int64     `json:"budget_fen"`
	UsedBudgetFen    int64     `json:"used_budget_fen"`
	Operator         string    `json:"operator"`
	CreatedAt        time.Time `json:"created_at"`
	UpdatedAt        time.Time `json:"updated_at"`
}

// CampaignSettlement 是活动结束后的汇总对账：发了多少、核销多少、谁承担。
type CampaignSettlement struct {
	CampaignCode   string            `json:"campaign_code"`
	CampaignName   string            `json:"campaign_name"`
	Status         string            `json:"status"`
	StartAt        time.Time         `json:"start_at"`
	EndAt          time.Time         `json:"end_at"`
	Batches        []SettlementBatch `json:"batches"`
	IssuedCount    int64             `json:"issued_count"`
	UsedCount      int64             `json:"used_count"`
	UnusedCount    int64             `json:"unused_count"`
	ExpiredCount   int64             `json:"expired_count"`
	IssueAmountFen int64             `json:"issue_amount_fen"` // 发放面额合计
	UsedAmountFen  int64             `json:"used_amount_fen"`  // 核销金额合计
	// SubsidyGrantedFen 是补贴台账已发金额（A08）。补贴不走券批次，
	// 单独一列并入预算消耗，财务对账才不会漏掉这部分平台承担的钱。
	SubsidyGrantedFen int64            `json:"subsidy_granted_fen"`
	SubsidyRecordCount int64           `json:"subsidy_record_count"`
	BearerSplit       map[string]int64 `json:"bearer_split"` // 承担方 -> 金额（分）
	BudgetFen      int64             `json:"budget_fen"`
	UsedBudgetFen  int64             `json:"used_budget_fen"`
	BudgetUsagePct float64           `json:"budget_usage_pct"`
	Closed         bool              `json:"closed"`
	GeneratedAt    time.Time         `json:"generated_at"`
}

// SettlementBatch 是汇总里按批次拆开的一行。
type SettlementBatch struct {
	BatchNo        string `json:"batch_no"`
	TemplateNo     string `json:"template_no"`
	TemplateName   string `json:"template_name"`
	Channel        string `json:"channel"`
	TotalCount     int64  `json:"total_count"`
	IssuedCount    int64  `json:"issued_count"`
	UsedCount      int64  `json:"used_count"`
	UnusedCount    int64  `json:"unused_count"`
	FaceValueFen   int64  `json:"face_value_fen"`
	IssueAmountFen int64  `json:"issue_amount_fen"`
	UsedAmountFen  int64  `json:"used_amount_fen"`
	Bearer         string `json:"bearer"`
	Status         string `json:"status"`
}

// ---- 构造与校验 ----

// NewCouponTemplate 校验并构造一张券模板。面额与门槛的合法性由券类型决定：
// 满减券必须有正的面额与非负门槛，折扣券必须有 1-99 的折扣率。
func NewCouponTemplate(templateNo, name, couponType string, faceValueFen, thresholdFen int64,
	discountRate int, maxDiscountFen int64, bearer string, validStart, validEnd time.Time,
	stock, perLimit int64, stackable bool, mutual []string, description string,
) (CouponTemplate, error) {
	templateNo = strings.TrimSpace(templateNo)
	if !isCode(templateNo) {
		return CouponTemplate{}, ErrInvalidTemplateNo
	}
	name = strings.TrimSpace(name)
	if utf8.RuneCountInString(name) < CouponNameMinLen || utf8.RuneCountInString(name) > CouponNameMaxLen {
		return CouponTemplate{}, ErrInvalidCouponName
	}
	if !isCouponType(couponType) {
		return CouponTemplate{}, ErrInvalidCouponType
	}
	if !isBearer(bearer) {
		return CouponTemplate{}, ErrInvalidCouponBearer
	}
	switch couponType {
	case CouponTypeDiscount:
		// 折扣率用整数百分比的十倍表达：80 表示 8.0 折，即支付 80%。
		if discountRate <= 0 || discountRate >= 100 {
			return CouponTemplate{}, ErrInvalidDiscountRate
		}
		if maxDiscountFen < 0 {
			return CouponTemplate{}, ErrInvalidCouponAmount
		}
	default:
		if faceValueFen <= 0 {
			return CouponTemplate{}, ErrInvalidCouponAmount
		}
		if thresholdFen < 0 {
			return CouponTemplate{}, ErrInvalidCouponAmount
		}
		// 满减券的面额不能超过门槛，否则等于免单还倒贴，属于配置事故。
		if thresholdFen > 0 && faceValueFen > thresholdFen {
			return CouponTemplate{}, fmt.Errorf("face value %d exceeds threshold %d: %w", faceValueFen, thresholdFen, ErrInvalidCouponAmount)
		}
	}
	if stock <= 0 {
		return CouponTemplate{}, ErrInvalidCouponStock
	}
	if perLimit <= 0 {
		return CouponTemplate{}, ErrInvalidCouponPerLimit
	}
	if perLimit > stock {
		return CouponTemplate{}, fmt.Errorf("per-limit %d exceeds stock %d: %w", perLimit, stock, ErrInvalidCouponPerLimit)
	}
	if !validEnd.After(validStart) {
		return CouponTemplate{}, ErrInvalidCouponValidity
	}
	if utf8.RuneCountInString(description) > CouponDescMaxLen {
		return CouponTemplate{}, ErrInvalidCouponName
	}
	return CouponTemplate{
		TemplateNo:      templateNo,
		Name:            name,
		CouponType:      couponType,
		FaceValueFen:    faceValueFen,
		ThresholdFen:    thresholdFen,
		DiscountRate:    discountRate,
		MaxDiscountFen:  maxDiscountFen,
		Bearer:          bearer,
		CityCodes:       []string{},
		ProductCodes:    []string{},
		ValidStart:      validStart,
		ValidEnd:        validEnd,
		Stock:           stock,
		PerLimit:        perLimit,
		Stackable:       stackable,
		MutualTemplates: mutual,
		Description:     strings.TrimSpace(description),
		State:           CouponTemplateStateDraft,
	}, nil
}

// CanPublish 判断能否发布：草稿才能发布，且有效期不能已经过期。
func (t CouponTemplate) CanPublish(now time.Time) error {
	if t.State != CouponTemplateStateDraft {
		return ErrCouponTemplatePublished
	}
	if !t.ValidEnd.After(now) {
		return ErrInvalidCouponValidity
	}
	return nil
}

// CanStop 判断能否停售：已发布才能停售。
func (t CouponTemplate) CanStop() error {
	if t.State != CouponTemplateStatePublished {
		return ErrCouponTemplateDraft
	}
	return nil
}

// CanIssue 判断这张模板当前能不能发券。
func (t CouponTemplate) CanIssue(now time.Time) error {
	if t.State != CouponTemplateStatePublished {
		if t.State == CouponTemplateStateStopped {
			return ErrCouponTemplateStopped
		}
		return ErrCouponTemplateDraft
	}
	if now.Before(t.ValidStart) || now.After(t.ValidEnd) {
		return ErrInvalidCouponValidity
	}
	return nil
}

// RemainingStock 返回剩余库存。
func (t CouponTemplate) RemainingStock() int64 {
	remaining := t.Stock - t.IssuedCount
	if remaining < 0 {
		return 0
	}
	return remaining
}

// ConflictsWith 判断这张券与用户已持有的券是否互斥。互斥只在模板显式声明
// 时成立，判定是双向的：任一方把对方写进 mutualTemplates 即视为互斥。
//
// Stackable 不参与这里的判定。它约束的是"同一笔订单能否与其它券叠加核销"，
// 属于交易侧的锁券/核销规则；若把它当作"不能与任何券同时持有"，那手里已经
// 有券的乘客将永远领不到新券，整个发放链路会被一张历史券堵死。
func (t CouponTemplate) ConflictsWith(held CouponTemplate) bool {
	// 同一张模板自己不互斥：持有几张由每人限领管。这条必须放在最前面，
	// 否则互斥清单配错时会把同模板的第二张也挡掉，perLimit > 1 就永远发不满。
	if t.TemplateNo == held.TemplateNo {
		return false
	}
	for _, no := range t.MutualTemplates {
		if no == held.TemplateNo {
			return true
		}
	}
	for _, no := range held.MutualTemplates {
		if no == t.TemplateNo {
			return true
		}
	}
	return false
}

// NewCouponBatch 校验并构造一个券批次。总量决定防超发上限。
func NewCouponBatch(batchNo, templateNo, templateName, channel string, totalCount int64,
	budgetFen int64, startAt, endAt time.Time, campaignCode string,
) (CouponBatch, error) {
	batchNo = strings.TrimSpace(batchNo)
	if !isCode(batchNo) || len(batchNo) > BatchNoLen {
		return CouponBatch{}, ErrInvalidBatchNo
	}
	if strings.TrimSpace(templateNo) == "" {
		return CouponBatch{}, ErrInvalidTemplateNo
	}
	if !isChannel(channel) {
		return CouponBatch{}, errors.New("invalid coupon channel")
	}
	if totalCount <= 0 {
		return CouponBatch{}, ErrInvalidCouponStock
	}
	if budgetFen < 0 {
		return CouponBatch{}, ErrCampaignBudgetInvalid
	}
	if !endAt.After(startAt) {
		return CouponBatch{}, ErrCouponBatchWindow
	}
	return CouponBatch{
		BatchNo:      batchNo,
		TemplateNo:   templateNo,
		TemplateName: strings.TrimSpace(templateName),
		TotalCount:   totalCount,
		Channel:      channel,
		BudgetFen:    budgetFen,
		StartAt:      startAt,
		EndAt:        endAt,
		CampaignCode: strings.TrimSpace(campaignCode),
		Status:       CouponBatchStateRunning,
	}, nil
}

// CanIssue 判断这个批次现在能不能发券：状态、时间窗、剩余量三重校验。
func (b CouponBatch) CanIssue(count int64, now time.Time) error {
	if count <= 0 {
		return ErrInvalidIssueCount
	}
	if count > MaxIssuePerRequest {
		return fmt.Errorf("issue count %d exceeds %d per request: %w", count, MaxIssuePerRequest, ErrInvalidIssueCount)
	}
	if b.Status != CouponBatchStateRunning {
		if b.Status == CouponBatchStateFinished || b.Status == CouponBatchStateStopped {
			return ErrCouponBatchClosed
		}
		return ErrCouponBatchNotRunning
	}
	if now.Before(b.StartAt) || now.After(b.EndAt) {
		return ErrCouponBatchWindow
	}
	remaining := b.TotalCount - b.IssuedCount
	if remaining <= 0 {
		return ErrCouponBatchExhausted
	}
	if count > remaining {
		return fmt.Errorf("issue count %d exceeds remaining %d: %w", count, remaining, ErrCouponBatchExhausted)
	}
	return nil
}

// RemainingCount 返回批次剩余可发数量。
func (b CouponBatch) RemainingCount() int64 {
	remaining := b.TotalCount - b.IssuedCount
	if remaining < 0 {
		return 0
	}
	return remaining
}

// NewCampaign 校验并构造一场活动。
func NewCampaign(code, name, campaignType, targetAudience, rules string,
	rewardAmountFen, budgetFen int64, startAt, endAt time.Time,
) (Campaign, error) {
	code = strings.TrimSpace(code)
	if !isCode(code) {
		return Campaign{}, ErrInvalidCampaignCode
	}
	name = strings.TrimSpace(name)
	if utf8.RuneCountInString(name) < CouponNameMinLen || utf8.RuneCountInString(name) > CouponNameMaxLen {
		return Campaign{}, ErrInvalidCouponName
	}
	if !isCampaignType(campaignType) {
		return Campaign{}, errors.New("invalid campaign type")
	}
	if rewardAmountFen < 0 || budgetFen < 0 {
		return Campaign{}, ErrCampaignBudgetInvalid
	}
	if budgetFen > 0 && rewardAmountFen > budgetFen {
		return Campaign{}, ErrCampaignBudgetInvalid
	}
	if !endAt.After(startAt) {
		return Campaign{}, ErrCampaignWindowInvalid
	}
	return Campaign{
		Code:            code,
		Name:            name,
		Type:            campaignType,
		TargetAudience:  targetAudience,
		Rules:           strings.TrimSpace(rules),
		RewardAmountFen: rewardAmountFen,
		BudgetFen:       budgetFen,
		StartAt:         startAt,
		EndAt:           endAt,
		Status:          CampaignStateDraft,
		CouponTemplates: []string{},
		CityCodes:       []string{},
	}, nil
}

// CanStart 判断活动能否开始：只有草稿可以开始。
func (c Campaign) CanStart() error {
	if c.Status != CampaignStateDraft {
		if c.Status == CampaignStateEnded {
			return ErrCampaignEnded
		}
		return ErrCampaignNotRunning
	}
	return nil
}

// CanPause 判断活动能否暂停：只有进行中可以暂停。
func (c Campaign) CanPause() error {
	if c.Status != CampaignStateRunning {
		return ErrCampaignNotRunning
	}
	return nil
}

// CanEnd 判断活动能否结束：进行中或暂停中都可以结束。
func (c Campaign) CanEnd() error {
	if c.Status == CampaignStateEnded {
		return ErrCampaignEnded
	}
	if c.Status == CampaignStateDraft {
		return ErrCampaignNotRunning
	}
	return nil
}

// ---- 小工具 ----

// isCode 校验编号：只允许字母、数字、- 和 _，避免把它们拼进查询时出问题。
func isCode(value string) bool {
	if value == "" || len(value) > 64 {
		return false
	}
	for _, r := range value {
		switch {
		case r >= 'a' && r <= 'z':
		case r >= 'A' && r <= 'Z':
		case r >= '0' && r <= '9':
		case r == '-' || r == '_':
		default:
			return false
		}
	}
	return true
}

func isCouponType(value string) bool {
	switch value {
	case CouponTypeCash, CouponTypeDiscount, CouponTypeNewUser:
		return true
	}
	return false
}

func isBearer(value string) bool {
	switch value {
	case CouponBearerPlatform, CouponBearerDriver, CouponBearerShare:
		return true
	}
	return false
}

func isChannel(value string) bool {
	switch value {
	case CouponChannelNewUser, CouponChannelActivity, CouponChannelManual,
		CouponChannelInvite, CouponChannelCompensate:
		return true
	}
	return false
}

func isCampaignType(value string) bool {
	switch value {
	case CampaignTypeNewUserGift, CampaignTypeInvite, CampaignTypeRecharge,
		CampaignTypeSignIn, CampaignTypeTaskCenter, CampaignTypeDriverBonus,
		CampaignTypeFlashSale, CampaignTypeRushSubsidy, CampaignTypePeriodSubsidy:
		return true
	}
	return false
}

// ValidCouponTypes 供前端渲染下拉框。
func ValidCouponTypes() []string {
	return []string{CouponTypeCash, CouponTypeDiscount, CouponTypeNewUser}
}

// ValidCouponBearers 供前端渲染下拉框。
func ValidCouponBearers() []string {
	return []string{CouponBearerPlatform, CouponBearerDriver, CouponBearerShare}
}

// ValidCouponChannels 供前端渲染下拉框。
func ValidCouponChannels() []string {
	return []string{CouponChannelNewUser, CouponChannelActivity, CouponChannelManual,
		CouponChannelInvite, CouponChannelCompensate}
}

// ValidCampaignTypes 供前端渲染下拉框。
func ValidCampaignTypes() []string {
	return []string{CampaignTypeNewUserGift, CampaignTypeInvite, CampaignTypeRecharge,
		CampaignTypeSignIn, CampaignTypeTaskCenter, CampaignTypeDriverBonus,
		CampaignTypeFlashSale, CampaignTypeRushSubsidy, CampaignTypePeriodSubsidy}
}
