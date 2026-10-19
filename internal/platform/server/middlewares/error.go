package middlewares

import (
	"context"
	"fmt"
	"net/http"
	"runtime/debug"
	"strconv"

	"github.com/labstack/echo/v5"

	"github.com/NSObjects/echo-admin/internal/platform/apperr"
	"github.com/NSObjects/echo-admin/internal/platform/infrastructure/logging"
	"github.com/NSObjects/echo-admin/internal/platform/requestctx"
	"github.com/NSObjects/echo-admin/internal/platform/server/httpresp"
)

// SystemErrorInput carries one internal API failure to a persistence adapter.
type SystemErrorInput struct {
	Code      int
	Message   string
	Detail    string
	Method    string
	Path      string
	IP        string
	UserAgent string
	RequestID string
	UserID    string
}

// SystemErrorRecorder stores internal API failure diagnostics outside the server.
type SystemErrorRecorder interface {
	RecordSystemError(context.Context, SystemErrorInput) error
}

// ErrorHandlerWithRecorder renders API errors and records internal failures.
func ErrorHandlerWithRecorder(recorder SystemErrorRecorder) echo.HTTPErrorHandler {
	return func(c *echo.Context, err error) {
		handleError(c, err, recorder)
	}
}

func handleError(c *echo.Context, err error, recorder SystemErrorRecorder) {
	if response, unwrapErr := echo.UnwrapResponse(c.Response()); unwrapErr == nil && response.Committed {
		return
	}

	normalized := normalizeError(err)
	info := apperr.NewInfo(normalized)
	logAPIError(c, info)
	recordSystemError(c, info, recorder)
	if renderErr := httpresp.APIError(c, normalized); renderErr != nil {
		logging.FromContext(c.Request().Context()).
			Error().
			Err(renderErr).
			Str("request_id", httpresp.RequestID(c)).
			Msg("API error render failed")
	}
}

func normalizeError(err error) error {
	if err == nil {
		return apperr.New(apperr.ErrInternalServer, "internal server error")
	}

	if appErr, ok := apperr.Parse(err); ok && appErr != nil {
		return err
	}

	if status := echo.StatusCode(err); status != 0 {
		return normalizeHTTPStatus(status, err)
	}

	return apperr.WrapInternal(err, "internal server error")
}

func normalizeHTTPStatus(status int, err error) error {
	switch status {
	case http.StatusBadRequest:
		return apperr.WrapBadRequest(err, "")
	case http.StatusUnauthorized:
		return apperr.WrapUnauthorized(err, "")
	case http.StatusForbidden:
		return apperr.WrapForbidden(err, "")
	case http.StatusNotFound:
		return apperr.WrapNotFound(err, "")
	case http.StatusMethodNotAllowed:
		return apperr.WrapMethodNotAllowed(err, "")
	case http.StatusConflict:
		return apperr.WrapConflict(err, "")
	default:
		if status >= http.StatusBadRequest && status < http.StatusInternalServerError {
			return apperr.WrapBadRequest(err, "")
		}
		return apperr.WrapInternal(err, "")
	}
}

func logAPIError(c *echo.Context, info apperr.Info) {
	logger := logging.FromContext(c.Request().Context())
	event := logger.Warn()
	message := "API business error"
	if info.IsInternal() {
		event = logger.Error()
		message = "API internal error"
	}
	event = event.
		Int("code", info.Code).
		Str("message", info.Message).
		Str("category", string(info.Category)).
		Str("request_id", httpresp.RequestID(c)).
		Str("method", c.Request().Method).
		Str("path", requestPath(c)).
		Str("user_agent", c.Request().UserAgent())
	if info.Detail != "" {
		event = event.Str("detail", info.Detail)
	}
	event.Msg(message)
}

func recordSystemError(c *echo.Context, info apperr.Info, recorder SystemErrorRecorder) {
	if recorder == nil || !info.IsInternal() {
		return
	}
	input := SystemErrorInput{
		Code:      info.Code,
		Message:   info.Message,
		Detail:    info.Detail,
		Method:    c.Request().Method,
		Path:      requestPath(c),
		IP:        c.RealIP(),
		UserAgent: c.Request().UserAgent(),
		RequestID: httpresp.RequestID(c),
		// The system-error log keeps user_id as a JSON string contract, so the
		// int64 context identity is formatted here and stays empty for
		// unauthenticated requests.
		UserID: formatUserID(requestctx.GetUserID(c.Request().Context())),
	}
	if err := recorder.RecordSystemError(c.Request().Context(), input); err != nil {
		logging.FromContext(c.Request().Context()).
			Error().
			Err(err).
			Str("request_id", input.RequestID).
			Msg("system error record failed")
	}
}

// errorRecovery recovers panics at the HTTP boundary.
func errorRecovery() echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c *echo.Context) error {
			defer func() {
				if r := recover(); r != nil {
					err := apperr.WrapInternal(
						fmt.Errorf("panic recovered: %v\n%s", r, debug.Stack()),
						"internal server error",
					)
					c.Echo().HTTPErrorHandler(c, err)
				}
			}()

			return next(c)
		}
	}
}

func formatUserID(id int64) string {
	if id <= 0 {
		return ""
	}
	return strconv.FormatInt(id, 10)
}
