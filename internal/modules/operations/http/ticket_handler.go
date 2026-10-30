package operationshttp

import (
	"github.com/labstack/echo/v5"

	"github.com/NSObjects/echo-admin/internal/modules/operations/usecase"
	"github.com/NSObjects/echo-admin/internal/platform/requestctx"
	"github.com/NSObjects/echo-admin/internal/platform/server/httpreq"
	"github.com/NSObjects/echo-admin/internal/platform/server/httpresp"
)

// ListTickets returns one page of the ticket workbench.
func (h *Handler) ListTickets(c *echo.Context) error {
	page, pageSize, err := httpreq.Pagination(c)
	if err != nil {
		return err
	}
	output, err := h.tickets.ListTickets(c.Request().Context(), usecase.ListTicketsInput{
		Keyword:  c.QueryParam("keyword"),
		State:    c.QueryParam("state"),
		Type:     c.QueryParam("type"),
		Group:    c.QueryParam("group"),
		Overdue:  c.QueryParam("overdue") == "true",
		Page:     page,
		PageSize: pageSize,
	})
	if err != nil {
		return err
	}
	return httpresp.Paginated(c, output.Items, page, pageSize, int(output.Total))
}

// TicketDetail returns one ticket with its duplicate candidates.
func (h *Handler) TicketDetail(c *echo.Context) error {
	output, err := h.tickets.Detail(c.Request().Context(), c.Param("ticket_no"))
	if err != nil {
		return err
	}
	return httpresp.OK(c, output)
}

// ExecuteTicketCommand runs one guarded state transition on a ticket.
func (h *Handler) ExecuteTicketCommand(c *echo.Context) error {
	operatorID, err := requestctx.RequireUserID(c.Request().Context())
	if err != nil {
		return err
	}
	var req ticketCommandRequest
	if err := httpreq.BindAndValidate(c, &req); err != nil {
		return err
	}
	ticketNo := c.Param("ticket_no")
	output, opErr := h.tickets.Execute(c.Request().Context(), usecase.TicketCommandInput{
		TicketNo:   ticketNo,
		Command:    req.Command,
		Message:    req.Message,
		Assignee:   req.Assignee,
		MergeNo:    req.MergeNo,
		OperatorID: operatorID,
		RequestID:  requestctx.GetRequestID(c.Request().Context()),
	})
	detail := "工单命令"
	if opErr == nil {
		detail = "工单命令：" + output.Command + " → " + output.State
	}
	if auditErr := h.audit.Record(c, "ticket_command", "ops_ticket", ticketNo, detail, opErr); auditErr != nil {
		return auditErr
	}
	if opErr != nil {
		return opErr
	}
	return httpresp.OK(c, output)
}

// AppealTicket opens a new handling round. It is a separate route because the
// appeal must be taken by a senior agent: the框架 enforces ops_ticket:senior
// here, and the usecase refuses the previous handler.
func (h *Handler) AppealTicket(c *echo.Context) error {
	operatorID, err := requestctx.RequireUserID(c.Request().Context())
	if err != nil {
		return err
	}
	var req ticketAppealRequest
	if err := httpreq.BindAndValidate(c, &req); err != nil {
		return err
	}
	ticketNo := c.Param("ticket_no")
	output, opErr := h.tickets.Execute(c.Request().Context(), usecase.TicketCommandInput{
		TicketNo:   ticketNo,
		Command:    "appeal",
		Message:    req.Message,
		OperatorID: operatorID,
		RequestID:  requestctx.GetRequestID(c.Request().Context()),
	})
	if auditErr := h.audit.Record(c, "ticket_appeal", "ops_ticket", ticketNo, "乘客申诉，开启新一轮处理", opErr); auditErr != nil {
		return auditErr
	}
	if opErr != nil {
		return opErr
	}
	return httpresp.OK(c, output)
}
