// Package domain contains operations-module business entities and rules.
// A01 用户管理：账号视图、限制类型、限制记录与业务留痕。
// 规则基线：《各模块技术点与实行方案》7.1 运营管理、《后端接口文档》3.12.1。
package domain

import (
	"errors"
	"strings"
	"time"
	"unicode/utf8"
)

// Restriction types split one "账号封禁" into four narrow business limits so
// consumers can gate exactly the surface they own. Tokens are stable wire
// values shared with the order/taxi business side.
const (
	// RestrictionNoNewRide blocks creating new ride orders only; running trips
	// and paying existing ones stay usable (受限会话保留).
	RestrictionNoNewRide = "NO_NEW_RIDE"
	// RestrictionNoNewOrder blocks a driver from receiving new dispatch tasks;
	// tasks already accepted must run to completion.
	RestrictionNoNewOrder = "NO_NEW_ORDER"
	// RestrictionNoWithdraw blocks withdrawal applications; income still books.
	RestrictionNoWithdraw = "NO_WITHDRAW"
	// RestrictionLoginRisk forces extra verification on next logins.
	RestrictionLoginRisk = "LOGIN_RISK"
)

// Restriction lifecycle states.
const (
	RestrictionActive  = "ACTIVE"
	RestrictionRevoked = "REVOKED"
)

// Account roles surfaced to the management UI.
const (
	RolePassenger = "pax"
	RoleDriver    = "drv"
)

// Restriction rule errors.
var (
	ErrInvalidRestrictionID     = errors.New("invalid restriction id")
	ErrInvalidRestrictionType   = errors.New("invalid restriction type")
	ErrInvalidRestrictionReason = errors.New("invalid restriction reason")
	ErrInvalidRestrictionScope  = errors.New("invalid restriction scope")
	ErrRestrictionActive        = errors.New("restriction already active")
	ErrRestrictionNotActive     = errors.New("restriction is not active")
	ErrInvalidRevokeReason      = errors.New("invalid revoke reason")
	ErrInvalidAccountID         = errors.New("invalid account id")
	ErrInvalidOperator          = errors.New("invalid operator")
)

// restrictionTypes lists every wire value accepted by the restrict command.
var restrictionTypes = map[string]struct{}{
	RestrictionNoNewRide:  {},
	RestrictionNoNewOrder: {},
	RestrictionNoWithdraw: {},
	RestrictionLoginRisk:  {},
}

// IsValidRestrictionType reports whether value is a known restriction type.
func IsValidRestrictionType(value string) bool {
	_, ok := restrictionTypes[value]
	return ok
}

// RestrictionTypeNames returns all restriction type tokens for UI options.
func RestrictionTypeNames() []string {
	return []string{
		RestrictionNoNewRide,
		RestrictionNoNewOrder,
		RestrictionNoWithdraw,
		RestrictionLoginRisk,
	}
}

// Account is the management-side view of one business account. It is a
// read-only projection owned by the login domain; operations never mutate it.
type Account struct {
	ID        string
	Phone     string
	RealName  string
	Status    int
	CreatedAt time.Time
	Passenger bool
	DriverID  string // non-empty when the account registered as a driver
}

// Role projects the account into the passenger/driver role used by filters.
// An account that never registered as a driver is a passenger.
func (a Account) Role() string {
	if a.DriverID != "" {
		return RoleDriver
	}
	return RolePassenger
}

// PhoneMask hides the middle four digits: 138****6688. Full numbers leave the
// system only through the audited detail endpoint.
func (a Account) PhoneMask() string {
	return MaskPhone(a.Phone)
}

// MaskPhone masks a mainland mobile number, keeping the first three and last
// four digits. Values that do not look like one are returned unchanged so the
// caller never renders an empty identifier.
func MaskPhone(phone string) string {
	runes := []rune(strings.TrimSpace(phone))
	if len(runes) != 11 {
		return strings.TrimSpace(phone)
	}
	return string(runes[:3]) + "****" + string(runes[len(runes)-4:])
}

// Restriction is one audited business limit applied to an account. Records are
// append-only: revoking flips the state instead of deleting the row so every
// decision stays traceable.
type Restriction struct {
	ID           string    `json:"id"`
	AccountID    string    `json:"account_id"`
	Type         string    `json:"type"`
	Reason       string    `json:"reason"`
	Scope        string    `json:"scope"`
	State        string    `json:"state"`
	Operator     string    `json:"operator"`
	CreatedAt    time.Time `json:"created_at"`
	RevokedAt    time.Time `json:"revoked_at"`
	RevokedBy    string    `json:"revoked_by"`
	RevokeReason string    `json:"revoke_reason"`
}

// Active reports whether the restriction currently gates the account.
func (r Restriction) Active() bool {
	return r.State == RestrictionActive
}

// RestoreRestriction rebuilds a restriction from a trusted store record.
func RestoreRestriction(id, accountID, restrictionType, reason, scope, state, operator string, createdAt time.Time) (Restriction, error) {
	if strings.TrimSpace(id) == "" {
		return Restriction{}, ErrInvalidRestrictionID
	}
	restriction, err := newRestrictionFields(accountID, restrictionType, reason, scope, operator)
	if err != nil {
		return Restriction{}, err
	}
	if state != RestrictionActive && state != RestrictionRevoked {
		return Restriction{}, ErrInvalidRestrictionType
	}
	restriction.ID = id
	restriction.State = state
	restriction.CreatedAt = createdAt
	return restriction, nil
}

// NewRestriction validates the restrict command payload and returns one
// ACTIVE restriction without identity: the store assigns ID and timestamps.
func NewRestriction(accountID, restrictionType, reason, scope, operator string) (Restriction, error) {
	return newRestrictionFields(accountID, restrictionType, reason, scope, operator)
}

// newRestrictionFields validates the mutable part of a restriction command.
func newRestrictionFields(accountID, restrictionType, reason, scope, operator string) (Restriction, error) {
	accountID = strings.TrimSpace(accountID)
	if accountID == "" {
		return Restriction{}, ErrInvalidAccountID
	}
	if !IsValidRestrictionType(restrictionType) {
		return Restriction{}, ErrInvalidRestrictionType
	}
	reason = strings.TrimSpace(reason)
	if utf8.RuneCountInString(reason) < 5 || utf8.RuneCountInString(reason) > 200 {
		return Restriction{}, ErrInvalidRestrictionReason
	}
	scope = strings.TrimSpace(scope)
	if scope == "" || utf8.RuneCountInString(scope) > 120 {
		return Restriction{}, ErrInvalidRestrictionScope
	}
	operator = strings.TrimSpace(operator)
	if operator == "" {
		return Restriction{}, ErrInvalidOperator
	}
	return Restriction{
		AccountID: accountID,
		Type:      restrictionType,
		Reason:    reason,
		Scope:     scope,
		State:     RestrictionActive,
		Operator:  operator,
	}, nil
}

// OperationLog is the business-side audit trail written into the same MongoDB
// database the taxi services read. It records who did what, against which
// target, with which request — without storing sensitive payloads.
type OperationLog struct {
	ID        string
	Module    string
	Operator  string
	Action    string
	Resource  string
	Target    string
	RequestID string
	Detail    string
	At        time.Time
}
