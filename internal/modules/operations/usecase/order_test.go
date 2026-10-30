package usecase

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/NSObjects/echo-admin/internal/modules/operations/domain"
)

// fakeOrderStore is an in-memory OrderStore double.
type fakeOrderStore struct {
	orders     []domain.Order
	passenger  map[string]PassengerBrief
	drivers    map[string]DriverBrief
	tasks      map[string][]domain.DispatchTask
	attempts   map[string][]domain.DispatchAttempt
	tracks     map[string]domain.Trajectory
	tickets    map[string][]domain.TicketBrief
	payments   map[string]domain.Payment
	logs       []domain.OperationLog
	stateAfter map[string]string
	applied    []OrderCommand
}

func (f *fakeOrderStore) ListOrders(_ context.Context, filter OrderFilter) ([]domain.Order, int64, error) {
	out := make([]domain.Order, 0, len(f.orders))
	for _, order := range f.orders {
		if filter.State != "" && order.State != filter.State {
			continue
		}
		if filter.Keyword != "" && order.OrderNo != filter.Keyword {
			continue
		}
		out = append(out, order)
	}
	return out, int64(len(out)), nil
}

func (f *fakeOrderStore) FindOrder(_ context.Context, orderNo string) (domain.Order, error) {
	for _, order := range f.orders {
		if order.OrderNo == orderNo {
			return order, nil
		}
	}
	return domain.Order{}, domain.ErrOrderNotFound
}

func (f *fakeOrderStore) PassengerBriefs(_ context.Context, ids []string) (map[string]PassengerBrief, error) {
	out := make(map[string]PassengerBrief, len(ids))
	for _, id := range ids {
		if brief, ok := f.passenger[id]; ok {
			out[id] = brief
		}
	}
	return out, nil
}

func (f *fakeOrderStore) DriverBriefs(_ context.Context, ids []string) (map[string]DriverBrief, error) {
	out := make(map[string]DriverBrief, len(ids))
	for _, id := range ids {
		if brief, ok := f.drivers[id]; ok {
			out[id] = brief
		}
	}
	return out, nil
}

func (f *fakeOrderStore) ListDispatchTasks(_ context.Context, ids []string) (map[string][]domain.DispatchTask, error) {
	out := make(map[string][]domain.DispatchTask, len(ids))
	for _, id := range ids {
		tasks := append([]domain.DispatchTask{}, f.tasks[id]...)
		for i := range tasks {
			tasks[i].DriverName = f.drivers[tasks[i].DriverID].Name
		}
		out[id] = tasks
	}
	return out, nil
}

func (f *fakeOrderStore) ListDispatchAttempts(_ context.Context, ids []string) (map[string][]domain.DispatchAttempt, error) {
	out := make(map[string][]domain.DispatchAttempt, len(ids))
	for _, id := range ids {
		out[id] = f.attempts[id]
	}
	return out, nil
}

func (f *fakeOrderStore) ListOrderTrajectories(_ context.Context, ids []string) (map[string]domain.Trajectory, error) {
	out := make(map[string]domain.Trajectory, len(ids))
	for _, id := range ids {
		if track, ok := f.tracks[id]; ok {
			out[id] = track
		}
	}
	return out, nil
}

func (f *fakeOrderStore) ListOrderTickets(_ context.Context, ids []string) (map[string][]domain.TicketBrief, error) {
	out := make(map[string][]domain.TicketBrief, len(ids))
	for _, id := range ids {
		out[id] = f.tickets[id]
	}
	return out, nil
}

func (f *fakeOrderStore) FindPayments(_ context.Context, orders []domain.Order) (map[string]domain.Payment, error) {
	out := make(map[string]domain.Payment, len(orders))
	for _, order := range orders {
		if payment, ok := f.payments[order.ID]; ok {
			out[order.ID] = payment
		}
	}
	return out, nil
}

func (f *fakeOrderStore) ExecuteCommand(_ context.Context, command OrderCommand) (domain.Order, error) {
	f.applied = append(f.applied, command)
	for i, order := range f.orders {
		if order.OrderNo != command.OrderNo {
			continue
		}
		// stateAfter 模拟另一个操作者抢先改了状态：CAS 命中不到预期状态。
		if next, ok := f.stateAfter[command.OrderNo]; ok {
			f.orders[i].State = next
			return domain.Order{}, domain.ErrOrderStateConflict
		}
		f.orders[i].State = command.TargetState
		if command.Command == domain.OrderCommandWaiveCancelFee {
			f.orders[i].CancelFee = 0
		}
		return f.orders[i], nil
	}
	return domain.Order{}, domain.ErrOrderNotFound
}

func (f *fakeOrderStore) RecordOperationLog(_ context.Context, log domain.OperationLog) error {
	f.logs = append(f.logs, log)
	return nil
}

func newOrderStore() *fakeOrderStore {
	return &fakeOrderStore{
		passenger:  map[string]PassengerBrief{"p1": {Name: "乘客A", Phone: "138****6688"}},
		drivers:    map[string]DriverBrief{"d1": {Name: "王师傅", Plate: "沪AD·66666"}},
		tasks:      map[string][]domain.DispatchTask{},
		attempts:   map[string][]domain.DispatchAttempt{},
		tracks:     map[string]domain.Trajectory{},
		tickets:    map[string][]domain.TicketBrief{},
		payments:   map[string]domain.Payment{},
		stateAfter: map[string]string{},
	}
}

func TestListOrdersEnrichesIdentity(t *testing.T) {
	store := newOrderStore()
	store.orders = []domain.Order{{
		ID: "o1", OrderNo: "HZ0001", PassengerID: "p1", DriverID: "d1",
		State: domain.OrderStateAccepted, Amount: 1370, CreatedAt: time.Now(),
	}}
	uc := NewOrder(store)
	out, err := uc.ListOrders(context.Background(), ListOrdersInput{Page: 1, PageSize: 20})
	if err != nil {
		t.Fatalf("ListOrders: %v", err)
	}
	if len(out.Items) != 1 || out.Total != 1 {
		t.Fatalf("items=%d total=%d, want 1/1", len(out.Items), out.Total)
	}
	row := out.Items[0]
	if row.PassengerName != "乘客A" || row.PassengerPhone != "138****6688" {
		t.Fatalf("passenger = %+v, want 乘客A/138****6688", row)
	}
	if row.DriverName != "王师傅" || row.Plate != "沪AD·66666" {
		t.Fatalf("driver = %+v, want 王师傅/沪AD·66666", row)
	}
}

func TestDetailReturnsLinkedRecords(t *testing.T) {
	store := newOrderStore()
	store.orders = []domain.Order{{ID: "o1", OrderNo: "HZ0001", PassengerID: "p1", DriverID: "d1",
		State: domain.OrderStateCompleted, Amount: 1370, PricingVersion: "v3", CreatedAt: time.Now()}}
	store.tasks["o1"] = []domain.DispatchTask{{TaskNo: "T1", DriverID: "d1"}}
	store.attempts["o1"] = []domain.DispatchAttempt{{DriverID: "d1", Result: "accept"}}
	store.tickets["o1"] = []domain.TicketBrief{{TicketNo: "W001", State: "待处理"}}
	store.payments["o1"] = domain.Payment{Channel: "wechat", TradeNo: "TR1", State: "paid", Amount: 1370}
	uc := NewOrder(store)
	detail, err := uc.Detail(context.Background(), "HZ0001")
	if err != nil {
		t.Fatalf("Detail: %v", err)
	}
	if len(detail.Tasks) != 1 || detail.Tasks[0].DriverName != "王师傅" {
		t.Fatalf("tasks = %+v, want 1 task with driver name", detail.Tasks)
	}
	if len(detail.Attempts) != 1 || detail.Attempts[0].Result != "accept" {
		t.Fatalf("attempts = %+v", detail.Attempts)
	}
	if len(detail.Tickets) != 1 || detail.Tickets[0].TicketNo != "W001" {
		t.Fatalf("tickets = %+v", detail.Tickets)
	}
	if detail.Payment == nil || detail.Payment.TradeNo != "TR1" {
		t.Fatalf("payment = %+v, want TR1", detail.Payment)
	}
	if detail.Pricing.Version != "v3" {
		t.Fatalf("pricing = %+v, want version v3", detail.Pricing)
	}
}

func TestCommandRejectsShortBasis(t *testing.T) {
	store := newOrderStore()
	store.orders = []domain.Order{{ID: "o1", OrderNo: "HZ0001", State: domain.OrderStateAccepted}}
	uc := NewOrder(store)
	_, err := uc.Execute(context.Background(), CommandInput{
		OrderNo: "HZ0001", Command: domain.OrderCommandReassign, Basis: "工单", OperatorID: 1,
	})
	if !errors.Is(err, domain.ErrInvalidBasis) {
		t.Fatalf("err = %v, want ErrInvalidBasis", err)
	}
	if len(store.applied) != 0 {
		t.Fatalf("command applied despite invalid basis")
	}
}

func TestCommandRejectsWrongState(t *testing.T) {
	store := newOrderStore()
	store.orders = []domain.Order{{ID: "o1", OrderNo: "HZ0001", State: domain.OrderStateCompleted}}
	uc := NewOrder(store)
	_, err := uc.Execute(context.Background(), CommandInput{
		OrderNo: "HZ0001", Command: domain.OrderCommandReassign, Basis: "工单 W001 乘客要求换司机", OperatorID: 1,
	})
	if !errors.Is(err, domain.ErrOrderStateConflict) {
		t.Fatalf("err = %v, want ErrOrderStateConflict", err)
	}
}

func TestCommandWaiveCancelFeeRequiresFee(t *testing.T) {
	store := newOrderStore()
	store.orders = []domain.Order{{ID: "o1", OrderNo: "HZ0001", State: domain.OrderStateCancelled}}
	uc := NewOrder(store)
	_, err := uc.Execute(context.Background(), CommandInput{
		OrderNo: "HZ0001", Command: domain.OrderCommandWaiveCancelFee, Basis: "工单 W002 司机迟到", OperatorID: 1,
	})
	if !errors.Is(err, domain.ErrCancelFeeNotFound) {
		t.Fatalf("err = %v, want ErrCancelFeeNotFound", err)
	}
}

func TestCommandSafeEndRequiresInService(t *testing.T) {
	store := newOrderStore()
	store.orders = []domain.Order{{ID: "o1", OrderNo: "HZ0001", State: domain.OrderStateAccepted}}
	uc := NewOrder(store)
	_, err := uc.Execute(context.Background(), CommandInput{
		OrderNo: "HZ0001", Command: domain.OrderCommandSafeEnd, Basis: "工单 W003 乘客安全诉求", OperatorID: 1,
	})
	if !errors.Is(err, domain.ErrSafeEndNotRunning) {
		t.Fatalf("err = %v, want ErrSafeEndNotRunning", err)
	}
}

func TestCommandSucceedsAndLogs(t *testing.T) {
	store := newOrderStore()
	store.orders = []domain.Order{{ID: "o1", OrderNo: "HZ0001", State: domain.OrderStateInService, Amount: 1370}}
	uc := NewOrder(store)
	out, err := uc.Execute(context.Background(), CommandInput{
		OrderNo: "HZ0001", Command: domain.OrderCommandSafeEnd, Basis: "工单 W003 乘客安全诉求", OperatorID: 7,
	})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if out.State != domain.OrderStateUnpaid {
		t.Fatalf("state = %s, want UNPAID", out.State)
	}
	if len(store.logs) != 1 {
		t.Fatalf("logs = %d, want 1", len(store.logs))
	}
	if store.logs[0].Operator != "admin:7" {
		t.Fatalf("operator = %s, want admin:7", store.logs[0].Operator)
	}
	if store.logs[0].Resource != "ops_order" || store.logs[0].Action != "order_command" {
		t.Fatalf("log = %+v", store.logs[0])
	}
}

func TestCommandLosesRaceOnStateChange(t *testing.T) {
	store := newOrderStore()
	store.orders = []domain.Order{{ID: "o1", OrderNo: "HZ0001", State: domain.OrderStateInService}}
	store.stateAfter["HZ0001"] = domain.OrderStateUnpaid
	uc := NewOrder(store)
	_, err := uc.Execute(context.Background(), CommandInput{
		OrderNo: "HZ0001", Command: domain.OrderCommandSafeEnd, Basis: "工单 W003 乘客安全诉求", OperatorID: 7,
	})
	if !errors.Is(err, domain.ErrOrderStateConflict) {
		t.Fatalf("err = %v, want ErrOrderStateConflict", err)
	}
	if len(store.logs) != 0 {
		t.Fatalf("lost race still wrote %d business logs", len(store.logs))
	}
}

func TestCommandUnknownOrder(t *testing.T) {
	store := newOrderStore()
	uc := NewOrder(store)
	_, err := uc.Execute(context.Background(), CommandInput{
		OrderNo: "HZ9999", Command: domain.OrderCommandReassign, Basis: "工单 W001 乘客要求换司机", OperatorID: 1,
	})
	if !errors.Is(err, domain.ErrOrderNotFound) {
		t.Fatalf("err = %v, want ErrOrderNotFound", err)
	}
}

func TestParseTimeFormats(t *testing.T) {
	cases := []struct {
		name    string
		value   string
		wantErr bool
	}{
		{name: "rfc3339", value: "2026-10-30T00:00:00Z"},
		{name: "console picker layout", value: "2026-10-30 08:30:00"},
		{name: "calendar day", value: "2026-10-30"},
		// 空输入由调用方（ListOrders）先判空跳过，parseTime 本身拒绝空串。
		{name: "empty rejected", value: "   ", wantErr: true},
		{name: "garbage", value: "yesterday", wantErr: true},
		{name: "space layout with bad month", value: "2026-13-30 08:30:00", wantErr: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := parseTime(tc.value)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("parseTime(%q) = %v, want error", tc.value, got)
				}
				return
			}
			if err != nil {
				t.Fatalf("parseTime(%q) error = %v, want nil", tc.value, err)
			}
			if got.IsZero() {
				t.Fatalf("parseTime(%q) = zero time, want parsed value", tc.value)
			}
		})
	}
}

func TestEndOfDayCoversWholeDay(t *testing.T) {
	end, ok := endOfDay("2026-10-30")
	if !ok {
		t.Fatal("endOfDay(2026-10-30) not ok, want date-only input accepted")
	}
	if end.Hour() != 23 || end.Minute() != 59 || end.Second() != 59 {
		t.Fatalf("endOfDay = %s, want 23:59:59 of the same day", end)
	}
	if _, ok := endOfDay("2026-10-30 08:00:00"); ok {
		t.Fatal("endOfDay on full timestamp should report false, want caller to parse as-is")
	}
}

// TestListOrdersRejectsBadTimeRange 保证时间边界解析失败直接报错，
// 而不是静默返回全量数据让操作员误以为筛选生效。
func TestListOrdersRejectsBadTimeRange(t *testing.T) {
	uc := NewOrder(newOrderStore())
	_, err := uc.ListOrders(context.Background(), ListOrdersInput{
		Page: 1, PageSize: 20, From: "yesterday",
	})
	if err == nil {
		t.Fatal("ListOrders with malformed from accepted, want bad request error")
	}
}

// TestEnrichKeepsOrderDriverName 保证乘客域快照里的司机姓名/车牌
// 不会被 enrich 覆盖成空：订单没有司机档案 ID 时也要展示姓名。
func TestEnrichKeepsOrderDriverName(t *testing.T) {
	store := newOrderStore()
	store.orders = []domain.Order{{
		ID: "o1", OrderNo: "HZ0002", PassengerID: "p1",
		DriverName: "张师傅", Plate: "沪AD·88888",
		State: domain.OrderStateAccepted, CreatedAt: time.Now(),
	}}
	uc := NewOrder(store)
	out, err := uc.ListOrders(context.Background(), ListOrdersInput{Page: 1, PageSize: 20})
	if err != nil {
		t.Fatalf("ListOrders: %v", err)
	}
	if len(out.Items) != 1 {
		t.Fatalf("items = %d, want 1", len(out.Items))
	}
	row := out.Items[0]
	if row.DriverName != "张师傅" || row.Plate != "沪AD·88888" {
		t.Fatalf("driver = %q/%q, want 张师傅/沪AD·88888", row.DriverName, row.Plate)
	}
	if row.PassengerName != "乘客A" {
		t.Fatalf("passenger = %q, want 乘客A", row.PassengerName)
	}
}
