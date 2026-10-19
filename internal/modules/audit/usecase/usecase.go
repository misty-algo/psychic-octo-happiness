package usecase

import (
	"context"
	"errors"
	"time"

	"github.com/NSObjects/echo-admin/internal/modules/audit/domain"
	"github.com/NSObjects/echo-admin/internal/platform/apperr"
	"github.com/NSObjects/echo-admin/internal/platform/pagination"
)

// RecordOperation stores one operation log.
func (u *Usecase) RecordOperation(ctx context.Context, input OperationInput) (OperationLog, error) {
	if err := u.ready(); err != nil {
		return OperationLog{}, err
	}
	log, err := domain.RestoreOperationLog(0, input.ActorID, input.Action, input.Resource, input.ResourceID, input.Method, input.Path, input.IP, input.UserAgent, input.Success, input.Message, time.Time{})
	if err != nil {
		return OperationLog{}, mapDomainError(err)
	}
	created, err := u.store.RecordOperation(ctx, log)
	if err != nil {
		return OperationLog{}, err
	}
	return fromOperationLog(created), nil
}

// RecordLogin stores one login attempt log.
func (u *Usecase) RecordLogin(ctx context.Context, input LoginInput) (LoginLog, error) {
	if err := u.ready(); err != nil {
		return LoginLog{}, err
	}
	username := input.Username
	if username == "" {
		username = "unknown"
	}
	log, err := domain.RestoreLoginLog(0, input.AdminID, username, input.IP, input.UserAgent, input.Success, input.Reason, time.Time{})
	if err != nil {
		return LoginLog{}, mapDomainError(err)
	}
	created, err := u.store.RecordLogin(ctx, log)
	if err != nil {
		return LoginLog{}, err
	}
	return fromLoginLog(created), nil
}

// RecordSystemError stores one internal API failure log.
func (u *Usecase) RecordSystemError(ctx context.Context, input SystemErrorInput) (SystemErrorLog, error) {
	if err := u.ready(); err != nil {
		return SystemErrorLog{}, err
	}
	log, err := domain.RestoreSystemErrorLog(0, input.Code, input.Message, input.Detail, input.Method, input.Path, input.IP, input.UserAgent, input.RequestID, input.UserID, false, "", 0, time.Time{}, time.Time{})
	if err != nil {
		return SystemErrorLog{}, mapDomainError(err)
	}
	created, err := u.store.RecordSystemError(ctx, log)
	if err != nil {
		return SystemErrorLog{}, err
	}
	return fromSystemErrorLog(created), nil
}

// ListOperationLogs returns paginated operation logs.
func (u *Usecase) ListOperationLogs(ctx context.Context, input ListInput) (OperationListOutput, error) {
	if err := u.ready(); err != nil {
		return OperationListOutput{}, err
	}
	filter, err := normalizeListInput(input)
	if err != nil {
		return OperationListOutput{}, err
	}
	logs, total, err := u.store.ListOperationLogs(ctx, filter)
	if err != nil {
		return OperationListOutput{}, err
	}
	return OperationListOutput{
		Items:    mapOperationLogs(logs),
		Page:     filter.Page,
		PageSize: filter.PageSize,
		Total:    total,
	}, nil
}

// FindOperationLog returns one operation log by id.
func (u *Usecase) FindOperationLog(ctx context.Context, id int64) (OperationLog, error) {
	if err := u.ready(); err != nil {
		return OperationLog{}, err
	}
	if err := normalizeLogID(id); err != nil {
		return OperationLog{}, err
	}
	log, err := u.store.FindOperationLog(ctx, id)
	if err != nil {
		return OperationLog{}, err
	}
	return fromOperationLog(log), nil
}

// DeleteOperationLogs removes operation logs by id.
func (u *Usecase) DeleteOperationLogs(ctx context.Context, ids []int64) error {
	if err := u.ready(); err != nil {
		return err
	}
	ids, err := normalizeLogIDs(ids)
	if err != nil {
		return err
	}
	return u.store.DeleteOperationLogs(ctx, ids)
}

// ListLoginLogs returns paginated login logs.
func (u *Usecase) ListLoginLogs(ctx context.Context, input ListInput) (LoginListOutput, error) {
	if err := u.ready(); err != nil {
		return LoginListOutput{}, err
	}
	filter, err := normalizeListInput(input)
	if err != nil {
		return LoginListOutput{}, err
	}
	logs, total, err := u.store.ListLoginLogs(ctx, filter)
	if err != nil {
		return LoginListOutput{}, err
	}
	return LoginListOutput{
		Items:    mapLoginLogs(logs),
		Page:     filter.Page,
		PageSize: filter.PageSize,
		Total:    total,
	}, nil
}

// FindLoginLog returns one login log by id.
func (u *Usecase) FindLoginLog(ctx context.Context, id int64) (LoginLog, error) {
	if err := u.ready(); err != nil {
		return LoginLog{}, err
	}
	if err := normalizeLogID(id); err != nil {
		return LoginLog{}, err
	}
	log, err := u.store.FindLoginLog(ctx, id)
	if err != nil {
		return LoginLog{}, err
	}
	return fromLoginLog(log), nil
}

// DeleteLoginLogs removes login logs by id.
func (u *Usecase) DeleteLoginLogs(ctx context.Context, ids []int64) error {
	if err := u.ready(); err != nil {
		return err
	}
	ids, err := normalizeLogIDs(ids)
	if err != nil {
		return err
	}
	return u.store.DeleteLoginLogs(ctx, ids)
}

// ListSystemErrorLogs returns paginated internal API failure logs.
func (u *Usecase) ListSystemErrorLogs(ctx context.Context, input ListInput) (SystemErrorListOutput, error) {
	if err := u.ready(); err != nil {
		return SystemErrorListOutput{}, err
	}
	filter, err := normalizeListInput(input)
	if err != nil {
		return SystemErrorListOutput{}, err
	}
	logs, total, err := u.store.ListSystemErrorLogs(ctx, filter)
	if err != nil {
		return SystemErrorListOutput{}, err
	}
	return SystemErrorListOutput{
		Items:    mapSystemErrorLogs(logs),
		Page:     filter.Page,
		PageSize: filter.PageSize,
		Total:    total,
	}, nil
}

// FindSystemErrorLog returns one system error log by id.
func (u *Usecase) FindSystemErrorLog(ctx context.Context, id int64) (SystemErrorLog, error) {
	if err := u.ready(); err != nil {
		return SystemErrorLog{}, err
	}
	if err := normalizeLogID(id); err != nil {
		return SystemErrorLog{}, err
	}
	log, err := u.store.FindSystemErrorLog(ctx, id)
	if err != nil {
		return SystemErrorLog{}, err
	}
	return fromSystemErrorLog(log), nil
}

// ResolveSystemErrorLog records that an administrator handled one system error.
func (u *Usecase) ResolveSystemErrorLog(ctx context.Context, input ResolveSystemErrorInput) (SystemErrorLog, error) {
	if err := u.ready(); err != nil {
		return SystemErrorLog{}, err
	}
	if err := normalizeLogID(input.ID); err != nil {
		return SystemErrorLog{}, err
	}
	if input.ResolverID <= 0 {
		return SystemErrorLog{}, apperr.NewBadRequest("invalid resolver id")
	}
	log, err := u.store.FindSystemErrorLog(ctx, input.ID)
	if err != nil {
		return SystemErrorLog{}, err
	}
	resolved, err := log.Resolve(input.Note, input.ResolverID, time.Now().UTC())
	if err != nil {
		return SystemErrorLog{}, mapDomainError(err)
	}
	saved, err := u.store.UpdateSystemErrorLog(ctx, resolved)
	if err != nil {
		return SystemErrorLog{}, err
	}
	return fromSystemErrorLog(saved), nil
}

// ReopenSystemErrorLog clears a wrong resolution from one system error.
func (u *Usecase) ReopenSystemErrorLog(ctx context.Context, id int64) (SystemErrorLog, error) {
	if err := u.ready(); err != nil {
		return SystemErrorLog{}, err
	}
	if err := normalizeLogID(id); err != nil {
		return SystemErrorLog{}, err
	}
	log, err := u.store.FindSystemErrorLog(ctx, id)
	if err != nil {
		return SystemErrorLog{}, err
	}
	reopened, err := log.Reopen()
	if err != nil {
		return SystemErrorLog{}, mapDomainError(err)
	}
	saved, err := u.store.UpdateSystemErrorLog(ctx, reopened)
	if err != nil {
		return SystemErrorLog{}, err
	}
	return fromSystemErrorLog(saved), nil
}

// DeleteSystemErrorLogs removes system error logs by id.
func (u *Usecase) DeleteSystemErrorLogs(ctx context.Context, ids []int64) error {
	if err := u.ready(); err != nil {
		return err
	}
	ids, err := normalizeLogIDs(ids)
	if err != nil {
		return err
	}
	return u.store.DeleteSystemErrorLogs(ctx, ids)
}

func (u *Usecase) ready() error {
	if u == nil || u.store == nil {
		return apperr.New(apperr.ErrInternalServer, "audit store is not configured")
	}
	return nil
}

func normalizeListInput(input ListInput) (ListFilter, error) {
	window, err := pagination.Normalize(input.Page, input.PageSize, pagination.DefaultOptions)
	if err != nil {
		return ListFilter{}, apperr.NewBadRequest("invalid pagination")
	}
	return ListFilter{Window: window}, nil
}

func normalizeLogID(id int64) error {
	if id <= 0 {
		return apperr.NewBadRequest("invalid log id")
	}
	return nil
}

func normalizeLogIDs(ids []int64) ([]int64, error) {
	if len(ids) == 0 {
		return nil, apperr.NewBadRequest("log ids are required")
	}
	seen := make(map[int64]struct{}, len(ids))
	out := make([]int64, 0, len(ids))
	for _, id := range ids {
		if id <= 0 {
			return nil, apperr.NewBadRequest("invalid log id")
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		out = append(out, id)
	}
	return out, nil
}

func mapOperationLogs(logs []domain.OperationLog) []OperationLog {
	out := make([]OperationLog, 0, len(logs))
	for _, log := range logs {
		out = append(out, fromOperationLog(log))
	}
	return out
}

func mapLoginLogs(logs []domain.LoginLog) []LoginLog {
	out := make([]LoginLog, 0, len(logs))
	for _, log := range logs {
		out = append(out, fromLoginLog(log))
	}
	return out
}

func mapSystemErrorLogs(logs []domain.SystemErrorLog) []SystemErrorLog {
	out := make([]SystemErrorLog, 0, len(logs))
	for _, log := range logs {
		out = append(out, fromSystemErrorLog(log))
	}
	return out
}

func fromOperationLog(log domain.OperationLog) OperationLog {
	return OperationLog{
		ID:         log.ID,
		ActorID:    log.ActorID,
		Action:     log.Action,
		Resource:   log.Resource,
		ResourceID: log.ResourceID,
		Method:     log.Method,
		Path:       log.Path,
		IP:         log.IP,
		UserAgent:  log.UserAgent,
		Success:    log.Success,
		Message:    log.Message,
		CreatedAt:  log.CreatedAt,
	}
}

func fromLoginLog(log domain.LoginLog) LoginLog {
	return LoginLog{
		ID:        log.ID,
		AdminID:   log.AdminID,
		Username:  log.Username,
		IP:        log.IP,
		UserAgent: log.UserAgent,
		Success:   log.Success,
		Reason:    log.Reason,
		CreatedAt: log.CreatedAt,
	}
}

func fromSystemErrorLog(log domain.SystemErrorLog) SystemErrorLog {
	var resolvedAt *time.Time
	if !log.ResolvedAt.IsZero() {
		value := log.ResolvedAt
		resolvedAt = &value
	}
	return SystemErrorLog{
		ID:          log.ID,
		Code:        log.Code,
		Message:     log.Message,
		Detail:      log.Detail,
		Method:      log.Method,
		Path:        log.Path,
		IP:          log.IP,
		UserAgent:   log.UserAgent,
		RequestID:   log.RequestID,
		UserID:      log.UserID,
		Resolved:    log.Resolved,
		ResolveNote: log.ResolveNote,
		ResolvedBy:  log.ResolvedBy,
		ResolvedAt:  resolvedAt,
		CreatedAt:   log.CreatedAt,
	}
}

func mapDomainError(err error) error {
	for _, entry := range domainErrorMessages {
		if errors.Is(err, entry.err) {
			return apperr.NewBadRequest(entry.message)
		}
	}
	return err
}

var domainErrorMessages = []struct {
	err     error
	message string
}{
	{domain.ErrInvalidLogID, "invalid log id"},
	{domain.ErrInvalidAction, "invalid action"},
	{domain.ErrInvalidResource, "invalid resource"},
	{domain.ErrInvalidLoginUsername, "invalid login username"},
	{domain.ErrInvalidErrorCode, "invalid error code"},
	{domain.ErrInvalidErrorMessage, "invalid error message"},
	{domain.ErrInvalidResolver, "invalid resolver id"},
}
