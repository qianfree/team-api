package admin

import (
	"context"

	"github.com/qianfree/team-api/api/admin/v1"
	"github.com/qianfree/team-api/internal/service"
)

func (c *ControllerV1) RequestAuditLogDetail(ctx context.Context, req *v1.RequestAuditLogDetailReq) (res *v1.RequestAuditLogDetailRes, err error) {
	return service.Admin().GetRequestAuditLogDetail(ctx, req)
}
