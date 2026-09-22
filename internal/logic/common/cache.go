package common

import (
	"context"
	"encoding/json"
	"math"
	"math/rand/v2"
	"reflect"
	"time"

	"github.com/gogf/gf/v2/database/gredis"
	"github.com/gogf/gf/v2/errors/gerror"
	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/os/gcache"
	"github.com/gogf/gf/v2/util/gconv"
	"golang.org/x/sync/singleflight"
)

// Cache provides a two-level caching mechanism:
// L1: In-memory cache (GoFrame gcache, handles Go structs natively)
// L2: Redis cache (stores JSON-serialized values, deserializes on read)
//
// Write: Set L1 (native) + Set L2 (JSON)
// Read: Get L1 → miss → Get L2 (JSON → struct) → miss → call fn → Set L1+L2
type Cache struct {
	prefix string
	ttl    time.Duration
	group  singleflight.Group
}

func jitterTTL(ttl time.Duration) time.Duration {
	if ttl <= 0 {
		return ttl
	}
	spread := ttl / 10
	if spread <= 0 {
		return ttl
	}
	delta := time.Duration(rand.Int64N(int64(spread)*2+1)) - spread
	return ttl + delta
}

// NewCache creates a new Cache instance with the given prefix and default TTL.
func NewCache(prefix string, ttl time.Duration) *Cache {
	return &Cache{
		prefix: prefix,
		ttl:    ttl,
	}
}

// fullKey returns the full cache key with prefix.
func (c *Cache) fullKey(key string) string {
	return c.prefix + ":" + key
}

// Set sets a value in both L1 (memory) and L2 (Redis) caches.
// L1 stores the value natively (Go struct pointers work).
// L2 stores a JSON-serialized copy for cross-process compatibility.
func (c *Cache) Set(ctx context.Context, key string, value any, ttl ...time.Duration) {
	expire := c.ttl
	if len(ttl) > 0 && ttl[0] > 0 {
		expire = ttl[0]
	}
	expire = jitterTTL(expire)
	fullKey := c.fullKey(key)

	// L1: memory cache (native Go struct storage)
	gcache.Set(ctx, fullKey, value, expire)

	// L2: Redis cache (JSON serialized)
	ttlSeconds := int64(math.Ceil(expire.Seconds()))
	jsonBytes, err := json.Marshal(value)
	if err != nil {
		g.Log().Warningf(ctx, "[Cache] JSON marshal failed key=%s: %v", fullKey, err)
		return
	}
	if ttlSeconds > 0 {
		_, _ = g.Redis().Do(ctx, "SETEX", fullKey, ttlSeconds, string(jsonBytes))
	} else {
		// 兜底：ttl ≤ 0 时使用 24h 兜底 TTL，避免产生永久 key
		g.Log().Warningf(ctx, "[Cache] TTL <= 0 for key=%s, using fallback 24h TTL", fullKey)
		_, _ = g.Redis().Do(ctx, "SETEX", fullKey, 86400, string(jsonBytes))
	}
}

// SetMany 批量写入 L1（内存）+ L2（Redis），语义与逐 key 调用 Set 一致。
// L1 逐 key 写本进程内存；L2 逐 key SETEX —— Redis 没有「批量且带过期」的原生命令
// （MSET 不支持 TTL），故写侧仍是 N 次往返。该方法只服务于「缓存未命中后回填」，
// 调用频次受 TTL 约束，不在热路径上；热路径的批量读走 GetMany（单次 MGET）。
func (c *Cache) SetMany(ctx context.Context, values map[string]any) {
	if len(values) == 0 {
		return
	}
	expire := jitterTTL(c.ttl)
	ttlSeconds := int64(math.Ceil(expire.Seconds()))
	if ttlSeconds <= 0 {
		// 兜底：ttl ≤ 0 时使用 24h 兜底 TTL，避免产生永久 key（与 Set 一致）
		g.Log().Warningf(ctx, "[Cache] TTL <= 0 for prefix=%s, using fallback 24h TTL", c.prefix)
		ttlSeconds = 86400
	}

	// L1 先整体落地：即使 L2 写入失败，本进程后续读取也能立即命中
	for key, value := range values {
		gcache.Set(ctx, c.fullKey(key), value, expire)
	}

	for key, value := range values {
		fullKey := c.fullKey(key)
		jsonBytes, err := json.Marshal(value)
		if err != nil {
			g.Log().Warningf(ctx, "[Cache] JSON marshal failed key=%s: %v", fullKey, err)
			continue
		}
		if _, err = g.Redis().Do(ctx, "SETEX", fullKey, ttlSeconds, string(jsonBytes)); err != nil {
			g.Log().Warningf(ctx, "[Cache] SETEX failed key=%s: %v", fullKey, err)
		}
	}
}

// GetMany 批量读取：先逐 key 查 L1（gcache，命中即零网络开销），
// 未命中的 key 用一次 MGET 走 L2（Redis），命中 L2 的值回填 L1。
// 返回「命中的原始 key → 值」，未命中的 key 不出现在结果中。
// key 为不含前缀的原始 key（与 Get/Set 一致）。
//
// 为什么必须批量：审计/日志列表每页要回填几十个关联名称，逐 key Get 会让冷路径
// 产生 N 次 Redis 往返，反而比直接查库更慢；MGET 把冷路径压成 1 次往返。
// Redis 不可用或 MGET 失败时只返回 L1 命中部分，未命中部分由调用方回落查库。
func (c *Cache) GetMany(ctx context.Context, keys []string) map[string]any {
	result := make(map[string]any, len(keys))
	if len(keys) == 0 {
		return result
	}

	missKeys := make([]string, 0, len(keys))
	missFullKeys := make([]string, 0, len(keys))
	for _, key := range keys {
		fullKey := c.fullKey(key)
		if val, err := gcache.Get(ctx, fullKey); err == nil && val != nil {
			result[key] = val.Interface()
			continue
		}
		missKeys = append(missKeys, key)
		missFullKeys = append(missFullKeys, fullKey)
	}
	if len(missFullKeys) == 0 {
		return result
	}

	// L2：一次 MGET 取回全部未命中 key（缺失 key 在返回 map 中为 nil 值）
	l2Values, err := g.Redis().MGet(ctx, missFullKeys...)
	if err != nil {
		g.Log().Warningf(ctx, "[Cache] MGET failed prefix=%s: %v", c.prefix, err)
		return result
	}
	for i, fullKey := range missFullKeys {
		v, ok := l2Values[fullKey]
		if !ok || v == nil || v.IsNil() {
			continue
		}
		jsonStr := v.String()
		if jsonStr == "" {
			continue
		}
		// 与 Get 一致：优先按 JSON 反序列化还原原始类型，非 JSON 的历史值按纯字符串兜底
		var raw any
		if unmarshalErr := json.Unmarshal([]byte(jsonStr), &raw); unmarshalErr != nil {
			raw = jsonStr
		}
		gcache.Set(ctx, fullKey, raw, jitterTTL(c.ttl))
		result[missKeys[i]] = raw
	}
	return result
}

// Get retrieves a value: L1 → L2 → miss.
// L1 returns native Go types. L2 deserializes JSON back to the original type.
func (c *Cache) Get(ctx context.Context, key string) (any, bool) {
	fullKey := c.fullKey(key)

	// Try L1: memory (native Go struct)
	val, err := gcache.Get(ctx, fullKey)
	if err == nil && val != nil {
		return val.Interface(), true
	}

	// Try L2: Redis (JSON → deserialize)
	redisVal, err := g.Redis().Do(ctx, "GET", fullKey)
	if err == nil && !redisVal.IsNil() {
		jsonStr := redisVal.String()
		if jsonStr != "" {
			// Unmarshal JSON to get the original Go value
			var raw any
			if unmarshalErr := json.Unmarshal([]byte(jsonStr), &raw); unmarshalErr == nil {
				gcache.Set(ctx, fullKey, raw, jitterTTL(c.ttl))
				return raw, true
			}
			// Fallback: treat as plain string (backward compat)
			gcache.Set(ctx, fullKey, jsonStr, jitterTTL(c.ttl))
			return jsonStr, true
		}
	}

	return nil, false
}

// GetJSON retrieves a JSON-serialized value from L2 and unmarshals it into target.
// Use this when caching Go struct pointers to avoid type assertion panics.
func (c *Cache) GetJSON(ctx context.Context, key string, target any) bool {
	fullKey := c.fullKey(key)

	// Try L1: memory (native Go struct — correct type already)
	val, err := gcache.Get(ctx, fullKey)
	if err == nil && val != nil {
		// L1 has native value; copy via JSON round-trip to populate target
		if jsonBytes, err := json.Marshal(val.Interface()); err == nil {
			return json.Unmarshal(jsonBytes, target) == nil
		}
	}

	// Try L2: Redis (JSON string)
	redisVal, err := g.Redis().Do(ctx, "GET", fullKey)
	if err == nil && !redisVal.IsNil() {
		jsonStr := redisVal.String()
		if jsonStr != "" {
			if unmarshalErr := json.Unmarshal([]byte(jsonStr), target); unmarshalErr != nil {
				g.Log().Warningf(ctx, "[Cache] JSON unmarshal failed key=%s: %v", fullKey, unmarshalErr)
				return false
			}
			// Backfill L1：存反序列化副本而非 target 本身。target 指针归调用方所有，
			// 若调用方在 GetJSON 返回后就地改写（如定价缓存按时段重评乘数），
			// 存储别名会与并发 L1 读者的 json.Marshal 构成数据竞争。
			// 副本失败时退回存 target（此时调用方须自律不改写，与历史行为一致）
			if cp, cpErr := unmarshalCopy(jsonStr, target); cpErr == nil {
				gcache.Set(ctx, fullKey, cp, jitterTTL(c.ttl))
			} else {
				gcache.Set(ctx, fullKey, target, jitterTTL(c.ttl))
			}
			return true
		}
	}

	return false
}

// unmarshalCopy 按 target 的具体类型从 JSON 生成一个独立副本（L1 回填用，避免
// L1 持有调用方 target 指针的别名）。target 必须是非 nil 指针（调用前已成功
// unmarshal 到 target，天然满足）。
func unmarshalCopy(jsonStr string, target any) (any, error) {
	t := reflect.TypeOf(target)
	if t == nil || t.Kind() != reflect.Pointer {
		return nil, gerror.New("cache: unmarshalCopy target must be a non-nil pointer")
	}
	cp := reflect.New(t.Elem()).Interface()
	if err := json.Unmarshal([]byte(jsonStr), cp); err != nil {
		return nil, err
	}
	return cp, nil
}

// GetOrSet retrieves a value or calls fn to set it if missing.
func (c *Cache) GetOrSet(ctx context.Context, key string, fn func(ctx context.Context) (any, error)) (any, error) {
	if val, ok := c.Get(ctx, key); ok {
		return val, nil
	}

	val, err, _ := c.group.Do(c.fullKey(key), func() (any, error) {
		if cached, ok := c.Get(ctx, key); ok {
			return cached, nil
		}
		loaded, loadErr := fn(ctx)
		if loadErr != nil {
			return nil, loadErr
		}
		c.Set(ctx, key, loaded)
		return loaded, nil
	})
	return val, err
}

// Delete removes a value from both L1 and L2.
func (c *Cache) Delete(ctx context.Context, key string) {
	fullKey := c.fullKey(key)

	// L1
	gcache.Remove(ctx, fullKey)

	// L2
	_, _ = g.Redis().Do(ctx, "DEL", fullKey)

	// Publish invalidation for other instances
	_, _ = g.Redis().Do(ctx, "PUBLISH", "cache:invalidate", fullKey)
}

// DeleteByPattern removes all cache entries matching the pattern.
// Uses SCAN + DEL to properly handle wildcard patterns (Redis DEL does not support wildcards).
// 同时清除 L1（gcache）并发布跨实例失效通知，与 Delete 行为对齐。
func (c *Cache) DeleteByPattern(ctx context.Context, pattern string) {
	fullPattern := c.fullKey(pattern)

	// 先收集所有匹配的 key，再统一处理 L1/L2/pub/sub
	var allKeys []string
	cursor := int64(0)
	for {
		result, err := g.Redis().Do(ctx, "SCAN", cursor, "MATCH", fullPattern, "COUNT", 100)
		if err != nil {
			g.Log().Warningf(ctx, "[Cache] SCAN failed pattern=%s: %v", fullPattern, err)
			break
		}
		slice := result.Slice()
		if len(slice) < 2 {
			break
		}
		cursor = gconv.Int64(slice[0])
		allKeys = append(allKeys, gconv.Strings(slice[1])...)
		if cursor == 0 {
			break
		}
	}

	if len(allKeys) == 0 {
		return
	}

	// L2: 批量删除 Redis（比逐条快）
	delArgs := make([]any, len(allKeys))
	for i, k := range allKeys {
		delArgs[i] = k
	}
	_, _ = g.Redis().Do(ctx, "DEL", delArgs...)

	// L1 + 跨实例失效：逐 key 清本进程内存缓存并发布失效通知
	// （gcache 不支持通配符删除，只能逐 key；PUBLISH 触发其他实例的订阅者清理 L1）
	for _, key := range allKeys {
		gcache.Remove(ctx, key)
		_, _ = g.Redis().Do(ctx, "PUBLISH", "cache:invalidate", key)
	}
}

// PublishInvalidation publishes a cache invalidation message via Redis Pub/Sub.
func PublishInvalidation(ctx context.Context, fullKey string) {
	_, _ = g.Redis().Do(ctx, "PUBLISH", "cache:invalidate", fullKey)
}

// StartCacheInvalidationSubscriber 订阅 Redis "cache:invalidate" 频道，收到失效通知时移除本进程 L1(gcache)中对应 key。
// C1 修复：此前 Cache.Delete / PublishInvalidation 只向 cache:invalidate 频道 PUBLISH，全仓库无人 SUBSCRIBE，
// 多实例部署下某实例删除 key 后，其他实例的 L1 gcache 会一直陈旧到自然 TTL。此订阅方补齐跨实例 L1 失效。
// 在 cmd.go 启动时调用一次；断线自动重连，语义与 ConfigService.StartSubscriber 一致。
func StartCacheInvalidationSubscriber(ctx context.Context) {
	go func() {
		var reconnectCount int
		for {
			conn, _, err := g.Redis().Subscribe(ctx, "cache:invalidate")
			if err != nil {
				reconnectCount++
				g.Log().Warningf(ctx, "[PubSub:cache] 连接失败 (第%d次): %v", reconnectCount, err)
				time.Sleep(5 * time.Second)
				continue
			}

			if reconnectCount > 0 {
				g.Log().Infof(ctx, "[PubSub:cache] 重连成功 (此前失败%d次)", reconnectCount)
				reconnectCount = 0
			} else {
				g.Log().Info(ctx, "[PubSub:cache] 订阅已启动")
			}

			for {
				v, err := conn.Receive(ctx)
				if err != nil {
					reconnectCount++
					g.Log().Warningf(ctx, "[PubSub:cache] 接收错误 (第%d次): %v", reconnectCount, err)
					time.Sleep(5 * time.Second)
					break // reconnect
				}

				msg, ok := v.Val().(*gredis.Message)
				if !ok {
					continue // skip Subscription/Pong 等
				}

				// 仅移除本进程 L1；L2(Redis) 由发布方已 DEL，无需重复处理
				gcache.Remove(ctx, msg.Payload)
			}

			conn.Close(ctx)
		}
	}()
}

// TenantGroupModelCache 缓存租户通过分组可访问的模型集合，TTL 300s
var TenantGroupModelCache = NewCache("tenant_group_models", 300*time.Second)

// TenantModelAccessCache 缓存租户模型访问权限（enabled + channel_scope），TTL 300s
// 缓存键格式：{tenantID}:{modelName}
// 缓存值：{"enabled": true/false, "channel_scope": [1,2,3]}
var TenantModelAccessCache = NewCache("tenant_model_access", 300*time.Second)
