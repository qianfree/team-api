package payment

import (
	"context"
	"fmt"

	do "github.com/qianfree/team-api/internal/model/do"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/errors/gerror"
	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/os/gtime"
	"github.com/shopspring/decimal"

	"github.com/qianfree/team-api/internal/dao"
	"github.com/qianfree/team-api/internal/logic/billing"
)

// FulfillOrder 履约订单（事务内完成：履约+更新订单状态）
func FulfillOrder(ctx context.Context, orderID int64) error {
	// walletTouched 标记本次履约是否变更了钱包余额；钱包缓存失效必须在事务提交后执行，
	// 否则并发 GetWallet 可能在提交前读到旧 DB 值并回写 Redis，污染缓存。
	var walletTouchedTenantID int64
	var walletTouched bool

	err := g.DB().Transaction(ctx, func(ctx context.Context, tx gdb.TX) error {
		var order *struct {
			TenantID    int64   `json:"tenant_id"`
			UserID      int64   `json:"user_id"`
			OrderType   string  `json:"order_type"`
			PlanID      int64   `json:"plan_id"`
			Amount      float64 `json:"amount"`
			FinalAmount float64 `json:"final_amount"`
			Status      string  `json:"status"`
		}
		// 无行锁时两个并发事务（如支付回调与管理后台手动履约）可同时读到 paid 各自履约 → 重复入账/重复发套餐。
		// 加锁后后到的事务阻塞，待前者提交（状态已改为 fulfilled）后再读到最新状态，据此跳过。
		err := dao.OrdOrders.Ctx(ctx).
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
			if err = subscribePlanTx(ctx, order.TenantID, order.PlanID, months, true); err != nil {
				return gerror.Wrapf(err, "subscribe plan failed")
			}

		case "recharge":
			// 折扣语义「付折后价、按原价入账」：用户实付 FinalAmount（折后 CNY），
			// 钱包按订单原价 Amount 换算 USD 入账，折扣让利真实生效；无折扣时两者相等。
			// 汇率只取一次，保证入账换算与快照使用同一汇率。
			rate := billing.GetExchangeRateCNYToUSD(ctx)
			usdAmount := billing.CeilUSD(billing.NewFromFloat(order.Amount).Mul(billing.NewFromFloat(rate)))
			if err = creditWalletTx(ctx, order.TenantID, usdAmount, orderID, fmt.Sprintf("Recharge: order #%d (CNY %.2f, paid %.2f → USD %.6f)", orderID, order.Amount, order.FinalAmount, usdAmount.InexactFloat64())); err != nil {
				return gerror.Wrapf(err, "credit wallet failed")
			}
			// 汇率结构化快照：持久化「原始 CNY（订单列）+ 当时汇率 + 入账 USD」，
			// 使历史换算可重建，汇率配置变更不影响已完成订单的现金对账。
			if _, err = g.DB().Ctx(ctx).Exec(ctx,
				"UPDATE ord_orders SET exchange_rate = ?, credited_usd = ? WHERE id = ?",
				billing.NewFromFloat(rate), usdAmount, orderID); err != nil {
				return gerror.Wrapf(err, "snapshot exchange rate failed")
			}
			_ = billing.CheckAndUpgradeLevel(ctx, order.TenantID)
			// 充值后检查是否需要重置低余额预警标记
			billing.ResetLowBalanceNotified(ctx, order.TenantID)
			walletTouchedTenantID = order.TenantID
			walletTouched = true

		default:
			return gerror.Newf("unsupported order type for fulfillment: %s", order.OrderType)
		}

		// 持有行锁时理论上不会出现 0 行；一旦出现说明状态被并发改动，回滚整个履约事务以防重复入账。
		res, err := dao.OrdOrders.Ctx(ctx).
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
	if err != nil {
		return err
	}

	// 事务提交后再清除钱包两级缓存（进程内 walletCache + Redis），避免充值后 GetWallet 在 300s 内仍返回旧余额。
	// 放在提交后执行，避免「DEL → 并发 GetWallet 读到旧 DB 值回写」的缓存污染窗口。
	if walletTouched {
		billing.InvalidateWallet(ctx, walletTouchedTenantID)
	}
	return nil
}

// planSubscribeLockNS 是套餐订阅串行化 advisory lock 的命名空间前缀。
// 采用单 key 形式 pg_advisory_xact_lock(bigint)，key = NS + tenantID：
// NS 取高位常量段，与其它业务模块（直接以主键为 key）的 advisory lock 隔离，避免冲突。
const planSubscribeLockNS int64 = 9_001_000_000_000_000

// subscribePlanTx 在事务内订阅套餐（依赖调用方传入携带事务的 ctx，内部统一用 dao.Xxx.Ctx(ctx) 传播）
func subscribePlanTx(ctx context.Context, tenantID int64, planID int64, months int, autoRenew bool) error {
	// 串行化同一租户的套餐订阅：两个并发履约（兑换码 + 自动续费、双支付回调等）可能同时读到
	// active 订阅并各自「置 expired + INSERT 新 active」，最终留下两条 active 订阅。
	// pg_advisory_xact_lock 随事务提交/回滚自动释放，且不占用任何行锁，无死锁风险；
	// 后到事务阻塞至前者提交（旧 active 已 expired）后再进入临界区，据此跳过重复订阅。
	if _, err := g.DB().Ctx(ctx).Exec(ctx, "SELECT pg_advisory_xact_lock(?)", planSubscribeLockNS+tenantID); err != nil {
		return gerror.Wrapf(err, "acquire subscribe advisory lock for tenant %d", tenantID)
	}

	var plan *struct {
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
	if plan == nil {
		return gerror.Newf("plan %d not found or inactive", planID)
	}

	if months <= 0 {
		months = 1
	}

	now := gtime.Now()
	endAt := now.AddDate(0, months, 0)

	// 先取消当前活跃订阅
	_, err = dao.PlnTenantPlans.Ctx(ctx).
		Where("tenant_id", tenantID).
		Where("status", "active").
		Data(do.PlnTenantPlans{
			Status:      "expired",
			CancelledAt: gtime.Now(),
		}).Update()
	if err != nil {
		return gerror.Wrapf(err, "cancel old plan for tenant %d", tenantID)
	}

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
	return err
}

// SubscribePlan 订阅套餐（公开函数，被自动续费、兑换码等外部调用）
func SubscribePlan(ctx context.Context, tenantID int64, planID int64, months int, autoRenew bool) error {
	return g.DB().Transaction(ctx, func(ctx context.Context, tx gdb.TX) error {
		return subscribePlanTx(ctx, tenantID, planID, months, autoRenew)
	})
}

// creditWalletTx 在事务内钱包入账（依赖调用方传入携带事务的 ctx）
// amount 为 USD（bil_ 层永远 USD）；用 decimal 直传原生 SQL（shopspring decimal 实现
// driver.Valuer → 精确 NUMERIC 字符串），避免 float64 入账在 balance+? 累加时产生漂移。
// orderID 写入流水 related_id：退款时靠它精确找回履约当时入账的 USD（禁止按当前汇率反算）。
func creditWalletTx(ctx context.Context, tenantID int64, amount decimal.Decimal, orderID int64, description string) error {
	var w *struct {
		ID int64 `json:"id"`
	}
	err := dao.BilWallets.Ctx(ctx).
		Where("tenant_id", tenantID).
		Fields("id").
		Scan(&w)
	if err != nil {
		return err
	}
	if w == nil {
		return gerror.Newf("wallet not found for tenant %d", tenantID)
	}

	_, err = g.DB().Ctx(ctx).Exec(ctx,
		"UPDATE bil_wallets SET balance = balance + ?, cumulative_recharge = cumulative_recharge + ?, updated_at = NOW() WHERE id = ?",
		amount, amount, w.ID)
	if err != nil {
		return err
	}

	var balance *struct {
		Balance       decimal.Decimal `json:"balance"`
		FrozenBalance decimal.Decimal `json:"frozen_balance"`
	}
	err = dao.BilWallets.Ctx(ctx).
		Where("id", w.ID).
		Fields("balance, frozen_balance").
		Scan(&balance)
	if err != nil {
		return err
	}
	if balance == nil {
		return gerror.New("wallet not found after update")
	}

	_, err = dao.BilTransactions.Ctx(ctx).Insert(do.BilTransactions{
		TenantId:     tenantID,
		WalletId:     w.ID,
		Type:         "recharge",
		Amount:       amount,
		BalanceAfter: balance.Balance,
		FrozenAfter:  balance.FrozenBalance,
		RelatedId:    orderID,
		RelatedType:  "order",
		Description:  description,
	})
	if err != nil {
		return err
	}

	// 注意：钱包缓存失效（billing.InvalidateWallet）不在事务内调用——事务尚未提交时 DEL Redis，
	// 并发 GetWallet 可能从 DB 读到旧余额并回写 Redis，导致缓存被旧值重新污染。
	// 失效由调用方 FulfillOrder 在事务提交后统一执行。
	return nil
}
