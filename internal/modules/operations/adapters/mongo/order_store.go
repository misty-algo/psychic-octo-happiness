package mongo

import (
	"context"
	"errors"
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

// 订单侧集合由乘客端/司机端拥有，字段名以库内既有索引为准：
// passenger_orders: order_no（订单唯一键）/ account_id / state / created_at。
// 文档由乘客域写入（snake_case），本模块只读。
// dispatch_tasks: uk_task_no(taskNo) / idx_driver(driverId)
// tickets: uk_ticket_no(ticketNo) / idx_order(orderId) / idx_state(state)
// payment_attempts: uk_channel_trade_no / uk_request_no / idx_bill(billId)
const (
	colOrders          = "passenger_orders"
	colDispatchTasks   = "dispatch_tasks"
	colTickets         = "tickets"
	colPaymentAttempts = "payment_attempts"
)

// orderDoc maps one passenger_orders row（乘客域写入的 snake_case 文档）。
type orderDoc struct {
	ID string `bson:"_id"`
	// OrderNo / AccountID / State 来自乘客域订单；订单 ID 就是文档主键（字符串）。
	OrderNo   string `bson:"order_no"`
	AccountID string `bson:"account_id"`
	State     string `bson:"state"`
	// PayStatus 乘客域未落支付单状态：待支付对应 COMPLETED_UNPAID，已完成对应 COMPLETED。
	PayStatus string `bson:"-"`

	// TotalFee / CancelFee / CouponFee 单位分。
	TotalFee  int64 `bson:"total_fee"`
	CancelFee int64 `bson:"cancel_fee"`
	CouponFee int64 `bson:"coupon_fee"`

	// PricingSnapshot 冻结下单时的计价规则版本。
	PricingVersion string `bson:"pricing_snapshot"`

	StartAddress string    `bson:"start_addr"`
	EndAddress   string    `bson:"end_addr"`
	StartName    string    `bson:"start_name"`
	EndName      string    `bson:"end_name"`
	DriverName   string    `bson:"driver_name"`
	DriverPlate  string    `bson:"driver_plate"`
	CreatedAt    time.Time `bson:"created_at"`
}

func (d orderDoc) toDomain() domain.Order {
	return domain.Order{
		ID:             d.ID,
		OrderNo:        d.OrderNo,
		PassengerID:    d.AccountID,
		// 乘客域订单只落司机姓名/车牌，司机 ID 在派单任务（hall_orders）里；
		// 列表页不回查，留空由详情接口按订单号补齐。
		DriverID:       "",
		DriverName:     d.DriverName,
		Plate:          d.DriverPlate,
		State:          d.State,
		PayStatus:      payStatusOf(d.State),
		Amount:         d.TotalFee,
		CancelFee:      d.CancelFee,
		PricingVersion: d.PricingVersion,
		StartAddress:   d.StartAddress,
		EndAddress:     d.EndAddress,
		CreatedAt:      d.CreatedAt,
	}
}

// payStatusOf 把乘客域订单状态映射为支付状态视图：
// COMPLETED_UNPAID（待支付）/ COMPLETED（已支付）/ CANCELLED（已取消）。
func payStatusOf(state string) string {
	switch state {
	case "COMPLETED_UNPAID":
		return "UNPAID"
	case "COMPLETED":
		return "PAID"
	case "CANCELLED":
		return "CANCELLED"
	default:
		return ""
	}
}

// pricingDoc builds the fare section of the detail view.
func (d orderDoc) pricing() domain.Pricing {
	return domain.Pricing{
		Version:     d.PricingVersion,
		EstimateFee: d.TotalFee,
		TotalFee:    d.TotalFee,
	}
}

// ListOrders returns one page of orders, newest first.
func (s *Store) ListOrders(ctx context.Context, filter usecase.OrderFilter) ([]domain.Order, int64, error) {
	query, err := s.orderQuery(ctx, filter)
	if err != nil {
		return nil, 0, err
	}
	collection := s.db.Collection(colOrders)
	total, err := collection.CountDocuments(ctx, query)
	if err != nil {
		return nil, 0, fmt.Errorf("count orders: %w", err)
	}
	cur, err := collection.Find(ctx, query, options.Find().
		SetSort(bson.D{bson.E{Key: "created_at", Value: -1}}).
		SetSkip(int64(filter.Skip)).
		SetLimit(int64(filter.Limit)))
	if err != nil {
		return nil, 0, fmt.Errorf("list orders: %w", err)
	}
	defer cur.Close(ctx)
	orders := make([]domain.Order, 0)
	for cur.Next(ctx) {
		var doc orderDoc
		if err := cur.Decode(&doc); err != nil {
			return nil, 0, fmt.Errorf("decode order: %w", err)
		}
		orders = append(orders, doc.toDomain())
	}
	return orders, total, cur.Err()
}

// FindOrder returns one order by order number.
func (s *Store) FindOrder(ctx context.Context, orderNo string) (domain.Order, error) {
	var doc orderDoc
	err := s.db.Collection(colOrders).
		FindOne(ctx, bson.M{"order_no": strings.TrimSpace(orderNo)}).
		Decode(&doc)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return domain.Order{}, domain.ErrOrderNotFound
	}
	if err != nil {
		return domain.Order{}, fmt.Errorf("find order: %w", err)
	}
	return doc.toDomain(), nil
}

// orderQuery builds the workbench filter. A keyword that is not an order
// number is resolved against passenger identity first, so operators can search
// by phone fragment the same way they do in user management.
func (s *Store) orderQuery(ctx context.Context, filter usecase.OrderFilter) (bson.M, error) {
	query := bson.M{}
	// 订单文档由乘客域写入，字段为 snake_case；时间维度必须用 created_at，
	// 否则「今日订单」这类按创建时间过滤的查询永远为空。
	if filter.State != "" {
		// 支持逗号分隔的多状态查询（例如「行程中」= ACCEPTED,ARRIVED,IN_SERVICE）。
		states := splitCSV(filter.State)
		if len(states) > 1 {
			values := make(bson.A, 0, len(states))
			for _, state := range states {
				values = append(values, state)
			}
			query["state"] = bson.M{"$in": values}
		} else {
			query["state"] = filter.State
		}
	}
	// 业务类型/城市/风险等级是排查订单最常用的三个维度，库里都有字段，直接筛。
	if filter.BizType != "" {
		query["bizType"] = filter.BizType
	}
	if filter.CityCode != "" {
		query["cityCode"] = filter.CityCode
	}
	if filter.RiskLevel != "" {
		query["riskLevel"] = filter.RiskLevel
	}
	if !filter.From.IsZero() || !filter.To.IsZero() {
		range_ := bson.M{}
		if !filter.From.IsZero() {
			range_["$gte"] = filter.From
		}
		if !filter.To.IsZero() {
			range_["$lte"] = filter.To
		}
		query["created_at"] = range_
	}
	keyword := strings.TrimSpace(filter.Keyword)
	if keyword == "" {
		return query, nil
	}
	conditions := bson.A{bson.M{"order_no": bson.M{"$regex": "^" + regexp.QuoteMeta(keyword)}}}
	ids, err := s.resolvePassengerIDs(ctx, keyword)
	if err != nil {
		return nil, err
	}
	if len(ids) > 0 {
		values := make(bson.A, 0, len(ids))
		for _, id := range ids {
			values = append(values, id)
		}
		conditions = append(conditions, bson.M{"account_id": bson.M{"$in": values}})
	}
	query["$or"] = conditions
	return query, nil
}

// splitCSV splits a comma-separated filter value, trimming blanks.
func splitCSV(value string) []string {
	parts := strings.Split(value, ",")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		item := strings.TrimSpace(part)
		if item != "" {
			out = append(out, item)
		}
	}
	return out
}

// objectIDs converts hex ids to ObjectIDs, skipping ids the business side
// never stored as ObjectId.
func objectIDs(ids []string) []bson.ObjectID {
	out := make([]bson.ObjectID, 0, len(ids))
	for _, id := range ids {
		if oid, err := bson.ObjectIDFromHex(id); err == nil {
			out = append(out, oid)
		}
	}
	return out
}

// objectID converts one hex id to an ObjectID.
func objectID(id string) (bson.ObjectID, error) {
	return bson.ObjectIDFromHex(id)
}

// keys returns the keys of a set as a slice.
func keys(set map[string]struct{}) []string {
	out := make([]string, 0, len(set))
	for key := range set {
		out = append(out, key)
	}
	return out
}

// resolvePassengerIDs turns a phone-mask, nickname or real-name fragment into
// passenger account ids. 乘客域账号（passenger_accounts）手机号是加密存储，
// 只能按脱敏手机号 / 昵称 / 实名姓名前缀匹配。
func (s *Store) resolvePassengerIDs(ctx context.Context, keyword string) ([]string, error) {
	cur, err := s.db.Collection("passenger_accounts").Find(ctx, bson.M{
		"$or": bson.A{
			bson.M{"phone_mask": bson.M{"$regex": regexp.QuoteMeta(keyword)}},
			bson.M{"nickname": bson.M{"$regex": regexp.QuoteMeta(keyword)}},
			bson.M{"real_name_name": bson.M{"$regex": regexp.QuoteMeta(keyword)}},
		},
	}, options.Find().SetProjection(bson.M{"_id": 1}).SetLimit(50))
	if err != nil {
		return nil, fmt.Errorf("resolve passenger ids: %w", err)
	}
	defer cur.Close(ctx)
	ids := make([]string, 0)
	for cur.Next(ctx) {
		var row struct {
			ID string `bson:"_id"`
		}
		if err := cur.Decode(&row); err != nil {
			return nil, fmt.Errorf("decode passenger id: %w", err)
		}
		ids = append(ids, row.ID)
	}
	return ids, cur.Err()
}
