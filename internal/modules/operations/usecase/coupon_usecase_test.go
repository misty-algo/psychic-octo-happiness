package usecase

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/NSObjects/echo-admin/internal/modules/operations/domain"
)

// fakeMarketingStore 是内存实现。CAS 语义照搬 Mongo 侧的条件更新，否则
// "防超发"的测试会测出假绿。
type fakeMarketingStore struct {
	templates  map[string]domain.CouponTemplate
	batches    map[string]domain.CouponBatch
	coupons    []domain.IssuedCoupon
	passengers map[string]domain.Account
	campaigns  []domain.Campaign
	config     map[string]int
	logs       []domain.OperationLog
	subsidies  []domain.SubsidyRecord
	dailyTotal map[string]int64
	// A05/A06/A09 配置集合
	pricingRules  []domain.PricingRule
	strategies    []domain.DispatchStrategy
	poolConfigs   []domain.PoolConfig
	specialPlans  []domain.SpecialPricePlan

	// stockFail 模拟库存扣减失败，用来验证额度是否会正确回滚。
	stockFail bool
	// quotaReleased / stockRestored 记录补偿动作是否发生。
	quotaReleased int64
	stockRestored int64
	couponSeq     int
}

func (f *fakeMarketingStore) ListCouponTemplates(_ context.Context, _ CouponTemplateFilter) ([]domain.CouponTemplate, int64, error) {
	out := make([]domain.CouponTemplate, 0, len(f.templates))
	for _, t := range f.templates {
		out = append(out, t)
	}
	return out, int64(len(out)), nil
}

func (f *fakeMarketingStore) FindCouponTemplate(_ context.Context, templateNo string) (domain.CouponTemplate, error) {
	t, ok := f.templates[templateNo]
	if !ok {
		return domain.CouponTemplate{}, domain.ErrCouponTemplateNotFound
	}
	return t, nil
}

func (f *fakeMarketingStore) FindCouponTemplates(_ context.Context, templateNos []string) (map[string]domain.CouponTemplate, error) {
	out := make(map[string]domain.CouponTemplate, len(templateNos))
	for _, no := range templateNos {
		if t, ok := f.templates[no]; ok {
			out[no] = t
		}
	}
	return out, nil
}

func (f *fakeMarketingStore) CreateCouponTemplate(_ context.Context, t domain.CouponTemplate) (domain.CouponTemplate, error) {
	f.templates[t.TemplateNo] = t
	return t, nil
}

func (f *fakeMarketingStore) UpdateCouponTemplate(_ context.Context, templateNo string, set CouponTemplateUpdate) (domain.CouponTemplate, error) {
	t := f.templates[templateNo]
	if set.State != "" {
		t.State = set.State
	}
	t.IssuedCount += set.IssuedBy
	t.UsedCount += set.UsedBy
	f.templates[templateNo] = t
	return t, nil
}

func (f *fakeMarketingStore) ConsumeTemplateStock(_ context.Context, templateNo string, count int64, _ time.Time) (bool, error) {
	if f.stockFail {
		return false, nil
	}
	t := f.templates[templateNo]
	if t.RemainingStock() < count {
		return false, nil
	}
	t.Stock -= count
	t.IssuedCount += count
	f.templates[templateNo] = t
	return true, nil
}

func (f *fakeMarketingStore) RestoreTemplateStock(_ context.Context, templateNo string, count int64) error {
	t := f.templates[templateNo]
	t.Stock += count
	t.IssuedCount -= count
	f.templates[templateNo] = t
	f.stockRestored += count
	return nil
}

func (f *fakeMarketingStore) ListCouponBatches(_ context.Context, _ CouponBatchFilter) ([]domain.CouponBatch, int64, error) {
	out := make([]domain.CouponBatch, 0, len(f.batches))
	for _, b := range f.batches {
		out = append(out, b)
	}
	return out, int64(len(out)), nil
}

func (f *fakeMarketingStore) FindCouponBatch(_ context.Context, batchNo string) (domain.CouponBatch, error) {
	b, ok := f.batches[batchNo]
	if !ok {
		return domain.CouponBatch{}, domain.ErrCouponBatchNotFound
	}
	return b, nil
}

func (f *fakeMarketingStore) CreateCouponBatch(_ context.Context, b domain.CouponBatch) (domain.CouponBatch, error) {
	f.batches[b.BatchNo] = b
	return b, nil
}

// ReserveBatchQuota 复刻了 Mongo 侧的条件更新：issuedCount + count 超过
// totalCount 就直接不写，这是防超发的唯一闸门。
func (f *fakeMarketingStore) ReserveBatchQuota(_ context.Context, batchNo string, count int64, _ time.Time) (bool, error) {
	b := f.batches[batchNo]
	if b.Status != domain.CouponBatchStateRunning {
		return false, nil
	}
	if b.IssuedCount+count > b.TotalCount {
		return false, nil
	}
	b.IssuedCount += count
	f.batches[batchNo] = b
	return true, nil
}

func (f *fakeMarketingStore) ReleaseBatchQuota(_ context.Context, batchNo string, count int64) error {
	b := f.batches[batchNo]
	b.IssuedCount -= count
	f.batches[batchNo] = b
	f.quotaReleased += count
	return nil
}

func (f *fakeMarketingStore) UpdateCouponBatch(_ context.Context, batchNo string, set CouponBatchUpdate) (domain.CouponBatch, error) {
	b := f.batches[batchNo]
	if set.Status != "" {
		b.Status = set.Status
	}
	b.IssuedCount += set.IssuedBy
	f.batches[batchNo] = b
	return b, nil
}

func (f *fakeMarketingStore) ListBatchesByCampaign(_ context.Context, campaignCode string) ([]domain.CouponBatch, error) {
	out := make([]domain.CouponBatch, 0)
	for _, b := range f.batches {
		if b.CampaignCode == campaignCode {
			out = append(out, b)
		}
	}
	return out, nil
}

func (f *fakeMarketingStore) CountHeldCoupons(_ context.Context, passengerIDs []string, templateNo string) (map[string]int64, error) {
	out := make(map[string]int64)
	for _, c := range f.coupons {
		if c.TemplateNo != templateNo {
			continue
		}
		for _, id := range passengerIDs {
			if id == c.PassengerID {
				out[id]++
			}
		}
	}
	return out, nil
}

func (f *fakeMarketingStore) ListHeldTemplateNos(_ context.Context, passengerIDs []string) (map[string][]string, error) {
	out := make(map[string][]string)
	for _, c := range f.coupons {
		if c.State != domain.CouponStateAvailable {
			continue
		}
		for _, id := range passengerIDs {
			if id == c.PassengerID {
				out[id] = append(out[id], c.TemplateNo)
			}
		}
	}
	return out, nil
}

func (f *fakeMarketingStore) FindPassengers(_ context.Context, ids []string) (map[string]domain.Account, error) {
	out := make(map[string]domain.Account)
	for _, id := range ids {
		if account, ok := f.passengers[id]; ok {
			out[id] = account
		}
	}
	return out, nil
}

func (f *fakeMarketingStore) InsertCoupons(_ context.Context, coupons []domain.IssuedCoupon) (int, error) {
	for i := range coupons {
		f.couponSeq++
		coupons[i].CouponNo = "CP-TEST-" + time.Now().Format("20060102") + "-" + string(rune('A'+f.couponSeq%26))
		f.coupons = append(f.coupons, coupons[i])
	}
	return len(coupons), nil
}

func (f *fakeMarketingStore) ListIssuedCoupons(_ context.Context, _ IssuedCouponFilter) ([]domain.IssuedCoupon, int64, error) {
	return f.coupons, int64(len(f.coupons)), nil
}

func (f *fakeMarketingStore) CountCouponsByBatch(_ context.Context, batchNos []string) (map[string]CouponStat, error) {
	out := make(map[string]CouponStat)
	for _, c := range f.coupons {
		for _, no := range batchNos {
			if c.BatchNo != no {
				continue
			}
			stat := out[no]
			stat.Issued++
			switch c.State {
			case domain.CouponStateUsed:
				stat.Used++
			case domain.CouponStateExpired:
				stat.Expired++
			}
			out[no] = stat
		}
	}
	return out, nil
}

func (f *fakeMarketingStore) ListCampaigns(_ context.Context, _ CampaignFilter) ([]domain.Campaign, int64, error) {
	return []domain.Campaign{}, 0, nil
}

func (f *fakeMarketingStore) FindCampaign(_ context.Context, code string) (domain.Campaign, error) {
	for _, c := range f.campaigns {
		if c.Code == code {
			return c, nil
		}
	}
	return domain.Campaign{}, domain.ErrCampaignNotFound
}

func (f *fakeMarketingStore) CreateCampaign(_ context.Context, c domain.Campaign) (domain.Campaign, error) {
	f.campaigns = append(f.campaigns, c)
	return c, nil
}

func (f *fakeMarketingStore) UpdateCampaign(_ context.Context, code string, set CampaignUpdate) (domain.Campaign, error) {
	for i := range f.campaigns {
		if f.campaigns[i].Code == code {
			if set.Status != "" {
				f.campaigns[i].Status = set.Status
			}
			return f.campaigns[i], nil
		}
	}
	return domain.Campaign{}, domain.ErrCampaignNotFound
}

// ---- 补贴台账（A08）的内存实现：CAS 语义照搬 Mongo 侧 ----

func (f *fakeMarketingStore) ReserveCampaignBudget(_ context.Context, code string, amountFen int64) (bool, error) {
	for i := range f.campaigns {
		if f.campaigns[i].Code != code {
			continue
		}
		if f.campaigns[i].UsedBudgetFen+amountFen > f.campaigns[i].BudgetFen {
			return false, nil
		}
		f.campaigns[i].UsedBudgetFen += amountFen
		return true, nil
	}
	return false, domain.ErrCampaignNotFound
}

func (f *fakeMarketingStore) ReleaseCampaignBudget(_ context.Context, code string, amountFen int64) error {
	for i := range f.campaigns {
		if f.campaigns[i].Code == code {
			f.campaigns[i].UsedBudgetFen -= amountFen
			return nil
		}
	}
	return domain.ErrCampaignNotFound
}

func (f *fakeMarketingStore) CountSubsidiesByDriver(_ context.Context, campaignCode, driverID string) (int64, error) {
	var n int64
	for _, r := range f.subsidies {
		if r.CampaignCode == campaignCode && r.DriverID == driverID {
			n++
		}
	}
	return n, nil
}

func (f *fakeMarketingStore) ReserveSubsidyDaily(_ context.Context, campaignCode, day string, capFen, amountFen int64) (bool, error) {
	key := campaignCode + "|" + day
	if f.dailyTotal[key]+amountFen > capFen {
		return false, nil
	}
	f.dailyTotal[key] += amountFen
	return true, nil
}

func (f *fakeMarketingStore) InsertSubsidyRecord(_ context.Context, record domain.SubsidyRecord) error {
	f.subsidies = append(f.subsidies, record)
	return nil
}

func (f *fakeMarketingStore) ListSubsidyRecords(_ context.Context, filter SubsidyRecordFilter) ([]domain.SubsidyRecord, int64, error) {
	out := make([]domain.SubsidyRecord, 0)
	for _, r := range f.subsidies {
		if filter.CampaignCode != "" && r.CampaignCode != filter.CampaignCode {
			continue
		}
		if filter.DriverID != "" && r.DriverID != filter.DriverID {
			continue
		}
		if filter.BizType != "" && r.BizType != filter.BizType {
			continue
		}
		if filter.Day != "" && r.Day != filter.Day {
			continue
		}
		out = append(out, r)
	}
	return out, int64(len(out)), nil
}

func (f *fakeMarketingStore) SummarizeSubsidies(_ context.Context, campaignCode, today string) (SubsidyAggregate, error) {
	var agg SubsidyAggregate
	drivers := map[string]bool{}
	for _, r := range f.subsidies {
		if r.CampaignCode != campaignCode {
			continue
		}
		agg.GrantedTotalFen += r.AmountFen
		agg.RecordCount++
		drivers[r.DriverID] = true
		if r.Day == today {
			agg.TodayGrantedFen += r.AmountFen
		}
	}
	agg.DriverCount = int64(len(drivers))
	return agg, nil
}

func (f *fakeMarketingStore) ReadMarketingConfig(_ context.Context, key string, fallback int) int {
	if v, ok := f.config[key]; ok {
		return v
	}
	return fallback
}

func (f *fakeMarketingStore) RecordOperationLog(_ context.Context, log domain.OperationLog) error {
	f.logs = append(f.logs, log)
	return nil
}

// ---- 测试装配 ----

type marketingFixture struct {
	uc      *MarketingUsecase
	store   *fakeMarketingStore
	now     time.Time
	advance func(d time.Duration)
}

func newMarketingFixture(t *testing.T) *marketingFixture {
	t.Helper()
	current := time.Date(2026, 6, 1, 12, 0, 0, 0, time.UTC)
	store := &fakeMarketingStore{
		templates:  map[string]domain.CouponTemplate{},
		batches:    map[string]domain.CouponBatch{},
		passengers: map[string]domain.Account{},
		config:     map[string]int{},
		dailyTotal: map[string]int64{},
	}
	uc := NewMarketing(store)
	uc.now = func() time.Time { return current }
	uc.limiter.Reset()
	f := &marketingFixture{uc: uc, store: store, now: current}
	f.advance = func(d time.Duration) { current = current.Add(d) }
	return f
}

func (f *marketingFixture) addTemplate(template domain.CouponTemplate) {
	template.State = domain.CouponTemplateStatePublished
	f.store.templates[template.TemplateNo] = template
}

func (f *marketingFixture) addBatch(batch domain.CouponBatch) {
	batch.Status = domain.CouponBatchStateRunning
	f.store.batches[batch.BatchNo] = batch
}

func (f *marketingFixture) addPassenger(id string) {
	f.store.passengers[id] = domain.Account{ID: id, Phone: "1380000" + id, RealName: "乘客" + id}
}

// ---- 用例测试 ----

func TestIssueCouponsWritesCouponsAndConsumesQuota(t *testing.T) {
	f := newMarketingFixture(t)
	f.addTemplate(domain.CouponTemplate{
		TemplateNo: "TPL-1", Name: "满20减5", CouponType: domain.CouponTypeCash,
		FaceValueFen: 500, ThresholdFen: 2000, Bearer: domain.CouponBearerPlatform,
		ValidStart: f.now.Add(-time.Hour), ValidEnd: f.now.Add(24 * time.Hour),
		Stock: 100, PerLimit: 2, Stackable: true,
	})
	f.addBatch(domain.CouponBatch{
		BatchNo: "B-1", TemplateNo: "TPL-1", TotalCount: 50,
		Channel: domain.CouponChannelManual, StartAt: f.now.Add(-time.Hour), EndAt: f.now.Add(time.Hour),
	})
	f.addPassenger("p1")
	f.addPassenger("p2")

	out, err := f.uc.IssueCoupons(context.Background(), "B-1", IssueCouponsInput{PassengerIDs: []string{"p1", "p2"}, OperatorID: 1})
	if err != nil {
		t.Fatalf("issue: %v", err)
	}
	if out.Issued != 2 {
		t.Fatalf("expected 2 issued, got %d", out.Issued)
	}
	if len(out.Skipped) != 0 {
		t.Fatalf("unexpected skips: %+v", out.Skipped)
	}
	if f.store.batches["B-1"].IssuedCount != 2 {
		t.Fatalf("batch quota must be consumed, got %d", f.store.batches["B-1"].IssuedCount)
	}
	if f.store.templates["TPL-1"].Stock != 98 {
		t.Fatalf("template stock must drop by 2, got %d", f.store.templates["TPL-1"].Stock)
	}
	if len(f.store.coupons) != 2 {
		t.Fatalf("expected 2 coupons written, got %d", len(f.store.coupons))
	}
	// 券码必须回填，运营要能查到发出去的是哪几张。
	if len(out.CouponNos) != 2 {
		t.Fatalf("coupon nos must be returned, got %d", len(out.CouponNos))
	}
	if out.Remaining != 48 {
		t.Fatalf("remaining must be 48, got %d", out.Remaining)
	}
	if len(f.store.logs) == 0 {
		t.Fatal("issuing must leave an audit trail")
	}
}

func TestIssueCouponsNeverExceedsBatchTotal(t *testing.T) {
	f := newMarketingFixture(t)
	f.addTemplate(domain.CouponTemplate{
		TemplateNo: "TPL-1", Name: "满20减5", CouponType: domain.CouponTypeCash,
		FaceValueFen: 500, ThresholdFen: 2000, Bearer: domain.CouponBearerPlatform,
		ValidStart: f.now.Add(-time.Hour), ValidEnd: f.now.Add(24 * time.Hour),
		Stock: 500, PerLimit: 10, Stackable: true,
	})
	f.addBatch(domain.CouponBatch{
		BatchNo: "B-1", TemplateNo: "TPL-1", TotalCount: 3,
		Channel: domain.CouponChannelManual, StartAt: f.now.Add(-time.Hour), EndAt: f.now.Add(time.Hour),
	})
	for _, id := range []string{"p1", "p2", "p3", "p4", "p5"} {
		f.addPassenger(id)
	}

	_, err := f.uc.IssueCoupons(context.Background(), "B-1",
		IssueCouponsInput{PassengerIDs: []string{"p1", "p2", "p3", "p4", "p5"}, OperatorID: 1})
	if err == nil {
		t.Fatal("issuing 5 when total is 3 must fail")
	}
	// 被拒时一张都不能发出去，配额也不能被占。
	if len(f.store.coupons) != 0 {
		t.Fatalf("no coupon must be written, got %d", len(f.store.coupons))
	}
	if f.store.batches["B-1"].IssuedCount != 0 {
		t.Fatalf("quota must stay untouched, got %d", f.store.batches["B-1"].IssuedCount)
	}
}

func TestIssueCouponsReleasesQuotaWhenStockFails(t *testing.T) {
	f := newMarketingFixture(t)
	f.addTemplate(domain.CouponTemplate{
		TemplateNo: "TPL-1", Name: "满20减5", CouponType: domain.CouponTypeCash,
		FaceValueFen: 500, ThresholdFen: 2000, Bearer: domain.CouponBearerPlatform,
		ValidStart: f.now.Add(-time.Hour), ValidEnd: f.now.Add(24 * time.Hour),
		Stock: 100, PerLimit: 5, Stackable: true,
	})
	f.addBatch(domain.CouponBatch{
		BatchNo: "B-1", TemplateNo: "TPL-1", TotalCount: 50,
		Channel: domain.CouponChannelManual, StartAt: f.now.Add(-time.Hour), EndAt: f.now.Add(time.Hour),
	})
	f.addPassenger("p1")
	f.store.stockFail = true

	if _, err := f.uc.IssueCoupons(context.Background(), "B-1",
		IssueCouponsInput{PassengerIDs: []string{"p1"}, OperatorID: 1}); err == nil {
		t.Fatal("stock failure must surface as an error")
	}
	// 关键：额度被占了但库存没扣成，必须还回去，否则账不平。
	if f.store.quotaReleased != 1 {
		t.Fatalf("quota must be rolled back, released=%d", f.store.quotaReleased)
	}
	if f.store.batches["B-1"].IssuedCount != 0 {
		t.Fatalf("batch must be back to 0, got %d", f.store.batches["B-1"].IssuedCount)
	}
}

func TestIssueCouponsSkipsPerLimitReached(t *testing.T) {
	f := newMarketingFixture(t)
	f.addTemplate(domain.CouponTemplate{
		TemplateNo: "TPL-1", Name: "满20减5", CouponType: domain.CouponTypeCash,
		FaceValueFen: 500, ThresholdFen: 2000, Bearer: domain.CouponBearerPlatform,
		ValidStart: f.now.Add(-time.Hour), ValidEnd: f.now.Add(24 * time.Hour),
		Stock: 100, PerLimit: 1, Stackable: true,
	})
	f.addBatch(domain.CouponBatch{
		BatchNo: "B-1", TemplateNo: "TPL-1", TotalCount: 50,
		Channel: domain.CouponChannelManual, StartAt: f.now.Add(-time.Hour), EndAt: f.now.Add(time.Hour),
	})
	f.addPassenger("p1")
	f.addPassenger("p2")
	// p1 已经拿过一张同模板的券。
	f.store.coupons = append(f.store.coupons, domain.IssuedCoupon{
		CouponNo: "CP-OLD", TemplateNo: "TPL-1", PassengerID: "p1", State: domain.CouponStateAvailable,
	})

	out, err := f.uc.IssueCoupons(context.Background(), "B-1",
		IssueCouponsInput{PassengerIDs: []string{"p1", "p2"}, OperatorID: 1})
	if err != nil {
		t.Fatalf("issue: %v", err)
	}
	if out.Issued != 1 {
		t.Fatalf("only p2 should receive one, got %d", out.Issued)
	}
	if len(out.Skipped) != 1 || out.Skipped[0].PassengerID != "p1" {
		t.Fatalf("p1 must be skipped with a reason, got %+v", out.Skipped)
	}
}

func TestIssueCouponsSkipsMutuallyExclusiveHolder(t *testing.T) {
	f := newMarketingFixture(t)
	f.addTemplate(domain.CouponTemplate{
		TemplateNo: "TPL-A", Name: "券A", CouponType: domain.CouponTypeCash,
		FaceValueFen: 500, ThresholdFen: 2000, Bearer: domain.CouponBearerPlatform,
		ValidStart: f.now.Add(-time.Hour), ValidEnd: f.now.Add(24 * time.Hour),
		Stock: 100, PerLimit: 2, Stackable: false,
		// 显式互斥：券A 与券B 不能共存。stackable 只约束核销阶段的叠加。
		MutualTemplates: []string{"TPL-B"},
	})
	f.addTemplate(domain.CouponTemplate{
		TemplateNo: "TPL-B", Name: "券B", CouponType: domain.CouponTypeCash,
		FaceValueFen: 300, ThresholdFen: 2000, Bearer: domain.CouponBearerPlatform,
		ValidStart: f.now.Add(-time.Hour), ValidEnd: f.now.Add(24 * time.Hour),
		Stock: 100, PerLimit: 2, Stackable: false, State: domain.CouponTemplateStatePublished,
	})
	f.addBatch(domain.CouponBatch{
		BatchNo: "B-1", TemplateNo: "TPL-A", TotalCount: 50,
		Channel: domain.CouponChannelManual, StartAt: f.now.Add(-time.Hour), EndAt: f.now.Add(time.Hour),
	})
	f.addPassenger("p1")
	f.addPassenger("p2")
	// p1 手上已经有一张别家的可用券，且两边都不可叠加。
	f.store.coupons = append(f.store.coupons, domain.IssuedCoupon{
		CouponNo: "CP-OLD", TemplateNo: "TPL-B", PassengerID: "p1", State: domain.CouponStateAvailable,
	})

	out, err := f.uc.IssueCoupons(context.Background(), "B-1",
		IssueCouponsInput{PassengerIDs: []string{"p1", "p2"}, OperatorID: 1})
	if err != nil {
		t.Fatalf("issue: %v", err)
	}
	if out.Issued != 1 {
		t.Fatalf("only p2 should receive one, got %d", out.Issued)
	}
	if len(out.Skipped) != 1 || out.Skipped[0].PassengerID != "p1" {
		t.Fatalf("p1 must be skipped as exclusive, got %+v", out.Skipped)
	}
}

func TestIssueCouponsSkipsUnknownPassenger(t *testing.T) {
	f := newMarketingFixture(t)
	f.addTemplate(domain.CouponTemplate{
		TemplateNo: "TPL-1", Name: "满20减5", CouponType: domain.CouponTypeCash,
		FaceValueFen: 500, ThresholdFen: 2000, Bearer: domain.CouponBearerPlatform,
		ValidStart: f.now.Add(-time.Hour), ValidEnd: f.now.Add(24 * time.Hour),
		Stock: 100, PerLimit: 2, Stackable: true,
	})
	f.addBatch(domain.CouponBatch{
		BatchNo: "B-1", TemplateNo: "TPL-1", TotalCount: 50,
		Channel: domain.CouponChannelManual, StartAt: f.now.Add(-time.Hour), EndAt: f.now.Add(time.Hour),
	})
	f.addPassenger("p1")

	out, err := f.uc.IssueCoupons(context.Background(), "B-1",
		IssueCouponsInput{PassengerIDs: []string{"p1", "ghost"}, OperatorID: 1})
	if err != nil {
		t.Fatalf("issue: %v", err)
	}
	if out.Issued != 1 || len(out.Skipped) != 1 {
		t.Fatalf("ghost must be skipped, got issued=%d skipped=%+v", out.Issued, out.Skipped)
	}
}

func TestIssueCouponsRespectsRateLimit(t *testing.T) {
	f := newMarketingFixture(t)
	f.addTemplate(domain.CouponTemplate{
		TemplateNo: "TPL-1", Name: "满20减5", CouponType: domain.CouponTypeCash,
		FaceValueFen: 500, ThresholdFen: 2000, Bearer: domain.CouponBearerPlatform,
		ValidStart: f.now.Add(-time.Hour), ValidEnd: f.now.Add(24 * time.Hour),
		Stock: 1000, PerLimit: 100, Stackable: true,
	})
	f.addBatch(domain.CouponBatch{
		BatchNo: "B-1", TemplateNo: "TPL-1", TotalCount: 1000,
		Channel: domain.CouponChannelManual, StartAt: f.now.Add(-time.Hour), EndAt: f.now.Add(time.Hour),
	})
	for _, id := range []string{"p1", "p2", "p3"} {
		f.addPassenger(id)
	}
	// 每分钟只允许发 2 张。
	f.store.config[ConfigCouponIssueRatePerMinute] = 2

	if _, err := f.uc.IssueCoupons(context.Background(), "B-1",
		IssueCouponsInput{PassengerIDs: []string{"p1", "p2", "p3"}, OperatorID: 1}); err == nil {
		t.Fatal("rate limit must reject the oversized request")
	}
	if len(f.store.coupons) != 0 {
		t.Fatalf("nothing must be issued when throttled, got %d", len(f.store.coupons))
	}
}

func TestCreateCouponBatchRejectsTotalAboveStock(t *testing.T) {
	f := newMarketingFixture(t)
	f.addTemplate(domain.CouponTemplate{
		TemplateNo: "TPL-1", Name: "满20减5", CouponType: domain.CouponTypeCash,
		FaceValueFen: 500, ThresholdFen: 2000, Bearer: domain.CouponBearerPlatform,
		ValidStart: f.now.Add(-time.Hour), ValidEnd: f.now.Add(24 * time.Hour),
		Stock: 10, PerLimit: 1, Stackable: true,
	})
	_, err := f.uc.CreateCouponBatch(context.Background(), CreateCouponBatchInput{
		BatchNo:    "B-BIG",
		TemplateNo: "TPL-1",
		TotalCount: 999,
		StartAt:    f.now.Format(time.RFC3339),
		EndAt:      f.now.Add(24 * time.Hour).Format(time.RFC3339),
		OperatorID: 1,
	})
	if err == nil {
		t.Fatal("batch total above template stock must be rejected")
	}
}

func TestCampaignSettlementAggregatesByBearer(t *testing.T) {
	f := newMarketingFixture(t)
	f.store.campaigns = append(f.store.campaigns, domain.Campaign{
		Code: "CAMP-1", Name: "夏日冲单", Status: domain.CampaignStateEnded,
		StartAt: f.now.Add(-time.Hour), EndAt: f.now.Add(time.Hour), BudgetFen: 100000,
	})
	f.addTemplate(domain.CouponTemplate{
		TemplateNo: "TPL-1", Name: "平台券", CouponType: domain.CouponTypeCash,
		FaceValueFen: 500, Bearer: domain.CouponBearerPlatform,
		ValidStart: f.now.Add(-time.Hour), ValidEnd: f.now.Add(24 * time.Hour), Stock: 100, PerLimit: 5,
	})
	f.store.batches["B-1"] = domain.CouponBatch{
		BatchNo: "B-1", TemplateNo: "TPL-1", TotalCount: 100, CampaignCode: "CAMP-1",
		Status: domain.CouponBatchStateFinished,
	}
	f.store.coupons = []domain.IssuedCoupon{
		{CouponNo: "c1", BatchNo: "B-1", TemplateNo: "TPL-1", State: domain.CouponStateUsed, FaceValueFen: 500},
		{CouponNo: "c2", BatchNo: "B-1", TemplateNo: "TPL-1", State: domain.CouponStateUsed, FaceValueFen: 500},
		{CouponNo: "c3", BatchNo: "B-1", TemplateNo: "TPL-1", State: domain.CouponStateAvailable, FaceValueFen: 500},
		{CouponNo: "c4", BatchNo: "B-1", TemplateNo: "TPL-1", State: domain.CouponStateExpired, FaceValueFen: 500},
	}

	settlement, err := f.uc.CampaignSettlement(context.Background(), "CAMP-1")
	if err != nil {
		t.Fatalf("settlement: %v", err)
	}
	if settlement.IssuedCount != 4 {
		t.Fatalf("issued must be 4, got %d", settlement.IssuedCount)
	}
	if settlement.UsedCount != 2 {
		t.Fatalf("used must be 2, got %d", settlement.UsedCount)
	}
	// 核销金额 = 已核销张数 × 面额。
	if settlement.UsedAmountFen != 1000 {
		t.Fatalf("used amount must be 1000 fen, got %d", settlement.UsedAmountFen)
	}
	if settlement.IssueAmountFen != 2000 {
		t.Fatalf("issue amount must be 2000 fen, got %d", settlement.IssueAmountFen)
	}
	if settlement.BearerSplit[domain.CouponBearerPlatform] != 1000 {
		t.Fatalf("platform must bear 1000 fen, got %d", settlement.BearerSplit[domain.CouponBearerPlatform])
	}
	if settlement.UnusedCount != 1 {
		t.Fatalf("unused must be 1, got %d", settlement.UnusedCount)
	}
	if !settlement.Closed {
		t.Fatal("ended campaign must be marked closed")
	}
	if settlement.BudgetUsagePct != 1 {
		t.Fatalf("budget usage must be 1%%, got %f", settlement.BudgetUsagePct)
	}
	csv := SettlementCSV(settlement)
	if csv == "" || !strings.Contains(csv, "CAMP-1") {
		t.Fatal("csv export must contain the campaign")
	}
}

func TestSettlementCSVForEmptyCampaignStillExports(t *testing.T) {
	// 没有批次也要给一行，避免财务拿到空文件以为接口坏了。
	settlement := domain.CampaignSettlement{CampaignCode: "CAMP-X", CampaignName: "空活动"}
	rows := ExportSettlement(settlement)
	if len(rows) != 1 {
		t.Fatalf("expected one placeholder row, got %d", len(rows))
	}
}
