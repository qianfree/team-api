package admin

import (
	"context"

	"github.com/qianfree/team-api/api/admin/v1"
	"github.com/qianfree/team-api/internal/service"
)

func (c *ControllerV1) AdminSettingsCategories(ctx context.Context, req *v1.AdminSettingsCategoriesReq) (res *v1.AdminSettingsCategoriesRes, err error) {
	return service.Admin().GetSettingsCategories(ctx, req)
}
