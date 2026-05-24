package payment

import (
	"context"
	"fmt"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/errors/gerror"
	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/os/gtime"

	"github.com/qianfree/team-api/internal/dao"
	"github.com/qianfree/team-api/internal/logic/billing"
	do "github.com/qianfree/team-api/internal/model/do"
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
	case "new_plan":
		err = fulfillNewPlan(ctx, order.TenantID, orderID, order.PlanID, order.FinalAmount)
		if err != nil {
			return gerror.Wrapf(err, "fulfill new plan failed")
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

// fulfillNewPlan 购买套餐履约
func fulfillNewPlan(ctx context.Context, tenantID int64, orderID int64, planID int64, paidCNY float64) error {
	// 查套餐信息
	var plan struct {
		Id           int64   `json:"id"`
		Status       string  `json:"status"`
		CreditAmount float64 `json:"credit_amount"`
		BonusAmount  float64 `json:"bonus_amount"`
		ValidityDays int     `json:"validity_days"`
		Stock        int     `json:"stock"`
	}
	err := dao.PlnPlans.Ctx(ctx).
		Where("id", planID).
		Scan(&plan)
	if err != nil {
		return err
	}
	if plan.Status != "active" {
		return gerror.New("plan is not active")
	}

	totalCredits := plan.CreditAmount + plan.BonusAmount

	err = g.DB().Transaction(ctx, func(ctx context.Context, tx gdb.TX) error {
		now := gtime.Now()
		endAt := now.AddDate(0, 0, plan.ValidityDays)

		// 扣减库存、累加已购次数
		if plan.Stock > 0 {
			result, err := tx.Ctx(ctx).Exec(
				"UPDATE pln_plans SET stock = stock - 1, total_purchased = total_purchased + 1, updated_at = NOW() WHERE id = ? AND stock > 0",
				plan.Id)
			if err != nil {
				return err
			}
			rows, _ := result.RowsAffected()
			if rows == 0 {
				return gerror.New("plan sold out")
			}
		} else {
			_, err := tx.Ctx(ctx).Exec(
				"UPDATE pln_plans SET total_purchased = total_purchased + 1, updated_at = NOW() WHERE id = ?",
				plan.Id)
			if err != nil {
				return err
			}
		}

		// 插入租户套餐记录
		_, err = tx.Ctx(ctx).Model("pln_tenant_plans").Insert(do.PlnTenantPlans{
			TenantId:         tenantID,
			PlanId:           planID,
			Status:           "active",
			StartAt:          now,
			EndAt:            endAt,
			TotalCredits:     totalCredits,
			RemainingCredits: totalCredits,
			PaidCny:          paidCNY,
			OrderId:          orderID,
		})
		return err
	})

	return err
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
