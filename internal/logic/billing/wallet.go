package billing

import (
	"context"
	"fmt"
	"strconv"
	"sync"
	"time"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/errors/gerror"
	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/util/gconv"
	"github.com/shopspring/decimal"
	"golang.org/x/sync/singleflight"

	"github.com/qianfree/team-api/internal/dao"
	lcommon "github.com/qianfree/team-api/internal/logic/common"
	do "github.com/qianfree/team-api/internal/model/do"
)

const (
	// PreDeductRedisKeyPrefix 预扣 Redis key 前缀。
	// v2（Phase 3）：预扣 amount 改为整数微单位(micro-USD)存储，与旧版 float 值不兼容，
	// 故 bump 版本号；旧 key 自然按 TTL 过期，杜绝新代码把旧 float 值误读成 micro。
	PreDeductRedisKeyPrefix = "prededuct:v2:"
	// PreDeductMaxAge 预扣记录最大存活时间（秒），防止异常未结算的预扣占用余额
	PreDeductMaxAge = 7200 // 2 小时（长流式/realtime 会话防误杀：孤儿清理与 Redis TTL 共用此阈值）

	// preDeductDBTimeout 预扣资金临界区（冻结落库 + 追踪写入事务）的兜底超时。
	// 临界区运行在 context.WithoutCancel 的独立 ctx 上——客户端断开不得中途取消资金写入——
	// 因此必须自带超时，防止 DB 异常时长期占用连接。
	preDeductDBTimeout = 10 * time.Second
)

// walletHashKey 钱包 Redis hash key。
// v2（Phase 3）：balance / frozen_balance 以整数 micro-USD 存储；旧版 float key 随 TTL 过期。
func walletHashKey(tenantID int64) string {
	return fmt.Sprintf("wallet:v2:%d", tenantID)
}

// walletCache 钱包缓存（TTL 300s）
var walletCache = lcommon.NewCache("wallet", 300*time.Second)

// walletSyncGroup 合并同一租户的并发 syncWalletToRedis DB 读取
var walletSyncGroup singleflight.Group

// WalletInfo 钱包信息
type WalletInfo struct {
	ID                 int64
	TenantID           int64
	Balance            float64
	FrozenBalance      float64
	WarningThreshold   float64
	Currency           string
	LowBalanceNotified bool
}

// GetWallet 获取租户钱包
func GetWallet(ctx context.Context, tenantID int64) (*WalletInfo, error) {
	cacheKey := fmt.Sprintf("%d", tenantID)
	var cached WalletInfo
	if walletCache.GetJSON(ctx, cacheKey, &cached) {
		return &cached, nil
	}

	type walletRow struct {
		ID                 int64   `json:"id"`
		TenantId           int64   `json:"tenant_id"`
		Balance            float64 `json:"balance"`
		FrozenBalance      float64 `json:"frozen_balance"`
		WarningThreshold   float64 `json:"warning_threshold"`
		Currency           string  `json:"currency"`
		LowBalanceNotified bool    `json:"low_balance_notified"`
	}

	var w *walletRow
	err := dao.BilWallets.Ctx(ctx).
		Where("tenant_id", tenantID).
		Fields("id, tenant_id, balance, frozen_balance, warning_threshold, currency, low_balance_notified").
		Scan(&w)
	if err != nil {
		return nil, gerror.Wrapf(err, "query wallet")
	}
	if w == nil {
		return nil, gerror.New("wallet not found")
	}

	info := &WalletInfo{
		ID:                 w.ID,
		TenantID:           w.TenantId,
		Balance:            w.Balance,
		FrozenBalance:      w.FrozenBalance,
		WarningThreshold:   w.WarningThreshold,
		Currency:           w.Currency,
		LowBalanceNotified: w.LowBalanceNotified,
	}

	walletCache.Set(ctx, cacheKey, info)
	return info, nil
}

// walletIDCache 租户ID→钱包ID 进程内映射缓存。
// 钱包一经创建（EnsureWallet）ID 即不变，可长期缓存且无需失效。
var walletIDCache sync.Map // tenantID(int64) → walletID(int64)

// GetWalletID 获取租户钱包 ID（进程内缓存，键值不变、永不失效）。
// 结算热路径只需要 walletID：不应因 walletCache 被每次结算失效而反复回源查 bil_wallets——
// 高并发下结算事务在钱包热点行上串行化、连接池吃紧时，这次"无谓的读"会排队到超时，
// 把本已成功的请求拖成结算失败（实测报错 settle_with_usage: get wallet: context deadline exceeded）。
func GetWalletID(ctx context.Context, tenantID int64) (int64, error) {
	if v, ok := walletIDCache.Load(tenantID); ok {
		return v.(int64), nil
	}
	w, err := GetWallet(ctx, tenantID)
	if err != nil {
		return 0, err
	}
	walletIDCache.Store(tenantID, w.ID)
	return w.ID, nil
}

// EnsureWallet 确保租户有钱包，没有则创建
// 使用 INSERT ... ON CONFLICT 保证并发安全
func EnsureWallet(ctx context.Context, tenantID int64) error {
	_, err := g.DB().Ctx(ctx).Exec(ctx,
		`INSERT INTO bil_wallets (tenant_id, balance, frozen_balance, warning_threshold, currency)
		 VALUES ($1, 0, 0, 0, 'USD')
		 ON CONFLICT (tenant_id) DO NOTHING`,
		tenantID)
	if err != nil {
		return gerror.Wrapf(err, "ensure wallet")
	}
	return nil
}

// AvailableBalance 获取可用余额（balance - frozen_balance）
func AvailableBalance(wallet *WalletInfo) float64 {
	return wallet.Balance - wallet.FrozenBalance
}

// PreDeduct 预扣费用（Redis Lua 原子操作）
// 冻结指定金额，返回预扣记录 ID 用于后续结算
func PreDeduct(ctx context.Context, tenantID int64, amount float64, requestID string, modelName string) (bool, error) {
	if amount <= 0 {
		return true, nil
	}

	// 先将钱包数据同步到 Redis（确保 balance 字段存在）
	if err := syncWalletToRedis(ctx, tenantID); err != nil {
		// Redis 不可用，降级到 DB 直接扣减
		return preDeductDB(ctx, tenantID, amount, requestID)
	}

	// Redis Lua 脚本：原子检查+冻结（v2：金额全部为整数 micro-USD，整数运算无浮点漂移）
	// KEYS[1] = wallet:v2:{tenant_id}  (hash: balance, frozen_balance —— 均为整数 micro)
	// KEYS[2] = prededuct:v2:{request_id}
	// ARGV[1] = amount_micro（整数微单位）
	// ARGV[2] = request_id
	// ARGV[3] = ttl (PreDeductMaxAge)
	// ARGV[4] = tenant_id
	// ARGV[5] = model_name
	// ARGV[6] = created_at (unix timestamp)
	// 返回值：1 冻结成功 / 0 可用余额不足 / 2 已预扣（幂等） / 3 钱包 hash 缺失需重建
	luaScript := `
local wallet_key = KEYS[1]
local prededuct_key = KEYS[2]
local amount = tonumber(ARGV[1])
local request_id = ARGV[2]
local ttl = tonumber(ARGV[3])

-- 检查是否已预扣（幂等）
local exists = redis.call("EXISTS", prededuct_key)
if exists == 1 then
    return 2
end

-- 钱包 hash 缺失/缺 balance 字段（sync 与本脚本之间被结算/解冻 DEL）：
-- 不得把缺失字段当 0 参与计算，返回 3 由调用方重新同步后重试
if redis.call("HEXISTS", wallet_key, "balance") == 0 then
    return 3
end

-- 获取钱包信息（整数 micro）
local balance = tonumber(redis.call("HGET", wallet_key, "balance") or "0")
local frozen = tonumber(redis.call("HGET", wallet_key, "frozen_balance") or "0")

-- 检查可用余额
local available = balance - frozen
if available < amount then
    return 0
end

-- 冻结金额（整数自增，无浮点漂移）
redis.call("HINCRBY", wallet_key, "frozen_balance", amount)
redis.call("HSET", prededuct_key, "amount", amount)
redis.call("HSET", prededuct_key, "tenant_id", ARGV[4])
redis.call("HSET", prededuct_key, "model_name", ARGV[5])
redis.call("HSET", prededuct_key, "created_at", ARGV[6])
local active_set = "prededuct_active:" .. ARGV[4]
redis.call("SADD", active_set, request_id)
redis.call("EXPIRE", active_set, 30 * 86400)
redis.call("EXPIRE", prededuct_key, ttl)
return 1
`

	walletRedisKey := walletHashKey(tenantID)
	predeductRedisKey := fmt.Sprintf("%s%s", PreDeductRedisKeyPrefix, requestID)

	amountMicro := ToMicro(NewFromFloat(amount))
	result, err := g.Redis().Do(ctx, "EVAL", luaScript, 2,
		walletRedisKey, predeductRedisKey,
		amountMicro, requestID, PreDeductMaxAge, tenantID, modelName, time.Now().Unix())
	if err != nil {
		// Redis 不可用，降级到 DB 直接扣减
		return preDeductDB(ctx, tenantID, amount, requestID)
	}

	code := result.Int64()

	if code == 3 {
		// 钱包 hash 在 sync 与 EVAL 之间被结算/解冻 DEL：重建后重试一次
		if err := syncWalletToRedis(ctx, tenantID); err != nil {
			return preDeductDB(ctx, tenantID, amount, requestID)
		}
		result, err = g.Redis().Do(ctx, "EVAL", luaScript, 2,
			walletRedisKey, predeductRedisKey,
			amountMicro, requestID, PreDeductMaxAge, tenantID, modelName, time.Now().Unix())
		if err != nil {
			return preDeductDB(ctx, tenantID, amount, requestID)
		}
		code = result.Int64()
		if code == 3 {
			// 极端竞态下仍缺失：降级到 DB 单语句原子预扣
			return preDeductDB(ctx, tenantID, amount, requestID)
		}
	}

	if code == 0 {
		return false, gerror.New("insufficient balance")
	}
	if code == 2 {
		return true, nil
	}

	// 同步到 DB（确保 DB frozen_balance 在返回前已更新）。任一步失败都不能把
	// 仅存在于 Redis 的冻结状态暴露给后续请求。
	// 冻结 UPDATE 与追踪 INSERT 必须在同一事务内原子完成，并脱离请求 ctx 的取消
	//（WithoutCancel + 独立超时）：客户端在两步之间断开会把资金状态撕裂——冻结已加、
	// 追踪未写，且回滚补偿若复用已取消的 ctx 也必然失败。追踪缺失的冻结不在
	// bil_prededuct_tracks 中，孤儿清理与对账永远发现不了，frozen_balance 将永久泄漏。
	dbCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), preDeductDBTimeout)
	defer cancel()
	err = g.DB().Transaction(dbCtx, func(txCtx context.Context, tx gdb.TX) error {
		if err := preDeductSyncDB(txCtx, tenantID, amount); err != nil {
			return err
		}
		return trackPreDeduct(txCtx, tenantID, amount, requestID, modelName)
	})
	if err != nil {
		// 事务已整体回滚（DB 无残留），仅需撤销 Redis 侧冻结
		rollbackRedisPreDeduct(ctx, tenantID, requestID)
		return false, err
	}

	return true, nil
}

// preDeductDB DB 降级预扣（Redis 不可用时）
func preDeductDB(ctx context.Context, tenantID int64, amount float64, requestID string) (bool, error) {
	// 请求 ctx 已取消（客户端断开/超时）时直接终止：Redis 操作报 context canceled 会被
	// 上层误判为 "Redis 不可用" 而降级到此处，此时预扣既无意义（请求即将失败），
	// 又会在资金临界区中途被取消造成状态撕裂，还平白放大 DB 热点行压力。
	if err := ctx.Err(); err != nil {
		return false, gerror.Wrapf(err, "pre-deduct aborted: request context canceled")
	}

	// A9 修复：用单条原子条件更新替代「先 SELECT ... FOR UPDATE，再独立 UPDATE」。
	// 原实现两条语句在 autocommit 下各自成一个事务，FOR UPDATE 的行锁在 SELECT 语句提交后即释放，
	// 两个并发降级预扣可都通过 available 检查、再各自 frozen += amount → 超额冻结（可用余额被冻成负）。
	// 单条 "WHERE tenant_id=? AND balance - frozen_balance >= amount" 的 UPDATE 在语句执行期间持有
	// 行锁并原子重算谓词：RowsAffected==1 表示冻结成功，==0 表示可用余额不足（或钱包不存在）。
	//
	// 冻结 UPDATE 与追踪 INSERT 在同一事务内原子完成（任一失败整体回滚，杜绝「冻结已加、
	// 追踪缺失」的泄漏窗口），并脱离请求 ctx 的取消（WithoutCancel + 独立超时）：
	// 追踪缺失的冻结对孤儿清理与对账不可见，frozen_balance 会永久泄漏。
	dbCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), preDeductDBTimeout)
	defer cancel()
	err := g.DB().Transaction(dbCtx, func(txCtx context.Context, tx gdb.TX) error {
		res, err := g.DB().Ctx(txCtx).Exec(txCtx,
			"UPDATE bil_wallets SET frozen_balance = frozen_balance + ?, updated_at = ? WHERE tenant_id = ? AND balance - frozen_balance >= ?",
			amount, time.Now(), tenantID, amount)
		if err != nil {
			return gerror.Wrapf(err, "pre-deduct db update")
		}
		affected, err := res.RowsAffected()
		if err != nil {
			return gerror.Wrapf(err, "pre-deduct db update result")
		}
		if affected == 0 {
			return gerror.New("insufficient balance")
		}
		// 追踪记录是后续精确解冻的金额来源，与冻结同事务落库
		return trackPreDeduct(txCtx, tenantID, amount, requestID, "")
	})
	if err != nil {
		return false, err
	}

	// 清除缓存（事务提交后）
	walletCache.Delete(ctx, fmt.Sprintf("%d", tenantID))
	return true, nil
}

// preDeductSyncDB sync pre-deduct to DB（同步调用，确保 DB frozen_balance 在返回前已更新）。
// 通过 g.DB().Ctx(ctx) 传播 ctx 携带的事务：在 PreDeduct 的资金临界区事务内调用时与追踪写入同事务。
func preDeductSyncDB(ctx context.Context, tenantID int64, amount float64) error {
	result, err := g.DB().Ctx(ctx).Exec(ctx,
		"UPDATE bil_wallets SET frozen_balance = frozen_balance + ?, updated_at = ? WHERE tenant_id = ?",
		amount, time.Now(), tenantID)
	if err != nil {
		return gerror.Wrapf(err, "pre-deduct sync db")
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return gerror.Wrapf(err, "pre-deduct sync db result")
	}
	if affected == 0 {
		return gerror.Newf("pre-deduct sync db: wallet not found for tenant %d", tenantID)
	}
	return nil
}

// UnfreezePreDeduct 解冻预扣金额（请求失败时调用）
func UnfreezePreDeduct(ctx context.Context, tenantID int64, requestID string) error {
	// 调用方不再提供金额。事务内锁定 frozen 追踪行并读取预扣时保存的金额，
	// 既避免错误金额解冻，也让重复调用只有一个事务能实际释放冻结。
	err := g.DB().Transaction(ctx, func(ctx context.Context, tx gdb.TX) error {
		var track *struct {
			Amount decimal.Decimal `json:"amount"`
		}
		if err := dao.BilPredeductTracks.Ctx(ctx).
			Where("tenant_id", tenantID).
			Where("request_id", requestID).
			Where("status", "frozen").
			Fields("amount").
			LockUpdate().
			Scan(&track); err != nil {
			return gerror.Wrapf(err, "load prededuct for unfreeze")
		}
		if track == nil || track.Amount.LessThanOrEqual(decimal.Zero) {
			return nil
		}

		if _, err := g.DB().Ctx(ctx).Exec(ctx,
			"UPDATE bil_wallets SET frozen_balance = GREATEST(frozen_balance - ?, 0), updated_at = ? WHERE tenant_id = ?",
			track.Amount, time.Now(), tenantID); err != nil {
			return gerror.Wrapf(err, "unfreeze wallet")
		}
		if _, err := dao.BilPredeductTracks.Ctx(ctx).
			Where("tenant_id", tenantID).
			Where("request_id", requestID).
			Where("status", "frozen").
			Data(do.BilPredeductTracks{Status: "released"}).
			Update(); err != nil {
			return gerror.Wrapf(err, "mark prededuct released")
		}
		return nil
	})
	if err != nil {
		InvalidateWalletRedis(ctx, tenantID)
		return err
	}

	walletCache.Delete(ctx, fmt.Sprintf("%d", tenantID))
	InvalidateWalletRedis(ctx, tenantID)
	CleanupPreDeduct(ctx, tenantID, requestID)
	return nil
}

// GetPreDeductAmount 获取预扣金额
func GetPreDeductAmount(ctx context.Context, requestID string) (float64, bool) {
	predeductRedisKey := fmt.Sprintf("%s%s", PreDeductRedisKeyPrefix, requestID)
	result, err := g.Redis().Do(ctx, "HGET", predeductRedisKey, "amount")
	if err == nil && !result.IsNil() {
		// v2：amount 以整数 micro 存储，换算回 USD
		return InexactFloat64(FromMicro(result.Int64())), true
	}
	return 0, false
}

// syncWalletToRedis 将钱包余额从 DB 同步到 Redis Hash
// 使用 singleflight 合并同一租户的并发请求，避免 N 个并发预扣打出 N 次相同的 DB 读
func syncWalletToRedis(ctx context.Context, tenantID int64) error {
	_, err, _ := walletSyncGroup.Do(strconv.FormatInt(tenantID, 10), func() (interface{}, error) {
		return nil, doSyncWalletToRedis(context.Background(), tenantID)
	})
	return err
}

// doSyncWalletToRedis 将钱包余额从 DB 同步到 Redis Hash
// 每次预扣前调用，确保 Redis 中的 balance 与 DB 一致
// frozen_balance 由预扣 Lua 脚本管理，仅在 key 首次创建时从 DB 初始化
func doSyncWalletToRedis(ctx context.Context, tenantID int64) error {
	walletRedisKey := walletHashKey(tenantID)

	// 从 DB 读取钱包数据（跳过内存缓存，直接查库确保最新）
	type walletRow struct {
		Balance       decimal.Decimal `json:"balance"`
		FrozenBalance decimal.Decimal `json:"frozen_balance"`
	}
	var w *walletRow
	err := dao.BilWallets.Ctx(ctx).
		Where("tenant_id", tenantID).
		Fields("balance, frozen_balance").
		Scan(&w)
	if err != nil {
		return gerror.Wrapf(err, "sync wallet to redis")
	}
	if w == nil {
		return gerror.New("wallet not found")
	}

	// 「检查是否存在 + 写入」必须原子完成：拆成 EXISTS 后再 HSET 两步时，结算/解冻的
	// DEL 可插入两步之间，重建出只有 balance、缺 frozen_balance 字段的 hash（预扣 Lua
	// 会把缺失的 frozen 当 0，导致按零冻结计算可用余额而超额放行）。
	// 返回 1 = 本次新建（balance + frozen_balance 完整初始化），0 = 已存在仅刷新 balance。
	syncLua := `
local key = KEYS[1]
if redis.call("EXISTS", key) == 1 then
    redis.call("HSET", key, "balance", ARGV[1])
    redis.call("EXPIRE", key, tonumber(ARGV[3]))
    return 0
end
redis.call("HSET", key, "balance", ARGV[1], "frozen_balance", ARGV[2])
redis.call("EXPIRE", key, tonumber(ARGV[3]))
return 1
`
	// 过期时间 600s，过期后下次预扣会重新初始化
	ret, err := g.Redis().Do(ctx, "EVAL", syncLua, 1, walletRedisKey,
		ToMicro(w.Balance), ToMicro(w.FrozenBalance), 600)
	if err != nil {
		return gerror.Wrapf(err, "sync wallet to redis")
	}
	if ret.Int64() == 1 {
		// 本次新建：从 DB 恢复活跃预扣明细到 Redis
		rebuildPredeductFromDB(ctx, tenantID)
	}

	return nil
}

// InvalidateWalletRedis 清除 Redis 中的钱包缓存（余额变更后调用）
func InvalidateWalletRedis(ctx context.Context, tenantID int64) {
	g.Redis().Do(ctx, "DEL", walletHashKey(tenantID))
}

func rollbackRedisPreDeduct(ctx context.Context, tenantID int64, requestID string) {
	// DB 同步失败后 DB 仍是权威状态。删除钱包 hash 强制下次从 DB 重建，
	// 同时清理本次尚未持久化成功的 Redis 预扣明细。
	rollbackCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
	defer cancel()
	InvalidateWalletRedis(rollbackCtx, tenantID)
	CleanupPreDeduct(rollbackCtx, tenantID, requestID)
}

// InvalidateWallet 清除租户钱包的两级缓存（进程内 walletCache + Redis hash）。
// 供 billing 包外（如充值履约 payment.creditWalletTx）在钱包余额变更后调用：
// walletCache 是 billing 包私有变量，跨包无法直接 Delete，仅清 Redis 会导致
// GetWallet 在 300s TTL 内继续命中进程内旧余额。余额变更后应统一调用本函数。
func InvalidateWallet(ctx context.Context, tenantID int64) {
	walletCache.Delete(ctx, fmt.Sprintf("%d", tenantID))
	InvalidateWalletRedis(ctx, tenantID)
}

// CleanupPreDeduct 清理预扣记录（结算成功后调用）
func CleanupPreDeduct(ctx context.Context, tenantID int64, requestID string) {
	predeductRedisKey := fmt.Sprintf("%s%s", PreDeductRedisKeyPrefix, requestID)
	activeSetKey := fmt.Sprintf("prededuct_active:%d", tenantID)
	g.Redis().Do(ctx, "DEL", predeductRedisKey)
	g.Redis().Do(ctx, "SREM", activeSetKey, requestID)
}

// FrozenItem 单个冻结项
type FrozenItem struct {
	RequestID string  `json:"request_id"`
	ModelName string  `json:"model_name"`
	Amount    float64 `json:"amount"`
	CreatedAt int64   `json:"created_at"`
	Remaining int64   `json:"remaining"`
}

// GetFrozenItems 获取租户当前所有冻结项
func GetFrozenItems(ctx context.Context, tenantID int64) ([]FrozenItem, error) {
	activeSetKey := fmt.Sprintf("prededuct_active:%d", tenantID)

	members, err := g.Redis().Do(ctx, "SMEMBERS", activeSetKey)
	if err != nil || members.IsNil() {
		return []FrozenItem{}, nil
	}

	requestIDs := members.Strings()
	var items []FrozenItem
	var staleIDs []string

	for _, reqID := range requestIDs {
		predeductKey := fmt.Sprintf("%s%s", PreDeductRedisKeyPrefix, reqID)

		exists, _ := g.Redis().Do(ctx, "EXISTS", predeductKey)
		if exists.Int64() == 0 {
			staleIDs = append(staleIDs, reqID)
			continue
		}

		data, err := g.Redis().Do(ctx, "HGETALL", predeductKey)
		if err != nil || data.IsNil() {
			staleIDs = append(staleIDs, reqID)
			continue
		}

		m := data.Map()

		ttl, _ := g.Redis().Do(ctx, "TTL", predeductKey)
		remaining := ttl.Int64()
		if remaining < 0 {
			remaining = 0
		}

		var amount float64
		if v, ok := m["amount"]; ok {
			// v2：amount 以整数 micro 存储，换算回 USD
			amount = InexactFloat64(FromMicro(gconv.Int64(v)))
		}

		var modelName string
		if v, ok := m["model_name"]; ok {
			modelName = gconv.String(v)
		}

		var createdAt int64
		if v, ok := m["created_at"]; ok {
			createdAt = gconv.Int64(v)
		}

		items = append(items, FrozenItem{
			RequestID: reqID,
			ModelName: modelName,
			Amount:    amount,
			CreatedAt: createdAt,
			Remaining: remaining,
		})
	}

	// 清理过期条目（TTL 已过但仍在 set 中的残留）
	if len(staleIDs) > 0 {
		args := make([]any, 0, len(staleIDs)+1)
		args = append(args, activeSetKey)
		for _, id := range staleIDs {
			args = append(args, id)
		}
		g.Redis().Do(ctx, "SREM", args...)
	}

	return items, nil
}

// trackPreDeduct 写入预扣追踪记录到 DB（用于精确解冻、孤儿清理和 Redis 重建）。
func trackPreDeduct(ctx context.Context, tenantID int64, amount float64, requestID string, modelName string) error {
	_, err := g.DB().Ctx(ctx).Exec(ctx,
		`INSERT INTO bil_prededuct_tracks (tenant_id, request_id, amount, model_name, status)
		 VALUES ($1, $2, $3, $4, 'frozen')
		 ON CONFLICT (request_id) DO NOTHING`,
		tenantID, requestID, amount, modelName)
	if err != nil {
		return gerror.Wrapf(err, "track prededuct: request=%s", requestID)
	}
	return nil
}

// rebuildPredeductFromDB 从 DB 恢复活跃预扣明细到 Redis（Redis 重启后调用）
func rebuildPredeductFromDB(ctx context.Context, tenantID int64) {
	type trackRow struct {
		RequestID string          `json:"request_id"`
		Amount    decimal.Decimal `json:"amount"`
		ModelName string          `json:"model_name"`
		CreatedAt int64           `json:"created_at"`
	}

	cutoff := time.Now().Add(-time.Duration(PreDeductMaxAge) * time.Second)
	var tracks []trackRow
	err := dao.BilPredeductTracks.Ctx(ctx).
		Where("tenant_id", tenantID).
		Where("status", "frozen").
		Where("created_at > ?", cutoff).
		Fields("request_id, amount, model_name, EXTRACT(EPOCH FROM created_at)::bigint as created_at").
		Scan(&tracks)
	if err != nil || len(tracks) == 0 {
		return
	}

	activeSetKey := fmt.Sprintf("prededuct_active:%d", tenantID)
	for _, t := range tracks {
		age := time.Now().Unix() - t.CreatedAt
		remainingTTL := int64(PreDeductMaxAge) - age
		if remainingTTL <= 0 {
			continue
		}

		predeductKey := fmt.Sprintf("%s%s", PreDeductRedisKeyPrefix, t.RequestID)
		g.Redis().Do(ctx, "HMSET", predeductKey,
			"amount", ToMicro(t.Amount),
			"tenant_id", tenantID,
			"model_name", t.ModelName,
			"created_at", t.CreatedAt,
		)
		g.Redis().Do(ctx, "EXPIRE", predeductKey, remainingTTL)
		g.Redis().Do(ctx, "SADD", activeSetKey, t.RequestID)
	}
	// 确保 active SET 有 TTL（30 天），过期后下次预扣时自动重建
	g.Redis().Do(ctx, "EXPIRE", activeSetKey, 30*86400)
}
