package admin

import (
	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/net/ghttp"

	"github.com/qianfree/team-api/internal/jsplugin"
)

// HandleJSPluginIcon 插件品牌图标（激活版本，SVG/PNG 二进制直写）。
// 与文件服务同理绕过统一 JSON 响应包装（路由注册在 cmd.go 的独立组）；
// 前端仅以 <img> 渲染本端点输出，杜绝 SVG 注入面。
// 无上传图标的插件（内置插件 manifest.icon 是语义标签，非图片数据）返回生成的
// 字母头像而非 404——图标缺失是常态而非异常，避免插件列表页一进来一串 404。
func HandleJSPluginIcon(r *ghttp.Request) {
	ctx := r.Context()
	key := r.Get("key").String()
	if key == "" {
		r.Response.WriteStatus(400)
		return
	}
	dataURI, err := jsplugin.DefaultRegistry().GetPluginIcon(ctx, key)
	if err != nil {
		// 无激活版本 = 插件不存在，按 404 处理（不生成头像兜底，保持存在性语义）
		r.Response.WriteStatus(404)
		return
	}

	mimeType, data := "image/svg+xml", []byte(nil)
	if dataURI != "" {
		var decErr error
		mimeType, data, decErr = jsplugin.DecodeIconDataURI(dataURI)
		if decErr != nil {
			// 入库数据解码失败：日志留痕并回落头像，不让单个坏图标 404 刷屏
			g.Log().Warningf(ctx, "jsplugin icon decode %s: %v", key, decErr)
			data = nil
		}
	}
	if len(data) == 0 {
		name := key
		if m, mErr := jsplugin.DefaultRegistry().ActiveManifest(ctx, key); mErr == nil && m.Name != "" {
			name = m.Name
		}
		data = jsplugin.LetterAvatarSVG(key, name)
	}
	// 静态内容 + 仅 <img> 渲染：长缓存，插件升级换图标由 URL 版本参数破缓存。
	// nosniff 阻止浏览器对 SVG 做内容嗅探改判（纵深防御，配合上传期白名单扫描）
	r.Response.Header().Set("Content-Type", mimeType)
	r.Response.Header().Set("X-Content-Type-Options", "nosniff")
	r.Response.Header().Set("Cache-Control", "private, max-age=300")
	// 状态码与内容须一次写入：WriteStatus 不带内容会把 "OK" 写进响应体，图标字节被污染
	r.Response.WriteStatus(200, data)
}
