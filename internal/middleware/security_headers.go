package middleware

import (
	"github.com/gogf/gf/v2/net/ghttp"
)

// SecurityHeaders 为全部响应（API / SPA / 静态资源）注入基础安全响应头。
//
// 背景：全站此前没有任何 CSP / XFO / nosniff 兜底，XSS 防御完全依赖"页面恰好没有
// 注入点"。这些头是第二道防线，一旦未来出现注入漏洞可显著降低可利用性：
//   - X-Content-Type-Options: nosniff —— 禁止浏览器 MIME 嗅探（配合文件 serve 的
//     inline 白名单，双保险阻断 text/html 同源渲染）
//   - X-Frame-Options: DENY + frame-ancestors 'none' —— 防点击劫持/被嵌入
//   - Referrer-Policy —— 跨站跳转只携带源，不泄露完整 URL
//   - CSP —— 限制脚本/样式/连接来源，内联脚本默认禁止执行
//
// CSP 兼容集（改前须核对两套前端的外部依赖）：
//   - script-src 放行 Turnstile（challenges.cloudflare.com，登录/注册人机验证依赖，
//     两端 index.html 均无内联脚本）；JSON-LD（application/ld+json）为非执行数据块，
//     不受 script-src 限制
//   - style-src 'unsafe-inline'：Vue/组件库动态内联样式的现实需要（Naive/Arco 大量
//     运行时注入 style 属性）
//   - img-src 放行 http(s) 外链与 data:/blob:：公告与 Markdown 内容含外链图床
//   - connect-src 'self'：SSE/axios 全部同源
//   - 后续接入新的第三方脚本时需同步调整此处，否则会被 CSP 静默拦截
func SecurityHeaders(r *ghttp.Request) {
	h := r.Response.Header()
	h.Set("X-Content-Type-Options", "nosniff")
	h.Set("X-Frame-Options", "DENY")
	h.Set("Referrer-Policy", "strict-origin-when-cross-origin")
	h.Set("Content-Security-Policy",
		"default-src 'self'; "+
			"script-src 'self' https://challenges.cloudflare.com; "+
			"style-src 'self' 'unsafe-inline'; "+
			"img-src 'self' data: blob: https: http:; "+
			"connect-src 'self' https://challenges.cloudflare.com; "+
			"font-src 'self' data:; "+
			"worker-src 'self' blob:; "+
			"frame-src https://challenges.cloudflare.com; "+
			"object-src 'none'; "+
			"base-uri 'self'; "+
			"frame-ancestors 'none'")
	// HSTS 仅对 HTTPS 请求下发：本地 http 调试不受影响，https 部署下强制浏览器
	// 后续走 HTTPS（经反向代理时按 X-Forwarded-Proto 判断）
	if r.TLS != nil || r.Header.Get("X-Forwarded-Proto") == "https" {
		h.Set("Strict-Transport-Security", "max-age=31536000")
	}
	r.Middleware.Next()
}
