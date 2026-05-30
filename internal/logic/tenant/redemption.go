package tenant

import (
	"context"
	"fmt"
	"github.com/qianfree/team-api/internal/dao"
	do "github.com/qianfree/team-api/internal/model/do"
	"time"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/errors/gerror"
	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/os/gtime"

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

	var redemption *struct {
		ID           int64     `json:"id"`
		Type         string    `json:"type"`
		Value        float64   `json:"value"`
		PlanID       int64     `json:"plan_id"`
		DurationDays int       `json:"duration_days"`
		MaxUses      int       `json:"max_uses"`
		UsedCount    int       `json:"used_count"`
		Status       string    `json:"status"`
		ExpiresAt    time.Time `json:"expires_at"`
	}
	err := dao.OrdRedemptions.Ctx(ctx).
		Where("code", req.Code).
		Scan(&redemption)
	if err != nil {
		return nil, err
	}
	if redemption == nil {
		return nil, lcommon.NewBadRequestError("兑换码无效")
	}
	if redemption.Status != "active" {
		return nil, gerror.Newf("兑换码状态为%s", redemption.Status)
	}
	if !redemption.ExpiresAt.IsZero() && redemption.ExpiresAt.Before(time.Now()) {
		dao.OrdRedemptions.Ctx(ctx).
			Where("id", redemption.ID).
			Data(do.OrdRedemptions{
				Status: "expired",
			}).Update()
		return nil, lcommon.NewBusinessError(422, "兑换码已过期")
	}
	if redemption.UsedCount >= redemption.MaxUses {
		return nil, lcommon.NewBusinessError(422, "兑换码已全部使用")
	}

	res := &v1.TenantRedeemCodeRes{Code: req.Code, Type: redemption.Type}
	var txID int64
	usageValue := float64(0)

	switch redemption.Type {
	case "quota":
		txID = creditWalletForRedemption(ctx, tenantID, redemption.Value, redemption.ID)
		usageValue = redemption.Value
		res.Credited = redemption.Value

	case "plan":
		if redemption.PlanID == 0 {
			return nil, lcommon.NewBusinessError(422, "套餐兑换码缺少plan_id")
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
			return nil, gerror.Wrapf(err, "激活套餐失败")
		}
		res.PlanId = redemption.PlanID
		res.Months = months

	case "duration":
		if redemption.DurationDays <= 0 {
			return nil, lcommon.NewBusinessError(422, "时长兑换码缺少duration_days")
		}
		extendPlanDuration(ctx, tenantID, redemption.DurationDays)
		res.ExtendedDays = redemption.DurationDays

	default:
		return nil, gerror.Newf("未知的兑换类型: %s", redemption.Type)
	}

	// 记录兑换使用记录
	dao.OrdRedemptionUsages.Ctx(ctx).Insert(do.OrdRedemptionUsages{
		RedemptionId:  redemption.ID,
		TenantId:      tenantID,
		UserId:        userID,
		Type:          redemption.Type,
		Value:         usageValue,
		TransactionId: txID,
	})

	// Update usage count
	g.DB().Exec(ctx,
		"UPDATE ord_redemptions SET used_count = used_count + 1, redeemed_by = ?, redeemed_at = ?, updated_at = ? WHERE id = ?",
		tenantID, time.Now(), time.Now(), redemption.ID)

	return res, nil
}

func creditWalletForRedemption(ctx context.Context, tenantID int64, amount float64, redemptionID int64) int64 {
	type walletRow struct {
		ID int64 `json:"id"`
	}
	var w *walletRow
	dao.BilWallets.Ctx(ctx).
		Where("tenant_id", tenantID).
		Fields("id").
		Scan(&w)
	if w == nil {
		return 0
	}

	var txID int64
	err := g.DB().Transaction(ctx, func(ctx context.Context, tx gdb.TX) error {
		_, err := tx.Ctx(ctx).Exec(
			"UPDATE bil_wallets SET balance = balance + ?, updated_at = NOW() WHERE id = ?",
			amount, w.ID)
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

		id, err := tx.Model("bil_transactions").Ctx(ctx).InsertAndGetId(do.BilTransactions{
			TenantId:     tenantID,
			WalletId:     w.ID,
			Type:         "recharge",
			Amount:       amount,
			BalanceAfter: balance.Balance,
			FrozenAfter:  balance.FrozenBalance,
			RelatedId:    redemptionID,
			RelatedType:  "redemption",
			Description:  "兑换码充值",
		})
		if err != nil {
			return err
		}
		txID = id
		return nil
	})
	if err != nil {
		return 0
	}

	// 清除 Redis 钱包缓存，下次预扣时从 DB 重新同步
	billing.InvalidateWalletRedis(ctx, tenantID)

	return txID
}

func extendPlanDuration(ctx context.Context, tenantID int64, days int) {
	g.DB().Exec(ctx,
		"UPDATE pln_tenant_plans SET end_at = end_at + ?::integer * INTERVAL '1 day', updated_at = ? WHERE tenant_id = ? AND status = ?",
		days, time.Now(), tenantID, "active")
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
