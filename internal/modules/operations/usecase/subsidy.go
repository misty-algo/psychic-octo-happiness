// A08 补贴活动：登记发放、预算/日封顶/每人上限约束与补贴口径汇总。
// 补贴金额由活动配置决定，登记时只认配置，不允许现场改金额。
package usecase

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"strings"
	"time"

	"github.com/NSObjects/echo-admin/internal/modules/operations/domain"
)

// GrantSubsidyInput 是登记一笔补贴发放的请求。
type GrantSubsidyInput struct {
	DriverID  string `json:"driver_id"`
	OrderNo   string `json:"order_no,omitempty"`
	BizType   string `json:"biz_type"`
	Reason    string `json:"reason"`
	OperatorID int64
	RequestID string
}

// GrantSubsidyOutput 是登记结果：台账记录 + 活动剩余预算。
type GrantSubsidyOutput struct {
	Record          domain.SubsidyRecord `json:"record"`
	BudgetRemainFen int64                `json:"budget_remain_fen"`
}

// GrantSubsidy 给司机登记一笔补贴。金额取自活动补贴配置，三项额度全部原子校验：
// 活动预算、活动当日封顶、司机累计次数。任何一项不够都不落账。
func (u *MarketingUsecase) GrantSubsidy(ctx context.Context, code string, in GrantSubsidyInput) (GrantSubsidyOutput, error) {
	campaign, err := u.store.FindCampaign(ctx, strings.TrimSpace(code))
	if err != nil {
		return GrantSubsidyOutput{}, mapMarketingError(err)
	}
	// 只有补贴类活动且带了结构化配置才能发钱，防的是"拿普通活动当钱包用"。
	if !domain.IsSubsidyCampaignType(campaign.Type) || campaign.SubsidyConfig == nil {
		return GrantSubsidyOutput{}, mapMarketingError(domain.ErrSubsidyNotConfigured)
	}
	now := u.now()
	if campaign.Status != domain.CampaignStateRunning || now.After(campaign.EndAt) {
		return GrantSubsidyOutput{}, mapMarketingError(domain.ErrSubsidyCampaignNotRun)
	}
	config := *campaign.SubsidyConfig
	record, err := domain.NewSubsidyRecord(newSubsidyRecordNo(now), campaign.Code, campaign.Name,
		in.DriverID, in.OrderNo, config.AmountFen, in.BizType, in.Reason, now.Format("2006-01-02"), now)
	if err != nil {
		return GrantSubsidyOutput{}, mapMarketingError(err)
	}
	// 每个司机的累计次数上限：命中就直接拒绝，不做部分发放。
	granted, err := u.store.CountSubsidiesByDriver(ctx, campaign.Code, record.DriverID)
	if err != nil {
		return GrantSubsidyOutput{}, fmt.Errorf("count subsidies by driver: %w", err)
	}
	if granted >= config.MaxPerDriver {
		return GrantSubsidyOutput{}, mapMarketingError(domain.ErrSubsidyDriverLimit)
	}
	// 预算是活动的钱袋子：先原子占住，后面日封顶失败要原样退回。
	ok, err := u.store.ReserveCampaignBudget(ctx, campaign.Code, config.AmountFen)
	if err != nil {
		return GrantSubsidyOutput{}, fmt.Errorf("reserve campaign budget: %w", err)
	}
	if !ok {
		return GrantSubsidyOutput{}, mapMarketingError(domain.ErrSubsidyBudgetExceeded)
	}
	if config.DailyCapFen > 0 {
		dailyOK, err := u.store.ReserveSubsidyDaily(ctx, campaign.Code, record.Day, config.DailyCapFen, config.AmountFen)
		if err != nil {
			if releaseErr := u.store.ReleaseCampaignBudget(ctx, campaign.Code, config.AmountFen); releaseErr != nil {
				return GrantSubsidyOutput{}, fmt.Errorf("release campaign budget after daily cap error: %w (release: %v)", err, releaseErr)
			}
			return GrantSubsidyOutput{}, fmt.Errorf("reserve subsidy daily: %w", err)
		}
		if !dailyOK {
			if releaseErr := u.store.ReleaseCampaignBudget(ctx, campaign.Code, config.AmountFen); releaseErr != nil {
				return GrantSubsidyOutput{}, fmt.Errorf("daily cap reached and release budget: %w (release: %v)", domain.ErrSubsidyDailyCapReached, releaseErr)
			}
			return GrantSubsidyOutput{}, mapMarketingError(domain.ErrSubsidyDailyCapReached)
		}
	}
	record.Operator = operatorName(in.OperatorID)
	if err := u.store.InsertSubsidyRecord(ctx, record); err != nil {
		return GrantSubsidyOutput{}, fmt.Errorf("insert subsidy record: %w", err)
	}
	remain := campaign.BudgetFen - (campaign.UsedBudgetFen + config.AmountFen)
	if err := u.store.RecordOperationLog(ctx, domain.OperationLog{
		Module:    "operations",
		Operator:  record.Operator,
		Action:    "grant_subsidy",
		Resource:  "campaign_subsidy",
		Target:    record.RecordNo,
		RequestID: in.RequestID,
		Detail: fmt.Sprintf("活动 %s 补贴 %d分 → 司机 %s（%s）事由:%s",
			campaign.Code, record.AmountFen, record.DriverID, record.BizType, record.Reason),
		At: now,
	}); err != nil {
		return GrantSubsidyOutput{}, err
	}
	return GrantSubsidyOutput{Record: record, BudgetRemainFen: remain}, nil
}

// ListSubsidyRecords 返回补贴台账一页。
func (u *MarketingUsecase) ListSubsidyRecords(ctx context.Context, in ListMarketingInput) (ListSubsidyRecordsOutput, error) {
	filter := SubsidyRecordFilter{
		CampaignCode: strings.TrimSpace(in.CampaignCode),
		DriverID:     strings.TrimSpace(in.DriverID),
		BizType:      strings.TrimSpace(in.BizType),
		Day:          strings.TrimSpace(in.Day),
		Skip:         skipOf(in.Page, in.PageSize),
		Limit:        limitOf(in.PageSize),
	}
	items, total, err := u.store.ListSubsidyRecords(ctx, filter)
	if err != nil {
		return ListSubsidyRecordsOutput{}, fmt.Errorf("list subsidy records: %w", err)
	}
	if items == nil {
		items = []domain.SubsidyRecord{}
	}
	return ListSubsidyRecordsOutput{Items: items, Total: total}, nil
}

// SubsidySummary 返回补贴口径汇总：预算、累计、当日进度与触达人数。
func (u *MarketingUsecase) SubsidySummary(ctx context.Context, code string) (domain.CampaignSubsidySummary, error) {
	campaign, err := u.store.FindCampaign(ctx, strings.TrimSpace(code))
	if err != nil {
		return domain.CampaignSubsidySummary{}, mapMarketingError(err)
	}
	agg, err := u.store.SummarizeSubsidies(ctx, campaign.Code, u.now().Format("2006-01-02"))
	if err != nil {
		return domain.CampaignSubsidySummary{}, fmt.Errorf("summarize subsidies: %w", err)
	}
	summary := domain.CampaignSubsidySummary{
		CampaignCode:    campaign.Code,
		CampaignName:    campaign.Name,
		Status:          campaign.Status,
		BudgetFen:       campaign.BudgetFen,
		UsedBudgetFen:   campaign.UsedBudgetFen,
		BudgetRemainFen: campaign.BudgetFen - campaign.UsedBudgetFen,
		GrantedTotalFen: agg.GrantedTotalFen,
		RecordCount:     agg.RecordCount,
		DriverCount:     agg.DriverCount,
		TodayGrantedFen: agg.TodayGrantedFen,
		Today:           u.now().Format("2006-01-02"),
	}
	if cfg := campaign.SubsidyConfig; cfg != nil {
		summary.TriggerType = cfg.TriggerType
		summary.AmountFen = cfg.AmountFen
		summary.MaxPerDriver = cfg.MaxPerDriver
		summary.DailyCapFen = cfg.DailyCapFen
	}
	// 预算用到 80% 就告警，100% 停止发放：预算耗尽必须是提前发现，不是事后补救。
	summary.BudgetWarn = domain.BudgetWarn(campaign.UsedBudgetFen, campaign.BudgetFen)
	summary.BudgetUsagePct = domain.BudgetUsagePct(campaign.UsedBudgetFen, campaign.BudgetFen)
	return summary, nil
}

// ReverseSubsidyInput 是追回一笔补贴的请求。
type ReverseSubsidyInput struct {
	Reason     string `json:"reason"`
	OperatorID int64
	RequestID  string
}

// ReverseSubsidy 追回一笔补贴：虚假订单或全额退款时冲正。
// 台账只追加不修改，所以追回是给原记录打标记 + 把预算退回去，
// 这样财务能看到"发了多少、追回多少"，而不是一笔被悄悄抹掉。
func (u *MarketingUsecase) ReverseSubsidy(ctx context.Context, code string, recordNo string, in ReverseSubsidyInput) (domain.SubsidyRecord, error) {
	if strings.TrimSpace(in.Reason) == "" {
		return domain.SubsidyRecord{}, mapMarketingError(domain.ErrSubsidyReversalReason)
	}
	campaign, err := u.store.FindCampaign(ctx, strings.TrimSpace(code))
	if err != nil {
		return domain.SubsidyRecord{}, mapMarketingError(err)
	}
	record, err := u.store.FindSubsidyRecord(ctx, strings.TrimSpace(recordNo))
	if err != nil {
		return domain.SubsidyRecord{}, mapMarketingError(err)
	}
	if record.CampaignCode != campaign.Code {
		return domain.SubsidyRecord{}, mapMarketingError(domain.ErrSubsidyRecordNotFound)
	}
	if !record.Voidable() {
		return domain.SubsidyRecord{}, mapMarketingError(domain.ErrSubsidyAlreadyReversed)
	}
	now := u.now()
	reversed := record
	reversed.Reversed = true
	reversed.ReversedAt = now
	reversed.ReversedBy = operatorName(in.OperatorID)
	reversed.ReverseReason = in.Reason
	// 预算退回：追回的钱要能再发给别人，不然活动预算凭空少一块。
	if err := u.store.ReleaseCampaignBudget(ctx, campaign.Code, record.AmountFen); err != nil {
		return domain.SubsidyRecord{}, fmt.Errorf("release budget on reversal: %w", err)
	}
	if err := u.store.MarkSubsidyReversed(ctx, reversed); err != nil {
		return domain.SubsidyRecord{}, fmt.Errorf("mark subsidy reversed: %w", err)
	}
	if err := u.store.RecordOperationLog(ctx, domain.OperationLog{
		Module:    "operations",
		Operator:  reversed.ReversedBy,
		Action:    "reverse_subsidy",
		Resource:  "campaign_subsidy",
		Target:    record.RecordNo,
		RequestID: in.RequestID,
		Detail: fmt.Sprintf("活动 %s 追回补贴 %d分（司机 %s）事由:%s",
			campaign.Code, record.AmountFen, record.DriverID, in.Reason),
		At: now,
	}); err != nil {
		return domain.SubsidyRecord{}, err
	}
	return reversed, nil
}

// newSubsidyRecordNo 生成补贴流水号：SR + 日期 + 8 位随机 hex，格式对齐券码。
func newSubsidyRecordNo(now time.Time) string {
	buf := make([]byte, 4)
	if _, err := rand.Read(buf[:]); err != nil {
		// 随机源失败是基础设施故障，宁可 panic 也不发一笔没有唯一号的补贴。
		panic(fmt.Sprintf("generate subsidy record no: %v", err))
	}
	return "SR" + now.Format("20060102") + strings.ToUpper(hex.EncodeToString(buf[:]))
}

// ListSubsidyRecordsOutput 是补贴台账一页。
type ListSubsidyRecordsOutput struct {
	Items []domain.SubsidyRecord `json:"records"`
	Total int64                  `json:"total"`
}
