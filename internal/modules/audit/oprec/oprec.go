// Package oprec turns HTTP management mutations into audit operation records.
// It lives in the audit module and is consumed by other modules' HTTP
// adapters, so audit semantics (including recording failed operations) have a
// single source of truth.
package oprec

import (
	"context"

	"github.com/labstack/echo/v5"

	auditusecase "github.com/NSObjects/echo-admin/internal/modules/audit/usecase"
	"github.com/NSObjects/echo-admin/internal/platform/infrastructure/logging"
	"github.com/NSObjects/echo-admin/internal/platform/requestctx"
)

// Audit persists operation records; it is satisfied by the audit usecase.
type Audit interface {
	RecordOperation(context.Context, auditusecase.OperationInput) (auditusecase.OperationLog, error)
}

// Recorder records management operations for audit.
type Recorder struct {
	audit Audit
}

// New creates an operation recorder backed by audit.
func New(audit Audit) *Recorder {
	return &Recorder{audit: audit}
}

// Record fills the actor and request fields from the Echo context and records
// one management operation. A non-nil opErr marks the operation failed, and
// failed operations are recorded too. Record returns opErr when the operation
// failed (an audit failure is logged instead of masking it) and the audit
// error otherwise, so handlers call it on both paths:
//
//	result, opErr := h.usecase.DoSomething(ctx, input)
//	if err := h.audit.Record(c, "do", "thing", id, "did something", opErr); err != nil {
//		return err
//	}
//	if opErr != nil {
//		return opErr
//	}
func (r *Recorder) Record(c *echo.Context, action, resource, resourceID, message string, opErr error) error {
	ctx := c.Request().Context()
	actorID, err := requestctx.RequireUserID(ctx)
	if err != nil {
		return err
	}
	_, err = r.audit.RecordOperation(ctx, auditusecase.OperationInput{
		ActorID:    actorID,
		Action:     action,
		Resource:   resource,
		ResourceID: resourceID,
		Method:     c.Request().Method,
		Path:       c.Path(),
		IP:         c.RealIP(),
		UserAgent:  c.Request().UserAgent(),
		Success:    opErr == nil,
		Message:    message,
	})
	if err != nil {
		if opErr != nil {
			logging.FromContext(ctx).Warn().Err(err).Str("action", action).Str("resource", resource).Msg("record failed operation for audit")
			return opErr
		}
		return err
	}
	return opErr
}
