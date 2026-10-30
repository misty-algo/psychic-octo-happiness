package mongo

import (
	"context"
	"errors"
	"fmt"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"

	"github.com/NSObjects/echo-admin/internal/modules/operations/domain"
	"github.com/NSObjects/echo-admin/internal/modules/operations/usecase"
)

// ExecuteCommand applies one order command. The expected state is part of the
// filter, so the update is a compare-and-set: a concurrent transition makes
// this command lose the race instead of overwriting the winner.
func (s *Store) ExecuteCommand(ctx context.Context, command usecase.OrderCommand) (domain.Order, error) {
	set := bson.M{
		"lastCommand":      command.Command,
		"lastCommandAt":    command.At,
		"lastCommandBy":    command.Operator,
		"lastCommandBasis": command.Basis,
	}
	switch command.Command {
	case domain.OrderCommandReassign:
		// 退回派单池：清空司机，由派单服务重新选人。
		set["state"] = command.TargetState
		set["driverId"] = nil
		set["reassignedAt"] = command.At
	case domain.OrderCommandWaiveCancelFee:
		// 免取消费：只动取消费字段，其他金额一律不碰。
		set["cancelFee"] = int64(0)
		set["cancelFeeWaived"] = true
		set["cancelFeeWaivedBy"] = command.Operator
		set["cancelFeeWaivedAt"] = command.At
	case domain.OrderCommandSafeEnd:
		set["state"] = command.TargetState
		set["safetyEnded"] = true
		set["safetyEndedBy"] = command.Operator
		set["safetyEndedAt"] = command.At
	case domain.OrderCommandReviewPricing:
		set["pricingReviewed"] = true
		set["pricingReviewedBy"] = command.Operator
		set["pricingReviewedAt"] = command.At
	default:
		return domain.Order{}, domain.ErrInvalidCommand
	}
	update := bson.M{
		"$set": set,
		"$push": bson.M{"opsCommands": commandDoc{
			Command:   command.Command,
			Basis:     command.Basis,
			Operator:  command.Operator,
			FromState: command.ExpectedState,
			ToState:   command.TargetState,
			At:        command.At,
		}},
	}
	var doc orderDoc
	err := s.db.Collection(colOrders).FindOneAndUpdate(ctx,
		bson.M{"orderNo": command.OrderNo, "state": command.ExpectedState},
		update,
		options.FindOneAndUpdate().SetReturnDocument(options.After),
	).Decode(&doc)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return domain.Order{}, domain.ErrOrderStateConflict
	}
	if err != nil {
		return domain.Order{}, fmt.Errorf("execute order command: %w", err)
	}
	return doc.toDomain(), nil
}

// commandDoc is the operations-owned command trail pushed onto the order. It
// keeps 谁、依据什么、把哪个状态改成哪个 inside the order document so the
// passenger-side history and the console agree without a second query.
type commandDoc struct {
	Command   string    `bson:"command"`
	Basis     string    `bson:"basis"`
	Operator  string    `bson:"operator"`
	FromState string    `bson:"from_state"`
	ToState   string    `bson:"to_state"`
	At        time.Time `bson:"at"`
}
