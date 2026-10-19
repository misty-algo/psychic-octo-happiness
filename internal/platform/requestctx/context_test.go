package requestctx

import (
	"context"
	"strings"
	"testing"

	"github.com/NSObjects/echo-admin/internal/platform/apperr"
)

const (
	requestIDForTest = "req-789"
	traceIDForTest   = "trace-123"
)

const (
	userIDForTest    = int64(42)
	roleIDForTest    = int64(7)
	sessionIDForTest = int64(1001)
)

func TestWithInfoStoresRequestMetadata(t *testing.T) {
	ctx := WithInfo(context.Background(), Info{
		TraceID:        traceIDForTest,
		SpanID:         "span-456",
		RequestID:      requestIDForTest,
		UserID:         userIDForTest,
		RoleID:         roleIDForTest,
		LoginSessionID: sessionIDForTest,
	})

	info, ok := FromContext(ctx)
	if !ok {
		t.Fatal("FromContext() ok = false, want true")
	}
	if info.TraceID != traceIDForTest {
		t.Fatalf("TraceID = %q, want %s", info.TraceID, traceIDForTest)
	}
	if info.SpanID != "span-456" {
		t.Fatalf("SpanID = %q, want span-456", info.SpanID)
	}
	if info.RequestID != requestIDForTest {
		t.Fatalf("RequestID = %q, want %s", info.RequestID, requestIDForTest)
	}
	if info.UserID != userIDForTest {
		t.Fatalf("UserID = %d, want %d", info.UserID, userIDForTest)
	}
	if info.RoleID != roleIDForTest {
		t.Fatalf("RoleID = %d, want %d", info.RoleID, roleIDForTest)
	}
	if info.LoginSessionID != sessionIDForTest {
		t.Fatalf("LoginSessionID = %d, want %d", info.LoginSessionID, sessionIDForTest)
	}
}

func TestFromContextWithoutMetadata(t *testing.T) {
	info, ok := FromContext(context.Background())
	if ok {
		t.Fatal("FromContext() ok = true, want false")
	}
	if info != (Info{}) {
		t.Fatalf("Info = %+v, want zero value", info)
	}
}

func TestWithTraceSpanPreservesExistingMetadata(t *testing.T) {
	ctx := WithInfo(context.Background(), Info{
		RequestID:      requestIDForTest,
		UserID:         userIDForTest,
		LoginSessionID: sessionIDForTest,
	})

	ctx = WithTraceSpan(ctx, traceIDForTest, "span-456")

	info, ok := FromContext(ctx)
	if !ok {
		t.Fatal("FromContext() ok = false, want true")
	}
	if info.RequestID != requestIDForTest {
		t.Fatalf("RequestID = %q, want %s", info.RequestID, requestIDForTest)
	}
	if info.UserID != userIDForTest {
		t.Fatalf("UserID = %d, want %d", info.UserID, userIDForTest)
	}
	if info.LoginSessionID != sessionIDForTest {
		t.Fatalf("LoginSessionID = %d, want %d", info.LoginSessionID, sessionIDForTest)
	}
	if info.TraceID != traceIDForTest {
		t.Fatalf("TraceID = %q, want %s", info.TraceID, traceIDForTest)
	}
	if info.SpanID != "span-456" {
		t.Fatalf("SpanID = %q, want span-456", info.SpanID)
	}
}

func TestCleanMetadataID(t *testing.T) {
	tests := []struct {
		name  string
		value string
		want  string
	}{
		{name: "empty", value: "", want: ""},
		{name: "visible ascii", value: "req-123", want: "req-123"},
		{name: "space is rejected", value: "req 123", want: ""},
		{name: "non ascii is rejected", value: "请求", want: ""},
		{name: "overlong is rejected", value: strings.Repeat("a", 129), want: ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := CleanMetadataID(tt.value); got != tt.want {
				t.Fatalf("CleanMetadataID() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestWithUserIDAddsAuthenticatedIdentity(t *testing.T) {
	ctx := WithInfo(context.Background(), Info{
		TraceID:   traceIDForTest,
		RequestID: "req-789",
	})

	ctx = WithUserID(ctx, userIDForTest)

	info, ok := FromContext(ctx)
	if !ok {
		t.Fatal("FromContext() ok = false, want true")
	}
	if info.TraceID != traceIDForTest {
		t.Fatalf("TraceID = %q, want %s", info.TraceID, traceIDForTest)
	}
	if info.UserID != userIDForTest {
		t.Fatalf("UserID = %d, want %d", info.UserID, userIDForTest)
	}
}

func TestWithRoleIDAddsActiveAuthenticatedRole(t *testing.T) {
	ctx := WithInfo(context.Background(), Info{
		TraceID:   traceIDForTest,
		RequestID: requestIDForTest,
		UserID:    userIDForTest,
	})

	ctx = WithRoleID(ctx, roleIDForTest)

	info, ok := FromContext(ctx)
	if !ok {
		t.Fatal("FromContext() ok = false, want true")
	}
	if info.UserID != userIDForTest {
		t.Fatalf("UserID = %d, want %d", info.UserID, userIDForTest)
	}
	if info.RoleID != roleIDForTest {
		t.Fatalf("RoleID = %d, want %d", info.RoleID, roleIDForTest)
	}
	if got := GetRoleID(ctx); got != roleIDForTest {
		t.Fatalf("GetRoleID() = %d, want %d", got, roleIDForTest)
	}
}

func TestWithLoginSessionIDAddsSessionMetadata(t *testing.T) {
	ctx := WithUserID(context.Background(), userIDForTest)
	ctx = WithLoginSessionID(ctx, sessionIDForTest)

	info, ok := FromContext(ctx)
	if !ok {
		t.Fatal("FromContext() ok = false, want true")
	}
	if info.UserID != userIDForTest {
		t.Fatalf("UserID = %d, want %d", info.UserID, userIDForTest)
	}
	if got := GetLoginSessionID(ctx); got != sessionIDForTest {
		t.Fatalf("GetLoginSessionID() = %d, want %d", got, sessionIDForTest)
	}
}

func TestContextAccessorsHandleNilContext(t *testing.T) {
	var ctx context.Context
	if got := GetRequestID(ctx); got != "" {
		t.Fatalf("GetRequestID(nil) = %q, want empty", got)
	}
	if got := GetUserID(ctx); got != 0 {
		t.Fatalf("GetUserID(nil) = %d, want 0", got)
	}
	if got := GetRoleID(ctx); got != 0 {
		t.Fatalf("GetRoleID(nil) = %d, want 0", got)
	}
	if got := GetLoginSessionID(ctx); got != 0 {
		t.Fatalf("GetLoginSessionID(nil) = %d, want 0", got)
	}
}

func TestRequireIdentityRejectsInvalidValues(t *testing.T) {
	validCtx := WithUserID(WithRoleID(WithLoginSessionID(context.Background(), sessionIDForTest), roleIDForTest), userIDForTest)
	tests := []struct {
		name    string
		ctx     context.Context
		wantErr bool
	}{
		{name: "valid identity", ctx: validCtx},
		{name: "missing identity", ctx: context.Background(), wantErr: true},
		{name: "zero identity", ctx: WithUserID(context.Background(), 0), wantErr: true},
		{name: "negative identity", ctx: WithUserID(context.Background(), -1), wantErr: true},
	}

	getters := []struct {
		name     string
		call     func(context.Context) (int64, error)
		validGot int64
	}{
		{name: "user", call: RequireUserID, validGot: userIDForTest},
		{name: "role", call: RequireRoleID, validGot: roleIDForTest},
		{name: "login session", call: RequireLoginSessionID, validGot: sessionIDForTest},
	}

	for _, getter := range getters {
		t.Run(getter.name, func(t *testing.T) {
			for _, tt := range tests {
				t.Run(tt.name, func(t *testing.T) {
					got, err := getter.call(tt.ctx)
					if tt.wantErr {
						if err == nil {
							t.Fatalf("error = nil, want unauthorized")
						}
						if info := apperr.NewInfo(err); info.Kind != apperr.KindUnauthorized {
							t.Fatalf("kind = %s, want %s", info.Kind, apperr.KindUnauthorized)
						}
						return
					}
					if err != nil {
						t.Fatalf("error = %v, want nil", err)
					}
					if got != getter.validGot {
						t.Fatalf("got = %d, want %d", got, getter.validGot)
					}
				})
			}
		})
	}
}

func TestRequireIdentityErrorsAreUnauthorized(t *testing.T) {
	_, err := RequireUserID(context.Background())
	if err == nil {
		t.Fatal("RequireUserID() error = nil, want unauthorized")
	}
	if code := apperr.NewInfo(err).Code; code != apperr.ErrUnauthorized {
		t.Fatalf("code = %d, want %d", code, apperr.ErrUnauthorized)
	}
}
