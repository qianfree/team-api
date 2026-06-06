package tenant

import (
	"context"

	v1 "github.com/qianfree/team-api/api/tenant/v1"
	"github.com/qianfree/team-api/internal/service"
)

func (c *ControllerV1) TenantApiKeyList(ctx context.Context, req *v1.TenantApiKeyListReq) (res *v1.TenantApiKeyListRes, err error) {
	return service.Tenant().ApiKeyList(ctx, req)
}
func (c *ControllerV1) TenantApiKeyCreate(ctx context.Context, req *v1.TenantApiKeyCreateReq) (res *v1.TenantApiKeyCreateRes, err error) {
	return service.Tenant().ApiKeyCreate(ctx, req)
}
func (c *ControllerV1) TenantApiKeyDelete(ctx context.Context, req *v1.TenantApiKeyDeleteReq) (res *v1.TenantApiKeyDeleteRes, err error) {
	return service.Tenant().ApiKeyDelete(ctx, req)
}
func (c *ControllerV1) TenantApiKeyUpdate(ctx context.Context, req *v1.TenantApiKeyUpdateReq) (res *v1.TenantApiKeyUpdateRes, err error) {
	return service.Tenant().ApiKeyUpdate(ctx, req)
}
func (c *ControllerV1) TenantApiKeyUpdateScopes(ctx context.Context, req *v1.TenantApiKeyUpdateScopesReq) (res *v1.TenantApiKeyUpdateScopesRes, err error) {
	return service.Tenant().ApiKeyUpdateScopes(ctx, req)
}
func (c *ControllerV1) TenantApiKeyModelScopes(ctx context.Context, req *v1.TenantApiKeyModelScopesReq) (res *v1.TenantApiKeyModelScopesRes, err error) {
	return service.Tenant().ApiKeyModelScopes(ctx, req)
}
func (c *ControllerV1) TenantApiKeyExport(ctx context.Context, req *v1.TenantApiKeyExportReq) (res *v1.TenantApiKeyExportRes, err error) {
	return service.Tenant().ExportApiKeys(ctx, req)
}
func (c *ControllerV1) TenantApiKeyReveal(ctx context.Context, req *v1.TenantApiKeyRevealReq) (res *v1.TenantApiKeyRevealRes, err error) {
	return service.Tenant().ApiKeyReveal(ctx, req)
}
