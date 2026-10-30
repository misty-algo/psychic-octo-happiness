// A06 派单策略与 A09 拼车/特价的 Mongo 适配。
package mongo

import (
	"context"
	"errors"
	"fmt"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"

	"github.com/NSObjects/echo-admin/internal/modules/operations/domain"
	"github.com/NSObjects/echo-admin/internal/modules/operations/usecase"
)

const (
	colDispatchStrategies = "dispatch_strategies"
	colPoolConfigs        = "pool_configs"
	colSpecialPlans       = "special_price_plans"
)

// ---- A06 派单策略 ----

type dispatchStrategyDoc struct {
	ID           bson.ObjectID `bson:"_id,omitempty"`
	StrategyID   string        `bson:"strategyId"`
	Version      int64         `bson:"version"`
	IncomeBp     int           `bson:"incomeBp"`
	DistanceBp   int           `bson:"distanceBp"`
	RatingBp     int           `bson:"ratingBp"`
	OnlineBp     int           `bson:"onlineSecBp"`
	SuccessBp    int           `bson:"successRateBp"`
	RadiusM      int           `bson:"radiusM"`
	InviteSec    int           `bson:"inviteSec"`
	TotalSec     int           `bson:"totalSec"`
	ListenSec    int           `bson:"listenSec"`
	GrabSec      int           `bson:"grabSec"`
	RegionSwitch bool          `bson:"regionSwitch"`
	ExtraWindow  int           `bson:"extraWindowSec"`
	MaxReturn    int           `bson:"maxAcceptedReturn"`
	State        string        `bson:"state"`
	Operator     string        `bson:"operator,omitempty"`
	CreatedAt    time.Time     `bson:"createdAt"`
	UpdatedAt    time.Time     `bson:"updatedAt"`
	PublishedAt  time.Time     `bson:"publishedAt,omitempty"`
}

func dispatchFromDomain(s domain.DispatchStrategy) dispatchStrategyDoc {
	return dispatchStrategyDoc{
		StrategyID:   s.StrategyID,
		Version:      s.Version,
		IncomeBp:     s.Weights.IncomeBp,
		DistanceBp:   s.Weights.DistanceBp,
		RatingBp:     s.Weights.RatingBp,
		OnlineBp:     s.Weights.OnlineSecBp,
		SuccessBp:    s.Weights.SuccessRateBp,
		RadiusM:      s.RadiusM,
		InviteSec:    s.InviteSec,
		TotalSec:     s.TotalSec,
		ListenSec:    s.ListenSec,
		GrabSec:      s.GrabSec,
		RegionSwitch: s.RegionSwitch,
		ExtraWindow:  s.Redispatch.ExtraWindowSec,
		MaxReturn:    s.Redispatch.MaxAcceptedReturn,
		State:        s.State,
		Operator:     s.Operator,
		CreatedAt:    s.CreatedAt,
		UpdatedAt:    s.UpdatedAt,
		PublishedAt:  s.PublishedAt,
	}
}

func (d dispatchStrategyDoc) toDomain() domain.DispatchStrategy {
	return domain.DispatchStrategy{
		StrategyID: d.StrategyID,
		Version:    d.Version,
		Weights: domain.DispatchWeights{
			IncomeBp:      d.IncomeBp,
			DistanceBp:    d.DistanceBp,
			RatingBp:      d.RatingBp,
			OnlineSecBp:   d.OnlineBp,
			SuccessRateBp: d.SuccessBp,
		},
		RadiusM:      d.RadiusM,
		InviteSec:    d.InviteSec,
		TotalSec:     d.TotalSec,
		ListenSec:    d.ListenSec,
		GrabSec:      d.GrabSec,
		RegionSwitch: d.RegionSwitch,
		Redispatch: domain.DispatchRedispatch{
			ExtraWindowSec:    d.ExtraWindow,
			MaxAcceptedReturn: d.MaxReturn,
		},
		State:       d.State,
		Operator:    d.Operator,
		CreatedAt:   d.CreatedAt,
		UpdatedAt:   d.UpdatedAt,
		PublishedAt: d.PublishedAt,
	}
}

// NextDispatchVersion 返回下一个策略版本号。
func (s *Store) NextDispatchVersion(ctx context.Context) (int64, error) {
	opts := options.FindOne().SetSort(bson.D{{Key: "version", Value: -1}}).SetProjection(bson.M{"version": 1})
	var doc dispatchStrategyDoc
	err := s.db.Collection(colDispatchStrategies).FindOne(ctx, bson.M{}, opts).Decode(&doc)
	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return 1, nil
		}
		return 0, fmt.Errorf("next dispatch version: %w", err)
	}
	return doc.Version + 1, nil
}

// CreateDispatchStrategy 写入一条草稿策略。
func (s *Store) CreateDispatchStrategy(ctx context.Context, strategy domain.DispatchStrategy) (domain.DispatchStrategy, error) {
	now := time.Now().UTC()
	strategy.StrategyID = fmt.Sprintf("DS-V%d", strategy.Version)
	strategy.CreatedAt = now
	strategy.UpdatedAt = now
	doc := dispatchFromDomain(strategy)
	doc.ID = bson.NewObjectID()
	if _, err := s.db.Collection(colDispatchStrategies).InsertOne(ctx, doc); err != nil {
		return domain.DispatchStrategy{}, fmt.Errorf("insert dispatch strategy: %w", err)
	}
	return doc.toDomain(), nil
}

// FindDispatchStrategy 按策略号查一条。
func (s *Store) FindDispatchStrategy(ctx context.Context, strategyID string) (domain.DispatchStrategy, error) {
	var doc dispatchStrategyDoc
	err := s.db.Collection(colDispatchStrategies).FindOne(ctx, bson.M{"strategyId": strategyID}).Decode(&doc)
	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return domain.DispatchStrategy{}, domain.ErrDispatchNotFound
		}
		return domain.DispatchStrategy{}, fmt.Errorf("find dispatch strategy: %w", err)
	}
	return doc.toDomain(), nil
}

// ListDispatchStrategies 返回策略一页，新版本在前。
func (s *Store) ListDispatchStrategies(ctx context.Context, filter usecase.DispatchFilter) ([]domain.DispatchStrategy, int64, error) {
	query := bson.M{}
	if filter.State != "" {
		query["state"] = filter.State
	}
	col := s.db.Collection(colDispatchStrategies)
	total, err := col.CountDocuments(ctx, query)
	if err != nil {
		return nil, 0, fmt.Errorf("count dispatch strategies: %w", err)
	}
	opts := options.Find().SetSort(bson.D{{Key: "version", Value: -1}}).
		SetSkip(int64(filter.Skip)).SetLimit(int64(filter.Limit))
	cursor, err := col.Find(ctx, query, opts)
	if err != nil {
		return nil, 0, fmt.Errorf("list dispatch strategies: %w", err)
	}
	defer cursor.Close(ctx)
	out := make([]domain.DispatchStrategy, 0)
	for cursor.Next(ctx) {
		var doc dispatchStrategyDoc
		if err := cursor.Decode(&doc); err != nil {
			return nil, 0, fmt.Errorf("decode dispatch strategy: %w", err)
		}
		out = append(out, doc.toDomain())
	}
	return out, total, cursor.Err()
}

// PublishDispatchStrategy 发布策略。
func (s *Store) PublishDispatchStrategy(ctx context.Context, strategyID string) (domain.DispatchStrategy, error) {
	return s.updateDispatchState(ctx, strategyID, domain.DispatchStatePublished)
}

// DisableDispatchStrategy 停用策略。
func (s *Store) DisableDispatchStrategy(ctx context.Context, strategyID string) (domain.DispatchStrategy, error) {
	return s.updateDispatchState(ctx, strategyID, domain.DispatchStateDisabled)
}

func (s *Store) updateDispatchState(ctx context.Context, strategyID, state string) (domain.DispatchStrategy, error) {
	update := bson.M{"state": state, "updatedAt": time.Now().UTC()}
	if state == domain.DispatchStatePublished {
		update["publishedAt"] = time.Now().UTC()
	}
	opts := options.FindOneAndUpdate().SetReturnDocument(options.After)
	var doc dispatchStrategyDoc
	err := s.db.Collection(colDispatchStrategies).FindOneAndUpdate(ctx,
		bson.M{"strategyId": strategyID}, bson.M{"$set": update}, opts).Decode(&doc)
	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return domain.DispatchStrategy{}, domain.ErrDispatchNotFound
		}
		return domain.DispatchStrategy{}, fmt.Errorf("update dispatch state: %w", err)
	}
	return doc.toDomain(), nil
}

// ListDispatchCandidates 从最近派单任务里取真实司机做排序预览。
// 收入用订单金额累计近似，距离与评分取尝试记录里的原始值。
func (s *Store) ListDispatchCandidates(ctx context.Context, limit int) ([]domain.DispatchCandidate, error) {
	if limit <= 0 {
		limit = 10
	}
	cursor, err := s.db.Collection(colDispatchTasks).Find(ctx,
		bson.M{"attempts": bson.M{"$exists": true, "$ne": bson.A{}}},
		options.Find().SetSort(bson.D{{Key: "createdAt", Value: -1}}).SetLimit(int64(limit)),
	)
	if err != nil {
		return nil, fmt.Errorf("list dispatch tasks for candidates: %w", err)
	}
	defer cursor.Close(ctx)
	out := make([]domain.DispatchCandidate, 0)
	seen := map[string]bool{}
	for cursor.Next(ctx) {
		var doc struct {
			Attempts []struct {
				DriverID bson.ObjectID `bson:"driverId"`
				Score    int           `bson:"score"`
				Distance int64         `bson:"distance"`
				Result   string        `bson:"result"`
			} `bson:"attempts"`
		}
		if err := cursor.Decode(&doc); err != nil {
			return nil, fmt.Errorf("decode dispatch task: %w", err)
		}
		for _, a := range doc.Attempts {
			id := a.DriverID.Hex()
			if id == "" || id == "000000000000000000000000" || seen[id] {
				continue
			}
			seen[id] = true
			out = append(out, domain.DispatchCandidate{
				DriverID:    id,
				DistanceM:   a.Distance,
				Rating:      a.Score / 100, // 0-10000 归一到 0-100
				SuccessRate: 80,
				OnlineSec:   0,
				IncomeFen:   0,
			})
			if len(out) >= limit {
				return out, cursor.Err()
			}
		}
	}
	return out, cursor.Err()
}

// ---- A09 拼车配置与特价方案 ----

type poolConfigDoc struct {
	ID        bson.ObjectID `bson:"_id,omitempty"`
	ConfigID  string        `bson:"configId"`
	Version   int64         `bson:"version"`
	MatchSec  int           `bson:"matchSec"`
	GroupMax  int           `bson:"groupMax"`
	SeatMax   int           `bson:"seatMax"`
	CityCodes []string      `bson:"cityCodes,omitempty"`
	State     string        `bson:"state"`
	Operator  string        `bson:"operator,omitempty"`
	CreatedAt time.Time     `bson:"createdAt"`
	UpdatedAt time.Time     `bson:"updatedAt"`
}

func poolFromDomain(c domain.PoolConfig) poolConfigDoc {
	return poolConfigDoc{
		ConfigID:  c.ConfigID,
		Version:   c.Version,
		MatchSec:  c.MatchSec,
		GroupMax:  c.GroupMax,
		SeatMax:   c.SeatMax,
		CityCodes: c.CityCodes,
		State:     c.State,
		Operator:  c.Operator,
		CreatedAt: c.CreatedAt,
		UpdatedAt: c.UpdatedAt,
	}
}

func (d poolConfigDoc) toDomain() domain.PoolConfig {
	return domain.PoolConfig{
		ConfigID:  d.ConfigID,
		Version:   d.Version,
		MatchSec:  d.MatchSec,
		GroupMax:  d.GroupMax,
		SeatMax:   d.SeatMax,
		CityCodes: d.CityCodes,
		State:     d.State,
		Operator:  d.Operator,
		CreatedAt: d.CreatedAt,
		UpdatedAt: d.UpdatedAt,
	}
}

// NextPoolVersion 返回下一个拼车配置版本号。
func (s *Store) NextPoolVersion(ctx context.Context) (int64, error) {
	opts := options.FindOne().SetSort(bson.D{{Key: "version", Value: -1}}).SetProjection(bson.M{"version": 1})
	var doc poolConfigDoc
	err := s.db.Collection(colPoolConfigs).FindOne(ctx, bson.M{}, opts).Decode(&doc)
	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return 1, nil
		}
		return 0, fmt.Errorf("next pool version: %w", err)
	}
	return doc.Version + 1, nil
}

// CreatePoolConfig 写入一份拼车配置草稿。
func (s *Store) CreatePoolConfig(ctx context.Context, config domain.PoolConfig) (domain.PoolConfig, error) {
	now := time.Now().UTC()
	config.ConfigID = fmt.Sprintf("POOL-V%d", config.Version)
	config.CreatedAt = now
	config.UpdatedAt = now
	doc := poolFromDomain(config)
	doc.ID = bson.NewObjectID()
	if _, err := s.db.Collection(colPoolConfigs).InsertOne(ctx, doc); err != nil {
		return domain.PoolConfig{}, fmt.Errorf("insert pool config: %w", err)
	}
	return doc.toDomain(), nil
}

// FindPoolConfig 按配置号查一份。
func (s *Store) FindPoolConfig(ctx context.Context, configID string) (domain.PoolConfig, error) {
	var doc poolConfigDoc
	err := s.db.Collection(colPoolConfigs).FindOne(ctx, bson.M{"configId": configID}).Decode(&doc)
	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return domain.PoolConfig{}, domain.ErrPoolConfigNotFound
		}
		return domain.PoolConfig{}, fmt.Errorf("find pool config: %w", err)
	}
	return doc.toDomain(), nil
}

// ListPoolConfigs 返回拼车配置一页。
func (s *Store) ListPoolConfigs(ctx context.Context, filter usecase.PoolFilter) ([]domain.PoolConfig, int64, error) {
	query := bson.M{}
	if filter.State != "" {
		query["state"] = filter.State
	}
	col := s.db.Collection(colPoolConfigs)
	total, err := col.CountDocuments(ctx, query)
	if err != nil {
		return nil, 0, fmt.Errorf("count pool configs: %w", err)
	}
	opts := options.Find().SetSort(bson.D{{Key: "version", Value: -1}}).
		SetSkip(int64(filter.Skip)).SetLimit(int64(filter.Limit))
	cursor, err := col.Find(ctx, query, opts)
	if err != nil {
		return nil, 0, fmt.Errorf("list pool configs: %w", err)
	}
	defer cursor.Close(ctx)
	out := make([]domain.PoolConfig, 0)
	for cursor.Next(ctx) {
		var doc poolConfigDoc
		if err := cursor.Decode(&doc); err != nil {
			return nil, 0, fmt.Errorf("decode pool config: %w", err)
		}
		out = append(out, doc.toDomain())
	}
	return out, total, cursor.Err()
}

// PublishPoolConfig 发布拼车配置。
func (s *Store) PublishPoolConfig(ctx context.Context, configID string) (domain.PoolConfig, error) {
	return s.updatePoolState(ctx, configID, domain.PoolConfigStatePublished)
}

// DisablePoolConfig 停用拼车配置。
func (s *Store) DisablePoolConfig(ctx context.Context, configID string) (domain.PoolConfig, error) {
	return s.updatePoolState(ctx, configID, domain.PoolConfigStateDisabled)
}

func (s *Store) updatePoolState(ctx context.Context, configID, state string) (domain.PoolConfig, error) {
	opts := options.FindOneAndUpdate().SetReturnDocument(options.After)
	var doc poolConfigDoc
	err := s.db.Collection(colPoolConfigs).FindOneAndUpdate(ctx,
		bson.M{"configId": configID},
		bson.M{"$set": bson.M{"state": state, "updatedAt": time.Now().UTC()}},
		opts,
	).Decode(&doc)
	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return domain.PoolConfig{}, domain.ErrPoolConfigNotFound
		}
		return domain.PoolConfig{}, fmt.Errorf("update pool state: %w", err)
	}
	return doc.toDomain(), nil
}

type specialPlanDoc struct {
	ID        bson.ObjectID `bson:"_id,omitempty"`
	PlanID    string        `bson:"planId"`
	Name      string        `bson:"name"`
	CityCode  string        `bson:"cityCode"`
	CarType   string        `bson:"carType"`
	FixedFen  int64         `bson:"fixedFen"`
	Seats     int           `bson:"seats"`
	ValidFrom time.Time     `bson:"validFrom"`
	ValidTo   time.Time     `bson:"validTo"`
	State     string        `bson:"state"`
	Operator  string        `bson:"operator,omitempty"`
	CreatedAt time.Time     `bson:"createdAt"`
	UpdatedAt time.Time     `bson:"updatedAt"`
}

func specialPlanFromDomain(p domain.SpecialPricePlan) specialPlanDoc {
	return specialPlanDoc{
		PlanID:    p.PlanID,
		Name:      p.Name,
		CityCode:  p.CityCode,
		CarType:   p.CarType,
		FixedFen:  p.FixedFen,
		Seats:     p.Seats,
		ValidFrom: p.ValidFrom,
		ValidTo:   p.ValidTo,
		State:     p.State,
		Operator:  p.Operator,
		CreatedAt: p.CreatedAt,
		UpdatedAt: p.UpdatedAt,
	}
}

func (d specialPlanDoc) toDomain() domain.SpecialPricePlan {
	return domain.SpecialPricePlan{
		PlanID:    d.PlanID,
		Name:      d.Name,
		CityCode:  d.CityCode,
		CarType:   d.CarType,
		FixedFen:  d.FixedFen,
		Seats:     d.Seats,
		ValidFrom: d.ValidFrom,
		ValidTo:   d.ValidTo,
		State:     d.State,
		Operator:  d.Operator,
		CreatedAt: d.CreatedAt,
		UpdatedAt: d.UpdatedAt,
	}
}

// CreateSpecialPlan 写入一个特价方案。
func (s *Store) CreateSpecialPlan(ctx context.Context, plan domain.SpecialPricePlan) (domain.SpecialPricePlan, error) {
	now := time.Now().UTC()
	plan.PlanID = fmt.Sprintf("SP-%s-%s-%d", plan.CityCode, plan.CarType, now.Unix())
	plan.CreatedAt = now
	plan.UpdatedAt = now
	doc := specialPlanFromDomain(plan)
	doc.ID = bson.NewObjectID()
	if _, err := s.db.Collection(colSpecialPlans).InsertOne(ctx, doc); err != nil {
		return domain.SpecialPricePlan{}, fmt.Errorf("insert special plan: %w", err)
	}
	return doc.toDomain(), nil
}

// ListSpecialPlans 返回特价方案一页。
func (s *Store) ListSpecialPlans(ctx context.Context, filter usecase.PoolFilter) ([]domain.SpecialPricePlan, int64, error) {
	query := bson.M{}
	if filter.State != "" {
		query["state"] = filter.State
	}
	col := s.db.Collection(colSpecialPlans)
	total, err := col.CountDocuments(ctx, query)
	if err != nil {
		return nil, 0, fmt.Errorf("count special plans: %w", err)
	}
	opts := options.Find().SetSort(bson.D{{Key: "createdAt", Value: -1}}).
		SetSkip(int64(filter.Skip)).SetLimit(int64(filter.Limit))
	cursor, err := col.Find(ctx, query, opts)
	if err != nil {
		return nil, 0, fmt.Errorf("list special plans: %w", err)
	}
	defer cursor.Close(ctx)
	out := make([]domain.SpecialPricePlan, 0)
	for cursor.Next(ctx) {
		var doc specialPlanDoc
		if err := cursor.Decode(&doc); err != nil {
			return nil, 0, fmt.Errorf("decode special plan: %w", err)
		}
		out = append(out, doc.toDomain())
	}
	return out, total, cursor.Err()
}

// PublishSpecialPlan 发布特价方案。
func (s *Store) PublishSpecialPlan(ctx context.Context, planID string) (domain.SpecialPricePlan, error) {
	return s.updatePlanState(ctx, planID, domain.PoolConfigStatePublished)
}

// DisableSpecialPlan 停用特价方案。
func (s *Store) DisableSpecialPlan(ctx context.Context, planID string) (domain.SpecialPricePlan, error) {
	return s.updatePlanState(ctx, planID, domain.PoolConfigStateDisabled)
}

func (s *Store) updatePlanState(ctx context.Context, planID, state string) (domain.SpecialPricePlan, error) {
	opts := options.FindOneAndUpdate().SetReturnDocument(options.After)
	var doc specialPlanDoc
	err := s.db.Collection(colSpecialPlans).FindOneAndUpdate(ctx,
		bson.M{"planId": planID},
		bson.M{"$set": bson.M{"state": state, "updatedAt": time.Now().UTC()}},
		opts,
	).Decode(&doc)
	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return domain.SpecialPricePlan{}, domain.ErrPoolPlanNotFound
		}
		return domain.SpecialPricePlan{}, fmt.Errorf("update plan state: %w", err)
	}
	return doc.toDomain(), nil
}

// PoolStats 统计拼车三个口径。
// 拼车订单以 bizType 含拼车语义为准；实际共乘按「同起终点 + 同日 + 两名不同
// 乘客且都已出发」聚合，拼未成 = 拼车单里没匹配到司机的那些。
func (s *Store) PoolStats(ctx context.Context) (domain.PoolStats, error) {
	poolTypes := bson.A{"pinche", "carpool", "pool", "拼车", "特价拼车"}
	poolQuery := bson.M{"bizType": bson.M{"$in": poolTypes}}

	poolOrderCount, err := s.db.Collection(colOrders).CountDocuments(ctx, poolQuery)
	if err != nil {
		return domain.PoolStats{}, fmt.Errorf("count pool orders: %w", err)
	}
	// 匹配成功：拼车单里有司机接单并进入行程态之后。
	matchedCount, err := s.db.Collection(colOrders).CountDocuments(ctx, bson.M{
		"bizType": bson.M{"$in": poolTypes},
		"state":   bson.M{"$in": bson.A{"ACCEPTED", "ARRIVED", "IN_TRIP", "ENDED"}},
	})
	if err != nil {
		return domain.PoolStats{}, fmt.Errorf("count matched pool orders: %w", err)
	}

	// 实际共乘：同起终点 + 同日 + 至少两名不同乘客。
	shared, err := s.countSharedGroups(ctx, poolTypes)
	if err != nil {
		return domain.PoolStats{}, err
	}

	stats := domain.PoolStats{
		PoolOrderCount: poolOrderCount,
		MatchedCount:   matchedCount,
		UnmatchedCount: poolOrderCount - matchedCount,
		GroupedCount:   shared,
		SharedCount:    shared,
	}
	if poolOrderCount > 0 {
		stats.MatchSuccessRate = float64(matchedCount) / float64(poolOrderCount) * 100
	}
	if matchedCount > 0 {
		stats.SharedRate = float64(shared) / float64(matchedCount) * 100
	}
	return stats, nil
}

// countSharedGroups 统计真实共乘组数：同起终点 + 同业务日 + 不同乘客 ≥ 2。
func (s *Store) countSharedGroups(ctx context.Context, poolTypes bson.A) (int64, error) {
	pipeline := bson.A{
		bson.M{"$match": bson.M{
			"bizType": bson.M{"$in": poolTypes},
			"state":   bson.M{"$in": bson.A{"IN_TRIP", "ENDED"}},
		}},
		bson.M{"$group": bson.M{
			"_id": bson.M{
				"start": bson.M{"$ifNull": bson.A{"$startPoiId", "$startAddress"}},
				"end":   bson.M{"$ifNull": bson.A{"$endPoiId", "$endAddress"}},
				"day":   bson.M{"$dateToString": bson.M{"format": "%Y-%m-%d", "date": "$createdAt"}},
			},
			"passengers": bson.M{"$addToSet": "$passengerId"},
		}},
		bson.M{"$project": bson.M{"count": bson.M{"$size": "$passengers"}}},
		bson.M{"$match": bson.M{"count": bson.M{"$gte": 2}}},
		bson.M{"$count": "groups"},
	}
	cursor, err := s.db.Collection(colOrders).Aggregate(ctx, pipeline)
	if err != nil {
		return 0, fmt.Errorf("aggregate shared groups: %w", err)
	}
	defer cursor.Close(ctx)
	var result struct {
		Groups int64 `bson:"groups"`
	}
	if cursor.Next(ctx) {
		if err := cursor.Decode(&result); err != nil {
			return 0, fmt.Errorf("decode shared groups: %w", err)
		}
	}
	return result.Groups, cursor.Err()
}
