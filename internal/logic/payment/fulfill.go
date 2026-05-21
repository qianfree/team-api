package payment

import (
	"context"
	"fmt"
	do "github.com/qianfree/team-api/internal/model/do"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/errors/gerror"
	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/os/gtime"

	"github.com/qianfree/team-api/internal/dao"
	"github.com/qianfree/team-api/internal/logic/billing"
)

// FulfillOrder 履约订单
func FulfillOrder(ctx context.Context, orderID int64) error {
	var order struct {
		TenantID    int64   `json:"tenant_id"`
		UserID      int64   `json:"user_id"`
		OrderType   string  `json:"order_type"`
		PlanID      int64   `json:"plan_id"`
		FinalAmount float64 `json:"final_amount"`
		Status      string  `json:"status"`
	}
	err := dao.OrdOrders.Ctx(ctx).
		Where("id", orderID).
		Scan(&order)
	if err != nil {
		return err
	}
	if order.Status != "paid" {
		return gerror.New("order status must be paid to fulfill")
	}

	switch order.OrderType {
	case "new_plan", "renew", "upgrade":
		months := 1
		if order.OrderType == "renew" {
			// 默认续费 1 个月，可根据需求从订单描述中解析
			months = 1
		}
		err = SubscribePlan(ctx, order.TenantID, order.PlanID, months, true)
		if err != nil {
			return gerror.Wrapf(err, "subscribe plan failed")
		}

	case "recharge":
		// 充值金额为 CNY，转换为 USD 后入账钱包
		usdAmount := billing.ConvertCNYToUSD(ctx, order.FinalAmount)
		err = creditWallet(ctx, order.TenantID, usdAmount, fmt.Sprintf("Recharge: order #%d (CNY %.2f → USD %.6f)", orderID, order.FinalAmount, usdAmount))
		if err != nil {
			return gerror.Wrapf(err, "credit wallet failed")
		}

	default:
		return gerror.Newf("unsupported order type for fulfillment: %s", order.OrderType)
	}

	// 标记订单为已履约
	_, err = dao.OrdOrders.Ctx(ctx).
		Where("id", orderID).
		Data(do.OrdOrders{
			Status:      "fulfilled",
			FulfilledAt: gtime.Now(),
		}).Update()
	return err
}

// SubscribePlan 订阅套餐（内部函数，被订单履约、自动续费调用）
func SubscribePlan(ctx context.Context, tenantID int64, planID int64, months int, autoRenew bool) error {
	// 查套餐信息
	var plan struct {
		MonthlyPrice       float64 `json:"monthly_price"`
		YearlyPrice        float64 `json:"yearly_price"`
		MonthlyQuotaTokens int64   `json:"monthly_quota_tokens"`
	}
	err := dao.PlnPlans.Ctx(ctx).
		Where("id", planID).
		Where("status", "active").
		Scan(&plan)
	if err != nil {
		return err
	}

	if months <= 0 {
		months = 1
	}

	now := gtime.Now()
	endAt := now.AddDate(0, months, 0)

	// 先取消当前活跃订阅
	_, _ = dao.PlnTenantPlans.Ctx(ctx).
		Where("tenant_id", tenantID).
		Where("status", "active").
		Data(do.PlnTenantPlans{
			Status:      "expired",
			CancelledAt: gtime.Now(),
		}).Update()

	_, err = dao.PlnTenantPlans.Ctx(ctx).Insert(do.PlnTenantPlans{
		TenantId:           tenantID,
		PlanId:             planID,
		Status:             "active",
		StartAt:            now,
		EndAt:              endAt,
		AutoRenew:          autoRenew,
		MonthlyQuotaTokens: plan.MonthlyQuotaTokens,
		UsedTokens:         0,
		LastResetAt:        now,
	})
	if err != nil {
		return err
	}

	return nil
}

// creditWallet 钱包入账
func creditWallet(ctx context.Context, tenantID int64, amount float64, description string) error {
	type walletRow struct {
		ID int64 `json:"id"`
	}
	var w walletRow
	err := dao.BilWallets.Ctx(ctx).
		Where("tenant_id", tenantID).
		Fields("id").
		Scan(&w)
	if err != nil || w.ID == 0 {
		return gerror.Newf("wallet not found for tenant %d", tenantID)
	}

	err = g.DB().Transaction(ctx, func(ctx context.Context, tx gdb.TX) error {
		_, err := tx.Ctx(ctx).Exec(
			"UPDATE bil_wallets SET balance = balance + ?, updated_at = NOW() WHERE id = ?",
			amount, w.ID)
		if err != nil {
			return err
		}

		var balance struct {
			Balance       float64 `json:"balance"`
			FrozenBalance float64 `json:"frozen_balance"`
		}
		err = tx.Model("bil_wallets").Ctx(ctx).
			Where("id", w.ID).
			Fields("balance, frozen_balance").
			Scan(&balance)
		if err != nil {
			return err
		}

		_, err = tx.Model("bil_transactions").Ctx(ctx).Insert(do.BilTransactions{
			TenantId:     tenantID,
			WalletId:     w.ID,
			Type:         "recharge",
			Amount:       amount,
			BalanceAfter: balance.Balance,
			FrozenAfter:  balance.FrozenBalance,
			Description:  description,
		})
		return err
	})
	if err != nil {
		return err
	}

	// 清除 Redis 钱包缓存，下次预扣时从 DB 重新同步
	billing.InvalidateWalletRedis(ctx, tenantID)

	return nil
}
