package middleware

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/net/ghttp"
	"github.com/shirou/gopsutil/v3/disk"
	"github.com/shirou/gopsutil/v3/mem"

	"github.com/qianfree/team-api/internal/logic/common"
)

const (
	// 内存采样缓存窗口：mem.VirtualMemory 在 Linux 上仅读 /proc/meminfo，微秒级，
	// 用短 TTL 兼顾新鲜度与 syscall 频率（环形缓冲每分钟一采，对保护闸门太陈旧）
	resourceGuardMemTTL = 5 * time.Second
	// 磁盘采样缓存窗口：statfs 亚毫秒~毫秒级且占用变化慢，放宽到 30s
	resourceGuardDiskTTL = 30 * time.Second
	// 拒绝响应的 Retry-After（秒）：资源过载恢复快于人工维护，不宜用 maintenance
	// 的 300s；太短（1~2s）则会放大客户端重试风暴
	resourceGuardRetryAfterSecs = 10
	// 拒绝告警节流窗口（秒）：持续超载期间若每请求都打 Warning，日志本身就会拖垮服务器
	resourceGuardLogIntervalSecs = 10
	// 磁盘检测路径：与 internal/logic/monitor/collector.go 的采集口径保持一致。
	// Docker 下为容器 overlay 根文件系统；k8s+lxcfs 部署则天然是 cgroup 视角
	resourceGuardDiskPath = "/"
)

// resourceViolation 记录一次资源超限事实（nil 表示放行）
type resourceViolation struct {
	Kind      string  // "memory" | "disk"
	Current   float64 // 当前占用百分比
	Threshold float64 // 配置的阈值百分比
}

// metricCache 单指标 TTL 缓存：mutex 内联刷新（锁内查过期 + syscall），
// 天然 single-flight，无需引入额外并发库；每个 TTL 窗口至多一次真实读取，
// 排队请求最多阻塞一次 syscall 时长。读失败写负缓存（ok=false 但更新 at），
// 同样享受 TTL，避免故障源被每请求 syscall 轰炸。
type metricCache struct {
	mu  sync.Mutex
	val float64
	ok  bool
	at  time.Time
}

// lookup 返回缓存的采样值；ok=false 表示无可用数据（未读过或读取失败）
func (c *metricCache) lookup(ttl time.Duration, read func() (float64, error)) (float64, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if time.Since(c.at) < ttl {
		return c.val, c.ok
	}
	c.at = time.Now()
	val, err := read()
	if err != nil {
		c.ok = false
		return 0, false
	}
	c.val = val
	c.ok = true
	return val, true
}

var (
	memPercentCache  metricCache
	diskPercentCache metricCache
	// lastGuardLogUnix 拒绝告警节流的时间戳（Unix 秒），atomic CAS 更新
	lastGuardLogUnix int64

	// 以下函数变量供同包测试注入替身（仿 body_limit_test.go 的注入模式），
	// 生产路径读真实系统指标与 sys_options 配置
	readMemPercent = func() (float64, error) {
		vm, err := mem.VirtualMemory()
		if err != nil {
			return 0, err
		}
		return vm.UsedPercent, nil
	}
	readDiskPercent = func() (float64, error) {
		du, err := disk.Usage(resourceGuardDiskPath)
		if err != nil {
			return 0, err
		}
		return du.UsedPercent, nil
	}
	getGuardLimits = func(ctx context.Context) (memLimit int, diskLimit int) {
		memLimit = common.Config().GetInt(ctx, "resource_memory_limit_percent")
		diskLimit = common.Config().GetInt(ctx, "resource_disk_limit_percent")
		return
	}
)

// ResourceGuard 服务器资源保护闸门：内存/磁盘占用超过 sys_options 配置的阈值
// （resource_memory_limit_percent / resource_disk_limit_percent，百分比，0=不启用）
// 时，新进的大模型代理请求在鉴权与预扣费之前直接返回 503 + Retry-After，并按
// 端点协议输出原生错误格式（/v1/messages 走 Claude、/v1beta 走 Gemini、其余走
// OpenAI），避免「请求走到一半失败、用户已被扣费、服务器还被继续拖垮」。
//
// 设计约束：
//   - 检查逻辑必须自包含在本包：internal/logic/monitor 已 import 本包
//     （monitor.go / api_metrics.go），反向引用会构成 import 环，勿把本文件
//     的采样逻辑"下沉"到 monitor 复用 collector；
//   - 指标读取失败 fail-open（与 AcquireConcurrent 的 Redis fail-open 哲学一致），
//     不能因 /proc 不可读把整个网关打入拒绝态；
//   - 阈值经 Config().GetInt 每请求读取（L1 内存缓存命中为 map 读），SetOption
//     自带缓存失效 + Redis pub/sub 广播，改配置秒级生效，无需 OnSettingsChanged；
//   - 已建立的 WebSocket 会话不受影响（本闸门拦在 HTTP 升级握手之前），
//     符合「拦新进请求」的语义。
func ResourceGuard(r *ghttp.Request) {
	memLimit, diskLimit := getGuardLimits(r.Context())
	// 双 0 = 显式关闭，零成本直通
	if memLimit <= 0 && diskLimit <= 0 {
		r.Middleware.Next()
		return
	}

	var violation *resourceViolation
	if memLimit > 0 {
		if used, ok := memPercentCache.lookup(resourceGuardMemTTL, readMemPercent); ok && used > float64(memLimit) {
			violation = &resourceViolation{Kind: "memory", Current: used, Threshold: float64(memLimit)}
		}
	}
	if violation == nil && diskLimit > 0 {
		if used, ok := diskPercentCache.lookup(resourceGuardDiskTTL, readDiskPercent); ok && used > float64(diskLimit) {
			violation = &resourceViolation{Kind: "disk", Current: used, Threshold: float64(diskLimit)}
		}
	}
	if violation == nil {
		r.Middleware.Next()
		return
	}

	logResourceGuardRejection(r, violation)

	// 先排空再响应：客户端此时可能仍在上传，抢先返回并关连接会让响应体在半路
	// 被掐断（body_limit.go 已踩过的坑）。排空量被 MaxBytesReader 封顶在
	// clientMaxBodySize，有界；GET 等无 body 请求为 no-op
	_, _ = io.Copy(io.Discard, r.Body)

	writeResourceGuardError(r, violation)
}

// allowGuardLog 用 CAS 把拒绝告警节流到每个窗口至多一条（多 goroutine 竞争时
// 只有一个成功），持续超载期间避免日志风暴
func allowGuardLog() bool {
	now := time.Now().Unix()
	for {
		last := atomic.LoadInt64(&lastGuardLogUnix)
		if now-last < resourceGuardLogIntervalSecs {
			return false
		}
		if atomic.CompareAndSwapInt64(&lastGuardLogUnix, last, now) {
			return true
		}
	}
}

// logResourceGuardRejection 记录节流后的拒绝告警。具体数值只进日志，
// 不透出给客户端（多租户面，避免泄露服务器负载细节）
func logResourceGuardRejection(r *ghttp.Request, v *resourceViolation) {
	if !allowGuardLog() {
		return
	}
	requestID := ""
	if rv := r.GetCtxVar("RequestId"); rv != nil {
		requestID = rv.String()
	}
	g.Log().Warningf(r.Context(),
		"[ResourceGuard] 服务器资源超限，请求已拒绝: request_id=%s, kind=%s, current=%.1f%%, threshold=%.1f%%, path=%s, client_ip=%s",
		requestID, v.Kind, v.Current, v.Threshold, r.URL.Path, r.GetClientIp())
}

// writeResourceGuardError 按端点协议写出 503 错误响应：
// /v1/messages 系列 → Claude 格式（overloaded_error + x-should-retry，与
// WriteClaudeRelayError 对 5xx 的口径一致）；/v1beta → Gemini 格式（503 →
// UNAVAILABLE，与 WriteGeminiRelayError 的映射一致）；其余（含 /v2 /suno）→
// OpenAI 格式（形状对齐 WriteRelayError）
func writeResourceGuardError(r *ghttp.Request, v *resourceViolation) {
	msg := "服务器内存占用过高，服务暂时不可用，请稍后重试"
	if v.Kind == "disk" {
		msg = "服务器磁盘占用过高，服务暂时不可用，请稍后重试"
	}

	r.Response.Header().Set("Retry-After", strconv.Itoa(resourceGuardRetryAfterSecs))
	r.Response.Header().Set("Content-Type", "application/json")

	var body []byte
	switch {
	case strings.HasPrefix(r.URL.Path, "/v1/messages"):
		r.Response.Header().Set("x-should-retry", "true")
		body, _ = json.Marshal(map[string]any{
			"type": "error",
			"error": map[string]string{
				"type":    "overloaded_error",
				"message": msg,
			},
		})
	case strings.HasPrefix(r.URL.Path, "/v1beta"):
		body, _ = json.Marshal(map[string]any{
			"error": map[string]any{
				"code":    http.StatusServiceUnavailable,
				"message": msg,
				"status":  "UNAVAILABLE",
			},
		})
	default:
		body, _ = json.Marshal(map[string]any{
			"error": map[string]any{
				"message": msg,
				"type":    "server_error",
				"param":   nil,
				"code":    nil,
			},
		})
	}

	r.Response.WriteHeader(http.StatusServiceUnavailable)
	r.Response.Write(body)
}
