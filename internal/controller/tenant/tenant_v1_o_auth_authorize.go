package tenant

import (
	"context"

	v1 "github.com/qianfree/team-api/api/tenant/v1"
	"github.com/qianfree/team-api/internal/service"
)

func (c *ControllerV1) OAuthAuthorize(ctx context.Context, req *v1.OAuthAuthorizeReq) (res *v1.OAuthAuthorizeRes, err error) {
	return service.Tenant().GetOAuthAuthorizeURL(ctx, req)
}
