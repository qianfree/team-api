package admin

import (
	"context"

	"github.com/qianfree/team-api/api/admin/v1"
	"github.com/qianfree/team-api/internal/service"
)

func (c *ControllerV1) AdminUserCreate(ctx context.Context, req *v1.AdminUserCreateReq) (res *v1.AdminUserCreateRes, err error) {
	return service.Admin().CreateUser(ctx, req)
}
