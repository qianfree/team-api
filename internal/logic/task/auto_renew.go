package task

import (
	"context"
	"fmt"
	"github.com/qianfree/team-api/internal/dao"
	do "github.com/qianfree/team-api/internal/model/do"
	"time"

	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/os/gtime"
	"github.com/qianfree/team-api/internal/logic/payment"
)

// AutoRenewScanner 自动续费扫描任务
// 扫描即将到期的自动续费订阅，创建续费订单并尝试扣款
func AutoRenewScanner(ctx context.Context) {
	g.Log().Info(ctx, "[AutoRenew] starting scan...")

	// 查找 3 天内到期且开启自动续费的活跃订阅
	var subscriptions []struct {
		ID       int64  `json:"id"`
		TenantID int64  `json:"tenant_id"`
		PlanID   int64  `json:"plan_id"`
		EndAt    string `json:"end_at"`
	}
	threeDaysLater := time.Now().Add(72 * time.Hour).Format("2006-01-02 15:04:05")

	err := dao.PlnTenantPlans.Ctx(ctx).
		Where("status", "active").
		Where("auto_renew", true).
		Where("end_at < ?", threeDaysLater).
		Fields("id, tenant_id, plan_id, end_at").
		Scan(&subscriptions)
	if err != nil {
		g.Log().Errorf(ctx, "[AutoRenew] query failed: %v", err)
		return
	}

	g.Log().Infof(ctx, "[AutoRenew] found %d subscriptions to process", len(subscriptions))

	for _, sub := range subscriptions {
		processAutoRenew(ctx, sub.ID, sub.TenantID, sub.PlanID)
	}
}

func processAutoRenew(ctx context.Context, subscriptionID, tenantID, planID int64) {
	// 查用户ID（取租户的 owner）
	var userID int64
	err := dao.TntUsers.Ctx(ctx).
		Where("tenant_id", tenantID).
		Where("role", "owner").
		Where("status", "active").
		Fields("id").
		Limit(1).
		Scan(&userID)
	if err != nil || userID == 0 {
		g.Log().Warningf(ctx, "[AutoRenew] tenant %d: no active owner found, skipping", tenantID)
		return
	}

	// 创建续费订单
	orderNo := fmt.Sprintf("RNW%s%04d", time.Now().Format("20060102150405"), time.Now().UnixNano()%10000)

	result, err := dao.OrdOrders.Ctx(ctx).Insert(do.OrdOrders{
		OrderNo:        orderNo,
		TenantId:       tenantID,
		UserId:         userID,
		OrderType:      "renew",
		PlanId:         planID,
		Amount:         0, // will be filled from plan price,
		FinalAmount:    0,
		Currency:       "CNY", // 套餐价为 CNY，订单层一律 CNY（见 CLAUDE.md 三层币种固定规则）
		PaymentChannel: "auto_renew",
		Status:         "pending",
		Description:    fmt.Sprintf("Auto-renewal for subscription #%d", subscriptionID),
	})
	if err != nil {
		g.Log().Errorf(ctx, "[AutoRenew] tenant %d: create order failed: %v", tenantID, err)
		return
	}
	orderID, _ := result.LastInsertId()

	// 查套餐价格并更新订单金额
	var plan *struct {
		MonthlyPrice float64 `json:"monthly_price"`
	}
	dao.PlnPlans.Ctx(ctx).
		Where("id", planID).
		Fields("monthly_price").
		Scan(&plan)
	if plan == nil {
		g.Log().Warningf(ctx, "[AutoRenew] plan %d not found", planID)
		return
	}

	dao.OrdOrders.Ctx(ctx).
		Where("id", orderID).
		Data(do.OrdOrders{
			Amount:      plan.MonthlyPrice,
			FinalAmount: plan.MonthlyPrice,
		}).Update()

	// 标记为已支付（自动续费默认成功）
	_, err = dao.OrdOrders.Ctx(ctx).
		Where("id", orderID).
		Data(do.OrdOrders{
			Status:    "paid",
			PaidAt:    gtime.Now(),
			PaymentNo: fmt.Sprintf("AUTO_RENEW_%d", orderID),
		}).Update()
	if err != nil {
		g.Log().Errorf(ctx, "[AutoRenew] tenant %d: mark as paid failed: %v", tenantID, err)
		return
	}

	// 执行履约：续费 1 个月
	err = payment.SubscribePlan(ctx, tenantID, planID, 1, true)
	if err != nil {
		g.Log().Errorf(ctx, "[AutoRenew] tenant %d: subscribe failed: %v", tenantID, err)
		// 标记订单为失败
		dao.OrdOrders.Ctx(ctx).
			Where("id", orderID).
			Data(do.OrdOrders{
				Status: "refund_failed",
			}).Update()
		return
	}

	// 标记订单为已履约
	dao.OrdOrders.Ctx(ctx).
		Where("id", orderID).
		Data(do.OrdOrders{
			Status:      "fulfilled",
			FulfilledAt: gtime.Now(),
		}).Update()

	g.Log().Infof(ctx, "[AutoRenew] tenant %d: renewal successful (order #%d)", tenantID, orderID)
}
