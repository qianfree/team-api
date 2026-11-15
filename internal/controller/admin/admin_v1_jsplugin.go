package admin

import (
	"context"

	"github.com/qianfree/team-api/api/admin/v1"
	"github.com/qianfree/team-api/internal/service"
)

func (c *ControllerV1) JSPluginList(ctx context.Context, req *v1.JSPluginListReq) (res *v1.JSPluginListRes, err error) {
	return service.Admin().JSPluginList(ctx, req)
}

func (c *ControllerV1) JSPluginUpload(ctx context.Context, req *v1.JSPluginUploadReq) (res *v1.JSPluginUploadRes, err error) {
	return service.Admin().JSPluginUpload(ctx, req)
}

func (c *ControllerV1) JSPluginActivate(ctx context.Context, req *v1.JSPluginActivateReq) (res *v1.JSPluginActivateRes, err error) {
	return service.Admin().JSPluginActivate(ctx, req)
}

func (c *ControllerV1) JSPluginEnable(ctx context.Context, req *v1.JSPluginEnableReq) (res *v1.JSPluginEnableRes, err error) {
	return service.Admin().JSPluginEnable(ctx, req)
}

func (c *ControllerV1) JSPluginDelete(ctx context.Context, req *v1.JSPluginDeleteReq) (res *v1.JSPluginDeleteRes, err error) {
	return service.Admin().JSPluginDelete(ctx, req)
}

func (c *ControllerV1) JSPluginRoutes(ctx context.Context, req *v1.JSPluginRoutesReq) (res *v1.JSPluginRoutesRes, err error) {
	return service.Admin().JSPluginRoutes(ctx, req)
}

func (c *ControllerV1) JSPluginMetrics(ctx context.Context, req *v1.JSPluginMetricsReq) (res *v1.JSPluginMetricsRes, err error) {
	return service.Admin().JSPluginMetrics(ctx, req)
}

func (c *ControllerV1) JSPluginSource(ctx context.Context, req *v1.JSPluginSourceReq) (res *v1.JSPluginSourceRes, err error) {
	return service.Admin().JSPluginSource(ctx, req)
}

func (c *ControllerV1) JSPluginDryRun(ctx context.Context, req *v1.JSPluginDryRunReq) (res *v1.JSPluginDryRunRes, err error) {
	return service.Admin().JSPluginDryRun(ctx, req)
}

func (c *ControllerV1) JSPluginTemplate(ctx context.Context, req *v1.JSPluginTemplateReq) (res *v1.JSPluginTemplateRes, err error) {
	return service.Admin().JSPluginTemplate(ctx, req)
}
func (c *ControllerV1) JSPluginOpsStats(ctx context.Context, req *v1.JSPluginOpsStatsReq) (res *v1.JSPluginOpsStatsRes, err error) {
	return service.Admin().JSPluginOpsStats(ctx, req)
}
