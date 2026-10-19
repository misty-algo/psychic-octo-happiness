// Package httpresp renders framework-specific HTTP error responses.
package httpresp

import (
	"errors"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/labstack/echo/v5"

	"github.com/NSObjects/echo-admin/internal/platform/apperr"
	"github.com/NSObjects/echo-admin/internal/platform/requestctx"
)

// ErrorResponse is the standard JSON error response for HTTP adapters.
type ErrorResponse struct {
	Code      int    `json:"code"`
	Message   string `json:"message"`
	RequestID string `json:"request_id"`
	Timestamp int64  `json:"timestamp"`
}

// Response is the standard JSON success response for HTTP adapters.
type Response struct {
	Code      int         `json:"code"`
	Message   string      `json:"message"`
	Data      interface{} `json:"data,omitempty"`
	RequestID string      `json:"request_id"`
	Timestamp int64       `json:"timestamp"`
}

// PageMeta describes paginated list response metadata.
type PageMeta struct {
	Page     int  `json:"page"`
	PageSize int  `json:"page_size"`
	Total    int  `json:"total"`
	HasNext  bool `json:"has_next"`
}

// ListResponse is the standard JSON list response for HTTP adapters.
type ListResponse struct {
	Code      int         `json:"code"`
	Message   string      `json:"message"`
	Data      interface{} `json:"data"`
	Page      PageMeta    `json:"page"`
	RequestID string      `json:"request_id"`
	Timestamp int64       `json:"timestamp"`
}

// OK renders a successful response envelope.
func OK(c *echo.Context, data interface{}) error {
	return c.JSON(http.StatusOK, Response{
		Code:      apperr.ErrSuccess,
		Message:   "OK",
		Data:      data,
		RequestID: RequestID(c),
		Timestamp: time.Now().Unix(),
	})
}

// Created renders a successful creation response envelope.
func Created(c *echo.Context, data interface{}) error {
	return c.JSON(http.StatusCreated, Response{
		Code:      apperr.ErrSuccess,
		Message:   "OK",
		Data:      data,
		RequestID: RequestID(c),
		Timestamp: time.Now().Unix(),
	})
}

// pageMeta builds pagination metadata from values already normalized by
// pagination.Normalize; it validates nothing and its HasNext computation
// stays overflow-safe for any row count.
func pageMeta(page, pageSize, total int) PageMeta {
	return PageMeta{
		Page:     page,
		PageSize: pageSize,
		Total:    total,
		HasNext:  (total+pageSize-1)/pageSize > page,
	}
}

// list renders a successful paginated list response envelope.
func list(c *echo.Context, data interface{}, page PageMeta) error {
	return c.JSON(http.StatusOK, ListResponse{
		Code:      apperr.ErrSuccess,
		Message:   "OK",
		Data:      data,
		Page:      page,
		RequestID: RequestID(c),
		Timestamp: time.Now().Unix(),
	})
}

// APIError renders a project error as a JSON HTTP response.
func APIError(c *echo.Context, err error) error {
	if err == nil {
		return errors.New("error cannot be nil")
	}
	if response, unwrapErr := echo.UnwrapResponse(c.Response()); unwrapErr == nil && response.Committed {
		return nil
	}

	info := apperr.NewInfo(err)
	rjson := ErrorResponse{
		Code:      info.Code,
		Message:   info.Message,
		RequestID: RequestID(c),
		Timestamp: time.Now().Unix(),
	}

	return c.JSON(status(info.Kind), rjson)
}

// status maps framework-free application error kinds to HTTP status codes.
// It is the single mapping of its kind: apperr itself stays free of HTTP
// concerns, and every error response flows through APIError.
func status(kind apperr.Kind) int {
	switch kind {
	case apperr.KindOK:
		return http.StatusOK
	case apperr.KindBadRequest, apperr.KindValidation:
		return http.StatusBadRequest
	case apperr.KindUnauthorized:
		return http.StatusUnauthorized
	case apperr.KindForbidden:
		return http.StatusForbidden
	case apperr.KindNotFound:
		return http.StatusNotFound
	case apperr.KindMethodNotAllowed:
		return http.StatusMethodNotAllowed
	case apperr.KindConflict:
		return http.StatusConflict
	default:
		return http.StatusInternalServerError
	}
}

// RequestID returns the request ID used in HTTP error responses.
func RequestID(c *echo.Context) string {
	if requestID := requestctx.GetRequestID(c.Request().Context()); requestID != "" {
		setResponseRequestID(c, requestID)
		return requestID
	}

	if requestID := requestctx.CleanMetadataID(c.Request().Header.Get("X-Request-ID")); requestID != "" {
		setResponseRequestID(c, requestID)
		return requestID
	}

	if requestID := requestctx.CleanMetadataID(c.Response().Header().Get("X-Request-ID")); requestID != "" {
		return requestID
	}

	requestID := generateRequestID()
	setResponseRequestID(c, requestID)
	return requestID
}

func setResponseRequestID(c *echo.Context, requestID string) {
	c.Response().Header().Set("X-Request-ID", requestID)
}

func generateRequestID() string {
	return uuid.NewString()
}

// Paginated renders a successful paginated list response envelope.
func Paginated(c *echo.Context, data interface{}, page, pageSize, total int) error {
	return list(c, data, pageMeta(page, pageSize, total))
}

type deletedIDsData struct {
	IDs []int64 `json:"ids"`
}

// DeletedIDs renders a successful batch-delete response echoing the IDs.
func DeletedIDs(c *echo.Context, ids []int64) error {
	return OK(c, deletedIDsData{IDs: ids})
}
