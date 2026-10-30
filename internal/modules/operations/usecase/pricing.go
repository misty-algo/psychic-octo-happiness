// A05 计价规则用例：草稿 → 校验 → 样例预览 → 发布新版本。
//
// 发布是唯一会改变线上算钱结果的动作，所以这里卡得最死：
//   1. 只有草稿能发布，已发布版本只读；
//   2. 同城市车型区域、相同优先级的时间区间不能重叠（区域重叠按优先级选一）；
//   3. 发布前必须预览至少三个分账样例，且样例必须资金守恒。
package usecase

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/NSObjects/echo-admin/internal/modules/operations/domain"
)

// PricingRuleFilter 计价规则列表筛选。
type PricingRuleFilter struct {
	CityCode string
	CarType  string
	State    string
	Keyword  string
	Skip     int
	Limit    int
}

// CreatePricingRuleInput 是新建草稿的入参。
type CreatePricingRuleInput struct {
	RuleID     string `json:"rule_id"`
	CityCode   string `json:"city_code"`
	Area       string `json:"area"`
	CarType    string `json:"car_type"`
	StartFen   int64  `json:"start_fen"`
	IncludeM   int64  `json:"include_m"`
	PerKmFen   int64  `json:"per_km_fen"`
	PerMinFen  int64  `json:"per_min_fen"`
	ServiceBp  int64  `json:"service_bp"`
	CommBp     int64  `json:"comm_bp"`
	SurgeBp    int64  `json:"surge_bp"`
	DiscountBp int64  `json:"discount_bp"`
	CancelFen  int64  `json:"cancel_fen"`
	FixedFen   int64  `json:"fixed_fen"`
	Priority   int    `json:"priority"`
	ValidFrom  string `json:"valid_from"`
	ValidTo    string `json:"valid_to"`
	OperatorID int64
	RequestID  string
}

// PricingPreviewInput 是样例预览的入参：可以是已存规则，也可以是一份待发草稿。
type PricingPreviewInput struct {
	Rule    *domain.PricingRule `json:"rule"`
	Samples []domain.FareInput  `json:"samples"`
}

// PricingUsecase 承载计价规则的配置与发布。
type PricingUsecase struct {
	store MarketingStore
	now   func() time.Time
}

// NewPricing 创建计价规则用例。
func NewPricing(store MarketingStore) *PricingUsecase {
	return &PricingUsecase{store: store, now: time.Now}
}

// CreateRule 建一条草稿规则。
func (u *PricingUsecase) CreateRule(ctx context.Context, in CreatePricingRuleInput) (domain.PricingRule, error) {
	validFrom, err := parseTime(in.ValidFrom)
	if err != nil {
		return domain.PricingRule{}, mapMarketingError(domain.ErrPricingWindowInvalid)
	}
	validTo, err := parseTime(in.ValidTo)
	if err != nil {
		return domain.PricingRule{}, mapMarketingError(domain.ErrPricingWindowInvalid)
	}
	rule := domain.PricingRule{
		CityCode:   strings.TrimSpace(in.CityCode),
		Area:       strings.TrimSpace(in.Area),
		CarType:    strings.TrimSpace(in.CarType),
		StartFen:   in.StartFen,
		IncludeM:   in.IncludeM,
		PerKmFen:   in.PerKmFen,
		PerMinFen:  in.PerMinFen,
		ServiceBp:  in.ServiceBp,
		CommBp:     in.CommBp,
		SurgeBp:    in.SurgeBp,
		DiscountBp: in.DiscountBp,
		CancelFen:  in.CancelFen,
		FixedFen:   in.FixedFen,
		Priority:   in.Priority,
		ValidFrom:  validFrom,
		ValidTo:    validTo,
		State:      domain.PricingRuleStateDraft,
		Operator:   operatorName(in.OperatorID),
	}
	if err := rule.Validate(); err != nil {
		return domain.PricingRule{}, mapMarketingError(err)
	}
	// 版本号由存储侧按「城市+车型+区域」递增，保证同范围版本连续可追溯。
	nextVersion, err := u.store.NextPricingVersion(ctx, rule.CityCode, rule.CarType, rule.Area)
	if err != nil {
		return domain.PricingRule{}, fmt.Errorf("next pricing version: %w", err)
	}
	rule.Version = nextVersion
	created, err := u.store.CreatePricingRule(ctx, rule)
	if err != nil {
		return domain.PricingRule{}, fmt.Errorf("create pricing rule: %w", err)
	}
	return created, nil
}

// PublishRule 发布一条草稿：校验重叠、要求先看过样例。
func (u *PricingUsecase) PublishRule(ctx context.Context, ruleID string) (domain.PricingRule, error) {
	rule, err := u.store.FindPricingRule(ctx, strings.TrimSpace(ruleID))
	if err != nil {
		return domain.PricingRule{}, mapMarketingError(err)
	}
	if rule.State != domain.PricingRuleStateDraft {
		return domain.PricingRule{}, mapMarketingError(domain.ErrPricingDraftOnly)
	}
	if err := rule.Validate(); err != nil {
		return domain.PricingRule{}, mapMarketingError(err)
	}
	// 同城市车型区域、相同优先级的生效区间不能重叠。
	overlap, err := u.store.FindPricingOverlap(ctx, rule)
	if err != nil {
		return domain.PricingRule{}, fmt.Errorf("find pricing overlap: %w", err)
	}
	if overlap != nil {
		return domain.PricingRule{}, mapMarketingError(domain.ErrPricingOverlap)
	}
	// 发布前必须算过至少三个样例，且样例必须守恒，否则不给发。
	samples := u.defaultSamples()
	for _, sample := range samples {
		fare := rule.ComputeFare(sample)
		if fare.GFen != 0 {
			return domain.PricingRule{}, mapMarketingError(fmt.Errorf("%w：样例 %s 分账不守恒（G=%d）", domain.ErrPricingSampleRequired, sample.Name, fare.GFen))
		}
	}
	published, err := u.store.PublishPricingRule(ctx, rule.RuleID)
	if err != nil {
		return domain.PricingRule{}, fmt.Errorf("publish pricing rule: %w", err)
	}
	return published, nil
}

// DisableRule 停用一条已发布规则：先冻结，再让人发新版本接上。
func (u *PricingUsecase) DisableRule(ctx context.Context, ruleID string) (domain.PricingRule, error) {
	rule, err := u.store.FindPricingRule(ctx, strings.TrimSpace(ruleID))
	if err != nil {
		return domain.PricingRule{}, mapMarketingError(err)
	}
	if rule.State == domain.PricingRuleStateDisabled {
		return rule, nil
	}
	disabled, err := u.store.DisablePricingRule(ctx, rule.RuleID)
	if err != nil {
		return domain.PricingRule{}, fmt.Errorf("disable pricing rule: %w", err)
	}
	return disabled, nil
}

// ListRules 返回规则列表。
func (u *PricingUsecase) ListRules(ctx context.Context, filter PricingRuleFilter) ([]domain.PricingRule, int64, error) {
	items, total, err := u.store.ListPricingRules(ctx, filter)
	if err != nil {
		return nil, 0, fmt.Errorf("list pricing rules: %w", err)
	}
	return items, total, nil
}

// RuleDetail 返回一条规则。
func (u *PricingUsecase) RuleDetail(ctx context.Context, ruleID string) (domain.PricingRule, error) {
	rule, err := u.store.FindPricingRule(ctx, strings.TrimSpace(ruleID))
	if err != nil {
		return domain.PricingRule{}, mapMarketingError(err)
	}
	return rule, nil
}

// Preview 算分账样例：没传样例就用三个默认样例（短途/中途/长途）。
func (u *PricingUsecase) Preview(ctx context.Context, in PricingPreviewInput) ([]domain.FareSample, error) {
	var rule domain.PricingRule
	switch {
	case in.Rule != nil && in.Rule.RuleID != "":
		stored, err := u.store.FindPricingRule(ctx, in.Rule.RuleID)
		if err != nil {
			return nil, mapMarketingError(err)
		}
		rule = stored
	case in.Rule != nil:
		rule = *in.Rule
		if err := rule.Validate(); err != nil {
			return nil, mapMarketingError(err)
		}
	default:
		return nil, mapMarketingError(domain.ErrPricingRuleNotFound)
	}

	inputs := in.Samples
	if len(inputs) == 0 {
		inputs = u.defaultSamples()
	}
	out := make([]domain.FareSample, 0, len(inputs))
	for i, sample := range inputs {
		out = append(out, domain.FareSample{
			Name:  sampleName(sample, i),
			Input: sample,
			Fare:  rule.ComputeFare(sample),
		})
	}
	return out, nil
}

// EffectiveRule 查某城市车型当前生效的规则，没有就明确报错——无有效价必须
// 停止该车型新单估价并告警，不能悄悄用一个错价。
func (u *PricingUsecase) EffectiveRule(ctx context.Context, cityCode, carType string, at time.Time) (domain.PricingRule, error) {
	rule, err := u.store.FindEffectivePricingRule(ctx, strings.TrimSpace(cityCode), strings.TrimSpace(carType), at)
	if err != nil {
		if errors.Is(err, domain.ErrPricingNoEffective) {
			return domain.PricingRule{}, mapMarketingError(err)
		}
		return domain.PricingRule{}, fmt.Errorf("find effective pricing rule: %w", err)
	}
	return rule, nil
}

// defaultSamples 是发布前必看的三个样例：短途、中途、长途（含券与补贴）。
func (u *PricingUsecase) defaultSamples() []domain.FareInput {
	return []domain.FareInput{
		{Name: "", DistM: 3000, DurS: 600, CouponFen: 0, SubsidyFen: 0},
		{Name: "", DistM: 12000, DurS: 1800, CouponFen: 300, SubsidyFen: 0},
		{Name: "", DistM: 35000, DurS: 4200, CouponFen: 500, SubsidyFen: 200},
	}
}

func sampleName(in domain.FareInput, index int) string {
	if in.Name != "" {
		return in.Name
	}
	names := []string{"短途样例", "中途样例", "长途样例"}
	if index < len(names) {
		return names[index]
	}
	return fmt.Sprintf("样例 %d", index+1)
}
