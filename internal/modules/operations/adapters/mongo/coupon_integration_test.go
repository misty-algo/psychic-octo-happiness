//go:build integration

// 营销中心存储层集成测试：直接打真实 MongoDB，验证索引创建、防超发 CAS、
// 券码唯一与对账统计。默认不参与 go test，跑法：
//
//	MARKETING_INTEGRATION=1 go test -tags=integration ./internal/modules/operations/adapters/mongo -run TestMarketing -v
package mongo

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"

	"github.com/NSObjects/echo-admin/internal/modules/operations/domain"
	"github.com/NSObjects/echo-admin/internal/modules/operations/usecase"
)

func integrationDB(t *testing.T) *Store {
	t.Helper()
	if os.Getenv("MARKETING_INTEGRATION") != "1" {
		t.Skip("set MARKETING_INTEGRATION=1 to run against a real MongoDB")
	}
	uri := os.Getenv("MARKETING_MONGO_URI")
	if uri == "" {
		uri = "mongodb://root:root@101.96.204.17:27017/huazhu?authSource=admin"
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	client, err := mongo.Connect(options.Client().ApplyURI(uri))
	if err != nil {
		t.Fatalf("connect mongo: %v", err)
	}
	if err := client.Ping(ctx, nil); err != nil {
		t.Fatalf("ping mongo: %v", err)
	}
	dbName := os.Getenv("MARKETING_MONGO_DB")
	if dbName == "" {
		dbName = "huazhu"
	}
	store, err := NewStore(ctx, client.Database(dbName))
	if err != nil {
		t.Fatalf("new store: %v", err)
	}
	return store
}

func TestMarketingIndexesAndOverIssueGuard(t *testing.T) {
	store := integrationDB(t)
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	if err := store.EnsureMarketingIndexes(ctx); err != nil {
		t.Fatalf("ensure indexes: %v", err)
	}

	// 幂等：再跑一次建索引也不能报错。
	if err := store.EnsureMarketingIndexes(ctx); err != nil {
		t.Fatalf("ensure indexes must be idempotent: %v", err)
	}

	now := time.Now()
	templateNo := fmt.Sprintf("TPL-IT-%d", now.UnixMilli()%100000)
	batchNo := fmt.Sprintf("B-IT-%d", now.UnixMilli()%100000)

	// 1. 建模板：库存 5、限领 5，批次总量 3，总量注定小于库存。
	if _, err := store.CreateCouponTemplate(ctx, domain.CouponTemplate{
		TemplateNo: templateNo, Name: "集成测试券", CouponType: domain.CouponTypeCash,
		FaceValueFen: 500, ThresholdFen: 2000, Bearer: domain.CouponBearerPlatform,
		ValidStart: now.Add(-time.Hour), ValidEnd: now.Add(24 * time.Hour),
		Stock: 5, PerLimit: 5, Stackable: true, State: domain.CouponTemplateStatePublished,
	}); err != nil {
		t.Fatalf("create template: %v", err)
	}
	if _, err := store.CreateCouponBatch(ctx, domain.CouponBatch{
		BatchNo: batchNo, TemplateNo: templateNo, TotalCount: 3,
		Channel: domain.CouponChannelManual, StartAt: now.Add(-time.Hour), EndAt: now.Add(time.Hour),
		Status: domain.CouponBatchStateRunning, CreatedAt: now, UpdatedAt: now,
	}); err != nil {
		t.Fatalf("create batch: %v", err)
	}

	// 2. 防超发：总量 3，先占 3，再占 1 必须失败且计数保持 3。
	ok, err := store.ReserveBatchQuota(ctx, batchNo, 3, now)
	if err != nil || !ok {
		t.Fatalf("reserve 3 of 3 must succeed: ok=%v err=%v", ok, err)
	}
	ok, _ = store.ReserveBatchQuota(ctx, batchNo, 1, now)
	if ok {
		t.Fatal("reserving beyond the total must fail (over-issue guard broken)")
	}
	batch, _ := store.FindCouponBatch(ctx, batchNo)
	if batch.IssuedCount != 3 {
		t.Fatalf("quota must stay at 3, got %d", batch.IssuedCount)
	}

	// 3. 库存 CAS：库存 5 扣 3 剩 2，扣 3 必须失败。
	ok, err = store.ConsumeTemplateStock(ctx, templateNo, 3, now)
	if err != nil || !ok {
		t.Fatalf("consume 3 of 5 must succeed: ok=%v err=%v", ok, err)
	}
	ok, _ = store.ConsumeTemplateStock(ctx, templateNo, 3, now)
	if ok {
		t.Fatal("consuming beyond remaining stock must fail")
	}

	// 4. 发券：3 张券码必须全部回填且唯一。
	passengerID := "6ad9adcc77e161c8570967b1"
	coupons := make([]domain.IssuedCoupon, 3)
	for i := range coupons {
		coupons[i] = domain.IssuedCoupon{
			TemplateNo: templateNo, BatchNo: batchNo, PassengerID: passengerID,
			State: domain.CouponStateAvailable, FaceValueFen: 500, ThresholdFen: 2000,
			Bearer: domain.CouponBearerPlatform, IssuedAt: now, ExpireAt: now.Add(24 * time.Hour),
			Channel: domain.CouponChannelManual,
		}
	}
	written, err := store.InsertCoupons(ctx, coupons)
	if err != nil {
		t.Fatalf("insert coupons: %v", err)
	}
	if written != 3 {
		t.Fatalf("expected 3 coupons, got %d", written)
	}
	seen := map[string]bool{}
	for _, c := range coupons {
		if c.CouponNo == "" {
			t.Fatal("coupon no must be filled back for the caller")
		}
		if seen[c.CouponNo] {
			t.Fatalf("duplicate coupon no generated: %s", c.CouponNo)
		}
		seen[c.CouponNo] = true
	}

	// 5. 统计：批次发放数必须是 3。
	stats, err := store.CountCouponsByBatch(ctx, []string{batchNo})
	if err != nil {
		t.Fatalf("count by batch: %v", err)
	}
	if stats[batchNo].Issued != 3 {
		t.Fatalf("batch stat issued must be 3, got %+v", stats[batchNo])
	}

	// 6. 每人限领依据：CountHeldCoupons 必须数出 3。
	held, err := store.CountHeldCoupons(ctx, []string{passengerID}, templateNo)
	if err != nil {
		t.Fatalf("count held: %v", err)
	}
	if held[passengerID] != 3 {
		t.Fatalf("held count must be 3, got %d", held[passengerID])
	}

	// 7. 清理集成测试数据，不留脏库。
	_, _ = store.db.Collection(colCoupons).DeleteMany(ctx, bson.M{"batchNo": batchNo})
	_, _ = store.db.Collection(colCouponBatches).DeleteOne(ctx, bson.M{"batchNo": batchNo})
	_, _ = store.db.Collection(colCouponTemplates).DeleteOne(ctx, bson.M{"templateNo": templateNo})
}

func TestMarketingConfigFallback(t *testing.T) {
	store := integrationDB(t)
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	// 不存在的键回落默认值；存在的键（若配了）必须解析为正整数。
	if got := store.ReadMarketingConfig(ctx, "coupon.no.such.key", 42); got != 42 {
		t.Fatalf("fallback expected 42, got %d", got)
	}
	_ = usecase.CouponStat{}
}
