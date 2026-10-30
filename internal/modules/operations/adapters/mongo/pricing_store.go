// A05 计价规则的 Mongo 适配：版本追加、区间重叠检测与生效版本选择。
package mongo

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"

	"github.com/NSObjects/echo-admin/internal/modules/operations/domain"
	"github.com/NSObjects/echo-admin/internal/modules/operations/usecase"
)

const colPricingRules = "pricing_rules"

// pricingRuleDoc 是计价规则文档。同一「城市+车型+区域」的版本追加存储，
// 靠 uk_scope_version 唯一索引兜住并发发版重复。
type pricingRuleDoc struct {
	ID        bson.ObjectID `bson:"_id,omitempty"`
	RuleID    string        `bson:"ruleId"`
	Version   int64         `bson:"version"`
	CityCode  string        `bson:"cityCode"`
	Area      string        `bson:"area"`
	CarType   string        `bson:"carType"`
	StartFen  int64         `bson:"startFen"`
	IncludeM  int64         `bson:"includeM"`
	PerKmFen  int64         `bson:"perKmFen"`
	PerMinFen int64         `bson:"perMinFen"`
	ServiceBp int64         `bson:"serviceBp"`
	CommBp    int64         `bson:"commBp"`
	SurgeBp   int64         `bson:"surgeBp"`
	Discount  int64         `bson:"discountBp"`
	CancelFen int64         `bson:"cancelFen"`
	FixedFen  int64         `bson:"fixedFen"`
	Priority  int           `bson:"priority"`
	ValidFrom time.Time     `bson:"validFrom"`
	ValidTo   time.Time     `bson:"validTo"`
	State     string        `bson:"state"`
	Operator  string        `bson:"operator,omitempty"`
	CreatedAt time.Time     `bson:"createdAt"`
	UpdatedAt time.Time     `bson:"updatedAt"`
}

func pricingFromDomain(r domain.PricingRule) pricingRuleDoc {
	return pricingRuleDoc{
		RuleID:    r.RuleID,
		Version:   r.Version,
		CityCode:  r.CityCode,
		Area:      r.Area,
		CarType:   r.CarType,
		StartFen:  r.StartFen,
		IncludeM:  r.IncludeM,
		PerKmFen:  r.PerKmFen,
		PerMinFen: r.PerMinFen,
		ServiceBp: r.ServiceBp,
		CommBp:    r.CommBp,
		SurgeBp:   r.SurgeBp,
		Discount:  r.DiscountBp,
		CancelFen: r.CancelFen,
		FixedFen:  r.FixedFen,
		Priority:  r.Priority,
		ValidFrom: r.ValidFrom,
		ValidTo:   r.ValidTo,
		State:     r.State,
		Operator:  r.Operator,
		CreatedAt: r.CreatedAt,
		UpdatedAt: r.UpdatedAt,
	}
}

func (d pricingRuleDoc) toDomain() domain.PricingRule {
	return domain.PricingRule{
		RuleID:     d.RuleID,
		Version:    d.Version,
		CityCode:   d.CityCode,
		Area:       d.Area,
		CarType:    d.CarType,
		StartFen:   d.StartFen,
		IncludeM:   d.IncludeM,
		PerKmFen:   d.PerKmFen,
		PerMinFen:  d.PerMinFen,
		ServiceBp:  d.ServiceBp,
		CommBp:     d.CommBp,
		SurgeBp:    d.SurgeBp,
		DiscountBp: d.Discount,
		CancelFen:  d.CancelFen,
		FixedFen:   d.FixedFen,
		Priority:   d.Priority,
		ValidFrom:  d.ValidFrom,
		ValidTo:    d.ValidTo,
		State:      d.State,
		Operator:   d.Operator,
		CreatedAt:  d.CreatedAt,
		UpdatedAt:  d.UpdatedAt,
	}
}

// NextPricingVersion 返回该范围下一个版本号。
//
// 区域可能为空（表示全城），空值在库里是「字段不存在」，直接用 area: "" 查会
// 匹配不到已有版本，导致第二条规则又从 V1 开始、规则号撞车。所以区域不作为
// 版本维度，版本号只按「城市 + 车型」递增，保证同范围内版本连续且唯一。
func (s *Store) NextPricingVersion(ctx context.Context, cityCode, carType, area string) (int64, error) {
	filter := bson.M{"cityCode": cityCode, "carType": carType}
	if area != "" {
		filter["area"] = area
	}
	opts := options.FindOne().SetSort(bson.D{{Key: "version", Value: -1}}).SetProjection(bson.M{"version": 1})
	var doc pricingRuleDoc
	err := s.db.Collection(colPricingRules).FindOne(ctx, filter, opts).Decode(&doc)
	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return 1, nil
		}
		return 0, fmt.Errorf("next pricing version: %w", err)
	}
	return doc.Version + 1, nil
}

// CreatePricingRule 写入一条草稿。
func (s *Store) CreatePricingRule(ctx context.Context, rule domain.PricingRule) (domain.PricingRule, error) {
	now := time.Now().UTC()
	rule.RuleID = newPricingRuleID(rule.CityCode, rule.CarType, rule.Version)
	rule.CreatedAt = now
	rule.UpdatedAt = now
	doc := pricingFromDomain(rule)
	doc.ID = bson.NewObjectID()
	if _, err := s.db.Collection(colPricingRules).InsertOne(ctx, doc); err != nil {
		return domain.PricingRule{}, fmt.Errorf("insert pricing rule: %w", err)
	}
	return doc.toDomain(), nil
}

// newPricingRuleID 生成可读规则号：PR-城市-车型-V版本。
func newPricingRuleID(cityCode, carType string, version int64) string {
	return fmt.Sprintf("PR-%s-%s-V%d", strings.ToUpper(cityCode), strings.ToUpper(carType), version)
}

// FindPricingRule 按规则号查一条。
func (s *Store) FindPricingRule(ctx context.Context, ruleID string) (domain.PricingRule, error) {
	var doc pricingRuleDoc
	err := s.db.Collection(colPricingRules).FindOne(ctx, bson.M{"ruleId": strings.TrimSpace(ruleID)}).Decode(&doc)
	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return domain.PricingRule{}, domain.ErrPricingRuleNotFound
		}
		return domain.PricingRule{}, fmt.Errorf("find pricing rule: %w", err)
	}
	return doc.toDomain(), nil
}

// ListPricingRules 返回规则一页，按城市车型与版本排序。
func (s *Store) ListPricingRules(ctx context.Context, filter usecase.PricingRuleFilter) ([]domain.PricingRule, int64, error) {
	query := bson.M{}
	if filter.CityCode != "" {
		query["cityCode"] = filter.CityCode
	}
	if filter.CarType != "" {
		query["carType"] = filter.CarType
	}
	if filter.State != "" {
		query["state"] = filter.State
	}
	if filter.Keyword != "" {
		query["$or"] = bson.A{
			bson.M{"ruleId": bson.M{"$regex": filter.Keyword, "$options": "i"}},
			bson.M{"cityCode": bson.M{"$regex": filter.Keyword, "$options": "i"}},
			bson.M{"area": bson.M{"$regex": filter.Keyword, "$options": "i"}},
		}
	}
	col := s.db.Collection(colPricingRules)
	total, err := col.CountDocuments(ctx, query)
	if err != nil {
		return nil, 0, fmt.Errorf("count pricing rules: %w", err)
	}
	opts := options.Find().SetSort(bson.D{
		{Key: "cityCode", Value: 1}, {Key: "carType", Value: 1}, {Key: "version", Value: -1},
	}).SetSkip(int64(filter.Skip)).SetLimit(int64(filter.Limit))
	cursor, err := col.Find(ctx, query, opts)
	if err != nil {
		return nil, 0, fmt.Errorf("list pricing rules: %w", err)
	}
	defer cursor.Close(ctx)
	out := make([]domain.PricingRule, 0)
	for cursor.Next(ctx) {
		var doc pricingRuleDoc
		if err := cursor.Decode(&doc); err != nil {
			return nil, 0, fmt.Errorf("decode pricing rule: %w", err)
		}
		out = append(out, doc.toDomain())
	}
	if err := cursor.Err(); err != nil {
		return nil, 0, fmt.Errorf("iterate pricing rules: %w", err)
	}
	return out, total, nil
}

// FindPricingOverlap 找同城市车型区域、相同优先级且区间重叠的已发布规则。
func (s *Store) FindPricingOverlap(ctx context.Context, rule domain.PricingRule) (*domain.PricingRule, error) {
	// 历史文档可能没有 area 字段，空区域要同时匹配空串与缺失，否则会漏检重叠，
	// 发出两份互相打架的价。
	areaValue := any(rule.Area)
	if rule.Area == "" {
		areaValue = bson.M{"$in": bson.A{"", nil}}
	}
	query := bson.M{
		"cityCode": rule.CityCode,
		"carType":  rule.CarType,
		"area":     areaValue,
		"priority": rule.Priority,
		"state":    domain.PricingRuleStatePublished,
		// 区间重叠：两者互相跨越对方的结束时间。
		"validFrom": bson.M{"$lt": rule.ValidTo},
		"validTo":   bson.M{"$gt": rule.ValidFrom},
	}
	if rule.RuleID != "" {
		query["ruleId"] = bson.M{"$ne": rule.RuleID}
	}
	var doc pricingRuleDoc
	err := s.db.Collection(colPricingRules).FindOne(ctx, query).Decode(&doc)
	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return nil, nil
		}
		return nil, fmt.Errorf("find pricing overlap: %w", err)
	}
	found := doc.toDomain()
	return &found, nil
}

// PublishPricingRule 把草稿置为已发布。
func (s *Store) PublishPricingRule(ctx context.Context, ruleID string) (domain.PricingRule, error) {
	return s.updatePricingState(ctx, ruleID, domain.PricingRuleStatePublished)
}

// DisablePricingRule 停用一条规则。
func (s *Store) DisablePricingRule(ctx context.Context, ruleID string) (domain.PricingRule, error) {
	return s.updatePricingState(ctx, ruleID, domain.PricingRuleStateDisabled)
}

func (s *Store) updatePricingState(ctx context.Context, ruleID, state string) (domain.PricingRule, error) {
	opts := options.FindOneAndUpdate().SetReturnDocument(options.After)
	var doc pricingRuleDoc
	err := s.db.Collection(colPricingRules).FindOneAndUpdate(ctx,
		bson.M{"ruleId": ruleID},
		bson.M{"$set": bson.M{"state": state, "updatedAt": time.Now().UTC()}},
		opts,
	).Decode(&doc)
	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return domain.PricingRule{}, domain.ErrPricingRuleNotFound
		}
		return domain.PricingRule{}, fmt.Errorf("update pricing state: %w", err)
	}
	return doc.toDomain(), nil
}

// FindEffectivePricingRule 取指定时刻生效的规则：命中多条时按优先级高者、
// 版本高者排序取第一条；一条都没有就是无有效价，必须告警而不是随便选一个。
func (s *Store) FindEffectivePricingRule(ctx context.Context, cityCode, carType string, at time.Time) (domain.PricingRule, error) {
	query := bson.M{
		"cityCode":  cityCode,
		"carType":   carType,
		"state":     domain.PricingRuleStatePublished,
		"validFrom": bson.M{"$lte": at},
		"validTo":   bson.M{"$gt": at},
	}
	opts := options.FindOne().SetSort(bson.D{
		{Key: "priority", Value: -1}, {Key: "version", Value: -1},
	})
	var doc pricingRuleDoc
	err := s.db.Collection(colPricingRules).FindOne(ctx, query, opts).Decode(&doc)
	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return domain.PricingRule{}, domain.ErrPricingNoEffective
		}
		return domain.PricingRule{}, fmt.Errorf("find effective pricing rule: %w", err)
	}
	return doc.toDomain(), nil
}
