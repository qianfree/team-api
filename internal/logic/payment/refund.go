package payment

import (
	"context"
	"math"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/errors/gerror"
	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/os/gtime"

	"github.com/qianfree/team-api/internal/dao"
	do "github.com/qianfree/team-api/internal/model/do"
)

// CalculateRefundAmount 计算套餐订单的可退金额
// 公式：refund = paid_cny × (remaining_credits / total_credits)
func CalculateRefundAmount(ctx context.Context, orderID int64) (refundCNY float64, canRefund bool, err error) {
	var tenantPlan struct {
		Id               int64   `json:"id"`
		Status           string  `json:"status"`
		TotalCredits     float64 `json:"total_credits"`
		RemainingCredits float64 `json:"remaining_credits"`
		PaidCny          float64 `json:"paid_cny"`
	}
	err = dao.PlnTenantPlans.Ctx(ctx).
		Where("order_id", orderID).
		Scan(&tenantPlan)
	if err != nil {
		return 0, false, err
	}
	if tenantPlan.Id == 0 {
		return 0, false, nil
	}
	if tenantPlan.Status != "active" {
		return 0, false, nil
	}

	if tenantPlan.TotalCredits <= 0 || tenantPlan.PaidCny <= 0 {
		return 0, true, nil
	}

	refundCNY = tenantPlan.PaidCny * (tenantPlan.RemainingCredits / tenantPlan.TotalCredits)
	// 向下取整到分
	refundCNY = math.Floor(refundCNY*100) / 100

	return refundCNY, true, nil
}

// RefundPlanOrder 执行套餐订单退款
func RefundPlanOrder(ctx context.Context, orderID int64, adminUserID int64, reason string) error {
	// 查订单
	var order struct {
		Id             int64   `json:"id"`
		TenantId       int64   `json:"tenant_id"`
		OrderType      string  `json:"order_type"`
		FinalAmount    float64 `json:"final_amount"`
		PaymentChannel string  `json:"payment_channel"`
		Status         string  `json:"status"`
	}
	err := dao.OrdOrders.Ctx(ctx).
		Where("id", orderID).
		Scan(&order)
	if err != nil {
		return err
	}
	if order.Id == 0 {
		return gerror.New("order not found")
	}
	if order.Status != "paid" && order.Status != "fulfilled" {
		return gerror.New("order status must be paid or fulfilled to refund")
	}

	// 计算可退金额
	refundCNY, canRefund, err := CalculateRefundAmount(ctx, orderID)
	if err != nil {
		return err
	}
	if !canRefund {
		return gerror.New("no active plan found for this order")
	}

	// 事务：更新租户套餐状态 + 创建退款记录 + 更新订单状态
	err = g.DB().Transaction(ctx, func(ctx context.Context, tx gdb.TX) error {
		// 将关联的租户套餐设为已退款
		_, err := tx.Ctx(ctx).Exec(
			"UPDATE pln_tenant_plans SET status = 'refunded', remaining_credits = 0, refunded_at = NOW(), updated_at = NOW() WHERE order_id = ? AND status = 'active'",
			orderID)
		if err != nil {
			return err
		}

		// 创建退款记录
		_, err = tx.Ctx(ctx).Model("ord_refunds").Insert(do.OrdRefunds{
			OrderId:        orderID,
			TenantId:       order.TenantId,
			Amount:         refundCNY,
			Reason:         reason,
			Status:         "approved",
			PaymentChannel: order.PaymentChannel,
			ApprovedBy:     adminUserID,
			ApprovedAt:     gtime.Now(),
		})
		if err != nil {
			return err
		}

		// 更新订单状态为已退款
		_, err = tx.Ctx(ctx).Exec(
			"UPDATE ord_orders SET status = 'refunded', updated_at = NOW() WHERE id = ?",
			orderID)
		return err
	})

	// 尝试调用支付渠道退款（非阻塞，失败不影响业务状态）
	if err == nil && refundCNY > 0 && order.PaymentChannel != "" && order.PaymentChannel != "mock" {
		provider := GetProvider(order.PaymentChannel)
		if provider != nil {
			jsonStr, _, cfgErr := LoadChannelConfig(ctx, order.PaymentChannel)
			if cfgErr == nil {
				_ = provider.Refund(ctx, &RefundRequest{
					OrderID:      orderID,
					RefundAmount: refundCNY,
					Reason:       reason,
				}, jsonStr)
			}
		}
	}

	return err
}
