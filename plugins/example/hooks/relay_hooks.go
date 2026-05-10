package hooks

import (
	"context"

	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/os/glog"

	"github.com/qianfree/team-api/internal/plugin"
)

// RelayBeforeRequest 记录 relay 请求日志。
func RelayBeforeRequest(ctx context.Context, payload plugin.HookPayload) (plugin.HookResult, error) {
	glog.Infof(ctx, "[example plugin] relay before request: model=%v", payload.Data["model"])
	return plugin.HookResult{Data: g.Map{}}, nil
}

// RelayAfterResponse 记录 relay 响应日志。
func RelayAfterResponse(ctx context.Context, payload plugin.HookPayload) (plugin.HookResult, error) {
	glog.Info(ctx, "[example plugin] relay after response")
	return plugin.HookResult{Data: g.Map{}}, nil
}
