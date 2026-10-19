// Package usecase coordinates operation and login audit workflows.
package usecase

import (
	"context"
	"time"

	"github.com/NSObjects/echo-admin/internal/modules/audit/domain"
	"github.com/NSObjects/echo-admin/internal/platform/pagination"
)

// Store persists operation and login logs.
type Store interface {
	RecordOperation(context.Context, domain.OperationLog) (domain.OperationLog, error)
	FindOperationLog(context.Context, int64) (domain.OperationLog, error)
	ListOperationLogs(context.Context, ListFilter) ([]domain.OperationLog, int, error)
	DeleteOperationLogs(context.Context, []int64) error
	RecordLogin(context.Context, domain.LoginLog) (domain.LoginLog, error)
	FindLoginLog(context.Context, int64) (domain.LoginLog, error)
	ListLoginLogs(context.Context, ListFilter) ([]domain.LoginLog, int, error)
	DeleteLoginLogs(context.Context, []int64) error
	RecordSystemError(context.Context, domain.SystemErrorLog) (domain.SystemErrorLog, error)
	FindSystemErrorLog(context.Context, int64) (domain.SystemErrorLog, error)
	ListSystemErrorLogs(context.Context, ListFilter) ([]domain.SystemErrorLog, int, error)
	UpdateSystemErrorLog(context.Context, domain.SystemErrorLog) (domain.SystemErrorLog, error)
	DeleteSystemErrorLogs(context.Context, []int64) error
}

// Usecase coordinates audit log rules.
type Usecase struct {
	store Store
}

// New creates an audit usecase.
func New(store Store) *Usecase {
	return &Usecase{store: store}
}

// OperationInput carries an operation log request from adapters.
type OperationInput struct {
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
}

// LoginInput carries a sign-in attempt audit request.
type LoginInput struct {
	AdminID   int64
	Username  string
	IP        string
	UserAgent string
	Success   bool
	Reason    string
}

// SystemErrorInput carries an internal API failure record from the server boundary.
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

// ResolveSystemErrorInput carries a resolution decision for one error log.
type ResolveSystemErrorInput struct {
	ID         int64
	ResolverID int64
	Note       string
}

// ListInput carries pagination for audit lists.
type ListInput struct {
	Page     int
	PageSize int
}

// ListFilter is the validated store-facing pagination window.
type ListFilter struct {
	pagination.Window
}

// OperationListOutput is a paginated operation log result.
type OperationListOutput struct {
	Items    []OperationLog
	Page     int
	PageSize int
	Total    int
}

// LoginListOutput is a paginated login log result.
type LoginListOutput struct {
	Items    []LoginLog
	Page     int
	PageSize int
	Total    int
}

// SystemErrorListOutput is a paginated system error log result.
type SystemErrorListOutput struct {
	Items    []SystemErrorLog
	Page     int
	PageSize int
	Total    int
}

// OperationLog is the adapter-facing operation log DTO.
type OperationLog struct {
	ID         int64     `json:"id"`
	ActorID    int64     `json:"actor_id"`
	Action     string    `json:"action"`
	Resource   string    `json:"resource"`
	ResourceID string    `json:"resource_id"`
	Method     string    `json:"method"`
	Path       string    `json:"path"`
	IP         string    `json:"ip"`
	UserAgent  string    `json:"user_agent"`
	Success    bool      `json:"success"`
	Message    string    `json:"message"`
	CreatedAt  time.Time `json:"created_at"`
}

// LoginLog is the adapter-facing login log DTO.
type LoginLog struct {
	ID        int64     `json:"id"`
	AdminID   int64     `json:"admin_id"`
	Username  string    `json:"username"`
	IP        string    `json:"ip"`
	UserAgent string    `json:"user_agent"`
	Success   bool      `json:"success"`
	Reason    string    `json:"reason"`
	CreatedAt time.Time `json:"created_at"`
}

// SystemErrorLog is the adapter-facing system error log DTO.
type SystemErrorLog struct {
	ID          int64      `json:"id"`
	Code        int        `json:"code"`
	Message     string     `json:"message"`
	Detail      string     `json:"detail"`
	Method      string     `json:"method"`
	Path        string     `json:"path"`
	IP          string     `json:"ip"`
	UserAgent   string     `json:"user_agent"`
	RequestID   string     `json:"request_id"`
	UserID      string     `json:"user_id"`
	Resolved    bool       `json:"resolved"`
	ResolveNote string     `json:"resolve_note"`
	ResolvedBy  int64      `json:"resolved_by"`
	ResolvedAt  *time.Time `json:"resolved_at,omitempty"`
	CreatedAt   time.Time  `json:"created_at"`
}
