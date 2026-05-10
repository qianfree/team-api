package admin

import (
	"context"

	"github.com/qianfree/team-api/api/admin/v1"
	"github.com/qianfree/team-api/internal/service"
)

func (c *ControllerV1) AdminRevokeSession(ctx context.Context, req *v1.AdminRevokeSessionReq) (res *v1.AdminRevokeSessionRes, err error) {
	return service.Admin().RevokeSession(ctx, req)
}
