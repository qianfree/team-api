package tenant

import (
	"context"

	"github.com/qianfree/team-api/api/tenant/v1"
	"github.com/qianfree/team-api/internal/service"
)

func (c *ControllerV1) TenantOrgInfo(ctx context.Context, req *v1.TenantOrgInfoReq) (res *v1.TenantOrgInfoRes, err error) {
	return service.Tenant().GetOrgInfo(ctx, req)
}
func (c *ControllerV1) TenantOrgUpdate(ctx context.Context, req *v1.TenantOrgUpdateReq) (res *v1.TenantOrgUpdateRes, err error) {
	return service.Tenant().UpdateOrgInfo(ctx, req)
}
func (c *ControllerV1) TenantOrgTransfer(ctx context.Context, req *v1.TenantOrgTransferReq) (res *v1.TenantOrgTransferRes, err error) {
	return service.Tenant().TransferOwnership(ctx, req)
}
func (c *ControllerV1) TenantProfile(ctx context.Context, req *v1.TenantProfileReq) (res *v1.TenantProfileRes, err error) {
	return service.Tenant().GetProfile(ctx, req)
}
func (c *ControllerV1) TenantProfileUpdate(ctx context.Context, req *v1.TenantProfileUpdateReq) (res *v1.TenantProfileUpdateRes, err error) {
	return service.Tenant().UpdateProfile(ctx, req)
}
func (c *ControllerV1) TenantIPWhitelistGet(ctx context.Context, req *v1.TenantIPWhitelistGetReq) (res *v1.TenantIPWhitelistGetRes, err error) {
	return service.Tenant().GetIPWhitelist(ctx, req)
}
func (c *ControllerV1) TenantIPWhitelistUpdate(ctx context.Context, req *v1.TenantIPWhitelistUpdateReq) (res *v1.TenantIPWhitelistUpdateRes, err error) {
	return service.Tenant().UpdateIPWhitelist(ctx, req)
}
