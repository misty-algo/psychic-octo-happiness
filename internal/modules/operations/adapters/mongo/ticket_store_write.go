package mongo

import (
	"context"
	"errors"
	"fmt"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"

	"github.com/NSObjects/echo-admin/internal/modules/operations/domain"
	"github.com/NSObjects/echo-admin/internal/modules/operations/usecase"
)

// ExecuteTicketCommand applies one command. The expected state is part of the
// filter, so the update is a compare-and-set: a concurrent transition makes
// this command lose the race instead of overwriting the winner.
func (s *Store) ExecuteTicketCommand(ctx context.Context, command usecase.TicketCommand) (domain.Ticket, error) {
	collection := s.db.Collection(colTickets)
	filter := bson.M{"ticketNo": command.TicketNo, "state": command.ExpectedState}
	var current ticketDoc
	if err := collection.FindOne(ctx, bson.M{"ticketNo": command.TicketNo}).Decode(&current); err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return domain.Ticket{}, domain.ErrTicketNotFound
		}
		return domain.Ticket{}, fmt.Errorf("load ticket: %w", err)
	}
	set := bson.M{"state": command.TargetState}
	push := bson.M{}
	switch command.Command {
	case domain.TicketCommandClaim:
		set["assignee"] = command.Assignee
		set["round"] = current.Round + 1
		push["rounds"] = roundDoc{
			Round:     current.Round + 1,
			Handler:   command.Assignee,
			State:     command.TargetState,
			StartedAt: command.Now,
		}
	case domain.TicketCommandReply:
		// 首次回复才打点：之后的回复不再动计时。
		if current.FirstRespondedAt == nil {
			set["firstRespondedAt"] = command.Now
		}
	case domain.TicketCommandWaitUser:
		// 暂停：记下暂停起点，到期时间戳原封不动。
		set["pausedAt"] = command.Now
	case domain.TicketCommandResume:
		// 续算：把这一段暂停并入累计，清空暂停起点。
		elapsed := int64(0)
		if command.ResumeFrom != nil {
			elapsed = command.Now.Sub(*command.ResumeFrom).Milliseconds()
		}
		if elapsed < 0 {
			elapsed = 0
		}
		set["pausedMs"] = current.PausedMs + elapsed
		set["pausedAt"] = nil
	case domain.TicketCommandAppeal:
		// 申诉开启新一轮，处理人换人。
		set["assignee"] = command.Assignee
		set["round"] = current.Round + 1
		set["firstRespondedAt"] = nil
		push["rounds"] = roundDoc{
			Round:     current.Round + 1,
			Handler:   command.Assignee,
			State:     command.TargetState,
			StartedAt: command.Now,
			Result:    "申诉重开",
		}
	case domain.TicketCommandTransfer:
		// 转派只换人：firstResponseDueAt 刻意不动，时限不刷新。
		set["assignee"] = command.Assignee
	case domain.TicketCommandEscalate:
		set["urgent"] = true
		set["escalated"] = true
		set["escalatedAt"] = command.Now
		set["escalateReason"] = command.Message
		if !command.UrgentDueAt.IsZero() {
			set["urgentResponseDueAt"] = command.UrgentDueAt
		}
	case domain.TicketCommandResolve:
		set["escalateReason"] = ""
	case domain.TicketCommandRevoke:
		set["falseReport"] = falseReportDoc{
			Revoked: true,
			By:      command.Actor,
			At:      command.Now,
			Reason:  command.Message,
		}
	}
	update := bson.M{"$set": set}
	if len(push) > 0 {
		update["$push"] = push
	}
	var doc ticketDoc
	err := collection.FindOneAndUpdate(ctx, filter, update,
		options.FindOneAndUpdate().SetReturnDocument(options.After)).Decode(&doc)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return domain.Ticket{}, domain.ErrTicketStateConflict
	}
	if err != nil {
		return domain.Ticket{}, fmt.Errorf("execute ticket command: %w", err)
	}
	// 留痕单独追加：谁、什么时间、从什么状态到什么状态、说了什么。
	entry := timelineDoc{
		Actor:   command.Actor,
		Action:  command.Command,
		From:    command.ExpectedState,
		To:      command.TargetState,
		Message: command.Message,
		At:      command.Now,
	}
	if _, err := collection.UpdateOne(ctx, bson.M{"ticketNo": command.TicketNo}, bson.M{"$push": bson.M{"timeline": entry}}); err != nil {
		return domain.Ticket{}, fmt.Errorf("append ticket timeline: %w", err)
	}
	if command.Command == domain.TicketCommandResolve || command.Command == domain.TicketCommandClose || command.Command == domain.TicketCommandRevoke {
		// 终结命令同时收口当前轮次：结果写回轮次，便于事后还原谁处理到了哪一步。
		rounds := doc.Rounds
		for i := range rounds {
			if rounds[i].Round != doc.Round {
				continue
			}
			rounds[i].State = doc.State
			rounds[i].EndedAt = &command.Now
			if command.Command == domain.TicketCommandResolve {
				rounds[i].Result = command.Message
			}
		}
		if _, err := collection.UpdateOne(ctx,
			bson.M{"ticketNo": command.TicketNo},
			bson.M{"$set": bson.M{"rounds": rounds}}); err != nil {
			return domain.Ticket{}, fmt.Errorf("close ticket round: %w", err)
		}
		doc.Rounds = rounds
	}
	return doc.toDomain(), nil
}

// MergeTicket folds a duplicate into the main ticket: the duplicate closes and
// points at the main one, the main one remembers what it absorbed.
func (s *Store) MergeTicket(ctx context.Context, main, duplicate usecase.TicketCommandMergeTarget) error {
	collection := s.db.Collection(colTickets)
	if _, err := collection.UpdateOne(ctx,
		bson.M{"ticketNo": duplicate.TicketNo, "state": bson.M{"$ne": domain.TicketStateClosed}, "mergedInto": bson.M{"$in": bson.A{"", nil}}},
		bson.M{
			"$set": bson.M{
				"state":      domain.TicketStateClosed,
				"mergedInto": main.TicketNo,
			},
			"$push": bson.M{"timeline": timelineDoc{
				Actor:   duplicate.Actor,
				Action:  domain.TicketCommandMerge,
				Message: fmt.Sprintf("重复投诉合并到 %s：%s", main.TicketNo, duplicate.Message),
				At:      duplicate.At,
			}},
		}); err != nil {
		return fmt.Errorf("close duplicate ticket: %w", err)
	}
	if _, err := collection.UpdateOne(ctx,
		bson.M{"ticketNo": main.TicketNo},
		bson.M{
			"$addToSet": bson.M{"mergedTicketNos": duplicate.TicketNo},
			"$push": bson.M{"timeline": timelineDoc{
				Actor:   main.Actor,
				Action:  domain.TicketCommandMerge,
				Message: fmt.Sprintf("合并重复投诉 %s：%s", duplicate.TicketNo, main.Message),
				At:      main.At,
			}},
		}); err != nil {
		return fmt.Errorf("record merged ticket: %w", err)
	}
	return nil
}
