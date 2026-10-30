// A05/A06/A09 配置类用例测试：公式守恒、权重校验、重叠发布与拼车约束。
package usecase

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/NSObjects/echo-admin/internal/modules/operations/domain"
)

func newPricingFixture(t *testing.T) (*marketingFixture, *PricingUsecase, *DispatchUsecase, *PoolUsecase) {
	t.Helper()
	f := newMarketingFixture(t)
	return f, NewPricing(f.store), NewDispatch(f.store), NewPool(f.store)
}

// ---- A05 计价规则 ----

func TestPricingFareFormulaIsConserved(t *testing.T) {
	rule := domain.PricingRule{
		StartFen: 800, IncludeM: 3000, PerKmFen: 200, PerMinFen: 50,
		ServiceBp: 500, CommBp: 2000, SurgeBp: 12000, DiscountBp: 8000,
	}
	samples := []domain.FareInput{
		{DistM: 2000, DurS: 300},
		{DistM: 12000, DurS: 1800, CouponFen: 300},
		{DistM: 35000, DurS: 4200, CouponFen: 500, SubsidyFen: 200},
	}
	for _, s := range samples {
		fare := rule.ComputeFare(s)
		// 资金守恒：P = D + N + G，且 G 必须为 0。
		if fare.PFen != fare.DFen+fare.NFen+fare.GFen {
			t.Fatalf("not conserved: P=%d D=%d N=%d G=%d sample=%+v", fare.PFen, fare.DFen, fare.NFen, fare.GFen, s)
		}
		if fare.GFen != 0 {
			t.Fatalf("G must be 0, got %d sample=%+v", fare.GFen, s)
		}
		if fare.PFen < 0 || fare.DFen < 0 {
			t.Fatalf("negative money: %+v sample=%+v", fare, s)
		}
	}
	// 固定价方案：不按里程时长算。
	fixed := rule
	fixed.FixedFen = 1500
	if got := fixed.ComputeFare(domain.FareInput{DistM: 35000, DurS: 4200}); got.TFen != 1500 {
		t.Fatalf("fixed price must win, got T=%d", got.TFen)
	}
	// 拼车走折扣：同样输入下拼车更便宜。
	normal := rule.ComputeFare(domain.FareInput{DistM: 12000, DurS: 1800})
	pooled := rule.ComputeFare(domain.FareInput{DistM: 12000, DurS: 1800, Pooled: true})
	if pooled.QFen >= normal.QFen {
		t.Fatalf("pooled must be cheaper: pooled=%d normal=%d", pooled.QFen, normal.QFen)
	}
}

func TestCreateAndPublishPricingRule(t *testing.T) {
	_, pricing, _, _ := newPricingFixture(t)
	ctx := context.Background()
	rule, err := pricing.CreateRule(ctx, CreatePricingRuleInput{
		CityCode: "310100", CarType: "economy", StartFen: 800, IncludeM: 3000,
		PerKmFen: 200, PerMinFen: 50, ServiceBp: 500, CommBp: 2000, SurgeBp: 10000,
		ValidFrom: "2026-10-01", ValidTo: "2026-12-31",
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if rule.State != domain.PricingRuleStateDraft {
		t.Fatalf("new rule must be draft, got %s", rule.State)
	}
	published, err := pricing.PublishRule(ctx, rule.RuleID)
	if err != nil {
		t.Fatalf("publish: %v", err)
	}
	if published.State != domain.PricingRuleStatePublished {
		t.Fatalf("published state wrong: %s", published.State)
	}
	// 已发布版本不能重复发布。
	if _, err := pricing.PublishRule(ctx, rule.RuleID); !errors.Is(err, domain.ErrPricingDraftOnly) {
		t.Fatalf("want ErrPricingDraftOnly, got %v", err)
	}
}

func TestPublishPricingRuleRejectsOverlap(t *testing.T) {
	f, pricing, _, _ := newPricingFixture(t)
	ctx := context.Background()
	first, err := pricing.CreateRule(ctx, CreatePricingRuleInput{
		CityCode: "310100", CarType: "economy", SurgeBp: 10000,
		ValidFrom: "2026-10-01", ValidTo: "2026-12-31",
	})
	if err != nil {
		t.Fatalf("create first: %v", err)
	}
	if _, err := pricing.PublishRule(ctx, first.RuleID); err != nil {
		t.Fatalf("publish first: %v", err)
	}
	// 同城市车型、同优先级、时间重叠的第二条必须被拒。
	second, err := pricing.CreateRule(ctx, CreatePricingRuleInput{
		CityCode: "310100", CarType: "economy", SurgeBp: 10000,
		ValidFrom: "2026-12-01", ValidTo: "2027-03-31",
	})
	if err != nil {
		t.Fatalf("create second: %v", err)
	}
	if _, err := pricing.PublishRule(ctx, second.RuleID); !errors.Is(err, domain.ErrPricingOverlap) {
		t.Fatalf("want ErrPricingOverlap, got %v", err)
	}
	_ = f
}

func TestEffectivePricingRuleMissingIsReported(t *testing.T) {
	f, pricing, _, _ := newPricingFixture(t)
	// 库里没有任何规则：必须明确报「无有效价」，不能悄悄返回空规则。
	_, err := pricing.EffectiveRule(context.Background(), "310100", "economy", f.now)
	if !errors.Is(err, domain.ErrPricingNoEffective) {
		t.Fatalf("want ErrPricingNoEffective, got %v", err)
	}
}

func TestPricingRejectsBadBpAndWindow(t *testing.T) {
	_, pricing, _, _ := newPricingFixture(t)
	ctx := context.Background()
	// 溢价超上限
	if _, err := pricing.CreateRule(ctx, CreatePricingRuleInput{
		CityCode: "310100", CarType: "economy", SurgeBp: 999999,
		ValidFrom: "2026-10-01", ValidTo: "2026-12-31",
	}); !errors.Is(err, domain.ErrPricingBpInvalid) {
		t.Fatalf("want ErrPricingBpInvalid, got %v", err)
	}
	// 时间反了
	if _, err := pricing.CreateRule(ctx, CreatePricingRuleInput{
		CityCode: "310100", CarType: "economy", SurgeBp: 10000,
		ValidFrom: "2026-12-31", ValidTo: "2026-10-01",
	}); !errors.Is(err, domain.ErrPricingWindowInvalid) {
		t.Fatalf("want ErrPricingWindowInvalid, got %v", err)
	}
}

// ---- A06 派单策略 ----

func TestDispatchWeightsMustSumTo10000(t *testing.T) {
	f, _, dispatch, _ := newPricingFixture(t)
	ctx := context.Background()
	// 权重总和不是 10000：拒绝。
	_, err := dispatch.CreateStrategy(ctx, CreateDispatchInput{
		Weights: domain.DispatchWeights{IncomeBp: 5000, DistanceBp: 2000},
		RadiusM: 5000, InviteSec: 15, TotalSec: 120, ListenSec: 60, GrabSec: 60,
		Redispatch: domain.DispatchRedispatch{ExtraWindowSec: 60, MaxAcceptedReturn: 2},
	})
	if !errors.Is(err, domain.ErrDispatchWeightInvalid) {
		t.Fatalf("want ErrDispatchWeightInvalid, got %v", err)
	}
	// 听单 + 抢单 ≠ 总窗口：拒绝。
	_, err = dispatch.CreateStrategy(ctx, CreateDispatchInput{
		Weights: domain.DispatchWeights{IncomeBp: 4000, DistanceBp: 3000, RatingBp: 1500, OnlineSecBp: 1000, SuccessRateBp: 500},
		RadiusM: 5000, InviteSec: 15, TotalSec: 120, ListenSec: 60, GrabSec: 30,
		Redispatch: domain.DispatchRedispatch{ExtraWindowSec: 60, MaxAcceptedReturn: 2},
	})
	if !errors.Is(err, domain.ErrDispatchWindowInvalid) {
		t.Fatalf("want ErrDispatchWindowInvalid, got %v", err)
	}
	// 合法策略可以创建并发布。
	strategy, err := dispatch.CreateStrategy(ctx, CreateDispatchInput{
		Weights: domain.DispatchWeights{IncomeBp: 4000, DistanceBp: 3000, RatingBp: 1500, OnlineSecBp: 1000, SuccessRateBp: 500},
		RadiusM: 5000, InviteSec: 15, TotalSec: 120, ListenSec: 60, GrabSec: 60,
		Redispatch: domain.DispatchRedispatch{ExtraWindowSec: 60, MaxAcceptedReturn: 2},
	})
	if err != nil {
		t.Fatalf("create strategy: %v", err)
	}
	if _, err := dispatch.PublishStrategy(ctx, strategy.StrategyID); err != nil {
		t.Fatalf("publish strategy: %v", err)
	}
	if _, err := dispatch.PublishStrategy(ctx, strategy.StrategyID); !errors.Is(err, domain.ErrDispatchDraftOnly) {
		t.Fatalf("want ErrDispatchDraftOnly, got %v", err)
	}
	_ = f
}

func TestDispatchRankingPrefersLowerIncomeThenScore(t *testing.T) {
	f, _, dispatch, _ := newPricingFixture(t)
	strategy, err := dispatch.CreateStrategy(context.Background(), CreateDispatchInput{
		Weights: domain.DispatchWeights{IncomeBp: 10000},
		RadiusM: 5000, InviteSec: 15, TotalSec: 120, ListenSec: 60, GrabSec: 60,
		Redispatch: domain.DispatchRedispatch{ExtraWindowSec: 60, MaxAcceptedReturn: 2},
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	ranked, err := dispatch.PreviewRanking(context.Background(), strategy.StrategyID, nil)
	if err != nil {
		t.Fatalf("preview: %v", err)
	}
	if len(ranked) < 2 {
		t.Fatalf("want candidates, got %d", len(ranked))
	}
	// 收入低的排前面。
	if ranked[0].Driver.IncomeFen > ranked[1].Driver.IncomeFen {
		t.Fatalf("lower income must rank first: %+v", ranked)
	}
	_ = f
}

// ---- A09 拼车与特价 ----

func TestPoolConfigEnforcesGroupBounds(t *testing.T) {
	f, _, _, pool := newPricingFixture(t)
	ctx := context.Background()
	// 每组最多 2 单 4 座。
	if _, err := pool.CreateConfig(ctx, CreatePoolInput{MatchSec: 60, GroupMax: 3, SeatMax: 4}); !errors.Is(err, domain.ErrPoolGroupMaxInvalid) {
		t.Fatalf("want ErrPoolGroupMaxInvalid, got %v", err)
	}
	// 匹配等待不能超过 180 秒。
	if _, err := pool.CreateConfig(ctx, CreatePoolInput{MatchSec: 300, GroupMax: 2, SeatMax: 4}); !errors.Is(err, domain.ErrPoolMatchSecInvalid) {
		t.Fatalf("want ErrPoolMatchSecInvalid, got %v", err)
	}
	config, err := pool.CreateConfig(ctx, CreatePoolInput{MatchSec: 60, GroupMax: 2, SeatMax: 4, CityCodes: []string{"310100"}})
	if err != nil {
		t.Fatalf("create config: %v", err)
	}
	if _, err := pool.PublishConfig(ctx, config.ConfigID); err != nil {
		t.Fatalf("publish config: %v", err)
	}
	if _, err := pool.PublishConfig(ctx, config.ConfigID); !errors.Is(err, domain.ErrPoolDraftOnly) {
		t.Fatalf("want ErrPoolDraftOnly, got %v", err)
	}
	_ = f
}

func TestSpecialPricePlanValidation(t *testing.T) {
	f, _, _, pool := newPricingFixture(t)
	ctx := context.Background()
	if _, err := pool.CreatePlan(ctx, CreatePlanInput{
		Name: "机场特价", CityCode: "310100", CarType: "economy", FixedFen: 0,
		ValidFrom: "2026-10-01", ValidTo: "2026-12-31",
	}); !errors.Is(err, domain.ErrPoolPlanFixedInvalid) {
		t.Fatalf("want ErrPoolPlanFixedInvalid, got %v", err)
	}
	plan, err := pool.CreatePlan(ctx, CreatePlanInput{
		Name: "机场特价", CityCode: "310100", CarType: "economy", FixedFen: 3900,
		ValidFrom: "2026-10-01", ValidTo: "2026-12-31",
	})
	if err != nil {
		t.Fatalf("create plan: %v", err)
	}
	if plan.PlanID == "" {
		t.Fatal("plan must carry an id")
	}
	_ = f
	_ = time.Now
}
