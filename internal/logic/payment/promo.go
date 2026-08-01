package payment

import (
	"context"

	"github.com/gogf/gf/v2/frame/g"

	"github.com/qianfree/team-api/internal/dao"
)

// ReleasePromoUsageForOrders 释放订单占用的优惠码用量：按订单聚合回退 used_count，
// 并删除对应使用记录。供订单取消（tenant.OrderCancel）与过期任务（task.ExpirePendingOrders）
// 复用，须在事务 ctx 内调用，与订单状态流转同事务提交，保证「订单终止」与「名额归还」原子。
// GREATEST(...,0) 兜底防负数（历史数据或重复释放时不至于把计数打穿）。
func ReleasePromoUsageForOrders(ctx context.Context, orderIDs []int64) error {
	if len(orderIDs) == 0 {
		return nil
	}
	_, err := g.DB().Ctx(ctx).Exec(ctx, `
		UPDATE ord_promo_codes p
		SET used_count = GREATEST(p.used_count - u.cnt, 0), updated_at = NOW()
		FROM (
			SELECT promo_code_id, COUNT(*) AS cnt
			FROM ord_promo_code_usages
			WHERE order_id IN (?)
			GROUP BY promo_code_id
		) u
		WHERE p.id = u.promo_code_id`, orderIDs)
	if err != nil {
		return err
	}
	_, err = dao.OrdPromoCodeUsages.Ctx(ctx).WhereIn("order_id", orderIDs).Delete()
	return err
}
