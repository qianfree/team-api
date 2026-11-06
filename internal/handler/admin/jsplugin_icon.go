package admin

import (
	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/net/ghttp"

	"github.com/qianfree/team-api/internal/jsplugin"
)

// HandleJSPluginIcon 插件品牌图标（激活版本，SVG/PNG 二进制直写）。
// 与文件服务同理绕过统一 JSON 响应包装（路由注册在 cmd.go 的独立组）；
// 前端仅以 <img> 渲染本端点输出，杜绝 SVG 注入面。
func HandleJSPluginIcon(r *ghttp.Request) {
	ctx := r.Context()
	key := r.Get("key").String()
	if key == "" {
		r.Response.WriteStatus(400)
		return
	}
	dataURI, err := jsplugin.DefaultRegistry().GetPluginIcon(ctx, key)
	if err != nil || dataURI == "" {
		r.Response.WriteStatus(404)
		return
	}
	mimeType, data, err := jsplugin.DecodeIconDataURI(dataURI)
	if err != nil {
		g.Log().Warningf(ctx, "jsplugin icon decode %s: %v", key, err)
		r.Response.WriteStatus(404)
		return
	}
	// 静态内容 + 仅 <img> 渲染：长缓存，插件升级换图标由 URL 版本参数破缓存
	r.Response.Header().Set("Content-Type", mimeType)
	r.Response.Header().Set("Cache-Control", "private, max-age=300")
	r.Response.WriteStatus(200)
	r.Response.Write(data)
}
