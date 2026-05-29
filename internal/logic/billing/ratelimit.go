package billing

import (
	"context"
	"fmt"
	"time"

	"github.com/gogf/gf/v2/frame/g"
	"github.com/qianfree/team-api/internal/logic/common"
)

// RateLimitResult 限流检查结果
type RateLimitResult struct {
	Allowed    bool
	LimitLevel string // system / tenant / user / key
	Limit      int    // 限制值
	Remaining  int    // 剩余可用
	ResetAt    int64  // 限制重置时间（Unix timestamp，秒）
}

// RateLimitConfig 限流配置
type RateLimitConfig struct {
	SystemQPS  int // 系统级 QPS 限制（默认 10000）
	TenantQPS  int // 租户级 QPS 限制（默认 1000）
	UserQPS    int // 用户级 QPS 限制（默认 100）
	KeyQPS     int // Key 级 QPS 限制（默认 60）
	TenantConc int // 租户级并发限制（默认 50）
	UserConc   int // 用户级并发限制（默认 10）
	KeyConc    int // Key 级并发限制（默认 5）
}

// DefaultRateLimitConfig 默认限流配置
var DefaultRateLimitConfig = RateLimitConfig{
	SystemQPS:  10000,
	TenantQPS:  1000,
	UserQPS:    100,
	KeyQPS:     60,
	TenantConc: 50,
	UserConc:   10,
	KeyConc:    5,
}

// LoadRateLimitConfig loads rate limit config from ConfigService, falling back to defaults.
func LoadRateLimitConfig(ctx context.Context) RateLimitConfig {
	cfg := common.Config()
	c := DefaultRateLimitConfig

	if v := cfg.GetInt(ctx, "global_qps_limit"); v > 0 {
		c.SystemQPS = v
	}
	if v := cfg.GetInt(ctx, "tenant_qps_limit"); v > 0 {
		c.TenantQPS = v
	}
	if v := cfg.GetInt(ctx, "user_qps_limit"); v > 0 {
		c.UserQPS = v
	}
	if v := cfg.GetInt(ctx, "key_qps_limit"); v > 0 {
		c.KeyQPS = v
	}
	if v := cfg.GetInt(ctx, "tenant_concurrency_limit"); v > 0 {
		c.TenantConc = v
	}

	return c
}

// ---------------------------------------------------------------------------
// QPS 限流：单个 Lua 脚本完成 4 级检查
// ---------------------------------------------------------------------------

// qpsLimitLua 一次 EVAL 完成 4 级 QPS 限流（原子操作）。
// KEYS[1..4]: system/tenant/user/key 的 Redis key
// ARGV[1..4]: system/tenant/user/key 的限制值
// ARGV[5]: 窗口过期时间（秒）
//
// 返回值编码为一个整数：高32位=触发限流的级别(0=通过,1-4=级别), 低32位=剩余量
// 通过时高32位为0, low32为最严格级别的剩余量
var qpsLimitLua = `
local function check(key, limit, expire)
    if limit <= 0 then
        return 0, -1
    end
    local count = redis.call("INCR", key)
    if count == 1 then
        redis.call("EXPIRE", key, expire)
    end
    local remaining = limit - count
    if remaining < 0 then remaining = 0 end
    if count > limit then
        return 1, remaining
    end
    return 0, remaining
end

local status, remaining = check(KEYS[1], tonumber(ARGV[1]), tonumber(ARGV[5]))
if status == 1 then return 1 * 4294967296 + remaining end

status, remaining = check(KEYS[2], tonumber(ARGV[2]), tonumber(ARGV[5]))
if status == 1 then return 2 * 4294967296 + remaining end

status, remaining = check(KEYS[3], tonumber(ARGV[3]), tonumber(ARGV[5]))
if status == 1 then return 3 * 4294967296 + remaining end

status, remaining = check(KEYS[4], tonumber(ARGV[4]), tonumber(ARGV[5]))
if status == 1 then return 4 * 4294967296 + remaining end

return remaining
`

var levelNames = [5]string{"", "system", "tenant", "user", "key"}

// CheckRateLimit 检查 QPS 限流（四级：系统→租户→用户→Key）
// 使用单个 Lua 脚本在一次 EVAL 中完成所有检查，替代原来的 4 次串行 INCR。
func CheckRateLimit(ctx context.Context, config RateLimitConfig, tenantID, userID, apiKeyID int64) *RateLimitResult {
	now := time.Now()
	windowStart := now.Unix()

	systemKey := fmt.Sprintf("ratelimit:system:%d", windowStart)
	tenantKey := fmt.Sprintf("ratelimit:tenant:%d:%d", tenantID, windowStart)
	userKey := fmt.Sprintf("ratelimit:user:%d:%d", userID, windowStart)
	keyKey := fmt.Sprintf("ratelimit:key:%d:%d", apiKeyID, windowStart)

	result, err := g.Redis().Do(ctx, "EVAL", qpsLimitLua, 4,
		systemKey, tenantKey, userKey, keyKey,
		config.SystemQPS, config.TenantQPS, config.UserQPS, config.KeyQPS,
		2) // EXPIRE 秒数
	if err != nil {
		// Redis 不可用，允许通过
		return &RateLimitResult{Allowed: true, LimitLevel: "system", Limit: config.SystemQPS, Remaining: -1}
	}

	val := result.Int64()
	resetAt := now.Add(1 * time.Second).Unix()

	// 解码：高32位=级别(0=通过,1-4=被限级别), 低32位=剩余量
	levelIdx := int(val >> 32)
	remaining := int(val & 0xFFFFFFFF)

	if levelIdx == 0 {
		// 通过，remaining 是 key 级的剩余量
		return &RateLimitResult{
			Allowed:    true,
			LimitLevel: "key",
			Limit:      config.KeyQPS,
			Remaining:  remaining,
			ResetAt:    resetAt,
		}
	}

	// 被限流
	if levelIdx < 1 || levelIdx > 4 {
		levelIdx = 4
	}
	var limitVal int
	switch levelIdx {
	case 1:
		limitVal = config.SystemQPS
	case 2:
		limitVal = config.TenantQPS
	case 3:
		limitVal = config.UserQPS
	case 4:
		limitVal = config.KeyQPS
	}

	return &RateLimitResult{
		Allowed:    false,
		LimitLevel: levelNames[levelIdx],
		Limit:      limitVal,
		Remaining:  remaining,
		ResetAt:    resetAt,
	}
}

// ---------------------------------------------------------------------------
// 并发限流：单个 Lua 脚本完成 4 级获取 + 失败回滚
// ---------------------------------------------------------------------------

// acquireConcurrentLua 一次 EVAL 完成 4 级并发许可获取。
// KEYS[1..4]: tenant/model/user/key 的 Redis key
// ARGV[1..4]: 各级并发限制（0 表示不检查该级）
// ARGV[5]: EXPIRE 秒数
//
// 逐级尝试获取，任一级失败则回滚已获取的许可。
// 返回 1=成功, 0=失败。
var acquireConcurrentLua = `
local keys = {KEYS[1], KEYS[2], KEYS[3], KEYS[4]}
local limits = {tonumber(ARGV[1]), tonumber(ARGV[2]), tonumber(ARGV[3]), tonumber(ARGV[4])}
local expire = tonumber(ARGV[5])
local acquired = {}

local function tryAcquire(i)
    if limits[i] <= 0 then return true end
    local count = redis.call("INCR", keys[i])
    if count == 1 then
        redis.call("EXPIRE", keys[i], expire)
    end
    if count > limits[i] then
        for j = 1, #acquired do
            local v = redis.call("DECR", acquired[j])
            if v < 0 then
                redis.call("SET", acquired[j], 0)
            end
        end
        return false
    end
    table.insert(acquired, keys[i])
    return true
end

if not tryAcquire(1) then return 0 end
if not tryAcquire(2) then return 0 end
if not tryAcquire(3) then return 0 end
if not tryAcquire(4) then return 0 end
return 1
`

// releaseConcurrentLua 一次 EVAL 释放 4 级并发许可。
var releaseConcurrentLua = `
for i = 1, 4 do
    local v = redis.call("DECR", KEYS[i])
    if v < 0 then
        redis.call("SET", KEYS[i], 0)
    end
end
return 1
`

// AcquireConcurrent 获取并发许可（四级：租户→模型→用户→Key）
// 租户级并发限制优先使用 tnt_tenants.max_concurrency，0 表示不限；
// 模型级并发限制使用 mdl_tenant_models.max_concurrency，NULL 表示不限；
// 若未设置则回退到全局配置 tenant_concurrency_limit。
func AcquireConcurrent(ctx context.Context, config RateLimitConfig, tenantID, userID, apiKeyID int64, modelName string) bool {
	tenantKey := fmt.Sprintf("conc:tenant:%d", tenantID)
	modelKey := fmt.Sprintf("conc:tenant:%d:model:%s", tenantID, modelName)
	userKey := fmt.Sprintf("conc:user:%d", userID)
	keyKey := fmt.Sprintf("conc:key:%d", apiKeyID)

	// 获取租户级和模型级并发限制（含缓存，可能查 DB）
	tenantLimit := getTenantConcurrencyLimit(ctx, tenantID, config.TenantConc)
	modelLimit := getModelConcurrencyLimit(ctx, tenantID, modelName)

	result, err := g.Redis().Do(ctx, "EVAL", acquireConcurrentLua, 4,
		tenantKey, modelKey, userKey, keyKey,
		tenantLimit, modelLimit, config.UserConc, config.KeyConc,
		300) // EXPIRE 秒数
	if err != nil {
		return true // Redis 不可用，允许通过
	}

	return result.Int() == 1
}

// ReleaseConcurrent 释放并发许可（含模型级）
// 单次 EVAL 释放所有 4 个 key，替代原来的 4 次 DECR。
func ReleaseConcurrent(ctx context.Context, tenantID, userID, apiKeyID int64, modelName string) {
	tenantKey := fmt.Sprintf("conc:tenant:%d", tenantID)
	modelKey := fmt.Sprintf("conc:tenant:%d:model:%s", tenantID, modelName)
	userKey := fmt.Sprintf("conc:user:%d", userID)
	keyKey := fmt.Sprintf("conc:key:%d", apiKeyID)

	g.Redis().Do(ctx, "EVAL", releaseConcurrentLua, 4,
		tenantKey, modelKey, userKey, keyKey)
}

// getTenantConcurrencyLimit 获取租户并发上限。
// 优先从 Redis 缓存读取 tnt_tenants.max_concurrency，缓存 300 秒；
// 缓存未命中则查询数据库并回填。0 表示不限，回退到全局默认值。
func getTenantConcurrencyLimit(ctx context.Context, tenantID int64, defaultLimit int) int {
	cacheKey := fmt.Sprintf("tenant:conc_limit:%d", tenantID)

	// 尝试从 Redis 缓存读取
	val, err := g.Redis().Do(ctx, "GET", cacheKey)
	if err == nil && !val.IsNil() {
		n := val.Int()
		if n > 0 {
			return n
		}
		if n == -1 {
			// -1 表示数据库中值为 0（不限），直接返回 0
			return 0
		}
	}

	// 缓存未命中，查数据库
	// 缓存未命中，查数据库
	var maxConc *int
	err = g.DB().Model("tnt_tenants").
		Where("id", tenantID).
		Fields("max_concurrency").
		Scan(&maxConc)
	if err != nil {
		// 查询失败，使用全局默认值
		return defaultLimit
	}

	// NULL 表示跟随等级配置
	if maxConc == nil {
		_, effectiveConc, err := GetTenantEffectiveLimits(ctx, tenantID)
		if err != nil {
			return defaultLimit
		}
		if effectiveConc > 0 {
			g.Redis().Do(ctx, "SET", cacheKey, effectiveConc, "EX", 300)
			return effectiveConc
		}
		// 等级配置也是 0（不限），缓存 -1 标记，回退到全局默认
		g.Redis().Do(ctx, "SET", cacheKey, -1, "EX", 300)
		return defaultLimit
	}

	if *maxConc > 0 {
		// 有自定义限制，缓存为实际值
		g.Redis().Do(ctx, "SET", cacheKey, *maxConc, "EX", 300)
		return *maxConc
	}

	// 数据库中为 0（不限），缓存 -1 标记，回退到全局默认
	g.Redis().Do(ctx, "SET", cacheKey, -1, "EX", 300)
	return defaultLimit

}

// getModelConcurrencyLimit 获取租户模型级并发上限。
// 查询 mdl_tenant_models.max_concurrency，缓存 300 秒；
// NULL 表示不限（返回 0）。
func getModelConcurrencyLimit(ctx context.Context, tenantID int64, modelName string) int {
	cacheKey := fmt.Sprintf("tenant:model:conc_limit:%d:%s", tenantID, modelName)

	// 尝试从 Redis 缓存读取
	val, err := g.Redis().Do(ctx, "GET", cacheKey)
	if err == nil && !val.IsNil() {
		n := val.Int()
		if n > 0 {
			return n
		}
		// -1 表示数据库中值为 NULL/0（不限）
		return 0
	}

	// 缓存未命中，查数据库：先获取 model_id，再查 tenant_models
	var modelID int64
	err = g.DB().Model("mdl_models").
		Where("model_name", modelName).
		Fields("id").
		Scan(&modelID)
	if err != nil || modelID == 0 {
		// 模型不存在，不限制
		g.Redis().Do(ctx, "SET", cacheKey, -1, "EX", 300)
		return 0
	}

	var maxConc *int
	err = g.DB().Model("mdl_tenant_models").
		Where("tenant_id", tenantID).
		Where("model_id", modelID).
		Fields("max_concurrency").
		Scan(&maxConc)
	if err != nil {
		// 查询失败，不限制
		g.Redis().Do(ctx, "SET", cacheKey, -1, "EX", 300)
		return 0
	}

	if maxConc != nil && *maxConc > 0 {
		g.Redis().Do(ctx, "SET", cacheKey, *maxConc, "EX", 300)
		return *maxConc
	}

	// NULL 或 0 表示不限
	g.Redis().Do(ctx, "SET", cacheKey, -1, "EX", 300)
	return 0
}

// RateLimitHeaders 构建 X-RateLimit 响应头
func RateLimitHeaders(result *RateLimitResult) map[string]string {
	if result == nil {
		return nil
	}
	return map[string]string{
		"X-RateLimit-Limit":     fmt.Sprintf("%d", result.Limit),
		"X-RateLimit-Remaining": fmt.Sprintf("%d", result.Remaining),
		"X-RateLimit-Reset":     fmt.Sprintf("%d", result.ResetAt),
	}
}
