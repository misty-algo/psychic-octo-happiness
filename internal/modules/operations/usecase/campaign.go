// A05 营销活动：启停、结束汇总对账与财务导出。
package usecase

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/NSObjects/echo-admin/internal/modules/operations/domain"
)

// ---- 活动 ----

// ListCampaigns 返回活动一页。
func (u *MarketingUsecase) ListCampaigns(ctx context.Context, in ListMarketingInput) (ListCampaignsOutput, error) {
	filter := CampaignFilter{
		Keyword: strings.TrimSpace(in.Keyword),
		Status:  strings.TrimSpace(in.Status),
		Type:    strings.TrimSpace(in.Type),
		Skip:    skipOf(in.Page, in.PageSize),
		Limit:   limitOf(in.PageSize),
	}
	items, total, err := u.store.ListCampaigns(ctx, filter)
	if err != nil {
		return ListCampaignsOutput{}, fmt.Errorf("list campaigns: %w", err)
	}
	if items == nil {
		items = []domain.Campaign{}
	}
	return ListCampaignsOutput{Items: items, Total: total}, nil
}

// CampaignDetail 返回单个活动。查不到要翻译成 404，不能裸抛成 500。
func (u *MarketingUsecase) CampaignDetail(ctx context.Context, code string) (domain.Campaign, error) {
	campaign, err := u.store.FindCampaign(ctx, strings.TrimSpace(code))
	if err != nil {
		return domain.Campaign{}, mapMarketingError(err)
	}
	return campaign, nil
}

// CreateCampaign 新建活动，落成草稿态。
func (u *MarketingUsecase) CreateCampaign(ctx context.Context, in CreateCampaignInput) (domain.Campaign, error) {
	startAt, err := parseTime(in.StartAt)
	if err != nil {
		return domain.Campaign{}, mapMarketingError(domain.ErrCampaignWindowInvalid)
	}
	endAt, err := parseTime(in.EndAt)
	if err != nil {
		return domain.Campaign{}, mapMarketingError(domain.ErrCampaignWindowInvalid)
	}
	if in.TargetAudience == "" {
		in.TargetAudience = "passenger"
	}
	campaign, err := domain.NewCampaign(in.Code, in.Name, in.Type, in.TargetAudience, in.Rules,
		in.RewardAmountFen, in.BudgetFen, startAt, endAt)
	if err != nil {
		return domain.Campaign{}, mapMarketingError(err)
	}
	// A08：补贴类活动必须携带合法的结构化补贴配置；非补贴类不接受补贴配置，
	// 否则前端漏传/误传都会造出一张发不出钱的空活动。
	if domain.IsSubsidyCampaignType(in.Type) {
		if in.SubsidyConfig == nil {
			return domain.Campaign{}, mapMarketingError(domain.ErrSubsidyNotConfigured)
		}
		if err := in.SubsidyConfig.Validate(); err != nil {
			return domain.Campaign{}, mapMarketingError(err)
		}
		// 预算至少要够发一笔，不然活动一上线就卡死。
		if in.BudgetFen > 0 && in.BudgetFen < in.SubsidyConfig.AmountFen {
			return domain.Campaign{}, mapMarketingError(domain.ErrCampaignBudgetInvalid)
		}
		campaign.SubsidyConfig = in.SubsidyConfig
	} else if in.SubsidyConfig != nil {
		return domain.Campaign{}, mapMarketingError(domain.ErrSubsidyConfigInvalid)
	}
	if len(in.CouponTemplates) > 0 {
		campaign.CouponTemplates = in.CouponTemplates
	}
	if len(in.CityCodes) > 0 {
		campaign.CityCodes = in.CityCodes
	}
	now := u.now()
	campaign.Operator = operatorName(in.OperatorID)
	campaign.CreatedAt = now
	campaign.UpdatedAt = now
	created, err := u.store.CreateCampaign(ctx, campaign)
	if err != nil {
		return domain.Campaign{}, mapMarketingError(err)
	}
	if err := u.store.RecordOperationLog(ctx, domain.OperationLog{
		Module:    "operations",
		Operator:  campaign.Operator,
		Action:    "create_campaign",
		Resource:  "campaign",
		Target:    created.Code,
		RequestID: in.RequestID,
		Detail:    fmt.Sprintf("创建活动 %s 类型=%s 预算=%d分 %s~%s", created.Name, created.Type, created.BudgetFen, created.StartAt.Format("2006-01-02"), created.EndAt.Format("2006-01-02")),
		At:        now,
	}); err != nil {
		return domain.Campaign{}, err
	}
	return created, nil
}

// ExecuteCampaignCommand 执行活动命令：start 开始、pause 暂停、end 结束。
// 结束是汇总对账的前提，只有 ended 的活动才能出结算单。
func (u *MarketingUsecase) ExecuteCampaignCommand(ctx context.Context, code, command string, operatorID int64, requestID string) (domain.Campaign, error) {
	campaign, err := u.store.FindCampaign(ctx, strings.TrimSpace(code))
	if err != nil {
		return domain.Campaign{}, mapMarketingError(err)
	}
	now := u.now()
	var nextStatus string
	switch command {
	case "start":
		if err := campaign.CanStart(); err != nil {
			return domain.Campaign{}, mapMarketingError(err)
		}
		if now.After(campaign.EndAt) {
			return domain.Campaign{}, mapMarketingError(domain.ErrCampaignWindowInvalid)
		}
		nextStatus = domain.CampaignStateRunning
	case "pause":
		if err := campaign.CanPause(); err != nil {
			return domain.Campaign{}, mapMarketingError(err)
		}
		nextStatus = domain.CampaignStatePaused
	case "end":
		if err := campaign.CanEnd(); err != nil {
			return domain.Campaign{}, mapMarketingError(err)
		}
		nextStatus = domain.CampaignStateEnded
	default:
		return domain.Campaign{}, mapMarketingError(fmt.Errorf("command %s: %w", command, ErrMarketingCommandUnknown))
	}
	updated, err := u.store.UpdateCampaign(ctx, campaign.Code, CampaignUpdate{
		Status:   nextStatus,
		Operator: operatorName(operatorID),
		At:       now,
	})
	if err != nil {
		return domain.Campaign{}, mapMarketingError(err)
	}
	if err := u.store.RecordOperationLog(ctx, domain.OperationLog{
		Module:    "operations",
		Operator:  operatorName(operatorID),
		Action:    "campaign_" + command,
		Resource:  "campaign",
		Target:    updated.Code,
		RequestID: requestID,
		Detail:    fmt.Sprintf("活动 %s %s→%s", updated.Name, campaign.Status, nextStatus),
		At:        now,
	}); err != nil {
		return domain.Campaign{}, err
	}
	return updated, nil
}

// ---- 汇总对账 ----

// CampaignSettlement 生成活动的结算汇总：发了多少、核销多少、谁承担、预算用掉多少。
// 这是财务口径的对账单，所有金额以分为单位计算，展示时再转元。
func (u *MarketingUsecase) CampaignSettlement(ctx context.Context, code string) (domain.CampaignSettlement, error) {
	campaign, err := u.store.FindCampaign(ctx, strings.TrimSpace(code))
	if err != nil {
		return domain.CampaignSettlement{}, mapMarketingError(err)
	}
	batches, err := u.store.ListBatchesByCampaign(ctx, campaign.Code)
	if err != nil {
		return domain.CampaignSettlement{}, fmt.Errorf("list campaign batches: %w", err)
	}
	settlement := domain.CampaignSettlement{
		CampaignCode: campaign.Code,
		CampaignName: campaign.Name,
		Status:       campaign.Status,
		StartAt:      campaign.StartAt,
		EndAt:        campaign.EndAt,
		Batches:      []domain.SettlementBatch{},
		BearerSplit:  map[string]int64{},
		BudgetFen:    campaign.BudgetFen,
		GeneratedAt:  u.now(),
	}
	if len(batches) == 0 {
		settlement.Closed = campaign.Status == domain.CampaignStateEnded
		return settlement, nil
	}
	batchNos := make([]string, 0, len(batches))
	for _, b := range batches {
		batchNos = append(batchNos, b.BatchNo)
	}
	stats, err := u.store.CountCouponsByBatch(ctx, batchNos)
	if err != nil {
		return domain.CampaignSettlement{}, fmt.Errorf("count coupons by batch: %w", err)
	}
	for _, b := range batches {
		stat := stats[b.BatchNo]
		template, err := u.store.FindCouponTemplate(ctx, b.TemplateNo)
		if err != nil {
			// 模板被删也要出对账单：面额按 0 记并留空名，不能让整张单子出不来。
			template = domain.CouponTemplate{TemplateNo: b.TemplateNo, Name: "", FaceValueFen: 0}
		}
		unused := stat.Issued - stat.Used - stat.Expired
		if unused < 0 {
			unused = 0
		}
		row := domain.SettlementBatch{
			BatchNo:        b.BatchNo,
			TemplateNo:     b.TemplateNo,
			TemplateName:   template.Name,
			Channel:        b.Channel,
			TotalCount:     b.TotalCount,
			IssuedCount:    stat.Issued,
			UsedCount:      stat.Used,
			UnusedCount:    unused,
			FaceValueFen:   template.FaceValueFen,
			IssueAmountFen: stat.Issued * template.FaceValueFen,
			UsedAmountFen:  stat.Used * template.FaceValueFen,
			Bearer:         template.Bearer,
			Status:         b.Status,
		}
		settlement.Batches = append(settlement.Batches, row)
		settlement.IssuedCount += stat.Issued
		settlement.UsedCount += stat.Used
		settlement.ExpiredCount += stat.Expired
		settlement.UnusedCount += unused
		settlement.IssueAmountFen += row.IssueAmountFen
		settlement.UsedAmountFen += row.UsedAmountFen
		settlement.BearerSplit[row.Bearer] += row.UsedAmountFen
	}
	// A08：补贴台账支出并入预算消耗。没有补贴配置的活动聚合返回 0，不影响券口径。
	if subsidyAgg, err := u.store.SummarizeSubsidies(ctx, campaign.Code, u.now().Format("2006-01-02")); err == nil {
		settlement.SubsidyGrantedFen = subsidyAgg.GrantedTotalFen
		settlement.SubsidyRecordCount = subsidyAgg.RecordCount
	} else {
		return domain.CampaignSettlement{}, fmt.Errorf("summarize subsidies for settlement: %w", err)
	}
	settlement.UsedBudgetFen = settlement.UsedAmountFen + campaign.RewardAmountFen + settlement.SubsidyGrantedFen
	settlement.BudgetUsagePct = percentOf(settlement.UsedBudgetFen, settlement.BudgetFen)
	settlement.Closed = campaign.Status == domain.CampaignStateEnded
	return settlement, nil
}

// SettlementRow 是导出用的一行，金额已转成元方便财务直接看。
type SettlementRow struct {
	CampaignCode    string
	CampaignName    string
	BatchNo         string
	TemplateNo      string
	TemplateName    string
	Channel         string
	Bearer          string
	TotalCount      int64
	IssuedCount     int64
	UsedCount       int64
	UnusedCount     int64
	ExpiredCount    int64
	FaceValueYuan   string
	IssueAmountYuan string
	UsedAmountYuan  string
}

// ExportSettlement 把对账单摊平成可导出的行。金额转元，不留"分"给财务。
func ExportSettlement(settlement domain.CampaignSettlement) []SettlementRow {
	rows := make([]SettlementRow, 0, len(settlement.Batches)+1)
	for _, b := range settlement.Batches {
		rows = append(rows, SettlementRow{
			CampaignCode:    settlement.CampaignCode,
			CampaignName:    settlement.CampaignName,
			BatchNo:         b.BatchNo,
			TemplateNo:      b.TemplateNo,
			TemplateName:    b.TemplateName,
			Channel:         b.Channel,
			Bearer:          b.Bearer,
			TotalCount:      b.TotalCount,
			IssuedCount:     b.IssuedCount,
			UsedCount:       b.UsedCount,
			UnusedCount:     b.UnusedCount,
			ExpiredCount:    0,
			FaceValueYuan:   yuan(b.FaceValueFen),
			IssueAmountYuan: yuan(b.IssueAmountFen),
			UsedAmountYuan:  yuan(b.UsedAmountFen),
		})
	}
	if len(rows) == 0 {
		// 没有任何批次也要给一行总计，避免导出空文件让财务以为接口坏了。
		rows = append(rows, SettlementRow{
			CampaignCode:    settlement.CampaignCode,
			CampaignName:    settlement.CampaignName,
			FaceValueYuan:   "0.00",
			IssueAmountYuan: "0.00",
			UsedAmountYuan:  "0.00",
		})
	}
	return rows
}

// SettlementCSV 把对账单渲染成 CSV，带 BOM 以便 Excel 正确识别 UTF-8。
func SettlementCSV(settlement domain.CampaignSettlement) string {
	rows := ExportSettlement(settlement)
	var sb strings.Builder
	sb.WriteString("\uFEFF")
	sb.WriteString("活动编码,活动名称,批次号,券模板,券名称,发放渠道,承担方,发行总量,已发放,已核销,未使用,券面额(元),发放金额(元),核销金额(元),补贴发放(元),补贴笔数\n")
	for _, r := range rows {
		sb.WriteString(fmt.Sprintf("%s,%s,%s,%s,%s,%s,%s,%d,%d,%d,%d,%s,%s,%s,,\n",
			csvCell(r.CampaignCode), csvCell(r.CampaignName), csvCell(r.BatchNo),
			csvCell(r.TemplateNo), csvCell(r.TemplateName), csvCell(r.Channel),
			csvCell(r.Bearer), r.TotalCount, r.IssuedCount, r.UsedCount, r.UnusedCount,
			r.FaceValueYuan, r.IssueAmountYuan, r.UsedAmountYuan))
	}
	sb.WriteString(fmt.Sprintf("合计,,,,,,,,%d,%d,%d,%d,,%s,%s,%s,%d\n",
		settlement.IssuedCount, settlement.UsedCount, settlement.UnusedCount, settlement.ExpiredCount,
		yuan(settlement.IssueAmountFen), yuan(settlement.UsedAmountFen),
		yuan(settlement.SubsidyGrantedFen), settlement.SubsidyRecordCount))
	return sb.String()
}

// csvCell 转义 CSV 单元格：含逗号、引号或换行时加引号。
func csvCell(value string) string {
	if strings.ContainsAny(value, ",\"\n") {
		return `"` + strings.ReplaceAll(value, `"`, `""`) + `"`
	}
	return value
}

// SettlementWindow 是对账的时间口径，供前端展示。
type SettlementWindow struct {
	StartAt time.Time `json:"start_at"`
	EndAt   time.Time `json:"end_at"`
	Closed  bool      `json:"closed"`
}
