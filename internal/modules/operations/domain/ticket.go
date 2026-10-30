// A04 投诉工单处理：工单视图、五态流转、计时器与业务命令规则。
// 规则基线：《业务规则说明书》A04 投诉工单处理、《各模块技术点与实行方案》运营管理 4。
package domain

import (
	"errors"
	"strings"
	"time"
	"unicode/utf8"
)

// 工单五态。状态取值与 SSOT 完全一致，管理端不自定义第六种状态。
const (
	// TicketStatePending 待处理：乘客已投诉，等待客服领单。
	TicketStatePending = "PENDING"
	// TicketStateProcessing 处理中：已领单并在处理。
	TicketStateProcessing = "PROCESSING"
	// TicketStateWaitingUser 等待用户：等乘客补充材料，计时暂停。
	TicketStateWaitingUser = "WAITING_USER"
	// TicketStateResolved 已完结：已给出处理结果。
	TicketStateResolved = "RESOLVED"
	// TicketStateClosed 已关闭：乘客撤诉或无效单。
	TicketStateClosed = "CLOSED"
)

// 投诉类型沿用乘客端枚举：绕路、服务态度、危险驾驶、费用异议、其他。
const (
	TicketTypeDetour    = "DETOUR"
	TicketTypeAttitude  = "SERVICE_ATTITUDE"
	TicketTypeDangerous = "DANGEROUS_DRIVING"
	TicketTypeFare      = "FARE_DISPUTE"
	TicketTypeOther     = "OTHER"
)

// 自动路由分组：分类决定谁来处理，危险驾驶走值班队列。
const (
	TicketGroupService = "客服组"
	TicketGroupSafety  = "安全组"
	TicketGroupFinance = "财务组"
)

// 工单命令。与订单模块一样，管理端没有"任意改状态"的入口，所有动作都是
// 带前置条件、带权限、带留痕的命令。
const (
	// TicketCommandClaim 领单：待处理 → 处理中。
	TicketCommandClaim = "claim"
	// TicketCommandReply 回复：首次回复即停止首次回复计时。
	TicketCommandReply = "reply"
	// TicketCommandWaitUser 等待用户补充：计时暂停。
	TicketCommandWaitUser = "wait_user"
	// TicketCommandResume 用户回应：计时按剩余时长续算。
	TicketCommandResume = "resume"
	// TicketCommandResolve 解决：处理中 → 已完结。
	TicketCommandResolve = "resolve"
	// TicketCommandClose 关闭：乘客撤诉或无效单。
	TicketCommandClose = "close"
	// TicketCommandTransfer 转派换人，首次回复时限不刷新。
	TicketCommandTransfer = "transfer"
	// TicketCommandEscalate 升级为紧急，通常由超时自动触发，也可人工升级。
	TicketCommandEscalate = "escalate"
	// TicketCommandAppeal 乘客申诉：开启新一轮次，必须换人且由高级客服处理。
	TicketCommandAppeal = "appeal"
	// TicketCommandRevoke 危险驾驶误报撤销：关单 + 恢复司机状态 + 记录误报。
	TicketCommandRevoke = "revoke"
	// TicketCommandMerge 重复投诉合并：同订单同类型 24 小时内合并到主工单。
	TicketCommandMerge = "merge"
)

// 事件类型：与审核模块共用 event_logs 出箱表，消费者按 eventId 幂等去重。
const (
	TicketEscalatedEventType    = "ticket.escalated"
	TicketDutyReminderEventType = "ticket.duty.reminder"
	TicketFalseReportEventType  = "ticket.false_report.revoked"
	TicketResolvedEventType     = "ticket.resolved"
)

// 工单规则错误。
var (
	ErrTicketNotFound       = errors.New("ticket not found")
	ErrInvalidTicketNo      = errors.New("invalid ticket no")
	ErrInvalidTicketCommand = errors.New("invalid ticket command")
	ErrInvalidTicketMessage = errors.New("invalid ticket message")
	ErrTicketStateConflict  = errors.New("ticket state rejects the command")
	ErrTicketClaimed        = errors.New("ticket already claimed")
	ErrTicketNotAssignee    = errors.New("operator is not the ticket assignee")
	ErrTicketNotWaiting     = errors.New("ticket is not waiting for user")
	ErrAppealSameHandler    = errors.New("appeal must be handled by another agent")
	ErrRevokeNotDangerous   = errors.New("only dangerous-driving tickets can be revoked as false report")
	ErrMergeTargetInvalid   = errors.New("merge target is not a duplicate of this ticket")
	ErrTicketTerminal       = errors.New("ticket reached a terminal state")
)

// Ticket is the management-side view of one complaint ticket. Tickets are the
// one place where operations writes back into the business flow, so every
// mutable field here is owned by this module.
type Ticket struct {
	ID             string    `json:"id"`
	TicketNo       string    `json:"ticket_no"`
	OrderID        string    `json:"order_id"`
	OrderNo        string    `json:"order_no"`
	Type           string    `json:"type"`
	Group          string    `json:"group"`
	CityCode       string    `json:"city_code"`
	ComplainantID  string    `json:"complainant_id"`
	PassengerName  string    `json:"passenger_name"`
	PassengerPhone string    `json:"passenger_phone"`
	DriverID       string    `json:"driver_id"`
	DriverName     string    `json:"driver_name"`
	Plate          string    `json:"plate"`
	Content        string    `json:"content"`
	EvidenceCount  int       `json:"evidence_count"`
	State          string    `json:"state"`
	Urgent         bool      `json:"urgent"`
	Assignee       string    `json:"assignee"`
	Round          int       `json:"round"`
	Rounds         []Round   `json:"rounds"`
	CreatedAt      time.Time `json:"created_at"`

	// 计时器：存绝对到期时间戳，暂停累计毫秒数，恢复按剩余时长续算。
	FirstResponseDueAt  time.Time  `json:"first_response_due_at"`
	FirstRespondedAt    *time.Time `json:"first_responded_at"`
	UrgentResponseDueAt *time.Time `json:"urgent_response_due_at"`
	PausedMs            int64      `json:"paused_ms"`
	PausedAt            *time.Time `json:"paused_at"`

	Escalated      bool       `json:"escalated"`
	EscalatedAt    *time.Time `json:"escalated_at"`
	EscalateReason string     `json:"escalate_reason"`
	DutyRemindedAt *time.Time `json:"duty_reminded_at"`

	MergedTicketNos []string     `json:"merged_ticket_nos"`
	MergedInto      string       `json:"merged_into"`
	FalseReport     *FalseReport `json:"false_report"`

	Timeline []TimelineEntry `json:"timeline"`
}

// Round is one handling round. An appeal opens a new round with an incremented
// number; the previous handler must not take the next one.
type Round struct {
	Round     int        `json:"round"`
	Handler   string     `json:"handler"`
	State     string     `json:"state"`
	StartedAt time.Time  `json:"started_at"`
	EndedAt   *time.Time `json:"ended_at"`
	Result    string     `json:"result"`
}

// TimelineEntry records one state change: 谁、什么时间、从什么状态到什么状态、
// 说了什么 — the traceability the ticket module exists for.
type TimelineEntry struct {
	Seq     int       `json:"seq"`
	Actor   string    `json:"actor"`
	Action  string    `json:"action"`
	From    string    `json:"from"`
	To      string    `json:"to"`
	Message string    `json:"message"`
	At      time.Time `json:"at"`
}

// FalseReport records a verified false dangerous-driving report.
type FalseReport struct {
	Revoked  bool      `json:"revoked"`
	By       string    `json:"by"`
	At       time.Time `json:"at"`
	Reason   string    `json:"reason"`
	Restored bool      `json:"driver_status_restored"`
}

// TicketDetail is the workbench detail view: the ticket, the order it belongs
// to, its handling rounds and the duplicate candidates that can be merged.
type TicketDetail struct {
	Ticket     Ticket   `json:"ticket"`
	Duplicates []Ticket `json:"duplicates"`
}

// Terminal reports whether the ticket has reached an end state.
func (t Ticket) Terminal() bool {
	return t.State == TicketStateResolved || t.State == TicketStateClosed
}

// RouteGroup maps a complaint type to the group that owns it: 服务态度→客服组、
// 危险驾驶→安全组（值班队列）、费用异议→财务组、绕路→客服组（可看轨迹）、
// 其他→按订单所在城市分配。
func RouteGroup(ticketType, cityCode string) string {
	switch strings.TrimSpace(ticketType) {
	case TicketTypeAttitude:
		return TicketGroupService
	case TicketTypeDangerous:
		return TicketGroupSafety
	case TicketTypeFare:
		return TicketGroupFinance
	case TicketTypeDetour:
		return TicketGroupService
	default:
		city := strings.TrimSpace(cityCode)
		if city == "" {
			return TicketGroupService
		}
		return TicketGroupService + "·" + city
	}
}

// ValidateTicketNo rejects blank or over-long ticket numbers.
func ValidateTicketNo(ticketNo string) error {
	ticketNo = strings.TrimSpace(ticketNo)
	if ticketNo == "" || utf8.RuneCountInString(ticketNo) > 64 {
		return ErrInvalidTicketNo
	}
	return nil
}

// NewTicketCommand validates a command token and its 备注/依据. Every state
// change must carry what was said or why, so an empty message is refused.
func NewTicketCommand(command, message string) (string, error) {
	command = strings.TrimSpace(command)
	if !IsValidTicketCommand(command) {
		return "", ErrInvalidTicketCommand
	}
	message = strings.TrimSpace(message)
	if utf8.RuneCountInString(message) < BasisMinLen || utf8.RuneCountInString(message) > BasisMaxLen {
		return "", ErrInvalidTicketMessage
	}
	return command, nil
}

// IsValidTicketCommand reports whether the token is a known ticket command.
func IsValidTicketCommand(command string) bool {
	switch strings.TrimSpace(command) {
	case TicketCommandClaim, TicketCommandReply, TicketCommandWaitUser,
		TicketCommandResume, TicketCommandResolve, TicketCommandClose,
		TicketCommandTransfer, TicketCommandEscalate, TicketCommandAppeal,
		TicketCommandRevoke, TicketCommandMerge:
		return true
	default:
		return false
	}
}

// TicketCommandNames lists every command token for permission and UI wiring.
func TicketCommandNames() []string {
	return []string{
		TicketCommandClaim,
		TicketCommandReply,
		TicketCommandWaitUser,
		TicketCommandResume,
		TicketCommandResolve,
		TicketCommandClose,
		TicketCommandTransfer,
		TicketCommandEscalate,
		TicketCommandAppeal,
		TicketCommandRevoke,
		TicketCommandMerge,
	}
}

// TicketTypeNames lists the passenger-side complaint enum.
func TicketTypeNames() []string {
	return []string{
		TicketTypeDetour,
		TicketTypeAttitude,
		TicketTypeDangerous,
		TicketTypeFare,
		TicketTypeOther,
	}
}

// EffectiveDue returns the first-response deadline: the absolute due timestamp
// created with the ticket, pushed forward by every millisecond already spent
// waiting for the user. Transfer never touches it, so reassignment cannot
// reset the seven-day promise.
func (t Ticket) EffectiveDue() time.Time {
	return t.FirstResponseDueAt.Add(time.Duration(t.PausedMs) * time.Millisecond)
}

// Paused reports whether the clock is currently frozen in 等待用户.
func (t Ticket) Paused() bool {
	return t.State == TicketStateWaitingUser && t.PausedAt != nil
}

// Remaining returns how much of the first-response window is left. While the
// ticket is paused the value is frozen at the moment of the pause: a passenger
// who does not answer cannot burn the operator's clock.
func (t Ticket) Remaining(now time.Time) time.Duration {
	if t.FirstRespondedAt != nil {
		return 0
	}
	if t.Paused() {
		return t.EffectiveDue().Sub(*t.PausedAt)
	}
	return t.EffectiveDue().Sub(now)
}

// Overdue reports whether the first response is late. Only tickets that were
// never answered can be overdue.
func (t Ticket) Overdue(now time.Time) bool {
	return t.FirstRespondedAt == nil && !t.Terminal() && t.Remaining(now) <= 0
}

// UrgentRemaining returns the remaining time of the shorter urgent-response
// window, or 0 when the ticket is not urgent.
func (t Ticket) UrgentRemaining(now time.Time) time.Duration {
	if !t.Urgent || t.UrgentResponseDueAt == nil || t.Terminal() {
		return 0
	}
	return t.UrgentResponseDueAt.Sub(now)
}

// LastHandler returns the operator who handled the most recent round. An
// appeal must go to somebody else.
func (t Ticket) LastHandler() string {
	if len(t.Rounds) == 0 {
		return t.Assignee
	}
	return t.Rounds[len(t.Rounds)-1].Handler
}

// CheckCommand reports whether the ticket's current state and the acting
// operator allow the command. These preconditions are the only guard against
// illegal transitions: the console exposes no free-form state editor.
func (t Ticket) CheckCommand(command string, actor string) error {
	switch command {
	case TicketCommandClaim:
		if t.State != TicketStatePending {
			return ErrTicketStateConflict
		}
		if strings.TrimSpace(t.Assignee) != "" {
			return ErrTicketClaimed
		}
		return nil
	case TicketCommandReply:
		if t.State != TicketStateProcessing {
			return ErrTicketStateConflict
		}
		return requireAssignee(t, actor)
	case TicketCommandWaitUser:
		if t.State != TicketStateProcessing {
			return ErrTicketStateConflict
		}
		return requireAssignee(t, actor)
	case TicketCommandResume:
		if t.State != TicketStateWaitingUser {
			return ErrTicketNotWaiting
		}
		return requireAssignee(t, actor)
	case TicketCommandResolve:
		if t.State != TicketStateProcessing {
			return ErrTicketStateConflict
		}
		return requireAssignee(t, actor)
	case TicketCommandClose:
		// 乘客撤诉或无效单：待处理与已完结都可以关。
		if t.State != TicketStateResolved && t.State != TicketStatePending {
			return ErrTicketStateConflict
		}
		return nil
	case TicketCommandTransfer:
		if t.Terminal() {
			return ErrTicketTerminal
		}
		return nil
	case TicketCommandEscalate:
		if t.Terminal() {
			return ErrTicketTerminal
		}
		return nil
	case TicketCommandAppeal:
		// 申诉只对已完结的工单开启新一轮，且不能由上一轮处理人接手。
		if t.State != TicketStateResolved {
			return ErrTicketStateConflict
		}
		if strings.TrimSpace(actor) != "" && actor == t.LastHandler() {
			return ErrAppealSameHandler
		}
		return nil
	case TicketCommandRevoke:
		// 误报只针对危险驾驶，核实后关单并恢复司机状态。
		if t.Type != TicketTypeDangerous {
			return ErrRevokeNotDangerous
		}
		if t.State != TicketStatePending && t.State != TicketStateProcessing {
			return ErrTicketStateConflict
		}
		return nil
	case TicketCommandMerge:
		if t.State == TicketStateClosed {
			return ErrTicketTerminal
		}
		return nil
	default:
		return ErrInvalidTicketCommand
	}
}

// CommandTargetState returns the state the command produces. Commands that only
// annotate the ticket keep the current state.
func (t Ticket) CommandTargetState(command string) string {
	switch command {
	case TicketCommandClaim:
		return TicketStateProcessing
	case TicketCommandWaitUser:
		return TicketStateWaitingUser
	case TicketCommandResume:
		return TicketStateProcessing
	case TicketCommandResolve:
		return TicketStateResolved
	case TicketCommandClose:
		return TicketStateClosed
	case TicketCommandRevoke:
		return TicketStateClosed
	case TicketCommandAppeal:
		return TicketStateProcessing
	default:
		return t.State
	}
}

// requireAssignee keeps a claimed ticket in its owner's hands: operators may
// not reply to, pause or resolve somebody else's ticket.
func requireAssignee(t Ticket, actor string) error {
	if strings.TrimSpace(t.Assignee) == "" {
		return ErrTicketNotAssignee
	}
	if strings.TrimSpace(actor) == "" || actor != t.Assignee {
		return ErrTicketNotAssignee
	}
	return nil
}

// MergeWindow reports whether candidate is a duplicate of t: same order, same
// complaint type, created inside the configured window.
func (t Ticket) IsDuplicate(candidate Ticket, window time.Duration) bool {
	if candidate.TicketNo == t.TicketNo {
		return false
	}
	if candidate.OrderID != t.OrderID || candidate.Type != t.Type {
		return false
	}
	if candidate.State == TicketStateClosed || strings.TrimSpace(candidate.MergedInto) != "" {
		return false
	}
	delta := candidate.CreatedAt.Sub(t.CreatedAt)
	if delta < 0 {
		delta = -delta
	}
	return delta <= window
}
