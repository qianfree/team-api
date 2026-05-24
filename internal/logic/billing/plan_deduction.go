package billing

import (
	"context"
	"fmt"
	"time"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/frame/g"

	lcommon "github.com/qianfree/team-api/internal/logic/common"
)

const (
	// planCacheTTL 套餐缓存 TTL
	planCacheTTL = 30 * time.Second
)

// planCache 套餐缓存
var planCache = lcommon.NewCache("plan_deduction", planCacheTTL)

// ActivePlan 租户持有的活跃套餐
type ActivePlan struct {
	ID               int64    `json:"id"`
	PlanID           int64    `json:"plan_id"`
	RemainingCredits float64  `json:"remaining_credits"`
	TotalCredits     float64  `json:"total_credits"`
	EndAt            string   `json:"end_at"`
	AllowedModels    []string `json:"allowed_models"`
}

// GetActivePlansForModel 查询租户对指定模型可用的活跃套餐（按 end_at ASC，优先消费快过期的）
func GetActivePlansForModel(ctx context.Context, tenantID int64, modelName string) ([]*ActivePlan, error) {
	cacheKey := fmt.Sprintf("%d:%s", tenantID, modelName)
	var cached []*ActivePlan
	if planCache.GetJSON(ctx, cacheKey, &cached) {
		return cached, nil
	}

	// 查询活跃套餐（allowed_models 为空数组表示全部模型，否则模型必须在列表中）
	query := g.DB().Model("pln_tenant_plans tp").
		Ctx(ctx).
		LeftJoin("pln_plans p", "p.id = tp.plan_id").
		Where("tp.tenant_id", tenantID).
		Where("tp.status", "active").
		Where("tp.remaining_credits > 0").
		Where("tp.end_at > NOW()")

	if modelName != "" {
		// allowed_models 为空数组（全部模型）OR 模型在 allowed_models 内
		query = query.Where("(p.allowed_models = '{}' OR ? = ANY(p.allowed_models))", modelName)
	}

	var plans []*ActivePlan
	err := query.Fields("tp.id, tp.plan_id, tp.remaining_credits, tp.total_credits, tp.end_at::text, COALESCE(p.allowed_models, '{}') as allowed_models").
		Order("tp.end_at ASC").
		Scan(&plans)
	if err != nil {
		return nil, err
	}

	if plans == nil {
		plans = []*ActivePlan{}
	}

	planCache.Set(ctx, cacheKey, plans)
	return plans, nil
}

// TryPreDeductFromPlan 尝试从套餐预扣费用
// 返回 (planDeducted 从套餐扣了多少, walletNeeded 还需要钱包扣多少, planID 使用的套餐ID, err)
func TryPreDeductFromPlan(ctx context.Context, tenantID int64, amount float64, requestID string, modelName string) (planDeducted, walletNeeded float64, planID int64, err error) {
	if amount <= 0 {
		return 0, amount, 0, nil
	}

	plans, err := GetActivePlansForModel(ctx, tenantID, modelName)
	if err != nil || len(plans) == 0 {
		return 0, amount, 0, nil
	}

	// FIFO: 优先扣快过期的（已按 end_at ASC 排序）
	for _, plan := range plans {
		if plan.RemainingCredits <= 0 {
			continue
		}

		// 尝试扣减：使用原子 UPDATE 确保不超扣
		if plan.RemainingCredits >= amount {
			// 全额扣套餐
			result, dbErr := g.DB().Ctx(ctx).Exec(ctx,
				"UPDATE pln_tenant_plans SET remaining_credits = remaining_credits - ?, updated_at = NOW() WHERE id = ? AND remaining_credits >= ? AND status = 'active'",
				amount, plan.ID, amount)
			if dbErr != nil {
				g.Log().Warningf(ctx, "[PLAN] pre-deduct failed for plan %d: %v", plan.ID, dbErr)
				continue
			}
			rows, _ := result.RowsAffected()
			if rows == 0 {
				continue // 并发冲突，尝试下一个套餐
			}

			storePlanDeductToRedis(ctx, requestID, amount, 0, plan.ID)
			return amount, 0, plan.ID, nil
		}

		// 部分套餐扣 + 剩余走钱包
		deduct := plan.RemainingCredits
		result, dbErr := g.DB().Ctx(ctx).Exec(ctx,
			"UPDATE pln_tenant_plans SET remaining_credits = remaining_credits - ?, updated_at = NOW() WHERE id = ? AND remaining_credits >= ? AND status = 'active'",
			deduct, plan.ID, deduct)
		if dbErr != nil {
			g.Log().Warningf(ctx, "[PLAN] partial pre-deduct failed for plan %d: %v", plan.ID, dbErr)
			continue
		}
		rows, _ := result.RowsAffected()
		if rows == 0 {
			continue
		}

		remaining := amount - deduct
		storePlanDeductToRedis(ctx, requestID, deduct, remaining, plan.ID)
		return deduct, remaining, plan.ID, nil
	}

	// 所有套餐都不够，全额走钱包
	return 0, amount, 0, nil
}

// RollbackPlanPreDeduct 回滚套餐预扣（钱包预扣失败时调用）
func RollbackPlanPreDeduct(ctx context.Context, tenantID int64, planID int64, amount float64) {
	if planID <= 0 || amount <= 0 {
		return
	}

	_, err := g.DB().Ctx(ctx).Exec(ctx,
		"UPDATE pln_tenant_plans SET remaining_credits = remaining_credits + ?, updated_at = NOW() WHERE id = ? AND status = 'active'",
		amount, planID)
	if err != nil {
		g.Log().Errorf(ctx, "[PLAN] rollback pre-deduct failed for plan %d: %v", planID, err)
	}

	clearPlanDeductFromRedis(ctx, "rollback_"+fmt.Sprintf("%d", planID))
}

// SettlePlanDeduction 在结算事务中处理套餐扣费
// 返回套餐实际扣费金额（可能小于预扣，差额退还到 remaining_credits）
func SettlePlanDeduction(ctx context.Context, tx gdb.TX, planID int64, planPreDeducted, actualCost float64) (planActual float64) {
	if planID <= 0 || planPreDeducted <= 0 {
		return 0
	}

	// 套餐实际扣费 = min(actualCost, planPreDeducted)
	planActual = actualCost
	if planActual > planPreDeducted {
		planActual = planPreDeducted
	}

	// 退还差额到 remaining_credits
	refund := planPreDeducted - planActual
	if refund > 0 {
		_, err := tx.Ctx(ctx).Exec(
			"UPDATE pln_tenant_plans SET remaining_credits = remaining_credits + ?, updated_at = NOW() WHERE id = ?",
			refund, planID)
		if err != nil {
			g.Log().Errorf(ctx, "[PLAN] settle refund to plan %d failed: %v", planID, err)
		}
	}

	return planActual
}

// GetPreDeductSplit 读取预扣时的套餐/钱包拆分
func GetPreDeductSplit(ctx context.Context, requestID string) (planAmount, walletAmount float64, planID int64) {
	predeductKey := PreDeductRedisKeyPrefix + requestID

	planVal, err := g.Redis().Do(ctx, "HGET", predeductKey, "plan_amount")
	if err == nil && !planVal.IsNil() {
		planAmount = planVal.Float64()
	}

	walletVal, err := g.Redis().Do(ctx, "HGET", predeductKey, "wallet_amount")
	if err == nil && !walletVal.IsNil() {
		walletAmount = walletVal.Float64()
	}

	planIDVal, err := g.Redis().Do(ctx, "HGET", predeductKey, "plan_id")
	if err == nil && !planIDVal.IsNil() {
		planID = planIDVal.Int64()
	}

	return
}

// storePlanDeductToRedis 将套餐扣费拆分存入 Redis prededuct hash
func storePlanDeductToRedis(ctx context.Context, requestID string, planAmount, walletAmount float64, planID int64) {
	predeductKey := PreDeductRedisKeyPrefix + requestID
	g.Redis().Do(ctx, "HSET", predeductKey, "plan_amount", planAmount)
	g.Redis().Do(ctx, "HSET", predeductKey, "wallet_amount", walletAmount)
	g.Redis().Do(ctx, "HSET", predeductKey, "plan_id", planID)
}

// clearPlanDeductFromRedis 清除套餐扣费拆分（失败回滚时）
func clearPlanDeductFromRedis(ctx context.Context, requestID string) {
	// 不删除整个 key，只清除套餐相关字段
	predeductKey := PreDeductRedisKeyPrefix + requestID
	g.Redis().Do(ctx, "HDEL", predeductKey, "plan_amount")
	g.Redis().Do(ctx, "HDEL", predeductKey, "wallet_amount")
	g.Redis().Do(ctx, "HDEL", predeductKey, "plan_id")
}

// InvalidatePlanCache 失效租户套餐缓存
func InvalidatePlanCache(ctx context.Context, tenantID int64) {
	// 删除该租户所有模型相关的套餐缓存
	planCache.DeleteByPattern(ctx, fmt.Sprintf("%d:*", tenantID))
}
