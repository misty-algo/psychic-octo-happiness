// A04 工单时限配置键与超时扫描器。时限一律读配置集合，代码里只放 SSOT 默认值。
package usecase

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"sync/atomic"
	"time"

	"github.com/NSObjects/echo-admin/internal/modules/operations/domain"
	"github.com/NSObjects/echo-admin/internal/platform/apperr"
)

// 配置键。运营侧与乘客侧共用同一份配置，禁止任何模块各自写死时限。
const (
	// ConfigTicketFirstResponseHours 首次回复时限（小时），从工单创建时间起算。
	ConfigTicketFirstResponseHours = "ticket.first_response.timeout_hours"
	// ConfigTicketUrgentResponseHours 升级为紧急后的更短响应时限（小时）。
	ConfigTicketUrgentResponseHours = "ticket.urgent.response_hours"
	// ConfigTicketDutyRemindSeconds 危险驾驶工单无人领单后推送值班队列的秒数。
	ConfigTicketDutyRemindSeconds = "ticket.dangerous.remind_seconds"
	// ConfigTicketDutyEscalateSeconds 危险驾驶工单无人领单后自动升级的秒数。
	ConfigTicketDutyEscalateSeconds = "ticket.dangerous.escalate_seconds"
	// ConfigTicketMergeWindowHours 重复投诉合并窗口（小时）。
	ConfigTicketMergeWindowHours = "ticket.merge.window_hours"
)

// SSOT 默认值：首次回复七天、紧急响应一天、危险驾驶一分钟提醒五分钟升级、
// 重复投诉二十四小时合并。
const (
	DefaultFirstResponseHours  = 168
	DefaultUrgentResponseHours = 24
	DefaultDutyRemindSeconds   = 60
	DefaultDutyEscalateSeconds = 300
	DefaultMergeWindowHours    = 24
)

// SweepBatchSize caps one sweeper pass so a backlog cannot stall the loop.
const SweepBatchSize = 200

// SweepInterval is how often the sweeper looks for expired deadlines.
const SweepInterval = 30 * time.Second

// 转派与合并命令的补充校验错误。
var (
	ErrTransferAssigneeRequired = errors.New("transfer requires a target assignee")
	ErrTransferSameAssignee     = errors.New("transfer target is already the assignee")
	ErrMergeTargetRequired      = errors.New("merge requires a duplicate ticket no")
)

// mapTicketError translates ticket rule failures into HTTP failures.
func mapTicketError(err error) error {
	switch {
	case err == nil:
		return nil
	case errors.Is(err, domain.ErrTicketNotFound):
		return apperr.WrapNotFound(err, "工单不存在")
	case errors.Is(err, domain.ErrInvalidTicketNo):
		return apperr.WrapBadRequest(err, "工单号不合法")
	case errors.Is(err, domain.ErrInvalidTicketCommand):
		return apperr.WrapBadRequest(err, "command 只能是 claim/reply/wait_user/resume/resolve/close/transfer/escalate/appeal/revoke/merge")
	case errors.Is(err, domain.ErrInvalidTicketMessage):
		return apperr.WrapBadRequest(err, fmt.Sprintf("命令备注必须填写 %d-%d 字", domain.BasisMinLen, domain.BasisMaxLen))
	case errors.Is(err, domain.ErrTicketStateConflict):
		return apperr.WrapConflict(err, "工单当前状态不允许执行该命令")
	case errors.Is(err, domain.ErrTicketClaimed):
		return apperr.WrapConflict(err, "工单已被领取")
	case errors.Is(err, domain.ErrTicketNotAssignee):
		return apperr.WrapConflict(err, "只有工单当前处理人可执行该命令")
	case errors.Is(err, domain.ErrTicketNotWaiting):
		return apperr.WrapConflict(err, "只有等待用户中的工单可以恢复计时")
	case errors.Is(err, domain.ErrAppealSameHandler):
		return apperr.WrapConflict(err, "申诉必须由另一名客服处理，不能是上一轮处理人")
	case errors.Is(err, domain.ErrRevokeNotDangerous):
		return apperr.WrapConflict(err, "只有危险驾驶类工单可以按误报撤销")
	case errors.Is(err, domain.ErrMergeTargetInvalid):
		return apperr.WrapConflict(err, "待合并工单不是同订单同类型的重复投诉")
	case errors.Is(err, domain.ErrTicketTerminal):
		return apperr.WrapConflict(err, "工单已终结，不再接受该命令")
	case errors.Is(err, ErrTransferAssigneeRequired):
		return apperr.WrapBadRequest(err, "转派必须指定新的处理人")
	case errors.Is(err, ErrTransferSameAssignee):
		return apperr.WrapConflict(err, "转派目标已是当前处理人")
	case errors.Is(err, ErrMergeTargetRequired):
		return apperr.WrapBadRequest(err, "合并必须指定待合并的工单号")
	default:
		return err
	}
}

// TicketSweeper periodically looks for expired ticket deadlines. Upgrades are
// idempotent per ticket, so a crashed or duplicated pass cannot escalate twice.
type TicketSweeper struct {
	usecase  *TicketUsecase
	interval time.Duration
	logger   *slog.Logger
	stop     chan struct{}
	done     chan struct{}
	stopOnce sync.Once
	started  atomic.Bool
}

// NewTicketSweeper creates a stopped sweeper; Start launches it.
func NewTicketSweeper(uc *TicketUsecase, logger *slog.Logger) *TicketSweeper {
	if logger == nil {
		logger = slog.Default()
	}
	return &TicketSweeper{
		usecase:  uc,
		interval: SweepInterval,
		logger:   logger,
		stop:     make(chan struct{}),
		done:     make(chan struct{}),
	}
}

// Start launches the scan loop. It returns immediately; Shutdown stops it.
func (s *TicketSweeper) Start() {
	if s.started.Swap(true) {
		return
	}
	go s.loop()
}

// Shutdown stops the scan loop and waits for the in-flight pass to finish.
// It satisfies the DI container's Shutdowner contract and is safe to call
// when the loop never started: waiting on a channel nobody closes would hang
// the whole shutdown sequence.
func (s *TicketSweeper) Shutdown() {
	s.stopOnce.Do(func() { close(s.stop) })
	if !s.started.Load() {
		return
	}
	<-s.done
}

// loop runs one pass immediately, then once per interval.
func (s *TicketSweeper) loop() {
	defer close(s.done)
	ticker := time.NewTicker(s.interval)
	defer ticker.Stop()
	s.pass()
	for {
		select {
		case <-s.stop:
			return
		case <-ticker.C:
			s.pass()
		}
	}
}

// pass runs one scan and logs what it changed.
func (s *TicketSweeper) pass() {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	report, err := s.usecase.ScanOnce(ctx)
	if err != nil {
		s.logger.Error("ticket sweeper pass failed", "error", err)
		return
	}
	if len(report.Escalated) == 0 && len(report.Reminded) == 0 && len(report.DutyEscalated) == 0 {
		return
	}
	s.logger.Info("ticket sweeper pass",
		"escalated", len(report.Escalated),
		"duty_reminded", len(report.Reminded),
		"duty_escalated", len(report.DutyEscalated))
}
