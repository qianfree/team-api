package admin

import (
	"context"

	"github.com/qianfree/team-api/api/admin/v1"
	"github.com/qianfree/team-api/internal/service"
)

func (c *ControllerV1) AuditConfigUpdate(ctx context.Context, req *v1.AuditConfigUpdateReq) (res *v1.AuditConfigUpdateRes, err error) {
	return service.Admin().UpdateAuditConfig(ctx, req)
}
