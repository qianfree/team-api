package middleware

import (
	"context"
	"fmt"
	"net/http"

	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/net/ghttp"

	"github.com/qianfree/team-api/internal/consts"
)

const (
	captchaRateLimitWindowSeconds = 60 // 计数窗口（秒）
	captchaRateLimitMax           = 20 // 每 IP 每窗口最大请求数（generate + verify 合计）
)

// CaptchaRateLimit 验证码端点的 IP 级限流。
//
// /api/captcha/* 无认证（登录/注册前就要用），generate 是 CPU 密集操作（背景图 +
// 拼图 base64），此前无限流，未认证流量可直接打满 CPU 与带宽。计数覆盖该组全部
// 请求（生成 + 校验）：正常登录流程每次消耗 2-4 个请求，20 次/分钟对真人足够宽裕，
// 对脚本化洪水和验证码暴力求解（生成→猜→再生成循环）则构成硬约束。
//
// Redis 不可用时 fail-open：验证码 state 本身依赖 Redis，届时整个验证码流程已不可
// 用，限流器放行不引入额外风险。g.Redis() 在未配置时会 panic（而非返回错误），
// 用 recover 兜底转 fail-open。
func CaptchaRateLimit(r *ghttp.Request) {
	ip := r.GetClientIp()
	if ip == "" {
		r.Middleware.Next()
		return
	}

	blocked := checkCaptchaRate(r.Context(), ip)
	if blocked {
		// 状态码 + JSON 体必须用 WriteHeader：WriteStatus 的参数是 HTTP 状态码，
		// 曾误传 CodeRateLimitExceeded(10014) 导致 net/http panic（invalid WriteHeader code）；
		// 且不带 content 的 WriteStatus 会先把状态文本写进响应体，污染 JSON
		r.Response.WriteHeader(http.StatusTooManyRequests)
		r.Response.WriteJson(g.Map{
			"code":       consts.CodeRateLimitExceeded,
			"message":    consts.MsgRateLimitExceeded,
			"data":       nil,
			"request_id": r.GetCtxVar("RequestId"),
		})
		r.Exit()
		return
	}
	r.Middleware.Next()
}

// checkCaptchaRate 返回该 IP 在当前窗口是否超限。任何内部错误（含 Redis 未配置
// panic）都按未超限处理，保证限流器自身不成为可用性风险。
func checkCaptchaRate(ctx context.Context, ip string) (blocked bool) {
	defer func() {
		if recover() != nil {
			blocked = false
		}
	}()
	// Lua 保证 INCR 与首次 EXPIRE 原子完成（与注册限流 incrementWithExpire 同模式）
	lua := `
local count = redis.call("INCR", KEYS[1])
if count == 1 then
	redis.call("EXPIRE", KEYS[1], ARGV[1])
end
return count
`
	key := fmt.Sprintf("captcha:rl:%s", ip)
	result, err := g.Redis().Do(ctx, "EVAL", lua, 1, key, captchaRateLimitWindowSeconds)
	if err != nil {
		g.Log().Warningf(ctx, "[CaptchaRateLimit] redis failed: %v", err)
		return false
	}
	return result.Int64() > captchaRateLimitMax
}
