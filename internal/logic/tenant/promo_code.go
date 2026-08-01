package tenant

import (
	"context"
	"fmt"
	"time"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/shopspring/decimal"

	"github.com/qianfree/team-api/internal/dao"
	do "github.com/qianfree/team-api/internal/model/do"

	"github.com/qianfree/team-api/internal/logic/billing"
	lcommon "github.com/qianfree/team-api/internal/logic/common"
	"github.com/qianfree/team-api/internal/middleware"

	v1 "github.com/qianfree/team-api/api/tenant/v1"
)

// promoValidationResult 是优惠码校验的类型化结果。
// 金额字段用 decimal（多步金额运算禁止 float64 中间值），API 出口再转 float64。
type promoValidationResult struct {
	PromoCodeID int64           // 优惠码 ID
	Type        string          // 优惠码类型（percentage / fixed）
	Discount    decimal.Decimal // 折扣金额（CNY）
	FinalAmount decimal.Decimal // 折后金额（CNY）
}

// ValidatePromoCode 校验优惠码并返回折扣金额（只读预检端点，不加锁、不消耗用量）
func (s *sTenant) ValidatePromoCode(ctx context.Context, req *v1.TenantValidatePromoCodeReq) (*v1.TenantValidatePromoCodeRes, error) {
	tenantID := middleware.GetTenantID(ctx)
	result, err := validatePromoCode(ctx, tenantID, req.Code, billing.NewFromFloat(req.Amount), 0, false)
	if err != nil {
		return nil, err
	}
	return &v1.TenantValidatePromoCodeRes{
		PromoCodeId: result.PromoCodeID,
		Type:        result.Type,
		Discount:    billing.InexactFloat64(result.Discount),
		FinalAmount: billing.InexactFloat64(result.FinalAmount),
	}, nil
}

// validatePromoCode 校验优惠码并计算折扣。
//
//   - forUpdate=true 时对优惠码行加 FOR UPDATE 锁（须在事务 ctx 内调用）：下单扣用量前
//     锁定，与 recordPromoUsageTx 的条件递增配合杜绝并发超用；
//   - planID > 0 时校验 plan_ids 限制（优惠码限定可用套餐，NULL/空数组表示不限）；
//   - 金额运算全程 decimal。
func validatePromoCode(ctx context.Context, tenantID int64, code string, amount decimal.Decimal, planID int64, forUpdate bool) (*promoValidationResult, error) {
	var promo *struct {
		ID            int64           `json:"id"`
		Type          string          `json:"type"`
		DiscountValue decimal.Decimal `json:"discount_value"`
		MinAmount     decimal.Decimal `json:"min_amount"`
		MaxDiscount   decimal.Decimal `json:"max_discount"`
		TotalCount    int             `json:"total_count"`
		UsedCount     int             `json:"used_count"`
		PerUserLimit  int             `json:"per_user_limit"`
		ValidFrom     time.Time       `json:"valid_from"`
		ValidTo       time.Time       `json:"valid_to"`
		Status        string          `json:"status"`
	}
	model := dao.OrdPromoCodes.Ctx(ctx).Where("code", code)
	if forUpdate {
		model = model.LockUpdate()
	}
	err := model.Scan(&promo)
	if err != nil {
		return nil, err
	}
	if promo == nil {
		return nil, lcommon.NewBusinessError(404, "优惠码无效")
	}
	if promo.Status != "active" {
		return nil, lcommon.NewBusinessError(422, fmt.Sprintf("优惠码状态异常: %s", promo.Status))
	}

	now := time.Now()
	if now.Before(promo.ValidFrom) || now.After(promo.ValidTo) {
		return nil, lcommon.NewBusinessError(422, "优惠码不在有效期内")
	}
	if promo.TotalCount > 0 && promo.UsedCount >= promo.TotalCount {
		return nil, lcommon.NewBusinessError(422, "优惠码已被全部使用")
	}

	if promo.PerUserLimit > 0 {
		userUsageCount, err := dao.OrdPromoCodeUsages.Ctx(ctx).
			Where("promo_code_id", promo.ID).
			Where("tenant_id", tenantID).
			Count()
		if err != nil {
			return nil, fmt.Errorf("query user promo usage count: %w", err)
		}
		if userUsageCount >= promo.PerUserLimit {
			return nil, lcommon.NewBusinessError(422, fmt.Sprintf("优惠码使用次数已达上限(%d次)", userUsageCount))
		}
	}

	// 套餐限制：plan_ids 为 NULL/空数组时不限，否则必须包含本次购买的套餐。
	// BIGINT[] 用 SQL 侧 ANY 判断，避免数组类型跨驱动扫描的兼容问题。
	if planID > 0 {
		matched, err := dao.OrdPromoCodes.Ctx(ctx).
			Where("id", promo.ID).
			Where("(plan_ids IS NULL OR cardinality(plan_ids) = 0 OR ? = ANY(plan_ids))", planID).
			Count()
		if err != nil {
			return nil, fmt.Errorf("check promo plan restriction: %w", err)
		}
		if matched == 0 {
			return nil, lcommon.NewBusinessError(422, "优惠码不适用于该套餐")
		}
	}

	if amount.LessThan(promo.MinAmount) {
		return nil, lcommon.NewBusinessError(422, fmt.Sprintf("订单金额不能低于 %.2f", billing.InexactFloat64(promo.MinAmount)))
	}

	var discount decimal.Decimal
	switch promo.Type {
	case "percentage":
		discount = billing.RoundMoney(amount.Mul(promo.DiscountValue).Div(decimal.NewFromInt(100)))
		if promo.MaxDiscount.GreaterThan(billing.Zero) && discount.GreaterThan(promo.MaxDiscount) {
			discount = promo.MaxDiscount
		}
	case "fixed":
		discount = promo.DiscountValue
		if discount.GreaterThan(amount) {
			discount = amount
		}
	default:
		return nil, lcommon.NewBusinessError(500, fmt.Sprintf("未知的优惠码类型: %s", promo.Type))
	}

	return &promoValidationResult{
		PromoCodeID: promo.ID,
		Type:        promo.Type,
		Discount:    discount,
		FinalAmount: billing.SubtractMoney(amount, discount),
	}, nil
}

// recordPromoUsageTx 记录优惠码使用并条件递增 used_count（须在事务 ctx 内、
// 且已通过 validatePromoCode(forUpdate=true) 持有优惠码行锁后调用）。
// 条件递增 + RowsAffected 校验是最终防线：total_count>0 时超量直接失败回滚整单。
func recordPromoUsageTx(ctx context.Context, promoCodeID, tenantID, userID, orderID int64, discount decimal.Decimal) error {
	_, err := dao.OrdPromoCodeUsages.Ctx(ctx).Insert(do.OrdPromoCodeUsages{
		PromoCodeId:    promoCodeID,
		TenantId:       tenantID,
		OrderId:        orderID,
		UserId:         userID,
		DiscountAmount: discount,
	})
	if err != nil {
		return fmt.Errorf("record promo code usage: %w", err)
	}

	result, err := dao.OrdPromoCodes.Ctx(ctx).
		Where("id", promoCodeID).
		Where("(total_count = 0 OR used_count < total_count)").
		Data(do.OrdPromoCodes{UsedCount: gdb.Raw("used_count + 1")}).
		Update()
	if err != nil {
		return fmt.Errorf("increment promo code used_count: %w", err)
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if affected == 0 {
		return lcommon.NewBusinessError(422, "优惠码已被全部使用")
	}
	return nil
}
