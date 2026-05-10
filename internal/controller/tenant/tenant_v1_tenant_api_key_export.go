package tenant

import (
	"context"

	"github.com/qianfree/team-api/api/tenant/v1"
	"github.com/qianfree/team-api/internal/service"
)

func (c *ControllerV1) TenantApiKeyExport(ctx context.Context, req *v1.TenantApiKeyExportReq) (res *v1.TenantApiKeyExportRes, err error) {
	return service.Tenant().ExportApiKeys(ctx, req)
}
