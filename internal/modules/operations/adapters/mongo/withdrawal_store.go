package mongo

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/NSObjects/echo-admin/internal/modules/operations/usecase"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

// withdrawRequestsCol 与主项目司机域共享的提现单集合（只读）。
const withdrawRequestsCol = "withdraw_requests"

// WithdrawalStore reads withdrawal requests owned by the driver domain.
type WithdrawalStore struct {
	db *mongo.Database
}

// NewWithdrawalStore creates the withdrawal read store.
func NewWithdrawalStore(db *mongo.Database) *WithdrawalStore {
	return &WithdrawalStore{db: db}
}

// ListWithdrawals returns one page of requests newest first.
func (s *WithdrawalStore) ListWithdrawals(ctx context.Context, filter usecase.WithdrawalFilter) ([]usecase.Withdrawal, int64, error) {
	query := bson.M{}
	if state := strings.TrimSpace(filter.State); state != "" {
		query["status"] = state
	}
	if kw := strings.TrimSpace(filter.Keyword); kw != "" {
		query["$or"] = bson.A{
			bson.M{"_id": bson.M{"$regex": kw, "$options": "i"}},
			bson.M{"driver_id": bson.M{"$regex": kw, "$options": "i"}},
		}
	}
	col := s.db.Collection(withdrawRequestsCol)
	total, err := col.CountDocuments(ctx, query)
	if err != nil {
		return nil, 0, fmt.Errorf("count withdrawals: %w", err)
	}
	opts := options.Find().
		SetSort(bson.D{{Key: "created_at", Value: -1}}).
		SetSkip(int64(filter.Skip)).
		SetLimit(int64(filter.Limit))
	cur, err := col.Find(ctx, query, opts)
	if err != nil {
		return nil, 0, fmt.Errorf("list withdrawals: %w", err)
	}
	defer cur.Close(ctx)
	rows := make([]usecase.Withdrawal, 0, filter.Limit)
	for cur.Next(ctx) {
		var doc struct {
			ID           string `bson:"_id"`
			DriverID     string `bson:"driver_id"`
			AmountFen    int64  `bson:"amount_fen"`
			Status       string `bson:"status"`
			DualApproval bool   `bson:"dual_approval"`
			Approvals    []struct {
				ApproverID string    `bson:"approver_id"`
				Comment    string    `bson:"comment"`
				ApprovedAt time.Time `bson:"approved_at"`
			} `bson:"approvals"`
			Reason    string    `bson:"reason"`
			CreatedAt time.Time `bson:"created_at"`
			UpdatedAt time.Time `bson:"updated_at"`
		}
		if err := cur.Decode(&doc); err != nil {
			return nil, 0, fmt.Errorf("decode withdrawal: %w", err)
		}
		rows = append(rows, usecase.Withdrawal{
			ID:           doc.ID,
			DriverID:     doc.DriverID,
			AmountFen:    doc.AmountFen,
			Status:       doc.Status,
			DualApproval: doc.DualApproval,
			Approvals:    len(doc.Approvals),
			Reason:       doc.Reason,
			CreatedAt:    doc.CreatedAt,
			UpdatedAt:    doc.UpdatedAt,
		})
	}
	if err := cur.Err(); err != nil {
		return nil, 0, err
	}
	return rows, total, nil
}

// DriverBriefs joins driver_no/name from the driver domain profiles.
// driver_profiles._id 是业务字符串 id，不是 ObjectID。
func (s *WithdrawalStore) DriverBriefs(ctx context.Context, driverIDs []string) (map[string]usecase.WithdrawalDriverBrief, error) {
	out := make(map[string]usecase.WithdrawalDriverBrief, len(driverIDs))
	ids := make(bson.A, 0, len(driverIDs))
	seen := make(map[string]struct{}, len(driverIDs))
	for _, id := range driverIDs {
		if id == "" {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		ids = append(ids, id)
	}
	if len(ids) == 0 {
		return out, nil
	}
	cur, err := s.db.Collection("driver_profiles").Find(ctx,
		bson.M{"_id": bson.M{"$in": ids}},
		options.Find().SetProjection(bson.M{"driver_no": 1, "name": 1}))
	if err != nil {
		return nil, fmt.Errorf("list withdrawal driver briefs: %w", err)
	}
	defer cur.Close(ctx)
	for cur.Next(ctx) {
		var doc struct {
			ID       string `bson:"_id"`
			DriverNo string `bson:"driver_no"`
			Name     string `bson:"name"`
		}
		if err := cur.Decode(&doc); err != nil {
			return nil, fmt.Errorf("decode withdrawal driver brief: %w", err)
		}
		out[doc.ID] = usecase.WithdrawalDriverBrief{DriverNo: doc.DriverNo, Name: doc.Name}
	}
	return out, cur.Err()
}
