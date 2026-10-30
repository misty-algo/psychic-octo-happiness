// A08 补贴台账的 MongoDB 适配。补贴落 campaign_subsidies，当日封顶聚合在
// campaign_subsidy_days，预算扣减直接 CAS campaigns.usedBudget。
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

const (
	colSubsidyRecords = "campaign_subsidies"
	colSubsidyDays    = "campaign_subsidy_days"
)

// subsidyRecordDoc 是补贴台账文档。
type subsidyRecordDoc struct {
	ID           bson.ObjectID `bson:"_id,omitempty"`
	RecordNo     string        `bson:"recordNo"`
	CampaignCode string        `bson:"campaignCode"`
	CampaignName string        `bson:"campaignName"`
	DriverID     string        `bson:"driverId"`
	OrderNo      string        `bson:"orderNo,omitempty"`
	AmountFen    int64         `bson:"amount"`
	BizType      string        `bson:"bizType"`
	Reason       string        `bson:"reason"`
	Day          string        `bson:"day"`
	Operator     string        `bson:"operator,omitempty"`
	CreatedAt    time.Time     `bson:"createdAt"`
	// 追回标记：冲正不改原记录，只追加这几个字段。
	Reversed      bool      `bson:"reversed,omitempty"`
	ReversedAt    time.Time `bson:"reversedAt,omitempty"`
	ReversedBy    string    `bson:"reversedBy,omitempty"`
	ReverseReason string    `bson:"reverseReason,omitempty"`
}

func subsidyFromDomain(r domain.SubsidyRecord) subsidyRecordDoc {
	return subsidyRecordDoc{
		RecordNo:     r.RecordNo,
		CampaignCode: r.CampaignCode,
		CampaignName: r.CampaignName,
		DriverID:     r.DriverID,
		OrderNo:      r.OrderNo,
		AmountFen:    r.AmountFen,
		BizType:      r.BizType,
		Reason:       r.Reason,
		Day:          r.Day,
		Operator:     r.Operator,
		CreatedAt:    r.CreatedAt,
		Reversed:     r.Reversed,
		ReversedAt:   r.ReversedAt,
		ReversedBy:   r.ReversedBy,
		ReverseReason: r.ReverseReason,
	}
}

func (d subsidyRecordDoc) toDomain() domain.SubsidyRecord {
	return domain.SubsidyRecord{
		RecordNo:     d.RecordNo,
		CampaignCode: d.CampaignCode,
		CampaignName: d.CampaignName,
		DriverID:     d.DriverID,
		OrderNo:      d.OrderNo,
		AmountFen:    d.AmountFen,
		BizType:      d.BizType,
		Reason:       d.Reason,
		Day:          d.Day,
		Operator:     d.Operator,
		CreatedAt:    d.CreatedAt,
		Reversed:     d.Reversed,
		ReversedAt:   d.ReversedAt,
		ReversedBy:   d.ReversedBy,
		ReverseReason: d.ReverseReason,
	}
}

// ReserveCampaignBudget 原子扣减活动预算：只在剩余额度足够时 inc，
// 条件更新失败就是预算不足，不写任何东西。
func (s *Store) ReserveCampaignBudget(ctx context.Context, code string, amountFen int64) (bool, error) {
	filter := bson.M{
		"code": strings.TrimSpace(code),
		"budget": bson.M{"$gte": amountFen}, // 负预算不该存在，双保险
		"$expr": bson.M{"$lte": bson.A{"$usedBudget", bson.M{"$subtract": bson.A{"$budget", amountFen}}}},
	}
	res, err := s.db.Collection(colCampaigns).UpdateOne(ctx, filter,
		bson.M{"$inc": bson.M{"usedBudget": amountFen}, "$set": bson.M{"updatedAt": time.Now()}})
	if err != nil {
		return false, fmt.Errorf("reserve campaign budget: %w", err)
	}
	return res.MatchedCount > 0, nil
}

// ReleaseCampaignBudget 回滚活动预算（日封顶失败时补偿）。
func (s *Store) ReleaseCampaignBudget(ctx context.Context, code string, amountFen int64) error {
	_, err := s.db.Collection(colCampaigns).UpdateOne(ctx,
		bson.M{"code": strings.TrimSpace(code)},
		bson.M{"$inc": bson.M{"usedBudget": -amountFen}, "$set": bson.M{"updatedAt": time.Now()}})
	if err != nil {
		return fmt.Errorf("release campaign budget: %w", err)
	}
	return nil
}

// CountSubsidiesByDriver 统计司机在活动下已领补贴次数。
func (s *Store) CountSubsidiesByDriver(ctx context.Context, campaignCode, driverID string) (int64, error) {
	n, err := s.db.Collection(colSubsidyRecords).CountDocuments(ctx, bson.M{
		"campaignCode": campaignCode,
		"driverId":     strings.TrimSpace(driverID),
	})
	if err != nil {
		return 0, fmt.Errorf("count subsidies by driver: %w", err)
	}
	return n, nil
}

// ReserveSubsidyDaily 原子占用活动当日补贴封顶额度。
// 当日聚合文档先幂等建出来，再用 CAS 条件更新防并发超发。
func (s *Store) ReserveSubsidyDaily(ctx context.Context, campaignCode, day string, capFen, amountFen int64) (bool, error) {
	key := campaignCode + "|" + day
	// 第一步：幂等初始化当日聚合（已存在则不动）。
	_, err := s.db.Collection(colSubsidyDays).UpdateOne(ctx,
		bson.M{"_id": key},
		bson.M{"$setOnInsert": bson.M{
			"campaignCode": campaignCode,
			"day":          day,
			"totalFen":     0,
			"count":        0,
		}},
		options.UpdateOne().SetUpsert(true))
	if err != nil {
		return false, fmt.Errorf("init subsidy daily: %w", err)
	}
	// 第二步：CAS 扣当日额度，剩余不够时 MatchedCount=0。
	filter := bson.M{"_id": key, "totalFen": bson.M{"$lte": capFen - amountFen}}
	res, err := s.db.Collection(colSubsidyDays).UpdateOne(ctx, filter,
		bson.M{"$inc": bson.M{"totalFen": amountFen, "count": 1}, "$set": bson.M{"updatedAt": time.Now()}})
	if err != nil {
		return false, fmt.Errorf("reserve subsidy daily: %w", err)
	}
	return res.MatchedCount > 0, nil
}

// InsertSubsidyRecord 写入一笔补贴台账。
func (s *Store) InsertSubsidyRecord(ctx context.Context, record domain.SubsidyRecord) error {
	_, err := s.db.Collection(colSubsidyRecords).InsertOne(ctx, subsidyFromDomain(record))
	if err != nil {
		return fmt.Errorf("insert subsidy record: %w", err)
	}
	return nil
}

// ListSubsidyRecords 返回补贴台账一页。
func (s *Store) ListSubsidyRecords(ctx context.Context, filter usecase.SubsidyRecordFilter) ([]domain.SubsidyRecord, int64, error) {
	query := bson.M{}
	if filter.CampaignCode != "" {
		query["campaignCode"] = filter.CampaignCode
	}
	if filter.DriverID != "" {
		query["driverId"] = filter.DriverID
	}
	if filter.BizType != "" {
		query["bizType"] = filter.BizType
	}
	if filter.Day != "" {
		query["day"] = filter.Day
	}
	col := s.db.Collection(colSubsidyRecords)
	total, err := col.CountDocuments(ctx, query)
	if err != nil {
		return nil, 0, fmt.Errorf("count subsidy records: %w", err)
	}
	opts := options.Find().SetSort(bson.D{{Key: "createdAt", Value: -1}}).
		SetSkip(int64(filter.Skip)).SetLimit(int64(filter.Limit))
	cursor, err := col.Find(ctx, query, opts)
	if err != nil {
		return nil, 0, fmt.Errorf("list subsidy records: %w", err)
	}
	defer cursor.Close(ctx)
	out := make([]domain.SubsidyRecord, 0)
	for cursor.Next(ctx) {
		var doc subsidyRecordDoc
		if err := cursor.Decode(&doc); err != nil {
			return nil, 0, fmt.Errorf("decode subsidy record: %w", err)
		}
		out = append(out, doc.toDomain())
	}
	if err := cursor.Err(); err != nil {
		return nil, 0, fmt.Errorf("iterate subsidy records: %w", err)
	}
	return out, total, nil
}

// FindSubsidyRecord 按流水号查一笔补贴。
func (s *Store) FindSubsidyRecord(ctx context.Context, recordNo string) (domain.SubsidyRecord, error) {
	var doc subsidyRecordDoc
	err := s.db.Collection(colSubsidyRecords).FindOne(ctx, bson.M{"recordNo": strings.TrimSpace(recordNo)}).Decode(&doc)
	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return domain.SubsidyRecord{}, domain.ErrSubsidyRecordNotFound
		}
		return domain.SubsidyRecord{}, fmt.Errorf("find subsidy record: %w", err)
	}
	return doc.toDomain(), nil
}

// MarkSubsidyReversed 把一笔补贴标记为已追回：台账只追加不修改，
// 所以这里只打追回标记与冲正说明，原始发放记录保留原样。
func (s *Store) MarkSubsidyReversed(ctx context.Context, record domain.SubsidyRecord) error {
	res, err := s.db.Collection(colSubsidyRecords).UpdateOne(ctx,
		bson.M{"recordNo": record.RecordNo, "reversed": bson.M{"$ne": true}},
		bson.M{"$set": bson.M{
			"reversed":      true,
			"reversedAt":    record.ReversedAt,
			"reversedBy":    record.ReversedBy,
			"reverseReason": record.ReverseReason,
		}},
	)
	if err != nil {
		return fmt.Errorf("mark subsidy reversed: %w", err)
	}
	if res.MatchedCount == 0 {
		return domain.ErrSubsidyAlreadyReversed
	}
	return nil
}

// SummarizeSubsidies 聚合补贴台账：累计发放、笔数、触达司机数与当日发放。
func (s *Store) SummarizeSubsidies(ctx context.Context, campaignCode, today string) (usecase.SubsidyAggregate, error) {
	col := s.db.Collection(colSubsidyRecords)
	// 总口径：总额 + 笔数 + 去重司机数。
	totalCursor, err := col.Aggregate(ctx, bson.A{
		bson.M{"$match": bson.M{"campaignCode": campaignCode}},
		bson.M{"$group": bson.M{
			"_id":         nil,
			"totalFen":    bson.M{"$sum": "$amount"},
			"recordCount": bson.M{"$sum": 1},
			"drivers":     bson.M{"$addToSet": "$driverId"},
		}},
	})
	if err != nil {
		return usecase.SubsidyAggregate{}, fmt.Errorf("aggregate subsidies: %w", err)
	}
	defer totalCursor.Close(ctx)
	var agg usecase.SubsidyAggregate
	if totalCursor.Next(ctx) {
		var row struct {
			TotalFen    int64    `bson:"totalFen"`
			RecordCount int64    `bson:"recordCount"`
			Drivers     []string `bson:"drivers"`
		}
		if err := totalCursor.Decode(&row); err != nil {
			return usecase.SubsidyAggregate{}, fmt.Errorf("decode subsidy aggregate: %w", err)
		}
		agg.GrantedTotalFen = row.TotalFen
		agg.RecordCount = row.RecordCount
		agg.DriverCount = int64(len(row.Drivers))
	}
	// 当日口径：只算今天的发放额。
	todayCursor, err := col.Aggregate(ctx, bson.A{
		bson.M{"$match": bson.M{"campaignCode": campaignCode, "day": today}},
		bson.M{"$group": bson.M{"_id": nil, "totalFen": bson.M{"$sum": "$amount"}}},
	})
	if err != nil {
		return usecase.SubsidyAggregate{}, fmt.Errorf("aggregate subsidies by day: %w", err)
	}
	defer todayCursor.Close(ctx)
	if todayCursor.Next(ctx) {
		var row struct {
			TotalFen int64 `bson:"totalFen"`
		}
		if err := todayCursor.Decode(&row); err != nil {
			return usecase.SubsidyAggregate{}, fmt.Errorf("decode subsidy daily aggregate: %w", err)
		}
		agg.TodayGrantedFen = row.TotalFen
	}
	return agg, nil
}
