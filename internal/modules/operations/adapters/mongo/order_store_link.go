package mongo

import (
	"context"
	"fmt"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo/options"

	"github.com/NSObjects/echo-admin/internal/modules/operations/domain"
)

// ListTickets returns complaint tickets linked to the given orders.
func (s *Store) ListOrderTickets(ctx context.Context, orderIDs []string) (map[string][]domain.TicketBrief, error) {
	out := make(map[string][]domain.TicketBrief, len(orderIDs))
	oids := objectIDs(orderIDs)
	if len(oids) == 0 {
		return out, nil
	}
	cur, err := s.db.Collection(colTickets).Find(ctx,
		bson.M{"orderId": bson.M{"$in": oids}},
		options.Find().SetSort(bson.D{bson.E{Key: "createdAt", Value: -1}}))
	if err != nil {
		return nil, fmt.Errorf("list order tickets: %w", err)
	}
	defer cur.Close(ctx)
	for cur.Next(ctx) {
		var doc struct {
			TicketNo  string        `bson:"ticketNo"`
			OrderID   bson.ObjectID `bson:"orderId"`
			Type      string        `bson:"type"`
			State     string        `bson:"state"`
			CreatedAt time.Time     `bson:"createdAt"`
		}
		if err := cur.Decode(&doc); err != nil {
			return nil, fmt.Errorf("decode order ticket: %w", err)
		}
		out[doc.OrderID.Hex()] = append(out[doc.OrderID.Hex()], domain.TicketBrief{
			TicketNo:  doc.TicketNo,
			Type:      doc.Type,
			State:     doc.State,
			CreatedAt: doc.CreatedAt,
		})
	}
	return out, cur.Err()
}

// FindPayments 返回每个订单的支付视图。主项目支付单在支付域自己的存储里
// （payment 服务用 MySQL），乘客订单文档只带状态：据此派生支付视图，
// 待支付（COMPLETED_UNPAID）显示 UNPAID，已完成显示 PAID。
func (s *Store) FindPayments(ctx context.Context, orders []domain.Order) (map[string]domain.Payment, error) {
	out := make(map[string]domain.Payment, len(orders))
	for _, order := range orders {
		out[order.ID] = domain.Payment{
			State:  order.PayStatus,
			Amount: order.Amount,
		}
	}
	return out, nil
}
