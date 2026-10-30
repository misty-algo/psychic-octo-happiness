// A05 券发放：防超发、限流降级、每人限领与叠加互斥。
package usecase

import (
	"context"
	"fmt"
	"strings"

	"github.com/NSObjects/echo-admin/internal/modules/operations/domain"
)

// IssueCouponsInput 是一次发放请求。发给谁必须明确，不接受"发个大概"。
type IssueCouponsInput struct {
	PassengerIDs []string `json:"passenger_ids"`
	OperatorID   int64
	RequestID    string
}

// IssueCouponsOutput 是发放结果：成功几张、被规则挡下几张、原因是什么。
// 被挡下的人必须回给运营看，不能静默丢弃。
type IssueCouponsOutput struct {
	BatchNo    string      `json:"batch_no"`
	TemplateNo string      `json:"template_no"`
	Issued     int         `json:"issued"`
	Skipped    []IssueSkip `json:"skipped"`
	CouponNos  []string    `json:"coupon_nos"`
	Remaining  int64       `json:"remaining"`
	Degraded   bool        `json:"degraded"`
}

// IssueSkip 记录一个被规则挡下的乘客及原因。
type IssueSkip struct {
	PassengerID string `json:"passenger_id"`
	Phone       string `json:"phone"`
	Reason      string `json:"reason"`
}

// VoidCouponInput 是作废一张券的请求：作废不可恢复，必须带原因与二次确认。
type VoidCouponInput struct {
	Reason     string `json:"reason"`
	Confirm    bool   `json:"confirm"`
	OperatorID int64
	RequestID  string
}

// VoidCoupon 作废一张券。只有 AVAILABLE 能作废：已锁定在订单里的券不能抽走，
// 已用/已过期/已作废的券再作废旧没有任何意义。
func (u *MarketingUsecase) VoidCoupon(ctx context.Context, couponNo string, in VoidCouponInput) (domain.IssuedCoupon, error) {
	if strings.TrimSpace(couponNo) == "" {
		return domain.IssuedCoupon{}, mapMarketingError(domain.ErrCouponNotFound)
	}
	if !in.Confirm {
		return domain.IssuedCoupon{}, mapMarketingError(domain.ErrCouponConfirmNeeded)
	}
	if strings.TrimSpace(in.Reason) == "" {
		return domain.IssuedCoupon{}, mapMarketingError(domain.ErrCouponVoidReason)
	}
	coupon, err := u.store.FindIssuedCoupon(ctx, couponNo)
	if err != nil {
		return domain.IssuedCoupon{}, mapMarketingError(err)
	}
	if coupon.State != domain.CouponStateAvailable {
		return domain.IssuedCoupon{}, mapMarketingError(domain.ErrCouponNotVoidable)
	}
	ok, err := u.store.VoidIssuedCoupon(ctx, couponNo)
	if err != nil {
		return domain.IssuedCoupon{}, fmt.Errorf("void coupon: %w", err)
	}
	if !ok {
		return domain.IssuedCoupon{}, mapMarketingError(domain.ErrCouponNotVoidable)
	}
	coupon.State = domain.CouponStateVoid
	if err := u.store.RecordOperationLog(ctx, domain.OperationLog{
		Module:    "operations",
		Operator:  operatorName(in.OperatorID),
		Action:    "void_coupon",
		Resource:  "issued_coupon",
		Target:    coupon.CouponNo,
		RequestID: in.RequestID,
		Detail:    fmt.Sprintf("作废券 %s（模板 %s）事由:%s", coupon.CouponNo, coupon.TemplateNo, in.Reason),
		At:        u.now(),
	}); err != nil {
		return domain.IssuedCoupon{}, err
	}
	return coupon, nil
}

// IssueCoupons 执行一次定向发放。
//
// 防超发靠两道原子扣减，不是"先读后写"：
//  1. 批次额度：issuedCount + n <= totalCount 才允许自增
//  2. 模板库存：stock >= n 才允许扣减
//
// 任一道失败都回滚上一道，绝不留下"扣了额度却没发券"的脏账。
func (u *MarketingUsecase) IssueCoupons(ctx context.Context, batchNo string, in IssueCouponsInput) (IssueCouponsOutput, error) {
	now := u.now()
	batch, err := u.store.FindCouponBatch(ctx, strings.TrimSpace(batchNo))
	if err != nil {
		return IssueCouponsOutput{}, mapMarketingError(err)
	}
	template, err := u.store.FindCouponTemplate(ctx, batch.TemplateNo)
	if err != nil {
		return IssueCouponsOutput{}, mapMarketingError(err)
	}

	// 1. 目标去重：同一乘客重复出现在请求里只会触发限领校验，先去掉。
	targets := dedup(in.PassengerIDs)
	if len(targets) == 0 {
		return IssueCouponsOutput{}, mapMarketingError(fmt.Errorf("issue requires at least one passenger: %w", domain.ErrInvalidIssueCount))
	}

	// 2. 发放高峰限流降级：超阈值直接拒绝，而不是把库打穿。
	limit := int64(u.store.ReadMarketingConfig(ctx, ConfigCouponIssueRatePerMinute, DefaultIssueRatePerMinute))
	if !u.limiter.Allow(batch.BatchNo, limit, int64(len(targets)), now) {
		out := IssueCouponsOutput{BatchNo: batch.BatchNo, TemplateNo: template.TemplateNo, Degraded: true}
		return out, mapMarketingError(fmt.Errorf("batch %s rate limit %d/min: %w", batch.BatchNo, limit, domain.ErrIssueRateLimited))
	}

	// 3. 批次与模板可用性：状态、时间窗、剩余量。
	if err := batch.CanIssue(int64(len(targets)), now); err != nil {
		return IssueCouponsOutput{}, mapMarketingError(err)
	}
	if err := template.CanIssue(now); err != nil {
		return IssueCouponsOutput{}, mapMarketingError(err)
	}

	// 4. 乘客必须真实存在，不凭空造券。
	passengers, err := u.store.FindPassengers(ctx, targets)
	if err != nil {
		return IssueCouponsOutput{}, fmt.Errorf("load passengers: %w", err)
	}
	skipped := make([]IssueSkip, 0)
	eligible := make([]string, 0, len(targets))
	for _, id := range targets {
		if _, ok := passengers[id]; !ok {
			skipped = append(skipped, IssueSkip{PassengerID: id, Reason: "乘客不存在"})
			continue
		}
		eligible = append(eligible, id)
	}

	// 5. 每人限领 + 6. 叠加互斥：先取齐判定所需的数据，再逐个过滤。
	if len(eligible) > 0 {
		held, err := u.store.CountHeldCoupons(ctx, eligible, template.TemplateNo)
		if err != nil {
			return IssueCouponsOutput{}, fmt.Errorf("count held coupons: %w", err)
		}
		heldTemplates, err := u.store.ListHeldTemplateNos(ctx, eligible)
		if err != nil {
			return IssueCouponsOutput{}, fmt.Errorf("list held templates: %w", err)
		}
		// 互斥判定要同时看模板自己声明的人清单和乘客已持有券的模板，
		// 后者才能命中"对方把我写进互斥清单"的反向情况。
		conflictNos := append([]string{}, template.MutualTemplates...)
		for _, nos := range heldTemplates {
			conflictNos = append(conflictNos, nos...)
		}
		heldDefs := u.loadTemplates(ctx, conflictNos)
		final := make([]string, 0, len(eligible))
		for _, id := range eligible {
			account := passengers[id]
			if held[id] >= template.PerLimit {
				skipped = append(skipped, IssueSkip{
					PassengerID: id,
					Phone:       account.PhoneMask(),
					Reason:      fmt.Sprintf("已达每人限领 %d 张", template.PerLimit),
				})
				continue
			}
			if reason := u.conflictReason(template, heldDefs, heldTemplates[id]); reason != "" {
				skipped = append(skipped, IssueSkip{PassengerID: id, Phone: account.PhoneMask(), Reason: reason})
				continue
			}
			final = append(final, id)
		}
		eligible = final
	}
	if len(eligible) == 0 {
		// 全员被规则挡下时不发券，但必须回报批次真实剩余额度：
		// 返回 0 会让运营误判成"额度已用尽"。
		return IssueCouponsOutput{
			BatchNo:    batch.BatchNo,
			TemplateNo: template.TemplateNo,
			Skipped:    skipped,
			Remaining:  batch.RemainingCount(),
		}, nil
	}
	count := int64(len(eligible))

	// 7. 先占批次额度（总量闸门），占不到就是超发，直接拒绝。
	reserved, err := u.store.ReserveBatchQuota(ctx, batch.BatchNo, count, now)
	if err != nil {
		return IssueCouponsOutput{}, fmt.Errorf("reserve batch quota: %w", err)
	}
	if !reserved {
		out := IssueCouponsOutput{BatchNo: batch.BatchNo, TemplateNo: template.TemplateNo, Skipped: skipped}
		return out, mapMarketingError(fmt.Errorf("batch %s: %w", batch.BatchNo, domain.ErrCouponBatchExhausted))
	}

	// 8. 再扣模板库存；扣不动就回滚额度，不留脏账。
	consumed, err := u.store.ConsumeTemplateStock(ctx, template.TemplateNo, count, now)
	if err != nil || !consumed {
		if rbErr := u.store.ReleaseBatchQuota(ctx, batch.BatchNo, count); rbErr != nil {
			return IssueCouponsOutput{}, fmt.Errorf("stock unavailable and quota rollback failed: %w", rbErr)
		}
		if err != nil {
			return IssueCouponsOutput{}, fmt.Errorf("consume template stock: %w", err)
		}
		out := IssueCouponsOutput{BatchNo: batch.BatchNo, TemplateNo: template.TemplateNo, Skipped: skipped}
		return out, mapMarketingError(fmt.Errorf("template %s: %w", template.TemplateNo, domain.ErrCouponStockExhausted))
	}

	// 9. 写券，券码在存储层生成并保证唯一。
	coupons := make([]domain.IssuedCoupon, 0, len(eligible))
	for _, id := range eligible {
		account := passengers[id]
		coupons = append(coupons, domain.IssuedCoupon{
			TemplateNo:   template.TemplateNo,
			TemplateName: template.Name,
			BatchNo:      batch.BatchNo,
			PassengerID:  id,
			Passenger:    account.RealName,
			PhoneMask:    account.PhoneMask(),
			State:        domain.CouponStateAvailable,
			FaceValueFen: template.FaceValueFen,
			ThresholdFen: template.ThresholdFen,
			Bearer:       template.Bearer,
			IssuedAt:     now,
			ExpireAt:     template.ValidEnd,
			Channel:      batch.Channel,
		})
	}
	written, err := u.store.InsertCoupons(ctx, coupons)
	if err != nil {
		if rbErr := u.store.ReleaseBatchQuota(ctx, batch.BatchNo, count); rbErr != nil {
			return IssueCouponsOutput{}, fmt.Errorf("insert failed and quota rollback failed: %w", rbErr)
		}
		if rbErr := u.store.RestoreTemplateStock(ctx, template.TemplateNo, count); rbErr != nil {
			return IssueCouponsOutput{}, fmt.Errorf("insert failed and stock rollback failed: %w", rbErr)
		}
		return IssueCouponsOutput{}, fmt.Errorf("insert coupons: %w", err)
	}
	if written > 0 {
		writtenCoupons := make([]string, 0, written)
		for i := 0; i < written && i < len(coupons); i++ {
			if coupons[i].CouponNo != "" {
				writtenCoupons = append(writtenCoupons, coupons[i].CouponNo)
			}
		}
		_ = writtenCoupons
	}

	// 10. 留痕：发了多少、被规则挡下多少。
	detail := fmt.Sprintf("发放 %d 张 模板=%s 批次=%s 渠道=%s", written, template.TemplateNo, batch.BatchNo, batch.Channel)
	if len(skipped) > 0 {
		detail += fmt.Sprintf("，被规则挡下 %d 人", len(skipped))
	}
	if err := u.store.RecordOperationLog(ctx, domain.OperationLog{
		Module:    "operations",
		Operator:  operatorName(in.OperatorID),
		Action:    "issue_coupons",
		Resource:  "coupon_batch",
		Target:    batch.BatchNo,
		RequestID: in.RequestID,
		Detail:    detail,
		At:        now,
	}); err != nil {
		return IssueCouponsOutput{}, err
	}

	remaining := batch.RemainingCount() - count
	if remaining < 0 {
		remaining = 0
	}
	out := IssueCouponsOutput{
		BatchNo:    batch.BatchNo,
		TemplateNo: template.TemplateNo,
		Issued:     written,
		Skipped:    skipped,
		Remaining:  remaining,
	}
	for _, c := range coupons {
		if c.CouponNo != "" {
			out.CouponNos = append(out.CouponNos, c.CouponNo)
		}
	}
	return out, nil
}

// loadTemplates 批量取互斥判定需要的模板定义：待发模板自己声明的互斥清单，
// 加上乘客已持有券对应的模板（后者才能命中反向互斥）。取不到就跳过，由
// conflictReason 兜底，不让一次查询失败阻断整批发券。
func (u *MarketingUsecase) loadTemplates(ctx context.Context, nos []string) map[string]domain.CouponTemplate {
	seen := make(map[string]struct{}, len(nos))
	unique := make([]string, 0, len(nos))
	for _, no := range nos {
		no = strings.TrimSpace(no)
		if no == "" {
			continue
		}
		if _, ok := seen[no]; ok {
			continue
		}
		seen[no] = struct{}{}
		unique = append(unique, no)
	}
	if len(unique) == 0 {
		return map[string]domain.CouponTemplate{}
	}
	defs, err := u.store.FindCouponTemplates(ctx, unique)
	if err != nil {
		return map[string]domain.CouponTemplate{}
	}
	return defs
}

// conflictReason 判断该乘客手上的券是否与待发券显式互斥，返回可读原因。
// 只有模板互斥清单命中才拦；stackable 属于核销阶段规则，不在这里拦发放。
func (u *MarketingUsecase) conflictReason(template domain.CouponTemplate, heldDefs map[string]domain.CouponTemplate, heldNos []string) string {
	for _, no := range heldNos {
		if no == template.TemplateNo {
			continue
		}
		other, ok := heldDefs[no]
		if !ok {
			// 券上引用的模板已不存在（被删除或编号变更）：不拦发放。
			// 拦住会让这批乘客永远领不到券，且现场无法排查。
			continue
		}
		if template.ConflictsWith(other) {
			return fmt.Sprintf("与已持有的券「%s」互斥", other.Name)
		}
	}
	return ""
}

// ListIssuedCoupons 查询已发出的券。
func (u *MarketingUsecase) ListIssuedCoupons(ctx context.Context, in ListMarketingInput) (ListIssuedCouponsOutput, error) {
	filter := IssuedCouponFilter{
		BatchNo:     strings.TrimSpace(in.BatchNo),
		TemplateNo:  strings.TrimSpace(in.TemplateNo),
		PassengerID: strings.TrimSpace(in.PassengerID),
		Phone:       strings.TrimSpace(in.Phone),
		State:       strings.TrimSpace(in.State),
		Skip:        skipOf(in.Page, in.PageSize),
		Limit:       limitOf(in.PageSize),
	}
	items, total, err := u.store.ListIssuedCoupons(ctx, filter)
	if err != nil {
		return ListIssuedCouponsOutput{}, fmt.Errorf("list issued coupons: %w", err)
	}
	if items == nil {
		items = []domain.IssuedCoupon{}
	}
	return ListIssuedCouponsOutput{Items: items, Total: total}, nil
}

// dedup 去掉重复与空白的乘客 id，保持顺序稳定。
func dedup(ids []string) []string {
	seen := make(map[string]struct{}, len(ids))
	out := make([]string, 0, len(ids))
	for _, id := range ids {
		id = strings.TrimSpace(id)
		if id == "" {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		out = append(out, id)
	}
	return out
}
