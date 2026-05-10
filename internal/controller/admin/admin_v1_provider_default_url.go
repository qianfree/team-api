package admin

import (
	"context"

	"github.com/qianfree/team-api/api/admin/v1"
	"github.com/qianfree/team-api/internal/service"
)

func (c *ControllerV1) ProviderDefaultURL(ctx context.Context, req *v1.ProviderDefaultURLReq) (res *v1.ProviderDefaultURLRes, err error) {
	return service.Admin().GetProviderDefaultURLs(ctx, req)
}
