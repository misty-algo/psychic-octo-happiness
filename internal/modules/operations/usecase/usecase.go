// Package usecase coordinates operations-module user management workflows.
package usecase

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/NSObjects/echo-admin/internal/modules/operations/domain"
	"github.com/NSObjects/echo-admin/internal/platform/apperr"
)

// mapRuleError attaches HTTP semantics to domain rule errors while keeping
// the sentinel in the unwrap chain for tests and callers.
func mapRuleError(err error) error {
	switch {
	case err == nil:
		return nil
	case errors.Is(err, domain.ErrRestrictionActive), errors.Is(err, domain.ErrRestrictionNotActive):
		return apperr.WrapConflict(err, "限制状态不允许该操作")
	case errors.Is(err, domain.ErrInvalidRestrictionType):
		return apperr.WrapBadRequest(err, "不支持的限制类型")
	case errors.Is(err, domain.ErrInvalidRestrictionReason), errors.Is(err, domain.ErrInvalidRevokeReason):
		return apperr.WrapBadRequest(err, "原因长度需在 5-200 字之间")
	case errors.Is(err, domain.ErrInvalidRestrictionScope):
		return apperr.WrapBadRequest(err, "限制范围必填且不超过 120 字")
	case errors.Is(err, domain.ErrInvalidAccountID):
		return apperr.WrapNotFound(err, "用户不存在")
	case errors.Is(err, domain.ErrInvalidRestrictionID):
		return apperr.WrapNotFound(err, "限制记录不存在")
	case errors.Is(err, domain.ErrInvalidOperator):
		return apperr.WrapUnauthorized(err, "缺少操作者身份")
	default:
		return mapAuditError(err)
	}
}

// Store persists operations user-management data. Implementations keep the
// business database (MongoDB) authoritative; the usecase never caches.
type Store interface {
	// ListAccounts returns one page of accounts plus the total count.
	ListAccounts(ctx context.Context, filter AccountFilter) ([]domain.Account, int64, error)
	// FindAccount returns one account or ErrAccountNotFound.
	FindAccount(ctx context.Context, id string) (domain.Account, error)
	// ListActiveRestrictions returns ACTIVE restrictions grouped by account id.
	ListActiveRestrictions(ctx context.Context, accountIDs []string) (map[string][]domain.Restriction, error)
	// FindActiveRestriction returns the ACTIVE restriction of one type.
	FindActiveRestriction(ctx context.Context, accountID, restrictionType string) (domain.Restriction, bool, error)
	// CreateRestriction appends one ACTIVE restriction record.
	CreateRestriction(ctx context.Context, restriction domain.Restriction) (domain.Restriction, error)
	// FindRestriction returns one restriction by id.
	FindRestriction(ctx context.Context, id string) (domain.Restriction, error)
	// RevokeRestriction flips one restriction to REVOKED.
	RevokeRestriction(ctx context.Context, id string, input RevokeRestrictionInput) (domain.Restriction, error)
	// RecordOperationLog appends one business-side audit record.
	RecordOperationLog(ctx context.Context, log domain.OperationLog) error
}

// RevokeRestrictionInput carries the revoke command payload.
type RevokeRestrictionInput struct {
	Reason   string
	Operator string
	At       time.Time
}

// AccountFilter narrows the account list. Skip/Limit are storage-facing
// paging bounds derived by the usecase from Page/PageSize inputs.
type AccountFilter struct {
	Role    string // "", domain.RolePassenger, domain.RoleDriver
	Keyword string // matches phone tail digits or real name
	Skip    int
	Limit   int
}

// ListUsersInput carries the user list query.
type ListUsersInput struct {
	Role     string
	Keyword  string
	Page     int
	PageSize int
}

// UserItem is one list row: masked phone plus effective restrictions.
type UserItem struct {
	ID        string               `json:"id"`
	PhoneMask string               `json:"phone_mask"`
	RealName  string               `json:"real_name"`
	Role      string               `json:"role"`
	Status    int                  `json:"status"`
	Restrict  []domain.Restriction `json:"restrict"`
	CreatedAt time.Time            `json:"created_at"`
}

// ListUsersOutput is one page of user items.
type ListUsersOutput struct {
	Items []UserItem `json:"users"`
	Total int64      `json:"total"`
}

// UserDetail exposes the full phone number; every read is audited.
type UserDetail struct {
	ID        string               `json:"id"`
	Phone     string               `json:"phone"`
	RealName  string               `json:"real_name"`
	Role      string               `json:"role"`
	Status    int                  `json:"status"`
	Passenger bool                 `json:"passenger"`
	DriverID  string               `json:"driver_id"`
	Restrict  []domain.Restriction `json:"restrict"`
	CreatedAt time.Time            `json:"created_at"`
}

// RestrictInput carries the restrict command payload.
type RestrictInput struct {
	AccountID  string
	Type       string
	Reason     string
	Scope      string
	OperatorID int64
	RequestID  string
}

// UnrestrictInput carries the unrestrict command payload.
type UnrestrictInput struct {
	AccountID     string
	RestrictionID string
	Reason        string
	OperatorID    int64
	RequestID     string
}

// maxPageSize caps one page so listing stays bounded.
const maxPageSize = 100

// usecase implements operations user-management rules.
type Usecase struct {
	store Store
	now   func() time.Time
}

// New creates an operations user-management usecase.
func New(store Store) *Usecase {
	return &Usecase{store: store, now: time.Now}
}

// operatorName renders the auditable actor identity. echo-admin owns admin
// identity; the business log references it as admin:<id> so cross-store
// tracing stays possible without duplicating profile data.
func operatorName(operatorID int64) string {
	return fmt.Sprintf("admin:%d", operatorID)
}

// ListUsers returns one page of accounts with their effective restrictions.
func (u *Usecase) ListUsers(ctx context.Context, input ListUsersInput) (ListUsersOutput, error) {
	filter := AccountFilter{
		Role:    strings.TrimSpace(input.Role),
		Keyword: strings.TrimSpace(input.Keyword),
		Skip:    (input.Page - 1) * input.PageSize,
		Limit:   input.PageSize,
	}
	if filter.Skip < 0 {
		filter.Skip = 0
	}
	if filter.Limit <= 0 || filter.Limit > maxPageSize {
		filter.Limit = maxPageSize
	}
	accounts, total, err := u.store.ListAccounts(ctx, filter)
	if err != nil {
		return ListUsersOutput{}, err
	}
	ids := make([]string, 0, len(accounts))
	for _, account := range accounts {
		ids = append(ids, account.ID)
	}
	restrictions, err := u.store.ListActiveRestrictions(ctx, ids)
	if err != nil {
		return ListUsersOutput{}, err
	}
	items := make([]UserItem, 0, len(accounts))
	for _, account := range accounts {
		// 空限制输出 [] 而不是 null，前端契约保持稳定。
		restrict := restrictions[account.ID]
		if restrict == nil {
			restrict = []domain.Restriction{}
		}
		items = append(items, UserItem{
			ID:        account.ID,
			PhoneMask: account.PhoneMask(),
			RealName:  account.RealName,
			Role:      account.Role(),
			Status:    account.Status,
			Restrict:  restrict,
			CreatedAt: account.CreatedAt,
		})
	}
	return ListUsersOutput{Items: items, Total: total}, nil
}

// UserDetail returns the account with the full phone number and records a
// business-side audit entry, because unmasking is a sensitive read.
func (u *Usecase) UserDetail(ctx context.Context, accountID string, operatorID int64, requestID string) (UserDetail, error) {
	account, err := u.store.FindAccount(ctx, strings.TrimSpace(accountID))
	if err != nil {
		return UserDetail{}, mapRuleError(err)
	}
	restrictions, err := u.store.ListActiveRestrictions(ctx, []string{account.ID})
	if err != nil {
		return UserDetail{}, err
	}
	if restrictions[account.ID] == nil {
		restrictions[account.ID] = []domain.Restriction{}
	}
	operator := operatorName(operatorID)
	if err := u.recordLog(ctx, domain.OperationLog{
		Module:    "operations",
		Operator:  operator,
		Action:    "view_full_phone",
		Resource:  "account",
		Target:    account.ID,
		RequestID: requestID,
		Detail:    "查看用户完整手机号",
	}); err != nil {
		return UserDetail{}, err
	}
	return UserDetail{
		ID:        account.ID,
		Phone:     account.Phone,
		RealName:  account.RealName,
		Role:      account.Role(),
		Status:    account.Status,
		Passenger: account.Passenger,
		DriverID:  account.DriverID,
		Restrict:  restrictions[account.ID],
		CreatedAt: account.CreatedAt,
	}, nil
}

// RestrictUser applies one business limit. Adding the same type while an
// ACTIVE restriction exists is rejected so history stays unambiguous.
func (u *Usecase) RestrictUser(ctx context.Context, input RestrictInput) (domain.Restriction, error) {
	if _, err := u.store.FindAccount(ctx, strings.TrimSpace(input.AccountID)); err != nil {
		return domain.Restriction{}, mapRuleError(err)
	}
	_, active, err := u.store.FindActiveRestriction(ctx, strings.TrimSpace(input.AccountID), strings.TrimSpace(input.Type))
	if err != nil {
		return domain.Restriction{}, mapRuleError(err)
	}
	if active {
		return domain.Restriction{}, mapRuleError(fmt.Errorf("restriction %s: %w", input.Type, domain.ErrRestrictionActive))
	}
	restriction, err := domain.NewRestriction(input.AccountID, input.Type, input.Reason, input.Scope, operatorName(input.OperatorID))
	if err != nil {
		return domain.Restriction{}, mapRuleError(err)
	}
	restriction.CreatedAt = u.now()
	created, err := u.store.CreateRestriction(ctx, restriction)
	if err != nil {
		return domain.Restriction{}, mapRuleError(err)
	}
	err = u.recordLog(ctx, domain.OperationLog{
		Module:    "operations",
		Operator:  restriction.Operator,
		Action:    "restrict",
		Resource:  "account",
		Target:    restriction.AccountID,
		RequestID: input.RequestID,
		Detail:    fmt.Sprintf("限制类型=%s 原因=%s 范围=%s", restriction.Type, restriction.Reason, restriction.Scope),
	})
	if err != nil {
		return domain.Restriction{}, err
	}
	return created, nil
}

// UnrestrictUser revokes one ACTIVE restriction. Revoke keeps the record with
// its reason so the decision trail remains complete.
func (u *Usecase) UnrestrictUser(ctx context.Context, input UnrestrictInput) (domain.Restriction, error) {
	restriction, err := u.store.FindRestriction(ctx, strings.TrimSpace(input.RestrictionID))
	if err != nil {
		return domain.Restriction{}, mapRuleError(err)
	}
	if restriction.AccountID != strings.TrimSpace(input.AccountID) {
		return domain.Restriction{}, mapRuleError(domain.ErrInvalidAccountID)
	}
	if !restriction.Active() {
		return domain.Restriction{}, mapRuleError(fmt.Errorf("restriction %s: %w", restriction.ID, domain.ErrRestrictionNotActive))
	}
	reason := strings.TrimSpace(input.Reason)
	if len([]rune(reason)) < 5 || len([]rune(reason)) > 200 {
		return domain.Restriction{}, mapRuleError(domain.ErrInvalidRevokeReason)
	}
	revoked, err := u.store.RevokeRestriction(ctx, restriction.ID, RevokeRestrictionInput{
		Reason:   reason,
		Operator: operatorName(input.OperatorID),
		At:       u.now(),
	})
	if err != nil {
		return domain.Restriction{}, mapRuleError(err)
	}
	err = u.recordLog(ctx, domain.OperationLog{
		Module:    "operations",
		Operator:  revoked.RevokedBy,
		Action:    "unrestrict",
		Resource:  "account",
		Target:    revoked.AccountID,
		RequestID: input.RequestID,
		Detail:    fmt.Sprintf("解除限制类型=%s 原因=%s", revoked.Type, reason),
	})
	if err != nil {
		return domain.Restriction{}, err
	}
	return revoked, nil
}

// recordLog appends the business-side audit entry; failures fail the command
// so audit stays transactional with the business effect.
func (u *Usecase) recordLog(ctx context.Context, log domain.OperationLog) error {
	log.At = u.now()
	return u.store.RecordOperationLog(ctx, log)
}
