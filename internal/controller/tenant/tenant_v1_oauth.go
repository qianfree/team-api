package tenant

import (
	"context"

	v1 "github.com/qianfree/team-api/api/tenant/v1"
	"github.com/qianfree/team-api/internal/service"
)

func (c *ControllerV1) OAuthAuthorize(ctx context.Context, req *v1.OAuthAuthorizeReq) (res *v1.OAuthAuthorizeRes, err error) {
	return service.Tenant().GetOAuthAuthorizeURL(ctx, req)
}
func (c *ControllerV1) OAuthCallback(ctx context.Context, req *v1.OAuthCallbackReq) (res *v1.OAuthCallbackRes, err error) {
	return service.Tenant().OAuthCallback(ctx, req)
}
func (c *ControllerV1) OAuthLink(ctx context.Context, req *v1.OAuthLinkReq) (res *v1.OAuthLinkRes, err error) {
	return service.Tenant().LinkOAuth(ctx, req)
}
func (c *ControllerV1) OAuthUnlink(ctx context.Context, req *v1.OAuthUnlinkReq) (res *v1.OAuthUnlinkRes, err error) {
	return service.Tenant().UnlinkOAuth(ctx, req)
}
func (c *ControllerV1) OAuthListProviders(ctx context.Context, req *v1.OAuthListProvidersReq) (res *v1.OAuthListProvidersRes, err error) {
	return service.Tenant().ListOAuthProviders(ctx, req)
}
