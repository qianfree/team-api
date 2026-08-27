package billing

import (
	"context"
	"fmt"
	"time"

	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/util/gconv"

	lcommon "github.com/qianfree/team-api/internal/logic/common"
)

const (
	// defaultWalletMaterializeInterval 钱包物化默认间隔：把 Redis 权威状态刷回 DB 的周期。
	// bil_wallets 是纯物化视图（展示/报表/灾难恢复源），数秒滞后不影响任何实时判断。
	defaultWalletMaterializeInterval = 5 * time.Second
	// walletMaterializeBatchSize 每 tick 最多物化的租户数（防爆量，超出部分下 tick 继续）
	walletMaterializeBatchSize = 200
	// predeductSweepFenceRetries 预扣清扫 fenced 写回的有界重试次数
	predeductSweepFenceRetries = 3
)

// walletMaterializeInterval 物化间隔（配置项 billing_wallet_materialize_interval_ms，默认 5000ms）
func walletMaterializeInterval(ctx context.Context) time.Duration {
	ms := lcommon.Config().GetInt(ctx, "billing_wallet_materialize_interval_ms")
	if ms <= 0 {
		return defaultWalletMaterializeInterval
	}
	return time.Duration(ms) * time.Millisecond
}

// StartWalletMaterializer 启动钱包物化器（boot 时调用一次）。
// 把 Redis 权威钱包状态周期全量覆盖到 DB bil_wallets（物化视图）。
// 不走 cron（gcron 5 字段最小粒度 1 分钟，无法满足秒级间隔），boot goroutine + ticker 驱动。
//
// 并发与崩溃安全：
//   - peek（SRANDMEMBER）→ 物化 → SREM：崩溃时成员残留，下 tick 幂等重写；
//   - 多实例并发物化同一租户是同值覆盖，无冲突；
//   - 覆盖期间产生的新变动会重新 SADD 脏标记，下个 tick 收敛（最终一致）。
func StartWalletMaterializer(ctx context.Context) {
	interval := walletMaterializeInterval(ctx)
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		g.Log().Infof(ctx, "[WALLET MATERIALIZER] started, interval=%s", interval)
		for {
			select {
			case <-ctx.Done():
				g.Log().Infof(ctx, "[WALLET MATERIALIZER] stopped")
				return
			case <-ticker.C:
				materializeDirtyWallets(ctx)
			}
		}
	}()
}

// materializeDirtyWallets 单轮物化：取脏租户样本 → 读 Redis 权威值 → 覆盖 DB → 清脏标记
func materializeDirtyWallets(ctx context.Context) {
	members, err := g.Redis().Do(ctx, "SRANDMEMBER", walletDirtyTenantsKey, walletMaterializeBatchSize)
	if err != nil {
		g.Log().Warningf(ctx, "[WALLET MATERIALIZER] read dirty tenants failed: %v", err)
		return
	}
	if members.IsNil() || members.IsEmpty() {
		return
	}

	for _, m := range members.Strings() {
		tenantID := gconv.Int64(m)
		if tenantID <= 0 {
			g.Redis().Do(ctx, "SREM", walletDirtyTenantsKey, m)
			continue
		}
		materializeOneWallet(ctx, tenantID)
	}
}

// materializeOneWallet 物化单个租户钱包；成功或 hash 缺失时清除脏标记
func materializeOneWallet(ctx context.Context, tenantID int64) {
	walletRedisKey := walletHashKey(tenantID)
	balanceMicro, frozenMicro, exists, err := readWalletHash(ctx, tenantID)
	if err != nil {
		// Redis 故障：保留脏标记，下 tick 重试
		g.Log().Warningf(ctx, "[WALLET MATERIALIZER] read wallet hash failed: tenant=%d: %v", tenantID, err)
		return
	}
	if !exists {
		// hash 不存在（灾难丢失后尚无资金操作触发重建）：无需物化，清脏标记
		g.Redis().Do(ctx, "SREM", walletDirtyTenantsKey, tenantID)
		return
	}

	// decimal 直传 NUMERIC（driver.Valuer 精确字符串），balance/frozen 全量覆盖
	_, err = g.DB().Ctx(ctx).Exec(ctx,
		"UPDATE bil_wallets SET balance = ?, frozen_balance = ?, updated_at = NOW() WHERE tenant_id = ?",
		FromMicro(balanceMicro), FromMicro(frozenMicro), tenantID)
	if err != nil {
		// DB 故障：保留脏标记，下 tick 重试
		g.Log().Warningf(ctx, "[WALLET MATERIALIZER] update db wallet failed: tenant=%d: %v", tenantID, err)
		return
	}

	// 旧版钱包 hash 带 600s TTL（缓存时代残留）：顺手摘除，权威 hash 不得过期重建
	g.Redis().Do(ctx, "PERSIST", walletRedisKey)
	g.Redis().Do(ctx, "SREM", walletDirtyTenantsKey, tenantID)
}

// PredeductSweep 预扣清扫（cron 每 2 分钟，替代原 DB tracks 孤儿清理）。
// 重算式自愈：逐租户重算 frozen = Σ(幸存预扣 hash amount)， fenced 写回钱包 hash——
// 覆盖一切 frozen 漂移来源：预扣 hash TTL 过期未释放、进程崩溃残留、异常路径遗漏。
// 相比 DB 版状态翻转清理，重算对「金额不可考」的丢失场景也能收敛到正确值。
func PredeductSweep(ctx context.Context) {
	cursor := 0
	for {
		res, err := g.Redis().Do(ctx, "SCAN", cursor, "MATCH", "prededuct_active:*", "COUNT", 100)
		if err != nil {
			g.Log().Warningf(ctx, "[PREDEDUCT SWEEP] scan active sets failed: %v", err)
			return
		}
		arr := res.Array()
		if len(arr) != 2 {
			return
		}
		cursor = gconv.Int(arr[0])
		for _, key := range gconv.Strings(arr[1]) {
			sweepOneTenantPrededucts(ctx, key)
		}
		if cursor == 0 {
			return
		}
	}
}

// sweepOneTenantPrededucts 重算单个租户的 frozen 并 fenced 写回，顺带清理活跃集合死亡成员
func sweepOneTenantPrededucts(ctx context.Context, activeSetKey string) {
	var tenantID int64
	if _, err := fmt.Sscanf(activeSetKey, "prededuct_active:%d", &tenantID); err != nil || tenantID <= 0 {
		return
	}

	for attempt := 0; attempt < predeductSweepFenceRetries; attempt++ {
		// 1. 读钱包版本号（hash 不存在：灾后未重建，跳过——由重建流程负责）
		walletKey := walletHashKey(tenantID)
		verRes, err := g.Redis().Do(ctx, "HGET", walletKey, "ver")
		if err != nil {
			g.Log().Warningf(ctx, "[PREDEDUCT SWEEP] read ver failed: tenant=%d: %v", tenantID, err)
			return
		}
		if verRes.IsNil() {
			return
		}
		ver := verRes.Int64()

		// 2. 重算 frozen = Σ(幸存预扣 hash amount)，死亡成员顺手 SREM
		members, err := g.Redis().Do(ctx, "SMEMBERS", activeSetKey)
		if err != nil {
			g.Log().Warningf(ctx, "[PREDEDUCT SWEEP] read active set failed: tenant=%d: %v", tenantID, err)
			return
		}
		var frozenMicro int64
		var staleIDs []any
		for _, rid := range members.Strings() {
			amt, err := g.Redis().Do(ctx, "HGET", PreDeductRedisKeyPrefix+rid, "amount")
			if err != nil || amt.IsNil() {
				staleIDs = append(staleIDs, rid)
				continue
			}
			frozenMicro += amt.Int64()
		}

		// 3. fenced 写回：读取版本号期间有任何资金变动（ver 变化）则放弃，下轮重算
		fencedLua := `
local ver = tonumber(redis.call("HGET", KEYS[1], "ver") or "0")
if ver ~= tonumber(ARGV[1]) then
    return 0
end
redis.call("HSET", KEYS[1], "frozen_balance", ARGV[2])
redis.call("SADD", "wallet_dirty_tenants", ARGV[3])
return 1
`
		ret, err := g.Redis().Do(ctx, "EVAL", fencedLua, 1, walletKey, ver, frozenMicro, tenantID)
		if err != nil {
			g.Log().Warningf(ctx, "[PREDEDUCT SWEEP] fenced write failed: tenant=%d: %v", tenantID, err)
			return
		}
		if ret.Int64() == 0 {
			continue // 有并发变动，重算后重试
		}

		if len(staleIDs) > 0 {
			args := append([]any{activeSetKey}, staleIDs...)
			g.Redis().Do(ctx, "SREM", args...)
		}
		// 心跳不打日志（每 2 分钟一条太吵），异常路径（围栏重试耗尽/Redis 故障）仍有 Warningf
		return
	}
	g.Log().Warningf(ctx, "[PREDEDUCT SWEEP] tenant=%d frozen recompute aborted after %d fenced retries (hot wallet)",
		tenantID, predeductSweepFenceRetries)
}
