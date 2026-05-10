package tenant

import (
	"context"

	"github.com/qianfree/team-api/api/tenant/v1"
	"github.com/qianfree/team-api/internal/service"
)

func (c *ControllerV1) Tenant2FARegenerateBackupCodes(ctx context.Context, req *v1.Tenant2FARegenerateBackupCodesReq) (res *v1.Tenant2FARegenerateBackupCodesRes, err error) {
	return service.Tenant().RegenerateBackupCodes(ctx, req)
}
