package task

import (
	"context"

	"github.com/gogf/gf/v2/frame/g"
)

// AutoRenewScanner 自动续费扫描（已废弃，套餐改为一次性购买模式）
// 保留空实现以防编译引用
func AutoRenewScanner(ctx context.Context) {
	g.Log().Debug(ctx, "[AutoRenew] skipped: subscription model removed")
}
