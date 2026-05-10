package admin

import (
	"context"

	"github.com/qianfree/team-api/api/admin/v1"
	"github.com/qianfree/team-api/internal/service"
)

func (c *ControllerV1) ChangelogList(ctx context.Context, req *v1.ChangelogListReq) (res *v1.ChangelogListRes, err error) {
	return service.Admin().ListChangelogs(ctx, req)
}
