package middleware

import (
	"strings"

	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/net/ghttp"

	"github.com/qianfree/team-api/internal/consts"
	"github.com/qianfree/team-api/internal/response"
)

var demoWhitelistPaths = map[string]bool{
	"/api/admin/auth/login":       true,
	"/api/admin/auth/refresh":     true,
	"/api/admin/auth/logout":      true,
	"/api/admin/auth/2fa/verify":  true,
	"/api/tenant/auth/login":      true,
	"/api/tenant/auth/refresh":    true,
	"/api/tenant/auth/logout":     true,
	"/api/tenant/auth/register":   true,
	"/api/tenant/auth/2fa/verify": true,
}

// DemoMode blocks all write operations when demo mode is enabled.
// Controlled via config file: demo.enabled = true.
// Read-only requests (GET/HEAD/OPTIONS) and whitelisted paths are allowed.
func DemoMode(r *ghttp.Request) {
	ctx := r.Context()
	if !g.Cfg().MustGet(ctx, "demo.enabled").Bool() {
		r.Middleware.Next()
		return
	}

	method := r.Method
	// Allow read-only requests
	if method == "GET" || method == "HEAD" || method == "OPTIONS" {
		r.Middleware.Next()
		return
	}

	path := r.URL.Path

	// Allow whitelisted paths
	if demoWhitelistPaths[path] {
		r.Middleware.Next()
		return
	}

	// Allow captcha and setup endpoints
	if strings.HasPrefix(path, "/api/captcha/") || strings.HasPrefix(path, "/api/setup/") {
		r.Middleware.Next()
		return
	}

	message := g.Cfg().MustGet(ctx, "demo.message").String()
	if message == "" {
		message = consts.MsgDemoModeRestricted
	}

	response.ErrorWithCode(r, 403, consts.CodeDemoModeRestricted, message)
}
