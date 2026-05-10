package admin

import (
	"context"

	"github.com/qianfree/team-api/api/admin/v1"
	"github.com/qianfree/team-api/internal/service"
)

func (c *ControllerV1) OperationLogList(ctx context.Context, req *v1.OperationLogListReq) (res *v1.OperationLogListRes, err error) {
	return service.Admin().ListOperationLogs(ctx, req)
}
