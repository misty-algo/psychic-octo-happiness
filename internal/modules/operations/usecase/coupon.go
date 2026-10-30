// A05 营销中心用例层：券模板与券批次。存储只走 MongoDB。
package usecase

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/NSObjects/echo-admin/internal/modules/operations/domain"
)

// MarketingStore 只暴露营销中心需要的存储能力。库存与配额的扣减必须是条件
// 更新（CAS），由存储层保证并发下不会超发——用"先读后写"一定会被并发击穿。
type MarketingStore interface {
	// ---- 券模板 ----
	ListCouponTemplates(ctx context.Context, filter CouponTemplateFilter) ([]domain.CouponTemplate, int64, error)
	FindCouponTemplate(ctx context.Context, templateNo string) (domain.CouponTemplate, error)
	// FindCouponTemplates 批量取模板定义，键同时支持 templateNo 与 _id 的
	// 十六进制形式（历史券只存 templateId），供互斥判定一次取齐。
	FindCouponTemplates(ctx context.Context, templateNos []string) (map[string]domain.CouponTemplate, error)
	CreateCouponTemplate(ctx context.Context, template domain.CouponTemplate) (domain.CouponTemplate, error)
	UpdateCouponTemplate(ctx context.Context, templateNo string, set CouponTemplateUpdate) (domain.CouponTemplate, error)
	// ConsumeTemplateStock 原子扣减库存；剩余不足时返回 false 且不写任何东西。
	ConsumeTemplateStock(ctx context.Context, templateNo string, count int64, at time.Time) (bool, error)
	// RestoreTemplateStock 回滚库存，用于发放中途失败时补偿。
	RestoreTemplateStock(ctx context.Context, templateNo string, count int64) error

	// ---- 券批次 ----
	ListCouponBatches(ctx context.Context, filter CouponBatchFilter) ([]domain.CouponBatch, int64, error)
	FindCouponBatch(ctx context.Context, batchNo string) (domain.CouponBatch, error)
	CreateCouponBatch(ctx context.Context, batch domain.CouponBatch) (domain.CouponBatch, error)
	// ReserveBatchQuota 原子占用发行额度；总额度不足时返回 false。
	ReserveBatchQuota(ctx context.Context, batchNo string, count int64, at time.Time) (bool, error)
	// ReleaseBatchQuota 回滚发行额度。
	ReleaseBatchQuota(ctx context.Context, batchNo string, count int64) error
	UpdateCouponBatch(ctx context.Context, batchNo string, set CouponBatchUpdate) (domain.CouponBatch, error)
	// ListBatchesByCampaign 汇总对账时按活动取批次。
	ListBatchesByCampaign(ctx context.Context, campaignCode string) ([]domain.CouponBatch, error)

	// ---- 已发放券 ----
	// CountHeldCoupons 返回每个乘客对该模板已持有的券数（每人限领校验）。
	CountHeldCoupons(ctx context.Context, passengerIDs []string, templateNo string) (map[string]int64, error)
	// ListHeldTemplateNos 返回乘客当前可用券的模板编号（叠加互斥校验）。
	ListHeldTemplateNos(ctx context.Context, passengerIDs []string) (map[string][]string, error)
	FindPassengers(ctx context.Context, ids []string) (map[string]domain.Account, error)
	InsertCoupons(ctx context.Context, coupons []domain.IssuedCoupon) (int, error)
	ListIssuedCoupons(ctx context.Context, filter IssuedCouponFilter) ([]domain.IssuedCoupon, int64, error)
	// CountCouponsByBatch 按批次统计发放/核销/过期数（汇总对账）。
	CountCouponsByBatch(ctx context.Context, batchNos []string) (map[string]CouponStat, error)

	// ---- 活动 ----
	ListCampaigns(ctx context.Context, filter CampaignFilter) ([]domain.Campaign, int64, error)
	FindCampaign(ctx context.Context, code string) (domain.Campaign, error)
	CreateCampaign(ctx context.Context, campaign domain.Campaign) (domain.Campaign, error)
	UpdateCampaign(ctx context.Context, code string, set CampaignUpdate) (domain.Campaign, error)

	// ---- 补贴台账（A08）----
	// ReserveCampaignBudget 原子扣减活动预算；预算不足时返回 false 且不写任何东西。
	ReserveCampaignBudget(ctx context.Context, code string, amountFen int64) (bool, error)
	// ReleaseCampaignBudget 回滚活动预算，用于日封顶扣减失败时的补偿。
	ReleaseCampaignBudget(ctx context.Context, code string, amountFen int64) error
	// CountSubsidiesByDriver 统计某司机在某活动下已领补贴次数（每人上限校验）。
	CountSubsidiesByDriver(ctx context.Context, campaignCode, driverID string) (int64, error)
	// ReserveSubsidyDaily 原子占用活动当日补贴封顶额度；超额返回 false。
	ReserveSubsidyDaily(ctx context.Context, campaignCode, day string, capFen, amountFen int64) (bool, error)
	// InsertSubsidyRecord 写入一笔补贴台账。
	InsertSubsidyRecord(ctx context.Context, record domain.SubsidyRecord) error
	// ListSubsidyRecords 返回补贴台账一页。
	ListSubsidyRecords(ctx context.Context, filter SubsidyRecordFilter) ([]domain.SubsidyRecord, int64, error)
	// SummarizeSubsidies 聚合补贴台账：累计发放、笔数、触达司机数与当日发放。
	SummarizeSubsidies(ctx context.Context, campaignCode, today string) (SubsidyAggregate, error)

	// ---- 计价规则（A05）----
	// NextPricingVersion 返回该城市车型区域的下一个版本号。
	NextPricingVersion(ctx context.Context, cityCode, carType, area string) (int64, error)
	CreatePricingRule(ctx context.Context, rule domain.PricingRule) (domain.PricingRule, error)
	FindPricingRule(ctx context.Context, ruleID string) (domain.PricingRule, error)
	ListPricingRules(ctx context.Context, filter PricingRuleFilter) ([]domain.PricingRule, int64, error)
	// FindPricingOverlap 找出与待发规则区间重叠的同优先级已发布规则；没有返回 nil。
	FindPricingOverlap(ctx context.Context, rule domain.PricingRule) (*domain.PricingRule, error)
	PublishPricingRule(ctx context.Context, ruleID string) (domain.PricingRule, error)
	DisablePricingRule(ctx context.Context, ruleID string) (domain.PricingRule, error)
	// FindEffectivePricingRule 取指定时刻生效的规则：无有效价返回 ErrPricingNoEffective。
	FindEffectivePricingRule(ctx context.Context, cityCode, carType string, at time.Time) (domain.PricingRule, error)

	// ---- 派单策略（A06）----
	NextDispatchVersion(ctx context.Context) (int64, error)
	CreateDispatchStrategy(ctx context.Context, strategy domain.DispatchStrategy) (domain.DispatchStrategy, error)
	FindDispatchStrategy(ctx context.Context, strategyID string) (domain.DispatchStrategy, error)
	ListDispatchStrategies(ctx context.Context, filter DispatchFilter) ([]domain.DispatchStrategy, int64, error)
	PublishDispatchStrategy(ctx context.Context, strategyID string) (domain.DispatchStrategy, error)
	DisableDispatchStrategy(ctx context.Context, strategyID string) (domain.DispatchStrategy, error)
	// ListDispatchCandidates 取最近一批真实派单候选司机，用于排序预览。
	ListDispatchCandidates(ctx context.Context, limit int) ([]domain.DispatchCandidate, error)

	// ---- 拼车与特价（A09）----
	NextPoolVersion(ctx context.Context) (int64, error)
	CreatePoolConfig(ctx context.Context, config domain.PoolConfig) (domain.PoolConfig, error)
	FindPoolConfig(ctx context.Context, configID string) (domain.PoolConfig, error)
	ListPoolConfigs(ctx context.Context, filter PoolFilter) ([]domain.PoolConfig, int64, error)
	PublishPoolConfig(ctx context.Context, configID string) (domain.PoolConfig, error)
	DisablePoolConfig(ctx context.Context, configID string) (domain.PoolConfig, error)
	CreateSpecialPlan(ctx context.Context, plan domain.SpecialPricePlan) (domain.SpecialPricePlan, error)
	ListSpecialPlans(ctx context.Context, filter PoolFilter) ([]domain.SpecialPricePlan, int64, error)
	PublishSpecialPlan(ctx context.Context, planID string) (domain.SpecialPricePlan, error)
	DisableSpecialPlan(ctx context.Context, planID string) (domain.SpecialPricePlan, error)
	// PoolStats 统计拼车口径：匹配成功率、实际共乘率、拼未成数。
	PoolStats(ctx context.Context) (domain.PoolStats, error)

	// ---- A07 券状态机补齐 ----
	// FindIssuedCoupon 按券码查一张已发放券。
	FindIssuedCoupon(ctx context.Context, couponNo string) (domain.IssuedCoupon, error)
	// VoidIssuedCoupon 作废一张券；只有 AVAILABLE 能作废，返回是否命中。
	VoidIssuedCoupon(ctx context.Context, couponNo string) (bool, error)

	// ---- A08 补贴追回 ----
	// FindSubsidyRecord 按流水号查一笔补贴台账。
	FindSubsidyRecord(ctx context.Context, recordNo string) (domain.SubsidyRecord, error)
	// MarkSubsidyReversed 把一笔补贴标记为已追回。
	MarkSubsidyReversed(ctx context.Context, record domain.SubsidyRecord) error

	// ---- 公共 ----
	ReadMarketingConfig(ctx context.Context, key string, fallback int) int
	RecordOperationLog(ctx context.Context, log domain.OperationLog) error
}

// ---- 筛选与更新载体 ----

// CouponStat 是一个批次下券的汇总计数。
type CouponStat struct {
	Issued  int64 `json:"issued"`
	Used    int64 `json:"used"`
	Expired int64 `json:"expired"`
}

// CouponTemplateFilter 券模板列表筛选。Skip/Limit 由用例层根据分页算出。
type CouponTemplateFilter struct {
	Keyword string
	State   string
	Type    string
	Skip    int
	Limit   int
}

// CouponBatchFilter 券批次列表筛选。
type CouponBatchFilter struct {
	TemplateNo   string
	Channel      string
	Status       string
	CampaignCode string
	Skip         int
	Limit        int
}

// IssuedCouponFilter 已发放券列表筛选。
type IssuedCouponFilter struct {
	BatchNo     string
	TemplateNo  string
	PassengerID string
	Phone       string
	State       string
	Skip        int
	Limit       int
}

// CampaignFilter 活动列表筛选。
type CampaignFilter struct {
	Keyword string
	Status  string
	Type    string
	Skip    int
	Limit   int
}

// SubsidyRecordFilter 补贴台账筛选。
type SubsidyRecordFilter struct {
	CampaignCode string
	DriverID     string
	BizType      string
	Day          string
	Skip         int
	Limit        int
}

// SubsidyAggregate 是补贴台账的聚合结果。
type SubsidyAggregate struct {
	GrantedTotalFen int64
	RecordCount     int64
	DriverCount     int64
	TodayGrantedFen int64
}

// CouponTemplateUpdate 是模板的可变字段。
type CouponTemplateUpdate struct {
	State    string
	IssuedBy int64
	UsedBy   int64
	At       time.Time
}

// CouponBatchUpdate 是批次的可变字段。
type CouponBatchUpdate struct {
	Status   string
	IssuedBy int64
	UsedBy   int64
	At       time.Time
}

// CampaignUpdate 是活动的可变字段。
type CampaignUpdate struct {
	Status       string
	UsedBudgetBy int64
	Operator     string
	At           time.Time
}

// ---- 输入输出 ----

// ListMarketingInput 是营销中心列表的通用分页入参。
type ListMarketingInput struct {
	Page         int
	PageSize     int
	Keyword      string
	State        string
	Status       string
	Type         string
	Channel      string
	TemplateNo   string
	BatchNo      string
	PassengerID  string
	Phone        string
	CampaignCode string
	DriverID     string
	BizType      string
	Day          string
}

// CreateCouponTemplateInput 是新建券模板的请求。金额一律是分。
type CreateCouponTemplateInput struct {
	TemplateNo      string   `json:"template_no"`
	Name            string   `json:"name"`
	CouponType      string   `json:"coupon_type"`
	FaceValueFen    int64    `json:"face_value_fen"`
	ThresholdFen    int64    `json:"threshold_fen"`
	DiscountRate    int      `json:"discount_rate"`
	MaxDiscountFen  int64    `json:"max_discount_fen"`
	Bearer          string   `json:"bearer"`
	CityCodes       []string `json:"city_codes"`
	ProductCodes    []string `json:"product_codes"`
	ValidStart      string   `json:"valid_start"`
	ValidEnd        string   `json:"valid_end"`
	Stock           int64    `json:"stock"`
	PerLimit        int64    `json:"per_limit"`
	Stackable       bool     `json:"stackable"`
	MutualTemplates []string `json:"mutual_templates"`
	Description     string   `json:"description"`
	OperatorID      int64
	RequestID       string
}

// CreateCouponBatchInput 是新建券批次的请求。
type CreateCouponBatchInput struct {
	BatchNo      string `json:"batch_no"`
	TemplateNo   string `json:"template_no"`
	TotalCount   int64  `json:"total_count"`
	Channel      string `json:"channel"`
	BudgetFen    int64  `json:"budget_fen"`
	StartAt      string `json:"start_at"`
	EndAt        string `json:"end_at"`
	CampaignCode string `json:"campaign_code"`
	OperatorID   int64
	RequestID    string
}

// CreateCampaignInput 是新建活动的请求。
type CreateCampaignInput struct {
	Code            string                 `json:"code"`
	Name            string                 `json:"name"`
	Type            string                 `json:"type"`
	TargetAudience  string                 `json:"target_audience"`
	Rules           string                 `json:"rules"`
	CouponTemplates []string               `json:"coupon_templates"`
	RewardAmountFen int64                  `json:"reward_amount_fen"`
	SubsidyConfig   *domain.SubsidyConfig  `json:"subsidy_config"`
	CityCodes       []string               `json:"city_codes"`
	StartAt         string                 `json:"start_at"`
	EndAt           string                 `json:"end_at"`
	BudgetFen       int64                  `json:"budget_fen"`
	OperatorID      int64
	RequestID       string
}

// ListCouponTemplatesOutput 是券模板一页。
type ListCouponTemplatesOutput struct {
	Items []domain.CouponTemplate `json:"templates"`
	Total int64                   `json:"total"`
}

// ListCouponBatchesOutput 是券批次一页。
type ListCouponBatchesOutput struct {
	Items []domain.CouponBatch `json:"batches"`
	Total int64                `json:"total"`
}

// ListIssuedCouponsOutput 是已发放券一页。
type ListIssuedCouponsOutput struct {
	Items []domain.IssuedCoupon `json:"coupons"`
	Total int64                 `json:"total"`
}

// ListCampaignsOutput 是活动一页。
type ListCampaignsOutput struct {
	Items []domain.Campaign `json:"campaigns"`
	Total int64             `json:"total"`
}

// ---- 用例 ----

// MarketingUsecase 承载营销中心的全部业务规则。
type MarketingUsecase struct {
	store   MarketingStore
	now     func() time.Time
	limiter *IssueLimiter
}

// NewMarketing 创建营销中心用例。
func NewMarketing(store MarketingStore) *MarketingUsecase {
	return &MarketingUsecase{store: store, now: time.Now, limiter: NewIssueLimiter()}
}

// ---- 券模板 ----

// ListCouponTemplates 返回券模板一页。
func (u *MarketingUsecase) ListCouponTemplates(ctx context.Context, in ListMarketingInput) (ListCouponTemplatesOutput, error) {
	filter := CouponTemplateFilter{
		Keyword: strings.TrimSpace(in.Keyword),
		State:   strings.TrimSpace(in.State),
		Type:    strings.TrimSpace(in.Type),
		Skip:    skipOf(in.Page, in.PageSize),
		Limit:   limitOf(in.PageSize),
	}
	items, total, err := u.store.ListCouponTemplates(ctx, filter)
	if err != nil {
		return ListCouponTemplatesOutput{}, fmt.Errorf("list coupon templates: %w", err)
	}
	if items == nil {
		items = []domain.CouponTemplate{}
	}
	return ListCouponTemplatesOutput{Items: items, Total: total}, nil
}

// CouponTemplateDetail 返回单个券模板。查不到要翻译成 404，不能裸抛成 500。
func (u *MarketingUsecase) CouponTemplateDetail(ctx context.Context, templateNo string) (domain.CouponTemplate, error) {
	template, err := u.store.FindCouponTemplate(ctx, strings.TrimSpace(templateNo))
	if err != nil {
		return domain.CouponTemplate{}, mapMarketingError(err)
	}
	return template, nil
}

// CreateCouponTemplate 新建券模板，落成草稿态；发布是独立动作，避免误发。
func (u *MarketingUsecase) CreateCouponTemplate(ctx context.Context, in CreateCouponTemplateInput) (domain.CouponTemplate, error) {
	validStart, err := parseTime(in.ValidStart)
	if err != nil {
		return domain.CouponTemplate{}, mapMarketingError(domain.ErrInvalidCouponValidity)
	}
	validEnd, err := parseTime(in.ValidEnd)
	if err != nil {
		return domain.CouponTemplate{}, mapMarketingError(domain.ErrInvalidCouponValidity)
	}
	if in.CouponType == "" {
		in.CouponType = domain.CouponTypeCash
	}
	if in.Bearer == "" {
		in.Bearer = domain.CouponBearerPlatform
	}
	template, err := domain.NewCouponTemplate(in.TemplateNo, in.Name, in.CouponType, in.FaceValueFen,
		in.ThresholdFen, in.DiscountRate, in.MaxDiscountFen, in.Bearer, validStart, validEnd,
		in.Stock, in.PerLimit, in.Stackable, in.MutualTemplates, in.Description)
	if err != nil {
		return domain.CouponTemplate{}, mapMarketingError(err)
	}
	if len(in.CityCodes) > 0 {
		template.CityCodes = in.CityCodes
	}
	if len(in.ProductCodes) > 0 {
		template.ProductCodes = in.ProductCodes
	}
	now := u.now()
	template.CreatedAt = now
	template.UpdatedAt = now
	created, err := u.store.CreateCouponTemplate(ctx, template)
	if err != nil {
		return domain.CouponTemplate{}, mapMarketingError(err)
	}
	if err := u.store.RecordOperationLog(ctx, domain.OperationLog{
		Module:    "operations",
		Operator:  operatorName(in.OperatorID),
		Action:    "create_coupon_template",
		Resource:  "coupon_template",
		Target:    created.TemplateNo,
		RequestID: in.RequestID,
		Detail: fmt.Sprintf("创建券模板 %s 面额=%d分 门槛=%d分 库存=%d 每人限领=%d 承担方=%s 可叠加=%v",
			created.Name, created.FaceValueFen, created.ThresholdFen, created.Stock, created.PerLimit, created.Bearer, created.Stackable),
		At: now,
	}); err != nil {
		return domain.CouponTemplate{}, err
	}
	return created, nil
}

// ExecuteTemplateCommand 执行券模板命令：publish 发布、stop 停售。
func (u *MarketingUsecase) ExecuteTemplateCommand(ctx context.Context, templateNo, command string, operatorID int64, requestID string) (domain.CouponTemplate, error) {
	template, err := u.store.FindCouponTemplate(ctx, strings.TrimSpace(templateNo))
	if err != nil {
		return domain.CouponTemplate{}, mapMarketingError(err)
	}
	now := u.now()
	var nextState string
	switch command {
	case "publish":
		if err := template.CanPublish(now); err != nil {
			return domain.CouponTemplate{}, mapMarketingError(err)
		}
		nextState = domain.CouponTemplateStatePublished
	case "stop":
		if err := template.CanStop(); err != nil {
			return domain.CouponTemplate{}, mapMarketingError(err)
		}
		nextState = domain.CouponTemplateStateStopped
	default:
		return domain.CouponTemplate{}, mapMarketingError(fmt.Errorf("command %s: %w", command, ErrMarketingCommandUnknown))
	}
	updated, err := u.store.UpdateCouponTemplate(ctx, template.TemplateNo, CouponTemplateUpdate{State: nextState, At: now})
	if err != nil {
		return domain.CouponTemplate{}, mapMarketingError(err)
	}
	if err := u.store.RecordOperationLog(ctx, domain.OperationLog{
		Module:    "operations",
		Operator:  operatorName(operatorID),
		Action:    "coupon_template_" + command,
		Resource:  "coupon_template",
		Target:    updated.TemplateNo,
		RequestID: requestID,
		Detail:    fmt.Sprintf("券模板 %s %s→%s", updated.Name, template.State, nextState),
		At:        now,
	}); err != nil {
		return domain.CouponTemplate{}, err
	}
	return updated, nil
}

// ---- 券批次 ----

// ListCouponBatches 返回券批次一页。
func (u *MarketingUsecase) ListCouponBatches(ctx context.Context, in ListMarketingInput) (ListCouponBatchesOutput, error) {
	filter := CouponBatchFilter{
		TemplateNo:   strings.TrimSpace(in.TemplateNo),
		Channel:      strings.TrimSpace(in.Channel),
		Status:       strings.TrimSpace(in.Status),
		CampaignCode: strings.TrimSpace(in.CampaignCode),
		Skip:         skipOf(in.Page, in.PageSize),
		Limit:        limitOf(in.PageSize),
	}
	items, total, err := u.store.ListCouponBatches(ctx, filter)
	if err != nil {
		return ListCouponBatchesOutput{}, fmt.Errorf("list coupon batches: %w", err)
	}
	if items == nil {
		items = []domain.CouponBatch{}
	}
	return ListCouponBatchesOutput{Items: items, Total: total}, nil
}

// CreateCouponBatch 新建券批次。总量就是防超发上限，创建后不可上调。
func (u *MarketingUsecase) CreateCouponBatch(ctx context.Context, in CreateCouponBatchInput) (domain.CouponBatch, error) {
	template, err := u.store.FindCouponTemplate(ctx, strings.TrimSpace(in.TemplateNo))
	if err != nil {
		return domain.CouponBatch{}, mapMarketingError(err)
	}
	// 批次总量不能超过模板剩余库存，否则这个批次注定发不完，属于配置事故。
	if in.TotalCount > template.RemainingStock() {
		return domain.CouponBatch{}, mapMarketingError(fmt.Errorf(
			"batch total %d exceeds remaining template stock %d: %w",
			in.TotalCount, template.RemainingStock(), domain.ErrInvalidCouponStock))
	}
	// 批次挂活动是为了后续对账，活动不存在就会挂成空账，这里直接拦住。
	if code := strings.TrimSpace(in.CampaignCode); code != "" {
		if _, err := u.store.FindCampaign(ctx, code); err != nil {
			return domain.CouponBatch{}, mapMarketingError(err)
		}
	}
	startAt, err := parseTime(in.StartAt)
	if err != nil {
		return domain.CouponBatch{}, mapMarketingError(domain.ErrCouponBatchWindow)
	}
	endAt, err := parseTime(in.EndAt)
	if err != nil {
		return domain.CouponBatch{}, mapMarketingError(domain.ErrCouponBatchWindow)
	}
	if in.Channel == "" {
		in.Channel = domain.CouponChannelManual
	}
	batch, err := domain.NewCouponBatch(in.BatchNo, template.TemplateNo, template.Name, in.Channel,
		in.TotalCount, in.BudgetFen, startAt, endAt, in.CampaignCode)
	if err != nil {
		return domain.CouponBatch{}, mapMarketingError(err)
	}
	now := u.now()
	batch.Operator = operatorName(in.OperatorID)
	batch.CreatedAt = now
	batch.UpdatedAt = now
	created, err := u.store.CreateCouponBatch(ctx, batch)
	if err != nil {
		return domain.CouponBatch{}, mapMarketingError(err)
	}
	if err := u.store.RecordOperationLog(ctx, domain.OperationLog{
		Module:    "operations",
		Operator:  batch.Operator,
		Action:    "create_coupon_batch",
		Resource:  "coupon_batch",
		Target:    created.BatchNo,
		RequestID: in.RequestID,
		Detail:    fmt.Sprintf("创建券批次 %s 模板=%s 总量=%d 渠道=%s 预算=%d分", created.BatchNo, created.TemplateNo, created.TotalCount, created.Channel, created.BudgetFen),
		At:        now,
	}); err != nil {
		return domain.CouponBatch{}, err
	}
	return created, nil
}

// ExecuteBatchCommand 执行批次命令：stop 停止发放、finish 结束批次。
func (u *MarketingUsecase) ExecuteBatchCommand(ctx context.Context, batchNo, command string, operatorID int64, requestID string) (domain.CouponBatch, error) {
	batch, err := u.store.FindCouponBatch(ctx, strings.TrimSpace(batchNo))
	if err != nil {
		return domain.CouponBatch{}, mapMarketingError(err)
	}
	if batch.Status == domain.CouponBatchStateFinished || batch.Status == domain.CouponBatchStateStopped {
		return domain.CouponBatch{}, mapMarketingError(domain.ErrCouponBatchClosed)
	}
	now := u.now()
	var nextStatus string
	switch command {
	case "stop":
		nextStatus = domain.CouponBatchStateStopped
	case "finish":
		nextStatus = domain.CouponBatchStateFinished
	default:
		return domain.CouponBatch{}, mapMarketingError(fmt.Errorf("command %s: %w", command, ErrMarketingCommandUnknown))
	}
	updated, err := u.store.UpdateCouponBatch(ctx, batch.BatchNo, CouponBatchUpdate{Status: nextStatus, At: now})
	if err != nil {
		return domain.CouponBatch{}, mapMarketingError(err)
	}
	if err := u.store.RecordOperationLog(ctx, domain.OperationLog{
		Module:    "operations",
		Operator:  operatorName(operatorID),
		Action:    "coupon_batch_" + command,
		Resource:  "coupon_batch",
		Target:    updated.BatchNo,
		RequestID: requestID,
		Detail:    fmt.Sprintf("券批次 %s %s→%s 已发=%d/%d", updated.BatchNo, batch.Status, nextStatus, updated.IssuedCount, updated.TotalCount),
		At:        now,
	}); err != nil {
		return domain.CouponBatch{}, err
	}
	return updated, nil
}
