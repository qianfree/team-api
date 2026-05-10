package tenant

import (
	"context"

	v1 "github.com/qianfree/team-api/api/tenant/v1"
	"github.com/qianfree/team-api/internal/service"
)

func (c *ControllerV1) OAuthListProviders(ctx context.Context, req *v1.OAuthListProvidersReq) (res *v1.OAuthListProvidersRes, err error) {
	return service.Tenant().ListOAuthProviders(ctx, req)
}
