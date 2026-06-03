package billing

import (
	"context"
	"fmt"
	"time"

	"github.com/gogf/gf/v2/errors/gerror"
	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/util/gconv"

	"github.com/qianfree/team-api/internal/dao"
	lcommon "github.com/qianfree/team-api/internal/logic/common"
)

const (
	// PreDeductRedisKeyPrefix 预扣 Redis key 前缀
	PreDeductRedisKeyPrefix = "prededuct:"
	// PreDeductMaxAge 预扣记录最大存活时间（秒），防止异常未结算的预扣占用余额
	PreDeductMaxAge = 1800 // 30 分钟
)

// walletCache 钱包缓存（TTL 300s）
var walletCache = lcommon.NewCache("wallet", 300*time.Second)

// WalletInfo 钱包信息
type WalletInfo struct {
	ID               int64
	TenantID         int64
	Balance          float64
	FrozenBalance    float64
	WarningThreshold float64
	Currency         string
}

// GetWallet 获取租户钱包
func GetWallet(ctx context.Context, tenantID int64) (*WalletInfo, error) {
	cacheKey := fmt.Sprintf("%d", tenantID)
	var cached WalletInfo
	if walletCache.GetJSON(ctx, cacheKey, &cached) {
		return &cached, nil
	}

	type walletRow struct {
		ID               int64   `json:"id"`
		TenantId         int64   `json:"tenant_id"`
		Balance          float64 `json:"balance"`
		FrozenBalance    float64 `json:"frozen_balance"`
		WarningThreshold float64 `json:"warning_threshold"`
		Currency         string  `json:"currency"`
	}

	var w *walletRow
	err := dao.BilWallets.Ctx(ctx).
		Where("tenant_id", tenantID).
		Fields("id, tenant_id, balance, frozen_balance, warning_threshold, currency").
		Scan(&w)
	if err != nil {
		return nil, gerror.Wrapf(err, "query wallet")
	}
	if w == nil {
		return nil, gerror.New("wallet not found")
	}

	info := &WalletInfo{
		ID:               w.ID,
		TenantID:         w.TenantId,
		Balance:          w.Balance,
		FrozenBalance:    w.FrozenBalance,
		WarningThreshold: w.WarningThreshold,
		Currency:         w.Currency,
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

// CheckBalance 检查余额是否足够
func CheckBalance(ctx context.Context, tenantID int64, amount float64) error {
	wallet, err := GetWallet(ctx, tenantID)
	if err != nil {
		return err
	}

	available := AvailableBalance(wallet)
	if available < amount {
		return gerror.Newf("insufficient balance: available %.6f, required %.6f", available, amount)
	}

	return nil
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

	// Redis Lua 脚本：原子检查+冻结
	// KEYS[1] = wallet:{tenant_id}  (hash: balance, frozen_balance)
	// KEYS[2] = prededuct:{request_id}
	// ARGV[1] = amount
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

-- 获取钱包信息
local balance = tonumber(redis.call("HGET", wallet_key, "balance") or "0")
local frozen = tonumber(redis.call("HGET", wallet_key, "frozen_balance") or "0")

-- 检查可用余额
local available = balance - frozen
if available < amount then
    return 0
end

-- 冻结金额
redis.call("HINCRBYFLOAT", wallet_key, "frozen_balance", amount)
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

	walletRedisKey := fmt.Sprintf("wallet:%d", tenantID)
	predeductRedisKey := fmt.Sprintf("%s%s", PreDeductRedisKeyPrefix, requestID)

	result, err := g.Redis().Do(ctx, "EVAL", luaScript, 2,
		walletRedisKey, predeductRedisKey,
		amount, requestID, PreDeductMaxAge, tenantID, modelName, time.Now().Unix())
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
	// 使用条件检查：WHERE balance - frozen_balance >= amount
	type walletRow struct {
		ID            int64   `json:"id"`
		Balance       float64 `json:"balance"`
		FrozenBalance float64 `json:"frozen_balance"`
	}
	var w *walletRow
	err := dao.BilWallets.Ctx(ctx).
		Where("tenant_id", tenantID).
		Where("balance - frozen_balance >= ?", amount).
		Fields("id, balance, frozen_balance").
		LockUpdate().
		Scan(&w)
	if err != nil || w == nil {
		return false, gerror.New("insufficient balance")
	}

	_, err = g.DB().Exec(ctx,
		"UPDATE bil_wallets SET frozen_balance = frozen_balance + ?, updated_at = ? WHERE id = ? ",
		amount, time.Now(), w.ID)
	if err != nil {
		return false, gerror.Wrapf(err, "pre-deduct db update")
	}

	// 清除缓存
	walletCache.Delete(ctx, fmt.Sprintf("%d", tenantID))

	// 异步写入预扣追踪记录
	go trackPreDeduct(context.Background(), tenantID, amount, requestID, "")

	return true, nil
}

// preDeductSyncDB sync pre-deduct to DB（同步调用，确保 DB frozen_balance 在返回前已更新）
func preDeductSyncDB(ctx context.Context, tenantID int64, amount float64) {
	type walletRow struct {
		ID int64
	}
	var w *walletRow
	err := dao.BilWallets.Ctx(ctx).
		Where("tenant_id", tenantID).
		Fields("id").
		Scan(&w)
	if err != nil || w == nil {
		g.Log().Errorf(ctx, "pre-deduct sync: wallet not found for tenant %d", tenantID)
		return
	}

	_, err = g.DB().Exec(ctx,
		"UPDATE bil_wallets SET frozen_balance = frozen_balance + ?, updated_at = ? WHERE id = ? ",
		amount, time.Now(), w.ID)
	if err != nil {
		g.Log().Errorf(ctx, "pre-deduct sync: %v", err)
	}

}

// UnfreezePreDeduct 解冻预扣金额（请求失败时调用）
func UnfreezePreDeduct(ctx context.Context, tenantID int64, requestID string, amount float64) {
	if amount <= 0 {
		return
	}

	predeductRedisKey := fmt.Sprintf("%s%s", PreDeductRedisKeyPrefix, requestID)
	walletRedisKey := fmt.Sprintf("wallet:%d", tenantID)
	activeSetKey := fmt.Sprintf("prededuct_active:%d", tenantID)

	// 先尝试 Redis 解冻
	_, err := g.Redis().Do(ctx, "DEL", predeductRedisKey)
	if err == nil {
		// 解冻
		g.Redis().Do(ctx, "HINCRBYFLOAT", walletRedisKey, "frozen_balance", -amount)
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
	type walletRow struct {
		ID int64 `json:"id"`
	}
	var w *walletRow
	err := dao.BilWallets.Ctx(ctx).
		Where("tenant_id", tenantID).
		Fields("id").
		Scan(&w)
	if err != nil || w == nil {
		return
	}

	g.DB().Exec(ctx,
		"UPDATE bil_wallets SET frozen_balance = GREATEST(frozen_balance - ?, 0), updated_at = ? WHERE id = ? ",
		amount, time.Now(), w.ID)

	walletCache.Delete(ctx, fmt.Sprintf("%d", tenantID))
}

// GetPreDeductAmount 获取预扣金额
func GetPreDeductAmount(ctx context.Context, requestID string) (float64, bool) {
	predeductRedisKey := fmt.Sprintf("%s%s", PreDeductRedisKeyPrefix, requestID)
	result, err := g.Redis().Do(ctx, "HGET", predeductRedisKey, "amount")
	if err == nil && !result.IsNil() {
		return result.Float64(), true
	}
	return 0, false
}

// syncWalletToRedis 将钱包余额从 DB 同步到 Redis Hash
// 每次预扣前调用，确保 Redis 中的 balance 与 DB 一致
// frozen_balance 由 Redis Lua 脚本管理，仅在 key 首次创建时从 DB 初始化
func syncWalletToRedis(ctx context.Context, tenantID int64) error {
	walletRedisKey := fmt.Sprintf("wallet:%d", tenantID)

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
		// key 不存在：完整初始化（balance + frozen_balance）
		_, err = g.Redis().Do(ctx, "HMSET", walletRedisKey,
			"balance", w.Balance,
			"frozen_balance", w.FrozenBalance,
		)
		if err != nil {
			return gerror.Wrapf(err, "sync wallet to redis")
		}

		// 从 DB 恢复活跃预扣明细到 Redis
		rebuildPredeductFromDB(ctx, tenantID)
	} else {
		// key 已存在：只更新 balance（frozen_balance 由 Lua 脚本管理，不覆盖）
		_, err = g.Redis().Do(ctx, "HSET", walletRedisKey,
			"balance", w.Balance,
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
	walletRedisKey := fmt.Sprintf("wallet:%d", tenantID)
	g.Redis().Do(ctx, "DEL", walletRedisKey)
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
			amount = gconv.Float64(v)
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
			"amount", t.Amount,
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
