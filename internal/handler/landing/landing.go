package landing

import (
	"strings"

	"github.com/gogf/gf/v2/net/ghttp"

	"github.com/qianfree/team-api/internal/consts"
	"github.com/qianfree/team-api/internal/packed"
)

// HandleLanding serves the embedded landing page for the backend root path.
// 未内嵌前端（非 embedweb 构建）时访问后端根路径会命中默认 404，这里提供项目介绍页，
// 引导用户从管理后台 / 租户控制台进入，并展示版本号与常用接口入口。
func HandleLanding(r *ghttp.Request) {
	html := strings.Replace(string(packed.LandingHTML), "__VERSION__", consts.Version, 1)
	r.Response.Header().Set("Content-Type", "text/html; charset=utf-8")
	r.Response.Write([]byte(html))
}
