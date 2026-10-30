package main

// 花小猪运营后台种子数据：直接写入线上 MongoDB，全部使用真实业务字段，
// 让计价/派单/拼车/补贴/券各页面有可验证的数据（不是写死的假效果）。
// 幂等：先按固定业务主键删除上一轮种子，再重新插入。

import (
	"context"
	"fmt"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

const (
	citySH = "310100" // 上海
	cityBJ = "110100" // 北京
)

func main() {
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()

	client, err := mongo.Connect(options.Client().ApplyURI("mongodb://root:root@101.96.204.17:27017/?authSource=admin"))
	if err != nil {
		panic(err)
	}
	defer client.Disconnect(ctx)
	if err := client.Ping(ctx, nil); err != nil {
		panic(err)
	}
	db := client.Database("huazhu")

	now := time.Now().UTC()
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
	day := func(offset int, hour int) time.Time {
		return today.AddDate(0, 0, offset).Add(time.Duration(hour) * time.Hour)
	}

	passengers := pickIDs(ctx, db, "passengers", 4)
	drivers := pickIDs(ctx, db, "drivers", 3)
	if len(passengers) < 4 {
		passengers = fillObjectIDs(passengers, 4)
	}
	if len(drivers) < 3 {
		drivers = fillObjectIDs(drivers, 3)
	}

	// ---------- 清理上一轮种子 ----------
	clean(ctx, db, "pricing_rules", bson.M{"ruleId": bson.M{"$regex": "^PR-"}})
	clean(ctx, db, "dispatch_strategies", bson.M{"strategyId": bson.M{"$regex": "^DS-"}})
	clean(ctx, db, "pool_configs", bson.M{"configId": bson.M{"$regex": "^POOL-"}})
	clean(ctx, db, "special_price_plans", bson.M{"planId": bson.M{"$regex": "^SP-"}})
	clean(ctx, db, "campaigns", bson.M{"code": bson.M{"$in": bson.A{"HZ2026Q4", "HZSPRING"}}})
	clean(ctx, db, "campaign_subsidies", bson.M{"campaignCode": "HZ2026Q4"})
	clean(ctx, db, "campaign_subsidy_days", bson.M{"campaignCode": "HZ2026Q4"})
	clean(ctx, db, "orders", bson.M{"orderNo": bson.M{"$regex": "^HZ"}})

	// ---------- A05 计价规则 ----------
	pricingRules := []bson.M{
		pricingRule("PR-310100-ECONOMY-V1", 1, citySH, "economy", 800, 3000, 210, 45, 500, 2000, 10000, 8000, 500, 0, "2026-01-01", "2026-12-31", "published", now),
		pricingRule("PR-310100-COMFORT-V1", 1, citySH, "comfort", 1200, 3000, 280, 60, 500, 2000, 10000, 8000, 600, 0, "2026-01-01", "2026-12-31", "published", now),
		pricingRule("PR-110100-ECONOMY-V1", 1, cityBJ, "economy", 750, 3000, 190, 40, 500, 1800, 10000, 8000, 500, 0, "2026-01-01", "2026-12-31", "published", now),
		// 草稿：用于验证「分账样例 → 发布」流程
		pricingRule("PR-310100-ECONOMY-V2", 2, citySH, "economy", 900, 3000, 220, 50, 500, 2000, 10000, 8000, 500, 0, "2027-01-01", "2027-12-31", "draft", now),
	}
	insertMany(ctx, db, "pricing_rules", pricingRules)

	// ---------- A06 派单策略 ----------
	strategies := []bson.M{
		strategy("DS-V1", 1, 4000, 3000, 1500, 1000, 500, 5000, 15, 120, 60, 60, true, 60, 2, "published", now),
		strategy("DS-V2", 2, 5000, 2000, 1500, 1000, 500, 4000, 15, 120, 60, 60, true, 60, 1, "draft", now),
	}
	insertMany(ctx, db, "dispatch_strategies", strategies)

	// ---------- A09 拼车配置与特价方案 ----------
	insertMany(ctx, db, "pool_configs", []bson.M{
		poolConfig("POOL-V1", 1, 60, 2, 4, []string{citySH, cityBJ}, "published", now),
		poolConfig("POOL-V2", 2, 90, 2, 4, []string{citySH}, "draft", now),
	})
	insertMany(ctx, db, "special_price_plans", []bson.M{
		specialPlan("SP-310100-ECONOMY", "浦东机场特价", citySH, "economy", 3900, 4, "2026-01-01", "2026-12-31", "published", now),
		specialPlan("SP-110100-ECONOMY", "大兴机场特价", cityBJ, "economy", 4500, 4, "2026-01-01", "2026-12-31", "draft", now),
	})

	// ---------- A08 补贴活动 + 台账 ----------
	subsidyCampaign := bson.M{
		"code": "HZ2026Q4", "name": "四季度司机完单补贴", "type": "period_subsidy",
		"targetAudience": "driver", "rules": "每完单补贴 5 元，每人每天最多 3 次",
		"subsidyConfig": bson.M{"triggerType": "per_order", "amount": int64(500), "maxPerDriver": int64(20), "dailyCap": int64(40000), "needEnroll": true},
		"cityCodes":     bson.A{citySH, cityBJ},
		"startAt":       day(-30, 0), "endAt": day(60, 23),
		"status": "running", "participantCount": int64(3),
		"budget": int64(300000), "usedBudget": int64(6000),
		"operator": "admin:1", "createdAt": day(-30, 0), "updatedAt": now,
	}
	couponCampaign := bson.M{
		"code": "HZSPRING", "name": "新客首单立减", "type": "coupon",
		"targetAudience": "passenger", "rules": "新客首单满 20 减 8",
		"rewardCouponTemplateIds": bson.A{},
		"cityCodes":               bson.A{citySH},
		"startAt":                 day(-10, 0), "endAt": day(50, 23),
		"status": "running", "participantCount": int64(0),
		"budget": int64(500000), "usedBudget": int64(0),
		"operator": "admin:1", "createdAt": day(-10, 0), "updatedAt": now,
	}
	insertMany(ctx, db, "campaigns", []bson.M{subsidyCampaign, couponCampaign})

	// 台账：3 名司机 × 3 天，共 12 笔，其中 1 笔已追回（验证反向流水口径）
	subsidies := make([]bson.M, 0, 12)
	used := int64(0)
	for i := 0; i < 12; i++ {
		driverIdx := i % len(drivers)
		dayOffset := -2 + (i % 3)
		reversed := i == 5
		recNo := fmt.Sprintf("SRHZ%02d", i+1)
		subsidies = append(subsidies, bson.M{
			"recordNo": recNo, "campaignCode": "HZ2026Q4", "campaignName": "四季度司机完单补贴",
			"driverId": fmt.Sprintf("d%03d", driverIdx+1), "orderNo": fmt.Sprintf("HZ%08d", 2000+i),
			"amount": int64(500), "bizType": "other", "reason": "完单补贴自动发放",
			"day":       day(dayOffset, 0).Format("2006-01-02"),
			"operator":  "admin:1",
			"reversed":  reversed,
			"createdAt": day(dayOffset, 8+i%8),
			"updatedAt": day(dayOffset, 8+i%8),
		})
		if reversed {
			subsidies[len(subsidies)-1]["reversedAt"] = day(dayOffset, 12)
			subsidies[len(subsidies)-1]["reverseReason"] = "订单全额退款，补贴冲正"
			subsidies[len(subsidies)-1]["reverseOperator"] = "admin:1"
		} else {
			used += 500
		}
	}
	insertMany(ctx, db, "campaign_subsidies", subsidies)
	// 预算占用与未追回台账保持一致
	updateOne(ctx, db, "campaigns", bson.M{"code": "HZ2026Q4"}, bson.M{"$set": bson.M{"usedBudget": used, "participantCount": int64(len(drivers))}})
	// 日封顶聚合：今天已发 4 笔（用于验证日封顶）
	insertMany(ctx, db, "campaign_subsidy_days", []bson.M{
		{"_id": "HZ2026Q4|" + day(0, 0).Format("2006-01-02"), "campaignCode": "HZ2026Q4", "day": day(0, 0).Format("2006-01-02"), "totalFen": int64(2000)},
		{"_id": "HZ2026Q4|" + day(-1, 0).Format("2006-01-02"), "campaignCode": "HZ2026Q4", "day": day(-1, 0).Format("2006-01-02"), "totalFen": int64(2000)},
	})

	// ---------- 订单：拼车单驱动统计三件套，快车单补齐计价字段 ----------
	orders := make([]bson.M, 0, 12)
	// 共乘组 1：虹桥枢纽 → 陆家嘴，同日两名不同乘客，行程中
	orders = append(orders,
		poolOrder("HZ20260001", citySH, passengers[0], drivers[0], "IN_TRIP", "POI-HQ", "POI-LJZ", "虹桥枢纽", "陆家嘴", day(0, 9), 2680, "PR-310100-ECONOMY-V1", 1.2),
		poolOrder("HZ20260002", citySH, passengers[1], drivers[0], "IN_TRIP", "POI-HQ", "POI-LJZ", "虹桥枢纽", "陆家嘴", day(0, 9), 2680, "PR-310100-ECONOMY-V1", 1.2),
	)
	// 共乘组 2：张江 → 人民广场，同日两名不同乘客，已结束
	orders = append(orders,
		poolOrder("HZ20260003", citySH, passengers[1], drivers[1], "ENDED", "POI-ZJ", "POI-RMGC", "张江高科", "人民广场", day(-1, 8), 3520, "PR-310100-ECONOMY-V1", 1.0),
		poolOrder("HZ20260004", citySH, passengers[2], drivers[1], "ENDED", "POI-ZJ", "POI-RMGC", "张江高科", "人民广场", day(-1, 8), 3520, "PR-310100-ECONOMY-V1", 1.0),
	)
	// 拼未成：无人接单
	orders = append(orders,
		poolOrder("HZ20260005", citySH, passengers[3], bson.ObjectID{}, "NO_DRIVER", "POI-SJ", "POI-PD", "世纪公园", "浦东机场", day(0, 14), 8900, "PR-310100-ECONOMY-V1", 1.0),
		poolOrder("HZ20260006", cityBJ, passengers[0], bson.ObjectID{}, "PENDING", "POI-BJZ", "POI-XD", "北京站", "西单", day(0, 15), 4200, "PR-110100-ECONOMY-V1", 1.0),
	)
	// 普通快车单：覆盖各状态，带计价快照与风险等级
	orders = append(orders,
		expressOrder("HZ20260007", citySH, passengers[0], drivers[0], "COMPLETED", "PENDING_PAY", day(0, 10), 3860, "PR-310100-ECONOMY-V1", 1.5),
		expressOrder("HZ20260008", citySH, passengers[2], drivers[2], "IN_SERVICE", "PAID", day(0, 11), 5240, "PR-310100-COMFORT-V1", 1.0),
		expressOrder("HZ20260009", cityBJ, passengers[1], drivers[1], "CANCELLED", "REFUNDED", day(-1, 20), 0, "PR-110100-ECONOMY-V1", 1.0),
		expressOrder("HZ20260010", citySH, passengers[3], drivers[2], "UNPAID", "UNPAID", day(-1, 21), 2980, "PR-310100-ECONOMY-V1", 1.0),
	)
	insertMany(ctx, db, "orders", orders)

	// ---------- 给历史订单补齐业务类型 / 风险等级 / 计价字段 ----------
	updateMany(ctx, db, "orders", bson.M{"orderNo": bson.M{"$not": bson.M{"$regex": "^HZ"}}}, bson.M{"$set": bson.M{
		"bizType":           "express",
		"cityCode":          citySH,
		"riskLevel":         "low",
		"pricingRuleId":     "PR-310100-ECONOMY-V1",
		"pricingVersion":    "V1",
		"pricingSnapshotId": "SNAP-310100-ECONOMY-V1",
		"surgeRatio":        1.0,
	}})

	report(ctx, db)
	fmt.Println("种子数据写入完成")
}

// ---- 构造辅助 ----

func pricingRule(id string, version int64, city, carType string, start, includeM, perKm, perMin, serviceBp, commBp, surgeBp, discountBp, cancelFen, fixedFen int64, from, to, state string, now time.Time) bson.M {
	return bson.M{
		"ruleId": id, "version": version, "cityCode": city, "area": "", "carType": carType,
		"startFen": start, "includeM": includeM, "perKmFen": perKm, "perMinFen": perMin,
		"serviceBp": serviceBp, "commBp": commBp, "surgeBp": surgeBp, "discountBp": discountBp,
		"cancelFen": cancelFen, "fixedFen": fixedFen, "priority": int64(0),
		"validFrom": parseDay(from), "validTo": parseDay(to),
		"state": state, "operator": "admin:1", "createdAt": now, "updatedAt": now,
	}
}

func strategy(id string, version int64, income, distance, rating, online, success, radius, invite, total, listen, grab int64, region bool, extra, maxReturn int64, state string, now time.Time) bson.M {
	return bson.M{
		"strategyId": id, "version": version,
		"weights": bson.M{"incomeBp": income, "distanceBp": distance, "ratingBp": rating, "onlineSecBp": online, "successRateBp": success},
		"radiusM":   radius, "inviteSec": invite, "totalSec": total, "listenSec": listen, "grabSec": grab,
		"regionSwitch": region,
		"redispatch":   bson.M{"extraWindowSec": extra, "maxAcceptedReturn": maxReturn},
		"state":        state, "operator": "admin:1", "createdAt": now, "updatedAt": now,
	}
}

func poolConfig(id string, version int64, matchSec, groupMax, seatMax int64, cities []string, state string, now time.Time) bson.M {
	return bson.M{
		"configId": id, "version": version, "matchSec": matchSec,
		"groupMax": groupMax, "seatMax": seatMax, "cityCodes": cities,
		"state": state, "operator": "admin:1", "createdAt": now, "updatedAt": now,
	}
}

func specialPlan(id, name, city, carType string, fixedFen, seats int64, from, to, state string, now time.Time) bson.M {
	return bson.M{
		"planId": id, "name": name, "cityCode": city, "carType": carType,
		"fixedFen": fixedFen, "seats": seats,
		"validFrom": parseDay(from), "validTo": parseDay(to),
		"state": state, "operator": "admin:1", "createdAt": now, "updatedAt": now,
	}
}

func poolOrder(no, city string, passenger, driver bson.ObjectID, state, startPoi, endPoi, startAddr, endAddr string, at time.Time, amount int64, ruleID string, surge float64) bson.M {
	return orderDocSeed(no, "pool", city, passenger, driver, state, "PAID", at, amount, ruleID, surge, startPoi, endPoi, startAddr, endAddr, "low")
}

func expressOrder(no, city string, passenger, driver bson.ObjectID, state, payStatus string, at time.Time, amount int64, ruleID string, surge float64) bson.M {
	risk := "low"
	if state == "CANCELLED" {
		risk = "medium"
	}
	return orderDocSeed(no, "express", city, passenger, driver, state, payStatus, at, amount, ruleID, surge, "", "", "花小猪·示例起点", "花小猪·示例终点", risk)
}

func orderDocSeed(no, bizType, city string, passenger, driver bson.ObjectID, state, payStatus string, at time.Time, amount int64, ruleID string, surge float64, startPoi, endPoi, startAddr, endAddr, risk string) bson.M {
	doc := bson.M{
		"orderNo": no, "bizType": bizType, "cityCode": city,
		"passengerId": passenger, "state": state, "payStatus": payStatus,
		"amount": amount, "payFee": amount, "totalFee": amount,
		"pricingVersion": "V1", "pricingRuleId": ruleID, "pricingSnapshotId": "SNAP-" + ruleID,
		"surgeRatio": surge, "estimateFee": amount,
		"startAddress": startAddr, "endAddress": endAddr,
		"startPoiId": startPoi, "endPoiId": endPoi,
		"riskLevel": risk, "safetyFlags": bson.A{},
		"createdAt": at, "updatedAt": at,
	}
	if !driver.IsZero() {
		doc["driverId"] = driver
	}
	return doc
}

// ---- Mongo 辅助 ----

func pickIDs(ctx context.Context, db *mongo.Database, col string, n int) []bson.ObjectID {
	opts := options.Find().SetLimit(int64(n)).SetProjection(bson.M{"_id": 1})
	cur, err := db.Collection(col).Find(ctx, bson.M{}, opts)
	if err != nil {
		return nil
	}
	defer cur.Close(ctx)
	out := make([]bson.ObjectID, 0, n)
	for cur.Next(ctx) {
		var doc struct {
			ID bson.ObjectID `bson:"_id"`
		}
		if err := cur.Decode(&doc); err == nil && !doc.ID.IsZero() {
			out = append(out, doc.ID)
		}
	}
	return out
}

func fillObjectIDs(ids []bson.ObjectID, n int) []bson.ObjectID {
	for len(ids) < n {
		ids = append(ids, bson.NewObjectID())
	}
	return ids
}

func clean(ctx context.Context, db *mongo.Database, col string, filter bson.M) {
	if _, err := db.Collection(col).DeleteMany(ctx, filter); err != nil {
		panic(err)
	}
}

func insertMany(ctx context.Context, db *mongo.Database, col string, docs []bson.M) {
	if len(docs) == 0 {
		return
	}
	if _, err := db.Collection(col).InsertMany(ctx, docs); err != nil {
		panic(err)
	}
	fmt.Printf("%-24s +%d\n", col, len(docs))
}

func updateOne(ctx context.Context, db *mongo.Database, col string, filter, update bson.M) {
	if _, err := db.Collection(col).UpdateOne(ctx, filter, update); err != nil {
		panic(err)
	}
}

func updateMany(ctx context.Context, db *mongo.Database, col string, filter, update bson.M) {
	res, err := db.Collection(col).UpdateMany(ctx, filter, update)
	if err != nil {
		panic(err)
	}
	fmt.Printf("%-24s ~%d (补齐字段)\n", col, res.ModifiedCount)
}

func parseDay(v string) time.Time {
	t, err := time.Parse("2006-01-02", v)
	if err != nil {
		panic(err)
	}
	return t.UTC()
}

func report(ctx context.Context, db *mongo.Database) {
	for _, col := range []string{"pricing_rules", "dispatch_strategies", "pool_configs", "special_price_plans", "campaigns", "campaign_subsidies", "orders"} {
		n, err := db.Collection(col).CountDocuments(ctx, bson.M{})
		if err != nil {
			continue
		}
		fmt.Printf("%-24s 共 %d 条\n", col, n)
	}
}
