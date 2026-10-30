// A08 补贴活动的用例测试：配置校验、三项额度闸门与台账聚合。
package usecase

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/NSObjects/echo-admin/internal/modules/operations/domain"
)

// addSubsidyCampaign 造一个运行中的补贴活动：每单 300 分，每人最多 2 次，单日封顶 900 分。
func (f *marketingFixture) addSubsidyCampaign() domain.Campaign {
	campaign := domain.Campaign{
		Code:    "C-SUB-1",
		Name:    "平峰出车补贴",
		Type:    "period_subsidy",
		TargetAudience: "driver",
		StartAt: f.now.Add(-time.Hour),
		EndAt:   f.now.Add(24 * time.Hour),
		Status:  domain.CampaignStateRunning,
		BudgetFen:     2000,
		UsedBudgetFen: 0,
		SubsidyConfig: &domain.SubsidyConfig{
			TriggerType:  domain.SubsidyTriggerPerOrder,
			AmountFen:    300,
			MaxPerDriver: 2,
			DailyCapFen:  900,
		},
	}
	f.store.campaigns = append(f.store.campaigns, campaign)
	return campaign
}

func TestCreateCampaignRequiresSubsidyConfig(t *testing.T) {
	f := newMarketingFixture(t)
	// 补贴类活动不带配置：直接拒。
	_, err := f.uc.CreateCampaign(context.Background(), CreateCampaignInput{
		Code: "C-SUB-X", Name: "缺配置的补贴活动", Type: "period_subsidy",
		StartAt: f.now.Format(time.RFC3339), EndAt: f.now.Add(time.Hour).Format(time.RFC3339),
	})
	if !errors.Is(err, domain.ErrSubsidyNotConfigured) {
		t.Fatalf("want ErrSubsidyNotConfigured, got %v", err)
	}
	// 非补贴类活动带配置：同样拒，防误传。
	_, err = f.uc.CreateCampaign(context.Background(), CreateCampaignInput{
		Code: "C-SUB-Y", Name: "不该有配置的活动", Type: "new_user_gift",
		SubsidyConfig: &domain.SubsidyConfig{TriggerType: domain.SubsidyTriggerPerOrder, AmountFen: 100, MaxPerDriver: 1},
		StartAt: f.now.Format(time.RFC3339), EndAt: f.now.Add(time.Hour).Format(time.RFC3339),
	})
	if !errors.Is(err, domain.ErrSubsidyConfigInvalid) {
		t.Fatalf("want ErrSubsidyConfigInvalid, got %v", err)
	}
}

func TestGrantSubsidyWritesLedgerAndConsumesBudget(t *testing.T) {
	f := newMarketingFixture(t)
	f.addSubsidyCampaign()

	out, err := f.uc.GrantSubsidy(context.Background(), "C-SUB-1", GrantSubsidyInput{
		DriverID: "d1", BizType: domain.SubsidyBizTimeSubsidy, Reason: "平峰时段出车 3 小时", OperatorID: 1,
	})
	if err != nil {
		t.Fatalf("grant: %v", err)
	}
	// 金额只能来自配置，不能现场改。
	if out.Record.AmountFen != 300 {
		t.Fatalf("amount must come from config, got %d", out.Record.AmountFen)
	}
	if out.Record.RecordNo == "" || out.Record.Day != f.now.Format("2006-01-02") {
		t.Fatalf("record must carry no and day, got %+v", out.Record)
	}
	if f.store.campaigns[0].UsedBudgetFen != 300 {
		t.Fatalf("budget must be consumed, got %d", f.store.campaigns[0].UsedBudgetFen)
	}
	if out.BudgetRemainFen != 1700 {
		t.Fatalf("remain = %d, want 1700", out.BudgetRemainFen)
	}
	// 留痕：操作日志必须有一笔 grant_subsidy。
	if len(f.store.logs) == 0 || f.store.logs[len(f.store.logs)-1].Action != "grant_subsidy" {
		t.Fatalf("grant must be audited, logs=%+v", f.store.logs)
	}
}

func TestGrantSubsidyEnforcesDriverLimit(t *testing.T) {
	f := newMarketingFixture(t)
	f.addSubsidyCampaign() // MaxPerDriver=2

	for i := 0; i < 2; i++ {
		if _, err := f.uc.GrantSubsidy(context.Background(), "C-SUB-1", GrantSubsidyInput{
			DriverID: "d1", BizType: domain.SubsidyBizOther, Reason: "第 n 次补贴", OperatorID: 1,
		}); err != nil {
			t.Fatalf("grant #%d: %v", i+1, err)
		}
	}
	_, err := f.uc.GrantSubsidy(context.Background(), "C-SUB-1", GrantSubsidyInput{
		DriverID: "d1", BizType: domain.SubsidyBizOther, Reason: "第三次应被拒", OperatorID: 1,
	})
	if !errors.Is(err, domain.ErrSubsidyDriverLimit) {
		t.Fatalf("want ErrSubsidyDriverLimit, got %v", err)
	}
	// 换个司机不受影响。
	if _, err := f.uc.GrantSubsidy(context.Background(), "C-SUB-1", GrantSubsidyInput{
		DriverID: "d2", BizType: domain.SubsidyBizOther, Reason: "另一个司机", OperatorID: 1,
	}); err != nil {
		t.Fatalf("other driver should pass: %v", err)
	}
}

func TestGrantSubsidyEnforcesDailyCapAndRollsBackBudget(t *testing.T) {
	f := newMarketingFixture(t)
	f.addSubsidyCampaign() // 日封顶 900 = 3 笔

	// 三个不同司机各领一次：300*3 = 900，正好打满日封顶。
	for _, id := range []string{"d1", "d2", "d3"} {
		if _, err := f.uc.GrantSubsidy(context.Background(), "C-SUB-1", GrantSubsidyInput{
			DriverID: id, BizType: domain.SubsidyBizTimeSubsidy, Reason: "当日补贴", OperatorID: 1,
		}); err != nil {
			t.Fatalf("grant %s: %v", id, err)
		}
	}
	usedBefore := f.store.campaigns[0].UsedBudgetFen
	// 第 4 笔触发日封顶：预算占位必须原样退回，台账不能多出一笔。
	_, err := f.uc.GrantSubsidy(context.Background(), "C-SUB-1", GrantSubsidyInput{
		DriverID: "d4", BizType: domain.SubsidyBizTimeSubsidy, Reason: "超封顶", OperatorID: 1,
	})
	if !errors.Is(err, domain.ErrSubsidyDailyCapReached) {
		t.Fatalf("want ErrSubsidyDailyCapReached, got %v", err)
	}
	if f.store.campaigns[0].UsedBudgetFen != usedBefore {
		t.Fatalf("budget must roll back after daily cap, before=%d after=%d", usedBefore, f.store.campaigns[0].UsedBudgetFen)
	}
	if len(f.store.subsidies) != 3 {
		t.Fatalf("ledger must stay at 3, got %d", len(f.store.subsidies))
	}
}

func TestGrantSubsidyEnforcesBudgetAndState(t *testing.T) {
	f := newMarketingFixture(t)
	campaign := f.addSubsidyCampaign()
	// 预算只剩 200 分，不够一笔 300 分。
	f.store.campaigns[0].UsedBudgetFen = 1800
	_ = campaign

	_, err := f.uc.GrantSubsidy(context.Background(), "C-SUB-1", GrantSubsidyInput{
		DriverID: "d1", BizType: domain.SubsidyBizOther, Reason: "预算不足", OperatorID: 1,
	})
	if !errors.Is(err, domain.ErrSubsidyBudgetExceeded) {
		t.Fatalf("want ErrSubsidyBudgetExceeded, got %v", err)
	}
	// 活动暂停后不能再发。
	f.store.campaigns[0].UsedBudgetFen = 0
	f.store.campaigns[0].Status = domain.CampaignStatePaused
	if _, err := f.uc.GrantSubsidy(context.Background(), "C-SUB-1", GrantSubsidyInput{
		DriverID: "d1", BizType: domain.SubsidyBizOther, Reason: "暂停中", OperatorID: 1,
	}); !errors.Is(err, domain.ErrSubsidyCampaignNotRun) {
		t.Fatalf("want ErrSubsidyCampaignNotRun, got %v", err)
	}
	// 非补贴类活动不能当钱包用。
	f.store.campaigns[0].Status = domain.CampaignStateRunning
	f.store.campaigns[0].Type = "new_user_gift"
	if _, err := f.uc.GrantSubsidy(context.Background(), "C-SUB-1", GrantSubsidyInput{
		DriverID: "d1", BizType: domain.SubsidyBizOther, Reason: "普通活动", OperatorID: 1,
	}); !errors.Is(err, domain.ErrSubsidyNotConfigured) {
		t.Fatalf("want ErrSubsidyNotConfigured, got %v", err)
	}
}

func TestSubsidySummaryAggregatesLedger(t *testing.T) {
	f := newMarketingFixture(t)
	f.addSubsidyCampaign()
	for _, id := range []string{"d1", "d1", "d2"} {
		if _, err := f.uc.GrantSubsidy(context.Background(), "C-SUB-1", GrantSubsidyInput{
			DriverID: id, BizType: domain.SubsidyBizOther, Reason: "汇总测试", OperatorID: 1,
		}); err != nil {
			t.Fatalf("grant %s: %v", id, err)
		}
	}
	summary, err := f.uc.SubsidySummary(context.Background(), "C-SUB-1")
	if err != nil {
		t.Fatalf("summary: %v", err)
	}
	if summary.GrantedTotalFen != 900 || summary.RecordCount != 3 || summary.DriverCount != 2 {
		t.Fatalf("aggregate wrong: %+v", summary)
	}
	if summary.TodayGrantedFen != 900 || summary.BudgetRemainFen != 1100 {
		t.Fatalf("daily/budget wrong: %+v", summary)
	}
	if summary.TriggerType != domain.SubsidyTriggerPerOrder || summary.AmountFen != 300 {
		t.Fatalf("config echo wrong: %+v", summary)
	}
}
