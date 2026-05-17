package admin

import (
	"context"

	v1 "github.com/qianfree/team-api/api/admin/v1"
	"github.com/qianfree/team-api/internal/service"
)

func (c *ControllerV1) ContentFilterLogList(ctx context.Context, req *v1.ContentFilterLogListReq) (res *v1.ContentFilterLogListRes, err error) {
	return service.Admin().ContentFilterLogList(ctx, req)
}
