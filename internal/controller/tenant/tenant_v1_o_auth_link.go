package tenant

import (
	"context"

	v1 "github.com/qianfree/team-api/api/tenant/v1"
	"github.com/qianfree/team-api/internal/service"
)

func (c *ControllerV1) OAuthLink(ctx context.Context, req *v1.OAuthLinkReq) (res *v1.OAuthLinkRes, err error) {
	return service.Tenant().LinkOAuth(ctx, req)
}
