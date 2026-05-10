package admin

import (
	"context"

	"github.com/qianfree/team-api/api/admin/v1"
	"github.com/qianfree/team-api/internal/service"
)

func (c *ControllerV1) AuditConfigGet(ctx context.Context, req *v1.AuditConfigGetReq) (res *v1.AuditConfigGetRes, err error) {
	return service.Admin().GetAuditConfig(ctx, req)
}
