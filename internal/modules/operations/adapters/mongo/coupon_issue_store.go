// A05 已发放券的 MongoDB 适配：写入、查询与统计。
package mongo

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"

	"github.com/NSObjects/echo-admin/internal/modules/operations/domain"
	"github.com/NSObjects/echo-admin/internal/modules/operations/usecase"
)

// couponDoc 是已发放券文档。coupons 是业务侧既有集合，索引
// idx_passenger_state{passengerId,state} 与 ttl_expire{expireAt} 已经存在，
// 本模块补 couponNo / templateNo / batchNo 用于券码与对账。
type couponDoc struct {
	ID           bson.ObjectID `bson:"_id,omitempty"`
	CouponNo     string        `bson:"couponNo,omitempty"`
	TemplateNo   string        `bson:"templateNo,omitempty"`
	TemplateID   bson.ObjectID `bson:"templateId,omitempty"`
	Name         string        `bson:"name,omitempty"`
	BatchNo      string        `bson:"batchNo,omitempty"`
	PassengerID  bson.ObjectID `bson:"passengerId"`
	State        string        `bson:"state"`
	FaceValueFen int64         `bson:"faceValueFen,omitempty"`
	ThresholdFen int64         `bson:"thresholdFen,omitempty"`
	Bearer       string        `bson:"bearer,omitempty"`
	Channel      string        `bson:"channel,omitempty"`
	IssuedAt     time.Time     `bson:"issuedAt,omitempty"`
	ExpireAt     time.Time     `bson:"expireAt"`
	UsedAt       *time.Time    `bson:"usedAt,omitempty"`
	OrderNo      string        `bson:"orderNo,omitempty"`
	CreatedAt    time.Time     `bson:"createdAt"`
}

func (d couponDoc) toDomain() domain.IssuedCoupon {
	// 业务侧历史券没有 couponNo / templateNo，只有 _id 与 templateId。
	// 券码回落到 _id（唯一且稳定），模板编号回落到 templateId 的十六进制，
	// 这样老券在列表里不会是空行，限领与互斥也能按同一编号空间统计。
	couponNo := strings.TrimSpace(d.CouponNo)
	if couponNo == "" {
		couponNo = d.ID.Hex()
	}
	templateNo := strings.TrimSpace(d.TemplateNo)
	if templateNo == "" && !d.TemplateID.IsZero() {
		templateNo = d.TemplateID.Hex()
	}
	return domain.IssuedCoupon{
		CouponNo:     couponNo,
		TemplateNo:   templateNo,
		Name:         d.Name,
		BatchNo:      d.BatchNo,
		PassengerID:  d.PassengerID.Hex(),
		State:        d.State,
		FaceValueFen: d.FaceValueFen,
		ThresholdFen: d.ThresholdFen,
		Bearer:       d.Bearer,
		Channel:      d.Channel,
		IssuedAt:     issuedAtOf(d),
		ExpireAt:     d.ExpireAt,
		UsedAt:       d.UsedAt,
		OrderNo:      d.OrderNo,
	}
}

// issuedAtOf 兼容历史券只有 createdAt 的情况。
func issuedAtOf(d couponDoc) time.Time {
	if !d.IssuedAt.IsZero() {
		return d.IssuedAt
	}
	return d.CreatedAt
}

func couponFromDomain(c domain.IssuedCoupon) couponDoc {
	doc := couponDoc{
		CouponNo:     c.CouponNo,
		TemplateNo:   c.TemplateNo,
		Name:         c.Name,
		BatchNo:      c.BatchNo,
		State:        c.State,
		FaceValueFen: c.FaceValueFen,
		ThresholdFen: c.ThresholdFen,
		Bearer:       c.Bearer,
		Channel:      c.Channel,
		IssuedAt:     c.IssuedAt,
		ExpireAt:     c.ExpireAt,
		UsedAt:       c.UsedAt,
		OrderNo:      c.OrderNo,
		CreatedAt:    c.IssuedAt,
	}
	if oid, err := bson.ObjectIDFromHex(c.PassengerID); err == nil {
		doc.PassengerID = oid
	}
	if oid, err := bson.ObjectIDFromHex(c.TemplateNo); err == nil {
		doc.TemplateID = oid
	}
	return doc
}

// newCouponNo 生成券码：CP + 日期 + 8 位随机十六进制，靠 uk_coupon_no
// 唯一索引兜底，撞码时上层会重试。
func newCouponNo(now time.Time) (string, error) {
	var buf [4]byte
	if _, err := rand.Read(buf[:]); err != nil {
		return "", fmt.Errorf("generate coupon no: %w", err)
	}
	return "CP" + now.Format("20060102") + strings.ToUpper(hex.EncodeToString(buf[:])), nil
}

// InsertCoupons 批量写券并把生成的券码回填到入参，调用方因此能拿到券码回显。
// 撞上唯一索引时逐条重试，绝不静默丢券。
func (s *Store) InsertCoupons(ctx context.Context, coupons []domain.IssuedCoupon) (int, error) {
	if len(coupons) == 0 {
		return 0, nil
	}
	now := time.Now()
	collection := s.db.Collection(colCoupons)
	docs := make([]any, 0, len(coupons))
	for i := range coupons {
		no, err := newCouponNo(now)
		if err != nil {
			return 0, err
		}
		coupons[i].CouponNo = no
		docs = append(docs, couponFromDomain(coupons[i]))
	}
	res, err := collection.InsertMany(ctx, docs, options.InsertMany().SetOrdered(false))
	if err == nil {
		return len(res.InsertedIDs), nil
	}
	if !isDuplicateKey(err) {
		return 0, fmt.Errorf("insert coupons: %w", err)
	}
	// 撞码：逐条重来，每条最多三次，仍失败就报错，不让券凭空消失。
	written := 0
	for i := range coupons {
		var lastErr error
		for attempt := 0; attempt < 3; attempt++ {
			no, genErr := newCouponNo(now)
			if genErr != nil {
				return written, genErr
			}
			coupons[i].CouponNo = no
			if _, lastErr = collection.InsertOne(ctx, couponFromDomain(coupons[i])); lastErr == nil {
				written++
				break
			}
		}
		if lastErr != nil && !isDuplicateKey(lastErr) {
			return written, fmt.Errorf("insert coupon: %w", lastErr)
		}
	}
	return written, nil
}

// ListIssuedCoupons 查询已发放券。
func (s *Store) ListIssuedCoupons(ctx context.Context, filter usecase.IssuedCouponFilter) ([]domain.IssuedCoupon, int64, error) {
	query := bson.M{}
	if filter.BatchNo != "" {
		query["batchNo"] = filter.BatchNo
	}
	if filter.TemplateNo != "" {
		query["$or"] = templateRefOr(filter.TemplateNo)
	}
	if filter.State != "" {
		query["state"] = filter.State
	}
	if filter.PassengerID != "" {
		if oid, err := bson.ObjectIDFromHex(filter.PassengerID); err == nil {
			query["passengerId"] = oid
		} else {
			query["passengerId"] = filter.PassengerID
		}
	}
	// 手机号筛选要先在 accounts 里定位乘客，库内 coupons 不存手机号。
	if filter.Phone != "" {
		ids, err := s.resolvePassengerIDs(ctx, filter.Phone)
		if err != nil {
			return nil, 0, err
		}
		if len(ids) == 0 {
			return []domain.IssuedCoupon{}, 0, nil
		}
		query["passengerId"] = bson.M{"$in": ids}
	}
	total, err := s.db.Collection(colCoupons).CountDocuments(ctx, query)
	if err != nil {
		return nil, 0, fmt.Errorf("count coupons: %w", err)
	}
	opts := options.Find().SetSort(bson.D{{Key: "createdAt", Value: -1}}).
		SetSkip(int64(filter.Skip)).SetLimit(int64(filter.Limit))
	cursor, err := s.db.Collection(colCoupons).Find(ctx, query, opts)
	if err != nil {
		return nil, 0, fmt.Errorf("list coupons: %w", err)
	}
	defer cursor.Close(ctx)
	out := make([]domain.IssuedCoupon, 0)
	for cursor.Next(ctx) {
		var doc couponDoc
		if err := cursor.Decode(&doc); err != nil {
			return nil, 0, fmt.Errorf("decode coupon: %w", err)
		}
		out = append(out, doc.toDomain())
	}
	if err := cursor.Err(); err != nil {
		return nil, 0, fmt.Errorf("iterate coupons: %w", err)
	}
	// 模板信息与乘客信息补齐，列表里要能看清是什么券、发给了谁。
	if err := s.fillCouponTemplates(ctx, out); err != nil {
		return nil, 0, err
	}
	if err := s.fillCouponPassengers(ctx, out); err != nil {
		return nil, 0, err
	}
	return out, total, nil
}

// templateRefOr 让模板筛选同时命中本模块写入的 templateNo 与业务侧既有券
// 的 templateId：mentions 历史券没有 templateNo，只按 templateNo 查会漏。
func templateRefOr(templateNo string) []bson.M {
	or := []bson.M{{"templateNo": templateNo}}
	if oid, err := bson.ObjectIDFromHex(templateNo); err == nil {
		or = append(or, bson.M{"templateId": oid})
	}
	return or
}

// FindIssuedCoupon 按券码查一张已发放券。
func (s *Store) FindIssuedCoupon(ctx context.Context, couponNo string) (domain.IssuedCoupon, error) {
	var doc couponDoc
	query := bson.M{"couponNo": strings.TrimSpace(couponNo)}
	// 历史券没有 couponNo，券码就是 _id：两种键都要认。
	if oid, err := bson.ObjectIDFromHex(strings.TrimSpace(couponNo)); err == nil {
		query = bson.M{"$or": bson.A{bson.M{"couponNo": couponNo}, bson.M{"_id": oid}}}
	}
	err := s.db.Collection(colCoupons).FindOne(ctx, query).Decode(&doc)
	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return domain.IssuedCoupon{}, domain.ErrCouponNotFound
		}
		return domain.IssuedCoupon{}, fmt.Errorf("find coupon: %w", err)
	}
	return doc.toDomain(), nil
}

// VoidIssuedCoupon 作废一张券：只有 AVAILABLE 能作废，条件更新天然防并发重复作废。
func (s *Store) VoidIssuedCoupon(ctx context.Context, couponNo string) (bool, error) {
	query := bson.M{"couponNo": strings.TrimSpace(couponNo), "state": domain.CouponStateAvailable}
	if oid, err := bson.ObjectIDFromHex(strings.TrimSpace(couponNo)); err == nil {
		query = bson.M{
			"$or":   bson.A{bson.M{"couponNo": couponNo}, bson.M{"_id": oid}},
			"state": domain.CouponStateAvailable,
		}
	}
	res, err := s.db.Collection(colCoupons).UpdateOne(ctx, query,
		bson.M{"$set": bson.M{"state": domain.CouponStateVoid, "updatedAt": time.Now()}})
	if err != nil {
		return false, fmt.Errorf("void coupon: %w", err)
	}
	return res.MatchedCount > 0, nil
}

// fillCouponTemplates 按 templateId / templateNo 补齐模板名称、面额与承担方。
// 业务侧历史券只有 templateId，两个键都要匹配。
func (s *Store) fillCouponTemplates(ctx context.Context, coupons []domain.IssuedCoupon) error {
	if len(coupons) == 0 {
		return nil
	}
	nos := make([]string, 0, len(coupons))
	ids := make([]bson.ObjectID, 0, len(coupons))
	seenNo := make(map[string]struct{}, len(coupons))
	seenID := make(map[string]struct{}, len(coupons))
	for _, c := range coupons {
		if c.TemplateNo == "" {
			continue
		}
		if _, ok := seenNo[c.TemplateNo]; !ok {
			seenNo[c.TemplateNo] = struct{}{}
			nos = append(nos, c.TemplateNo)
		}
		if oid, err := bson.ObjectIDFromHex(c.TemplateNo); err == nil {
			if _, ok := seenID[oid.Hex()]; !ok {
				seenID[oid.Hex()] = struct{}{}
				ids = append(ids, oid)
			}
		}
	}
	if len(nos) == 0 {
		return nil
	}
	or := []bson.M{{"templateNo": bson.M{"$in": nos}}}
	if len(ids) > 0 {
		or = append(or, bson.M{"_id": bson.M{"$in": ids}})
	}
	cursor, err := s.db.Collection(colCouponTemplates).Find(ctx, bson.M{"$or": or})
	if err != nil {
		return fmt.Errorf("load coupon templates: %w", err)
	}
	defer cursor.Close(ctx)
	byNo := make(map[string]couponTemplateDoc, len(nos))
	byID := make(map[string]couponTemplateDoc, len(ids))
	for cursor.Next(ctx) {
		var doc couponTemplateDoc
		if err := cursor.Decode(&doc); err != nil {
			return fmt.Errorf("decode coupon template: %w", err)
		}
		byID[doc.ID.Hex()] = doc
		if doc.TemplateNo != "" {
			byNo[doc.TemplateNo] = doc
		}
	}
	if err := cursor.Err(); err != nil {
		return fmt.Errorf("iterate coupon templates: %w", err)
	}
	for i := range coupons {
		doc, ok := byNo[coupons[i].TemplateNo]
		if !ok {
			doc, ok = byID[coupons[i].TemplateNo]
		}
		if !ok {
			continue
		}
		tpl := doc.toDomain()
		coupons[i].TemplateName = tpl.Name
		if coupons[i].FaceValueFen == 0 {
			coupons[i].FaceValueFen = tpl.FaceValueFen
		}
		if coupons[i].ThresholdFen == 0 {
			coupons[i].ThresholdFen = tpl.ThresholdFen
		}
		if coupons[i].Bearer == "" {
			coupons[i].Bearer = tpl.Bearer
		}
	}
	return nil
}

// fillCouponPassengers 按 passengerId 补齐姓名与掩码手机号。
func (s *Store) fillCouponPassengers(ctx context.Context, coupons []domain.IssuedCoupon) error {
	if len(coupons) == 0 {
		return nil
	}
	seen := make(map[string]struct{}, len(coupons))
	oids := make([]bson.ObjectID, 0, len(coupons))
	for _, c := range coupons {
		if _, ok := seen[c.PassengerID]; ok {
			continue
		}
		seen[c.PassengerID] = struct{}{}
		if oid, err := bson.ObjectIDFromHex(c.PassengerID); err == nil {
			oids = append(oids, oid)
		}
	}
	if len(oids) == 0 {
		return nil
	}
	cursor, err := s.db.Collection(colAccounts).Find(ctx, bson.M{"_id": bson.M{"$in": oids}})
	if err != nil {
		return fmt.Errorf("load coupon passengers: %w", err)
	}
	defer cursor.Close(ctx)
	profiles := make(map[string]accountDoc)
	for cursor.Next(ctx) {
		var doc accountDoc
		if err := cursor.Decode(&doc); err != nil {
			return fmt.Errorf("decode account: %w", err)
		}
		profiles[doc.ID.Hex()] = doc
	}
	for i := range coupons {
		if doc, ok := profiles[coupons[i].PassengerID]; ok {
			account := doc.toDomain()
			coupons[i].Passenger = account.RealName
			coupons[i].PhoneMask = account.PhoneMask()
		}
	}
	return cursor.Err()
}

// CountHeldCoupons 统计每个乘客对该模板已持有的券数（含已用，限领按领取次数算）。
func (s *Store) CountHeldCoupons(ctx context.Context, passengerIDs []string, templateNo string) (map[string]int64, error) {
	out := make(map[string]int64)
	oids := objectIDs(passengerIDs)
	if len(oids) == 0 || strings.TrimSpace(templateNo) == "" {
		return out, nil
	}
	cursor, err := s.db.Collection(colCoupons).Aggregate(ctx, []bson.M{
		{"$match": bson.M{"passengerId": bson.M{"$in": oids}, "$or": templateRefOr(templateNo)}},
		{"$group": bson.M{"_id": "$passengerId", "count": bson.M{"$sum": 1}}},
	})
	if err != nil {
		return nil, fmt.Errorf("count held coupons: %w", err)
	}
	defer cursor.Close(ctx)
	for cursor.Next(ctx) {
		var row struct {
			ID    bson.ObjectID `bson:"_id"`
			Count int64         `bson:"count"`
		}
		if err := cursor.Decode(&row); err != nil {
			return nil, fmt.Errorf("decode held count: %w", err)
		}
		out[row.ID.Hex()] = row.Count
	}
	return out, cursor.Err()
}

// ListHeldTemplateNos 返回乘客当前可用券对应的模板编号，用于叠加互斥判定。
func (s *Store) ListHeldTemplateNos(ctx context.Context, passengerIDs []string) (map[string][]string, error) {
	out := make(map[string][]string)
	oids := objectIDs(passengerIDs)
	if len(oids) == 0 {
		return out, nil
	}
	cursor, err := s.db.Collection(colCoupons).Aggregate(ctx, []bson.M{
		{"$match": bson.M{"passengerId": bson.M{"$in": oids}, "state": domain.CouponStateAvailable}},
		// 历史券只有 templateId，统一折算成编号后再判定互斥，否则老券
		// 不参与叠加互斥，会出现"两张不可叠加的券同时被锁"。
		{"$group": bson.M{"_id": "$passengerId", "templates": bson.M{"$addToSet": bson.M{
			"$ifNull": bson.A{"$templateNo", bson.M{"$toString": "$templateId"}},
		}}}},
	})
	if err != nil {
		return nil, fmt.Errorf("list held templates: %w", err)
	}
	defer cursor.Close(ctx)
	for cursor.Next(ctx) {
		var row struct {
			ID        bson.ObjectID `bson:"_id"`
			Templates []any         `bson:"templates"`
		}
		if err := cursor.Decode(&row); err != nil {
			return nil, fmt.Errorf("decode held templates: %w", err)
		}
		nos := make([]string, 0, len(row.Templates))
		for _, raw := range row.Templates {
			if no, ok := raw.(string); ok && strings.TrimSpace(no) != "" {
				nos = append(nos, no)
			}
		}
		out[row.ID.Hex()] = nos
	}
	return out, cursor.Err()
}

// CountCouponsByBatch 按批次统计发放/核销/过期数，供汇总对账。
func (s *Store) CountCouponsByBatch(ctx context.Context, batchNos []string) (map[string]usecase.CouponStat, error) {
	out := make(map[string]usecase.CouponStat, len(batchNos))
	if len(batchNos) == 0 {
		return out, nil
	}
	cursor, err := s.db.Collection(colCoupons).Aggregate(ctx, []bson.M{
		{"$match": bson.M{"batchNo": bson.M{"$in": batchNos}}},
		{"$group": bson.M{
			"_id":     "$batchNo",
			"issued":  bson.M{"$sum": 1},
			"used":    bson.M{"$sum": bson.M{"$cond": bson.A{bson.M{"$eq": bson.A{"$state", domain.CouponStateUsed}}, 1, 0}}},
			"expired": bson.M{"$sum": bson.M{"$cond": bson.A{bson.M{"$eq": bson.A{"$state", domain.CouponStateExpired}}, 1, 0}}},
		}},
	})
	if err != nil {
		return nil, fmt.Errorf("count coupons by batch: %w", err)
	}
	defer cursor.Close(ctx)
	for cursor.Next(ctx) {
		var row struct {
			ID      string `bson:"_id"`
			Issued  int64  `bson:"issued"`
			Used    int64  `bson:"used"`
			Expired int64  `bson:"expired"`
		}
		if err := cursor.Decode(&row); err != nil {
			return nil, fmt.Errorf("decode batch stat: %w", err)
		}
		out[row.ID] = usecase.CouponStat{Issued: row.Issued, Used: row.Used, Expired: row.Expired}
	}
	return out, cursor.Err()
}

// FindPassengers 批量取乘客档案，不存在的直接不出现在结果里。
func (s *Store) FindPassengers(ctx context.Context, ids []string) (map[string]domain.Account, error) {
	out := make(map[string]domain.Account, len(ids))
	oids := objectIDs(ids)
	if len(oids) == 0 {
		return out, nil
	}
	cursor, err := s.db.Collection(colAccounts).Find(ctx, bson.M{"_id": bson.M{"$in": oids}})
	if err != nil {
		return nil, fmt.Errorf("find passengers: %w", err)
	}
	defer cursor.Close(ctx)
	for cursor.Next(ctx) {
		var doc accountDoc
		if err := cursor.Decode(&doc); err != nil {
			return nil, fmt.Errorf("decode account: %w", err)
		}
		out[doc.ID.Hex()] = doc.toDomain()
	}
	return out, cursor.Err()
}

// ReadMarketingConfig 读取营销配置，读不到就用 SSOT 默认值。
func (s *Store) ReadMarketingConfig(ctx context.Context, key string, fallback int) int {
	if key == "" {
		return fallback
	}
	var row struct {
		DictValue string `bson:"dictValue"`
	}
	err := s.db.Collection(colConfigDictionaries).FindOne(ctx,
		bson.M{"dictKey": key, "state": "已发布"},
		options.FindOne().SetSort(bson.D{{Key: "version", Value: -1}}),
	).Decode(&row)
	if err != nil {
		return fallback
	}
	value, err := strconv.Atoi(row.DictValue)
	if err != nil || value <= 0 {
		return fallback
	}
	return value
}

// isDuplicateKey 判断是否是唯一键冲突（11000）。
func isDuplicateKey(err error) bool {
	if err == nil {
		return false
	}
	var writeException mongo.WriteException
	if errors.As(err, &writeException) {
		for _, we := range writeException.WriteErrors {
			if we.Code == 11000 {
				return true
			}
		}
	}
	var bulkException mongo.BulkWriteException
	if errors.As(err, &bulkException) {
		for _, we := range bulkException.WriteErrors {
			if we.Code == 11000 {
				return true
			}
		}
	}
	return mongo.IsDuplicateKeyError(err)
}
