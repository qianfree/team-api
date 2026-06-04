package admin

import (
	"context"

	"github.com/qianfree/team-api/api/admin/v1"
	"github.com/qianfree/team-api/internal/service"
)

func (c *ControllerV1) ChannelList(ctx context.Context, req *v1.ChannelListReq) (res *v1.ChannelListRes, err error) {
	return service.Admin().ListChannels(ctx, req)
}
func (c *ControllerV1) ChannelCreate(ctx context.Context, req *v1.ChannelCreateReq) (res *v1.ChannelCreateRes, err error) {
	return service.Admin().CreateChannel(ctx, req)
}
func (c *ControllerV1) ChannelUpdate(ctx context.Context, req *v1.ChannelUpdateReq) (res *v1.ChannelUpdateRes, err error) {
	return service.Admin().UpdateChannel(ctx, req)
}
func (c *ControllerV1) ChannelDelete(ctx context.Context, req *v1.ChannelDeleteReq) (res *v1.ChannelDeleteRes, err error) {
	return service.Admin().DeleteChannel(ctx, req)
}
func (c *ControllerV1) ChannelDetail(ctx context.Context, req *v1.ChannelDetailReq) (res *v1.ChannelDetailRes, err error) {
	return service.Admin().GetChannelDetail(ctx, req)
}
func (c *ControllerV1) ChannelKeyCreate(ctx context.Context, req *v1.ChannelKeyCreateReq) (res *v1.ChannelKeyCreateRes, err error) {
	return service.Admin().AddChannelKey(ctx, req)
}
func (c *ControllerV1) ChannelKeyDelete(ctx context.Context, req *v1.ChannelKeyDeleteReq) (res *v1.ChannelKeyDeleteRes, err error) {
	return service.Admin().DeleteChannelKey(ctx, req)
}
func (c *ControllerV1) ChannelAbilityBatch(ctx context.Context, req *v1.ChannelAbilityBatchReq) (res *v1.ChannelAbilityBatchRes, err error) {
	return service.Admin().SetChannelAbilities(ctx, req)
}
func (c *ControllerV1) ProviderDefaultURL(ctx context.Context, req *v1.ProviderDefaultURLReq) (res *v1.ProviderDefaultURLRes, err error) {
	return service.Admin().GetProviderDefaultURLs(ctx, req)
}
func (c *ControllerV1) ChannelKeyList(ctx context.Context, req *v1.ChannelKeyListReq) (res *v1.ChannelKeyListRes, err error) {
	return service.Admin().GetChannelKeys(ctx, req)
}
func (c *ControllerV1) ChannelAbilitiesGet(ctx context.Context, req *v1.ChannelAbilitiesGetReq) (res *v1.ChannelAbilitiesGetRes, err error) {
	return service.Admin().GetChannelAbilities(ctx, req)
}
func (c *ControllerV1) ChannelHealthTrend(ctx context.Context, req *v1.ChannelHealthTrendReq) (res *v1.ChannelHealthTrendRes, err error) {
	return service.Admin().GetChannelHealthTrend(ctx, req)
}
func (c *ControllerV1) ChannelExport(ctx context.Context, req *v1.ChannelExportReq) (res *v1.ChannelExportRes, err error) {
	return service.Admin().ExportChannels(ctx, req)
}
func (c *ControllerV1) ChannelClone(ctx context.Context, req *v1.ChannelCloneReq) (res *v1.ChannelCloneRes, err error) {
	return service.Admin().CloneChannel(ctx, req)
}
