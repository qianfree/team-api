package tenant

import (
	"context"

	"github.com/qianfree/team-api/api/tenant/v1"
	"github.com/qianfree/team-api/internal/service"
)

func (c *ControllerV1) SandboxQuota(ctx context.Context, req *v1.SandboxQuotaReq) (res *v1.SandboxQuotaRes, err error) {
	return service.Tenant().SandboxQuota(ctx, req)
}
