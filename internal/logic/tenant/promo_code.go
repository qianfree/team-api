package tenant

import (
	"context"
	"fmt"
	"github.com/qianfree/team-api/internal/dao"
	do "github.com/qianfree/team-api/internal/model/do"
	"time"

	"github.com/gogf/gf/v2/frame/g"

	lcommon "github.com/qianfree/team-api/internal/logic/common"

	v1 "github.com/qianfree/team-api/api/tenant/v1"
)

// ValidatePromoCode 校验优惠码并返回折扣金额
func (s *sTenant) ValidatePromoCode(ctx context.Context, req *v1.TenantValidatePromoCodeReq) (*v1.TenantValidatePromoCodeRes, error) {
	tenantID := ctxTenantID(ctx)

	var promo struct {
		ID            int64     `json:"id"`
		Type          string    `json:"type"`
		DiscountValue float64   `json:"discount_value"`
		MinAmount     float64   `json:"min_amount"`
		MaxDiscount   float64   `json:"max_discount"`
		TotalCount    int       `json:"total_count"`
		UsedCount     int       `json:"used_count"`
		PerUserLimit  int       `json:"per_user_limit"`
		ValidFrom     time.Time `json:"valid_from"`
		ValidTo       time.Time `json:"valid_to"`
		Status        string    `json:"status"`
	}
	err := dao.OrdPromoCodes.Ctx(ctx).
		Where("code", req.Code).
		Scan(&promo)
	if err != nil {
		return nil, err
	}
	if promo.ID == 0 {
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

	// Check per-user limit
	if promo.PerUserLimit > 0 {
		var userUsageCount int
		dao.OrdPromoCodeUsages.Ctx(ctx).
			Where("promo_code_id", promo.ID).
			Where("tenant_id", tenantID).
			Count(&userUsageCount)
		if userUsageCount >= promo.PerUserLimit {
			return nil, lcommon.NewBusinessError(422, fmt.Sprintf("优惠码使用次数已达上限(%d次)", userUsageCount))
		}
	}

	if req.Amount < promo.MinAmount {
		return nil, lcommon.NewBusinessError(422, fmt.Sprintf("订单金额不能低于 %.2f", promo.MinAmount))
	}

	// Calculate discount
	var discount float64
	switch promo.Type {
	case "percentage":
		discount = req.Amount * promo.DiscountValue / 100
		if promo.MaxDiscount > 0 && discount > promo.MaxDiscount {
			discount = promo.MaxDiscount
		}
	case "fixed":
		discount = promo.DiscountValue
		if discount > req.Amount {
			discount = req.Amount
		}
	default:
		return nil, lcommon.NewBusinessError(500, fmt.Sprintf("未知的优惠码类型: %s", promo.Type))
	}

	return &v1.TenantValidatePromoCodeRes{Data: map[string]any{
		"promo_code_id": promo.ID,
		"type":          promo.Type,
		"discount":      discount,
		"final_amount":  req.Amount - discount,
	}}, nil
}

// applyPromoCode 应用优惠码到订单
func applyPromoCode(ctx context.Context, tenantID int64, code string, orderID int64) error {
	var promo struct {
		ID int64 `json:"id"`
	}
	err := dao.OrdPromoCodes.Ctx(ctx).
		Where("code", code).
		Scan(&promo)
	if err != nil || promo.ID == 0 {
		return lcommon.NewBusinessError(404, "优惠码无效")
	}

	// Get the discount amount from validate result
	result, err := validatePromoCodeInternal(ctx, tenantID, code, 0)
	if err != nil {
		return err
	}

	// Record usage
	_, err = dao.OrdPromoCodeUsages.Ctx(ctx).Insert(do.OrdPromoCodeUsages{
		PromoCodeId:    promo.ID,
		TenantId:       tenantID,
		OrderId:        orderID,
		UserId:         0,
		DiscountAmount: result["discount"],
	})
	if err != nil {
		return err
	}

	// Increment used_count
	g.DB().Exec(ctx,
		"UPDATE ord_promo_codes SET used_count = used_count + 1, updated_at = ? WHERE id = ?",
		time.Now(), promo.ID)

	return nil
}

// validatePromoCodeInternal is the internal version of promo code validation used by applyPromoCode.
func validatePromoCodeInternal(ctx context.Context, tenantID int64, code string, amount float64) (map[string]any, error) {
	var promo struct {
		ID            int64     `json:"id"`
		Type          string    `json:"type"`
		DiscountValue float64   `json:"discount_value"`
		MinAmount     float64   `json:"min_amount"`
		MaxDiscount   float64   `json:"max_discount"`
		TotalCount    int       `json:"total_count"`
		UsedCount     int       `json:"used_count"`
		PerUserLimit  int       `json:"per_user_limit"`
		ValidFrom     time.Time `json:"valid_from"`
		ValidTo       time.Time `json:"valid_to"`
		Status        string    `json:"status"`
	}
	err := dao.OrdPromoCodes.Ctx(ctx).
		Where("code", code).
		Scan(&promo)
	if err != nil {
		return nil, err
	}
	if promo.ID == 0 {
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
		var userUsageCount int
		dao.OrdPromoCodeUsages.Ctx(ctx).
			Where("promo_code_id", promo.ID).
			Where("tenant_id", tenantID).
			Count(&userUsageCount)
		if userUsageCount >= promo.PerUserLimit {
			return nil, lcommon.NewBusinessError(422, fmt.Sprintf("优惠码使用次数已达上限(%d次)", userUsageCount))
		}
	}

	if amount < promo.MinAmount {
		return nil, lcommon.NewBusinessError(422, fmt.Sprintf("订单金额不能低于 %.2f", promo.MinAmount))
	}

	var discount float64
	switch promo.Type {
	case "percentage":
		discount = amount * promo.DiscountValue / 100
		if promo.MaxDiscount > 0 && discount > promo.MaxDiscount {
			discount = promo.MaxDiscount
		}
	case "fixed":
		discount = promo.DiscountValue
		if discount > amount {
			discount = amount
		}
	default:
		return nil, lcommon.NewBusinessError(500, fmt.Sprintf("未知的优惠码类型: %s", promo.Type))
	}

	return g.Map{
		"promo_code_id": promo.ID,
		"type":          promo.Type,
		"discount":      discount,
		"final_amount":  amount - discount,
	}, nil
}
