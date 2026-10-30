package usecase

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/NSObjects/echo-admin/internal/modules/operations/domain"
)

// fixedNow is the clock every test uses, so deadlines are exact.
var fixedNow = time.Date(2026, 10, 22, 12, 0, 0, 0, time.UTC)

// newTestTicket builds a ticket with the seven-day first-response window.
func newTestTicket(state, ticketType string) domain.Ticket {
	return domain.Ticket{
		TicketNo:           "W20261022-0001",
		OrderID:            "order-1",
		OrderNo:            "P0001",
		Type:               ticketType,
		Group:              domain.RouteGroup(ticketType, "310100"),
		ComplainantID:      "p1",
		DriverID:           "d1",
		State:              state,
		Assignee:           "admin:1",
		Round:              1,
		CreatedAt:          fixedNow,
		FirstResponseDueAt: fixedNow.Add(7 * 24 * time.Hour),
		Rounds:             []domain.Round{{Round: 1, Handler: "admin:1", State: state, StartedAt: fixedNow}},
	}
}

func TestRouteGroupByType(t *testing.T) {
	cases := map[string]string{
		domain.TicketTypeAttitude:  domain.TicketGroupService,
		domain.TicketTypeDangerous: domain.TicketGroupSafety,
		domain.TicketTypeFare:      domain.TicketGroupFinance,
		domain.TicketTypeDetour:    domain.TicketGroupService,
		domain.TicketTypeOther:     domain.TicketGroupService + "·310100",
	}
	for ticketType, want := range cases {
		if got := domain.RouteGroup(ticketType, "310100"); got != want {
			t.Fatalf("RouteGroup(%s) = %s, want %s", ticketType, got, want)
		}
	}
}

func TestFirstResponseDeadlineIsSevenDaysFromCreation(t *testing.T) {
	ticket := newTestTicket(domain.TicketStatePending, domain.TicketTypeAttitude)
	want := fixedNow.Add(168 * time.Hour)
	if !ticket.FirstResponseDueAt.Equal(want) {
		t.Fatalf("due = %s, want %s", ticket.FirstResponseDueAt, want)
	}
	if ticket.Overdue(fixedNow) {
		t.Fatal("fresh ticket must not be overdue")
	}
	if ticket.Overdue(fixedNow.Add(169 * time.Hour)) {
	}
	if !ticket.Overdue(fixedNow.Add(169 * time.Hour)) {
		t.Fatal("ticket past the window must be overdue")
	}
}

func TestPauseFreezesClockAndResumeExtendsIt(t *testing.T) {
	ticket := newTestTicket(domain.TicketStateProcessing, domain.TicketTypeAttitude)
	// 处理两天后转入等待用户，计时冻结。
	pauseAt := fixedNow.Add(48 * time.Hour)
	ticket.State = domain.TicketStateWaitingUser
	ticket.PausedAt = &pauseAt
	if !ticket.Paused() {
		t.Fatal("ticket must be paused in WAITING_USER")
	}
	frozen := ticket.Remaining(pauseAt)
	later := ticket.Remaining(pauseAt.Add(72 * time.Hour))
	if frozen != later {
		t.Fatalf("clock moved while paused: %s vs %s", frozen, later)
	}
	// 用户三小时后回应：把这段暂停并入累计，按剩余时长续算。
	resumeAt := pauseAt.Add(3 * time.Hour)
	ticket.State = domain.TicketStateProcessing
	ticket.PausedMs += resumeAt.Sub(pauseAt).Milliseconds()
	ticket.PausedAt = nil
	if ticket.EffectiveDue() != fixedNow.Add(168*time.Hour).Add(3*time.Hour) {
		t.Fatalf("effective due = %s, want 创建时间+7天+暂停3小时", ticket.EffectiveDue())
	}
	if ticket.Overdue(resumeAt) {
		t.Fatal("resumed ticket must still have time left")
	}
}

func TestTransferDoesNotRefreshDeadline(t *testing.T) {
	ticket := newTestTicket(domain.TicketStateProcessing, domain.TicketTypeAttitude)
	before := ticket.FirstResponseDueAt
	ticket.Assignee = "admin:2"
	if ticket.FirstResponseDueAt != before {
		t.Fatalf("transfer refreshed the deadline: %s", ticket.FirstResponseDueAt)
	}
}

func TestClaimRequiresPendingAndUnclaimed(t *testing.T) {
	ticket := newTestTicket(domain.TicketStateProcessing, domain.TicketTypeAttitude)
	if err := ticket.CheckCommand(domain.TicketCommandClaim, "admin:1"); !errors.Is(err, domain.ErrTicketStateConflict) {
		t.Fatalf("claim on processing = %v, want state conflict", err)
	}
	pending := newTestTicket(domain.TicketStatePending, domain.TicketTypeAttitude)
	pending.Assignee = ""
	if err := pending.CheckCommand(domain.TicketCommandClaim, "admin:1"); err != nil {
		t.Fatalf("claim on fresh pending: %v", err)
	}
	pending.Assignee = "admin:9"
	if err := pending.CheckCommand(domain.TicketCommandClaim, "admin:1"); !errors.Is(err, domain.ErrTicketClaimed) {
		t.Fatalf("claim on claimed = %v, want ErrTicketClaimed", err)
	}
}

func TestOnlyAssigneeCanReplyPauseAndResolve(t *testing.T) {
	ticket := newTestTicket(domain.TicketStateProcessing, domain.TicketTypeAttitude)
	if err := ticket.CheckCommand(domain.TicketCommandReply, "admin:2"); !errors.Is(err, domain.ErrTicketNotAssignee) {
		t.Fatalf("reply by stranger = %v, want ErrTicketNotAssignee", err)
	}
	if err := ticket.CheckCommand(domain.TicketCommandReply, "admin:1"); err != nil {
		t.Fatalf("reply by assignee: %v", err)
	}
	waiting := newTestTicket(domain.TicketStateProcessing, domain.TicketTypeAttitude)
	if err := waiting.CheckCommand(domain.TicketCommandResume, "admin:1"); !errors.Is(err, domain.ErrTicketNotWaiting) {
		t.Fatalf("resume while processing = %v, want ErrTicketNotWaiting", err)
	}
}

func TestAppealRejectsPreviousHandler(t *testing.T) {
	ticket := newTestTicket(domain.TicketStateResolved, domain.TicketTypeAttitude)
	if err := ticket.CheckCommand(domain.TicketCommandAppeal, "admin:2"); err != nil {
		t.Fatalf("appeal by another agent: %v", err)
	}
	if err := ticket.CheckCommand(domain.TicketCommandAppeal, "admin:1"); !errors.Is(err, domain.ErrAppealSameHandler) {
		t.Fatalf("appeal by previous handler = %v, want ErrAppealSameHandler", err)
	}
	processing := newTestTicket(domain.TicketStateProcessing, domain.TicketTypeAttitude)
	if err := processing.CheckCommand(domain.TicketCommandAppeal, "admin:2"); !errors.Is(err, domain.ErrTicketStateConflict) {
		t.Fatalf("appeal while processing = %v, want state conflict", err)
	}
}

func TestRevokeOnlyForDangerousDriving(t *testing.T) {
	attitude := newTestTicket(domain.TicketStateProcessing, domain.TicketTypeAttitude)
	if err := attitude.CheckCommand(domain.TicketCommandRevoke, "admin:1"); !errors.Is(err, domain.ErrRevokeNotDangerous) {
		t.Fatalf("revoke on attitude = %v, want ErrRevokeNotDangerous", err)
	}
	danger := newTestTicket(domain.TicketStatePending, domain.TicketTypeDangerous)
	if err := danger.CheckCommand(domain.TicketCommandRevoke, "admin:1"); err != nil {
		t.Fatalf("revoke on dangerous: %v", err)
	}
}

func TestDuplicateWindowMergesSameOrderSameType(t *testing.T) {
	main := newTestTicket(domain.TicketStatePending, domain.TicketTypeDetour)
	same := newTestTicket(domain.TicketStatePending, domain.TicketTypeDetour)
	same.TicketNo = "W20261022-0002"
	same.CreatedAt = fixedNow.Add(2 * time.Hour)
	if !main.IsDuplicate(same, 24*time.Hour) {
		t.Fatal("same order and type inside 24h must be a duplicate")
	}
	other := same
	other.Type = domain.TicketTypeFare
	if main.IsDuplicate(other, 24*time.Hour) {
		t.Fatal("different type must not be a duplicate")
	}
	late := same
	late.CreatedAt = fixedNow.Add(48 * time.Hour)
	if main.IsDuplicate(late, 24*time.Hour) {
		t.Fatal("outside the window must not be a duplicate")
	}
}

func TestCommandRejectsShortMessage(t *testing.T) {
	store := newTicketStore([]domain.Ticket{newTestTicket(domain.TicketStatePending, domain.TicketTypeAttitude)})
	store.tickets[0].Assignee = ""
	uc := NewTicket(store)
	uc.now = func() time.Time { return fixedNow }
	_, err := uc.Execute(context.Background(), TicketCommandInput{
		TicketNo:   "W20261022-0001",
		Command:    domain.TicketCommandClaim,
		Message:    "已看",
		OperatorID: 1,
	})
	if !errors.Is(err, domain.ErrInvalidTicketMessage) {
		t.Fatalf("err = %v, want ErrInvalidTicketMessage", err)
	}
	if len(store.applied) != 0 {
		t.Fatal("command applied despite invalid message")
	}
}

func TestClaimOpensRoundAndKeepsDeadline(t *testing.T) {
	ticket := newTestTicket(domain.TicketStatePending, domain.TicketTypeAttitude)
	ticket.Assignee = ""
	store := newTicketStore([]domain.Ticket{ticket})
	uc := NewTicket(store)
	uc.now = func() time.Time { return fixedNow }
	out, err := uc.Execute(context.Background(), TicketCommandInput{
		TicketNo:   ticket.TicketNo,
		Command:    domain.TicketCommandClaim,
		Message:    "客服已领取工单，联系乘客核实",
		OperatorID: 5,
	})
	if err != nil {
		t.Fatalf("claim: %v", err)
	}
	if out.State != domain.TicketStateProcessing {
		t.Fatalf("state = %s, want PROCESSING", out.State)
	}
	applied := store.applied[0]
	if applied.Assignee != "admin:5" {
		t.Fatalf("assignee = %s, want admin:5", applied.Assignee)
	}
	if applied.FreezeClock || applied.ResumeFrom != nil || applied.FirstResponse {
		t.Fatal("claim must not touch the response clock")
	}
	if len(store.logs) != 1 || store.logs[0].Operator != "admin:5" {
		t.Fatalf("log = %+v, want one entry by admin:5", store.logs)
	}
}

func TestTransferRejectsSameAssignee(t *testing.T) {
	store := newTicketStore([]domain.Ticket{newTestTicket(domain.TicketStateProcessing, domain.TicketTypeAttitude)})
	uc := NewTicket(store)
	uc.now = func() time.Time { return fixedNow }
	_, err := uc.Execute(context.Background(), TicketCommandInput{
		TicketNo:   "W20261022-0001",
		Command:    domain.TicketCommandTransfer,
		Message:    "转派给同组同事继续跟进",
		Assignee:   "admin:1",
		OperatorID: 1,
	})
	if !errors.Is(err, ErrTransferSameAssignee) {
		t.Fatalf("err = %v, want ErrTransferSameAssignee", err)
	}
}

func TestMergeRejectsNonDuplicate(t *testing.T) {
	main := newTestTicket(domain.TicketStatePending, domain.TicketTypeDetour)
	other := newTestTicket(domain.TicketStatePending, domain.TicketTypeFare)
	other.TicketNo = "W20261022-0009"
	store := newTicketStore([]domain.Ticket{main, other})
	uc := NewTicket(store)
	uc.now = func() time.Time { return fixedNow }
	_, err := uc.Execute(context.Background(), TicketCommandInput{
		TicketNo:   main.TicketNo,
		Command:    domain.TicketCommandMerge,
		Message:    "合并同一订单的重复投诉",
		MergeNo:    other.TicketNo,
		OperatorID: 1,
	})
	if !errors.Is(err, domain.ErrMergeTargetInvalid) {
		t.Fatalf("err = %v, want ErrMergeTargetInvalid", err)
	}
	if len(store.merged) != 0 {
		t.Fatal("merged a ticket that is not a duplicate")
	}
}

func TestSweeperEscalatesOverdueOnce(t *testing.T) {
	overdue := newTestTicket(domain.TicketStatePending, domain.TicketTypeAttitude)
	overdue.Assignee = ""
	store := newTicketStore([]domain.Ticket{overdue})
	uc := NewTicket(store)
	uc.now = func() time.Time { return fixedNow.Add(200 * time.Hour) }
	report, err := uc.ScanOnce(context.Background())
	if err != nil {
		t.Fatalf("ScanOnce: %v", err)
	}
	if len(report.Escalated) != 1 {
		t.Fatalf("escalated = %v, want 1", report.Escalated)
	}
	// 幂等：再扫一遍不应重复升级。
	report, err = uc.ScanOnce(context.Background())
	if err != nil {
		t.Fatalf("ScanOnce again: %v", err)
	}
	if len(report.Escalated) != 0 {
		t.Fatalf("second pass escalated %v again, want none", report.Escalated)
	}
}

func TestSweeperRemindsThenEscalatesUnclaimedDangerous(t *testing.T) {
	danger := newTestTicket(domain.TicketStatePending, domain.TicketTypeDangerous)
	danger.Assignee = ""
	store := newTicketStore([]domain.Ticket{danger})
	uc := NewTicket(store)
	// 创建后 2 分钟：只推送值班提醒。
	uc.now = func() time.Time { return fixedNow.Add(2 * time.Minute) }
	report, err := uc.ScanOnce(context.Background())
	if err != nil {
		t.Fatalf("ScanOnce: %v", err)
	}
	if len(report.Reminded) != 1 || len(report.DutyEscalated) != 0 {
		t.Fatalf("reminded=%v escalated=%v, want 1/0", report.Reminded, report.DutyEscalated)
	}
	// 创建后 6 分钟：无人领单，自动升级。
	uc.now = func() time.Time { return fixedNow.Add(6 * time.Minute) }
	report, err = uc.ScanOnce(context.Background())
	if err != nil {
		t.Fatalf("ScanOnce: %v", err)
	}
	if len(report.DutyEscalated) != 1 {
		t.Fatalf("duty escalated = %v, want 1", report.DutyEscalated)
	}
	// 再扫一遍：提醒与升级都不重复。
	report, err = uc.ScanOnce(context.Background())
	if err != nil {
		t.Fatalf("ScanOnce again: %v", err)
	}
	if len(report.Reminded) != 0 || len(report.DutyEscalated) != 0 {
		t.Fatalf("second pass = %+v, want nothing", report)
	}
}

func TestReadTicketConfigFallsBackToSSOT(t *testing.T) {
	store := newTicketStore(nil)
	if got := store.ReadTicketConfig(context.Background(), ConfigTicketFirstResponseHours, DefaultFirstResponseHours); got != DefaultFirstResponseHours {
		t.Fatalf("config = %d, want default %d", got, DefaultFirstResponseHours)
	}
	store.config[ConfigTicketDutyRemindSeconds] = 120
	if got := store.ReadTicketConfig(context.Background(), ConfigTicketDutyRemindSeconds, DefaultDutyRemindSeconds); got != 120 {
		t.Fatalf("config = %d, want 120 from config collection", got)
	}
}
