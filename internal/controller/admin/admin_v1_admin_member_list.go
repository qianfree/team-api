package admin

import (
	"context"

	"github.com/qianfree/team-api/api/admin/v1"
	"github.com/qianfree/team-api/internal/service"
)

func (c *ControllerV1) AdminMemberList(ctx context.Context, req *v1.AdminMemberListReq) (res *v1.AdminMemberListRes, err error) {
	return service.Admin().ListAllMembers(ctx, req)
}
