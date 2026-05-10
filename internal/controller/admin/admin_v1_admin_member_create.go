package admin

import (
	"context"

	"github.com/qianfree/team-api/api/admin/v1"
	"github.com/qianfree/team-api/internal/service"
)

func (c *ControllerV1) AdminMemberCreate(ctx context.Context, req *v1.AdminMemberCreateReq) (res *v1.AdminMemberCreateRes, err error) {
	return service.Admin().CreateMember(ctx, req)
}
