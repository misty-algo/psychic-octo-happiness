// 领券中心公开投放：生成一批无主券码写入共享 coupons 集合，
// 乘客端领券中心按 state=AVAILABLE 展示，先到先得领取。
package usecase

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/NSObjects/echo-admin/internal/modules/operations/domain"
)

// PublishPublicCouponsInput 是一次公开投放请求。
// 与定向发放不同：不指定乘客，券进入领券中心由乘客自行领取。
type PublishPublicCouponsInput struct {
	// Name 为券的展示名称（领券中心直接显示），空则用「打车券」。
	Name string `json:"name"`
	// FaceValueFen 面值（分），必须大于 0。
	FaceValueFen int64 `json:"face_value_fen"`
	// ThresholdFen 使用门槛（分），0 表示无门槛。
	ThresholdFen int64 `json:"threshold_fen"`
	// Count 投放张数，每张一个独立券码，领完即止。
	Count int `json:"count"`
	// ExpireAt 券的过期时间，支持 RFC3339 与「2006-01-02 15:04:05」两种格式。
	ExpireAt string `json:"expire_at"`
	OperatorID int64
	RequestID  string
}

// PublishPublicCouponsOutput 回显投放结果与生成的券码。
type PublishPublicCouponsOutput struct {
	Count     int      `json:"count"`
	CouponNos []string `json:"coupon_nos"`
	ExpireAt  string   `json:"expire_at"`
}

// PublishPublicCoupons 投放一批公开券到领券中心。
// 每张券一个独立券码（state=AVAILABLE、无 passengerId），
// 乘客端领取时原子抢占（AVAILABLE→CLAIMED），领完即止。
func (u *MarketingUsecase) PublishPublicCoupons(ctx context.Context, in PublishPublicCouponsInput) (PublishPublicCouponsOutput, error) {
	name := strings.TrimSpace(in.Name)
	if name == "" {
		name = "打车券"
	}
	count := in.Count
	if count <= 0 {
		count = 1
	}
	if count > 500 {
		return PublishPublicCouponsOutput{}, mapMarketingError(fmt.Errorf("publish count %d over limit 500: %w", count, domain.ErrInvalidIssueCount))
	}
	if in.FaceValueFen <= 0 {
		return PublishPublicCouponsOutput{}, mapMarketingError(fmt.Errorf("face value must be positive: %w", domain.ErrInvalidIssueCount))
	}
	if in.ThresholdFen < 0 {
		return PublishPublicCouponsOutput{}, mapMarketingError(fmt.Errorf("threshold must not be negative: %w", domain.ErrInvalidIssueCount))
	}
	if in.ThresholdFen > 0 && in.ThresholdFen <= in.FaceValueFen {
		return PublishPublicCouponsOutput{}, mapMarketingError(fmt.Errorf("threshold %d must exceed face value %d: %w", in.ThresholdFen, in.FaceValueFen, domain.ErrInvalidIssueCount))
	}
	expireAt, err := parseCouponExpireAt(in.ExpireAt)
	if err != nil {
		return PublishPublicCouponsOutput{}, mapMarketingError(fmt.Errorf("invalid expire_at: %w", domain.ErrInvalidIssueCount))
	}
	if expireAt.Before(time.Now()) {
		return PublishPublicCouponsOutput{}, mapMarketingError(fmt.Errorf("expire_at %s is in the past: %w", in.ExpireAt, domain.ErrInvalidIssueCount))
	}

	now := u.now()
	coupons := make([]domain.IssuedCoupon, 0, count)
	for i := 0; i < count; i++ {
		coupons = append(coupons, domain.IssuedCoupon{
			Name:         name,
			State:        domain.CouponStateAvailable,
			FaceValueFen: in.FaceValueFen,
			ThresholdFen: in.ThresholdFen,
			IssuedAt:     now,
			ExpireAt:     expireAt,
			Channel:      "claim_center",
		})
	}
	written, err := u.store.InsertCoupons(ctx, coupons)
	if err != nil {
		return PublishPublicCouponsOutput{}, fmt.Errorf("insert public coupons: %w", err)
	}
	nos := make([]string, 0, written)
	for _, c := range coupons {
		if c.CouponNo != "" {
			nos = append(nos, c.CouponNo)
		}
	}
	if err := u.store.RecordOperationLog(ctx, domain.OperationLog{
		Module:    "operations",
		Operator:  operatorName(in.OperatorID),
		Action:    "publish_public_coupons",
		Resource:  "issued_coupon",
		Target:    strings.Join(nos, ","),
		RequestID: in.RequestID,
		Detail:    fmt.Sprintf("投放领券中心：%s %d张 面值%d分 门槛%d分 到期%s", name, written, in.FaceValueFen, in.ThresholdFen, expireAt.Format(time.RFC3339)),
		At:        now,
	}); err != nil {
		return PublishPublicCouponsOutput{}, err
	}
	return PublishPublicCouponsOutput{
		Count:     written,
		CouponNos: nos,
		ExpireAt:  expireAt.Format(time.RFC3339),
	}, nil
}

// parseCouponExpireAt 兼容前端常见的两种时间格式。
func parseCouponExpireAt(raw string) (time.Time, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return time.Time{}, fmt.Errorf("empty expire_at")
	}
	for _, layout := range []string{time.RFC3339, "2006-01-02 15:04:05", "2006-01-02"} {
		if t, err := time.ParseInLocation(layout, raw, time.Local); err == nil {
			return t.UTC(), nil
		}
	}
	return time.Time{}, fmt.Errorf("unrecognized time: %s", raw)
}
