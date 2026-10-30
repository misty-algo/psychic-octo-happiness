package mongo

import (
	"context"
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

// EscalateOverdue marks unanswered tickets past their deadline urgent. The
// paused total is folded into the deadline with $expr so a ticket waiting for
// the passenger is judged on the time it actually had.
func (s *Store) EscalateOverdue(ctx context.Context, now time.Time, limit int) ([]string, error) {
	if limit <= 0 {
		limit = usecase.SweepBatchSize
	}
	query := bson.M{
		"state":            bson.M{"$in": bson.A{domain.TicketStatePending, domain.TicketStateProcessing, domain.TicketStateWaitingUser}},
		"firstRespondedAt": nil,
		"escalated":        false,
		// 没有到期时间的工单（老数据或写入方漏字段）不能参与超时判定：
		// null 在 BSON 比较序里小于任何数字，$lte 会把它判成"已超时"。
		"firstResponseDueAt": bson.M{"$ne": nil},
		// 到期时间 + 累计暂停毫秒 与当前时间比较。日期要先转成毫秒数：
		// BSON 里 Date 与 Number 按类型序比较，直接比会永远判为未到期。
		"$expr": bson.M{"$lte": bson.A{
			bson.M{"$add": bson.A{bson.M{"$toLong": "$firstResponseDueAt"}, "$pausedMs"}},
			now.UnixMilli(),
		}},
	}
	cur, err := s.db.Collection(colTickets).Find(ctx, query,
		options.Find().SetProjection(bson.M{"ticketNo": 1}).SetLimit(int64(limit)))
	if err != nil {
		return nil, fmt.Errorf("find overdue tickets: %w", err)
	}
	defer cur.Close(ctx)
	numbers := make([]string, 0)
	for cur.Next(ctx) {
		var row struct {
			TicketNo string `bson:"ticketNo"`
		}
		if err := cur.Decode(&row); err != nil {
			return nil, fmt.Errorf("decode overdue ticket: %w", err)
		}
		numbers = append(numbers, row.TicketNo)
	}
	if err := cur.Err(); err != nil {
		return nil, fmt.Errorf("find overdue tickets: %w", err)
	}
	escalated := make([]string, 0, len(numbers))
	for _, ticketNo := range numbers {
		// 幂等：过滤器带上 escalated=false，抢先的那一遍赢，重复扫描不会二次升级。
		filter := bson.M{"ticketNo": ticketNo, "escalated": false}
		hours := s.ReadTicketConfig(ctx, usecase.ConfigTicketUrgentResponseHours, usecase.DefaultUrgentResponseHours)
		update := bson.M{
			"$set": bson.M{
				"urgent":              true,
				"escalated":           true,
				"escalatedAt":         now,
				"escalateReason":      "超过首次回复时限自动升级",
				"urgentResponseDueAt": now.Add(time.Duration(hours) * time.Hour),
			},
			"$push": bson.M{"timeline": timelineDoc{
				Actor:   "system:sweeper",
				Action:  domain.TicketCommandEscalate,
				Message: "超过首次回复时限，自动升级为紧急",
				At:      now,
			}},
		}
		result, err := s.db.Collection(colTickets).UpdateOne(ctx, filter, update)
		if err != nil {
			return escalated, fmt.Errorf("escalate overdue ticket: %w", err)
		}
		if result.MatchedCount > 0 {
			escalated = append(escalated, ticketNo)
		}
	}
	return escalated, nil
}

// RemindDuty pushes unclaimed dangerous-driving tickets to the duty queue one
// minute after creation. dutyRemindedAt guards against pushing twice.
func (s *Store) RemindDuty(ctx context.Context, deadline time.Time, limit int) ([]string, error) {
	return s.markDuty(ctx, deadline, limit, "dutyRemindedAt", false)
}

// EscalateDuty escalates dangerous-driving tickets nobody claimed within five
// minutes of creation.
func (s *Store) EscalateDuty(ctx context.Context, deadline time.Time, limit int) ([]string, error) {
	return s.markDuty(ctx, deadline, limit, "", true)
}

// markDuty scans unclaimed dangerous-driving tickets created before deadline
// and applies the duty action once per ticket.
func (s *Store) markDuty(ctx context.Context, deadline time.Time, limit int, flag string, escalate bool) ([]string, error) {
	if limit <= 0 {
		limit = usecase.SweepBatchSize
	}
	query := bson.M{
		"type":       domain.TicketTypeDangerous,
		"state":      domain.TicketStatePending,
		"assignee":   bson.M{"$in": bson.A{"", nil}},
		"createdAt":  bson.M{"$lte": deadline},
		"mergedInto": bson.M{"$in": bson.A{"", nil}},
	}
	if flag != "" {
		query[flag] = nil
	} else {
		query["escalated"] = false
	}
	cur, err := s.db.Collection(colTickets).Find(ctx, query,
		options.Find().SetProjection(bson.M{"ticketNo": 1}).SetLimit(int64(limit)))
	if err != nil {
		return nil, fmt.Errorf("find duty tickets: %w", err)
	}
	defer cur.Close(ctx)
	numbers := make([]string, 0)
	for cur.Next(ctx) {
		var row struct {
			TicketNo string `bson:"ticketNo"`
		}
		if err := cur.Decode(&row); err != nil {
			return nil, fmt.Errorf("decode duty ticket: %w", err)
		}
		numbers = append(numbers, row.TicketNo)
	}
	if err := cur.Err(); err != nil {
		return nil, fmt.Errorf("find duty tickets: %w", err)
	}
	done := make([]string, 0, len(numbers))
	for _, ticketNo := range numbers {
		filter := bson.M{"ticketNo": ticketNo, "state": domain.TicketStatePending, "assignee": bson.M{"$in": bson.A{"", nil}}}
		if flag != "" {
			filter[flag] = nil
		} else {
			filter["escalated"] = false
		}
		now := time.Now()
		set := bson.M{}
		message := "危险驾驶工单无人领单，已推送值班队列提醒"
		if flag != "" {
			set[flag] = now
		}
		if escalate {
			hours := s.ReadTicketConfig(ctx, usecase.ConfigTicketUrgentResponseHours, usecase.DefaultUrgentResponseHours)
			set["urgent"] = true
			set["escalated"] = true
			set["escalatedAt"] = now
			set["escalateReason"] = "危险驾驶工单超时无人领单，自动升级"
			set["urgentResponseDueAt"] = now.Add(time.Duration(hours) * time.Hour)
			message = "危险驾驶工单五分钟无人领单，自动升级"
		}
		result, err := s.db.Collection(colTickets).UpdateOne(ctx, filter, bson.M{
			"$set": set,
			"$push": bson.M{"timeline": timelineDoc{
				Actor:   "system:sweeper",
				Action:  domain.TicketCommandEscalate,
				Message: message,
				At:      now,
			}},
		})
		if err != nil {
			return done, fmt.Errorf("mark duty ticket: %w", err)
		}
		if result.MatchedCount > 0 {
			done = append(done, ticketNo)
		}
	}
	return done, nil
}

// RevokeTicketRestrictions lifts the limits this ticket imposed on a driver,
// so a false report leaves no lasting damage. Returns how many were lifted.
//
// 字段名必须是 account_id：限制集合的唯一索引就是
// uniq_active_account_type{account_id,type,state}，且存的是账号字符串而非
// ObjectId。写错字段名会让撤销静默匹配到零条，司机状态根本没恢复——撤销
// 条数一定要如实返回给上层留痕，不能假装成功。
func (s *Store) RevokeTicketRestrictions(ctx context.Context, ticketNo, driverAccountID, operator, reason string, at time.Time) (int, error) {
	driverAccountID = strings.TrimSpace(driverAccountID)
	if driverAccountID == "" || ticketNo == "" {
		return 0, nil
	}
	// 只撤销"生效中"的限制；已撤销的不重复写，避免覆盖原始撤销留痕。
	query := bson.M{
		"account_id": driverAccountID,
		"state":      domain.RestrictionActive,
	}
	result, err := s.db.Collection(colRestrictions).UpdateMany(ctx, query, bson.M{"$set": bson.M{
		"state":         domain.RestrictionRevoked,
		"revoked_at":    at,
		"revoked_by":    operator,
		"revoke_reason": fmt.Sprintf("工单 %s 核实为误报：%s", ticketNo, reason),
	}})
	if err != nil {
		return 0, fmt.Errorf("revoke ticket restrictions: %w", err)
	}
	return int(result.ModifiedCount), nil
}

// EmitTicketEvent writes one outbox row. Duplicate event ids are tolerated so
// a replayed scan cannot produce duplicate downstream work.
func (s *Store) EmitTicketEvent(ctx context.Context, event usecase.TicketEvent) error {
	payload := bson.M{}
	for key, value := range event.Payload {
		payload[key] = value
	}
	doc := eventLogDoc{
		ID:          bson.NewObjectID(),
		EventID:     event.EventID,
		Type:        event.Type,
		Aggregate:   event.Aggregate,
		AggregateID: event.AggregateID,
		Payload:     payload,
		Status:      "PENDING",
		At:          event.At,
	}
	if _, err := s.db.Collection(colEventLogs).InsertOne(ctx, doc); err != nil {
		if mongo.IsDuplicateKeyError(err) {
			return nil
		}
		return fmt.Errorf("emit ticket event: %w", err)
	}
	return nil
}

// ReadTicketConfig reads one ticket deadline from config_dictionaries. Missing
// or unparsable keys fall back to the SSOT default instead of failing the
// sweep, so a bad config row degrades to the documented behaviour.
func (s *Store) ReadTicketConfig(ctx context.Context, key string, fallback int) int {
	if key == "" {
		return fallback
	}
	var row struct {
		DictValue string `bson:"dictValue"`
	}
	err := s.db.Collection(colConfigDictionaries).FindOne(ctx,
		bson.M{"dictKey": key, "state": "已发布"},
		options.FindOne().SetSort(bson.D{bson.E{Key: "version", Value: -1}}),
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
