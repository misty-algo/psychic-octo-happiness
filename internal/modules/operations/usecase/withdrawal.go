package usecase

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/NSObjects/echo-admin/internal/platform/apperr"
)

// WithdrawalStore reads the driver-side withdraw_requests collection. The
// collection is owned by the ride-hailing driver domain (资金预占/返还/出账
// 都在那边), so the console must never write it directly.
type WithdrawalStore interface {
	// ListWithdrawals returns one page of withdrawal requests newest first.
	ListWithdrawals(ctx context.Context, filter WithdrawalFilter) ([]Withdrawal, int64, error)
	// DriverBriefs returns driver_no/name for the given driver ids
	// (driver_profiles._id is a business string id, not an ObjectID).
	DriverBriefs(ctx context.Context, driverIDs []string) (map[string]WithdrawalDriverBrief, error)
}

// WithdrawalDriverBrief is the identity shown next to a request.
type WithdrawalDriverBrief struct {
	DriverNo string
	Name     string
}

// WithdrawalFilter narrows the approval queue.
type WithdrawalFilter struct {
	State   string // reviewing/approved/paying/success/rejected
	Keyword string // withdrawal id or driver id fragment
	Skip    int
	Limit   int
}

// Withdrawal is one withdrawal request row.
type Withdrawal struct {
	ID           string    `json:"id"`
	DriverID     string    `json:"driver_id"`
	AmountFen    int64     `json:"amount_fen"`
	Status       string    `json:"status"`
	DualApproval bool      `json:"dual_approval"`
	Approvals    int       `json:"approvals"`
	Reason       string    `json:"reason"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

// ListWithdrawalsInput carries the queue query.
type ListWithdrawalsInput struct {
	State    string
	Keyword  string
	Page     int
	PageSize int
}

// ListWithdrawalsOutput is one page of enriched rows.
type ListWithdrawalsOutput struct {
	Items []WithdrawalItem `json:"items"`
	Total int64            `json:"total"`
}

// WithdrawalItem joins driver identity onto the raw request.
type WithdrawalItem struct {
	Withdrawal
	DriverNo   string `json:"driver_no"`
	DriverName string `json:"driver_name"`
}

// WithdrawalActionUsecase approves/rejects/pays withdrawals by calling the
// ride-hailing gateway: 预占返还与出账逻辑都在司机域服务层，控制台只发起命令，
// 绝不直接改 withdraw_requests。
type WithdrawalActionUsecase struct {
	store  WithdrawalStore
	gwBase string
	client *http.Client
}

// NewWithdrawal creates the withdrawal approval usecase. gatewayBaseURL is
// the ride-hailing gateway (主项目网关) root, e.g. http://127.0.0.1:8080.
func NewWithdrawal(store WithdrawalStore, gatewayBaseURL string) *WithdrawalActionUsecase {
	return &WithdrawalActionUsecase{
		store:  store,
		gwBase: strings.TrimRight(gatewayBaseURL, "/"),
		client: &http.Client{Timeout: 10 * time.Second},
	}
}

// ListWithdrawals returns the approval queue.
func (u *WithdrawalActionUsecase) ListWithdrawals(ctx context.Context, input ListWithdrawalsInput) (ListWithdrawalsOutput, error) {
	filter := WithdrawalFilter{
		State:   strings.TrimSpace(input.State),
		Keyword: strings.TrimSpace(input.Keyword),
		Skip:    (input.Page - 1) * input.PageSize,
		Limit:   input.PageSize,
	}
	if filter.Skip < 0 {
		filter.Skip = 0
	}
	if filter.Limit <= 0 || filter.Limit > maxPageSize {
		filter.Limit = maxPageSize
	}
	rows, total, err := u.store.ListWithdrawals(ctx, filter)
	if err != nil {
		return ListWithdrawalsOutput{}, err
	}
	driverIDs := make([]string, 0, len(rows))
	for _, row := range rows {
		driverIDs = append(driverIDs, row.DriverID)
	}
	briefs, err := u.store.DriverBriefs(ctx, driverIDs)
	if err != nil {
		return ListWithdrawalsOutput{}, err
	}
	items := make([]WithdrawalItem, 0, len(rows))
	for _, row := range rows {
		brief := briefs[row.DriverID]
		items = append(items, WithdrawalItem{
			Withdrawal: row,
			DriverNo:   brief.DriverNo,
			DriverName: brief.Name,
		})
	}
	return ListWithdrawalsOutput{Items: items, Total: total}, nil
}

// WithdrawalDecisionInput carries an approve/reject command.
type WithdrawalDecisionInput struct {
	WithdrawalID string
	Reason       string
	OperatorID   int64
}

// WithdrawalResult is the post-command state as seen by the driver domain.
type WithdrawalResult struct {
	WithdrawalID string `json:"withdrawal_id"`
	Status       string `json:"status"`
	Paid         bool   `json:"paid"`
	Message      string `json:"message"`
}

// statusOnly 只提取司机域返回的提现状态。
type statusOnly struct {
	Status string `json:"status"`
}

// Approve records one approval via the gateway. 一旦司机域判定审批齐备
// （非大额 1 人、大额 2 人），控制台随即发起打款回调完成出账；仍在
// reviewing 则说明双人审批还差一票。
func (u *WithdrawalActionUsecase) Approve(ctx context.Context, input WithdrawalDecisionInput) (WithdrawalResult, error) {
	if strings.TrimSpace(input.WithdrawalID) == "" {
		return WithdrawalResult{}, apperr.WrapBadRequest(fmt.Errorf("empty withdrawal id"), "提现单号不能为空")
	}
	approver := operatorName(input.OperatorID)
	var payload statusOnly
	if err := u.gateway(ctx, "/api/v1/driver/withdrawals/approve", map[string]any{
		"withdrawal_id": input.WithdrawalID,
		"approver_id":   approver,
		"comment":       strings.TrimSpace(input.Reason),
	}, &payload); err != nil {
		return WithdrawalResult{}, err
	}
	// 双人审批未凑齐时仍是 reviewing：不打款，提示需要另一位审核员。
	if payload.Status != "approved" && payload.Status != "paying" {
		return WithdrawalResult{
			WithdrawalID: input.WithdrawalID, Status: payload.Status,
			Message: "已记一笔审批，还需另一位审核员审批",
		}, nil
	}
	// 审批齐备即打款：本地演示无代付通道，直接回调司机域完成出账。
	var paid statusOnly
	if err := u.gateway(ctx, "/api/v1/driver/withdrawals/complete", map[string]any{
		"withdrawal_id": input.WithdrawalID, "success": true,
	}, &paid); err != nil {
		return WithdrawalResult{
			WithdrawalID: input.WithdrawalID, Status: payload.Status,
			Message: "审批已通过，打款回调失败，请在打款页重试",
		}, nil
	}
	return WithdrawalResult{
		WithdrawalID: input.WithdrawalID, Status: paid.Status, Paid: true,
		Message: "审批通过，已打款",
	}, nil
}

// Reject returns the reserved funds to the driver's available bucket.
func (u *WithdrawalActionUsecase) Reject(ctx context.Context, input WithdrawalDecisionInput) (WithdrawalResult, error) {
	if strings.TrimSpace(input.WithdrawalID) == "" {
		return WithdrawalResult{}, apperr.WrapBadRequest(fmt.Errorf("empty withdrawal id"), "提现单号不能为空")
	}
	if len(strings.TrimSpace(input.Reason)) < 5 {
		return WithdrawalResult{}, apperr.WrapBadRequest(fmt.Errorf("reason too short"), "驳回必须填写原因（至少 5 字）")
	}
	approver := operatorName(input.OperatorID)
	var payload statusOnly
	if err := u.gateway(ctx, "/api/v1/driver/withdrawals/reject", map[string]any{
		"withdrawal_id": input.WithdrawalID,
		"approver_id":   approver,
		"reason":        strings.TrimSpace(input.Reason),
	}, &payload); err != nil {
		return WithdrawalResult{}, err
	}
	return WithdrawalResult{
		WithdrawalID: input.WithdrawalID, Status: payload.Status,
		Message: "已驳回，预占额度已返还司机可用余额",
	}, nil
}

// Pay completes the payout for an already approved request.
func (u *WithdrawalActionUsecase) Pay(ctx context.Context, withdrawalID string) (WithdrawalResult, error) {
	if strings.TrimSpace(withdrawalID) == "" {
		return WithdrawalResult{}, apperr.WrapBadRequest(fmt.Errorf("empty withdrawal id"), "提现单号不能为空")
	}
	var payload statusOnly
	if err := u.gateway(ctx, "/api/v1/driver/withdrawals/complete", map[string]any{
		"withdrawal_id": withdrawalID, "success": true,
	}, &payload); err != nil {
		return WithdrawalResult{}, err
	}
	return WithdrawalResult{
		WithdrawalID: withdrawalID, Status: payload.Status, Paid: true,
		Message: "打款完成",
	}, nil
}

// gateway POSTs one command to the ride-hailing gateway and decodes data.
func (u *WithdrawalActionUsecase) gateway(ctx context.Context, path string, body map[string]any, out *statusOnly) error {
	buf, err := json.Marshal(body)
	if err != nil {
		return apperr.WrapInternal(err, "编码请求失败")
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, u.gwBase+path, bytes.NewReader(buf))
	if err != nil {
		return apperr.WrapInternal(err, "构造网关请求失败")
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := u.client.Do(req)
	if err != nil {
		return apperr.WrapInternal(err, "连接打车网关失败，请确认主项目服务已启动")
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return apperr.WrapInternal(err, "读取网关响应失败")
	}
	if resp.StatusCode >= 300 {
		// 网关错误体形如 {"error":{"code":"...","message":"..."}}，透传给操作员；
		// 解析不出结构时把原始响应带出来，避免只剩一句默认文案没法排查。
		detail := string(raw)
		if len(detail) > 500 {
			detail = detail[:500]
		}
		var errBody struct {
			Error struct {
				Code    string `json:"code"`
				Message string `json:"message"`
			} `json:"error"`
		}
		if json.Unmarshal(raw, &errBody) == nil && errBody.Error.Message != "" {
			return apperr.WrapBadRequest(fmt.Errorf("gateway %s: %s", errBody.Error.Code, errBody.Error.Message), errBody.Error.Message)
		}
		if resp.StatusCode < 500 {
			return apperr.WrapBadRequest(fmt.Errorf("gateway status %d: %s", resp.StatusCode, detail), "网关拒绝请求："+detail)
		}
		return apperr.WrapInternal(fmt.Errorf("gateway status %d: %s", resp.StatusCode, detail), "网关返回异常")
	}
	var envelope struct {
		Data *struct {
			Status string `json:"status"`
		} `json:"data"`
	}
	if err := json.Unmarshal(raw, &envelope); err != nil {
		return apperr.WrapInternal(err, "解析网关响应失败")
	}
	if envelope.Data != nil {
		out.Status = envelope.Data.Status
	}
	return nil
}
