package mongo

import (
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"

	"github.com/NSObjects/echo-admin/internal/modules/operations/domain"
)

// ticketDoc maps one ticket row. tickets 集合索引沿用乘客端已建的：
// uk_ticket_no(ticketNo) / idx_order(orderId) / idx_state(state)。 The timer stores absolute deadlines plus the
// accumulated pause, never a countdown: a countdown cannot survive a restart
// and cannot tell how much of the pause was already consumed.
type ticketDoc struct {
	ID            bson.ObjectID `bson:"_id"`
	TicketNo      string        `bson:"ticketNo"`
	OrderID       bson.ObjectID `bson:"orderId"`
	OrderNo       string        `bson:"orderNo"`
	Type          string        `bson:"type"`
	Group         string        `bson:"group"`
	CityCode      string        `bson:"cityCode"`
	ComplainantID bson.ObjectID `bson:"complainantId"`
	DriverID      bson.ObjectID `bson:"driverId"`
	Content       string        `bson:"content"`
	Evidences     []string      `bson:"evidences"`
	State         string        `bson:"state"`
	Urgent        bool          `bson:"urgent"`
	Assignee      string        `bson:"assignee"`
	Round         int           `bson:"round"`
	Rounds        []roundDoc    `bson:"rounds"`
	CreatedAt     time.Time     `bson:"createdAt"`

	FirstResponseDueAt  time.Time  `bson:"firstResponseDueAt"`
	FirstRespondedAt    *time.Time `bson:"firstRespondedAt"`
	UrgentResponseDueAt *time.Time `bson:"urgentResponseDueAt"`
	PausedMs            int64      `bson:"pausedMs"`
	PausedAt            *time.Time `bson:"pausedAt"`

	Escalated      bool       `bson:"escalated"`
	EscalatedAt    *time.Time `bson:"escalatedAt"`
	EscalateReason string     `bson:"escalateReason"`
	DutyRemindedAt *time.Time `bson:"dutyRemindedAt"`

	MergedTicketNos []string        `bson:"mergedTicketNos"`
	MergedInto      string          `bson:"mergedInto"`
	FalseReport     *falseReportDoc `bson:"falseReport"`

	Timeline []timelineDoc `bson:"timeline"`
}

// roundDoc is one handling round; an appeal appends a new one.
type roundDoc struct {
	Round     int        `bson:"round"`
	Handler   string     `bson:"handler"`
	State     string     `bson:"state"`
	StartedAt time.Time  `bson:"startedAt"`
	EndedAt   *time.Time `bson:"endedAt"`
	Result    string     `bson:"result"`
}

// timelineDoc records 谁、什么时间、从什么状态到什么状态、说了什么。
type timelineDoc struct {
	Seq     int       `bson:"seq"`
	Actor   string    `bson:"actor"`
	Action  string    `bson:"action"`
	From    string    `bson:"from"`
	To      string    `bson:"to"`
	Message string    `bson:"message"`
	At      time.Time `bson:"at"`
}

// falseReportDoc records a verified false dangerous-driving report.
type falseReportDoc struct {
	Revoked  bool      `bson:"revoked"`
	By       string    `bson:"by"`
	At       time.Time `bson:"at"`
	Reason   string    `bson:"reason"`
	Restored bool      `bson:"driverStatusRestored"`
}

func (d ticketDoc) toDomain() domain.Ticket {
	ticket := domain.Ticket{
		ID:                  d.ID.Hex(),
		TicketNo:            d.TicketNo,
		OrderID:             d.OrderID.Hex(),
		OrderNo:             d.OrderNo,
		Type:                d.Type,
		Group:               d.Group,
		CityCode:            d.CityCode,
		ComplainantID:       d.ComplainantID.Hex(),
		DriverID:            d.DriverID.Hex(),
		Content:             d.Content,
		EvidenceCount:       len(d.Evidences),
		State:               d.State,
		Urgent:              d.Urgent,
		Assignee:            d.Assignee,
		Round:               d.Round,
		CreatedAt:           d.CreatedAt,
		FirstResponseDueAt:  d.FirstResponseDueAt,
		FirstRespondedAt:    d.FirstRespondedAt,
		UrgentResponseDueAt: d.UrgentResponseDueAt,
		PausedMs:            d.PausedMs,
		PausedAt:            d.PausedAt,
		Escalated:           d.Escalated,
		EscalatedAt:         d.EscalatedAt,
		EscalateReason:      d.EscalateReason,
		DutyRemindedAt:      d.DutyRemindedAt,
		MergedTicketNos:     d.MergedTicketNos,
		MergedInto:          d.MergedInto,
	}
	if d.FalseReport != nil {
		ticket.FalseReport = &domain.FalseReport{
			Revoked:  d.FalseReport.Revoked,
			By:       d.FalseReport.By,
			At:       d.FalseReport.At,
			Reason:   d.FalseReport.Reason,
			Restored: d.FalseReport.Restored,
		}
	}
	for _, round := range d.Rounds {
		item := domain.Round{
			Round:     round.Round,
			Handler:   round.Handler,
			State:     round.State,
			StartedAt: round.StartedAt,
			Result:    round.Result,
		}
		if round.EndedAt != nil {
			ended := *round.EndedAt
			item.EndedAt = &ended
		}
		ticket.Rounds = append(ticket.Rounds, item)
	}
	for _, entry := range d.Timeline {
		ticket.Timeline = append(ticket.Timeline, domain.TimelineEntry{
			Seq:     entry.Seq,
			Actor:   entry.Actor,
			Action:  entry.Action,
			From:    entry.From,
			To:      entry.To,
			Message: entry.Message,
			At:      entry.At,
		})
	}
	return ticket
}
