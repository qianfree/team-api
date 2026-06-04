package admin

import (
	"context"

	"github.com/qianfree/team-api/api/admin/v1"
	"github.com/qianfree/team-api/internal/service"
)

func (c *ControllerV1) ErrorLogList(ctx context.Context, req *v1.ErrorLogListReq) (res *v1.ErrorLogListRes, err error) {
	return service.Admin().ErrorLogList(ctx, req)
}
func (c *ControllerV1) ErrorLogDetail(ctx context.Context, req *v1.ErrorLogDetailReq) (res *v1.ErrorLogDetailRes, err error) {
	return service.Admin().ErrorLogDetail(ctx, req)
}
func (c *ControllerV1) ErrorLogResolve(ctx context.Context, req *v1.ErrorLogResolveReq) (res *v1.ErrorLogResolveRes, err error) {
	return service.Admin().ErrorLogResolve(ctx, req)
}
func (c *ControllerV1) ErrorLogBatchResolve(ctx context.Context, req *v1.ErrorLogBatchResolveReq) (res *v1.ErrorLogBatchResolveRes, err error) {
	return service.Admin().ErrorLogBatchResolve(ctx, req)
}
func (c *ControllerV1) ErrorLogStats(ctx context.Context, req *v1.ErrorLogStatsReq) (res *v1.ErrorLogStatsRes, err error) {
	return service.Admin().ErrorLogStats(ctx, req)
}
