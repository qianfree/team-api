package task

import (
	"context"

	"github.com/gogf/gf/v2/frame/g"

	"github.com/qianfree/team-api/internal/logic/billing"
)

// CheckPlanExpirations 批量检查并过期已到期的租户套餐
func CheckPlanExpirations(ctx context.Context) error {
	result, err := g.DB().Ctx(ctx).Exec(ctx, `
		UPDATE pln_tenant_plans
		SET status = 'expired', remaining_credits = 0, updated_at = NOW()
		WHERE status = 'active' AND end_at < NOW()
	`)
	if err != nil {
		return err
	}

	rows, _ := result.RowsAffected()
	if rows > 0 {
		g.Log().Infof(ctx, "[CRON] expired %d tenant plans", rows)

		// 获取受影响的租户列表，失效缓存
		var tenantIDs []int64
		err := g.DB().Ctx(ctx).Model("pln_tenant_plans").
			Where("status", "expired").
			Where("updated_at > NOW() - INTERVAL '1 minute'").
			Fields("DISTINCT tenant_id").
			Scan(&tenantIDs)
		if err == nil {
			for _, tid := range tenantIDs {
				billing.InvalidatePlanCache(ctx, tid)
			}
		}
	}

	return nil
}
