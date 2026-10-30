// A05 营销活动的 MongoDB 适配。campaigns 集合库内还是空的，字段按数据字典 8.4。
package mongo

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo/options"

	"github.com/NSObjects/echo-admin/internal/modules/operations/domain"
	"github.com/NSObjects/echo-admin/internal/modules/operations/usecase"
)

// subsidyConfigDoc 是补贴规则的内嵌文档（A08）。
type subsidyConfigDoc struct {
	TriggerType  string `bson:"triggerType"`
	AmountFen    int64  `bson:"amount"`
	MaxPerDriver int64  `bson:"maxPerDriver"`
	DailyCapFen  int64  `bson:"dailyCap"`
	TaskType     string `bson:"taskType,omitempty"`
	NeedEnroll   bool   `bson:"needEnroll"`
}

func subsidyConfigToDoc(c *domain.SubsidyConfig) *subsidyConfigDoc {
	if c == nil {
		return nil
	}
	return &subsidyConfigDoc{
		TriggerType:  c.TriggerType,
		AmountFen:    c.AmountFen,
		MaxPerDriver: c.MaxPerDriver,
		DailyCapFen:  c.DailyCapFen,
		TaskType:     c.TaskType,
		NeedEnroll:   c.NeedEnroll,
	}
}

func (d *subsidyConfigDoc) toDomain() *domain.SubsidyConfig {
	if d == nil {
		return nil
	}
	return &domain.SubsidyConfig{
		TriggerType:  d.TriggerType,
		AmountFen:    d.AmountFen,
		MaxPerDriver: d.MaxPerDriver,
		DailyCapFen:  d.DailyCapFen,
		TaskType:     d.TaskType,
		NeedEnroll:   d.NeedEnroll,
	}
}

// campaignDoc 是营销活动文档。
type campaignDoc struct {
	ID               bson.ObjectID `bson:"_id,omitempty"`
	Code             string        `bson:"code"`
	Name             string        `bson:"name"`
	Type             string        `bson:"type"`
	TargetAudience   string        `bson:"targetAudience"`
	Rules            string        `bson:"rules,omitempty"`
	CouponTemplates  []string      `bson:"rewardCouponTemplateIds,omitempty"`
	RewardAmountFen  int64         `bson:"rewardAmount,omitempty"`
	SubsidyConfig    *subsidyConfigDoc `bson:"subsidyConfig,omitempty"`
	CityCodes        []string      `bson:"cityCodes,omitempty"`
	StartAt          time.Time     `bson:"startAt"`
	EndAt            time.Time     `bson:"endAt"`
	Status           string        `bson:"status"`
	ParticipantCount int64         `bson:"participantCount,omitempty"`
	BudgetFen        int64         `bson:"budget,omitempty"`
	UsedBudgetFen    int64         `bson:"usedBudget,omitempty"`
	Operator         string        `bson:"operator,omitempty"`
	CreatedAt        time.Time     `bson:"createdAt"`
	UpdatedAt        time.Time     `bson:"updatedAt"`
}

func (d campaignDoc) toDomain() domain.Campaign {
	return domain.Campaign{
		Code:             d.Code,
		Name:             d.Name,
		Type:             d.Type,
		TargetAudience:   d.TargetAudience,
		Rules:            d.Rules,
		CouponTemplates:  d.CouponTemplates,
		RewardAmountFen:  d.RewardAmountFen,
		SubsidyConfig:    d.SubsidyConfig.toDomain(),
		CityCodes:        d.CityCodes,
		StartAt:          d.StartAt,
		EndAt:            d.EndAt,
		Status:           d.Status,
		ParticipantCount: d.ParticipantCount,
		BudgetFen:        d.BudgetFen,
		UsedBudgetFen:    d.UsedBudgetFen,
		Operator:         d.Operator,
		CreatedAt:        d.CreatedAt,
		UpdatedAt:        d.UpdatedAt,
	}
}

func campaignFromDomain(c domain.Campaign) campaignDoc {
	return campaignDoc{
		Code:             c.Code,
		Name:             c.Name,
		Type:             c.Type,
		TargetAudience:   c.TargetAudience,
		Rules:            c.Rules,
		CouponTemplates:  c.CouponTemplates,
		RewardAmountFen:  c.RewardAmountFen,
		SubsidyConfig:    subsidyConfigToDoc(c.SubsidyConfig),
		CityCodes:        c.CityCodes,
		StartAt:          c.StartAt,
		EndAt:            c.EndAt,
		Status:           c.Status,
		ParticipantCount: c.ParticipantCount,
		BudgetFen:        c.BudgetFen,
		UsedBudgetFen:    c.UsedBudgetFen,
		Operator:         c.Operator,
		CreatedAt:        c.CreatedAt,
		UpdatedAt:        c.UpdatedAt,
	}
}

// ListCampaigns 返回活动一页。
func (s *Store) ListCampaigns(ctx context.Context, filter usecase.CampaignFilter) ([]domain.Campaign, int64, error) {
	query := bson.M{}
	if filter.Status != "" {
		query["status"] = filter.Status
	}
	if filter.Type != "" {
		query["type"] = filter.Type
	}
	if keyword := strings.TrimSpace(filter.Keyword); keyword != "" {
		query["$or"] = bson.A{
			bson.M{"code": bson.M{"$regex": regexp.QuoteMeta(keyword), "$options": "i"}},
			bson.M{"name": bson.M{"$regex": regexp.QuoteMeta(keyword), "$options": "i"}},
		}
	}
	total, err := s.db.Collection(colCampaigns).CountDocuments(ctx, query)
	if err != nil {
		return nil, 0, fmt.Errorf("count campaigns: %w", err)
	}
	opts := options.Find().SetSort(bson.D{{Key: "createdAt", Value: -1}}).
		SetSkip(int64(filter.Skip)).SetLimit(int64(filter.Limit))
	cursor, err := s.db.Collection(colCampaigns).Find(ctx, query, opts)
	if err != nil {
		return nil, 0, fmt.Errorf("list campaigns: %w", err)
	}
	defer cursor.Close(ctx)
	out := make([]domain.Campaign, 0)
	for cursor.Next(ctx) {
		var doc campaignDoc
		if err := cursor.Decode(&doc); err != nil {
			return nil, 0, fmt.Errorf("decode campaign: %w", err)
		}
		out = append(out, doc.toDomain())
	}
	if err := cursor.Err(); err != nil {
		return nil, 0, fmt.Errorf("iterate campaigns: %w", err)
	}
	return out, total, nil
}

// FindCampaign 返回一个活动。
func (s *Store) FindCampaign(ctx context.Context, code string) (domain.Campaign, error) {
	var doc campaignDoc
	err := s.db.Collection(colCampaigns).FindOne(ctx, bson.M{"code": strings.TrimSpace(code)}).Decode(&doc)
	if err != nil {
		if isNoDocuments(err) {
			return domain.Campaign{}, domain.ErrCampaignNotFound
		}
		return domain.Campaign{}, fmt.Errorf("find campaign: %w", err)
	}
	return doc.toDomain(), nil
}

// CreateCampaign 写入一个新活动。
func (s *Store) CreateCampaign(ctx context.Context, campaign domain.Campaign) (domain.Campaign, error) {
	doc := campaignFromDomain(campaign)
	if _, err := s.db.Collection(colCampaigns).InsertOne(ctx, doc); err != nil {
		return domain.Campaign{}, fmt.Errorf("create campaign: %w", err)
	}
	return s.FindCampaign(ctx, campaign.Code)
}

// UpdateCampaign 更新活动状态与预算消耗。
func (s *Store) UpdateCampaign(ctx context.Context, code string, set usecase.CampaignUpdate) (domain.Campaign, error) {
	update := bson.M{"$set": bson.M{"updatedAt": set.At}}
	if set.Status != "" {
		update["$set"].(bson.M)["status"] = set.Status
	}
	if set.Operator != "" {
		update["$set"].(bson.M)["operator"] = set.Operator
	}
	if set.UsedBudgetBy != 0 {
		update["$inc"] = bson.M{"usedBudget": set.UsedBudgetBy}
	}
	if _, err := s.db.Collection(colCampaigns).UpdateOne(ctx,
		bson.M{"code": strings.TrimSpace(code)}, update); err != nil {
		return domain.Campaign{}, fmt.Errorf("update campaign: %w", err)
	}
	return s.FindCampaign(ctx, code)
}
