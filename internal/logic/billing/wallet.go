package billing

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/gogf/gf/v2/errors/gerror"
	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/util/gconv"
	"golang.org/x/sync/singleflight"

	"github.com/qianfree/team-api/internal/dao"
	lcommon "github.com/qianfree/team-api/internal/logic/common"
)

const (
	// PreDeductRedisKeyPrefix 预扣 Redis key 前缀。
	// v2（Phase 3）：预扣 amount 改为整数微单位(micro-USD)存储，与旧版 float 值不兼容，
	// 故 bump 版本号；旧 key 自然按 TTL 过期，杜绝新代码把旧 float 值误读成 micro。
	PreDeductRedisKeyPrefix = "prededuct:v2:"
	// PreDeductMaxAge 预扣记录最大存活时间（秒），防止异常未结算的预扣占用余额
	PreDeductMaxAge = 1800 // 30 分钟
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

// EnsureWallet 确保租户有钱包，没有则创建
// 使用 INSERT ... ON CONFLICT 保证并发安全
func EnsureWallet(ctx context.Context, tenantID int64) error {
	_, err := g.DB().Ctx(ctx).Exec(ctx,
		`INSERT INTO bil_wallets (tenant_id, balance, frozen_balance, warning_threshold, currency)
		 VALUES ($1, 0, 0, 1.00, 'USD')
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
	luaScript := `
local wallet_key = KEYS[1]
local prededuct_key = KEYS[2]
local amount = tonumber(ARGV[1])
local request_id = ARGV[2]
local ttl = tonumber(ARGV[3])

-- 检查是否已预扣（幂等）
local exists = redis.call("EXISTS", prededuct_key)
if exists == 1 then
    return 1
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

	amountMicro := toMicro(amount)
	result, err := g.Redis().Do(ctx, "EVAL", luaScript, 2,
		walletRedisKey, predeductRedisKey,
		amountMicro, requestID, PreDeductMaxAge, tenantID, modelName, time.Now().Unix())
	if err != nil {
		// Redis 不可用，降级到 DB 直接扣减
		return preDeductDB(ctx, tenantID, amount, requestID)
	}

	code := result.Int64()

	if code == 0 {
		return false, gerror.New("insufficient balance")
	}

	// 同步同步到 DB（确保 DB frozen_balance 在返回前已更新）
	preDeductSyncDB(ctx, tenantID, amount)

	// 异步写入预扣追踪记录（用于孤儿清理和 Redis 重建）
	go trackPreDeduct(context.Background(), tenantID, amount, requestID, modelName)

	return true, nil
}

// preDeductDB DB 降级预扣（Redis 不可用时）
func preDeductDB(ctx context.Context, tenantID int64, amount float64, requestID string) (bool, error) {
	// A9 修复：用单条原子条件更新替代「先 SELECT ... FOR UPDATE，再独立 UPDATE」。
	// 原实现两条语句在 autocommit 下各自成一个事务，FOR UPDATE 的行锁在 SELECT 语句提交后即释放，
	// 两个并发降级预扣可都通过 available 检查、再各自 frozen += amount → 超额冻结（可用余额被冻成负）。
	// 单条 "WHERE tenant_id=? AND balance - frozen_balance >= amount" 的 UPDATE 在语句执行期间持有
	// 行锁并原子重算谓词：RowsAffected==1 表示冻结成功，==0 表示可用余额不足（或钱包不存在）。
	res, err := g.DB().Exec(ctx,
		"UPDATE bil_wallets SET frozen_balance = frozen_balance + ?, updated_at = ? WHERE tenant_id = ? AND balance - frozen_balance >= ?",
		amount, time.Now(), tenantID, amount)
	if err != nil {
		return false, gerror.Wrapf(err, "pre-deduct db update")
	}
	affected, err := res.RowsAffected()
	if err != nil {
		return false, gerror.Wrapf(err, "pre-deduct db update result")
	}
	if affected == 0 {
		return false, gerror.New("insufficient balance")
	}

	// 清除缓存
	walletCache.Delete(ctx, fmt.Sprintf("%d", tenantID))

	// 异步写入预扣追踪记录
	go trackPreDeduct(context.Background(), tenantID, amount, requestID, "")

	return true, nil
}

// preDeductSyncDB sync pre-deduct to DB（同步调用，确保 DB frozen_balance 在返回前已更新）
func preDeductSyncDB(ctx context.Context, tenantID int64, amount float64) {
	_, err := g.DB().Exec(ctx,
		"UPDATE bil_wallets SET frozen_balance = frozen_balance + ?, updated_at = ? WHERE tenant_id = ?",
		amount, time.Now(), tenantID)
	if err != nil {
		g.Log().Errorf(ctx, "pre-deduct sync: %v", err)
	}
}

// unfreezeClampLua 解冻 frozen_balance 并保证不低于 0（对齐 DB 侧 GREATEST(frozen_balance - ?, 0) 下限）。
// v2：金额为整数 micro，整数运算无浮点漂移。钱包 hash 不存在（TTL 过期）时直接返回，
// 不凭空创建只含 frozen_balance 的残缺 key——冻结状态以 DB 为准，下次 PreDeduct 的 doSyncWalletToRedis 会重建。
// KEYS[1] = wallet:v2:{tenant_id}；ARGV[1] = 解冻金额（整数 micro）
const unfreezeClampLua = `
local wallet_key = KEYS[1]
local amount = tonumber(ARGV[1])
if redis.call("EXISTS", wallet_key) == 0 then
    return 0
end
local frozen = tonumber(redis.call("HGET", wallet_key, "frozen_balance") or "0")
local newFrozen = frozen - amount
if newFrozen < 0 then
    newFrozen = 0
end
redis.call("HSET", wallet_key, "frozen_balance", newFrozen)
return 1
`

// UnfreezePreDeduct 解冻预扣金额（请求失败时调用）
func UnfreezePreDeduct(ctx context.Context, tenantID int64, requestID string, amount float64) {
	if amount <= 0 {
		return
	}

	predeductRedisKey := fmt.Sprintf("%s%s", PreDeductRedisKeyPrefix, requestID)
	walletRedisKey := walletHashKey(tenantID)
	activeSetKey := fmt.Sprintf("prededuct_active:%d", tenantID)

	// 先尝试 Redis 解冻
	_, err := g.Redis().Do(ctx, "DEL", predeductRedisKey)
	if err == nil {
		// 解冻（带 0 下限保护，整数运算）。HINCRBY 亦无下限：若因重复/多余调用扣减超过已冻结额，
		// 会把 frozen_balance 打成负数。改用 Lua 读-clamp-写，与 DB 侧
		// GREATEST(frozen_balance - ?, 0) 保持一致；钱包 hash 不存在时不凭空创建（交由 DB 兜底）。
		g.Redis().Do(ctx, "EVAL", unfreezeClampLua, 1, walletRedisKey, toMicro(amount))
		g.Redis().Do(ctx, "SREM", activeSetKey, requestID)
		go unfreezeSyncDB(tenantID, amount)
		return
	}

	// Redis 失败，直接 DB 解冻
	unfreezeDB(ctx, tenantID, amount)
}

func unfreezeSyncDB(tenantID int64, amount float64) {
	bgCtx := context.Background()
	unfreezeDB(bgCtx, tenantID, amount)
}

func unfreezeDB(ctx context.Context, tenantID int64, amount float64) {
	// A9：单条原子更新即可，无需先 SELECT 再 UPDATE。
	// GREATEST(frozen_balance - ?, 0) 保证不会扣成负数；WHERE tenant_id 直接定位钱包行。
	_, err := g.DB().Exec(ctx,
		"UPDATE bil_wallets SET frozen_balance = GREATEST(frozen_balance - ?, 0), updated_at = ? WHERE tenant_id = ?",
		amount, time.Now(), tenantID)
	if err != nil {
		g.Log().Errorf(ctx, "unfreeze db: tenant=%d amount=%.6f: %v", tenantID, amount, err)
	}

	walletCache.Delete(ctx, fmt.Sprintf("%d", tenantID))
}

// GetPreDeductAmount 获取预扣金额
func GetPreDeductAmount(ctx context.Context, requestID string) (float64, bool) {
	predeductRedisKey := fmt.Sprintf("%s%s", PreDeductRedisKeyPrefix, requestID)
	result, err := g.Redis().Do(ctx, "HGET", predeductRedisKey, "amount")
	if err == nil && !result.IsNil() {
		// v2：amount 以整数 micro 存储，换算回 USD
		return fromMicro(result.Int64()), true
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
// frozen_balance 由 Redis Lua 脚本管理，仅在 key 首次创建时从 DB 初始化
func doSyncWalletToRedis(ctx context.Context, tenantID int64) error {
	walletRedisKey := walletHashKey(tenantID)

	// 从 DB 读取钱包数据（跳过内存缓存，直接查库确保最新）
	type walletRow struct {
		Balance       float64 `json:"balance"`
		FrozenBalance float64 `json:"frozen_balance"`
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

	// 检查 key 是否已存在
	exists, _ := g.Redis().Do(ctx, "EXISTS", walletRedisKey)

	if exists.Int64() == 0 {
		// key 不存在：完整初始化（balance + frozen_balance，均为整数 micro）
		_, err = g.Redis().Do(ctx, "HMSET", walletRedisKey,
			"balance", toMicro(w.Balance),
			"frozen_balance", toMicro(w.FrozenBalance),
		)
		if err != nil {
			return gerror.Wrapf(err, "sync wallet to redis")
		}

		// 从 DB 恢复活跃预扣明细到 Redis
		rebuildPredeductFromDB(ctx, tenantID)
	} else {
		// key 已存在：只更新 balance（frozen_balance 由 Lua 脚本管理，不覆盖）
		_, err = g.Redis().Do(ctx, "HSET", walletRedisKey,
			"balance", toMicro(w.Balance),
		)
	}
	if err != nil {
		return gerror.Wrapf(err, "sync wallet to redis")
	}

	// 设置过期时间（600s），过期后下次预扣会重新初始化
	g.Redis().Do(ctx, "EXPIRE", walletRedisKey, 600)

	return nil
}

// InvalidateWalletRedis 清除 Redis 中的钱包缓存（余额变更后调用）
func InvalidateWalletRedis(ctx context.Context, tenantID int64) {
	g.Redis().Do(ctx, "DEL", walletHashKey(tenantID))
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
			amount = fromMicro(gconv.Int64(v))
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

// trackPreDeduct 异步写入预扣追踪记录到 DB（用于孤儿清理和 Redis 重建）
func trackPreDeduct(ctx context.Context, tenantID int64, amount float64, requestID string, modelName string) {
	_, err := g.DB().Ctx(ctx).Exec(ctx,
		`INSERT INTO bil_prededuct_tracks (tenant_id, request_id, amount, model_name, status)
		 VALUES ($1, $2, $3, $4, 'frozen')
		 ON CONFLICT (request_id) DO NOTHING`,
		tenantID, requestID, amount, modelName)
	if err != nil {
		g.Log().Warningf(ctx, "[PRE-DEDUCT] track prededuct failed: request=%s err=%v", requestID, err)
	}
}

// rebuildPredeductFromDB 从 DB 恢复活跃预扣明细到 Redis（Redis 重启后调用）
func rebuildPredeductFromDB(ctx context.Context, tenantID int64) {
	type trackRow struct {
		RequestID string  `json:"request_id"`
		Amount    float64 `json:"amount"`
		ModelName string  `json:"model_name"`
		CreatedAt int64   `json:"created_at"`
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
			"amount", toMicro(t.Amount),
			"tenant_id", tenantID,
			"model_name", t.ModelName,
			"created_at", t.CreatedAt,
		)
		g.Redis().Do(ctx, "EXPIRE", predeductKey, remainingTTL)
		g.Redis().Do(ctx, "SADD", activeSetKey, t.RequestID)
	}
	// 确保 active SET 有 TTL（30 天），过期后下次预扣时自动重建
	g.Redis().Do(ctx, "EXPIRE", activeSetKey, 30*86400)

	g.Log().Infof(ctx, "[PRE-DEDUCT] rebuilt %d active tracks from DB for tenant=%d", len(tracks), tenantID)
}
