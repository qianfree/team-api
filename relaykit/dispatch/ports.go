package dispatch

import (
	"context"
	"time"
)

// CatalogPort 渠道目录端口：返回某租户 + 模型的候选渠道快照（含最近健康/负载读值）。
// 实现方保证 O(1) 内存读取，禁止在调用路径中查库（消除 P1）。
type CatalogPort interface {
	Snapshot(ctx context.Context, tenantID int64, model string, scope []int64) []Channel
}

// StatePort 运行时状态端口：绑定 / 健康上报 / 探测令牌 / 容量租约 / 凭证冷却。
// 实现方保证多副本原子性（Redis Lua，阶段 1 实现）。
type StatePort interface {
	// 绑定（会话粘性）。失败不删绑定（A3），只有 TTL 过期或守卫不合格触发重绑。
	GetBinding(ctx context.Context, sessionKey string) (channelID int64, ok bool)
	SetBinding(ctx context.Context, sessionKey string, channelID int64, ttl time.Duration)
	TouchBinding(ctx context.Context, sessionKey string, ttl time.Duration)
	InvalidateChannelBindings(ctx context.Context, channelID int64)

	// 健康上报：fire-and-forget，实现方内部异步化（消除 H2）。
	ReportOutcome(o Outcome)

	// 熔断探测令牌：HALF_OPEN 时每窗口全局只放行一个真实请求。
	TryProbeToken(ctx context.Context, channelID int64) bool

	// 容量租约（多副本安全，实例崩溃后自愈）。
	AcquireLease(ctx context.Context, channelID int64, softLimit int, requestID string) bool
	RefreshLease(ctx context.Context, channelID int64, requestID string)
	ReleaseLease(ctx context.Context, channelID int64, requestID string)

	// 凭证冷却（修订 R1）。
	IsCredentialCooled(ctx context.Context, keyID int64) bool
	CoolCredential(ctx context.Context, keyID int64, ttl time.Duration)
}

// Clock 时钟端口，便于测试注入假时钟。
type Clock interface {
	Now() time.Time
}

// Entropy 熵源：返回 [0,1) 均匀随机数，用于退避 jitter。测试注入固定值即可复现。
type Entropy func() float64

// SystemClock 真实时钟。
type SystemClock struct{}

// Now 返回当前时间。
func (SystemClock) Now() time.Time { return time.Now() }
