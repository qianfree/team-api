package billing

import (
	"context"
	"fmt"
	"strconv"
	"sync"
	"time"

	"github.com/gogf/gf/v2/errors/gerror"
	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/util/gconv"
	"github.com/shopspring/decimal"
	"golang.org/x/sync/singleflight"

	"github.com/qianfree/team-api/internal/dao"
	lcommon "github.com/qianfree/team-api/internal/logic/common"
)

const (
	// PreDeductRedisKeyPrefix 预扣 Redis key 前缀。
	// v2：预扣 amount 为整数微单位(micro-USD)存储，与旧版 float 值不兼容，
	// 故 bump 版本号；旧 key 自然按 TTL 过期，杜绝新代码把旧 float 值误读成 micro。
	PreDeductRedisKeyPrefix = "prededuct:v2:"
	// PreDeductMaxAge 预扣记录最大存活时间（秒），防止异常未结算的预扣占用余额
	PreDeductMaxAge = 7200 // 2 小时（长流式/realtime 会话防误杀：孤儿清扫与 Redis TTL 共用此阈值）

	// walletDirtyTenantsKey 钱包脏租户集合：每次资金变动（预扣/结算/解冻/充值等）都会
	// SADD 租户 ID，物化器按此集合把 Redis 权威状态周期性刷回 DB（bil_wallets 物化视图）。
	walletDirtyTenantsKey = "wallet_dirty_tenants"
)

// walletHashKey 钱包 Redis hash key。
// v2：balance / frozen_balance 以整数 micro-USD 存储；旧版 float key 随 TTL 过期。
//
// 钱包 hash 是资金状态的【唯一实时权威】（Redis 权威化架构）：
//   - 所有资金变动（预扣冻结、结算扣款、解冻、充值/退款/调账）全部通过 Lua 原子作用于它；
//   - DB bil_wallets 只是滞后物化视图（物化器每数秒从 Redis 全量覆盖），不得反向重建钱包 hash——
//     从滞后的 DB 重建会回滚未物化的增量，造成余额"复活"双花；
//   - 因此钱包 hash【不设 TTL】，仅在 Redis 重启/故障转移等灾难场景丢失后，
//     才走 rebuildWalletFromDB 灾难恢复（接受窗口期超扣风险）。
func walletHashKey(tenantID int64) string {
	return fmt.Sprintf("wallet:v2:%d", tenantID)
}

// walletCache 钱包静态字段缓存（TTL 300s）：id/预警阈值/币种/低余额标记。
// 注意：balance/frozen_balance 不进此缓存——权威值在 Redis，每次 GetWallet 实时读取。
var walletCache = lcommon.NewCache("wallet", 300*time.Second)

// walletRebuildGroup 合并同一租户的并发 rebuildWalletFromDB 恢复
var walletRebuildGroup singleflight.Group

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

// walletStaticFields 钱包静态字段（低频变更，走 DB + 进程内缓存）
type walletStaticFields struct {
	ID                 int64   `json:"id"`
	WarningThreshold   float64 `json:"warning_threshold"`
	Currency           string  `json:"currency"`
	LowBalanceNotified bool    `json:"low_balance_notified"`
}

// loadWalletStatic 读取钱包静态字段（进程内缓存 300s，回源 DB）
func loadWalletStatic(ctx context.Context, tenantID int64) (*walletStaticFields, error) {
	cacheKey := fmt.Sprintf("%d", tenantID)
	var cached walletStaticFields
	if walletCache.GetJSON(ctx, cacheKey, &cached) {
		return &cached, nil
	}

	var row *walletStaticFields
	err := dao.BilWallets.Ctx(ctx).
		Where("tenant_id", tenantID).
		Fields("id, warning_threshold, currency, low_balance_notified").
		Scan(&row)
	if err != nil {
		return nil, gerror.Wrapf(err, "query wallet static fields")
	}
	if row == nil {
		return nil, gerror.New("wallet not found")
	}

	walletCache.Set(ctx, cacheKey, row)
	return row, nil
}

// GetWallet 获取租户钱包：静态字段走 DB 缓存，balance/frozen_balance 读 Redis 权威值。
// Redis 中钱包 hash 缺失时触发灾难恢复重建后再读。
func GetWallet(ctx context.Context, tenantID int64) (*WalletInfo, error) {
	static, err := loadWalletStatic(ctx, tenantID)
	if err != nil {
		return nil, err
	}

	balance, frozen, err := getWalletRedisState(ctx, tenantID)
	if err != nil {
		return nil, err
	}

	return &WalletInfo{
		ID:                 static.ID,
		TenantID:           tenantID,
		Balance:            balance,
		FrozenBalance:      frozen,
		WarningThreshold:   static.WarningThreshold,
		Currency:           static.Currency,
		LowBalanceNotified: static.LowBalanceNotified,
	}, nil
}

// getWalletRedisState 读取 Redis 权威余额/冻结（USD float）。hash 缺失时先灾难恢复重建。
func getWalletRedisState(ctx context.Context, tenantID int64) (balance, frozen float64, err error) {
	balanceMicro, frozenMicro, exists, err := readWalletHash(ctx, tenantID)
	if err != nil {
		return 0, 0, err
	}
	if !exists {
		if err = rebuildWalletFromDB(ctx, tenantID); err != nil {
			return 0, 0, err
		}
		balanceMicro, frozenMicro, _, err = readWalletHash(ctx, tenantID)
		if err != nil {
			return 0, 0, err
		}
	}
	return InexactFloat64(FromMicro(balanceMicro)), InexactFloat64(FromMicro(frozenMicro)), nil
}

// readWalletHash 读取钱包 hash 的 balance/frozen_balance（micro-USD）。
// exists=false 表示 hash 不存在（灾难恢复场景）。
func readWalletHash(ctx context.Context, tenantID int64) (balanceMicro, frozenMicro int64, exists bool, err error) {
	res, err := g.Redis().Do(ctx, "HGETALL", walletHashKey(tenantID))
	if err != nil {
		return 0, 0, false, gerror.Wrapf(err, "read wallet hash")
	}
	if res.IsNil() || res.IsEmpty() {
		return 0, 0, false, nil
	}
	m := res.Map()
	balanceMicro = gconv.Int64(m["balance"])
	frozenMicro = gconv.Int64(m["frozen_balance"])
	return balanceMicro, frozenMicro, true, nil
}

// rebuildWalletFromDB 灾难恢复：钱包 hash 缺失时从 DB 物化值重建。
// frozen 不取 DB 值，而是按该租户【幸存的预扣 hash】重算——Redis 全丢时在途预扣已不可考，
// frozen 归零（接受窗口期超扣风险）；部分丢失（如 hash 被误删）时幸存明细仍能提供精确冻结。
func rebuildWalletFromDB(ctx context.Context, tenantID int64) error {
	_, err, _ := walletRebuildGroup.Do(strconv.FormatInt(tenantID, 10), func() (interface{}, error) {
		return nil, doRebuildWalletFromDB(context.Background(), tenantID)
	})
	return err
}

func doRebuildWalletFromDB(ctx context.Context, tenantID int64) error {
	walletRedisKey := walletHashKey(tenantID)

	// 已存在则无需重建（并发场景下其他请求已完成）
	exists, err := g.Redis().Do(ctx, "EXISTS", walletRedisKey)
	if err != nil {
		return gerror.Wrapf(err, "rebuild wallet: check exists")
	}
	if exists.Int64() == 1 {
		return nil
	}

	var w *struct {
		Balance decimal.Decimal `json:"balance"`
	}
	err = dao.BilWallets.Ctx(ctx).
		Where("tenant_id", tenantID).
		Fields("balance").
		Scan(&w)
	if err != nil {
		return gerror.Wrapf(err, "rebuild wallet: query db")
	}
	if w == nil {
		return gerror.New("wallet not found")
	}

	// 幸存预扣明细重算 frozen（不取 DB frozen_balance：其中可能含已丢失预扣的残留）
	frozenMicro := sumSurvivingPrededucts(ctx, tenantID)

	// 「检查不存在 + 写入」原子完成，避免并发重建互相覆盖
	rebuildLua := `
local key = KEYS[1]
if redis.call("EXISTS", key) == 1 then
    return 0
end
redis.call("HSET", key, "balance", ARGV[1], "frozen_balance", ARGV[2], "ver", 0)
redis.call("SADD", "wallet_dirty_tenants", ARGV[3])
return 1
`
	ret, err := g.Redis().Do(ctx, "EVAL", rebuildLua, 1, walletRedisKey,
		ToMicro(w.Balance), frozenMicro, tenantID)
	if err != nil {
		return gerror.Wrapf(err, "rebuild wallet: redis")
	}
	if ret.Int64() == 1 {
		g.Log().Warningf(ctx,
			"[WALLET REBUILD] wallet hash rebuilt from DB (disaster recovery): tenant=%d balance=%s frozen_micro=%d — in-flight pre-deducts may be lost",
			tenantID, w.Balance.String(), frozenMicro)
	}
	return nil
}

// sumSurvivingPrededucts 求该租户幸存预扣 hash 的金额总和（micro-USD），用于灾后 frozen 重算
func sumSurvivingPrededucts(ctx context.Context, tenantID int64) int64 {
	activeSetKey := fmt.Sprintf("prededuct_active:%d", tenantID)
	members, err := g.Redis().Do(ctx, "SMEMBERS", activeSetKey)
	if err != nil || members.IsNil() {
		return 0
	}
	var sum int64
	for _, rid := range members.Strings() {
		amt, err := g.Redis().Do(ctx, "HGET", PreDeductRedisKeyPrefix+rid, "amount")
		if err == nil && !amt.IsNil() {
			sum += amt.Int64()
		}
	}
	return sum
}

// walletIDCache 租户ID→钱包ID 进程内映射缓存。
// 钱包一经创建（EnsureWallet）ID 即不变，可长期缓存且无需失效。
var walletIDCache sync.Map // tenantID(int64) → walletID(int64)

// GetWalletID 获取租户钱包 ID（进程内缓存，键值不变、永不失效）。
// 走静态字段缓存而非 GetWallet：结算热路径不应为取 ID 额外访问 Redis。
func GetWalletID(ctx context.Context, tenantID int64) (int64, error) {
	if v, ok := walletIDCache.Load(tenantID); ok {
		return v.(int64), nil
	}
	static, err := loadWalletStatic(ctx, tenantID)
	if err != nil {
		return 0, err
	}
	walletIDCache.Store(tenantID, static.ID)
	return static.ID, nil
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

// 预扣 Lua：原子检查+冻结（金额全部为整数 micro-USD，整数运算无浮点漂移）
// KEYS[1] = wallet:v2:{tenant_id}  (hash: balance, frozen_balance, ver —— 金额均为整数 micro)
// KEYS[2] = prededuct:v2:{request_id}
// ARGV[1] = amount_micro（整数微单位）
// ARGV[2] = request_id
// ARGV[3] = ttl (PreDeductMaxAge)
// ARGV[4] = tenant_id
// ARGV[5] = model_name
// ARGV[6] = created_at (unix timestamp)
// 返回值：1 冻结成功 / 0 可用余额不足 / 2 已预扣（幂等） / 3 钱包 hash 缺失需重建
const preDeductLua = `
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

-- 钱包 hash 缺失：不得把缺失字段当 0 参与计算，返回 3 由调用方重建后重试
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

-- 冻结金额（整数自增，无浮点漂移），递增版本号并标记脏租户（驱动物化）
redis.call("HINCRBY", wallet_key, "frozen_balance", amount)
redis.call("HINCRBY", wallet_key, "ver", 1)
redis.call("SADD", "wallet_dirty_tenants", ARGV[4])
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

// PreDeduct 预扣费用（Redis Lua 原子操作，资金提交点在 Redis，无 DB 写）。
// Redis 不可用时 fail-closed 返回错误（用户确认：故障期间拒绝请求，不放行无额度保护的流量）。
func PreDeduct(ctx context.Context, tenantID int64, amount float64, requestID string, modelName string) (bool, error) {
	if amount <= 0 {
		return true, nil
	}

	amountMicro := ToMicro(NewFromFloat(amount))
	code, err := evalPreDeduct(ctx, tenantID, requestID, modelName, amountMicro)
	if err != nil {
		// fail-closed：预扣无法完成时拒绝请求，不做无冻结放行
		return false, gerror.Wrapf(err, "pre-deduct: redis unavailable")
	}

	if code == 3 {
		// 钱包 hash 缺失：灾难恢复重建后重试一次
		if err = rebuildWalletFromDB(ctx, tenantID); err != nil {
			return false, gerror.Wrapf(err, "pre-deduct: rebuild wallet")
		}
		code, err = evalPreDeduct(ctx, tenantID, requestID, modelName, amountMicro)
		if err != nil {
			return false, gerror.Wrapf(err, "pre-deduct: redis unavailable after rebuild")
		}
		if code == 3 {
			return false, gerror.Newf("pre-deduct: wallet rebuild ineffective for tenant %d", tenantID)
		}
	}

	switch code {
	case 0:
		return false, gerror.New("insufficient balance")
	case 2:
		// 幂等：同一 request_id 已预扣过
		return true, nil
	}
	return true, nil
}

// evalPreDeduct 执行预扣 Lua，返回脚本结果码
func evalPreDeduct(ctx context.Context, tenantID int64, requestID, modelName string, amountMicro int64) (int64, error) {
	result, err := g.Redis().Do(ctx, "EVAL", preDeductLua, 2,
		walletHashKey(tenantID), PreDeductRedisKeyPrefix+requestID,
		amountMicro, requestID, PreDeductMaxAge, tenantID, modelName, time.Now().Unix())
	if err != nil {
		return -1, err
	}
	return result.Int64(), nil
}

// 解冻 Lua：认领预扣 hash 并释放冻结（认领即删，天然幂等——重复调用认领不到返回 0）。
// KEYS[1] = wallet:v2:{tenant_id}
// KEYS[2] = prededuct:v2:{request_id}
// ARGV[1] = request_id
// ARGV[2] = tenant_id
// 返回值：实际释放的金额（micro），0 表示预扣不存在（已释放/已结算/已过期）
const unfreezeLua = `
local wallet_key = KEYS[1]
local prededuct_key = KEYS[2]
local request_id = ARGV[1]
local tenant_id = ARGV[2]

local amount = redis.call("HGET", prededuct_key, "amount")
if not amount then
    return 0
end
amount = tonumber(amount)

redis.call("DEL", prededuct_key)
redis.call("SREM", "prededuct_active:" .. tenant_id, request_id)

local frozen = tonumber(redis.call("HINCRBY", wallet_key, "frozen_balance", -amount))
if frozen < 0 then
    redis.call("HSET", wallet_key, "frozen_balance", 0)
end
redis.call("HINCRBY", wallet_key, "ver", 1)
redis.call("SADD", "wallet_dirty_tenants", tenant_id)
return amount
`

// UnfreezePreDeduct 解冻预扣金额（请求失败/管理端释放时调用）。
// 金额由 Redis 预扣 hash 决定，不信任调用方传入的估算值；认领即删保证恰好一次。
// 返回实际解冻金额（0 表示预扣已不存在，幂等空操作）。
func UnfreezePreDeduct(ctx context.Context, tenantID int64, requestID string) (decimal.Decimal, error) {
	result, err := g.Redis().Do(ctx, "EVAL", unfreezeLua, 2,
		walletHashKey(tenantID), PreDeductRedisKeyPrefix+requestID,
		requestID, tenantID)
	if err != nil {
		return Zero, gerror.Wrapf(err, "unfreeze: redis")
	}
	return FromMicro(result.Int64()), nil
}

// 结算认领 Lua：认领预扣（可多 key，task 结算含 "_adjust"）→ 释放冻结 → 扣减余额。
// KEYS[1] = wallet:v2:{tenant_id}
// KEYS[2..n] = prededuct:v2:{request_id}（与 ARGV[3..] 的 request_id 一一对应）
// ARGV[1] = cost_micro（实际扣款金额）
// ARGV[2] = tenant_id
// ARGV[3..] = request_id
// 返回值：{claimed_micro, balance_after_micro, frozen_after_micro}
// 预扣 hash 不存在（已释放/过期/丢失）时按 claimed=0 处理：只扣 balance 不动 frozen，
// 与既有「track 丢失只扣 balance」语义一致（调用方据此打告警）。
const settleClaimLua = `
local wallet_key = KEYS[1]
local cost = tonumber(ARGV[1])
local tenant_id = ARGV[2]
local active_set = "prededuct_active:" .. tenant_id

local claimed = 0
for i = 2, #KEYS do
    local rid = ARGV[i + 1]
    local amt = redis.call("HGET", KEYS[i], "amount")
    if amt then
        claimed = claimed + tonumber(amt)
        redis.call("DEL", KEYS[i])
        redis.call("SREM", active_set, rid)
    end
end

local frozen = tonumber(redis.call("HINCRBY", wallet_key, "frozen_balance", -claimed))
if frozen < 0 then
    redis.call("HSET", wallet_key, "frozen_balance", 0)
    frozen = 0
end
local balance = tonumber(redis.call("HINCRBY", wallet_key, "balance", -cost))
redis.call("HINCRBY", wallet_key, "ver", 1)
redis.call("SADD", "wallet_dirty_tenants", tenant_id)
return {claimed, balance, frozen}
`

// SettleClaim 结算认领（Redis 提交点）：原子完成「认领预扣 → 释放冻结 → 扣减余额」。
// 返回实际认领金额与扣款后的余额/冻结快照（micro 精度，供流水 balance_after/frozen_after）。
func SettleClaim(ctx context.Context, tenantID int64, cost decimal.Decimal, requestIDs []string) (claimed, balanceAfter, frozenAfter decimal.Decimal, err error) {
	evalArgs := make([]any, 0, 4+2*len(requestIDs))
	evalArgs = append(evalArgs, settleClaimLua, 1+len(requestIDs), walletHashKey(tenantID))
	for _, rid := range requestIDs {
		evalArgs = append(evalArgs, PreDeductRedisKeyPrefix+rid)
	}
	evalArgs = append(evalArgs, ToMicro(cost), tenantID)
	for _, rid := range requestIDs {
		evalArgs = append(evalArgs, rid)
	}

	result, err := g.Redis().Do(ctx, "EVAL", evalArgs...)
	if err != nil {
		return Zero, Zero, Zero, gerror.Wrapf(err, "settle claim: redis")
	}
	vals := result.Int64s()
	if len(vals) != 3 {
		return Zero, Zero, Zero, gerror.Newf("settle claim: unexpected redis reply %v", vals)
	}
	return FromMicro(vals[0]), FromMicro(vals[1]), FromMicro(vals[2]), nil
}

// 充值/调账加款 Lua：原子增加余额并返回 {加后余额, 当前冻结}（micro）。
// KEYS[1] = wallet:v2:{tenant_id}
// ARGV[1] = amount_micro（正数；补偿逆转场景传负数等价于无门槛扣款）
// ARGV[2] = tenant_id
const creditWalletLua = `
local balance = redis.call("HINCRBY", KEYS[1], "balance", ARGV[1])
local frozen = tonumber(redis.call("HGET", KEYS[1], "frozen_balance") or "0")
redis.call("HINCRBY", KEYS[1], "ver", 1)
redis.call("SADD", "wallet_dirty_tenants", ARGV[2])
return {balance, frozen}
`

// 扣款 Lua（带可用余额门槛）：balance - frozen >= amount 才扣，否则返回 {-1, 0}。
// KEYS[1] = wallet:v2:{tenant_id}
// ARGV[1] = amount_micro（正数）
// ARGV[2] = tenant_id
const debitWalletLua = `
local amount = tonumber(ARGV[1])
local balance = tonumber(redis.call("HGET", KEYS[1], "balance") or "0")
local frozen = tonumber(redis.call("HGET", KEYS[1], "frozen_balance") or "0")
if balance - frozen < amount then
    return {-1, 0}
end
balance = redis.call("HINCRBY", KEYS[1], "balance", -amount)
redis.call("HINCRBY", KEYS[1], "ver", 1)
redis.call("SADD", "wallet_dirty_tenants", ARGV[2])
return {balance, frozen}
`

// CreditWalletRedis 钱包加款（Redis 提交点）：充值/兑换/人工充值/结算补偿逆转等场景。
// 返回加款后余额与当前冻结快照（供流水 balance_after/frozen_after）。
// hash 缺失时 HINCRBY 会新建仅含 balance 的 hash——仅应发生在「新租户首充」等合法场景；
// 其他场景说明发生了灾难丢失，由后续 rebuild/物化流程收敛。
func CreditWalletRedis(ctx context.Context, tenantID int64, amount decimal.Decimal) (balanceAfter, frozenAfter decimal.Decimal, err error) {
	result, err := g.Redis().Do(ctx, "EVAL", creditWalletLua, 1,
		walletHashKey(tenantID), ToMicro(amount), tenantID)
	if err != nil {
		return Zero, Zero, gerror.Wrapf(err, "credit wallet: redis")
	}
	vals := result.Int64s()
	if len(vals) != 2 {
		return Zero, Zero, gerror.Newf("credit wallet: unexpected redis reply %v", vals)
	}
	return FromMicro(vals[0]), FromMicro(vals[1]), nil
}

// DebitWalletRedis 钱包扣款（Redis 提交点，带可用余额门槛）：退款/管理端扣减等场景。
// ok=false 表示可用余额不足（balance - frozen < amount），未发生任何变更。
func DebitWalletRedis(ctx context.Context, tenantID int64, amount decimal.Decimal) (balanceAfter, frozenAfter decimal.Decimal, ok bool, err error) {
	result, err := g.Redis().Do(ctx, "EVAL", debitWalletLua, 1,
		walletHashKey(tenantID), ToMicro(amount), tenantID)
	if err != nil {
		return Zero, Zero, false, gerror.Wrapf(err, "debit wallet: redis")
	}
	vals := result.Int64s()
	if len(vals) != 2 {
		return Zero, Zero, false, gerror.Newf("debit wallet: unexpected redis reply %v", vals)
	}
	if vals[0] == -1 {
		return Zero, Zero, false, nil
	}
	return FromMicro(vals[0]), FromMicro(vals[1]), true, nil
}

// InvalidateWalletStaticCache 清除钱包静态字段缓存（预警阈值、低余额标记等低频字段变更后调用）。
// 注意：只清进程内静态缓存，【绝不】清 Redis 钱包 hash——hash 是权威余额（非可失效缓存），
// 删除会导致下次从滞后的 DB 重建而回滚未物化增量。
func InvalidateWalletStaticCache(ctx context.Context, tenantID int64) {
	walletCache.Delete(ctx, fmt.Sprintf("%d", tenantID))
}

// GetPreDeductAmount 获取预扣金额
func GetPreDeductAmount(ctx context.Context, requestID string) (float64, bool) {
	predeductRedisKey := PreDeductRedisKeyPrefix + requestID
	result, err := g.Redis().Do(ctx, "HGET", predeductRedisKey, "amount")
	if err == nil && !result.IsNil() {
		// v2：amount 以整数 micro 存储，换算回 USD
		return InexactFloat64(FromMicro(result.Int64())), true
	}
	return 0, false
}

// CleanupPreDeduct 清理预扣记录（结算/终态后调用，幂等）
func CleanupPreDeduct(ctx context.Context, tenantID int64, requestID string) {
	predeductRedisKey := PreDeductRedisKeyPrefix + requestID
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
		predeductKey := PreDeductRedisKeyPrefix + reqID

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
