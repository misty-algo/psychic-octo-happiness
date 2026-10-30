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

// AuditStore persists driver/vehicle audit applications. The vehicle_audits
// collection is shared with the driver side, so reads must tolerate rows it
// writes and writes must obey its unique (application_id, revision) index.
type AuditStore interface {
	// ListAudits returns one page of audit revisions plus the total count.
	ListAudits(ctx context.Context, filter AuditFilter) ([]domain.VehicleAudit, int64, error)
	// FindAudit returns one revision or domain.ErrAuditNotFound.
	FindAudit(ctx context.Context, applicationID string, revision int) (domain.VehicleAudit, error)
	// ListAuditRevisions returns the full version chain, newest first.
	ListAuditRevisions(ctx context.Context, applicationID string) ([]domain.VehicleAudit, error)
	// CountRejections counts rejected revisions per application id.
	CountRejections(ctx context.Context, applicationIDs []string) (map[string]int, error)
	// DriverBriefs returns driver name/plate for the given driver ids.
	DriverBriefs(ctx context.Context, driverIDs []string) (map[string]DriverBrief, error)
	// ClaimAudit takes the revision atomically; a lost race returns
	// domain.ErrAuditClaimedByOther.
	ClaimAudit(ctx context.Context, applicationID string, revision int, operator string, now time.Time, lease time.Duration) (domain.VehicleAudit, error)
	// DecideAudit writes the terminal decision under a version lock.
	DecideAudit(ctx context.Context, command DecideCommand) (domain.VehicleAudit, error)
	// ApplyVehicleSnapshot projects an approved vehicle onto the driver record.
	ApplyVehicleSnapshot(ctx context.Context, driverID string, vehicle domain.VehicleSnapshot, now time.Time) error
	// EmitQualificationEvent publishes the qualification-changed event.
	EmitQualificationEvent(ctx context.Context, audit domain.VehicleAudit, now time.Time) error
	// MarkAuditProjected records that projection and event are done.
	MarkAuditProjected(ctx context.Context, applicationID string, revision int) error
	// RecordOperationLog appends one business-side audit record.
	RecordOperationLog(ctx context.Context, log domain.OperationLog) error
}

// DriverBrief is the driver-side identity shown next to an application.
type DriverBrief struct {
	Name  string
	Plate string
}

// AuditFilter narrows the audit work queue.
type AuditFilter struct {
	State   string // "", 待审/通过/驳回
	Keyword string // application id or plate fragment
	Skip    int
	Limit   int
}

// DecideCommand is the compare-and-set decision payload handed to the store.
type DecideCommand struct {
	ApplicationID string
	Revision      int
	Version       int
	State         string
	Reason        string
	Operator      string
	At            time.Time
}

// ListAuditsInput carries the work-queue query.
type ListAuditsInput struct {
	State    string
	Keyword  string
	Page     int
	PageSize int
}

// AuditItem is one work-queue row enriched with driver identity and the
// rejection count that drives the manual-review flag.
type AuditItem struct {
	ApplicationID string                 `json:"application_id"`
	Revision      int                    `json:"revision"`
	DriverID      string                 `json:"driver_id"`
	DriverName    string                 `json:"driver_name"`
	Plate         string                 `json:"plate"`
	Submitter     string                 `json:"submitter"`
	State         string                 `json:"state"`
	Vehicle       domain.VehicleSnapshot `json:"vehicle"`
	ClaimedBy     string                 `json:"claimed_by"`
	ClaimedAt     time.Time              `json:"claimed_at"`
	DecidedBy     string                 `json:"decided_by"`
	DecidedAt     time.Time              `json:"decided_at"`
	Reason        string                 `json:"reason"`
	Version       int                    `json:"version"`
	SubmittedAt   time.Time              `json:"submitted_at"`
	Rejections    int                    `json:"rejections"`
	ManualReview  bool                   `json:"manual_review"`
}

// ListAuditsOutput is one page of audit rows.
type ListAuditsOutput struct {
	Items []AuditItem `json:"items"`
	Total int64       `json:"total"`
}

// AuditHistoryOutput is the immutable version chain of one application.
type AuditHistoryOutput struct {
	ApplicationID string      `json:"application_id"`
	Rejections    int         `json:"rejections"`
	ManualReview  bool        `json:"manual_review"`
	Revisions     []AuditItem `json:"revisions"`
}

// ClaimInput carries the claim command payload.
type ClaimInput struct {
	ApplicationID string
	Revision      int
	OperatorID    int64
	RequestID     string
}

// DecideInput carries the approve/reject command payload. Version is the
// optimistic lock value the operator saw when claiming.
type DecideInput struct {
	ApplicationID string
	Revision      int
	Action        string
	Reason        string
	Version       int
	OperatorID    int64
	RequestID     string
}

// AuditUsecase implements the driver/vehicle audit workbench rules.
type AuditUsecase struct {
	store AuditStore
	now   func() time.Time
}

// NewAudit creates the audit usecase.
func NewAudit(store AuditStore) *AuditUsecase {
	return &AuditUsecase{store: store, now: time.Now}
}

// ListAudits returns the work queue. Approved rows whose projection never
// completed are reconciled lazily here so a crash between decision and
// projection cannot leave a driver permanently unqualified.
func (u *AuditUsecase) ListAudits(ctx context.Context, input ListAuditsInput) (ListAuditsOutput, error) {
	filter := AuditFilter{
		State:   strings.TrimSpace(input.State),
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
	audits, total, err := u.store.ListAudits(ctx, filter)
	if err != nil {
		return ListAuditsOutput{}, err
	}
	items, err := u.enrich(ctx, audits)
	if err != nil {
		return ListAuditsOutput{}, err
	}
	return ListAuditsOutput{Items: items, Total: total}, nil
}

// History returns the full version chain so reviewers can see earlier
// rejections instead of only the latest revision.
func (u *AuditUsecase) History(ctx context.Context, applicationID string) (AuditHistoryOutput, error) {
	applicationID = strings.TrimSpace(applicationID)
	if err := domain.ValidateAuditKey(applicationID, 1); err != nil {
		return AuditHistoryOutput{}, mapRuleError(err)
	}
	audits, err := u.store.ListAuditRevisions(ctx, applicationID)
	if err != nil {
		return AuditHistoryOutput{}, mapRuleError(err)
	}
	if len(audits) == 0 {
		return AuditHistoryOutput{}, mapRuleError(domain.ErrAuditNotFound)
	}
	items, err := u.enrich(ctx, audits)
	if err != nil {
		return AuditHistoryOutput{}, err
	}
	rejections := 0
	for _, item := range items {
		if item.State == domain.AuditStateRejected {
			rejections++
		}
	}
	return AuditHistoryOutput{
		ApplicationID: applicationID,
		Rejections:    rejections,
		ManualReview:  rejections >= domain.ManualReviewThreshold,
		Revisions:     items,
	}, nil
}

// Claim takes one revision into the operator's queue.
func (u *AuditUsecase) Claim(ctx context.Context, input ClaimInput) (AuditItem, error) {
	applicationID := strings.TrimSpace(input.ApplicationID)
	if err := domain.ValidateAuditKey(applicationID, input.Revision); err != nil {
		return AuditItem{}, mapRuleError(err)
	}
	operator := operatorName(input.OperatorID)
	audit, err := u.store.FindAudit(ctx, applicationID, input.Revision)
	if err != nil {
		return AuditItem{}, mapRuleError(err)
	}
	if !audit.Pending() {
		return AuditItem{}, mapRuleError(domain.ErrAuditNotPending)
	}
	// 提交人不能审批自己的资料：领件即进入审批流程，直接拒绝。
	if audit.Submitter == operator {
		return AuditItem{}, mapRuleError(domain.ErrSelfApproval)
	}
	now := u.now()
	if !audit.ClaimableBy(operator, now) {
		return AuditItem{}, mapRuleError(domain.ErrAuditClaimedByOther)
	}
	claimed, err := u.store.ClaimAudit(ctx, applicationID, input.Revision, operator, now, domain.ClaimLeaseDuration)
	if err != nil {
		return AuditItem{}, mapRuleError(err)
	}
	if err := u.recordLog(ctx, domain.OperationLog{
		Module:    "operations",
		Operator:  operator,
		Action:    "claim_driver_audit",
		Resource:  "vehicle_audit",
		Target:    applicationID,
		RequestID: input.RequestID,
		Detail:    fmt.Sprintf("领取审核版本 revision=%d", input.Revision),
	}); err != nil {
		return AuditItem{}, err
	}
	return u.singleItem(ctx, claimed)
}

// Decide approves or rejects one claimed revision under a version lock. The
// terminal decision is unique: state and version are compared atomically, and
// a stale version loses with a conflict instead of overwriting the winner.
func (u *AuditUsecase) Decide(ctx context.Context, input DecideInput) (AuditItem, error) {
	applicationID := strings.TrimSpace(input.ApplicationID)
	if err := domain.ValidateAuditKey(applicationID, input.Revision); err != nil {
		return AuditItem{}, mapRuleError(err)
	}
	state, err := domain.NewDecision(input.Action, input.Reason)
	if err != nil {
		return AuditItem{}, mapRuleError(err)
	}
	operator := operatorName(input.OperatorID)
	audit, err := u.store.FindAudit(ctx, applicationID, input.Revision)
	if err != nil {
		return AuditItem{}, mapRuleError(err)
	}
	if !audit.Pending() {
		return AuditItem{}, mapRuleError(domain.ErrAuditNotPending)
	}
	if audit.Submitter == operator {
		return AuditItem{}, mapRuleError(domain.ErrSelfApproval)
	}
	if audit.ClaimedBy == "" {
		return AuditItem{}, mapRuleError(domain.ErrAuditNotClaimed)
	}
	if audit.ClaimedBy != operator {
		return AuditItem{}, mapRuleError(domain.ErrAuditClaimedByOther)
	}
	if input.Version != audit.Version {
		return AuditItem{}, mapRuleError(domain.ErrVersionConflict)
	}
	decided, err := u.store.DecideAudit(ctx, DecideCommand{
		ApplicationID: applicationID,
		Revision:      input.Revision,
		Version:       input.Version,
		State:         state,
		Reason:        strings.TrimSpace(input.Reason),
		Operator:      operator,
		At:            u.now(),
	})
	if err != nil {
		return AuditItem{}, mapRuleError(err)
	}
	if err := u.recordLog(ctx, domain.OperationLog{
		Module:    "operations",
		Operator:  operator,
		Action:    "decide_driver_audit",
		Resource:  "vehicle_audit",
		Target:    applicationID,
		RequestID: input.RequestID,
		Detail:    fmt.Sprintf("裁决=%s revision=%d 原因=%s", state, input.Revision, strings.TrimSpace(input.Reason)),
	}); err != nil {
		return AuditItem{}, err
	}
	// 通过后投影车辆资料并发布资质变化事件；投影幂等，失败可由下次列表
	// 读取补偿重放，裁决本身不回滚（终态唯一）。
	if state == domain.AuditStateApproved {
		if projectErr := u.finalize(ctx, decided); projectErr != nil {
			return AuditItem{}, apperr.WrapInternal(projectErr, "审核已通过但资质联动失败，请稍后重试")
		}
		decided.Projected = true
	}
	return u.singleItem(ctx, decided)
}

// finalize applies the approved vehicle snapshot and publishes the
// qualification event exactly once.
func (u *AuditUsecase) finalize(ctx context.Context, audit domain.VehicleAudit) error {
	if audit.Projected {
		return nil
	}
	now := u.now()
	if err := u.store.ApplyVehicleSnapshot(ctx, audit.DriverID, audit.Vehicle, now); err != nil {
		return err
	}
	if err := u.store.EmitQualificationEvent(ctx, audit, now); err != nil {
		return err
	}
	return u.store.MarkAuditProjected(ctx, audit.ApplicationID, audit.Revision)
}

// enrich joins driver identity and rejection counts onto raw audit rows and
// reconciles approvals whose projection is still pending.
func (u *AuditUsecase) enrich(ctx context.Context, audits []domain.VehicleAudit) ([]AuditItem, error) {
	if len(audits) == 0 {
		return []AuditItem{}, nil
	}
	applicationIDs := make([]string, 0, len(audits))
	driverIDs := make([]string, 0, len(audits))
	seen := make(map[string]struct{}, len(audits))
	for _, audit := range audits {
		if _, ok := seen[audit.ApplicationID]; !ok {
			seen[audit.ApplicationID] = struct{}{}
			applicationIDs = append(applicationIDs, audit.ApplicationID)
		}
		driverIDs = append(driverIDs, audit.DriverID)
	}
	rejections, err := u.store.CountRejections(ctx, applicationIDs)
	if err != nil {
		return nil, err
	}
	briefs, err := u.store.DriverBriefs(ctx, driverIDs)
	if err != nil {
		return nil, err
	}
	items := make([]AuditItem, 0, len(audits))
	for _, audit := range audits {
		// 崩溃在裁决与投影之间时自愈：已通过但未投影的版本在此重放。
		if audit.State == domain.AuditStateApproved && !audit.Projected {
			_ = u.finalize(ctx, audit)
		}
		brief := briefs[audit.DriverID]
		plate := audit.Vehicle.Plate
		if plate == "" {
			plate = brief.Plate
		}
		count := rejections[audit.ApplicationID]
		items = append(items, AuditItem{
			ApplicationID: audit.ApplicationID,
			Revision:      audit.Revision,
			DriverID:      audit.DriverID,
			DriverName:    brief.Name,
			Plate:         plate,
			Submitter:     audit.Submitter,
			State:         audit.State,
			Vehicle:       audit.Vehicle,
			ClaimedBy:     audit.ClaimedBy,
			ClaimedAt:     audit.ClaimedAt,
			DecidedBy:     audit.DecidedBy,
			DecidedAt:     audit.DecidedAt,
			Reason:        audit.Reason,
			Version:       audit.Version,
			SubmittedAt:   audit.SubmittedAt,
			Rejections:    count,
			ManualReview:  count >= domain.ManualReviewThreshold,
		})
	}
	return items, nil
}

// singleItem enriches exactly one revision.
func (u *AuditUsecase) singleItem(ctx context.Context, audit domain.VehicleAudit) (AuditItem, error) {
	items, err := u.enrich(ctx, []domain.VehicleAudit{audit})
	if err != nil {
		return AuditItem{}, err
	}
	if len(items) == 0 {
		return AuditItem{}, domain.ErrAuditNotFound
	}
	return items[0], nil
}

// recordLog appends the business-side audit entry.
func (u *AuditUsecase) recordLog(ctx context.Context, log domain.OperationLog) error {
	log.At = u.now()
	return u.store.RecordOperationLog(ctx, log)
}

// mapAuditError keeps audit sentinels in the unwrap chain while attaching the
// HTTP class each rule failure deserves.
func mapAuditError(err error) error {
	switch {
	case err == nil:
		return nil
	case errors.Is(err, domain.ErrAuditNotFound):
		return apperr.WrapNotFound(err, "审核申请不存在")
	case errors.Is(err, domain.ErrAuditNotPending):
		return apperr.WrapConflict(err, "该版本已裁决，不能重复审批")
	case errors.Is(err, domain.ErrAuditClaimedByOther):
		return apperr.WrapConflict(err, "该版本已被其他审核员领取")
	case errors.Is(err, domain.ErrAuditNotClaimed):
		return apperr.WrapConflict(err, "请先领取该版本再审批")
	case errors.Is(err, domain.ErrVersionConflict):
		return apperr.WrapConflict(err, "版本号已变化，请刷新后重试")
	case errors.Is(err, domain.ErrSelfApproval):
		return apperr.WrapForbidden(err, "提交人不能审批自己的资料")
	case errors.Is(err, domain.ErrRejectReasonRequired):
		return apperr.WrapBadRequest(err, "驳回必须填写 5-200 字的原因")
	case errors.Is(err, domain.ErrInvalidAuditAction):
		return apperr.WrapBadRequest(err, "action 只能为 approve 或 reject")
	case errors.Is(err, domain.ErrInvalidApplicationID):
		return apperr.WrapBadRequest(err, "申请编号不合法")
	case errors.Is(err, domain.ErrInvalidRevision):
		return apperr.WrapBadRequest(err, "版本号不合法")
	default:
		return err
	}
}
