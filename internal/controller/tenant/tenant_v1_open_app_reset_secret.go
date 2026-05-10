package tenant

import (
	"context"

	"github.com/qianfree/team-api/api/tenant/v1"
	"github.com/qianfree/team-api/internal/service"
)

func (c *ControllerV1) OpenAppResetSecret(ctx context.Context, req *v1.OpenAppResetSecretReq) (res *v1.OpenAppResetSecretRes, err error) {
	return service.Tenant().OpenAppResetSecret(ctx, req)
}
