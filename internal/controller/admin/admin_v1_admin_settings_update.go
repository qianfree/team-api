package admin

import (
	"context"

	"github.com/qianfree/team-api/api/admin/v1"
	"github.com/qianfree/team-api/internal/service"
)

func (c *ControllerV1) AdminSettingsUpdate(ctx context.Context, req *v1.AdminSettingsUpdateReq) (res *v1.AdminSettingsUpdateRes, err error) {
	return service.Admin().UpdateSettings(ctx, req)
}
