package admin

import (
	"context"

	"github.com/qianfree/team-api/api/admin/v1"
	"github.com/qianfree/team-api/internal/service"
)

func (c *ControllerV1) AdminSettingsGet(ctx context.Context, req *v1.AdminSettingsGetReq) (res *v1.AdminSettingsGetRes, err error) {
	return service.Admin().GetSettings(ctx, req)
}
