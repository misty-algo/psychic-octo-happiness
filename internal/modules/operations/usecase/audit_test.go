package usecase

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/NSObjects/echo-admin/internal/modules/operations/domain"
)

// fakeAuditStore is an in-memory AuditStore double.
type fakeAuditStore struct {
	audits  []domain.VehicleAudit
	drivers map[string]DriverBrief
	logs    []domain.OperationLog
	events  []domain.VehicleAudit
	applied []domain.VehicleSnapshot
}

func newFakeAuditStore() *fakeAuditStore {
	return &fakeAuditStore{
		audits: []domain.VehicleAudit{
			{
				ApplicationID: "VA-1", Revision: 1, DriverID: "drv-1",
				Submitter: "account:acc-1", State: domain.AuditStatePending,
				Vehicle:     domain.VehicleSnapshot{Plate: "沪AD·66666", Model: "比亚迪秦PLUS", Seats: 4, CertNo: "YS-0091", CertExpire: "2027-12-31"},
				Version:     1,
				SubmittedAt: time.Date(2026, 10, 20, 9, 0, 0, 0, time.UTC),
			},
			{
				ApplicationID: "VA-2", Revision: 1, DriverID: "drv-1",
				Submitter: "account:acc-1", State: domain.AuditStateRejected,
				Version: 3, Reason: "行驶证照片不清晰",
				SubmittedAt: time.Date(2026, 10, 21, 9, 0, 0, 0, time.UTC),
			},
			{
				ApplicationID: "VA-2", Revision: 2, DriverID: "drv-1",
				Submitter: "account:acc-1", State: domain.AuditStatePending,
				Version: 1, SubmittedAt: time.Date(2026, 10, 22, 9, 0, 0, 0, time.UTC),
			},
			{
				ApplicationID: "VA-3", Revision: 1, DriverID: "drv-1",
				Submitter: "admin:1", State: domain.AuditStatePending,
				Version: 1, SubmittedAt: time.Date(2026, 10, 22, 10, 0, 0, 0, time.UTC),
			},
		},
		drivers: map[string]DriverBrief{"drv-1": {Name: "王师傅", Plate: "沪AD·88888"}},
	}
}

func (f *fakeAuditStore) ListAudits(_ context.Context, filter AuditFilter) ([]domain.VehicleAudit, int64, error) {
	out := make([]domain.VehicleAudit, 0, len(f.audits))
	for _, audit := range f.audits {
		if filter.State != "" && audit.State != filter.State {
			continue
		}
		out = append(out, audit)
	}
	total := int64(len(out))
	if filter.Limit > 0 {
		start := filter.Skip
		if start >= len(out) {
			return []domain.VehicleAudit{}, total, nil
		}
		end := start + filter.Limit
		if end > len(out) {
			end = len(out)
		}
		out = out[start:end]
	}
	return out, total, nil
}

func (f *fakeAuditStore) FindAudit(_ context.Context, applicationID string, revision int) (domain.VehicleAudit, error) {
	for _, audit := range f.audits {
		if audit.ApplicationID == applicationID && audit.Revision == revision {
			return audit, nil
		}
	}
	return domain.VehicleAudit{}, domain.ErrAuditNotFound
}

func (f *fakeAuditStore) ListAuditRevisions(_ context.Context, applicationID string) ([]domain.VehicleAudit, error) {
	out := make([]domain.VehicleAudit, 0)
	for _, audit := range f.audits {
		if audit.ApplicationID == applicationID {
			out = append(out, audit)
		}
	}
	return out, nil
}

func (f *fakeAuditStore) CountRejections(_ context.Context, applicationIDs []string) (map[string]int, error) {
	want := make(map[string]struct{}, len(applicationIDs))
	for _, id := range applicationIDs {
		want[id] = struct{}{}
	}
	out := make(map[string]int)
	for _, audit := range f.audits {
		if audit.State != domain.AuditStateRejected {
			continue
		}
		if _, ok := want[audit.ApplicationID]; ok {
			out[audit.ApplicationID]++
		}
	}
	return out, nil
}

func (f *fakeAuditStore) DriverBriefs(_ context.Context, driverIDs []string) (map[string]DriverBrief, error) {
	out := make(map[string]DriverBrief, len(driverIDs))
	for _, id := range driverIDs {
		if brief, ok := f.drivers[id]; ok {
			out[id] = brief
		}
	}
	return out, nil
}

func (f *fakeAuditStore) ClaimAudit(_ context.Context, applicationID string, revision int, operator string, now time.Time, lease time.Duration) (domain.VehicleAudit, error) {
	for i := range f.audits {
		audit := f.audits[i]
		if audit.ApplicationID != applicationID || audit.Revision != revision {
			continue
		}
		if !audit.Pending() || !audit.ClaimableBy(operator, now) {
			return domain.VehicleAudit{}, domain.ErrAuditClaimedByOther
		}
		f.audits[i].ClaimedBy = operator
		f.audits[i].ClaimedAt = now
		f.audits[i].Version++
		return f.audits[i], nil
	}
	return domain.VehicleAudit{}, domain.ErrAuditNotFound
}

func (f *fakeAuditStore) DecideAudit(_ context.Context, command DecideCommand) (domain.VehicleAudit, error) {
	for i := range f.audits {
		audit := f.audits[i]
		if audit.ApplicationID != command.ApplicationID || audit.Revision != command.Revision {
			continue
		}
		if !audit.Pending() || audit.Version != command.Version || audit.ClaimedBy != command.Operator {
			return domain.VehicleAudit{}, domain.ErrVersionConflict
		}
		f.audits[i].State = command.State
		f.audits[i].Reason = command.Reason
		f.audits[i].DecidedBy = command.Operator
		f.audits[i].DecidedAt = command.At
		f.audits[i].Version++
		f.audits[i].Projected = command.State != domain.AuditStateApproved
		return f.audits[i], nil
	}
	return domain.VehicleAudit{}, domain.ErrAuditNotFound
}

func (f *fakeAuditStore) ApplyVehicleSnapshot(_ context.Context, driverID string, vehicle domain.VehicleSnapshot, _ time.Time) error {
	f.applied = append(f.applied, vehicle)
	return nil
}

func (f *fakeAuditStore) EmitQualificationEvent(_ context.Context, audit domain.VehicleAudit, _ time.Time) error {
	f.events = append(f.events, audit)
	return nil
}

func (f *fakeAuditStore) MarkAuditProjected(_ context.Context, applicationID string, revision int) error {
	for i := range f.audits {
		if f.audits[i].ApplicationID == applicationID && f.audits[i].Revision == revision {
			f.audits[i].Projected = true
		}
	}
	return nil
}

func (f *fakeAuditStore) RecordOperationLog(_ context.Context, log domain.OperationLog) error {
	f.logs = append(f.logs, log)
	return nil
}

func TestAuditListEnrichesDriverAndRejectionFlag(t *testing.T) {
	store := newFakeAuditStore()
	uc := NewAudit(store)

	output, err := uc.ListAudits(context.Background(), ListAuditsInput{State: domain.AuditStatePending})
	if err != nil {
		t.Fatalf("ListAudits() error = %v", err)
	}
	if output.Total != 3 || len(output.Items) != 3 {
		t.Fatalf("ListAudits() total=%d items=%d, want 3/3", output.Total, len(output.Items))
	}
	for _, item := range output.Items {
		if item.DriverName != "王师傅" {
			t.Fatalf("DriverName = %q, want 王师傅", item.DriverName)
		}
	}
	// VA-2 有一版驳回，未达 3 次不标重点复核。
	var second AuditItem
	for _, item := range output.Items {
		if item.ApplicationID == "VA-2" {
			second = item
		}
	}
	if second.Rejections != 1 || second.ManualReview {
		t.Fatalf("VA-2 rejections=%d manual=%v, want 1/false", second.Rejections, second.ManualReview)
	}
}

func TestAuditClaimRequiresPendingAndBlocksSelf(t *testing.T) {
	store := newFakeAuditStore()
	uc := NewAudit(store)

	claimed, err := uc.Claim(context.Background(), ClaimInput{ApplicationID: "VA-1", Revision: 1, OperatorID: 7})
	if err != nil {
		t.Fatalf("Claim() error = %v", err)
	}
	if claimed.ClaimedBy != "admin:7" || claimed.Version != 2 {
		t.Fatalf("claimed by=%q version=%d, want admin:7/2", claimed.ClaimedBy, claimed.Version)
	}

	// 已被他人领取：租约未到期时拒绝。
	if _, err := uc.Claim(context.Background(), ClaimInput{ApplicationID: "VA-1", Revision: 1, OperatorID: 8}); !errors.Is(err, domain.ErrAuditClaimedByOther) {
		t.Fatalf("second Claim() error = %v, want ErrAuditClaimedByOther", err)
	}

	// 提交人不能审批自己的资料。
	if _, err := uc.Claim(context.Background(), ClaimInput{ApplicationID: "VA-3", Revision: 1, OperatorID: 1}); !errors.Is(err, domain.ErrSelfApproval) {
		t.Fatalf("self Claim() error = %v, want ErrSelfApproval", err)
	}

	// 已终结版本不可再领取。
	if _, err := uc.Claim(context.Background(), ClaimInput{ApplicationID: "VA-2", Revision: 1, OperatorID: 7}); !errors.Is(err, domain.ErrAuditNotPending) {
		t.Fatalf("rejected Claim() error = %v, want ErrAuditNotPending", err)
	}
}

func TestAuditDecideApprovesWithProjectionAndEvent(t *testing.T) {
	store := newFakeAuditStore()
	uc := NewAudit(store)

	claimed, err := uc.Claim(context.Background(), ClaimInput{ApplicationID: "VA-1", Revision: 1, OperatorID: 7})
	if err != nil {
		t.Fatalf("Claim() error = %v", err)
	}
	decided, err := uc.Decide(context.Background(), DecideInput{
		ApplicationID: "VA-1", Revision: 1, Action: domain.AuditActionApprove,
		Version: claimed.Version, OperatorID: 7, RequestID: "req-1",
	})
	if err != nil {
		t.Fatalf("Decide() error = %v", err)
	}
	if decided.State != domain.AuditStateApproved || decided.DecidedBy != "admin:7" {
		t.Fatalf("decided = %+v, want 通过 by admin:7", decided)
	}
	if len(store.applied) != 1 || store.applied[0].Plate != "沪AD·66666" {
		t.Fatalf("applied = %+v, want one vehicle projection", store.applied)
	}
	if len(store.events) != 1 {
		t.Fatalf("events = %d, want 1 qualification event", len(store.events))
	}
	// 裁决留痕 + 领取留痕
	if len(store.logs) != 2 || store.logs[1].Action != "decide_driver_audit" {
		t.Fatalf("logs = %+v, want claim+decide", store.logs)
	}
}

func TestAuditDecideRejectsStaleVersionAndUnclaimed(t *testing.T) {
	store := newFakeAuditStore()
	uc := NewAudit(store)

	// 未领取直接裁决。
	if _, err := uc.Decide(context.Background(), DecideInput{
		ApplicationID: "VA-1", Revision: 1, Action: domain.AuditActionApprove, Version: 1, OperatorID: 7,
	}); !errors.Is(err, domain.ErrAuditNotClaimed) {
		t.Fatalf("unclaimed Decide() error = %v, want ErrAuditNotClaimed", err)
	}

	claimed, err := uc.Claim(context.Background(), ClaimInput{ApplicationID: "VA-1", Revision: 1, OperatorID: 7})
	if err != nil {
		t.Fatalf("Claim() error = %v", err)
	}
	// 版本锁：用过期的 version 提交。
	if _, err := uc.Decide(context.Background(), DecideInput{
		ApplicationID: "VA-1", Revision: 1, Action: domain.AuditActionApprove, Version: claimed.Version - 1, OperatorID: 7,
	}); !errors.Is(err, domain.ErrVersionConflict) {
		t.Fatalf("stale Decide() error = %v, want ErrVersionConflict", err)
	}
	// 他人领取的版本不能裁决。
	if _, err := uc.Decide(context.Background(), DecideInput{
		ApplicationID: "VA-1", Revision: 1, Action: domain.AuditActionApprove, Version: claimed.Version, OperatorID: 8,
	}); !errors.Is(err, domain.ErrAuditClaimedByOther) {
		t.Fatalf("foreign Decide() error = %v, want ErrAuditClaimedByOther", err)
	}
}

func TestAuditRejectRequiresReasonAndSkipsProjection(t *testing.T) {
	store := newFakeAuditStore()
	uc := NewAudit(store)

	claimed, err := uc.Claim(context.Background(), ClaimInput{ApplicationID: "VA-1", Revision: 1, OperatorID: 7})
	if err != nil {
		t.Fatalf("Claim() error = %v", err)
	}
	if _, err := uc.Decide(context.Background(), DecideInput{
		ApplicationID: "VA-1", Revision: 1, Action: domain.AuditActionReject, Reason: "太短",
		Version: claimed.Version, OperatorID: 7,
	}); !errors.Is(err, domain.ErrRejectReasonRequired) {
		t.Fatalf("reject without reason error = %v, want ErrRejectReasonRequired", err)
	}
	decided, err := uc.Decide(context.Background(), DecideInput{
		ApplicationID: "VA-1", Revision: 1, Action: domain.AuditActionReject, Reason: "行驶证与车辆识别代号不一致",
		Version: claimed.Version, OperatorID: 7,
	})
	if err != nil {
		t.Fatalf("Decide(reject) error = %v", err)
	}
	if decided.State != domain.AuditStateRejected {
		t.Fatalf("decided = %+v, want 驳回", decided)
	}
	// 驳回无需投影，存储侧直接视为已投影。
	for _, audit := range store.audits {
		if audit.ApplicationID == "VA-1" && !audit.Projected {
			t.Fatalf("rejected audit projected = false, want true (no projection needed)")
		}
	}
	if len(store.applied) != 0 || len(store.events) != 0 {
		t.Fatalf("applied=%d events=%d, want none for rejection", len(store.applied), len(store.events))
	}
	// 重复裁决同一版本被拒（终态唯一）。
	if _, err := uc.Decide(context.Background(), DecideInput{
		ApplicationID: "VA-1", Revision: 1, Action: domain.AuditActionApprove, Version: decided.Version, OperatorID: 7,
	}); !errors.Is(err, domain.ErrAuditNotPending) {
		t.Fatalf("second Decide() error = %v, want ErrAuditNotPending", err)
	}
}

func TestAuditHistoryCountsRejectionsAndFlagsManualReview(t *testing.T) {
	store := newFakeAuditStore()
	// 追加两次驳回，凑满 3 次重点复核阈值。
	for i := 3; i <= 4; i++ {
		store.audits = append(store.audits, domain.VehicleAudit{
			ApplicationID: "VA-2", Revision: i, DriverID: "drv-1",
			Submitter: "account:acc-1", State: domain.AuditStateRejected,
			Version: 2, Reason: "车辆照片再次不清晰",
		})
	}
	uc := NewAudit(store)
	output, err := uc.History(context.Background(), "VA-2")
	if err != nil {
		t.Fatalf("History() error = %v", err)
	}
	if output.Rejections != 3 || !output.ManualReview {
		t.Fatalf("history rejections=%d manual=%v, want 3/true", output.Rejections, output.ManualReview)
	}
	if len(output.Revisions) != 4 {
		t.Fatalf("revisions = %d, want 4", len(output.Revisions))
	}
}

func TestAuditApproveLazilyReconcilesUnprojectedRows(t *testing.T) {
	store := newFakeAuditStore()
	// 模拟“裁决成功但投影未完成”：已通过、projected=false。
	store.audits = append(store.audits, domain.VehicleAudit{
		ApplicationID: "VA-9", Revision: 1, DriverID: "drv-1",
		Submitter: "account:acc-1", State: domain.AuditStateApproved,
		Vehicle: domain.VehicleSnapshot{Plate: "沪AD·12345", Model: "埃安S", Seats: 4, CertNo: "YS-1", CertExpire: "2028-01-31"},
		Version: 2, Projected: false,
	})
	uc := NewAudit(store)
	if _, err := uc.ListAudits(context.Background(), ListAuditsInput{}); err != nil {
		t.Fatalf("ListAudits() error = %v", err)
	}
	if len(store.applied) != 1 || len(store.events) != 1 {
		t.Fatalf("applied=%d events=%d, want lazy reconcile 1/1", len(store.applied), len(store.events))
	}
}
