package admin

import (
	"context"

	"github.com/qianfree/team-api/api/admin/v1"
	"github.com/qianfree/team-api/internal/service"
)

func (c *ControllerV1) ChannelDebugLogList(ctx context.Context, req *v1.ChannelDebugLogListReq) (res *v1.ChannelDebugLogListRes, err error) {
	return service.Admin().ChannelDebugLogList(ctx, req)
}
func (c *ControllerV1) ChannelDebugLogStats(ctx context.Context, req *v1.ChannelDebugLogStatsReq) (res *v1.ChannelDebugLogStatsRes, err error) {
	return service.Admin().ChannelDebugLogStats(ctx, req)
}
func (c *ControllerV1) ChannelDebugLogDetail(ctx context.Context, req *v1.ChannelDebugLogDetailReq) (res *v1.ChannelDebugLogDetailRes, err error) {
	return service.Admin().ChannelDebugLogDetail(ctx, req)
}
func (c *ControllerV1) ChannelDebugLogDelete(ctx context.Context, req *v1.ChannelDebugLogDeleteReq) (res *v1.ChannelDebugLogDeleteRes, err error) {
	return service.Admin().ChannelDebugLogDelete(ctx, req)
}
func (c *ControllerV1) ChannelDebugLogClear(ctx context.Context, req *v1.ChannelDebugLogClearReq) (res *v1.ChannelDebugLogClearRes, err error) {
	return service.Admin().ChannelDebugLogClear(ctx, req)
}
