package admin

import (
	"context"

	"github.com/qianfree/team-api/api/admin/v1"
	"github.com/qianfree/team-api/internal/service"
)

func (c *ControllerV1) AdminDataScopeUpdate(ctx context.Context, req *v1.AdminDataScopeUpdateReq) (res *v1.AdminDataScopeUpdateRes, err error) {
	return service.Admin().UpdateUserDataScopes(ctx, req)
}
