package admin

import (
	"context"

	"github.com/qianfree/team-api/api/admin/v1"
	"github.com/qianfree/team-api/internal/service"
)

func (c *ControllerV1) PluginDetail(ctx context.Context, req *v1.PluginDetailReq) (res *v1.PluginDetailRes, err error) {
	return service.Admin().PluginDetail(ctx, req)
}
