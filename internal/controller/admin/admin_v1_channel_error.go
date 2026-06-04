package admin

import (
	"context"

	"github.com/qianfree/team-api/api/admin/v1"
	"github.com/qianfree/team-api/internal/service"
)

func (c *ControllerV1) ChannelErrorEventList(ctx context.Context, req *v1.ChannelErrorEventListReq) (res *v1.ChannelErrorEventListRes, err error) {
	return service.Admin().ChannelErrorEventList(ctx, req)
}
func (c *ControllerV1) ChannelErrorStats(ctx context.Context, req *v1.ChannelErrorStatsReq) (res *v1.ChannelErrorStatsRes, err error) {
	return service.Admin().ChannelErrorStats(ctx, req)
}
func (c *ControllerV1) ChannelErrorTrend(ctx context.Context, req *v1.ChannelErrorTrendReq) (res *v1.ChannelErrorTrendRes, err error) {
	return service.Admin().ChannelErrorTrend(ctx, req)
}
func (c *ControllerV1) ChannelErrorTopChannels(ctx context.Context, req *v1.ChannelErrorTopChannelsReq) (res *v1.ChannelErrorTopChannelsRes, err error) {
	return service.Admin().ChannelErrorTopChannels(ctx, req)
}
func (c *ControllerV1) ChannelErrorCategories(ctx context.Context, req *v1.ChannelErrorCategoriesReq) (res *v1.ChannelErrorCategoriesRes, err error) {
	return service.Admin().ChannelErrorCategories(ctx, req)
}
