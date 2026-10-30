package usecase

import (
	"context"
	"strings"
	"time"

	"github.com/NSObjects/echo-admin/internal/modules/operations/domain"
)

// ---- 计价规则（A05）的内存实现 ----

func (f *fakeMarketingStore) NextPricingVersion(_ context.Context, cityCode, carType, area string) (int64, error) {
	var max int64
	for _, r := range f.pricingRules {
		if r.CityCode == cityCode && r.CarType == carType && r.Area == area && r.Version > max {
			max = r.Version
		}
	}
	return max + 1, nil
}

func (f *fakeMarketingStore) CreatePricingRule(_ context.Context, rule domain.PricingRule) (domain.PricingRule, error) {
	if rule.RuleID == "" {
		rule.RuleID = "PR-" + strings.ToUpper(rule.CityCode) + "-" + strings.ToUpper(rule.CarType) + "-V" + itoa(rule.Version)
	}
	f.pricingRules = append(f.pricingRules, rule)
	return rule, nil
}

func (f *fakeMarketingStore) FindPricingRule(_ context.Context, ruleID string) (domain.PricingRule, error) {
	for _, r := range f.pricingRules {
		if r.RuleID == strings.TrimSpace(ruleID) {
			return r, nil
		}
	}
	return domain.PricingRule{}, domain.ErrPricingRuleNotFound
}

func (f *fakeMarketingStore) ListPricingRules(_ context.Context, filter PricingRuleFilter) ([]domain.PricingRule, int64, error) {
	out := make([]domain.PricingRule, 0)
	for _, r := range f.pricingRules {
		if filter.CityCode != "" && r.CityCode != filter.CityCode {
			continue
		}
		if filter.CarType != "" && r.CarType != filter.CarType {
			continue
		}
		if filter.State != "" && r.State != filter.State {
			continue
		}
		out = append(out, r)
	}
	return out, int64(len(out)), nil
}

func (f *fakeMarketingStore) FindPricingOverlap(_ context.Context, rule domain.PricingRule) (*domain.PricingRule, error) {
	for i := range f.pricingRules {
		if f.pricingRules[i].State == domain.PricingRuleStatePublished && rule.Overlaps(f.pricingRules[i]) {
			found := f.pricingRules[i]
			return &found, nil
		}
	}
	return nil, nil
}

func (f *fakeMarketingStore) PublishPricingRule(_ context.Context, ruleID string) (domain.PricingRule, error) {
	return f.setPricingState(ruleID, domain.PricingRuleStatePublished)
}

func (f *fakeMarketingStore) DisablePricingRule(_ context.Context, ruleID string) (domain.PricingRule, error) {
	return f.setPricingState(ruleID, domain.PricingRuleStateDisabled)
}

func (f *fakeMarketingStore) setPricingState(ruleID, state string) (domain.PricingRule, error) {
	for i := range f.pricingRules {
		if f.pricingRules[i].RuleID == ruleID {
			f.pricingRules[i].State = state
			return f.pricingRules[i], nil
		}
	}
	return domain.PricingRule{}, domain.ErrPricingRuleNotFound
}

func (f *fakeMarketingStore) FindEffectivePricingRule(_ context.Context, cityCode, carType string, at time.Time) (domain.PricingRule, error) {
	var best *domain.PricingRule
	for i := range f.pricingRules {
		r := f.pricingRules[i]
		if r.CityCode != cityCode || r.CarType != carType {
			continue
		}
		if !r.EffectiveAt(at) {
			continue
		}
		if best == nil || r.Priority > best.Priority || (r.Priority == best.Priority && r.Version > best.Version) {
			best = &f.pricingRules[i]
		}
	}
	if best == nil {
		return domain.PricingRule{}, domain.ErrPricingNoEffective
	}
	return *best, nil
}

// ---- 派单策略（A06）的内存实现 ----

func (f *fakeMarketingStore) NextDispatchVersion(_ context.Context) (int64, error) {
	var max int64
	for _, s := range f.strategies {
		if s.Version > max {
			max = s.Version
		}
	}
	return max + 1, nil
}

func (f *fakeMarketingStore) CreateDispatchStrategy(_ context.Context, strategy domain.DispatchStrategy) (domain.DispatchStrategy, error) {
	if strategy.StrategyID == "" {
		strategy.StrategyID = "DS-V" + itoa(strategy.Version)
	}
	f.strategies = append(f.strategies, strategy)
	return strategy, nil
}

func (f *fakeMarketingStore) FindDispatchStrategy(_ context.Context, strategyID string) (domain.DispatchStrategy, error) {
	for _, s := range f.strategies {
		if s.StrategyID == strings.TrimSpace(strategyID) {
			return s, nil
		}
	}
	return domain.DispatchStrategy{}, domain.ErrDispatchNotFound
}

func (f *fakeMarketingStore) ListDispatchStrategies(_ context.Context, filter DispatchFilter) ([]domain.DispatchStrategy, int64, error) {
	out := make([]domain.DispatchStrategy, 0)
	for _, s := range f.strategies {
		if filter.State != "" && s.State != filter.State {
			continue
		}
		out = append(out, s)
	}
	return out, int64(len(out)), nil
}

func (f *fakeMarketingStore) PublishDispatchStrategy(_ context.Context, strategyID string) (domain.DispatchStrategy, error) {
	for i := range f.strategies {
		if f.strategies[i].StrategyID == strategyID {
			f.strategies[i].State = domain.DispatchStatePublished
			return f.strategies[i], nil
		}
	}
	return domain.DispatchStrategy{}, domain.ErrDispatchNotFound
}

func (f *fakeMarketingStore) DisableDispatchStrategy(_ context.Context, strategyID string) (domain.DispatchStrategy, error) {
	for i := range f.strategies {
		if f.strategies[i].StrategyID == strategyID {
			f.strategies[i].State = domain.DispatchStateDisabled
			return f.strategies[i], nil
		}
	}
	return domain.DispatchStrategy{}, domain.ErrDispatchNotFound
}

func (f *fakeMarketingStore) ListDispatchCandidates(_ context.Context, _ int) ([]domain.DispatchCandidate, error) {
	return []domain.DispatchCandidate{
		{DriverID: "drv-1", IncomeFen: 30000, DistanceM: 800, Rating: 95, OnlineSec: 7200, SuccessRate: 90},
		{DriverID: "drv-2", IncomeFen: 12000, DistanceM: 1500, Rating: 88, OnlineSec: 3600, SuccessRate: 85},
		{DriverID: "drv-3", IncomeFen: 12000, DistanceM: 600, Rating: 92, OnlineSec: 5400, SuccessRate: 95},
	}, nil
}

// ---- 拼车与特价（A09）的内存实现 ----

func (f *fakeMarketingStore) NextPoolVersion(_ context.Context) (int64, error) {
	var max int64
	for _, c := range f.poolConfigs {
		if c.Version > max {
			max = c.Version
		}
	}
	return max + 1, nil
}

func (f *fakeMarketingStore) CreatePoolConfig(_ context.Context, config domain.PoolConfig) (domain.PoolConfig, error) {
	if config.ConfigID == "" {
		config.ConfigID = "POOL-V" + itoa(config.Version)
	}
	f.poolConfigs = append(f.poolConfigs, config)
	return config, nil
}

func (f *fakeMarketingStore) FindPoolConfig(_ context.Context, configID string) (domain.PoolConfig, error) {
	for _, c := range f.poolConfigs {
		if c.ConfigID == strings.TrimSpace(configID) {
			return c, nil
		}
	}
	return domain.PoolConfig{}, domain.ErrPoolConfigNotFound
}

func (f *fakeMarketingStore) ListPoolConfigs(_ context.Context, filter PoolFilter) ([]domain.PoolConfig, int64, error) {
	out := make([]domain.PoolConfig, 0)
	for _, c := range f.poolConfigs {
		if filter.State != "" && c.State != filter.State {
			continue
		}
		out = append(out, c)
	}
	return out, int64(len(out)), nil
}

func (f *fakeMarketingStore) PublishPoolConfig(_ context.Context, configID string) (domain.PoolConfig, error) {
	for i := range f.poolConfigs {
		if f.poolConfigs[i].ConfigID == configID {
			f.poolConfigs[i].State = domain.PoolConfigStatePublished
			return f.poolConfigs[i], nil
		}
	}
	return domain.PoolConfig{}, domain.ErrPoolConfigNotFound
}

func (f *fakeMarketingStore) DisablePoolConfig(_ context.Context, configID string) (domain.PoolConfig, error) {
	for i := range f.poolConfigs {
		if f.poolConfigs[i].ConfigID == configID {
			f.poolConfigs[i].State = domain.PoolConfigStateDisabled
			return f.poolConfigs[i], nil
		}
	}
	return domain.PoolConfig{}, domain.ErrPoolConfigNotFound
}

func (f *fakeMarketingStore) CreateSpecialPlan(_ context.Context, plan domain.SpecialPricePlan) (domain.SpecialPricePlan, error) {
	if plan.PlanID == "" {
		plan.PlanID = "SP-" + plan.CityCode + "-" + plan.CarType
	}
	f.specialPlans = append(f.specialPlans, plan)
	return plan, nil
}

func (f *fakeMarketingStore) ListSpecialPlans(_ context.Context, filter PoolFilter) ([]domain.SpecialPricePlan, int64, error) {
	out := make([]domain.SpecialPricePlan, 0)
	for _, p := range f.specialPlans {
		if filter.State != "" && p.State != filter.State {
			continue
		}
		out = append(out, p)
	}
	return out, int64(len(out)), nil
}

func (f *fakeMarketingStore) PublishSpecialPlan(_ context.Context, planID string) (domain.SpecialPricePlan, error) {
	for i := range f.specialPlans {
		if f.specialPlans[i].PlanID == planID {
			f.specialPlans[i].State = domain.PoolConfigStatePublished
			return f.specialPlans[i], nil
		}
	}
	return domain.SpecialPricePlan{}, domain.ErrPoolPlanNotFound
}

func (f *fakeMarketingStore) DisableSpecialPlan(_ context.Context, planID string) (domain.SpecialPricePlan, error) {
	for i := range f.specialPlans {
		if f.specialPlans[i].PlanID == planID {
			f.specialPlans[i].State = domain.PoolConfigStateDisabled
			return f.specialPlans[i], nil
		}
	}
	return domain.SpecialPricePlan{}, domain.ErrPoolPlanNotFound
}

func (f *fakeMarketingStore) PoolStats(_ context.Context) (domain.PoolStats, error) {
	return domain.PoolStats{}, nil
}

// ---- A07 券作废与 A08 补贴追回 ----

func (f *fakeMarketingStore) FindIssuedCoupon(_ context.Context, couponNo string) (domain.IssuedCoupon, error) {
	for _, c := range f.coupons {
		if c.CouponNo == couponNo {
			return c, nil
		}
	}
	return domain.IssuedCoupon{}, domain.ErrCouponNotFound
}

func (f *fakeMarketingStore) VoidIssuedCoupon(_ context.Context, couponNo string) (bool, error) {
	for i := range f.coupons {
		if f.coupons[i].CouponNo == couponNo && f.coupons[i].State == domain.CouponStateAvailable {
			f.coupons[i].State = domain.CouponStateVoid
			return true, nil
		}
	}
	return false, nil
}

func (f *fakeMarketingStore) FindSubsidyRecord(_ context.Context, recordNo string) (domain.SubsidyRecord, error) {
	for _, r := range f.subsidies {
		if r.RecordNo == recordNo {
			return r, nil
		}
	}
	return domain.SubsidyRecord{}, domain.ErrSubsidyRecordNotFound
}

func (f *fakeMarketingStore) MarkSubsidyReversed(_ context.Context, record domain.SubsidyRecord) error {
	for i := range f.subsidies {
		if f.subsidies[i].RecordNo == record.RecordNo {
			if f.subsidies[i].Reversed {
				return domain.ErrSubsidyAlreadyReversed
			}
			f.subsidies[i] = record
			return nil
		}
	}
	return domain.ErrSubsidyRecordNotFound
}

// itoa 是测试用的小工具，避免引入 strconv 依赖到每个用例里。
func itoa(v int64) string {
	if v == 0 {
		return "0"
	}
	buf := make([]byte, 0, 20)
	for v > 0 {
		buf = append([]byte{byte('0' + v%10)}, buf...)
		v /= 10
	}
	return string(buf)
}
