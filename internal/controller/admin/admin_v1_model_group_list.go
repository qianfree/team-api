package admin

import (
	"context"

	"github.com/qianfree/team-api/api/admin/v1"
	"github.com/qianfree/team-api/internal/service"
)

func (c *ControllerV1) ModelGroupList(ctx context.Context, req *v1.ModelGroupListReq) (res *v1.ModelGroupListRes, err error) {
	return service.Admin().ListModelGroups(ctx, req)
}
