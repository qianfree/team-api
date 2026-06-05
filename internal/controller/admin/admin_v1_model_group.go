package admin

import (
	"context"

	"github.com/qianfree/team-api/api/admin/v1"
	"github.com/qianfree/team-api/internal/service"
)

func (c *ControllerV1) ModelGroupList(ctx context.Context, req *v1.ModelGroupListReq) (res *v1.ModelGroupListRes, err error) {
	return service.Admin().ListModelGroups(ctx, req)
}
func (c *ControllerV1) ModelGroupCreate(ctx context.Context, req *v1.ModelGroupCreateReq) (res *v1.ModelGroupCreateRes, err error) {
	return service.Admin().CreateModelGroup(ctx, req)
}
func (c *ControllerV1) ModelGroupUpdate(ctx context.Context, req *v1.ModelGroupUpdateReq) (res *v1.ModelGroupUpdateRes, err error) {
	return service.Admin().UpdateModelGroup(ctx, req)
}
func (c *ControllerV1) ModelGroupDelete(ctx context.Context, req *v1.ModelGroupDeleteReq) (res *v1.ModelGroupDeleteRes, err error) {
	return service.Admin().DeleteModelGroup(ctx, req)
}
func (c *ControllerV1) GroupModelsList(ctx context.Context, req *v1.GroupModelsListReq) (res *v1.GroupModelsListRes, err error) {
	return service.Admin().ListGroupModels(ctx, req)
}
func (c *ControllerV1) GroupModelsSet(ctx context.Context, req *v1.GroupModelsSetReq) (res *v1.GroupModelsSetRes, err error) {
	return service.Admin().SetGroupModels(ctx, req)
}
func (c *ControllerV1) TenantGroupsList(ctx context.Context, req *v1.TenantGroupsListReq) (res *v1.TenantGroupsListRes, err error) {
	return service.Admin().ListTenantGroups(ctx, req)
}
func (c *ControllerV1) TenantGroupsSet(ctx context.Context, req *v1.TenantGroupsSetReq) (res *v1.TenantGroupsSetRes, err error) {
	return service.Admin().SetTenantGroups(ctx, req)
}
func (c *ControllerV1) ModelGroupOptions(ctx context.Context, req *v1.ModelGroupOptionsReq) (res *v1.ModelGroupOptionsRes, err error) {
	return service.Admin().ListModelGroupOptions(ctx, req)
}
