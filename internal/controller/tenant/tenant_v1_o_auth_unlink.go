package tenant

import (
	"context"

	v1 "github.com/qianfree/team-api/api/tenant/v1"
	"github.com/qianfree/team-api/internal/service"
)

func (c *ControllerV1) OAuthUnlink(ctx context.Context, req *v1.OAuthUnlinkReq) (res *v1.OAuthUnlinkRes, err error) {
	return service.Tenant().UnlinkOAuth(ctx, req)
}
