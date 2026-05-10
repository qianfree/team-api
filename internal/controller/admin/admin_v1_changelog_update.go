package admin

import (
	"context"

	"github.com/qianfree/team-api/api/admin/v1"
	"github.com/qianfree/team-api/internal/service"
)

func (c *ControllerV1) ChangelogUpdate(ctx context.Context, req *v1.ChangelogUpdateReq) (res *v1.ChangelogUpdateRes, err error) {
	return service.Admin().UpdateChangelog(ctx, req)
}
