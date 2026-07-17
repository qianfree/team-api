package admin

import (
	"context"

	"github.com/qianfree/team-api/api/admin/v1"
	"github.com/qianfree/team-api/internal/service"
)

func (c *ControllerV1) FileList(ctx context.Context, req *v1.FileListReq) (res *v1.FileListRes, err error) {
	return service.Admin().FileList(ctx, req)
}

func (c *ControllerV1) FileStats(ctx context.Context, req *v1.FileStatsReq) (res *v1.FileStatsRes, err error) {
	return service.Admin().FileStats(ctx, req)
}

func (c *ControllerV1) FileDownload(ctx context.Context, req *v1.FileDownloadReq) (res *v1.FileDownloadRes, err error) {
	return service.Admin().FileDownload(ctx, req)
}

func (c *ControllerV1) FileDelete(ctx context.Context, req *v1.FileDeleteReq) (res *v1.FileDeleteRes, err error) {
	return service.Admin().FileDelete(ctx, req)
}

func (c *ControllerV1) FileCleanup(ctx context.Context, req *v1.FileCleanupReq) (res *v1.FileCleanupRes, err error) {
	return service.Admin().FileCleanup(ctx, req)
}
