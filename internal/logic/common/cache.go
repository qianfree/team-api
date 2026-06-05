package common

import (
	"context"
	"encoding/json"
	"time"

	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/os/gcache"
	"github.com/gogf/gf/v2/util/gconv"
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
	fullKey := c.fullKey(key)

	// L1: memory cache (native Go struct storage)
	gcache.Set(ctx, fullKey, value, expire)

	// L2: Redis cache (JSON serialized)
	ttlSeconds := int64(expire.Seconds())
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
				gcache.Set(ctx, fullKey, raw, c.ttl)
				return raw, true
			}
			// Fallback: treat as plain string (backward compat)
			gcache.Set(ctx, fullKey, jsonStr, c.ttl)
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
			// Backfill L1
			gcache.Set(ctx, fullKey, target, c.ttl)
			return true
		}
	}

	return false
}

// GetOrSet retrieves a value or calls fn to set it if missing.
func (c *Cache) GetOrSet(ctx context.Context, key string, fn func(ctx context.Context) (any, error)) (any, error) {
	if val, ok := c.Get(ctx, key); ok {
		return val, nil
	}

	val, err := fn(ctx)
	if err != nil {
		return nil, err
	}

	c.Set(ctx, key, val)
	return val, nil
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
func (c *Cache) DeleteByPattern(ctx context.Context, pattern string) {
	fullPattern := c.fullKey(pattern)

	// L2: Redis — SCAN matching keys and DEL them
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
		keys := gconv.Strings(slice[1])
		if len(keys) > 0 {
			delArgs := make([]any, len(keys))
			for i, k := range keys {
				delArgs[i] = k
			}
			_, _ = g.Redis().Do(ctx, "DEL", delArgs...)
		}
		if cursor == 0 {
			break
		}
	}

	// L1: memory cache — gcache doesn't support pattern delete,
	// but entries will expire via TTL anyway
}

// PublishInvalidation publishes a cache invalidation message via Redis Pub/Sub.
func PublishInvalidation(ctx context.Context, fullKey string) {
	_, _ = g.Redis().Do(ctx, "PUBLISH", "cache:invalidate", fullKey)
}

// TenantGroupModelCache 缓存租户通过分组可访问的模型集合，TTL 300s
var TenantGroupModelCache = NewCache("tenant_group_models", 300*time.Second)
