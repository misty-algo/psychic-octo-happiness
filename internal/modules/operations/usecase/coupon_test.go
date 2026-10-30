package usecase

import (
	"testing"
	"time"

	"github.com/NSObjects/echo-admin/internal/modules/operations/domain"
)

var (
	couponValidStart = time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	couponValidEnd   = time.Date(2026, 12, 31, 23, 59, 59, 0, time.UTC)
)

func newTestTemplate(t *testing.T) domain.CouponTemplate {
	t.Helper()
	template, err := domain.NewCouponTemplate("TPL-001", "满20减5", domain.CouponTypeCash,
		500, 2000, 0, 0, domain.CouponBearerPlatform, couponValidStart, couponValidEnd,
		1000, 2, true, nil, "新客首单可用")
	if err != nil {
		t.Fatalf("new template: %v", err)
	}
	return template
}

// ---- 券模板校验 ----

func TestNewCouponTemplateRejectsFaceValueAboveThreshold(t *testing.T) {
	// 面额超过门槛等于免单还倒贴，属于配置事故，必须在创建时就拦下。
	_, err := domain.NewCouponTemplate("TPL-BAD", "满20减30", domain.CouponTypeCash,
		3000, 2000, 0, 0, domain.CouponBearerPlatform, couponValidStart, couponValidEnd,
		100, 1, false, nil, "")
	if err == nil {
		t.Fatal("face value above threshold must be rejected")
	}
}

func TestNewCouponTemplateRejectsBadStockAndPerLimit(t *testing.T) {
	if _, err := domain.NewCouponTemplate("TPL-S0", "零库存", domain.CouponTypeCash,
		500, 2000, 0, 0, domain.CouponBearerPlatform, couponValidStart, couponValidEnd,
		0, 1, false, nil, ""); err == nil {
		t.Fatal("zero stock must be rejected")
	}
	// 每人限领不能超过总库存，否则永远发不满。
	if _, err := domain.NewCouponTemplate("TPL-PL", "限领超库存", domain.CouponTypeCash,
		500, 2000, 0, 0, domain.CouponBearerPlatform, couponValidStart, couponValidEnd,
		10, 20, false, nil, ""); err == nil {
		t.Fatal("per-limit above stock must be rejected")
	}
}

func TestNewCouponTemplateRejectsBadValidity(t *testing.T) {
	_, err := domain.NewCouponTemplate("TPL-V", "倒挂有效期", domain.CouponTypeCash,
		500, 2000, 0, 0, domain.CouponBearerPlatform, couponValidEnd, couponValidStart,
		100, 1, false, nil, "")
	if err == nil {
		t.Fatal("end before start must be rejected")
	}
}

func TestNewCouponTemplateDiscountNeedsRate(t *testing.T) {
	if _, err := domain.NewCouponTemplate("TPL-D0", "零折扣", domain.CouponTypeDiscount,
		0, 0, 0, 0, domain.CouponBearerPlatform, couponValidStart, couponValidEnd,
		100, 1, false, nil, ""); err == nil {
		t.Fatal("discount coupon without rate must be rejected")
	}
	if _, err := domain.NewCouponTemplate("TPL-D100", "全额免", domain.CouponTypeDiscount,
		0, 0, 100, 0, domain.CouponBearerPlatform, couponValidStart, couponValidEnd,
		100, 1, false, nil, ""); err == nil {
		t.Fatal("discount rate 100 must be rejected")
	}
}

// ---- 状态机 ----

func TestCouponTemplateStateMachine(t *testing.T) {
	template := newTestTemplate(t)
	if template.State != domain.CouponTemplateStateDraft {
		t.Fatalf("new template must be draft, got %s", template.State)
	}
	if err := template.CanStop(); err == nil {
		t.Fatal("draft cannot be stopped before publishing")
	}
	if err := template.CanPublish(couponValidStart.Add(time.Hour)); err != nil {
		t.Fatalf("draft must be publishable: %v", err)
	}
	if err := template.CanIssue(couponValidStart.Add(time.Hour)); err == nil {
		t.Fatal("draft cannot be issued")
	}
	template.State = domain.CouponTemplateStatePublished
	if err := template.CanPublish(couponValidStart.Add(time.Hour)); err == nil {
		t.Fatal("published template cannot be published again")
	}
	// 有效期之外不能发券。
	if err := template.CanIssue(couponValidEnd.Add(time.Hour)); err == nil {
		t.Fatal("expired template cannot be issued")
	}
	template.State = domain.CouponTemplateStateStopped
	if err := template.CanIssue(couponValidStart.Add(time.Hour)); err == nil {
		t.Fatal("stopped template cannot be issued")
	}
}

// ---- 叠加互斥 ----

func TestConflictsWithHonoursBothDirections(t *testing.T) {
	base := newTestTemplate(t)
	base.State = domain.CouponTemplateStatePublished
	base.Stackable = false
	other := newTestTemplate(t)
	other.TemplateNo = "TPL-002"
	other.State = domain.CouponTemplateStatePublished
	other.Stackable = false

	// 不可叠加约束的是"同一笔订单能否叠加核销"，不等于"不能同时持有"。
	// 若在此处判冲突，手里已有券的乘客将永远领不到新券。
	if base.ConflictsWith(other) {
		t.Fatal("non-stackable coupons must still be issuable to a holder of other coupons")
	}

	base.Stackable = true
	other.Stackable = true
	if base.ConflictsWith(other) {
		t.Fatal("two stackable coupons must not conflict")
	}

	// 单向写进互斥清单也算冲突。
	base.MutualTemplates = []string{"TPL-002"}
	if !base.ConflictsWith(other) {
		t.Fatal("mutual list must be honoured")
	}
	base.MutualTemplates = nil
	other.MutualTemplates = []string{"TPL-001"}
	if !base.ConflictsWith(other) {
		t.Fatal("reverse mutual list must be honoured")
	}
}

func TestConflictsWithIgnoresUnpublishedAndSelf(t *testing.T) {
	base := newTestTemplate(t)
	base.State = domain.CouponTemplateStatePublished
	base.Stackable = false
	draft := newTestTemplate(t)
	draft.TemplateNo = "TPL-003"
	draft.State = domain.CouponTemplateStateDraft
	if base.ConflictsWith(draft) {
		t.Fatal("draft template cannot block issuance")
	}
	self := base
	if base.ConflictsWith(self) {
		t.Fatal("same template is handled by per-limit, not mutual exclusion")
	}
}

// ---- 券批次 ----

func TestCouponBatchCanIssue(t *testing.T) {
	batch, err := domain.NewCouponBatch("BATCH-001", "TPL-001", "满20减5", domain.CouponChannelManual,
		100, 0, couponValidStart, couponValidEnd, "")
	if err != nil {
		t.Fatalf("new batch: %v", err)
	}
	now := time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)
	if err := batch.CanIssue(50, now); err != nil {
		t.Fatalf("batch must accept 50: %v", err)
	}
	if err := batch.CanIssue(101, now); err == nil {
		t.Fatal("issuing more than total must be rejected")
	}
	batch.IssuedCount = 100
	if err := batch.CanIssue(1, now); err == nil {
		t.Fatal("exhausted batch must be rejected")
	}
	if batch.RemainingCount() != 0 {
		t.Fatalf("remaining must be 0, got %d", batch.RemainingCount())
	}
	if err := batch.CanIssue(0, now); err == nil {
		t.Fatal("zero count must be rejected")
	}
	if err := batch.CanIssue(domain.MaxIssuePerRequest+1, now); err == nil {
		t.Fatal("above per-request cap must be rejected")
	}
}

func TestCouponBatchRespectsWindowAndStatus(t *testing.T) {
	batch, _ := domain.NewCouponBatch("BATCH-002", "TPL-001", "满20减5", domain.CouponChannelManual,
		100, 0, couponValidStart, couponValidEnd, "")
	before := couponValidStart.Add(-time.Hour)
	if err := batch.CanIssue(1, before); err == nil {
		t.Fatal("before start must be rejected")
	}
	batch.Status = domain.CouponBatchStateStopped
	if err := batch.CanIssue(1, time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)); err == nil {
		t.Fatal("stopped batch must be rejected")
	}
}

// ---- 活动 ----

func TestCampaignStateMachine(t *testing.T) {
	campaign, err := domain.NewCampaign("CAMP-001", "新客礼包", domain.CampaignTypeNewUserGift,
		"passenger", "注册后发放", 0, 100000, couponValidStart, couponValidEnd)
	if err != nil {
		t.Fatalf("new campaign: %v", err)
	}
	if campaign.Status != domain.CampaignStateDraft {
		t.Fatalf("new campaign must be draft, got %s", campaign.Status)
	}
	if err := campaign.CanPause(); err == nil {
		t.Fatal("draft cannot be paused")
	}
	if err := campaign.CanEnd(); err == nil {
		t.Fatal("draft cannot be ended")
	}
	if err := campaign.CanStart(); err != nil {
		t.Fatalf("draft must be startable: %v", err)
	}
	campaign.Status = domain.CampaignStateRunning
	if err := campaign.CanStart(); err == nil {
		t.Fatal("running cannot start again")
	}
	if err := campaign.CanPause(); err != nil {
		t.Fatalf("running must be pausable: %v", err)
	}
	campaign.Status = domain.CampaignStateEnded
	if err := campaign.CanEnd(); err == nil {
		t.Fatal("ended cannot end again")
	}
}

func TestNewCampaignRejectsRewardAboveBudget(t *testing.T) {
	if _, err := domain.NewCampaign("CAMP-BAD", "超预算", domain.CampaignTypeInvite,
		"passenger", "", 200000, 100000, couponValidStart, couponValidEnd); err == nil {
		t.Fatal("reward above budget must be rejected")
	}
}

// ---- 限流器 ----

func TestIssueLimiterBlocksBeyondWindow(t *testing.T) {
	limiter := NewIssueLimiter()
	now := time.Date(2026, 6, 1, 12, 0, 0, 0, time.UTC)
	if !limiter.Allow("BATCH-1", 10, 6, now) {
		t.Fatal("first 6 must pass")
	}
	if limiter.Allow("BATCH-1", 10, 6, now) {
		t.Fatal("next 6 must be limited: only 4 left in window")
	}
	if !limiter.Allow("BATCH-1", 10, 4, now) {
		t.Fatal("exactly the remaining 4 must pass")
	}
	// 过了窗口就该放行。
	later := now.Add(2 * time.Minute)
	if !limiter.Allow("BATCH-1", 10, 10, later) {
		t.Fatal("after the window the counter must reset")
	}
	// 不同批次各算各的。
	if !limiter.Allow("BATCH-2", 10, 10, now) {
		t.Fatal("other batches must have their own counter")
	}
}

func TestIssueLimiterZeroLimitMeansUnlimited(t *testing.T) {
	limiter := NewIssueLimiter()
	if !limiter.Allow("BATCH-1", 0, 1000, time.Now()) {
		t.Fatal("limit 0 must mean no throttling")
	}
}

// ---- 分页工具 ----

func TestSkipAndLimitOf(t *testing.T) {
	if got := limitOf(0); got != 20 {
		t.Fatalf("invalid page size must fall back to 20, got %d", got)
	}
	if got := limitOf(-5); got != 20 {
		t.Fatalf("negative page size must fall back to 20, got %d", got)
	}
	if got := limitOf(5000); got != maxPageSize {
		t.Fatalf("page size must be capped, got %d", got)
	}
	if got := skipOf(0, 20); got != 0 {
		t.Fatalf("page 0 must skip 0, got %d", got)
	}
	if got := skipOf(3, 20); got != 40 {
		t.Fatalf("page 3 must skip 40, got %d", got)
	}
}

func TestPercentOfAvoidsDivideByZero(t *testing.T) {
	if got := percentOf(5, 0); got != 0 {
		t.Fatalf("zero total must yield 0, got %f", got)
	}
	if got := percentOf(25, 100); got != 25 {
		t.Fatalf("expected 25, got %f", got)
	}
}

func TestYuanFormatting(t *testing.T) {
	if got := yuan(1500); got != "15.00" {
		t.Fatalf("expected 15.00, got %s", got)
	}
	if got := yuan(5); got != "0.05" {
		t.Fatalf("expected 0.05, got %s", got)
	}
	if got := yuan(0); got != "0.00" {
		t.Fatalf("expected 0.00, got %s", got)
	}
}
