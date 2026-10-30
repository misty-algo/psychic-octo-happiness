// A05 券批次的 MongoDB 适配。额度占用必须是条件更新，靠它防超发。
package mongo

import (
	"context"
	"fmt"
	"strings"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo/options"

	"github.com/NSObjects/echo-admin/internal/modules/operations/domain"
	"github.com/NSObjects/echo-admin/internal/modules/operations/usecase"
)

// couponBatchDoc 是券批次文档。
type couponBatchDoc struct {
	ID            bson.ObjectID `bson:"_id,omitempty"`
	BatchNo       string        `bson:"batchNo"`
	TemplateNo    string        `bson:"templateNo"`
	TemplateName  string        `bson:"templateName,omitempty"`
	TotalCount    int64         `bson:"totalCount"`
	IssuedCount   int64         `bson:"issuedCount"`
	UsedCount     int64         `bson:"usedCount"`
	Channel       string        `bson:"channel"`
	BudgetFen     int64         `bson:"budgetFen"`
	UsedBudgetFen int64         `bson:"usedBudgetFen"`
	Status        string        `bson:"status"`
	StartAt       time.Time     `bson:"startAt"`
	EndAt         time.Time     `bson:"endAt"`
	CampaignCode  string        `bson:"campaignCode,omitempty"`
	Operator      string        `bson:"operator,omitempty"`
	CreatedAt     time.Time     `bson:"createdAt"`
	UpdatedAt     time.Time     `bson:"updatedAt"`
}

func (d couponBatchDoc) toDomain() domain.CouponBatch {
	return domain.CouponBatch{
		BatchNo:       d.BatchNo,
		TemplateNo:    d.TemplateNo,
		TemplateName:  d.TemplateName,
		TotalCount:    d.TotalCount,
		IssuedCount:   d.IssuedCount,
		UsedCount:     d.UsedCount,
		Channel:       d.Channel,
		BudgetFen:     d.BudgetFen,
		UsedBudgetFen: d.UsedBudgetFen,
		Status:        d.Status,
		StartAt:       d.StartAt,
		EndAt:         d.EndAt,
		CampaignCode:  d.CampaignCode,
		Operator:      d.Operator,
		CreatedAt:     d.CreatedAt,
		UpdatedAt:     d.UpdatedAt,
	}
}

func couponBatchFromDomain(b domain.CouponBatch) couponBatchDoc {
	return couponBatchDoc{
		BatchNo:       b.BatchNo,
		TemplateNo:    b.TemplateNo,
		TemplateName:  b.TemplateName,
		TotalCount:    b.TotalCount,
		IssuedCount:   b.IssuedCount,
		UsedCount:     b.UsedCount,
		Channel:       b.Channel,
		BudgetFen:     b.BudgetFen,
		UsedBudgetFen: b.UsedBudgetFen,
		Status:        b.Status,
		StartAt:       b.StartAt,
		EndAt:         b.EndAt,
		CampaignCode:  b.CampaignCode,
		Operator:      b.Operator,
		CreatedAt:     b.CreatedAt,
		UpdatedAt:     b.UpdatedAt,
	}
}

// ListCouponBatches 返回券批次一页。
func (s *Store) ListCouponBatches(ctx context.Context, filter usecase.CouponBatchFilter) ([]domain.CouponBatch, int64, error) {
	query := bson.M{}
	if filter.TemplateNo != "" {
		query["templateNo"] = filter.TemplateNo
	}
	if filter.Channel != "" {
		query["channel"] = filter.Channel
	}
	if filter.Status != "" {
		query["status"] = filter.Status
	}
	if filter.CampaignCode != "" {
		query["campaignCode"] = filter.CampaignCode
	}
	total, err := s.db.Collection(colCouponBatches).CountDocuments(ctx, query)
	if err != nil {
		return nil, 0, fmt.Errorf("count coupon batches: %w", err)
	}
	opts := options.Find().SetSort(bson.D{{Key: "createdAt", Value: -1}}).
		SetSkip(int64(filter.Skip)).SetLimit(int64(filter.Limit))
	cursor, err := s.db.Collection(colCouponBatches).Find(ctx, query, opts)
	if err != nil {
		return nil, 0, fmt.Errorf("list coupon batches: %w", err)
	}
	defer cursor.Close(ctx)
	out := make([]domain.CouponBatch, 0)
	for cursor.Next(ctx) {
		var doc couponBatchDoc
		if err := cursor.Decode(&doc); err != nil {
			return nil, 0, fmt.Errorf("decode coupon batch: %w", err)
		}
		out = append(out, doc.toDomain())
	}
	if err := cursor.Err(); err != nil {
		return nil, 0, fmt.Errorf("iterate coupon batches: %w", err)
	}
	return out, total, nil
}

// FindCouponBatch 返回一个券批次。
func (s *Store) FindCouponBatch(ctx context.Context, batchNo string) (domain.CouponBatch, error) {
	var doc couponBatchDoc
	err := s.db.Collection(colCouponBatches).FindOne(ctx, bson.M{"batchNo": strings.TrimSpace(batchNo)}).Decode(&doc)
	if err != nil {
		if isNoDocuments(err) {
			return domain.CouponBatch{}, domain.ErrCouponBatchNotFound
		}
		return domain.CouponBatch{}, fmt.Errorf("find coupon batch: %w", err)
	}
	return doc.toDomain(), nil
}

// CreateCouponBatch 写入一个新批次。
func (s *Store) CreateCouponBatch(ctx context.Context, batch domain.CouponBatch) (domain.CouponBatch, error) {
	doc := couponBatchFromDomain(batch)
	if _, err := s.db.Collection(colCouponBatches).InsertOne(ctx, doc); err != nil {
		return domain.CouponBatch{}, fmt.Errorf("create coupon batch: %w", err)
	}
	return s.FindCouponBatch(ctx, batch.BatchNo)
}

// UpdateCouponBatch 更新批次状态与计数。
func (s *Store) UpdateCouponBatch(ctx context.Context, batchNo string, set usecase.CouponBatchUpdate) (domain.CouponBatch, error) {
	update := bson.M{"$set": bson.M{"updatedAt": set.At}}
	if set.Status != "" {
		update["$set"].(bson.M)["status"] = set.Status
	}
	if set.IssuedBy != 0 {
		update["$inc"] = bson.M{"issuedCount": set.IssuedBy}
	}
	if set.UsedBy != 0 {
		inc, _ := update["$inc"].(bson.M)
		if inc == nil {
			inc = bson.M{}
		}
		inc["usedCount"] = set.UsedBy
		update["$inc"] = inc
	}
	if _, err := s.db.Collection(colCouponBatches).UpdateOne(ctx,
		bson.M{"batchNo": strings.TrimSpace(batchNo)}, update); err != nil {
		return domain.CouponBatch{}, fmt.Errorf("update coupon batch: %w", err)
	}
	return s.FindCouponBatch(ctx, batchNo)
}

// ReserveBatchQuota 原子占用发行额度。
//
// 条件更新是关键：只有 issuedCount + count <= totalCount 才自增。并发下发时
// 后来者会匹配不到文档而返回 ModifiedCount=0，即"超发被拦下"。先读后写在
// 并发下一定会超发，所以这里绝不能拆成两步。
func (s *Store) ReserveBatchQuota(ctx context.Context, batchNo string, count int64, at time.Time) (bool, error) {
	if count <= 0 {
		return false, nil
	}
	filter := bson.M{
		"batchNo": strings.TrimSpace(batchNo),
		"status":  domain.CouponBatchStateRunning,
		"$expr": bson.M{"$lte": bson.A{
			bson.M{"$add": bson.A{"$issuedCount", count}},
			"$totalCount",
		}},
	}
	result, err := s.db.Collection(colCouponBatches).UpdateOne(ctx, filter, bson.M{
		"$inc": bson.M{"issuedCount": count},
		"$set": bson.M{"updatedAt": at},
	})
	if err != nil {
		return false, fmt.Errorf("reserve batch quota: %w", err)
	}
	return result.ModifiedCount == 1, nil
}

// ReleaseBatchQuota 回滚发行额度。
func (s *Store) ReleaseBatchQuota(ctx context.Context, batchNo string, count int64) error {
	if count <= 0 {
		return nil
	}
	if _, err := s.db.Collection(colCouponBatches).UpdateOne(ctx,
		bson.M{"batchNo": strings.TrimSpace(batchNo)},
		bson.M{"$inc": bson.M{"issuedCount": -count}}); err != nil {
		return fmt.Errorf("release batch quota: %w", err)
	}
	return nil
}

// ListBatchesByCampaign 取活动下所有批次，用于汇总对账。
func (s *Store) ListBatchesByCampaign(ctx context.Context, campaignCode string) ([]domain.CouponBatch, error) {
	cursor, err := s.db.Collection(colCouponBatches).Find(ctx,
		bson.M{"campaignCode": strings.TrimSpace(campaignCode)},
		options.Find().SetSort(bson.D{{Key: "createdAt", Value: 1}}))
	if err != nil {
		return nil, fmt.Errorf("list batches by campaign: %w", err)
	}
	defer cursor.Close(ctx)
	out := make([]domain.CouponBatch, 0)
	for cursor.Next(ctx) {
		var doc couponBatchDoc
		if err := cursor.Decode(&doc); err != nil {
			return nil, fmt.Errorf("decode coupon batch: %w", err)
		}
		out = append(out, doc.toDomain())
	}
	return out, cursor.Err()
}
