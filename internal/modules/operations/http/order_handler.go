package operationshttp

import (
	"github.com/labstack/echo/v5"

	"github.com/NSObjects/echo-admin/internal/modules/operations/usecase"
	"github.com/NSObjects/echo-admin/internal/platform/requestctx"
	"github.com/NSObjects/echo-admin/internal/platform/server/httpreq"
	"github.com/NSObjects/echo-admin/internal/platform/server/httpresp"
)

// ListOrders returns one page of the order workbench.
func (h *Handler) ListOrders(c *echo.Context) error {
	page, pageSize, err := httpreq.Pagination(c)
	if err != nil {
		return err
	}
	output, err := h.orders.ListOrders(c.Request().Context(), usecase.ListOrdersInput{
		Keyword:   c.QueryParam("keyword"),
		State:     c.QueryParam("state"),
		BizType:   c.QueryParam("biz_type"),
		CityCode:  c.QueryParam("city_code"),
		RiskLevel: c.QueryParam("risk_level"),
		From:      c.QueryParam("from"),
		To:        c.QueryParam("to"),
		Page:      page,
		PageSize:  pageSize,
	})
	if err != nil {
		return err
	}
	return httpresp.Paginated(c, output.Items, page, pageSize, int(output.Total))
}

// OrderDetail returns the penetration view of one order.
func (h *Handler) OrderDetail(c *echo.Context) error {
	output, err := h.orders.Detail(c.Request().Context(), c.Param("order_no"))
	if err != nil {
		return err
	}
	return httpresp.OK(c, output)
}

// ExecuteOrderCommand runs one guarded business command on an order.
func (h *Handler) ExecuteOrderCommand(c *echo.Context) error {
	operatorID, err := requestctx.RequireUserID(c.Request().Context())
	if err != nil {
		return err
	}
	var req orderCommandRequest
	if err := httpreq.BindAndValidate(c, &req); err != nil {
		return err
	}
	orderNo := c.Param("order_no")
	output, opErr := h.orders.Execute(c.Request().Context(), usecase.CommandInput{
		OrderNo:    orderNo,
		Command:    req.Command,
		Basis:      req.Basis,
		OperatorID: operatorID,
		RequestID:  requestctx.GetRequestID(c.Request().Context()),
	})
	detail := "订单命令"
	if opErr == nil {
		detail = "订单命令：" + output.Command + " → " + output.State
	}
	if auditErr := h.audit.Record(c, "order_command", "ops_order", orderNo, detail, opErr); auditErr != nil {
		return auditErr
	}
	if opErr != nil {
		return opErr
	}
	return httpresp.OK(c, output)
}
