package admin

import (
	"context"

	"github.com/qianfree/team-api/api/admin/v1"
	"github.com/qianfree/team-api/internal/service"
)

func (c *ControllerV1) TenantSelect(ctx context.Context, req *v1.TenantSelectReq) (res *v1.TenantSelectRes, err error) {
	return service.Admin().TenantSelect(ctx, req)
}
