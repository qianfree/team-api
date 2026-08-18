package common

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/gogf/gf/v2/database/gredis"
	"github.com/gogf/gf/v2/frame/g"
)

// ResponseRoute response_id → 渠道路由记录，供生命周期端点
// （GET /v1/responses/{id}、POST .../cancel、DELETE）还原原始请求落到的渠道。
// ModelName 存 lookupModel 口径（去 thinking 后缀），与 MaterializeSelection 入参一致。
type ResponseRoute struct {
	ChannelID int64  `json:"channel_id"`
	ModelName string `json:"model_name"`
}

// ResponseRouteStore 路由存储接口（测试可注入 fake）
type ResponseRouteStore interface {
	// Record 记录路由（幂等 SET，重复调用刷新 TTL）。热路径调用，失败只记日志不中断。
	Record(ctx context.Context, tenantID int64, responseID string, route ResponseRoute)
	// Lookup 查询路由；miss 或存储不可用返回 false（调用方按 404 处理）
	Lookup(ctx context.Context, tenantID int64, responseID string) (ResponseRoute, bool)
	// Delete 删除路由（DELETE 生命周期端点成功后调用）
	Delete(ctx context.Context, tenantID int64, responseID string)
}

// DefaultResponseRouteStore 包级单例，adaptor 响应处理与 lifecycle handler 共用
var DefaultResponseRouteStore ResponseRouteStore = &redisResponseRouteStore{}

// responseRouteTTL 路由保留期，与 OpenAI Responses 存储保留期（30 天）对齐；
// 超期后上游本身也不再可 retrieve，路由条目自然过期即可。
const responseRouteTTL = 30 * 24 * time.Hour

// responseRouteKey 路由按租户隔离，防止跨租户探测 response_id
func responseRouteKey(tenantID int64, responseID string) string {
	return fmt.Sprintf("relay:resp:route:%d:%s", tenantID, responseID)
}

// redisResponseRouteStore Redis 实现。Redis 未配置/不可用时全程降级 no-op：
// Record 静默丢弃、Lookup 恒 miss、Delete 静默——生命周期端点退化为"不可用"而非报错。
type redisResponseRouteStore struct{}

var (
	redisProbeOnce sync.Once
	redisClient    *gredis.Redis
)

// redisClientOrNil 返回 Redis 客户端。注意 gf v2 的 g.Redis() 在未配置 redis 时
// 会 panic（gins 懒加载抛错）而非返回 nil，故以 recover 探测并缓存结果（仅探测一次，
// 配置在进程内不动态增删 redis 节点，探测失败后全程按不可用处理）。
func redisClientOrNil() *gredis.Redis {
	redisProbeOnce.Do(func() {
		defer func() {
			// g.Redis() panic → 保持 redisClient = nil
			_ = recover()
		}()
		redisClient = g.Redis()
	})
	return redisClient
}

func (s *redisResponseRouteStore) Record(ctx context.Context, tenantID int64, responseID string, route ResponseRoute) {
	r := redisClientOrNil()
	if r == nil || responseID == "" {
		return
	}
	val, err := json.Marshal(route)
	if err != nil {
		g.Log().Debugf(ctx, "[ResponseRoute] marshal route failed: %v", err)
		return
	}
	if _, err := r.Do(ctx, "SET", responseRouteKey(tenantID, responseID), string(val), "EX", int(responseRouteTTL.Seconds())); err != nil {
		// 路由记录失败不影响响应转发，仅损失后续 retrieve/cancel 能力
		g.Log().Debugf(ctx, "[ResponseRoute] record failed: responseID=%s err=%v", responseID, err)
	}
}

func (s *redisResponseRouteStore) Lookup(ctx context.Context, tenantID int64, responseID string) (ResponseRoute, bool) {
	r := redisClientOrNil()
	if r == nil || responseID == "" {
		return ResponseRoute{}, false
	}
	res, err := r.Do(ctx, "GET", responseRouteKey(tenantID, responseID))
	if err != nil || res.IsNil() {
		return ResponseRoute{}, false
	}
	var route ResponseRoute
	if err := json.Unmarshal([]byte(res.String()), &route); err != nil {
		return ResponseRoute{}, false
	}
	return route, true
}

func (s *redisResponseRouteStore) Delete(ctx context.Context, tenantID int64, responseID string) {
	r := redisClientOrNil()
	if r == nil || responseID == "" {
		return
	}
	if _, err := r.Do(ctx, "DEL", responseRouteKey(tenantID, responseID)); err != nil {
		g.Log().Debugf(ctx, "[ResponseRoute] delete failed: responseID=%s err=%v", responseID, err)
	}
}
