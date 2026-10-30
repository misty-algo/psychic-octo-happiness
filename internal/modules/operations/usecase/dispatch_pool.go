// A06 派单策略与 A09 拼车/特价配置的用例层。
//
// 两块都是「配置即线上行为」，所以共用同一套约束：草稿可改、发布只读、停用
// 需另发新版本；发布前必须能预览效果（派单看候选排序，拼车看统计口径）。
package usecase

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/NSObjects/echo-admin/internal/modules/operations/domain"
)

// ---- A06 派单策略 ----

// DispatchFilter 派单策略列表筛选。
type DispatchFilter struct {
	State   string
	Skip    int
	Limit   int
}

// CreateDispatchInput 是新建派单策略草稿的入参。
type CreateDispatchInput struct {
	Weights      domain.DispatchWeights    `json:"weights"`
	RadiusM      int                       `json:"radius_m"`
	InviteSec    int                       `json:"invite_sec"`
	TotalSec     int                       `json:"total_sec"`
	ListenSec    int                       `json:"listen_sec"`
	GrabSec      int                       `json:"grab_sec"`
	RegionSwitch bool                      `json:"region_switch"`
	Redispatch   domain.DispatchRedispatch `json:"redispatch"`
	OperatorID   int64
	RequestID    string
}

// DispatchUsecase 管理派单策略。
type DispatchUsecase struct {
	store MarketingStore
	now   func() time.Time
}

// NewDispatch 创建派单策略用例。
func NewDispatch(store MarketingStore) *DispatchUsecase {
	return &DispatchUsecase{store: store, now: time.Now}
}

// CreateStrategy 建一条草稿策略。
func (u *DispatchUsecase) CreateStrategy(ctx context.Context, in CreateDispatchInput) (domain.DispatchStrategy, error) {
	strategy := domain.DispatchStrategy{
		Weights:      in.Weights,
		RadiusM:      in.RadiusM,
		InviteSec:    in.InviteSec,
		TotalSec:     in.TotalSec,
		ListenSec:    in.ListenSec,
		GrabSec:      in.GrabSec,
		RegionSwitch: in.RegionSwitch,
		Redispatch:   in.Redispatch,
		State:        domain.DispatchStateDraft,
		Operator:     operatorName(in.OperatorID),
	}
	if err := strategy.Validate(); err != nil {
		return domain.DispatchStrategy{}, mapMarketingError(err)
	}
	version, err := u.store.NextDispatchVersion(ctx)
	if err != nil {
		return domain.DispatchStrategy{}, fmt.Errorf("next dispatch version: %w", err)
	}
	strategy.Version = version
	created, err := u.store.CreateDispatchStrategy(ctx, strategy)
	if err != nil {
		return domain.DispatchStrategy{}, fmt.Errorf("create dispatch strategy: %w", err)
	}
	return created, nil
}

// PublishStrategy 发布草稿策略。
func (u *DispatchUsecase) PublishStrategy(ctx context.Context, strategyID string) (domain.DispatchStrategy, error) {
	strategy, err := u.store.FindDispatchStrategy(ctx, strings.TrimSpace(strategyID))
	if err != nil {
		return domain.DispatchStrategy{}, mapMarketingError(err)
	}
	if strategy.State != domain.DispatchStateDraft {
		return domain.DispatchStrategy{}, mapMarketingError(domain.ErrDispatchDraftOnly)
	}
	if err := strategy.Validate(); err != nil {
		return domain.DispatchStrategy{}, mapMarketingError(err)
	}
	published, err := u.store.PublishDispatchStrategy(ctx, strategy.StrategyID)
	if err != nil {
		return domain.DispatchStrategy{}, fmt.Errorf("publish dispatch strategy: %w", err)
	}
	return published, nil
}

// DisableStrategy 停用策略。
func (u *DispatchUsecase) DisableStrategy(ctx context.Context, strategyID string) (domain.DispatchStrategy, error) {
	if _, err := u.store.FindDispatchStrategy(ctx, strings.TrimSpace(strategyID)); err != nil {
		return domain.DispatchStrategy{}, mapMarketingError(err)
	}
	disabled, err := u.store.DisableDispatchStrategy(ctx, strings.TrimSpace(strategyID))
	if err != nil {
		return domain.DispatchStrategy{}, fmt.Errorf("disable dispatch strategy: %w", err)
	}
	return disabled, nil
}

// ListStrategies 返回策略列表。
func (u *DispatchUsecase) ListStrategies(ctx context.Context, filter DispatchFilter) ([]domain.DispatchStrategy, int64, error) {
	items, total, err := u.store.ListDispatchStrategies(ctx, filter)
	if err != nil {
		return nil, 0, fmt.Errorf("list dispatch strategies: %w", err)
	}
	return items, total, nil
}

// StrategyDetail 返回一条策略。
func (u *DispatchUsecase) StrategyDetail(ctx context.Context, strategyID string) (domain.DispatchStrategy, error) {
	strategy, err := u.store.FindDispatchStrategy(ctx, strings.TrimSpace(strategyID))
	if err != nil {
		return domain.DispatchStrategy{}, mapMarketingError(err)
	}
	return strategy, nil
}

// PreviewRanking 用当前（或指定）策略给候选司机排序，让运营看到「谁先被叫到」。
// 没传候选时，从真实派单尝试里取最近的一批司机做预览。
func (u *DispatchUsecase) PreviewRanking(ctx context.Context, strategyID string, candidates []domain.DispatchCandidate) ([]domain.RankedCandidate, error) {
	var strategy domain.DispatchStrategy
	if strategyID != "" {
		stored, err := u.store.FindDispatchStrategy(ctx, strings.TrimSpace(strategyID))
		if err != nil {
			return nil, mapMarketingError(err)
		}
		strategy = stored
	} else {
		return nil, mapMarketingError(domain.ErrDispatchNotFound)
	}
	if len(candidates) == 0 {
		real, err := u.store.ListDispatchCandidates(ctx, 10)
		if err != nil {
			return nil, fmt.Errorf("list dispatch candidates: %w", err)
		}
		candidates = real
	}
	if len(candidates) == 0 {
		return []domain.RankedCandidate{}, nil
	}
	return strategy.RankCandidates(candidates), nil
}

// ---- A09 拼车与特价 ----

// PoolFilter 拼车配置列表筛选。
type PoolFilter struct {
	State string
	Skip  int
	Limit int
}

// CreatePoolInput 是拼车匹配参数草稿入参。
type CreatePoolInput struct {
	MatchSec  int      `json:"match_sec"`
	GroupMax  int      `json:"group_max"`
	SeatMax   int      `json:"seat_max"`
	CityCodes []string `json:"city_codes"`
	OperatorID int64
	RequestID  string
}

// CreatePlanInput 是特价固定价方案入参。
type CreatePlanInput struct {
	Name      string `json:"name"`
	CityCode  string `json:"city_code"`
	CarType   string `json:"car_type"`
	FixedFen  int64  `json:"fixed_fen"`
	Seats     int    `json:"seats"`
	ValidFrom string `json:"valid_from"`
	ValidTo   string `json:"valid_to"`
	OperatorID int64
	RequestID  string
}

// PoolUsecase 管理拼车配置与特价方案。
type PoolUsecase struct {
	store MarketingStore
	now   func() time.Time
}

// NewPool 创建拼车配置用例。
func NewPool(store MarketingStore) *PoolUsecase {
	return &PoolUsecase{store: store, now: time.Now}
}

// CreateConfig 建一份拼车配置草稿。
func (u *PoolUsecase) CreateConfig(ctx context.Context, in CreatePoolInput) (domain.PoolConfig, error) {
	config := domain.PoolConfig{
		MatchSec:  in.MatchSec,
		GroupMax:  in.GroupMax,
		SeatMax:   in.SeatMax,
		CityCodes: in.CityCodes,
		State:     domain.PoolConfigStateDraft,
		Operator:  operatorName(in.OperatorID),
	}
	if err := config.Validate(); err != nil {
		return domain.PoolConfig{}, mapMarketingError(err)
	}
	version, err := u.store.NextPoolVersion(ctx)
	if err != nil {
		return domain.PoolConfig{}, fmt.Errorf("next pool version: %w", err)
	}
	config.Version = version
	created, err := u.store.CreatePoolConfig(ctx, config)
	if err != nil {
		return domain.PoolConfig{}, fmt.Errorf("create pool config: %w", err)
	}
	return created, nil
}

// PublishConfig 发布拼车配置。
func (u *PoolUsecase) PublishConfig(ctx context.Context, configID string) (domain.PoolConfig, error) {
	config, err := u.store.FindPoolConfig(ctx, strings.TrimSpace(configID))
	if err != nil {
		return domain.PoolConfig{}, mapMarketingError(err)
	}
	if config.State != domain.PoolConfigStateDraft {
		return domain.PoolConfig{}, mapMarketingError(domain.ErrPoolDraftOnly)
	}
	if err := config.Validate(); err != nil {
		return domain.PoolConfig{}, mapMarketingError(err)
	}
	published, err := u.store.PublishPoolConfig(ctx, config.ConfigID)
	if err != nil {
		return domain.PoolConfig{}, fmt.Errorf("publish pool config: %w", err)
	}
	return published, nil
}

// DisableConfig 停用拼车配置。
func (u *PoolUsecase) DisableConfig(ctx context.Context, configID string) (domain.PoolConfig, error) {
	disabled, err := u.store.DisablePoolConfig(ctx, strings.TrimSpace(configID))
	if err != nil {
		return domain.PoolConfig{}, mapMarketingError(err)
	}
	return disabled, nil
}

// ListConfigs 返回拼车配置列表。
func (u *PoolUsecase) ListConfigs(ctx context.Context, filter PoolFilter) ([]domain.PoolConfig, int64, error) {
	items, total, err := u.store.ListPoolConfigs(ctx, filter)
	if err != nil {
		return nil, 0, fmt.Errorf("list pool configs: %w", err)
	}
	return items, total, nil
}

// CreatePlan 建一个特价固定价方案。
func (u *PoolUsecase) CreatePlan(ctx context.Context, in CreatePlanInput) (domain.SpecialPricePlan, error) {
	validFrom, err := parseTime(in.ValidFrom)
	if err != nil {
		return domain.SpecialPricePlan{}, mapMarketingError(domain.ErrPoolPlanWindowInvalid)
	}
	validTo, err := parseTime(in.ValidTo)
	if err != nil {
		return domain.SpecialPricePlan{}, mapMarketingError(domain.ErrPoolPlanWindowInvalid)
	}
	plan := domain.SpecialPricePlan{
		Name:      strings.TrimSpace(in.Name),
		CityCode:  strings.TrimSpace(in.CityCode),
		CarType:   strings.TrimSpace(in.CarType),
		FixedFen:  in.FixedFen,
		Seats:     in.Seats,
		ValidFrom: validFrom,
		ValidTo:   validTo,
		State:     domain.PoolConfigStateDraft,
		Operator:  operatorName(in.OperatorID),
	}
	if err := plan.Validate(); err != nil {
		return domain.SpecialPricePlan{}, mapMarketingError(err)
	}
	created, err := u.store.CreateSpecialPlan(ctx, plan)
	if err != nil {
		return domain.SpecialPricePlan{}, fmt.Errorf("create special plan: %w", err)
	}
	return created, nil
}

// ListPlans 返回特价方案列表。
func (u *PoolUsecase) ListPlans(ctx context.Context, filter PoolFilter) ([]domain.SpecialPricePlan, int64, error) {
	items, total, err := u.store.ListSpecialPlans(ctx, filter)
	if err != nil {
		return nil, 0, fmt.Errorf("list special plans: %w", err)
	}
	return items, total, nil
}

// PublishPlan 发布特价方案。
func (u *PoolUsecase) PublishPlan(ctx context.Context, planID string) (domain.SpecialPricePlan, error) {
	published, err := u.store.PublishSpecialPlan(ctx, strings.TrimSpace(planID))
	if err != nil {
		return domain.SpecialPricePlan{}, mapMarketingError(err)
	}
	return published, nil
}

// DisablePlan 停用特价方案。
func (u *PoolUsecase) DisablePlan(ctx context.Context, planID string) (domain.SpecialPricePlan, error) {
	disabled, err := u.store.DisableSpecialPlan(ctx, strings.TrimSpace(planID))
	if err != nil {
		return domain.SpecialPricePlan{}, mapMarketingError(err)
	}
	return disabled, nil
}

// Stats 返回拼车口径的三个统计。
func (u *PoolUsecase) Stats(ctx context.Context) (domain.PoolStats, error) {
	stats, err := u.store.PoolStats(ctx)
	if err != nil {
		return domain.PoolStats{}, fmt.Errorf("pool stats: %w", err)
	}
	return stats, nil
}
