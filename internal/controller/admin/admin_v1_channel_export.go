package admin

import (
	"context"

	"github.com/qianfree/team-api/api/admin/v1"
	"github.com/qianfree/team-api/internal/service"
)

func (c *ControllerV1) ChannelExport(ctx context.Context, req *v1.ChannelExportReq) (res *v1.ChannelExportRes, err error) {
	return service.Admin().ExportChannels(ctx, req)
}
