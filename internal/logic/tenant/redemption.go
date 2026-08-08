package tenant

import (
	"context"
	"fmt"
	"time"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/errors/gerror"
	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/os/gtime"
	"github.com/shopspring/decimal"

	do "github.com/qianfree/team-api/internal/model/do"

	"github.com/qianfree/team-api/internal/dao"
	"github.com/qianfree/team-api/internal/logic/billing"
	lcommon "github.com/qianfree/team-api/internal/logic/common"
	"github.com/qianfree/team-api/internal/logic/payment"

	v1 "github.com/qianfree/team-api/api/tenant/v1"
	lcommon2 "github.com/qianfree/team-api/internal/logic/common"
	"github.com/qianfree/team-api/internal/middleware"
)

// RedeemCode 租户兑换码
func (s *sTenant) RedeemCode(ctx context.Context, req *v1.TenantRedeemCodeReq) (*v1.TenantRedeemCodeRes, error) {
	role := middleware.GetUserRole(ctx)
	if role != "owner" && role != "admin" {
		return nil, lcommon.NewForbiddenError("需要 owner 或 admin 权限")
	}
	tenantID := middleware.GetTenantID(ctx)
	userID := middleware.GetUserID(ctx)

	var res *v1.TenantRedeemCodeRes
	// 过期兑换码的标记不能在事务内做：事务因「兑换码已过期」业务错误回滚时，
	// 标记也会一并回滚、永远落不了库。记下 ID，事务返回后在事务外单独更新。
	var expiredRedemptionID int64
	// redisCreditedAmount 记录事务内已发生的 Redis 钱包加款：Redis 是资金提交点、
	// 不受 DB 事务回滚，事务失败时必须按此补偿逆转。
	var redisCreditedAmount decimal.Decimal

	err := g.DB().Transaction(ctx, func(ctx context.Context, tx gdb.TX) error {
		// 加行锁防止并发超发
		var redemption *struct {
			ID           int64           `json:"id"`
			Type         string          `json:"type"`
			Value        decimal.Decimal `json:"value"`
			PlanID       int64           `json:"plan_id"`
			DurationDays int             `json:"duration_days"`
			MaxUses      int             `json:"max_uses"`
			UsedCount    int             `json:"used_count"`
			Status       string          `json:"status"`
			ExpiresAt    time.Time       `json:"expires_at"`
		}
		err := dao.OrdRedemptions.Ctx(ctx).
			Where("code", req.Code).
			LockUpdate().
			Scan(&redemption)
		if err != nil {
			return err
		}
		if redemption == nil {
			return lcommon.NewBadRequestError("兑换码无效")
		}
		if redemption.Status != "active" {
			return gerror.Newf("兑换码状态为%s", redemption.Status)
		}
		if !redemption.ExpiresAt.IsZero() && redemption.ExpiresAt.Before(time.Now()) {
			expiredRedemptionID = redemption.ID
			return lcommon.NewBusinessError(422, "兑换码已过期")
		}
		if redemption.UsedCount >= redemption.MaxUses {
			return lcommon.NewBusinessError(422, "兑换码已全部使用")
		}

		res = &v1.TenantRedeemCodeRes{Code: req.Code, Type: redemption.Type}
		var txID int64
		usageValue := billing.Zero

		switch redemption.Type {
		case "quota":
			var credited decimal.Decimal
			txID, credited, err = creditWalletForRedemptionTx(ctx, tenantID, redemption.Value, redemption.ID)
			// 先记录已发生的 Redis 加款（可能 >0 即使整体失败），供事务回滚后补偿逆转
			redisCreditedAmount = credited
			if err != nil {
				return err
			}
			usageValue = redemption.Value
			res.Credited = billing.InexactFloat64(redemption.Value)

		case "plan":
			if redemption.PlanID == 0 {
				return lcommon.NewBusinessError(422, "套餐兑换码缺少plan_id")
			}
			months := 1
			if redemption.DurationDays > 0 {
				months = (redemption.DurationDays + 29) / 30
				if months < 1 {
					months = 1
				}
			}
			err = payment.SubscribePlan(ctx, tenantID, redemption.PlanID, months, false)
			if err != nil {
				return gerror.Wrapf(err, "激活套餐失败")
			}
			res.PlanId = redemption.PlanID
			res.Months = months

		case "duration":
			if redemption.DurationDays <= 0 {
				return lcommon.NewBusinessError(422, "时长兑换码缺少duration_days")
			}
			err = extendPlanDurationTx(ctx, tenantID, redemption.DurationDays)
			if err != nil {
				return gerror.Wrapf(err, "延长套餐时长失败")
			}
			res.ExtendedDays = redemption.DurationDays

		default:
			return gerror.Newf("未知的兑换类型: %s", redemption.Type)
		}

		// 记录兑换使用记录
		_, err = dao.OrdRedemptionUsages.Ctx(ctx).Insert(do.OrdRedemptionUsages{
			RedemptionId:  redemption.ID,
			TenantId:      tenantID,
			UserId:        userID,
			Type:          redemption.Type,
			Value:         usageValue,
			TransactionId: txID,
		})
		if err != nil {
			return gerror.Wrapf(err, "记录兑换使用记录失败")
		}

		// 原子递增 used_count
		_, err = dao.OrdRedemptions.Ctx(ctx).
			Where("id", redemption.ID).
			Data(do.OrdRedemptions{
				UsedCount:  gdb.Raw("used_count + 1"),
				RedeemedBy: &tenantID,
				RedeemedAt: gtime.Now(),
			}).Update()
		if err != nil {
			return gerror.Wrapf(err, "更新兑换码使用计数失败")
		}

		return nil
	})
	if err != nil {
		// 过期标记在事务外落库（事务已因业务错误回滚，这里的更新才能真正生效）；
		// 条件带 status='active' 防止覆盖并发路径已写入的其他终态。
		if expiredRedemptionID != 0 {
			if _, updateErr := dao.OrdRedemptions.Ctx(ctx).
				Where("id", expiredRedemptionID).
				Where("status", "active").
				Data(do.OrdRedemptions{Status: "expired"}).
				Update(); updateErr != nil {
				g.Log().Warningf(ctx, "mark redemption %d expired failed: %v", expiredRedemptionID, updateErr)
			}
		}
		// 事务回滚：补偿逆转已发生的 Redis 钱包加款（兑换未生效，钱包不得入账）。
		// 补偿自身失败意味着钱包多入账——打 CRITICAL 日志人工追回。
		if redisCreditedAmount.GreaterThan(billing.Zero) {
			compCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 10*time.Second)
			defer cancel()
			if _, _, compErr := billing.CreditWalletRedis(compCtx, tenantID, redisCreditedAmount.Neg()); compErr != nil {
				g.Log().Errorf(ctx,
					"CRITICAL: compensate redemption redis credit failed: tenant=%d usd=%s: %v — wallet over-credited, manual fix required",
					tenantID, redisCreditedAmount.String(), compErr)
			}
		}
		return nil, err
	}

	// 兑换码充值后，重置低余额预警标记（余额可能已恢复到阈值以上）
	billing.ResetLowBalanceNotified(ctx, tenantID)

	return res, nil
}

// creditWalletForRedemptionTx 在事务内为租户钱包充值（依赖调用方传入携带事务的 ctx，
// 且处于兑换码行锁保护内）。
// amount 为 USD（bil_ 层永远 USD），decimal 直传流水字段，全程无 float64 中间运算。
// 流水类型用独立的 "redemption"：与真实充值（recharge）区分，现金对账（对支付渠道流水）
// 时不会把兑换码入账误计入充值；兑换也不推动 cumulative_recharge / 租户等级。
//
// Redis 权威化架构下：Redis 加款为资金提交点（行锁保护内执行，天然幂等）；
// bil_wallets.balance 由物化器从 Redis 覆盖，DB 侧仅写账本流水。
// 返回的 credited 为【已发生的 Redis 加款金额】（>0 即使后续失败），供事务回滚后补偿逆转。
func creditWalletForRedemptionTx(ctx context.Context, tenantID int64, amount decimal.Decimal, redemptionID int64) (int64, decimal.Decimal, error) {
	type walletRow struct {
		ID int64 `json:"id"`
	}
	var w *walletRow
	err := dao.BilWallets.Ctx(ctx).
		Where("tenant_id", tenantID).
		Fields("id").
		Scan(&w)
	if err != nil {
		return 0, billing.Zero, err
	}
	if w == nil {
		// 钱包在租户注册/管理后台建租户时初始化，正常不应缺失。这里必须报错回滚整个
		// 兑换事务：若静默返回成功，外层会照常写使用记录、递增 used_count 并向用户
		// 报告"兑换成功"，但钱包分文未入账——兑换码被无声作废，用户权益丢失且无法追溯。
		return 0, billing.Zero, gerror.Newf("租户 %d 钱包不存在，无法入账兑换额度", tenantID)
	}

	// Redis 加款（资金提交点）；流水快照取 Redis 返回值，账本链连续可信
	balanceAfter, frozenAfter, err := billing.CreditWalletRedis(ctx, tenantID, amount)
	if err != nil {
		return 0, billing.Zero, err
	}

	id, err := dao.BilTransactions.Ctx(ctx).InsertAndGetId(do.BilTransactions{
		TenantId:     tenantID,
		WalletId:     w.ID,
		Type:         "redemption",
		Amount:       amount,
		BalanceAfter: balanceAfter,
		FrozenAfter:  frozenAfter,
		RelatedId:    redemptionID,
		RelatedType:  "redemption",
		Description:  "兑换码充值",
	})
	if err != nil {
		return 0, amount, err
	}

	return id, amount, nil
}

// extendPlanDurationTx 在事务内延长套餐时长（依赖调用方传入携带事务的 ctx）
func extendPlanDurationTx(ctx context.Context, tenantID int64, days int) error {
	_, err := g.DB().Ctx(ctx).Exec(ctx,
		"UPDATE pln_tenant_plans SET end_at = end_at + ?::integer * INTERVAL '1 day' WHERE tenant_id = ? AND status = ?",
		days, tenantID, "active")
	return err
}

// ListRedemptionUsages 获取当前租户的兑换历史
func (s *sTenant) ListRedemptionUsages(ctx context.Context, req *v1.TenantRedemptionUsagesReq) (*v1.TenantRedemptionUsagesRes, error) {
	role := middleware.GetUserRole(ctx)
	if role != "owner" && role != "admin" {
		return nil, lcommon.NewForbiddenError("需要 owner 或 admin 权限")
	}
	tenantID := middleware.GetTenantID(ctx)
	page, pageSize := lcommon2.NormalizePagination(req.Page, req.PageSize)

	fromClause := "ord_redemption_usages ru LEFT JOIN ord_redemptions r ON ru.redemption_id = r.id"
	where := "WHERE ru.tenant_id = ?"
	args := []any{tenantID}

	countSQL := "SELECT COUNT(*) AS total FROM " + fromClause + " " + where
	countResult, err := g.DB().Ctx(ctx).Query(ctx, countSQL, args...)
	if err != nil {
		return nil, err
	}
	total := 0
	if len(countResult) > 0 {
		total = countResult[0]["total"].Int()
	}

	dataSQL := fmt.Sprintf(
		`SELECT ru.id, ru.redemption_id, ru.type, ru.value, ru.transaction_id, ru.created_at,
			COALESCE(r.code, '') AS code
		 FROM %s %s ORDER BY ru.created_at DESC LIMIT %d OFFSET %d`,
		fromClause, where, pageSize, (page-1)*pageSize,
	)
	result, err := g.DB().Ctx(ctx).Query(ctx, dataSQL, args...)
	if err != nil {
		return nil, err
	}

	list := make([]*v1.TenantRedemptionUsageItem, 0, len(result))
	for _, row := range result {
		list = append(list, &v1.TenantRedemptionUsageItem{
			Id:            row["id"].Int64(),
			RedemptionId:  row["redemption_id"].Int64(),
			Code:          row["code"].String(),
			Type:          row["type"].String(),
			Value:         row["value"].Float64(),
			TransactionId: row["transaction_id"].Int64(),
			CreatedAt:     gtime.NewFromTime(row["created_at"].Time()),
		})
	}

	return &v1.TenantRedemptionUsagesRes{
		List:     list,
		Total:    total,
		Page:     page,
		PageSize: pageSize,
	}, nil
}
