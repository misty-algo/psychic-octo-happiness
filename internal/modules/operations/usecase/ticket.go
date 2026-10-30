// A04 投诉工单处理用例层：工单处理台、命令执行与超时扫描。
package usecase

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/NSObjects/echo-admin/internal/modules/operations/domain"
	"github.com/NSObjects/echo-admin/internal/platform/apperr"
)

// TicketStore persists ticket reads and writes. tickets is owned by this
// module; orders, accounts and drivers are read-only.
type TicketStore interface {
	// ListTickets returns one page of tickets plus the total count.
	ListTickets(ctx context.Context, filter TicketFilter) ([]domain.Ticket, int64, error)
	// FindTicket returns one ticket or domain.ErrTicketNotFound.
	FindTicket(ctx context.Context, ticketNo string) (domain.Ticket, error)
	// ListDuplicates returns same-order same-type tickets created inside the
	// window, excluding the ticket itself.
	ListDuplicates(ctx context.Context, ticket domain.Ticket, window time.Duration) ([]domain.Ticket, error)
	// PassengerBriefs returns masked passenger identity for the given ids.
	PassengerBriefs(ctx context.Context, ids []string) (map[string]PassengerBrief, error)
	// DriverBriefs returns driver name/plate for the given driver ids.
	DriverBriefs(ctx context.Context, ids []string) (map[string]DriverBrief, error)
	// ExecuteTicketCommand applies one command under a state compare-and-set.
	ExecuteTicketCommand(ctx context.Context, command TicketCommand) (domain.Ticket, error)
	// MergeTicket marks the duplicate closed and attaches it to the main ticket.
	MergeTicket(ctx context.Context, main, duplicate TicketCommandMergeTarget) error
	// RevokeTicketRestrictions lifts the limits this ticket imposed on a driver.
	RevokeTicketRestrictions(ctx context.Context, ticketNo, driverAccountID, operator, reason string, at time.Time) (int, error)
	// EscalateOverdue marks every unanswered ticket past its deadline urgent.
	EscalateOverdue(ctx context.Context, now time.Time, limit int) ([]string, error)
	// RemindDuty pushes unclaimed dangerous-driving tickets to the duty queue.
	RemindDuty(ctx context.Context, deadline time.Time, limit int) ([]string, error)
	// EscalateDuty escalates dangerous-driving tickets nobody claimed in time.
	EscalateDuty(ctx context.Context, deadline time.Time, limit int) ([]string, error)
	// EmitTicketEvent writes one idempotent outbox row.
	EmitTicketEvent(ctx context.Context, event TicketEvent) error
	// ReadTicketConfig reads one config key with its SSOT default.
	ReadTicketConfig(ctx context.Context, key string, fallback int) int
	// RecordOperationLog appends one business-side audit record.
	RecordOperationLog(ctx context.Context, log domain.OperationLog) error
}

// TicketFilter narrows the ticket workbench.
type TicketFilter struct {
	Keyword string // ticket no or order no
	State   string
	Type    string
	Group   string
	// OnlyOverdue keeps tickets whose first response is already late.
	OnlyOverdue bool
	Skip        int
	Limit       int
}

// ListTicketsInput carries the workbench query.
type ListTicketsInput struct {
	Keyword  string
	State    string
	Type     string
	Group    string
	Overdue  bool
	Page     int
	PageSize int
}

// TicketRow is one workbench row: the ticket plus what the clock says about it.
type TicketRow struct {
	TicketNo       string    `json:"ticket_no"`
	OrderNo        string    `json:"order_no"`
	Type           string    `json:"type"`
	Group          string    `json:"group"`
	CityCode       string    `json:"city_code"`
	PassengerName  string    `json:"passenger_name"`
	PassengerPhone string    `json:"passenger_phone"`
	DriverName     string    `json:"driver_name"`
	Plate          string    `json:"plate"`
	State          string    `json:"state"`
	Urgent         bool      `json:"urgent"`
	Assignee       string    `json:"assignee"`
	Round          int       `json:"round"`
	CreatedAt      time.Time `json:"created_at"`
	DueAt          time.Time `json:"first_response_due_at"`
	// RemainingHours is how much of the first-response window is left, frozen
	// while the ticket waits for the passenger.
	RemainingHours float64 `json:"remaining_hours"`
	Overdue        bool    `json:"overdue"`
	Paused         bool    `json:"paused"`
	Escalated      bool    `json:"escalated"`
	// Rank orders the queue: urgent and overdue float to the top.
	Rank int `json:"rank"`
}

// ListTicketsOutput is one page of ticket rows.
type ListTicketsOutput struct {
	Items []TicketRow `json:"items"`
	Total int64       `json:"total"`
}

// TicketCommandInput carries one command request from the console.
type TicketCommandInput struct {
	TicketNo   string
	Command    string
	Message    string
	Assignee   string // transfer target
	MergeNo    string // merge source
	OperatorID int64
	RequestID  string
}

// TicketCommandOutput echoes what the command did.
type TicketCommandOutput struct {
	TicketNo string `json:"ticket_no"`
	Command  string `json:"command"`
	State    string `json:"state"`
	Round    int    `json:"round"`
	Assignee string `json:"assignee"`
	DueAt    string `json:"first_response_due_at"`
}

// TicketCommand is the compare-and-set payload handed to the store. The
// expected state is part of the filter, so a concurrent transition loses the
// race instead of overwriting the winner.
type TicketCommand struct {
	TicketNo      string
	Command       string
	Message       string
	Actor         string
	Assignee      string
	MergeTicketNo string
	ExpectedState string
	TargetState   string
	Now           time.Time
	// FreezeClock marks the moment the timer pauses; ResumeFrom adds the elapsed
	// pause back to the accumulated total.
	FreezeClock bool
	ResumeFrom  *time.Time
	// FirstResponse stamps the first reply, which stops the response clock.
	FirstResponse bool
	OpenRound     bool
	MarkFalse     bool
	UrgentDueAt   time.Time
}

// TicketCommandMergeTarget identifies one side of a merge.
type TicketCommandMergeTarget struct {
	TicketNo string
	Actor    string
	Message  string
	At       time.Time
}

// TicketEvent is one outbox row for downstream consumers.
type TicketEvent struct {
	EventID     string
	Type        string
	Aggregate   string
	AggregateID string
	Payload     map[string]any
	At          time.Time
}

// TicketScanReport summarises one sweeper pass for logs and tests.
type TicketScanReport struct {
	Escalated     []string `json:"escalated"`
	Reminded      []string `json:"reminded"`
	DutyEscalated []string `json:"duty_escalated"`
}

// TicketUsecase implements A04 complaint ticket handling.
type TicketUsecase struct {
	store TicketStore
	now   func() time.Time
}

// NewTicket creates a ticket usecase.
func NewTicket(store TicketStore) *TicketUsecase {
	return &TicketUsecase{store: store, now: time.Now}
}

// ListTickets returns one page of the workbench, urgent and overdue first.
func (u *TicketUsecase) ListTickets(ctx context.Context, in ListTicketsInput) (ListTicketsOutput, error) {
	filter := TicketFilter{
		Keyword:     strings.TrimSpace(in.Keyword),
		State:       strings.TrimSpace(in.State),
		Type:        strings.TrimSpace(in.Type),
		Group:       strings.TrimSpace(in.Group),
		OnlyOverdue: in.Overdue,
		Skip:        (in.Page - 1) * in.PageSize,
		Limit:       in.PageSize,
	}
	if filter.Skip < 0 {
		filter.Skip = 0
	}
	if filter.Limit <= 0 || filter.Limit > maxPageSize {
		filter.Limit = maxPageSize
	}
	tickets, total, err := u.store.ListTickets(ctx, filter)
	if err != nil {
		return ListTicketsOutput{}, err
	}
	rows, err := u.enrichTickets(ctx, tickets)
	if err != nil {
		return ListTicketsOutput{}, err
	}
	now := u.now()
	for i := range rows {
		rankTicket(&rows[i], now)
	}
	sortTicketRows(rows)
	if filter.OnlyOverdue {
		rows = keepOverdue(rows)
	}
	return ListTicketsOutput{Items: rows, Total: total}, nil
}

// Detail returns one ticket plus its duplicate candidates.
func (u *TicketUsecase) Detail(ctx context.Context, ticketNo string) (domain.TicketDetail, error) {
	if err := domain.ValidateTicketNo(ticketNo); err != nil {
		return domain.TicketDetail{}, apperr.WrapBadRequest(err, "工单号不合法")
	}
	ticket, err := u.store.FindTicket(ctx, ticketNo)
	if err != nil {
		return domain.TicketDetail{}, mapTicketError(err)
	}
	rows, err := u.enrichTickets(ctx, []domain.Ticket{ticket})
	if err != nil {
		return domain.TicketDetail{}, err
	}
	ticket.PassengerName = rows[0].PassengerName
	ticket.PassengerPhone = rows[0].PassengerPhone
	ticket.DriverName = rows[0].DriverName
	ticket.Plate = rows[0].Plate
	window := time.Duration(u.store.ReadTicketConfig(ctx, ConfigTicketMergeWindowHours, DefaultMergeWindowHours)) * time.Hour
	duplicates, err := u.store.ListDuplicates(ctx, ticket, window)
	if err != nil {
		return domain.TicketDetail{}, fmt.Errorf("list duplicate tickets: %w", err)
	}
	enriched, err := u.enrichTickets(ctx, duplicates)
	if err != nil {
		return domain.TicketDetail{}, err
	}
	for i := range duplicates {
		duplicates[i].PassengerName = enriched[i].PassengerName
		duplicates[i].PassengerPhone = enriched[i].PassengerPhone
		duplicates[i].DriverName = enriched[i].DriverName
		duplicates[i].Plate = enriched[i].Plate
	}
	return domain.TicketDetail{Ticket: ticket, Duplicates: duplicates}, nil
}

// Execute validates the command against the ticket and applies it.
func (u *TicketUsecase) Execute(ctx context.Context, in TicketCommandInput) (TicketCommandOutput, error) {
	if err := domain.ValidateTicketNo(in.TicketNo); err != nil {
		return TicketCommandOutput{}, apperr.WrapBadRequest(err, "工单号不合法")
	}
	command, err := domain.NewTicketCommand(in.Command, in.Message)
	if err != nil {
		return TicketCommandOutput{}, mapTicketError(err)
	}
	ticket, err := u.store.FindTicket(ctx, in.TicketNo)
	if err != nil {
		return TicketCommandOutput{}, mapTicketError(err)
	}
	actor := operatorName(in.OperatorID)
	message := strings.TrimSpace(in.Message)
	if preErr := ticket.CheckCommand(command, actor); preErr != nil {
		return TicketCommandOutput{}, mapTicketError(preErr)
	}
	now := u.now()
	// 合并先在库外校验：待合并工单必须是同订单同类型且在同一窗口内，校验不过
	// 就不能往工单上写任何东西，否则时间线会留下一条失败的"合并"。
	if command == domain.TicketCommandMerge {
		if err := u.validateMerge(ctx, ticket, in); err != nil {
			return TicketCommandOutput{}, mapTicketError(err)
		}
	}
	fromState := ticket.State
	applied, err := u.buildCommand(ctx, ticket, command, in, actor, now)
	if err != nil {
		return TicketCommandOutput{}, mapTicketError(err)
	}
	appliedTicket, err := u.store.ExecuteTicketCommand(ctx, applied)
	if err != nil {
		return TicketCommandOutput{}, mapTicketError(err)
	}
	if command == domain.TicketCommandMerge {
		if err := u.merge(ctx, appliedTicket, in, actor, now); err != nil {
			return TicketCommandOutput{}, err
		}
	}
	if command == domain.TicketCommandRevoke {
		lifted, err := u.restoreDriver(ctx, appliedTicket, actor, now)
		if err != nil {
			return TicketCommandOutput{}, err
		}
		// 撤销了几条就写几条：零条也要写清楚，避免"提示成功但司机没恢复"。
		message = fmt.Sprintf("%s（已撤销该司机 %d 条生效中的限制）", message, lifted)
	}
	if err := u.recordLog(ctx, appliedTicket, fromState, command, message, actor, in.RequestID, now); err != nil {
		return TicketCommandOutput{}, err
	}
	if err := u.emitLifecycleEvent(ctx, appliedTicket, command, now); err != nil {
		return TicketCommandOutput{}, err
	}
	return TicketCommandOutput{
		TicketNo: appliedTicket.TicketNo,
		Command:  command,
		State:    appliedTicket.State,
		Round:    appliedTicket.Round,
		Assignee: appliedTicket.Assignee,
		DueAt:    appliedTicket.FirstResponseDueAt.Format(time.RFC3339),
	}, nil
}

// buildCommand turns a validated request into the store payload, translating
// the clock rules into concrete timestamp updates.
func (u *TicketUsecase) buildCommand(ctx context.Context, ticket domain.Ticket, command string, in TicketCommandInput, actor string, now time.Time) (TicketCommand, error) {
	applied := TicketCommand{
		TicketNo:      ticket.TicketNo,
		Command:       command,
		Message:       strings.TrimSpace(in.Message),
		Actor:         actor,
		Assignee:      strings.TrimSpace(in.Assignee),
		MergeTicketNo: strings.TrimSpace(in.MergeNo),
		ExpectedState: ticket.State,
		TargetState:   ticket.CommandTargetState(command),
		Now:           now,
	}
	switch command {
	case domain.TicketCommandClaim:
		if applied.Assignee == "" {
			applied.Assignee = actor
		}
		applied.OpenRound = true
	case domain.TicketCommandReply:
		// 首次回复打点：之后的回复不再影响计时。
		applied.FirstResponse = ticket.FirstRespondedAt == nil
	case domain.TicketCommandWaitUser:
		applied.FreezeClock = true
	case domain.TicketCommandResume:
		if ticket.PausedAt != nil {
			pausedFrom := *ticket.PausedAt
			applied.ResumeFrom = &pausedFrom
		}
	case domain.TicketCommandAppeal:
		applied.Assignee = actor
		applied.OpenRound = true
	case domain.TicketCommandTransfer:
		if applied.Assignee == "" {
			return TicketCommand{}, ErrTransferAssigneeRequired
		}
		if applied.Assignee == ticket.Assignee {
			return TicketCommand{}, ErrTransferSameAssignee
		}
		// 转派不刷新首次回复时限：这里刻意不动 DueAt。
	case domain.TicketCommandEscalate:
		hours := u.store.ReadTicketConfig(ctx, ConfigTicketUrgentResponseHours, DefaultUrgentResponseHours)
		applied.UrgentDueAt = now.Add(time.Duration(hours) * time.Hour)
	case domain.TicketCommandRevoke:
		applied.MarkFalse = true
	case domain.TicketCommandMerge:
		if applied.MergeTicketNo == "" {
			return TicketCommand{}, ErrMergeTargetRequired
		}
	}
	return applied, nil
}

// validateMerge rejects a merge whose target is not a real duplicate.
func (u *TicketUsecase) validateMerge(ctx context.Context, ticket domain.Ticket, in TicketCommandInput) error {
	if strings.TrimSpace(in.MergeNo) == "" {
		return ErrMergeTargetRequired
	}
	window := time.Duration(u.store.ReadTicketConfig(ctx, ConfigTicketMergeWindowHours, DefaultMergeWindowHours)) * time.Hour
	duplicate, err := u.store.FindTicket(ctx, strings.TrimSpace(in.MergeNo))
	if err != nil {
		return mapTicketError(err)
	}
	if !ticket.IsDuplicate(duplicate, window) {
		return apperr.WrapConflict(domain.ErrMergeTargetInvalid,
			fmt.Sprintf("仅同订单同类型且 %d 小时内的投诉可合并", int(window.Hours())))
	}
	return nil
}

// merge folds the already validated duplicate into the main ticket.
func (u *TicketUsecase) merge(ctx context.Context, ticket domain.Ticket, in TicketCommandInput, actor string, now time.Time) error {
	err := u.store.MergeTicket(ctx,
		TicketCommandMergeTarget{TicketNo: ticket.TicketNo, Actor: actor, Message: strings.TrimSpace(in.Message), At: now},
		TicketCommandMergeTarget{TicketNo: strings.TrimSpace(in.MergeNo), Actor: actor, Message: strings.TrimSpace(in.Message), At: now})
	if err != nil {
		return fmt.Errorf("merge ticket: %w", err)
	}
	return nil
}

// restoreDriver lifts the limits the false report imposed and reports how many
// were actually lifted. Zero is a real answer, not a failure: it means the
// driver had no active restriction to undo.
func (u *TicketUsecase) restoreDriver(ctx context.Context, ticket domain.Ticket, actor string, now time.Time) (int, error) {
	if strings.TrimSpace(ticket.DriverID) == "" {
		return 0, nil
	}
	lifted, err := u.store.RevokeTicketRestrictions(ctx, ticket.TicketNo, ticket.DriverID, actor, "危险驾驶核实为误报，恢复司机状态", now)
	if err != nil {
		return 0, fmt.Errorf("restore driver status: %w", err)
	}
	return lifted, nil
}

// recordLog appends the business-side audit row for one command: who, what,
// from which state to which state, and what was said.
func (u *TicketUsecase) recordLog(ctx context.Context, ticket domain.Ticket, fromState, command, message, actor, requestID string, now time.Time) error {
	if err := u.store.RecordOperationLog(ctx, domain.OperationLog{
		ID:        fmt.Sprintf("%s:%s:%d", ticket.TicketNo, command, now.UnixNano()),
		Module:    "operations",
		Operator:  actor,
		Action:    "ticket_command",
		Resource:  "ops_ticket",
		Target:    ticket.TicketNo,
		RequestID: requestID,
		Detail:    fmt.Sprintf("%s %s→%s 第%d轮 备注：%s", command, fromState, ticket.State, ticket.Round, message),
		At:        now,
	}); err != nil {
		return fmt.Errorf("record ticket command log: %w", err)
	}
	return nil
}

// emitLifecycleEvent publishes the events downstream consumers subscribe to.
func (u *TicketUsecase) emitLifecycleEvent(ctx context.Context, ticket domain.Ticket, command string, now time.Time) error {
	eventType := ""
	switch command {
	case domain.TicketCommandResolve:
		eventType = domain.TicketResolvedEventType
	case domain.TicketCommandRevoke:
		eventType = domain.TicketFalseReportEventType
	default:
		return nil
	}
	event := TicketEvent{
		EventID:     fmt.Sprintf("%s:%s:%d", eventType, ticket.TicketNo, ticket.Round),
		Type:        eventType,
		Aggregate:   "ticket",
		AggregateID: ticket.TicketNo,
		Payload: map[string]any{
			"ticket_no":   ticket.TicketNo,
			"order_no":    ticket.OrderNo,
			"type":        ticket.Type,
			"state":       ticket.State,
			"round":       ticket.Round,
			"driver_id":   ticket.DriverID,
			"complainant": ticket.ComplainantID,
		},
		At: now,
	}
	if err := u.store.EmitTicketEvent(ctx, event); err != nil {
		return fmt.Errorf("emit ticket event: %w", err)
	}
	return nil
}

// ScanOnce runs one sweeper pass: escalate overdue tickets, then push and
// escalate unclaimed dangerous-driving tickets. Every write is idempotent.
func (u *TicketUsecase) ScanOnce(ctx context.Context) (TicketScanReport, error) {
	now := u.now()
	report := TicketScanReport{}
	overdue, err := u.store.EscalateOverdue(ctx, now, SweepBatchSize)
	if err != nil {
		return report, fmt.Errorf("escalate overdue tickets: %w", err)
	}
	report.Escalated = overdue
	remindSeconds := u.store.ReadTicketConfig(ctx, ConfigTicketDutyRemindSeconds, DefaultDutyRemindSeconds)
	reminded, err := u.store.RemindDuty(ctx, now.Add(-time.Duration(remindSeconds)*time.Second), SweepBatchSize)
	if err != nil {
		return report, fmt.Errorf("remind duty queue: %w", err)
	}
	report.Reminded = reminded
	escalateSeconds := u.store.ReadTicketConfig(ctx, ConfigTicketDutyEscalateSeconds, DefaultDutyEscalateSeconds)
	escalated, err := u.store.EscalateDuty(ctx, now.Add(-time.Duration(escalateSeconds)*time.Second), SweepBatchSize)
	if err != nil {
		return report, fmt.Errorf("escalate duty tickets: %w", err)
	}
	report.DutyEscalated = escalated
	for _, ticketNo := range append(append([]string{}, overdue...), escalated...) {
		if err := u.store.EmitTicketEvent(ctx, TicketEvent{
			EventID:     fmt.Sprintf("%s:%s", domain.TicketEscalatedEventType, ticketNo),
			Type:        domain.TicketEscalatedEventType,
			Aggregate:   "ticket",
			AggregateID: ticketNo,
			Payload:     map[string]any{"ticket_no": ticketNo, "reason": "超时未首次回复或危险驾驶无人领单"},
			At:          now,
		}); err != nil {
			return report, fmt.Errorf("emit escalation event: %w", err)
		}
	}
	for _, ticketNo := range reminded {
		if err := u.store.EmitTicketEvent(ctx, TicketEvent{
			EventID:     fmt.Sprintf("%s:%s", domain.TicketDutyReminderEventType, ticketNo),
			Type:        domain.TicketDutyReminderEventType,
			Aggregate:   "ticket",
			AggregateID: ticketNo,
			Payload:     map[string]any{"ticket_no": ticketNo, "reason": "危险驾驶工单一分钟无人领单"},
			At:          now,
		}); err != nil {
			return report, fmt.Errorf("emit duty reminder event: %w", err)
		}
	}
	return report, nil
}

// enrichTickets attaches passenger and driver identity to tickets.
func (u *TicketUsecase) enrichTickets(ctx context.Context, tickets []domain.Ticket) ([]TicketRow, error) {
	passengerIDs := make([]string, 0, len(tickets))
	driverIDs := make([]string, 0, len(tickets))
	for _, ticket := range tickets {
		if ticket.ComplainantID != "" {
			passengerIDs = append(passengerIDs, ticket.ComplainantID)
		}
		if ticket.DriverID != "" {
			driverIDs = append(driverIDs, ticket.DriverID)
		}
	}
	passengers, err := u.store.PassengerBriefs(ctx, passengerIDs)
	if err != nil {
		return nil, fmt.Errorf("load passenger briefs: %w", err)
	}
	drivers, err := u.store.DriverBriefs(ctx, driverIDs)
	if err != nil {
		return nil, fmt.Errorf("load driver briefs: %w", err)
	}
	rows := make([]TicketRow, 0, len(tickets))
	for _, ticket := range tickets {
		row := TicketRow{
			TicketNo:  ticket.TicketNo,
			OrderNo:   ticket.OrderNo,
			Type:      ticket.Type,
			Group:     ticket.Group,
			CityCode:  ticket.CityCode,
			State:     ticket.State,
			Urgent:    ticket.Urgent,
			Assignee:  ticket.Assignee,
			Round:     ticket.Round,
			CreatedAt: ticket.CreatedAt,
			DueAt:     ticket.FirstResponseDueAt,
			Paused:    ticket.Paused(),
			Escalated: ticket.Escalated,
		}
		if brief, ok := passengers[ticket.ComplainantID]; ok {
			row.PassengerName = brief.Name
			row.PassengerPhone = brief.Phone
		}
		if brief, ok := drivers[ticket.DriverID]; ok {
			row.DriverName = brief.Name
			row.Plate = brief.Plate
		}
		rows = append(rows, row)
	}
	return rows, nil
}

// rankTicket fills the clock columns and the queue rank: overdue first, then
// urgent, then everything else. Remaining hours are frozen while the ticket
// waits for the passenger, matching the pause rule.
func rankTicket(row *TicketRow, now time.Time) {
	if row.DueAt.IsZero() {
		return
	}
	remaining := row.DueAt.Sub(now)
	if row.Paused {
		remaining = 0
	}
	row.RemainingHours = remaining.Hours()
	row.Overdue = remaining <= 0 &&
		row.State != domain.TicketStateResolved &&
		row.State != domain.TicketStateClosed
	switch {
	case row.Overdue:
		row.Rank = 0
	case row.Urgent:
		row.Rank = 1
	default:
		row.Rank = 2
	}
}

// sortTicketRows orders the workbench: overdue, urgent, then newest first.
func sortTicketRows(rows []TicketRow) {
	for i := 1; i < len(rows); i++ {
		for j := i; j > 0; j-- {
			left, right := rows[j-1], rows[j]
			if left.Rank < right.Rank {
				break
			}
			if left.Rank == right.Rank && left.CreatedAt.After(right.CreatedAt) {
				break
			}
			rows[j-1], rows[j] = right, left
		}
	}
}

// keepOverdue drops rows that are not late, for the overdue-only filter.
func keepOverdue(rows []TicketRow) []TicketRow {
	out := make([]TicketRow, 0, len(rows))
	for _, row := range rows {
		if row.Overdue {
			out = append(out, row)
		}
	}
	return out
}
