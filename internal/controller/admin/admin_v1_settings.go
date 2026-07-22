package admin

import (
	"context"

	"github.com/qianfree/team-api/api/admin/v1"
	"github.com/qianfree/team-api/internal/service"
)

func (c *ControllerV1) AdminSettingsCategories(ctx context.Context, req *v1.AdminSettingsCategoriesReq) (res *v1.AdminSettingsCategoriesRes, err error) {
	return service.Admin().GetSettingsCategories(ctx, req)
}
func (c *ControllerV1) AdminSettingsGet(ctx context.Context, req *v1.AdminSettingsGetReq) (res *v1.AdminSettingsGetRes, err error) {
	return service.Admin().GetSettings(ctx, req)
}
func (c *ControllerV1) AdminSettingsUpdate(ctx context.Context, req *v1.AdminSettingsUpdateReq) (res *v1.AdminSettingsUpdateRes, err error) {
	return service.Admin().UpdateSettings(ctx, req)
}
func (c *ControllerV1) AdminStorageTest(ctx context.Context, req *v1.AdminStorageTestReq) (res *v1.AdminStorageTestRes, err error) {
	return service.Admin().TestStorageConfig(ctx, req)
}
