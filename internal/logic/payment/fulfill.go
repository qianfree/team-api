package payment

import (
	"context"
	"fmt"
	do "github.com/qianfree/team-api/internal/model/do"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/errors/gerror"
	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/os/gtime"
	"github.com/qianfree/team-api/internal/logic/billing"
)

// FulfillOrder 履约订单（事务内完成：履约+更新订单状态）
func FulfillOrder(ctx context.Context, orderID int64) error {
	return g.DB().Transaction(ctx, func(ctx context.Context, tx gdb.TX) error {
		var order *struct {
			TenantID    int64   `json:"tenant_id"`
			UserID      int64   `json:"user_id"`
			OrderType   string  `json:"order_type"`
			PlanID      int64   `json:"plan_id"`
			FinalAmount float64 `json:"final_amount"`
			Status      string  `json:"status"`
		}
		// A6 修复：SELECT ... FOR UPDATE 对订单行加锁，串行化并发履约。
		// 无行锁时两个并发事务（如支付回调与管理后台手动履约）可同时读到 paid 各自履约 → 重复入账/重复发套餐。
		// 加锁后后到的事务阻塞，待前者提交（状态已改为 fulfilled）后再读到最新状态，据此跳过。
		err := tx.Model("ord_orders").Ctx(ctx).
			Where("id", orderID).
			LockUpdate().
			Scan(&order)
		if err != nil {
			return err
		}
		if order == nil {
			return gerror.Newf("order %d not found", orderID)
		}
		// 已履约：幂等空操作（并发后到者 / 回调重放 / 管理后台重复点击都会走到这里）
		if order.Status == "fulfilled" {
			return nil
		}
		if order.Status != "paid" {
			return gerror.New("order status must be paid to fulfill")
		}

		switch order.OrderType {
		case "new_plan", "renew", "upgrade":
			months := 1
			if order.OrderType == "renew" {
				months = 1
			}
			if err = subscribePlanTx(ctx, tx, order.TenantID, order.PlanID, months, true); err != nil {
				return gerror.Wrapf(err, "subscribe plan failed")
			}

		case "recharge":
			usdAmount := billing.ConvertCNYToUSD(ctx, order.FinalAmount)
			if err = creditWalletTx(ctx, tx, order.TenantID, usdAmount, fmt.Sprintf("Recharge: order #%d (CNY %.2f → USD %.6f)", orderID, order.FinalAmount, usdAmount)); err != nil {
				return gerror.Wrapf(err, "credit wallet failed")
			}
			_ = billing.CheckAndUpgradeLevel(ctx, order.TenantID)
			// 充值后检查是否需要重置低余额预警标记
			billing.ResetLowBalanceNotified(ctx, order.TenantID)

		default:
			return gerror.Newf("unsupported order type for fulfillment: %s", order.OrderType)
		}

		// A6 修复：末尾状态流转用条件更新 paid → fulfilled 并校验 RowsAffected，作为纵深防御。
		// 持有行锁时理论上不会出现 0 行；一旦出现说明状态被并发改动，回滚整个履约事务以防重复入账。
		res, err := tx.Ctx(ctx).Model("ord_orders").
			Where("id", orderID).
			Where("status", "paid").
			Data(do.OrdOrders{
				Status:      "fulfilled",
				FulfilledAt: gtime.Now(),
			}).Update()
		if err != nil {
			return err
		}
		affected, err := res.RowsAffected()
		if err != nil {
			return err
		}
		if affected == 0 {
			return gerror.Newf("order %d fulfill aborted: status changed concurrently", orderID)
		}
		return nil
	})
}

// subscribePlanTx 在事务内订阅套餐
func subscribePlanTx(ctx context.Context, tx gdb.TX, tenantID int64, planID int64, months int, autoRenew bool) error {
	var plan *struct {
		MonthlyPrice       float64 `json:"monthly_price"`
		YearlyPrice        float64 `json:"yearly_price"`
		MonthlyQuotaTokens int64   `json:"monthly_quota_tokens"`
	}
	err := tx.Model("pln_plans").Ctx(ctx).
		Where("id", planID).
		Where("status", "active").
		Scan(&plan)
	if err != nil {
		return err
	}
	if plan == nil {
		return gerror.Newf("plan %d not found or inactive", planID)
	}

	if months <= 0 {
		months = 1
	}

	now := gtime.Now()
	endAt := now.AddDate(0, months, 0)

	// 先取消当前活跃订阅
	_, err = tx.Model("pln_tenant_plans").Ctx(ctx).
		Where("tenant_id", tenantID).
		Where("status", "active").
		Data(do.PlnTenantPlans{
			Status:      "expired",
			CancelledAt: gtime.Now(),
		}).Update()
	if err != nil {
		return gerror.Wrapf(err, "cancel old plan for tenant %d", tenantID)
	}

	_, err = tx.Model("pln_tenant_plans").Ctx(ctx).Insert(do.PlnTenantPlans{
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
	return err
}

// SubscribePlan 订阅套餐（公开函数，被自动续费、兑换码等外部调用）
func SubscribePlan(ctx context.Context, tenantID int64, planID int64, months int, autoRenew bool) error {
	return g.DB().Transaction(ctx, func(ctx context.Context, tx gdb.TX) error {
		return subscribePlanTx(ctx, tx, tenantID, planID, months, autoRenew)
	})
}

// creditWalletTx 在事务内钱包入账
func creditWalletTx(ctx context.Context, tx gdb.TX, tenantID int64, amount float64, description string) error {
	var w *struct {
		ID int64 `json:"id"`
	}
	err := tx.Model("bil_wallets").Ctx(ctx).
		Where("tenant_id", tenantID).
		Fields("id").
		Scan(&w)
	if err != nil {
		return err
	}
	if w == nil {
		return gerror.Newf("wallet not found for tenant %d", tenantID)
	}

	_, err = tx.Ctx(ctx).Exec(
		"UPDATE bil_wallets SET balance = balance + ?, cumulative_recharge = cumulative_recharge + ?, updated_at = NOW() WHERE id = ?",
		amount, amount, w.ID)
	if err != nil {
		return err
	}

	var balance *struct {
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
	if balance == nil {
		return gerror.New("wallet not found after update")
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
	if err != nil {
		return err
	}

	// 清除 Redis 钱包缓存
	billing.InvalidateWalletRedis(ctx, tenantID)
	return nil
}
