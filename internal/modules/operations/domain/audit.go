package domain

import (
	"errors"
	"strings"
	"time"
	"unicode/utf8"
)

// 审核状态取值与库内 vehicle_audits.state 保持一致（中文终态），
// 司机端 D14 提交的申请直接进入该集合，两侧共用同一套状态字典。
const (
	AuditStatePending  = "待审"
	AuditStateApproved = "通过"
	AuditStateRejected = "驳回"
)

// 裁决动作。
const (
	AuditActionApprove = "approve"
	AuditActionReject  = "reject"
)

// ManualReviewThreshold 同一申请累计驳回达到该次数后标记人工重点复核。
const ManualReviewThreshold = 3

// ClaimLeaseDuration 领件租约：超时未裁决的工单可被其他审核员重新领取，
// 避免审核员离岗导致工单被永久占用。
const ClaimLeaseDuration = 30 * time.Minute

// 审核规则错误。
var (
	ErrInvalidApplicationID   = errors.New("invalid application id")
	ErrInvalidRevision        = errors.New("invalid revision")
	ErrAuditNotFound          = errors.New("audit application not found")
	ErrAuditNotPending        = errors.New("audit application is not pending")
	ErrAuditClaimedByOther    = errors.New("audit application claimed by another operator")
	ErrAuditNotClaimed        = errors.New("audit application is not claimed")
	ErrInvalidAuditAction     = errors.New("invalid audit action")
	ErrRejectReasonRequired   = errors.New("reject reason is required")
	ErrSelfApproval           = errors.New("submitter cannot decide own application")
	ErrVersionConflict        = errors.New("audit version conflict")
	ErrInvalidDriverID        = errors.New("invalid driver id")
	ErrInvalidVehicleSnapshot = errors.New("invalid vehicle snapshot")
)

// VehicleSnapshot is the vehicle data frozen into one audit revision. Approval
// projects it onto the driver record; history revisions keep their own copy so
// later edits never rewrite what was decided.
type VehicleSnapshot struct {
	Plate      string `json:"plate"`
	Model      string `json:"model"`
	Seats      int    `json:"seats"`
	CertNo     string `json:"cert_no"`
	CertExpire string `json:"cert_expire"`
}

// NewVehicleSnapshot validates a submitted vehicle payload.
func NewVehicleSnapshot(plate, model string, seats int, certNo, certExpire string) (VehicleSnapshot, error) {
	plate = strings.TrimSpace(plate)
	model = strings.TrimSpace(model)
	certNo = strings.TrimSpace(certNo)
	certExpire = strings.TrimSpace(certExpire)
	if plate == "" || utf8.RuneCountInString(plate) > 20 {
		return VehicleSnapshot{}, ErrInvalidVehicleSnapshot
	}
	if model == "" || utf8.RuneCountInString(model) > 60 {
		return VehicleSnapshot{}, ErrInvalidVehicleSnapshot
	}
	if seats <= 0 || seats > 60 {
		return VehicleSnapshot{}, ErrInvalidVehicleSnapshot
	}
	if certNo == "" || utf8.RuneCountInString(certNo) > 60 {
		return VehicleSnapshot{}, ErrInvalidVehicleSnapshot
	}
	if _, err := time.Parse(time.DateOnly, certExpire); err != nil {
		return VehicleSnapshot{}, ErrInvalidVehicleSnapshot
	}
	return VehicleSnapshot{Plate: plate, Model: model, Seats: seats, CertNo: certNo, CertExpire: certExpire}, nil
}

// VehicleAudit is one immutable application revision awaiting a decision.
// application_id + revision form the version chain; the terminal decision is
// unique because every decision is a compare-and-set on state and version.
type VehicleAudit struct {
	ApplicationID string          `json:"application_id"`
	Revision      int             `json:"revision"`
	DriverID      string          `json:"driver_id"`
	Submitter     string          `json:"submitter"`
	State         string          `json:"state"`
	Vehicle       VehicleSnapshot `json:"vehicle"`
	ClaimedBy     string          `json:"claimed_by"`
	ClaimedAt     time.Time       `json:"claimed_at"`
	Version       int             `json:"version"`
	Reason        string          `json:"reason"`
	DecidedBy     string          `json:"decided_by"`
	DecidedAt     time.Time       `json:"decided_at"`
	SubmittedAt   time.Time       `json:"submitted_at"`
	Projected     bool            `json:"projected"`
}

// Pending reports whether the revision still accepts a decision.
func (a VehicleAudit) Pending() bool {
	return a.State == AuditStatePending
}

// ClaimableBy reports whether operator may take the revision: unclaimed,
// already owned by the same operator, or the previous lease expired.
func (a VehicleAudit) ClaimableBy(operator string, now time.Time) bool {
	if !a.Pending() {
		return false
	}
	switch {
	case a.ClaimedBy == "":
		return true
	case a.ClaimedBy == operator:
		return true
	default:
		return !a.ClaimedAt.IsZero() && now.Sub(a.ClaimedAt) >= ClaimLeaseDuration
	}
}

// ValidateAuditKey rejects empty application ids and non-positive revisions.
func ValidateAuditKey(applicationID string, revision int) error {
	if strings.TrimSpace(applicationID) == "" || utf8.RuneCountInString(applicationID) > 64 {
		return ErrInvalidApplicationID
	}
	if revision <= 0 || revision > 1000 {
		return ErrInvalidRevision
	}
	return nil
}

// NewDecision validates an approve/reject command and returns the target state.
// Rejecting without a substantive reason is refused: the reason is the only
// thing the driver side ever sees about a rejection.
func NewDecision(action, reason string) (string, error) {
	action = strings.TrimSpace(action)
	reason = strings.TrimSpace(reason)
	switch action {
	case AuditActionApprove:
		return AuditStateApproved, nil
	case AuditActionReject:
		if utf8.RuneCountInString(reason) < 5 || utf8.RuneCountInString(reason) > 200 {
			return "", ErrRejectReasonRequired
		}
		return AuditStateRejected, nil
	default:
		return "", ErrInvalidAuditAction
	}
}

// QualificationEventType is the domain event emitted after an approval so the
// admission service can recompute whether the driver may take orders. The
// operations module never flips driver status itself.
const QualificationEventType = "driver.qualification.changed"

// EventLog is the outbox record consumed by the business side. DedupeKey makes
// event emission idempotent so a retried approval cannot double-publish.
type EventLog struct {
	ID          string
	Type        string
	Aggregate   string
	AggregateID string
	DedupeKey   string
	Payload     map[string]any
	Status      string
	CreatedAt   time.Time
}
