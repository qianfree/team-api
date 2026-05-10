package admin

import (
	"context"

	"github.com/qianfree/team-api/api/admin/v1"
	"github.com/qianfree/team-api/internal/service"
)

func (c *ControllerV1) ChangelogPublish(ctx context.Context, req *v1.ChangelogPublishReq) (res *v1.ChangelogPublishRes, err error) {
	return service.Admin().PublishChangelog(ctx, req)
}
