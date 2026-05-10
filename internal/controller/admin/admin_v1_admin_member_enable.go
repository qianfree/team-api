package admin

import (
	"context"

	"github.com/qianfree/team-api/api/admin/v1"
	"github.com/qianfree/team-api/internal/service"
)

func (c *ControllerV1) AdminMemberEnable(ctx context.Context, req *v1.AdminMemberEnableReq) (res *v1.AdminMemberEnableRes, err error) {
	return service.Admin().EnableMember(ctx, req)
}
