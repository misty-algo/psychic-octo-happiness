// Package mysql persists audit logs in MySQL.
package mysql

import (
	"context"
	"errors"
	"time"

	"gorm.io/gorm"

	"github.com/NSObjects/echo-admin/internal/modules/audit/domain"
	"github.com/NSObjects/echo-admin/internal/modules/audit/usecase"
	"github.com/NSObjects/echo-admin/internal/platform/apperr"
)

// Store persists operation and login logs in MySQL.
type Store struct {
	db *gorm.DB
}

// NewStore migrates the MySQL audit tables.
func NewStore(ctx context.Context, db *gorm.DB) (*Store, error) {
	if ctx == nil {
		return nil, errors.New("create audit store: nil context")
	}
	if db == nil {
		return nil, errors.New("create audit store: nil db")
	}
	if err := db.WithContext(ctx).AutoMigrate(&operationLogModel{}, &loginLogModel{}, &systemErrorLogModel{}); err != nil {
		return nil, apperr.WrapDatabase(err, "migrate audit tables")
	}
	return &Store{db: db}, nil
}

// RecordOperation inserts one operation log.
func (s *Store) RecordOperation(ctx context.Context, log domain.OperationLog) (domain.OperationLog, error) {
	if err := ctx.Err(); err != nil {
		return domain.OperationLog{}, err
	}
	model := operationLogModelFromDomain(log, time.Now().UTC())
	if err := s.db.WithContext(ctx).Create(&model).Error; err != nil {
		return domain.OperationLog{}, apperr.WrapDatabase(err, "record operation log")
	}
	return model.toDomain()
}

// ListOperationLogs returns operation logs ordered by id descending.
func (s *Store) ListOperationLogs(ctx context.Context, filter usecase.ListFilter) ([]domain.OperationLog, int, error) {
	if err := ctx.Err(); err != nil {
		return nil, 0, err
	}
	return listAuditRows(ctx, s.db, &operationLogModel{}, filter, "count operation logs", "list operation logs", operationLogModel.toDomain)
}

// FindOperationLog returns one operation log by id.
func (s *Store) FindOperationLog(ctx context.Context, id int64) (domain.OperationLog, error) {
	if err := ctx.Err(); err != nil {
		return domain.OperationLog{}, err
	}
	return findAuditRow(ctx, s.db, id, "operation log", "find operation log", operationLogModel.toDomain)
}

// DeleteOperationLogs removes operation logs by id.
func (s *Store) DeleteOperationLogs(ctx context.Context, ids []int64) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	return deleteAuditRows(ctx, s.db, &operationLogModel{}, ids, "delete operation logs")
}

// RecordLogin inserts one login log.
func (s *Store) RecordLogin(ctx context.Context, log domain.LoginLog) (domain.LoginLog, error) {
	if err := ctx.Err(); err != nil {
		return domain.LoginLog{}, err
	}
	model := loginLogModelFromDomain(log, time.Now().UTC())
	if err := s.db.WithContext(ctx).Create(&model).Error; err != nil {
		return domain.LoginLog{}, apperr.WrapDatabase(err, "record login log")
	}
	return model.toDomain()
}

// ListLoginLogs returns login logs ordered by id descending.
func (s *Store) ListLoginLogs(ctx context.Context, filter usecase.ListFilter) ([]domain.LoginLog, int, error) {
	if err := ctx.Err(); err != nil {
		return nil, 0, err
	}
	return listAuditRows(ctx, s.db, &loginLogModel{}, filter, "count login logs", "list login logs", loginLogModel.toDomain)
}

// FindLoginLog returns one login log by id.
func (s *Store) FindLoginLog(ctx context.Context, id int64) (domain.LoginLog, error) {
	if err := ctx.Err(); err != nil {
		return domain.LoginLog{}, err
	}
	return findAuditRow(ctx, s.db, id, "login log", "find login log", loginLogModel.toDomain)
}

// DeleteLoginLogs removes login logs by id.
func (s *Store) DeleteLoginLogs(ctx context.Context, ids []int64) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	return deleteAuditRows(ctx, s.db, &loginLogModel{}, ids, "delete login logs")
}

// RecordSystemError inserts one internal API failure log.
func (s *Store) RecordSystemError(ctx context.Context, log domain.SystemErrorLog) (domain.SystemErrorLog, error) {
	if err := ctx.Err(); err != nil {
		return domain.SystemErrorLog{}, err
	}
	model := systemErrorLogModelFromDomain(log, time.Now().UTC())
	if err := s.db.WithContext(ctx).Create(&model).Error; err != nil {
		return domain.SystemErrorLog{}, apperr.WrapDatabase(err, "record system error log")
	}
	return model.toDomain()
}

// ListSystemErrorLogs returns system error logs ordered by id descending.
func (s *Store) ListSystemErrorLogs(ctx context.Context, filter usecase.ListFilter) ([]domain.SystemErrorLog, int, error) {
	if err := ctx.Err(); err != nil {
		return nil, 0, err
	}
	return listAuditRows(ctx, s.db, &systemErrorLogModel{}, filter, "count system error logs", "list system error logs", systemErrorLogModel.toDomain)
}

// FindSystemErrorLog returns one system error log by id.
func (s *Store) FindSystemErrorLog(ctx context.Context, id int64) (domain.SystemErrorLog, error) {
	if err := ctx.Err(); err != nil {
		return domain.SystemErrorLog{}, err
	}
	return findAuditRow(ctx, s.db, id, "system error log", "find system error log", systemErrorLogModel.toDomain)
}

// UpdateSystemErrorLog replaces mutable handling fields on one system error.
func (s *Store) UpdateSystemErrorLog(ctx context.Context, log domain.SystemErrorLog) (domain.SystemErrorLog, error) {
	if err := ctx.Err(); err != nil {
		return domain.SystemErrorLog{}, err
	}
	model := systemErrorLogModelFromDomain(log, time.Now().UTC())
	result := s.db.WithContext(ctx).Save(&model)
	if result.Error != nil {
		return domain.SystemErrorLog{}, apperr.WrapDatabase(result.Error, "update system error log")
	}
	if result.RowsAffected == 0 {
		return domain.SystemErrorLog{}, apperr.NewNotFound("system error log")
	}
	return model.toDomain()
}

// DeleteSystemErrorLogs removes system error logs by id.
func (s *Store) DeleteSystemErrorLogs(ctx context.Context, ids []int64) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	return deleteAuditRows(ctx, s.db, &systemErrorLogModel{}, ids, "delete system error logs")
}

type operationLogModel struct {
	ID         int64     `gorm:"primaryKey"`
	ActorID    int64     `gorm:"not null;index"`
	Action     string    `gorm:"type:varchar(80);not null"`
	Resource   string    `gorm:"type:varchar(80);not null"`
	ResourceID string    `gorm:"type:varchar(120);not null"`
	Method     string    `gorm:"type:varchar(16);not null"`
	Path       string    `gorm:"type:varchar(255);not null"`
	IP         string    `gorm:"type:varchar(64);not null"`
	UserAgent  string    `gorm:"type:varchar(255);not null"`
	Success    bool      `gorm:"not null"`
	Message    string    `gorm:"type:varchar(255);not null"`
	CreatedAt  time.Time `gorm:"not null;index"`
}

func (operationLogModel) TableName() string {
	return "audit_operation_logs"
}

func operationLogModelFromDomain(log domain.OperationLog, createdAt time.Time) operationLogModel {
	return operationLogModel{
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
		CreatedAt:  coalesceTime(log.CreatedAt, createdAt),
	}
}

func (m operationLogModel) toDomain() (domain.OperationLog, error) {
	return domain.RestoreOperationLog(m.ID, m.ActorID, m.Action, m.Resource, m.ResourceID, m.Method, m.Path, m.IP, m.UserAgent, m.Success, m.Message, m.CreatedAt)
}

type loginLogModel struct {
	ID        int64     `gorm:"primaryKey"`
	AdminID   int64     `gorm:"not null;index"`
	Username  string    `gorm:"type:varchar(64);not null;index"`
	IP        string    `gorm:"type:varchar(64);not null"`
	UserAgent string    `gorm:"type:varchar(255);not null"`
	Success   bool      `gorm:"not null;index"`
	Reason    string    `gorm:"type:varchar(255);not null"`
	CreatedAt time.Time `gorm:"not null;index"`
}

func (loginLogModel) TableName() string {
	return "audit_login_logs"
}

func loginLogModelFromDomain(log domain.LoginLog, createdAt time.Time) loginLogModel {
	return loginLogModel{
		ID:        log.ID,
		AdminID:   log.AdminID,
		Username:  log.Username,
		IP:        log.IP,
		UserAgent: log.UserAgent,
		Success:   log.Success,
		Reason:    log.Reason,
		CreatedAt: coalesceTime(log.CreatedAt, createdAt),
	}
}

func (m loginLogModel) toDomain() (domain.LoginLog, error) {
	return domain.RestoreLoginLog(m.ID, m.AdminID, m.Username, m.IP, m.UserAgent, m.Success, m.Reason, m.CreatedAt)
}

type systemErrorLogModel struct {
	ID          int64      `gorm:"primaryKey"`
	Code        int        `gorm:"not null;index"`
	Message     string     `gorm:"type:varchar(255);not null"`
	Detail      string     `gorm:"type:text;not null"`
	Method      string     `gorm:"type:varchar(16);not null"`
	Path        string     `gorm:"type:varchar(255);not null;index"`
	IP          string     `gorm:"type:varchar(64);not null"`
	UserAgent   string     `gorm:"type:varchar(255);not null"`
	RequestID   string     `gorm:"type:varchar(128);not null;index"`
	UserID      string     `gorm:"type:varchar(64);not null;index"`
	Resolved    bool       `gorm:"not null;index"`
	ResolveNote string     `gorm:"type:text;not null"`
	ResolvedBy  int64      `gorm:"not null;index"`
	ResolvedAt  *time.Time `gorm:"index"`
	CreatedAt   time.Time  `gorm:"not null;index"`
}

func (systemErrorLogModel) TableName() string {
	return "audit_system_error_logs"
}

func systemErrorLogModelFromDomain(log domain.SystemErrorLog, createdAt time.Time) systemErrorLogModel {
	var resolvedAt *time.Time
	if !log.ResolvedAt.IsZero() {
		value := log.ResolvedAt
		resolvedAt = &value
	}
	return systemErrorLogModel{
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
		CreatedAt:   coalesceTime(log.CreatedAt, createdAt),
	}
}

func (m systemErrorLogModel) toDomain() (domain.SystemErrorLog, error) {
	resolvedAt := time.Time{}
	if m.ResolvedAt != nil {
		resolvedAt = *m.ResolvedAt
	}
	return domain.RestoreSystemErrorLog(m.ID, m.Code, m.Message, m.Detail, m.Method, m.Path, m.IP, m.UserAgent, m.RequestID, m.UserID, m.Resolved, m.ResolveNote, m.ResolvedBy, resolvedAt, m.CreatedAt)
}

func coalesceTime(value, fallback time.Time) time.Time {
	if value.IsZero() {
		return fallback
	}
	return value
}

func listAuditRows[M, D any](
	ctx context.Context,
	db *gorm.DB,
	model any,
	filter usecase.ListFilter,
	countOperation string,
	listOperation string,
	convert func(M) (D, error),
) ([]D, int, error) {
	var total int64
	query := db.WithContext(ctx).Model(model)
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, apperr.WrapDatabase(err, countOperation)
	}
	var models []M
	err := query.Order("id DESC").Offset(filter.Offset).Limit(filter.Limit).Find(&models).Error
	if err != nil {
		return nil, 0, apperr.WrapDatabase(err, listOperation)
	}
	items := make([]D, 0, len(models))
	for _, row := range models {
		item, err := convert(row)
		if err != nil {
			return nil, 0, err
		}
		items = append(items, item)
	}
	return items, int(total), nil
}

func findAuditRow[M, D any](
	ctx context.Context,
	db *gorm.DB,
	id int64,
	resource string,
	operation string,
	convert func(M) (D, error),
) (D, error) {
	var model M
	err := db.WithContext(ctx).First(&model, "id = ?", id).Error
	if err != nil {
		var zero D
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return zero, apperr.NewNotFound(resource)
		}
		return zero, apperr.WrapDatabase(err, operation)
	}
	return convert(model)
}

func deleteAuditRows(ctx context.Context, db *gorm.DB, model any, ids []int64, operation string) error {
	result := db.WithContext(ctx).Where("id IN ?", ids).Delete(model)
	if result.Error != nil {
		return apperr.WrapDatabase(result.Error, operation)
	}
	if result.RowsAffected == 0 {
		return apperr.NewNotFound("log")
	}
	return nil
}
