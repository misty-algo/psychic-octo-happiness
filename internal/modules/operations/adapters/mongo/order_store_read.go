package mongo

import (
	"context"
	"fmt"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo/options"

	"github.com/NSObjects/echo-admin/internal/modules/operations/domain"
	"github.com/NSObjects/echo-admin/internal/modules/operations/usecase"
)

// PassengerBriefs returns masked passenger identity for the given account ids.
// 乘客域账号的 _id 是业务生成的字符串（id-xxx），不是 ObjectID；手机号加密
// 存储，这里只取脱敏号、昵称与实名姓名，完整号码只从审计过的用户详情出口。
func (s *Store) PassengerBriefs(ctx context.Context, ids []string) (map[string]usecase.PassengerBrief, error) {
	out := make(map[string]usecase.PassengerBrief, len(ids))
	clean := make(bson.A, 0, len(ids))
	for _, id := range ids {
		if id != "" {
			clean = append(clean, id)
		}
	}
	if len(clean) == 0 {
		return out, nil
	}
	// 乘客域账号集合与关键词搜索（resolvePassengerIDs）保持同一来源。
	cur, err := s.db.Collection("passenger_accounts").Find(ctx,
		bson.M{"_id": bson.M{"$in": clean}},
		options.Find().SetProjection(bson.M{"nickname": 1, "phone_mask": 1, "real_name_name": 1}))
	if err != nil {
		return nil, fmt.Errorf("list passenger briefs: %w", err)
	}
	defer cur.Close(ctx)
	for cur.Next(ctx) {
		var row struct {
			ID       string `bson:"_id"`
			Nickname string `bson:"nickname"`
			Phone    string `bson:"phone_mask"`
			RealName string `bson:"real_name_name"`
		}
		if err := cur.Decode(&row); err != nil {
			return nil, fmt.Errorf("decode passenger brief: %w", err)
		}
		name := row.RealName
		if name == "" {
			name = row.Nickname
		}
		out[row.ID] = usecase.PassengerBrief{
			Name:  name,
			Phone: row.Phone,
		}
	}
	return out, cur.Err()
}

// ListDispatchTasks returns dispatch records of the given orders, keyed by
// order id. dispatch_tasks is owned by the dispatch service.
func (s *Store) ListDispatchTasks(ctx context.Context, orderIDs []string) (map[string][]domain.DispatchTask, error) {
	out := make(map[string][]domain.DispatchTask, len(orderIDs))
	oids := objectIDs(orderIDs)
	if len(oids) == 0 {
		return out, nil
	}
	cur, err := s.db.Collection(colDispatchTasks).Find(ctx,
		bson.M{"orderId": bson.M{"$in": oids}},
		options.Find().SetSort(bson.D{bson.E{Key: "createdAt", Value: 1}}))
	if err != nil {
		return nil, fmt.Errorf("list dispatch tasks: %w", err)
	}
	defer cur.Close(ctx)
	drivers := make(map[string]struct{})
	for cur.Next(ctx) {
		var doc struct {
			TaskNo    string        `bson:"taskNo"`
			OrderID   bson.ObjectID `bson:"orderId"`
			DriverID  bson.ObjectID `bson:"driverId"`
			Round     int           `bson:"round"`
			State     string        `bson:"state"`
			CreatedAt time.Time     `bson:"createdAt"`
		}
		if err := cur.Decode(&doc); err != nil {
			return nil, fmt.Errorf("decode dispatch task: %w", err)
		}
		out[doc.OrderID.Hex()] = append(out[doc.OrderID.Hex()], domain.DispatchTask{
			TaskNo:    doc.TaskNo,
			DriverID:  doc.DriverID.Hex(),
			Round:     doc.Round,
			State:     doc.State,
			CreatedAt: doc.CreatedAt,
		})
		if !doc.DriverID.IsZero() {
			drivers[doc.DriverID.Hex()] = struct{}{}
		}
	}
	if err := cur.Err(); err != nil {
		return nil, fmt.Errorf("list dispatch tasks: %w", err)
	}
	names, err := s.DriverBriefs(ctx, keys(drivers))
	if err != nil {
		return nil, err
	}
	for orderID, tasks := range out {
		for i := range tasks {
			tasks[i].DriverName = names[tasks[i].DriverID].Name
		}
		out[orderID] = tasks
	}
	return out, nil
}

// ListDispatchAttempts returns every driver response recorded on the dispatch
// tasks: the 接单尝试 list the workbench shows.
func (s *Store) ListDispatchAttempts(ctx context.Context, orderIDs []string) (map[string][]domain.DispatchAttempt, error) {
	out := make(map[string][]domain.DispatchAttempt, len(orderIDs))
	oids := objectIDs(orderIDs)
	if len(oids) == 0 {
		return out, nil
	}
	cur, err := s.db.Collection(colDispatchTasks).Find(ctx,
		bson.M{"orderId": bson.M{"$in": oids}, "attempts": bson.M{"$exists": true}},
		options.Find().SetSort(bson.D{bson.E{Key: "createdAt", Value: 1}}))
	if err != nil {
		return nil, fmt.Errorf("list dispatch attempts: %w", err)
	}
	defer cur.Close(ctx)
	drivers := make(map[string]struct{})
	for cur.Next(ctx) {
		var doc struct {
			OrderID  bson.ObjectID `bson:"orderId"`
			Attempts []struct {
				DriverID    bson.ObjectID `bson:"driverId"`
				Result      string        `bson:"result"`
				Score       float64       `bson:"score"`
				Distance    float64       `bson:"distance"`
				RespondedAt time.Time     `bson:"respondedAt"`
			} `bson:"attempts"`
		}
		if err := cur.Decode(&doc); err != nil {
			return nil, fmt.Errorf("decode dispatch attempts: %w", err)
		}
		for _, attempt := range doc.Attempts {
			out[doc.OrderID.Hex()] = append(out[doc.OrderID.Hex()], domain.DispatchAttempt{
				DriverID:    attempt.DriverID.Hex(),
				Result:      attempt.Result,
				Score:       attempt.Score,
				Distance:    attempt.Distance,
				RespondedAt: attempt.RespondedAt,
			})
			if !attempt.DriverID.IsZero() {
				drivers[attempt.DriverID.Hex()] = struct{}{}
			}
		}
	}
	if err := cur.Err(); err != nil {
		return nil, fmt.Errorf("list dispatch attempts: %w", err)
	}
	names, err := s.DriverBriefs(ctx, keys(drivers))
	if err != nil {
		return nil, err
	}
	for orderID, attempts := range out {
		for i := range attempts {
			attempts[i].DriverName = names[attempts[i].DriverID].Name
		}
		out[orderID] = attempts
	}
	return out, nil
}

// ListOrderTrajectories returns raw tracking points. Point keys differ per
// client, so rows are passed through untouched instead of being reshaped into
// a guessed schema.
func (s *Store) ListOrderTrajectories(ctx context.Context, orderIDs []string) (map[string]domain.Trajectory, error) {
	out := make(map[string]domain.Trajectory, len(orderIDs))
	oids := objectIDs(orderIDs)
	if len(oids) == 0 {
		return out, nil
	}
	cur, err := s.db.Collection(colOrders).Find(ctx,
		bson.M{"_id": bson.M{"$in": oids}, "track": bson.M{"$exists": true}},
		options.Find().SetProjection(bson.M{"track": 1}))
	if err != nil {
		return nil, fmt.Errorf("list order trajectories: %w", err)
	}
	defer cur.Close(ctx)
	for cur.Next(ctx) {
		var doc struct {
			ID    bson.ObjectID `bson:"_id"`
			Track []bson.Raw    `bson:"track"`
		}
		if err := cur.Decode(&doc); err != nil {
			return nil, fmt.Errorf("decode order trajectory: %w", err)
		}
		points := make([]map[string]any, 0, len(doc.Track))
		for _, raw := range doc.Track {
			var point map[string]any
			if err := bson.Unmarshal(raw, &point); err != nil {
				return nil, fmt.Errorf("decode trajectory point: %w", err)
			}
			points = append(points, point)
		}
		out[doc.ID.Hex()] = domain.Trajectory{Source: "orders.track", Points: points}
	}
	return out, cur.Err()
}
