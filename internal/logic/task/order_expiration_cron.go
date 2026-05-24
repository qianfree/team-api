package task

import (
	"context"

	"github.com/gogf/gf/v2/frame/g"
)

// ExpirePendingOrders 批量将超时未支付的 pending 订单标记为 expired。
func ExpirePendingOrders(ctx context.Context) error {
	result, err := g.DB().Ctx(ctx).Exec(ctx, `
		UPDATE ord_orders
		SET status = 'expired', updated_at = NOW()
		WHERE status = 'pending' AND expired_at IS NOT NULL AND expired_at < NOW()
	`)
	if err != nil {
		return err
	}

	rows, _ := result.RowsAffected()
	if rows > 0 {
		g.Log().Infof(ctx, "[CRON] expired %d pending orders", rows)
	}
	return nil
}
