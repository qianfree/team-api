package admin

import (
	"context"

	"github.com/qianfree/team-api/api/admin/v1"
	"github.com/qianfree/team-api/internal/service"
)

func (c *ControllerV1) RequestAuditLogList(ctx context.Context, req *v1.RequestAuditLogListReq) (res *v1.RequestAuditLogListRes, err error) {
	return service.Admin().ListRequestAuditLogs(ctx, req)
}
