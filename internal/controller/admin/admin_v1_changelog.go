package admin

import (
	"context"

	"github.com/qianfree/team-api/api/admin/v1"
	"github.com/qianfree/team-api/internal/service"
)

func (c *ControllerV1) ChangelogCreate(ctx context.Context, req *v1.ChangelogCreateReq) (res *v1.ChangelogCreateRes, err error) {
	return service.Admin().CreateChangelog(ctx, req)
}
func (c *ControllerV1) ChangelogList(ctx context.Context, req *v1.ChangelogListReq) (res *v1.ChangelogListRes, err error) {
	return service.Admin().ListChangelogs(ctx, req)
}
func (c *ControllerV1) ChangelogUpdate(ctx context.Context, req *v1.ChangelogUpdateReq) (res *v1.ChangelogUpdateRes, err error) {
	return service.Admin().UpdateChangelog(ctx, req)
}
func (c *ControllerV1) ChangelogDelete(ctx context.Context, req *v1.ChangelogDeleteReq) (res *v1.ChangelogDeleteRes, err error) {
	return service.Admin().DeleteChangelog(ctx, req)
}
func (c *ControllerV1) ChangelogPublish(ctx context.Context, req *v1.ChangelogPublishReq) (res *v1.ChangelogPublishRes, err error) {
	return service.Admin().PublishChangelog(ctx, req)
}
