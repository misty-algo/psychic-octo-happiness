// A05 券模板的 MongoDB 适配。字段沿用库内真实写入的命名（faceValueFen /
// thresholdFen / bearer / stock / perLimit / state），其中 state 是中文取值。
package mongo

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"

	"github.com/NSObjects/echo-admin/internal/modules/operations/domain"
	"github.com/NSObjects/echo-admin/internal/modules/operations/usecase"
)

// 营销中心用到的集合。
const (
	colCouponTemplates = "coupon_templates"
	colCouponBatches   = "coupon_batches"
	colCoupons         = "coupons"
	colCampaigns       = "campaigns"
)

// couponTemplateDoc 是券模板文档。模板编号与库存由本模块写入；业务侧已经
// 存在的模板没有 templateNo，读取时回落到 _id 的字符串形式。
type couponTemplateDoc struct {
	ID              bson.ObjectID `bson:"_id,omitempty"`
	TemplateNo      string        `bson:"templateNo,omitempty"`
	Name            string        `bson:"name"`
	CouponType      string        `bson:"couponType,omitempty"`
	FaceValueFen    int64         `bson:"faceValueFen"`
	ThresholdFen    int64         `bson:"thresholdFen"`
	DiscountRate    int           `bson:"discountRate,omitempty"`
	MaxDiscountFen  int64         `bson:"maxDiscountFen,omitempty"`
	Bearer          string        `bson:"bearer"`
	CityCodes       []string      `bson:"cityCodes,omitempty"`
	ProductCodes    []string      `bson:"productCodes,omitempty"`
	ValidStart      time.Time     `bson:"validStart"`
	ValidEnd        time.Time     `bson:"validEnd"`
	Stock           int64         `bson:"stock"`
	IssuedCount     int64         `bson:"issuedCount,omitempty"`
	UsedCount       int64         `bson:"usedCount,omitempty"`
	PerLimit        int64         `bson:"perLimit"`
	Stackable       bool          `bson:"stackable,omitempty"`
	MutualTemplates []string      `bson:"mutualTemplates,omitempty"`
	Description     string        `bson:"description,omitempty"`
	State           string        `bson:"state"`
	CreatedAt       time.Time     `bson:"createdAt,omitempty"`
	UpdatedAt       time.Time     `bson:"updatedAt,omitempty"`
}

// toDomain 转成领域对象。历史模板没有 templateNo，用 _id 兜底，保证老数据
// 也能出现在列表里并且能被发放。
func (d couponTemplateDoc) toDomain() domain.CouponTemplate {
	templateNo := d.TemplateNo
	if strings.TrimSpace(templateNo) == "" {
		templateNo = d.ID.Hex()
	}
	couponType := d.CouponType
	if couponType == "" {
		couponType = domain.CouponTypeCash
	}
	bearer := d.Bearer
	if bearer == "" {
		bearer = domain.CouponBearerPlatform
	}
	perLimit := d.PerLimit
	if perLimit <= 0 {
		perLimit = 1
	}
	// 业务侧历史模板没有 createdAt / updatedAt，用 _id 内嵌时间戳还原真实
	// 创建时间（ObjectID 前 4 字节即生成时间），避免列表出现 0001 年。
	createdAt := d.CreatedAt
	if createdAt.IsZero() && !d.ID.IsZero() {
		createdAt = d.ID.Timestamp().UTC()
	}
	updatedAt := d.UpdatedAt
	if updatedAt.IsZero() {
		updatedAt = createdAt
	}
	return domain.CouponTemplate{
		TemplateNo:      templateNo,
		Name:            d.Name,
		CouponType:      couponType,
		FaceValueFen:    d.FaceValueFen,
		ThresholdFen:    d.ThresholdFen,
		DiscountRate:    d.DiscountRate,
		MaxDiscountFen:  d.MaxDiscountFen,
		Bearer:          bearer,
		CityCodes:       d.CityCodes,
		ProductCodes:    d.ProductCodes,
		ValidStart:      d.ValidStart,
		ValidEnd:        d.ValidEnd,
		Stock:           d.Stock,
		IssuedCount:     d.IssuedCount,
		UsedCount:       d.UsedCount,
		PerLimit:        perLimit,
		Stackable:       d.Stackable,
		MutualTemplates: d.MutualTemplates,
		Description:     d.Description,
		State:           d.State,
		CreatedAt:       createdAt,
		UpdatedAt:       updatedAt,
	}
}

// fromDomain 把领域对象写成文档。_id 只在能解析成 ObjectId 时带上，用于更新
// 业务侧已有的模板。
func couponTemplateFromDomain(t domain.CouponTemplate) couponTemplateDoc {
	doc := couponTemplateDoc{
		TemplateNo:      t.TemplateNo,
		Name:            t.Name,
		CouponType:      t.CouponType,
		FaceValueFen:    t.FaceValueFen,
		ThresholdFen:    t.ThresholdFen,
		DiscountRate:    t.DiscountRate,
		MaxDiscountFen:  t.MaxDiscountFen,
		Bearer:          t.Bearer,
		CityCodes:       t.CityCodes,
		ProductCodes:    t.ProductCodes,
		ValidStart:      t.ValidStart,
		ValidEnd:        t.ValidEnd,
		Stock:           t.Stock,
		IssuedCount:     t.IssuedCount,
		UsedCount:       t.UsedCount,
		PerLimit:        t.PerLimit,
		Stackable:       t.Stackable,
		MutualTemplates: t.MutualTemplates,
		Description:     t.Description,
		State:           t.State,
		CreatedAt:       t.CreatedAt,
		UpdatedAt:       t.UpdatedAt,
	}
	if oid, err := bson.ObjectIDFromHex(t.TemplateNo); err == nil {
		doc.ID = oid
		doc.TemplateNo = ""
	}
	return doc
}

// templateFilter 定位一张模板：优先按 templateNo，兼容按 _id 定位的老数据。
func templateFilter(templateNo string) bson.M {
	templateNo = strings.TrimSpace(templateNo)
	if templateNo == "" {
		return bson.M{"_id": bson.ObjectID{}}
	}
	if oid, err := bson.ObjectIDFromHex(templateNo); err == nil {
		return bson.M{"$or": bson.A{bson.M{"templateNo": templateNo}, bson.M{"_id": oid}}}
	}
	return bson.M{"templateNo": templateNo}
}

// EnsureMarketingIndexes 建立营销中心需要的索引，幂等可重复调用。
//
// coupon_templates / coupons 里已经有业务侧写进来的历史文档，它们没有
// templateNo / couponNo 字段，因此唯一索引必须是 sparse：否则多条缺失字段的
// 文档会被当成重复的 null 而导致建索引失败。
func (s *Store) EnsureMarketingIndexes(ctx context.Context) error {
	templates := []mongo.IndexModel{
		{Keys: bson.D{{Key: "templateNo", Value: 1}}, Options: options.Index().SetUnique(true).SetSparse(true).SetName("uk_template_no")},
		{Keys: bson.D{{Key: "state", Value: 1}}, Options: options.Index().SetName("idx_coupon_template_state")},
	}
	batches := []mongo.IndexModel{
		{Keys: bson.D{{Key: "batchNo", Value: 1}}, Options: options.Index().SetUnique(true).SetName("uk_batch_no")},
		{Keys: bson.D{{Key: "campaignCode", Value: 1}}, Options: options.Index().SetName("idx_batch_campaign")},
		{Keys: bson.D{{Key: "status", Value: 1}}, Options: options.Index().SetName("idx_batch_status")},
	}
	coupons := []mongo.IndexModel{
		{Keys: bson.D{{Key: "couponNo", Value: 1}}, Options: options.Index().SetUnique(true).SetSparse(true).SetName("uk_coupon_no")},
		{Keys: bson.D{{Key: "batchNo", Value: 1}}, Options: options.Index().SetName("idx_coupon_batch")},
		{Keys: bson.D{{Key: "templateNo", Value: 1}, {Key: "state", Value: 1}}, Options: options.Index().SetName("idx_coupon_template_state")},
	}
	campaigns := []mongo.IndexModel{
		{Keys: bson.D{{Key: "code", Value: 1}}, Options: options.Index().SetUnique(true).SetName("uk_campaign_code")},
		{Keys: bson.D{{Key: "status", Value: 1}}, Options: options.Index().SetName("idx_campaign_status")},
	}
	for name, pair := range map[string][]mongo.IndexModel{
		colCouponTemplates: templates,
		colCouponBatches:   batches,
		colCoupons:         coupons,
		colCampaigns:       campaigns,
	} {
		if _, err := s.db.Collection(name).Indexes().CreateMany(ctx, pair); err != nil {
			return fmt.Errorf("create indexes for %s: %w", name, err)
		}
	}
	return nil
}

// ---- 券模板读写 ----

// ListCouponTemplates 返回券模板一页。
func (s *Store) ListCouponTemplates(ctx context.Context, filter usecase.CouponTemplateFilter) ([]domain.CouponTemplate, int64, error) {
	query := bson.M{}
	if filter.State != "" {
		query["state"] = filter.State
	}
	if filter.Type != "" {
		query["couponType"] = filter.Type
	}
	if keyword := strings.TrimSpace(filter.Keyword); keyword != "" {
		query["name"] = bson.M{"$regex": regexp.QuoteMeta(keyword), "$options": "i"}
	}
	total, err := s.db.Collection(colCouponTemplates).CountDocuments(ctx, query)
	if err != nil {
		return nil, 0, fmt.Errorf("count coupon templates: %w", err)
	}
	opts := options.Find().SetSort(bson.D{{Key: "createdAt", Value: -1}}).
		SetSkip(int64(filter.Skip)).SetLimit(int64(filter.Limit))
	cursor, err := s.db.Collection(colCouponTemplates).Find(ctx, query, opts)
	if err != nil {
		return nil, 0, fmt.Errorf("list coupon templates: %w", err)
	}
	defer cursor.Close(ctx)
	out := make([]domain.CouponTemplate, 0)
	for cursor.Next(ctx) {
		var doc couponTemplateDoc
		if err := cursor.Decode(&doc); err != nil {
			return nil, 0, fmt.Errorf("decode coupon template: %w", err)
		}
		out = append(out, doc.toDomain())
	}
	if err := cursor.Err(); err != nil {
		return nil, 0, fmt.Errorf("iterate coupon templates: %w", err)
	}
	return out, total, nil
}

// FindCouponTemplate 返回一张券模板。
func (s *Store) FindCouponTemplate(ctx context.Context, templateNo string) (domain.CouponTemplate, error) {
	var doc couponTemplateDoc
	err := s.db.Collection(colCouponTemplates).FindOne(ctx, templateFilter(templateNo)).Decode(&doc)
	if err != nil {
		if isNoDocuments(err) {
			return domain.CouponTemplate{}, domain.ErrCouponTemplateNotFound
		}
		return domain.CouponTemplate{}, fmt.Errorf("find coupon template: %w", err)
	}
	return doc.toDomain(), nil
}

// FindCouponTemplates 批量取模板定义。返回的 map 同时以 templateNo 与 _id 的
// 十六进制为键：历史券只存 templateId，互斥判定按券上的编号回查时要能命中。
func (s *Store) FindCouponTemplates(ctx context.Context, templateNos []string) (map[string]domain.CouponTemplate, error) {
	out := make(map[string]domain.CouponTemplate, len(templateNos))
	nos := make([]string, 0, len(templateNos))
	ids := make([]bson.ObjectID, 0, len(templateNos))
	for _, no := range templateNos {
		if strings.TrimSpace(no) == "" {
			continue
		}
		nos = append(nos, no)
		if oid, err := bson.ObjectIDFromHex(no); err == nil {
			ids = append(ids, oid)
		}
	}
	if len(nos) == 0 {
		return out, nil
	}
	or := []bson.M{{"templateNo": bson.M{"$in": nos}}}
	if len(ids) > 0 {
		or = append(or, bson.M{"_id": bson.M{"$in": ids}})
	}
	cursor, err := s.db.Collection(colCouponTemplates).Find(ctx, bson.M{"$or": or})
	if err != nil {
		return nil, fmt.Errorf("find coupon templates: %w", err)
	}
	defer cursor.Close(ctx)
	for cursor.Next(ctx) {
		var doc couponTemplateDoc
		if err := cursor.Decode(&doc); err != nil {
			return nil, fmt.Errorf("decode coupon template: %w", err)
		}
		tpl := doc.toDomain()
		out[tpl.TemplateNo] = tpl
		out[doc.ID.Hex()] = tpl
	}
	if err := cursor.Err(); err != nil {
		return nil, fmt.Errorf("iterate coupon templates: %w", err)
	}
	return out, nil
}

// CreateCouponTemplate 写入一张新模板。
func (s *Store) CreateCouponTemplate(ctx context.Context, template domain.CouponTemplate) (domain.CouponTemplate, error) {
	doc := couponTemplateFromDomain(template)
	doc.ID = bson.ObjectID{}
	doc.TemplateNo = template.TemplateNo
	if _, err := s.db.Collection(colCouponTemplates).InsertOne(ctx, doc); err != nil {
		return domain.CouponTemplate{}, fmt.Errorf("create coupon template: %w", err)
	}
	return s.FindCouponTemplate(ctx, template.TemplateNo)
}

// UpdateCouponTemplate 更新模板状态与计数。
func (s *Store) UpdateCouponTemplate(ctx context.Context, templateNo string, set usecase.CouponTemplateUpdate) (domain.CouponTemplate, error) {
	update := bson.M{"$set": bson.M{"updatedAt": set.At}}
	if set.State != "" {
		update["$set"].(bson.M)["state"] = set.State
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
	if _, err := s.db.Collection(colCouponTemplates).UpdateOne(ctx, templateFilter(templateNo), update); err != nil {
		return domain.CouponTemplate{}, fmt.Errorf("update coupon template: %w", err)
	}
	return s.FindCouponTemplate(ctx, templateNo)
}

// ConsumeTemplateStock 原子扣减库存。这是防超发的第二道闸门：只有剩余库存
// 足够时才扣，失败意味着被并发抢完了，返回 false 且一个字节都不写。
func (s *Store) ConsumeTemplateStock(ctx context.Context, templateNo string, count int64, at time.Time) (bool, error) {
	if count <= 0 {
		return false, nil
	}
	filter := templateFilter(templateNo)
	filter["stock"] = bson.M{"$gte": count}
	result, err := s.db.Collection(colCouponTemplates).UpdateOne(ctx, filter, bson.M{
		"$inc": bson.M{"stock": -count, "issuedCount": count},
		"$set": bson.M{"updatedAt": at},
	})
	if err != nil {
		return false, fmt.Errorf("consume template stock: %w", err)
	}
	return result.ModifiedCount == 1, nil
}

// RestoreTemplateStock 回滚库存与已发计数。
func (s *Store) RestoreTemplateStock(ctx context.Context, templateNo string, count int64) error {
	if count <= 0 {
		return nil
	}
	if _, err := s.db.Collection(colCouponTemplates).UpdateOne(ctx, templateFilter(templateNo), bson.M{
		"$inc": bson.M{"stock": count, "issuedCount": -count},
	}); err != nil {
		return fmt.Errorf("restore template stock: %w", err)
	}
	return nil
}
