package admin

import (
	"context"

	"github.com/qianfree/team-api/api/admin/v1"
	"github.com/qianfree/team-api/internal/service"
)

func (c *ControllerV1) UpdateCheck(ctx context.Context, req *v1.UpdateCheckReq) (res *v1.UpdateCheckRes, err error) {
	return service.Admin().UpdateCheck(ctx, req)
}
func (c *ControllerV1) UpdateStatus(ctx context.Context, req *v1.UpdateStatusReq) (res *v1.UpdateStatusRes, err error) {
	return service.Admin().UpdateStatus(ctx, req)
}
func (c *ControllerV1) UpdateExecute(ctx context.Context, req *v1.UpdateExecuteReq) (res *v1.UpdateExecuteRes, err error) {
	return service.Admin().UpdateExecute(ctx, req)
}
func (c *ControllerV1) UpdateRollback(ctx context.Context, req *v1.UpdateRollbackReq) (res *v1.UpdateRollbackRes, err error) {
	return service.Admin().UpdateRollback(ctx, req)
}
