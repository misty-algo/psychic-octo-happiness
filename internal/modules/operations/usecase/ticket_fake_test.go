package usecase

import (
	"context"
	"errors"
	"time"

	"github.com/NSObjects/echo-admin/internal/modules/operations/domain"
)

// fakeTicketStore is an in-memory TicketStore double. It models the same rules
// the Mongo store enforces, so the usecase tests exercise real behaviour.
type fakeTicketStore struct {
	tickets   []domain.Ticket
	config    map[string]int
	applied   []TicketCommand
	merged    []TicketCommandMergeTarget
	revoked   []string
	logs      []domain.OperationLog
	events    []TicketEvent
	escalated map[string]bool
	reminded  map[string]bool
}

func newTicketStore(tickets []domain.Ticket) *fakeTicketStore {
	return &fakeTicketStore{
		tickets:   tickets,
		config:    map[string]int{},
		escalated: map[string]bool{},
		reminded:  map[string]bool{},
	}
}

func (f *fakeTicketStore) find(ticketNo string) (int, domain.Ticket, bool) {
	for i, ticket := range f.tickets {
		if ticket.TicketNo == ticketNo {
			return i, ticket, true
		}
	}
	return -1, domain.Ticket{}, false
}

func (f *fakeTicketStore) ListTickets(_ context.Context, filter TicketFilter) ([]domain.Ticket, int64, error) {
	out := make([]domain.Ticket, 0, len(f.tickets))
	for _, ticket := range f.tickets {
		if filter.State != "" && ticket.State != filter.State {
			continue
		}
		if filter.Type != "" && ticket.Type != filter.Type {
			continue
		}
		out = append(out, ticket)
	}
	return out, int64(len(out)), nil
}

func (f *fakeTicketStore) FindTicket(_ context.Context, ticketNo string) (domain.Ticket, error) {
	if _, ticket, ok := f.find(ticketNo); ok {
		return ticket, nil
	}
	return domain.Ticket{}, domain.ErrTicketNotFound
}

func (f *fakeTicketStore) ListDuplicates(_ context.Context, ticket domain.Ticket, window time.Duration) ([]domain.Ticket, error) {
	out := make([]domain.Ticket, 0)
	for _, candidate := range f.tickets {
		if ticket.IsDuplicate(candidate, window) {
			out = append(out, candidate)
		}
	}
	return out, nil
}

func (f *fakeTicketStore) PassengerBriefs(_ context.Context, ids []string) (map[string]PassengerBrief, error) {
	out := make(map[string]PassengerBrief, len(ids))
	for _, id := range ids {
		out[id] = PassengerBrief{Name: "乘客A", Phone: "138****6688"}
	}
	return out, nil
}

func (f *fakeTicketStore) DriverBriefs(_ context.Context, ids []string) (map[string]DriverBrief, error) {
	out := make(map[string]DriverBrief, len(ids))
	for _, id := range ids {
		out[id] = DriverBrief{Name: "王师傅", Plate: "沪AD·66666"}
	}
	return out, nil
}

func (f *fakeTicketStore) ExecuteTicketCommand(_ context.Context, command TicketCommand) (domain.Ticket, error) {
	f.applied = append(f.applied, command)
	index, ticket, ok := f.find(command.TicketNo)
	if !ok {
		return domain.Ticket{}, domain.ErrTicketNotFound
	}
	if ticket.State != command.ExpectedState {
		return domain.Ticket{}, domain.ErrTicketStateConflict
	}
	ticket.State = command.TargetState
	if command.Assignee != "" {
		ticket.Assignee = command.Assignee
	}
	if command.OpenRound {
		ticket.Round++
		ticket.Rounds = append(ticket.Rounds, domain.Round{
			Round:     ticket.Round,
			Handler:   command.Assignee,
			State:     command.TargetState,
			StartedAt: command.Now,
		})
	}
	if command.FirstResponse {
		at := command.Now
		ticket.FirstRespondedAt = &at
	}
	if command.FreezeClock {
		at := command.Now
		ticket.PausedAt = &at
	}
	if command.ResumeFrom != nil {
		ticket.PausedMs += command.Now.Sub(*command.ResumeFrom).Milliseconds()
		ticket.PausedAt = nil
	}
	if command.MarkFalse {
		ticket.FalseReport = &domain.FalseReport{Revoked: true, By: command.Actor, At: command.Now, Reason: command.Message}
	}
	ticket.Timeline = append(ticket.Timeline, domain.TimelineEntry{
		Seq:     len(ticket.Timeline) + 1,
		Actor:   command.Actor,
		Action:  command.Command,
		From:    command.ExpectedState,
		To:      command.TargetState,
		Message: command.Message,
		At:      command.Now,
	})
	f.tickets[index] = ticket
	return ticket, nil
}

func (f *fakeTicketStore) MergeTicket(_ context.Context, main, duplicate TicketCommandMergeTarget) error {
	f.merged = append(f.merged, duplicate)
	index, ticket, ok := f.find(duplicate.TicketNo)
	if !ok {
		return domain.ErrTicketNotFound
	}
	ticket.State = domain.TicketStateClosed
	ticket.MergedInto = main.TicketNo
	f.tickets[index] = ticket
	mainIndex, mainTicket, _ := f.find(main.TicketNo)
	mainTicket.MergedTicketNos = append(mainTicket.MergedTicketNos, duplicate.TicketNo)
	f.tickets[mainIndex] = mainTicket
	return nil
}

func (f *fakeTicketStore) RevokeTicketRestrictions(_ context.Context, ticketNo, _, _, _ string, _ time.Time) (int, error) {
	f.revoked = append(f.revoked, ticketNo)
	return 1, nil
}

func (f *fakeTicketStore) EscalateOverdue(_ context.Context, now time.Time, _ int) ([]string, error) {
	out := make([]string, 0)
	for i, ticket := range f.tickets {
		if ticket.FirstRespondedAt != nil || ticket.Escalated || f.escalated[ticket.TicketNo] {
			continue
		}
		if !ticket.Overdue(now) {
			continue
		}
		f.escalated[ticket.TicketNo] = true
		f.tickets[i].Escalated = true
		f.tickets[i].Urgent = true
		out = append(out, ticket.TicketNo)
	}
	return out, nil
}

func (f *fakeTicketStore) RemindDuty(_ context.Context, deadline time.Time, _ int) ([]string, error) {
	out := make([]string, 0)
	for i, ticket := range f.tickets {
		if ticket.Type != domain.TicketTypeDangerous || ticket.State != domain.TicketStatePending {
			continue
		}
		if ticket.Assignee != "" || ticket.DutyRemindedAt != nil || f.reminded[ticket.TicketNo] {
			continue
		}
		if ticket.CreatedAt.After(deadline) {
			continue
		}
		f.reminded[ticket.TicketNo] = true
		at := ticket.CreatedAt
		f.tickets[i].DutyRemindedAt = &at
		out = append(out, ticket.TicketNo)
	}
	return out, nil
}

func (f *fakeTicketStore) EscalateDuty(_ context.Context, deadline time.Time, _ int) ([]string, error) {
	out := make([]string, 0)
	for i, ticket := range f.tickets {
		if ticket.Type != domain.TicketTypeDangerous || ticket.State != domain.TicketStatePending {
			continue
		}
		if ticket.Assignee != "" || ticket.Escalated {
			continue
		}
		if ticket.CreatedAt.After(deadline) {
			continue
		}
		f.tickets[i].Escalated = true
		f.tickets[i].Urgent = true
		out = append(out, ticket.TicketNo)
	}
	return out, nil
}

func (f *fakeTicketStore) EmitTicketEvent(_ context.Context, event TicketEvent) error {
	f.events = append(f.events, event)
	return nil
}

func (f *fakeTicketStore) ReadTicketConfig(_ context.Context, key string, fallback int) int {
	if value, ok := f.config[key]; ok {
		return value
	}
	return fallback
}

func (f *fakeTicketStore) RecordOperationLog(_ context.Context, log domain.OperationLog) error {
	f.logs = append(f.logs, log)
	return nil
}

// unused keeps the compiler honest about the error helper if rules change.
var _ = errors.Is
