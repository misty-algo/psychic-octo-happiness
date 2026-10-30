// A03 订单全量管理：订单视图、穿透详情与业务命令规则。
// 规则基线：《各模块技术点与实行方案》7.1 运营管理、《后端接口文档》3.12.3-3.12.4。
package domain

import (
	"errors"
	"strings"
	"time"
	"unicode/utf8"
)

// 订单状态取值与库内 orders.state 完全一致（乘客端/司机端写入，管理端只读）。
// 集合索引 idx_passenger_state / idx_driver_state / idx_created_at 即按这些字段建立。
const (
	OrderStatePending   = "PENDING"
	OrderStateNoDriver  = "NO_DRIVER"
	OrderStateAccepted  = "ACCEPTED"
	OrderStateArrived   = "ARRIVED"
	OrderStateInService = "IN_SERVICE"
	OrderStateUnpaid    = "UNPAID"
	OrderStateCompleted = "COMPLETED"
	OrderStateCancelled = "CANCELLED"
	OrderStateSuspended = "SUSPENDED"
)

// 业务命令。管理端没有"任意改状态/改金额"的入口，所有人工动作都收敛为这四个
// 带权限、带前置条件、带依据留痕的命令。
const (
	// OrderCommandReassign 把未进入服务中的订单退回派单池重新派单。
	OrderCommandReassign = "reassign"
	// OrderCommandWaiveCancelFee 免除已取消订单的取消费。
	OrderCommandWaiveCancelFee = "waive_cancel_fee"
	// OrderCommandSafeEnd 因安全原因提前结束行程，订单进入待支付。
	OrderCommandSafeEnd = "safe_end"
	// OrderCommandReviewPricing 异常计费复核，只打复核标记，不改金额。
	OrderCommandReviewPricing = "review_pricing"
)

// 命令依据（工单号/说明）长度约束：太短无法追溯，过长没有意义。
const (
	BasisMinLen = 5
	BasisMaxLen = 200
)

// 订单规则错误。
var (
	ErrOrderNotFound      = errors.New("order not found")
	ErrInvalidOrderNo     = errors.New("invalid order no")
	ErrInvalidCommand     = errors.New("invalid order command")
	ErrInvalidBasis       = errors.New("invalid command basis")
	ErrOrderStateConflict = errors.New("order state rejects the command")
	ErrCancelFeeNotFound  = errors.New("order has no cancel fee to waive")
	ErrPricingReviewed    = errors.New("order pricing already reviewed")
	ErrSafeEndNotRunning  = errors.New("order is not in service")
)

// Order is the management-side read model of one order. Amounts are integer
// 分：管理端只展示，不做任何金额换算或调整。
type Order struct {
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
	CancelSide     string    `json:"cancel_side"`
	PricingVersion string    `json:"pricing_version"`
	// 计价明细：异常计费复核要看的就是这几个数，缺了就只能凭总价拍脑袋。
	PricingRuleID  string    `json:"pricing_rule_id"`
	SnapshotID     string    `json:"pricing_snapshot_id"`
	SurgeRatio     float64   `json:"surge_ratio"`
	EstimateFee    int64     `json:"estimate_fee"`
	Reviewed       bool      `json:"pricing_reviewed"`
	ReviewedBy     string    `json:"pricing_reviewed_by"`
	StartAddress   string    `json:"start_address"`
	EndAddress     string    `json:"end_address"`
	SafetyFlags    []string  `json:"safety_flags"`
	RiskLevel      string    `json:"risk_level"`
	CreatedAt      time.Time `json:"created_at"`
}

// Finished reports whether the order reached a terminal state. Terminal orders
// accept no state-changing command; only read-only review may run.
func (o Order) Finished() bool {
	return o.State == OrderStateCompleted || o.State == OrderStateCancelled
}

// DispatchTask is one dispatch record attached to an order. The driver side
// owns dispatch_tasks; management only reads it.
type DispatchTask struct {
	TaskNo     string    `json:"task_no"`
	DriverID   string    `json:"driver_id"`
	DriverName string    `json:"driver_name"`
	Round      int       `json:"round"`
	State      string    `json:"state"`
	CreatedAt  time.Time `json:"created_at"`
}

// DispatchAttempt is one driver's response to a dispatch broadcast: what the
// workbench shows as 接单尝试.
type DispatchAttempt struct {
	DriverID    string    `json:"driver_id"`
	DriverName  string    `json:"driver_name"`
	Result      string    `json:"result"`
	Score       float64   `json:"score"`
	Distance    float64   `json:"distance"`
	RespondedAt time.Time `json:"responded_at"`
}

// Trajectory carries raw tracking points. Point shapes differ per client, so
// the operations module passes them through verbatim instead of guessing keys.
type Trajectory struct {
	Source string           `json:"source"`
	Points []map[string]any `json:"points"`
}

// Pricing is the fare composition frozen onto the order: 计价版本与溢价。
type Pricing struct {
	Version     string    `json:"version"`
	RuleID      string    `json:"rule_id"`
	SnapshotID  string    `json:"snapshot_id"`
	EstimateFee int64     `json:"estimate_fee"`
	SurgeRatio  float64   `json:"surge_ratio"`
	TotalFee    int64     `json:"total_fee"`
	Reviewed    bool      `json:"reviewed"`
	ReviewedAt  time.Time `json:"reviewed_at"`
	ReviewedBy  string    `json:"reviewed_by"`
}

// Payment is the normalized part of a payment row plus its untouched source
// document, because the payment-side collection is still settling its schema.
type Payment struct {
	Channel      string         `json:"channel"`
	TradeNo      string         `json:"trade_no"`
	State        string         `json:"state"`
	Amount       int64          `json:"amount"`
	Refunded     bool           `json:"refunded"`
	RefundAmount int64          `json:"refund_amount"`
	Raw          map[string]any `json:"raw"`
}

// TicketBrief is a complaint/work ticket linked to the order; the full ticket
// lives in the A04 ticket module.
type TicketBrief struct {
	TicketNo  string    `json:"ticket_no"`
	Type      string    `json:"type"`
	State     string    `json:"state"`
	CreatedAt time.Time `json:"created_at"`
}

// OrderDetail is the penetration view: order plus every linked record the
// operator needs to explain what happened, without leaving the console.
type OrderDetail struct {
	Order      Order             `json:"order"`
	Tasks      []DispatchTask    `json:"tasks"`
	Attempts   []DispatchAttempt `json:"attempts"`
	Trajectory Trajectory        `json:"trajectory"`
	Pricing    Pricing           `json:"pricing"`
	Payment    *Payment          `json:"payment"`
	Tickets    []TicketBrief     `json:"tickets"`
}

// OrderCommandName is the validated command token plus its basis.
type OrderCommandName string

// ValidateOrderNo rejects blank or over-long order numbers.
func ValidateOrderNo(orderNo string) error {
	orderNo = strings.TrimSpace(orderNo)
	if orderNo == "" || utf8.RuneCountInString(orderNo) > 64 {
		return ErrInvalidOrderNo
	}
	return nil
}

// NewOrderCommand validates a command token and its 工单依据. Every command
// must be traceable to a ticket or written justification, so an empty or
// token-only basis is refused.
func NewOrderCommand(command, basis string) (string, error) {
	switch strings.TrimSpace(command) {
	case OrderCommandReassign, OrderCommandWaiveCancelFee, OrderCommandSafeEnd, OrderCommandReviewPricing:
	default:
		return "", ErrInvalidCommand
	}
	basis = strings.TrimSpace(basis)
	if utf8.RuneCountInString(basis) < BasisMinLen || utf8.RuneCountInString(basis) > BasisMaxLen {
		return "", ErrInvalidBasis
	}
	return strings.TrimSpace(command), nil
}

// CommandNames lists every command token for permission and UI wiring.
func CommandNames() []string {
	return []string{
		OrderCommandReassign,
		OrderCommandWaiveCancelFee,
		OrderCommandSafeEnd,
		OrderCommandReviewPricing,
	}
}

// CheckCommand reports whether the order's current state allows the command.
// State preconditions are the only guard against illegal transitions: the
// console exposes no free-form state or amount editor.
func (o Order) CheckCommand(command string) error {
	switch command {
	case OrderCommandReassign:
		// 已在服务中或已终结的订单不能改派：行程开始后换司机属于新订单。
		switch o.State {
		case OrderStatePending, OrderStateNoDriver, OrderStateAccepted, OrderStateArrived:
			return nil
		default:
			return ErrOrderStateConflict
		}
	case OrderCommandWaiveCancelFee:
		if o.State != OrderStateCancelled {
			return ErrOrderStateConflict
		}
		if o.CancelFee <= 0 {
			return ErrCancelFeeNotFound
		}
		return nil
	case OrderCommandSafeEnd:
		if o.State != OrderStateInService {
			return ErrSafeEndNotRunning
		}
		return nil
	case OrderCommandReviewPricing:
		// 只有已产生计费的订单需要复核，未出账无法复核。
		if o.State != OrderStateUnpaid && o.State != OrderStateCompleted {
			return ErrOrderStateConflict
		}
		return nil
	default:
		return ErrInvalidCommand
	}
}

// CommandTargetState returns the state the command produces. Commands that
// only annotate the order keep the current state.
func (o Order) CommandTargetState(command string) string {
	switch command {
	case OrderCommandReassign:
		return OrderStatePending
	case OrderCommandSafeEnd:
		return OrderStateUnpaid
	default:
		return o.State
	}
}
