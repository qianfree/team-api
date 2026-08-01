package task

import (
	"context"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/frame/g"

	"github.com/qianfree/team-api/internal/logic/payment"
)

// ExpirePendingOrders 批量将超时未支付的 pending 订单标记为 expired。
// 事务内先释放这批订单占用的优惠码用量（否则过期订单永久占用优惠码名额），再统一置过期；
// FOR UPDATE 锁定候选行，与并发的用户取消/支付回调在行锁上串行化——被并发改走状态的行
// 在锁等待后重新求值谓词时自动排除，不会二次处理。
func ExpirePendingOrders(ctx context.Context) error {
	return g.DB().Transaction(ctx, func(ctx context.Context, tx gdb.TX) error {
		rows, err := g.DB().Ctx(ctx).GetAll(ctx, `
			SELECT id FROM ord_orders
			WHERE status = 'pending' AND expired_at IS NOT NULL AND expired_at < NOW()
			FOR UPDATE`)
		if err != nil {
			return err
		}
		if len(rows) == 0 {
			return nil
		}
		ids := make([]int64, 0, len(rows))
		for _, row := range rows {
			ids = append(ids, row["id"].Int64())
		}

		if err = payment.ReleasePromoUsageForOrders(ctx, ids); err != nil {
			return err
		}

		if _, err = g.DB().Ctx(ctx).Exec(ctx,
			"UPDATE ord_orders SET status = 'expired', updated_at = NOW() WHERE id IN (?)", ids); err != nil {
			return err
		}

		g.Log().Infof(ctx, "[CRON] expired %d pending orders", len(ids))
		return nil
	})
}
