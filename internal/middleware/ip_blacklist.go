package middleware

import (
	"encoding/json"
	"net"
	"net/http"
	"strings"
	"sync/atomic"

	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/net/ghttp"

	"github.com/qianfree/team-api/internal/consts"
	"github.com/qianfree/team-api/internal/logic/common"
)

// ipBlacklistMatcher 按配置原始串编译好的黑名单条目快照。
// 配置经 ConfigService 的 L1 内存缓存读取本身够快，但 JSON 反解析不值得每请求重复，
// 以原始串为版本号做原子快照：配置变更（Pub/Sub 失效缓存）后的首个请求重新编译。
type ipBlacklistMatcher struct {
	raw     string
	entries []common.IpBlacklistEntry
}

var ipBlacklistMatcherSnapshot atomic.Value

// loadIpBlacklistMatcher 取黑名单快照，配置串未变化时零开销复用。
// 并发下可能重复编译一次，原子覆盖无锁竞争，结果一致，无需消重。
func loadIpBlacklistMatcher(raw string) *ipBlacklistMatcher {
	if m, ok := ipBlacklistMatcherSnapshot.Load().(*ipBlacklistMatcher); ok && m.raw == raw {
		return m
	}
	m := &ipBlacklistMatcher{raw: raw, entries: common.ParseIpBlacklistEntries(raw)}
	ipBlacklistMatcherSnapshot.Store(m)
	return m
}

func (m *ipBlacklistMatcher) match(ip net.IP) bool {
	if ip == nil || len(m.entries) == 0 {
		return false
	}
	for i := range m.entries {
		if m.entries[i].Contains(ip) {
			return true
		}
	}
	return false
}

// IpBlacklist 全局中间件：命中黑名单 IP 的请求在任何业务路由之前直接拒绝（403）。
// 挂在 Recovery/RequestId/ServiceName 之后（保证 panic 兜底与 request_id 就绪）、
// setup 守卫与所有路由组之前，是请求进入业务链路的第一道闸门。
// /api/health 豁免：存活探针来自 LB/编排器，探针 IP 一旦被误拉黑会引发实例被
// 连锁摘流的级联故障，而健康端点本身无业务价值，不值得拦。
func IpBlacklist(r *ghttp.Request) {
	if r.URL.Path == "/api/health" {
		r.Middleware.Next()
		return
	}

	ctx := r.Context()
	if !common.Config().GetBool(ctx, common.OptionKeyIpBlacklistEnabled) {
		r.Middleware.Next()
		return
	}

	m := loadIpBlacklistMatcher(common.Config().GetString(ctx, common.OptionKeyIpBlacklistList))
	// GetClientIp 与全站限流/审计同一信任模型（取 X-Forwarded-For 等代理头），
	// 部署必须保证代理层覆写这些头，否则客户端可伪造绕过
	ipText := r.GetClientIp()
	if !m.match(net.ParseIP(ipText)) {
		r.Middleware.Next()
		return
	}

	// 拦截计数 best-effort：决策已定，统计失败不影响响应
	common.IncrIpBlacklistBlocked(ctx, ipText)
	writeIpBlacklistedResponse(r)
}

// writeIpBlacklistedResponse 按端点协议返回拒绝响应：
// AI 代理端点（/v1 /v1beta /v2 /suno）保持供应商原生错误格式（/v1/messages 用 Claude 格式，
// 其余用 OpenAI 格式），管理类端点走平台统一 JSON 格式——与 ApiMaintenance 的分流方式一致。
func writeIpBlacklistedResponse(r *ghttp.Request) {
	const message = "IP 已被列入黑名单，禁止访问"
	path := r.URL.Path

	r.Response.Header().Set("Content-Type", "application/json")
	r.Response.WriteHeader(http.StatusForbidden)

	switch {
	case strings.HasPrefix(path, "/v1/messages"):
		body, _ := json.Marshal(map[string]any{
			"type":  "error",
			"error": map[string]string{"type": "forbidden", "message": message},
		})
		r.Response.Write(body)
	case strings.HasPrefix(path, "/v1/") || strings.HasPrefix(path, "/v1beta/") ||
		strings.HasPrefix(path, "/v2/") || strings.HasPrefix(path, "/suno/"):
		body, _ := json.Marshal(map[string]any{
			"error": map[string]string{"type": "forbidden", "message": message},
		})
		r.Response.Write(body)
	default:
		r.Response.WriteJson(g.Map{
			"code":       consts.CodeForbidden,
			"message":    message,
			"data":       nil,
			"request_id": r.GetCtxVar("RequestId"),
		})
	}
	r.Exit()
}
