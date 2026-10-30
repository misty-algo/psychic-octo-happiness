package operationshttp

import (
	"github.com/labstack/echo/v5"

	"github.com/NSObjects/echo-admin/internal/modules/operations/usecase"
	"github.com/NSObjects/echo-admin/internal/platform/requestctx"
	"github.com/NSObjects/echo-admin/internal/platform/server/httpreq"
	"github.com/NSObjects/echo-admin/internal/platform/server/httpresp"
)

// withdrawalDecisionRequest 是审批/驳回的请求体。
type withdrawalDecisionRequest struct {
	Reason string `json:"reason"`
}

// ListWithdrawals 提现审批队列。
func (h *Handler) ListWithdrawals(c *echo.Context) error {
	page, pageSize, err := httpreq.Pagination(c)
	if err != nil {
		return err
	}
	output, err := h.withdrawals.ListWithdrawals(c.Request().Context(), usecase.ListWithdrawalsInput{
		State:    c.QueryParam("state"),
		Keyword:  c.QueryParam("keyword"),
		Page:     page,
		PageSize: pageSize,
	})
	if err != nil {
		return err
	}
	return httpresp.Paginated(c, output.Items, page, pageSize, int(output.Total))
}

// ApproveWithdrawal 通过一笔提现；审批齐备时随即打款出账。
func (h *Handler) ApproveWithdrawal(c *echo.Context) error {
	operatorID, err := requestctx.RequireUserID(c.Request().Context())
	if err != nil {
		return err
	}
	var req withdrawalDecisionRequest
	if err := httpreq.BindAndValidate(c, &req); err != nil {
		return err
	}
	withdrawalID := c.Param("id")
	output, opErr := h.withdrawals.Approve(c.Request().Context(), usecase.WithdrawalDecisionInput{
		WithdrawalID: withdrawalID,
		Reason:       req.Reason,
		OperatorID:   operatorID,
	})
	detail := "提现审批"
	if opErr == nil {
		detail = "提现审批：" + output.Status + "，" + output.Message
	}
	if auditErr := h.audit.Record(c, "withdrawal_approve", "ops_withdraw", withdrawalID, detail, opErr); auditErr != nil {
		return auditErr
	}
	if opErr != nil {
		return opErr
	}
	return httpresp.OK(c, output)
}

// RejectWithdrawal 驳回一笔提现（返还司机可用余额）。
func (h *Handler) RejectWithdrawal(c *echo.Context) error {
	operatorID, err := requestctx.RequireUserID(c.Request().Context())
	if err != nil {
		return err
	}
	var req withdrawalDecisionRequest
	if err := httpreq.BindAndValidate(c, &req); err != nil {
		return err
	}
	withdrawalID := c.Param("id")
	output, opErr := h.withdrawals.Reject(c.Request().Context(), usecase.WithdrawalDecisionInput{
		WithdrawalID: withdrawalID,
		Reason:       req.Reason,
		OperatorID:   operatorID,
	})
	detail := "提现驳回"
	if opErr == nil {
		detail = "提现驳回：" + output.Status + "，" + output.Message
	}
	if auditErr := h.audit.Record(c, "withdrawal_reject", "ops_withdraw", withdrawalID, detail, opErr); auditErr != nil {
		return auditErr
	}
	if opErr != nil {
		return opErr
	}
	return httpresp.OK(c, output)
}

// PayWithdrawal 对已通过但未打款的提现补发打款回调。
func (h *Handler) PayWithdrawal(c *echo.Context) error {
	// 打款由财务侧执行，无需操作员身份参与司机域命令，仅校验已登录。
	if _, err := requestctx.RequireUserID(c.Request().Context()); err != nil {
		return err
	}
	withdrawalID := c.Param("id")
	output, opErr := h.withdrawals.Pay(c.Request().Context(), withdrawalID)
	detail := "提现打款"
	if opErr == nil {
		detail = "提现打款：" + output.Status
	}
	if auditErr := h.audit.Record(c, "withdrawal_pay", "ops_withdraw", withdrawalID, detail, opErr); auditErr != nil {
		return auditErr
	}
	if opErr != nil {
		return opErr
	}
	return httpresp.OK(c, output)
}
