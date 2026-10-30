package usecase

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/NSObjects/echo-admin/internal/modules/operations/domain"
	"github.com/NSObjects/echo-admin/internal/platform/apperr"
)

// OrderStore persists order reads and commands. orders / dispatch_tasks /
// tickets are owned by the passenger and driver sides, so the operations
// module only reads them and writes a dedicated opsCommands trail plus its own
// operation_logs rows — never business fields other commands do not own.
type OrderStore interface {
	// ListOrders returns one page of orders plus the total count.
	ListOrders(ctx context.Context, filter OrderFilter) ([]domain.Order, int64, error)
	// FindOrder returns one order or domain.ErrOrderNotFound.
	FindOrder(ctx context.Context, orderNo string) (domain.Order, error)
	// PassengerBriefs returns masked passenger identity for the given ids.
	PassengerBriefs(ctx context.Context, ids []string) (map[string]PassengerBrief, error)
	// DriverBriefs returns driver name/plate for the given driver ids.
	DriverBriefs(ctx context.Context, ids []string) (map[string]DriverBrief, error)
	// ListDispatchTasks returns dispatch records keyed by order id.
	ListDispatchTasks(ctx context.Context, orderIDs []string) (map[string][]domain.DispatchTask, error)
	// ListDispatchAttempts returns driver responses keyed by order id.
	ListDispatchAttempts(ctx context.Context, orderIDs []string) (map[string][]domain.DispatchAttempt, error)
	// ListOrderTrajectories returns raw tracking points keyed by order id.
	ListOrderTrajectories(ctx context.Context, orderIDs []string) (map[string]domain.Trajectory, error)
	// ListTickets returns linked work tickets keyed by order id.
	ListOrderTickets(ctx context.Context, orderIDs []string) (map[string][]domain.TicketBrief, error)
	// FindPayments returns the payment row of each order, keyed by order id.
	FindPayments(ctx context.Context, orders []domain.Order) (map[string]domain.Payment, error)
	// ExecuteCommand applies one command under a state compare-and-set.
	ExecuteCommand(ctx context.Context, command OrderCommand) (domain.Order, error)
	// RecordOperationLog appends one business-side audit record.
	RecordOperationLog(ctx context.Context, log domain.OperationLog) error
}

// PassengerBrief is the masked passenger identity shown in the workbench.
type PassengerBrief struct {
	Name  string
	Phone string
}

// OrderFilter narrows the order workbench.
type OrderFilter struct {
	Keyword   string // order no or passenger phone/name fragment
	State     string // "", PENDING/...
	BizType   string // 业务类型（快车/拼车/特价…）
	CityCode  string // 城市
	RiskLevel string // 风险等级
	From      time.Time
	To        time.Time
	Skip      int
	Limit     int
}

// ListOrdersInput carries the workbench query.
type ListOrdersInput struct {
	Keyword   string
	State     string
	BizType   string
	CityCode  string
	RiskLevel string
	From      string
	To        string
	Page      int
	PageSize  int
}

// OrderRow is one workbench row enriched with passenger and driver identity.
type OrderRow struct {
	ID             string    `json:"id"`
	OrderNo        string    `json:"order_no"`
	BizType        string    `json:"biz_type"`
	CityCode       string    `json:"city_code"`
	PassengerID    string    `json:"passenger_id"`
	PassengerName  string    `json:"passenger_name"`
	PassengerPhone string    `json:"passenger_phone"`
	DriverID       string    `json:"driver_id"`
	DriverName     string    `json:"driver_name"`
	Plate          string    `json:"plate"`
	State          string    `json:"state"`
	PayStatus      string    `json:"pay_status"`
	Amount         int64     `json:"amount"`
	CancelFee      int64     `json:"cancel_fee"`
	StartAddress   string    `json:"start_address"`
	EndAddress     string    `json:"end_address"`
	RiskLevel      string    `json:"risk_level"`
	CreatedAt      time.Time `json:"created_at"`
}

// ListOrdersOutput is one page of order rows.
type ListOrdersOutput struct {
	Items []OrderRow `json:"items"`
	Total int64      `json:"total"`
}

// CommandInput carries one order command request.
type CommandInput struct {
	OrderNo    string
	Command    string
	Basis      string
	OperatorID int64
	RequestID  string
}

// CommandOutput echoes what the command did, matching the documented response
// shape { order_id, command, state }.
type CommandOutput struct {
	OrderID string `json:"order_id"`
	OrderNo string `json:"order_no"`
	Command string `json:"command"`
	State   string `json:"state"`
	Basis   string `json:"basis"`
}

// OrderCommand is the compare-and-set payload handed to the store. Expected
// state is part of the filter, so a concurrent transition loses the race
// instead of overwriting the winner.
type OrderCommand struct {
	OrderNo       string
	Command       string
	Basis         string
	Operator      string
	ExpectedState string
	TargetState   string
	At            time.Time
}

// OrderUsecase implements A03 order management.
type OrderUsecase struct {
	store OrderStore
	now   func() time.Time
}

// NewOrder creates an order usecase.
func NewOrder(store OrderStore) *OrderUsecase {
	return &OrderUsecase{store: store, now: time.Now}
}

// ListOrders returns one page of the order workbench with identity enrichment.
func (u *OrderUsecase) ListOrders(ctx context.Context, in ListOrdersInput) (ListOrdersOutput, error) {
	filter := OrderFilter{Keyword: strings.TrimSpace(in.Keyword), State: strings.TrimSpace(in.State)}
	// 时间边界解析失败直接报错，不静默放过：静默会让操作员以为筛过了，
	// 实际返回的是全量数据。
	if strings.TrimSpace(in.From) != "" {
		from, err := parseTime(in.From)
		if err != nil {
			return ListOrdersOutput{}, apperr.WrapBadRequest(err, "创建时间起点格式不正确")
		}
		filter.From = from
	}
	if strings.TrimSpace(in.To) != "" {
		// 上界若只选了日期（无时分秒），按当天 23:59:59 处理，避免漏掉当天订单。
		if end, ok := endOfDay(in.To); ok {
			filter.To = end
		} else {
			to, err := parseTime(in.To)
			if err != nil {
				return ListOrdersOutput{}, apperr.WrapBadRequest(err, "创建时间终点格式不正确")
			}
			filter.To = to
		}
	}
	filter.Skip = (in.Page - 1) * in.PageSize
	if filter.Skip < 0 {
		filter.Skip = 0
	}
	filter.Limit = in.PageSize
	if filter.Limit <= 0 || filter.Limit > maxPageSize {
		filter.Limit = maxPageSize
	}

	orders, total, err := u.store.ListOrders(ctx, filter)
	if err != nil {
		return ListOrdersOutput{}, err
	}
	rows, err := u.enrich(ctx, orders)
	if err != nil {
		return ListOrdersOutput{}, err
	}
	return ListOrdersOutput{Items: rows, Total: total}, nil
}

// Detail returns the penetration view of one order.
func (u *OrderUsecase) Detail(ctx context.Context, orderNo string) (domain.OrderDetail, error) {
	if err := domain.ValidateOrderNo(orderNo); err != nil {
		return domain.OrderDetail{}, apperr.WrapBadRequest(err, "订单号不合法")
	}
	order, err := u.store.FindOrder(ctx, orderNo)
	if err != nil {
		return domain.OrderDetail{}, mapOrderError(err)
	}
	rows, err := u.enrich(ctx, []domain.Order{order})
	if err != nil {
		return domain.OrderDetail{}, err
	}
	enriched := rows[0]
	order.PassengerName = enriched.PassengerName
	order.PassengerPhone = enriched.PassengerPhone
	order.DriverName = enriched.DriverName
	order.Plate = enriched.Plate

	ids := []string{order.ID}
	detail := domain.OrderDetail{
		Order:      order,
		Tasks:      []domain.DispatchTask{},
		Attempts:   []domain.DispatchAttempt{},
		Trajectory: domain.Trajectory{Points: []map[string]any{}},
		Tickets:    []domain.TicketBrief{},
	}
	if tasks, err := u.store.ListDispatchTasks(ctx, ids); err == nil {
		detail.Tasks = tasks[order.ID]
	} else {
		return domain.OrderDetail{}, fmt.Errorf("list dispatch tasks: %w", err)
	}
	if attempts, err := u.store.ListDispatchAttempts(ctx, ids); err == nil {
		detail.Attempts = attempts[order.ID]
	} else {
		return domain.OrderDetail{}, fmt.Errorf("list dispatch attempts: %w", err)
	}
	if tracks, err := u.store.ListOrderTrajectories(ctx, ids); err == nil {
		if track, ok := tracks[order.ID]; ok {
			detail.Trajectory = track
		}
	} else {
		return domain.OrderDetail{}, fmt.Errorf("list order trajectory: %w", err)
	}
	if tickets, err := u.store.ListOrderTickets(ctx, ids); err == nil {
		detail.Tickets = tickets[order.ID]
	} else {
		return domain.OrderDetail{}, fmt.Errorf("list order tickets: %w", err)
	}
	payments, err := u.store.FindPayments(ctx, []domain.Order{order})
	if err != nil {
		return domain.OrderDetail{}, fmt.Errorf("find order payment: %w", err)
	}
	if payment, ok := payments[order.ID]; ok {
		found := payment
		detail.Payment = &found
	}
	// 计价明细必须来自订单自身的计价快照，不能拿总价冒充预估价——复核
	// 「异常计费」时运营要看到规则号、溢价系数与预估/实收的差额。
	detail.Pricing = domain.Pricing{
		Version:     order.PricingVersion,
		RuleID:      order.PricingRuleID,
		SnapshotID:  order.SnapshotID,
		SurgeRatio:  order.SurgeRatio,
		EstimateFee: order.EstimateFee,
		TotalFee:    order.Amount,
		Reviewed:    order.Reviewed,
		ReviewedBy:  order.ReviewedBy,
	}
	return detail, nil
}

// Execute validates the command against the order state and applies it.
func (u *OrderUsecase) Execute(ctx context.Context, in CommandInput) (CommandOutput, error) {
	if err := domain.ValidateOrderNo(in.OrderNo); err != nil {
		return CommandOutput{}, apperr.WrapBadRequest(err, "订单号不合法")
	}
	command, err := domain.NewOrderCommand(in.Command, in.Basis)
	if err != nil {
		return CommandOutput{}, mapOrderError(err)
	}
	order, err := u.store.FindOrder(ctx, in.OrderNo)
	if err != nil {
		return CommandOutput{}, mapOrderError(err)
	}
	if preErr := order.CheckCommand(command); preErr != nil {
		return CommandOutput{}, mapOrderError(preErr)
	}
	operator := operatorName(in.OperatorID)
	applied, err := u.store.ExecuteCommand(ctx, OrderCommand{
		OrderNo:       order.OrderNo,
		Command:       command,
		Basis:         strings.TrimSpace(in.Basis),
		Operator:      operator,
		ExpectedState: order.State,
		TargetState:   order.CommandTargetState(command),
		At:            u.now(),
	})
	if err != nil {
		return CommandOutput{}, mapOrderError(err)
	}
	if logErr := u.store.RecordOperationLog(ctx, domain.OperationLog{
		ID:        fmt.Sprintf("%s:%s:%d", order.OrderNo, command, u.now().UnixNano()),
		Module:    "operations",
		Operator:  operator,
		Action:    "order_command",
		Resource:  "ops_order",
		Target:    order.OrderNo,
		RequestID: in.RequestID,
		Detail:    fmt.Sprintf("%s %s→%s 依据：%s", command, order.State, applied.State, strings.TrimSpace(in.Basis)),
		At:        u.now(),
	}); logErr != nil {
		return CommandOutput{}, fmt.Errorf("record order command log: %w", logErr)
	}
	return CommandOutput{
		OrderID: order.ID,
		OrderNo: order.OrderNo,
		Command: command,
		State:   applied.State,
		Basis:   strings.TrimSpace(in.Basis),
	}, nil
}

// enrich attaches passenger and driver identity to order rows.
func (u *OrderUsecase) enrich(ctx context.Context, orders []domain.Order) ([]OrderRow, error) {
	passengerIDs := make([]string, 0, len(orders))
	driverIDs := make([]string, 0, len(orders))
	for _, order := range orders {
		if order.PassengerID != "" {
			passengerIDs = append(passengerIDs, order.PassengerID)
		}
		if order.DriverID != "" {
			driverIDs = append(driverIDs, order.DriverID)
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
	rows := make([]OrderRow, 0, len(orders))
	for _, order := range orders {
		row := OrderRow{
			ID:           order.ID,
			OrderNo:      order.OrderNo,
			BizType:      order.BizType,
			CityCode:     order.CityCode,
			PassengerID:  order.PassengerID,
			DriverID:     order.DriverID,
			// 乘客域订单落了司机姓名/车牌快照，先带上；有司机档案时再被覆盖。
			DriverName:   order.DriverName,
			Plate:        order.Plate,
			State:        order.State,
			PayStatus:    order.PayStatus,
			Amount:       order.Amount,
			CancelFee:    order.CancelFee,
			StartAddress: order.StartAddress,
			EndAddress:   order.EndAddress,
			RiskLevel:    order.RiskLevel,
			CreatedAt:    order.CreatedAt,
		}
		if brief, ok := passengers[order.PassengerID]; ok {
			row.PassengerName = brief.Name
			row.PassengerPhone = brief.Phone
		}
		if brief, ok := drivers[order.DriverID]; ok {
			row.DriverName = brief.Name
			row.Plate = brief.Plate
		}
		rows = append(rows, row)
	}
	return rows, nil
}

// parseTime accepts the shapes operators actually send: RFC3339, a plain
// calendar day, or the "2006-01-02 15:04:05" layout the console date picker
// emits. Empty input means "no bound"; unsupported formats fail loudly so the
// caller rejects the query instead of silently returning unfiltered data.
func parseTime(value string) (time.Time, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return time.Time{}, fmt.Errorf("empty time")
	}
	if parsed, err := time.Parse(time.RFC3339, value); err == nil {
		return parsed, nil
	}
	if parsed, err := time.ParseInLocation(time.DateTime, value, time.Local); err == nil {
		return parsed, nil
	}
	parsed, err := time.ParseInLocation(time.DateOnly, value, time.Local)
	if err != nil {
		return time.Time{}, fmt.Errorf("parse time %q: %w", value, err)
	}
	return parsed, nil
}

// endOfDay widens a date-only upper bound to 23:59:59 so selecting a single
// day covers the whole day instead of just midnight.
func endOfDay(value string) (time.Time, bool) {
	if _, err := time.ParseInLocation(time.DateOnly, value, time.Local); err != nil {
		return time.Time{}, false
	}
	parsed, err := time.ParseInLocation(time.DateTime, value+" 23:59:59", time.Local)
	if err != nil {
		return time.Time{}, false
	}
	return parsed, true
}

// mapOrderError translates order rule failures into HTTP failures.
func mapOrderError(err error) error {
	switch {
	case err == nil:
		return nil
	case errors.Is(err, domain.ErrOrderNotFound):
		return apperr.WrapNotFound(err, "订单不存在")
	case errors.Is(err, domain.ErrInvalidOrderNo):
		return apperr.WrapBadRequest(err, "订单号不合法")
	case errors.Is(err, domain.ErrInvalidCommand):
		return apperr.WrapBadRequest(err, "command 只能是 reassign/waive_cancel_fee/safe_end/review_pricing")
	case errors.Is(err, domain.ErrInvalidBasis):
		return apperr.WrapBadRequest(err, fmt.Sprintf("命令依据必须填写 %d-%d 字", domain.BasisMinLen, domain.BasisMaxLen))
	case errors.Is(err, domain.ErrOrderStateConflict):
		return apperr.WrapConflict(err, "订单当前状态不允许执行该命令")
	case errors.Is(err, domain.ErrCancelFeeNotFound):
		return apperr.WrapConflict(err, "该订单没有可免除的取消费")
	case errors.Is(err, domain.ErrSafeEndNotRunning):
		return apperr.WrapConflict(err, "只有服务中的订单可以安全提前结束")
	case errors.Is(err, domain.ErrPricingReviewed):
		return apperr.WrapConflict(err, "该订单计费已复核")
	default:
		return err
	}
}
