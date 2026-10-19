// Package domain contains audit log business rules.
package domain

import (
	"errors"
	"strings"
	"time"
)

const (
	maxMethodLength    = 16
	maxPathLength      = 255
	maxIPLength        = 64
	maxUserAgentLength = 255
	maxMessageLength   = 255
	maxReasonLength    = 255
	maxRequestIDLength = 128
	maxUserIDLength    = 64
	maxDetailLength    = 8000
	maxResolveNoteLen  = 1000
)

// Audit validation errors.
var (
	ErrInvalidLogID         = errors.New("invalid log id")
	ErrInvalidAction        = errors.New("invalid action")
	ErrInvalidResource      = errors.New("invalid resource")
	ErrInvalidLoginUsername = errors.New("invalid login username")
	ErrInvalidErrorCode     = errors.New("invalid error code")
	ErrInvalidErrorMessage  = errors.New("invalid error message")
	ErrInvalidResolver      = errors.New("invalid error resolver")
)

// OperationLog is a validated record of one state-changing back-office operation.
type OperationLog struct {
	ID         int64
	ActorID    int64
	Action     string
	Resource   string
	ResourceID string
	Method     string
	Path       string
	IP         string
	UserAgent  string
	Success    bool
	Message    string
	CreatedAt  time.Time
}

// RestoreOperationLog rebuilds an operation log from a trusted store representation.
func RestoreOperationLog(id, actorID int64, action, resource, resourceID, method, path, ip, userAgent string, success bool, message string, createdAt time.Time) (OperationLog, error) {
	action = normalizeToken(action)
	resource = normalizeToken(resource)
	if id < 0 {
		return OperationLog{}, ErrInvalidLogID
	}
	if action == "" || len(action) > 80 {
		return OperationLog{}, ErrInvalidAction
	}
	if resource == "" || len(resource) > 80 {
		return OperationLog{}, ErrInvalidResource
	}
	return OperationLog{
		ID:         id,
		ActorID:    actorID,
		Action:     action,
		Resource:   resource,
		ResourceID: strings.TrimSpace(resourceID),
		Method:     limitString(method, maxMethodLength),
		Path:       limitString(path, maxPathLength),
		IP:         limitString(ip, maxIPLength),
		UserAgent:  limitString(userAgent, maxUserAgentLength),
		Success:    success,
		Message:    limitString(message, maxMessageLength),
		CreatedAt:  createdAt,
	}, nil
}

// LoginLog is a validated record of one sign-in attempt.
type LoginLog struct {
	ID        int64
	AdminID   int64
	Username  string
	IP        string
	UserAgent string
	Success   bool
	Reason    string
	CreatedAt time.Time
}

// RestoreLoginLog rebuilds a login log from a trusted store representation.
func RestoreLoginLog(id, adminID int64, username, ip, userAgent string, success bool, reason string, createdAt time.Time) (LoginLog, error) {
	username = normalizeToken(username)
	if id < 0 {
		return LoginLog{}, ErrInvalidLogID
	}
	if username == "" || len(username) > 64 {
		return LoginLog{}, ErrInvalidLoginUsername
	}
	return LoginLog{
		ID:        id,
		AdminID:   adminID,
		Username:  username,
		IP:        limitString(ip, maxIPLength),
		UserAgent: limitString(userAgent, maxUserAgentLength),
		Success:   success,
		Reason:    limitString(reason, maxReasonLength),
		CreatedAt: createdAt,
	}, nil
}

// SystemErrorLog is a validated diagnostic record for one internal API failure.
type SystemErrorLog struct {
	ID          int64
	Code        int
	Message     string
	Detail      string
	Method      string
	Path        string
	IP          string
	UserAgent   string
	RequestID   string
	UserID      string
	Resolved    bool
	ResolveNote string
	ResolvedBy  int64
	ResolvedAt  time.Time
	CreatedAt   time.Time
}

// RestoreSystemErrorLog rebuilds a system error log from a trusted representation.
func RestoreSystemErrorLog(id int64, code int, message, detail, method, path, ip, userAgent, requestID, userID string, resolved bool, resolveNote string, resolvedBy int64, resolvedAt, createdAt time.Time) (SystemErrorLog, error) {
	message = limitString(message, maxMessageLength)
	resolveNote = limitString(resolveNote, maxResolveNoteLen)
	if id < 0 {
		return SystemErrorLog{}, ErrInvalidLogID
	}
	if code <= 0 {
		return SystemErrorLog{}, ErrInvalidErrorCode
	}
	if message == "" {
		return SystemErrorLog{}, ErrInvalidErrorMessage
	}
	if resolved {
		if resolvedBy <= 0 {
			return SystemErrorLog{}, ErrInvalidResolver
		}
	} else {
		resolveNote = ""
		resolvedBy = 0
		resolvedAt = time.Time{}
	}
	return SystemErrorLog{
		ID:          id,
		Code:        code,
		Message:     message,
		Detail:      limitString(detail, maxDetailLength),
		Method:      limitString(method, maxMethodLength),
		Path:        limitString(path, maxPathLength),
		IP:          limitString(ip, maxIPLength),
		UserAgent:   limitString(userAgent, maxUserAgentLength),
		RequestID:   limitString(requestID, maxRequestIDLength),
		UserID:      limitString(userID, maxUserIDLength),
		Resolved:    resolved,
		ResolveNote: resolveNote,
		ResolvedBy:  resolvedBy,
		ResolvedAt:  resolvedAt,
		CreatedAt:   createdAt,
	}, nil
}

// Resolve marks an internal error as handled by an administrator.
func (l SystemErrorLog) Resolve(note string, resolverID int64, resolvedAt time.Time) (SystemErrorLog, error) {
	return RestoreSystemErrorLog(l.ID, l.Code, l.Message, l.Detail, l.Method, l.Path, l.IP, l.UserAgent, l.RequestID, l.UserID, true, note, resolverID, resolvedAt, l.CreatedAt)
}

// Reopen clears the handled state when a previous resolution was wrong.
func (l SystemErrorLog) Reopen() (SystemErrorLog, error) {
	return RestoreSystemErrorLog(l.ID, l.Code, l.Message, l.Detail, l.Method, l.Path, l.IP, l.UserAgent, l.RequestID, l.UserID, false, "", 0, time.Time{}, l.CreatedAt)
}

func normalizeToken(value string) string {
	return strings.ToLower(strings.TrimSpace(value))
}

func limitString(value string, maxLength int) string {
	value = strings.TrimSpace(value)
	if len(value) <= maxLength {
		return value
	}
	return value[:maxLength]
}
