package admin

import (
	"context"

	"github.com/qianfree/team-api/api/admin/v1"
	"github.com/qianfree/team-api/internal/service"
)

func (c *ControllerV1) TenantCreate(ctx context.Context, req *v1.TenantCreateReq) (res *v1.TenantCreateRes, err error) {
	return service.Admin().CreateTenant(ctx, req)
}
func (c *ControllerV1) TenantList(ctx context.Context, req *v1.TenantListReq) (res *v1.TenantListRes, err error) {
	return service.Admin().ListTenants(ctx, req)
}
func (c *ControllerV1) TenantGet(ctx context.Context, req *v1.TenantGetReq) (res *v1.TenantGetRes, err error) {
	return service.Admin().GetTenant(ctx, req)
}
func (c *ControllerV1) TenantChannelScopeUpdate(ctx context.Context, req *v1.TenantChannelScopeUpdateReq) (res *v1.TenantChannelScopeUpdateRes, err error) {
	return service.Admin().UpdateTenantChannelScope(ctx, req)
}
func (c *ControllerV1) TenantUpdateStatus(ctx context.Context, req *v1.TenantUpdateStatusReq) (res *v1.TenantUpdateStatusRes, err error) {
	return service.Admin().UpdateTenantStatus(ctx, req)
}
func (c *ControllerV1) TenantUpdate(ctx context.Context, req *v1.TenantUpdateReq) (res *v1.TenantUpdateRes, err error) {
	return service.Admin().UpdateTenant(ctx, req)
}
func (c *ControllerV1) AdminMemberList(ctx context.Context, req *v1.AdminMemberListReq) (res *v1.AdminMemberListRes, err error) {
	return service.Admin().ListAllMembers(ctx, req)
}
func (c *ControllerV1) TenantExport(ctx context.Context, req *v1.TenantExportReq) (res *v1.TenantExportRes, err error) {
	return service.Admin().ExportTenants(ctx, req)
}
func (c *ControllerV1) AdminMemberExport(ctx context.Context, req *v1.AdminMemberExportReq) (res *v1.AdminMemberExportRes, err error) {
	return service.Admin().ExportMembers(ctx, req)
}
func (c *ControllerV1) TenantSelect(ctx context.Context, req *v1.TenantSelectReq) (res *v1.TenantSelectRes, err error) {
	return service.Admin().TenantSelect(ctx, req)
}
