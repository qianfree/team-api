package billing

import (
	"context"

	"github.com/gogf/gf/v2/errors/gerror"
	"github.com/gogf/gf/v2/frame/g"
	"github.com/shopspring/decimal"

	"github.com/qianfree/team-api/internal/dao"
)

// seedTotalConsumedLua 补种 Lua：仅当 hash 存在且缺 total_consumed 字段时写入历史基线
// （HSET-if-missing 语义，幂等）。
// KEYS[1] = wallet:v2:{tenant_id}
// ARGV[1] = total_consumed_micro
const seedTotalConsumedLua = `
if redis.call("EXISTS", KEYS[1]) == 1 and redis.call("HEXISTS", KEYS[1], "total_consumed") == 0 then
    redis.call("HSET", KEYS[1], "total_consumed", ARGV[1])
    return 1
end
return 0
`

// seedTotalConsumedIfMissing 为钱包 hash 补种累计消费基线（hash 不存在或字段已存在均为 no-op）。
// 供 boot 种子与物化器自愈共用（三层补种机制的①③层）。
func seedTotalConsumedIfMissing(ctx context.Context, tenantID int64, consumedMicro int64) error {
	_, err := g.Redis().Do(ctx, "EVAL", seedTotalConsumedLua, 1,
		walletHashKey(tenantID), consumedMicro)
	if err != nil {
		return gerror.Wrapf(err, "seed total_consumed: redis")
	}
	return nil
}

// queryTotalConsumed 读 DB 物化副本中的累计消费（钱包行不存在时返回 0——物化 UPDATE
// 同样影响 0 行，无需区分）。
func queryTotalConsumed(ctx context.Context, tenantID int64) (decimal.Decimal, error) {
	var w *struct {
		TotalConsumed decimal.Decimal `json:"total_consumed"`
	}
	err := dao.BilWallets.Ctx(ctx).
		Where("tenant_id", tenantID).
		Fields("total_consumed").
		Scan(&w)
	if err != nil {
		return Zero, gerror.Wrapf(err, "query total_consumed")
	}
	if w == nil {
		return Zero, nil
	}
	return w.TotalConsumed, nil
}

// SeedWalletTotalConsumed 启动种子（三层补种之①层）：把 DB 物化副本中的累计消费历史基线
// 种进已存在的钱包 hash。
//
// 背景：total_consumed 字段上线时，存量钱包 hash（wallet:v2:*）不含该字段，结算 Lua 的
// HINCRBY 会从 0 起算，丢失迁移回填的历史值。本函数在启动时（relay 流量进入前）为所有
// total_consumed > 0 的钱包补种，幂等可重跑：
//   - hash 已含字段（已种过/已发生结算）→ no-op；
//   - hash 不存在（该租户尚无 Redis 钱包）→ 跳过，由 rebuildWalletFromDB 在首次资金操作时补种（②层）；
//   - 部署后新出现的字段缺失 hash 由物化器自愈兜底（③层）。
//
// 不 SADD 脏集合——种子值与 DB 相同，无需再物化。
// DB/Redis 故障时记 Warning 放行（Redis 故障随后由预扣/结算 fail-closed 机制接管，不阻塞启动）。
// 租户量级为千级以内时逐条 EVAL 足够（秒级完成）；量级显著增长时可改 pipeline。
func SeedWalletTotalConsumed(ctx context.Context) {
	var rows []struct {
		TenantId      int64           `json:"tenant_id"`
		TotalConsumed decimal.Decimal `json:"total_consumed"`
	}
	err := dao.BilWallets.Ctx(ctx).
		Fields("tenant_id, total_consumed").
		Where("total_consumed > 0").
		Scan(&rows)
	if err != nil {
		g.Log().Warningf(ctx, "[WALLET SEED] query wallets for total_consumed seed failed: %v", err)
		return
	}

	seeded := 0
	for _, row := range rows {
		if err := seedTotalConsumedIfMissing(ctx, row.TenantId, ToMicro(row.TotalConsumed)); err != nil {
			g.Log().Warningf(ctx, "[WALLET SEED] seed total_consumed failed: tenant=%d: %v", row.TenantId, err)
			continue
		}
		seeded++
	}
	if len(rows) > 0 {
		g.Log().Infof(ctx, "[WALLET SEED] total_consumed baseline seeded: %d/%d wallets", seeded, len(rows))
	}
}
