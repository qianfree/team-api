package payment

import (
	"context"
	"fmt"
	"time"

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
	// redisCredited* 记录事务内已发生的 Redis 钱包加款：Redis 是资金提交点、不受 DB 事务回滚，
	// 事务失败时必须按此补偿逆转，防止「订单未履约但钱包已入账」。
	var redisCreditedTenantID int64
	var redisCreditedAmount decimal.Decimal

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
			// 钱包按订单原价 Amount 入账，折扣让利真实生效；无折扣时两者相等。
			var creditAmount decimal.Decimal
			var desc string
			if billing.IsCNY(ctx) {
				// 本位币=CNY：订单层与记账层同为人民币，按原价直接入账，无换汇环节
				creditAmount = billing.NewFromFloat(order.Amount)
				desc = fmt.Sprintf("Recharge: order #%d (CNY %.2f, paid %.2f → CNY %.2f)", orderID, order.Amount, order.FinalAmount, creditAmount.InexactFloat64())
			} else {
				// 本位币=USD：按原价换算美元入账；汇率只取一次，保证入账换算与快照使用同一汇率。
				rate := billing.GetExchangeRateCNYToUSD(ctx)
				creditAmount = billing.CeilUSD(billing.NewFromFloat(order.Amount).Mul(billing.NewFromFloat(rate)))
				desc = fmt.Sprintf("Recharge: order #%d (CNY %.2f, paid %.2f → USD %.6f)", orderID, order.Amount, order.FinalAmount, creditAmount.InexactFloat64())
			}
			credited, err := creditWalletTx(ctx, order.TenantID, creditAmount, orderID, desc)
			// 先记录已发生的 Redis 加款（可能 >0 即使整体失败），供事务回滚后补偿逆转
			redisCreditedTenantID = order.TenantID
			redisCreditedAmount = credited
			if err != nil {
				return gerror.Wrapf(err, "credit wallet failed")
			}
			// 汇率结构化快照：持久化「原始 CNY（订单列）+ 当时汇率 + 入账金额」，
			// 使历史换算可重建，汇率配置变更不影响已完成订单的现金对账。
			// 本位币=CNY 时 exchange_rate=1（无换汇）、credited_usd 存本位币金额。
			snapRate := billing.One
			if !billing.IsCNY(ctx) {
				snapRate = billing.NewFromFloat(billing.GetExchangeRateCNYToUSD(ctx))
			}
			if _, err = g.DB().Ctx(ctx).Exec(ctx,
				"UPDATE ord_orders SET exchange_rate = ?, credited_usd = ? WHERE id = ?",
				snapRate, creditAmount, orderID); err != nil {
				return gerror.Wrapf(err, "snapshot exchange rate failed")
			}
			_ = billing.CheckAndUpgradeLevel(ctx, order.TenantID)
			// 充值后检查是否需要重置低余额预警标记
			billing.ResetLowBalanceNotified(ctx, order.TenantID)

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
		// 事务回滚：补偿逆转已发生的 Redis 钱包加款（订单未履约，钱包不得入账）。
		// 补偿自身失败意味着钱包多入账——打 CRITICAL 日志人工追回。
		if redisCreditedAmount.GreaterThan(billing.Zero) {
			compCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 10*time.Second)
			defer cancel()
			if _, _, compErr := billing.CreditWalletRedis(compCtx, redisCreditedTenantID, redisCreditedAmount.Neg()); compErr != nil {
				g.Log().Errorf(ctx,
					"CRITICAL: compensate recharge redis credit failed: tenant=%d usd=%s order=%d: %v — wallet over-credited, manual fix required",
					redisCreditedTenantID, redisCreditedAmount.String(), orderID, compErr)
			}
		}
		return err
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

// creditWalletTx 钱包入账（依赖调用方传入携带事务的 ctx，且处于订单行锁保护内）。
// amount 为本位币金额（bil_ 层币种 = 部署级本位币，USD 或 CNY）；orderID 写入流水
// related_id：退款时靠它精确找回履约当时入账的本位币金额（禁止按当前汇率反算）。
//
// Redis 权威化架构下的职责拆分：
//   - Redis 加款是资金提交点（实时余额立即生效），在行锁保护内执行，并发回调串行化、天然幂等；
//   - DB 侧只维护「累计充值 + 账本流水」——bil_wallets.balance 由物化器从 Redis 覆盖，
//     不再在事务内更新，避免与滞后物化值互相回滚；
//   - 返回值为【已发生的 Redis 加款金额】（>0 即使后续 DB 步骤失败），供调用方在事务
//     回滚后补偿逆转。
func creditWalletTx(ctx context.Context, tenantID int64, amount decimal.Decimal, orderID int64, description string) (decimal.Decimal, error) {
	var w *struct {
		ID int64 `json:"id"`
	}
	err := dao.BilWallets.Ctx(ctx).
		Where("tenant_id", tenantID).
		Fields("id").
		Scan(&w)
	if err != nil {
		return billing.Zero, err
	}
	if w == nil {
		return billing.Zero, gerror.Newf("wallet not found for tenant %d", tenantID)
	}

	// Redis 加款（资金提交点）；流水快照取 Redis 返回值，账本链连续可信
	balanceAfter, frozenAfter, err := billing.CreditWalletRedis(ctx, tenantID, amount)
	if err != nil {
		return billing.Zero, err
	}

	// DB 仅累加累计充值（decimal 直传 NUMERIC，driver.Valuer 精确字符串无漂移）
	_, err = g.DB().Ctx(ctx).Exec(ctx,
		"UPDATE bil_wallets SET cumulative_recharge = cumulative_recharge + ?, updated_at = NOW() WHERE id = ?",
		amount, w.ID)
	if err != nil {
		return amount, err
	}

	_, err = dao.BilTransactions.Ctx(ctx).Insert(do.BilTransactions{
		TenantId:     tenantID,
		WalletId:     w.ID,
		Type:         "recharge",
		Amount:       amount,
		BalanceAfter: balanceAfter,
		FrozenAfter:  frozenAfter,
		RelatedId:    orderID,
		RelatedType:  "order",
		Description:  description,
	})
	if err != nil {
		return amount, err
	}

	return amount, nil
}
