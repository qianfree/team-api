package common

import (
	"context"
	"fmt"
	"strings"

	"github.com/gogf/gf/v2/frame/g"
)

// PublicBaseURL 平台后端对外访问基址（无尾斜杠）：site_url（管理后台「基础配置-站点
// 后端地址」）优先，未配置时按当前请求 Host 推断（经反向代理按 X-Forwarded-Proto/Host）。
// 无请求上下文且未配置 site_url 时返回空串（调用方输出相对路径）。
func PublicBaseURL(ctx context.Context) string {
	if base := strings.TrimRight(strings.TrimSpace(Config().GetOption(ctx, "site_url")), "/"); base != "" {
		return base
	}
	r := g.RequestFromCtx(ctx)
	if r == nil {
		return ""
	}
	host := r.GetHeader("X-Forwarded-Host")
	if host == "" {
		host = r.Host
	}
	if host == "" {
		return ""
	}
	scheme := "https"
	switch {
	case r.GetHeader("X-Forwarded-Proto") != "":
		scheme = r.GetHeader("X-Forwarded-Proto")
	case r.TLS == nil:
		scheme = "http"
	}
	return fmt.Sprintf("%s://%s", scheme, host)
}
