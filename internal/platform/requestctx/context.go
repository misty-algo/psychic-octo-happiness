// Package requestctx carries request-scoped metadata without depending on an
// HTTP framework.
package requestctx

import (
	"context"

	"github.com/NSObjects/echo-admin/internal/platform/apperr"
)

type contextKey struct{}

const (
	maxMetadataIDLength  = 128
	minVisibleASCIIValue = 33
	maxVisibleASCIIValue = 126
)

// Info is metadata extracted at a delivery boundary and passed through
// request-scoped work. Identity fields are int64 database keys written once by
// authentication middlewares; zero means the request carries no such identity.
// Trace and span IDs stay strings because they are external metadata, not
// owned keys.
type Info struct {
	TraceID        string
	SpanID         string
	RequestID      string
	UserID         int64
	RoleID         int64
	LoginSessionID int64
}

// WithInfo returns a child context carrying request metadata.
func WithInfo(ctx context.Context, info Info) context.Context {
	return context.WithValue(ctx, contextKey{}, info)
}

// CleanMetadataID returns value only when it is short visible ASCII metadata.
func CleanMetadataID(value string) string {
	if value == "" || len(value) > maxMetadataIDLength {
		return ""
	}
	for _, r := range value {
		if r < minVisibleASCIIValue || r > maxVisibleASCIIValue {
			return ""
		}
	}
	return value
}

// WithTraceSpan returns a child context with updated trace metadata while
// preserving existing request metadata.
func WithTraceSpan(ctx context.Context, traceID, spanID string) context.Context {
	info, _ := FromContext(ctx)
	info.TraceID = CleanMetadataID(traceID)
	info.SpanID = CleanMetadataID(spanID)
	return WithInfo(ctx, info)
}

// WithUserID returns a child context carrying authenticated user identity.
func WithUserID(ctx context.Context, userID int64) context.Context {
	info, _ := FromContext(ctx)
	info.UserID = userID
	return WithInfo(ctx, info)
}

// WithRoleID returns a child context carrying the active authenticated role.
func WithRoleID(ctx context.Context, roleID int64) context.Context {
	info, _ := FromContext(ctx)
	info.RoleID = roleID
	return WithInfo(ctx, info)
}

// WithLoginSessionID returns a child context carrying the authenticated login
// session identity. API-token requests intentionally leave this field zero.
func WithLoginSessionID(ctx context.Context, sessionID int64) context.Context {
	info, _ := FromContext(ctx)
	info.LoginSessionID = sessionID
	return WithInfo(ctx, info)
}

// FromContext returns request metadata from ctx.
func FromContext(ctx context.Context) (Info, bool) {
	if ctx == nil {
		return Info{}, false
	}
	info, ok := ctx.Value(contextKey{}).(Info)
	return info, ok
}

// GetRequestID returns the request ID stored in ctx.
func GetRequestID(ctx context.Context) string {
	info, ok := FromContext(ctx)
	if !ok {
		return ""
	}
	return info.RequestID
}

// GetUserID returns the user ID stored in ctx; zero when absent.
func GetUserID(ctx context.Context) int64 {
	info, ok := FromContext(ctx)
	if !ok {
		return 0
	}
	return info.UserID
}

// GetRoleID returns the active role ID stored in ctx; zero when absent.
func GetRoleID(ctx context.Context) int64 {
	info, ok := FromContext(ctx)
	if !ok {
		return 0
	}
	return info.RoleID
}

// GetLoginSessionID returns the login session ID stored in ctx; zero when
// absent.
func GetLoginSessionID(ctx context.Context) int64 {
	info, ok := FromContext(ctx)
	if !ok {
		return 0
	}
	return info.LoginSessionID
}

// RequireUserID returns the authenticated user ID. It is the single rule that
// decides when a request carries no valid administrator identity: a missing,
// zero, or negative ID is always an authentication failure, never a
// not-found or business error.
func RequireUserID(ctx context.Context) (int64, error) {
	if id := GetUserID(ctx); id > 0 {
		return id, nil
	}
	return 0, apperr.NewUnauthorized()
}

// RequireRoleID returns the active role ID or an authentication failure when
// the request carries no valid active role.
func RequireRoleID(ctx context.Context) (int64, error) {
	if id := GetRoleID(ctx); id > 0 {
		return id, nil
	}
	return 0, apperr.NewUnauthorized()
}

// RequireLoginSessionID returns the authenticated login session ID or an
// authentication failure when the request carries no valid login session.
func RequireLoginSessionID(ctx context.Context) (int64, error) {
	if id := GetLoginSessionID(ctx); id > 0 {
		return id, nil
	}
	return 0, apperr.NewUnauthorized()
}
