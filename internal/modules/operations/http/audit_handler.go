package operationshttp

import (
	"strconv"
	"strings"

	"github.com/labstack/echo/v5"

	"github.com/NSObjects/echo-admin/internal/modules/operations/usecase"
	"github.com/NSObjects/echo-admin/internal/platform/apperr"
	"github.com/NSObjects/echo-admin/internal/platform/requestctx"
	"github.com/NSObjects/echo-admin/internal/platform/server/httpreq"
	"github.com/NSObjects/echo-admin/internal/platform/server/httpresp"
)

// ListDriverAudits returns one page of the driver/vehicle audit work queue.
func (h *Handler) ListDriverAudits(c *echo.Context) error {
	page, pageSize, err := httpreq.Pagination(c)
	if err != nil {
		return err
	}
	output, err := h.audits.ListAudits(c.Request().Context(), usecase.ListAuditsInput{
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

// DriverAuditHistory returns the full version chain of one application.
func (h *Handler) DriverAuditHistory(c *echo.Context) error {
	output, err := h.audits.History(c.Request().Context(), c.Param("application_id"))
	if err != nil {
		return err
	}
	return httpresp.OK(c, output)
}

// ClaimDriverAudit takes one revision into the operator's queue.
func (h *Handler) ClaimDriverAudit(c *echo.Context) error {
	operatorID, err := requestctx.RequireUserID(c.Request().Context())
	if err != nil {
		return err
	}
	revision, err := pathRevision(c)
	if err != nil {
		return err
	}
	applicationID := c.Param("application_id")
	item, opErr := h.audits.Claim(c.Request().Context(), usecase.ClaimInput{
		ApplicationID: applicationID,
		Revision:      revision,
		OperatorID:    operatorID,
		RequestID:     requestctx.GetRequestID(c.Request().Context()),
	})
	if auditErr := h.audit.Record(c, "claim_driver_audit", "ops_driver_audit", applicationID, "领取审核版本", opErr); auditErr != nil {
		return auditErr
	}
	if opErr != nil {
		return opErr
	}
	return httpresp.OK(c, item)
}

// DecideDriverAudit approves or rejects one claimed revision.
func (h *Handler) DecideDriverAudit(c *echo.Context) error {
	operatorID, err := requestctx.RequireUserID(c.Request().Context())
	if err != nil {
		return err
	}
	revision, err := pathRevision(c)
	if err != nil {
		return err
	}
	var req decideAuditRequest
	if err := httpreq.BindAndValidate(c, &req); err != nil {
		return err
	}
	applicationID := c.Param("application_id")
	item, opErr := h.audits.Decide(c.Request().Context(), usecase.DecideInput{
		ApplicationID: applicationID,
		Revision:      revision,
		Action:        req.Action,
		Reason:        req.Reason,
		Version:       req.Version,
		OperatorID:    operatorID,
		RequestID:     requestctx.GetRequestID(c.Request().Context()),
	})
	detail := "审核裁决"
	if opErr == nil {
		detail = "审核裁决：" + item.State
	}
	if auditErr := h.audit.Record(c, "decide_driver_audit", "ops_driver_audit", applicationID, detail, opErr); auditErr != nil {
		return auditErr
	}
	if opErr != nil {
		return opErr
	}
	return httpresp.OK(c, item)
}

// pathRevision parses the positive revision path parameter.
func pathRevision(c *echo.Context) (int, error) {
	raw := strings.TrimSpace(c.Param("revision"))
	revision, err := strconv.Atoi(raw)
	if err != nil || revision <= 0 {
		return 0, apperr.NewBadRequest("版本号必须为正整数")
	}
	return revision, nil
}
