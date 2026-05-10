package open

import (
	"context"

	v1 "github.com/qianfree/team-api/api/open/v1"
	"github.com/qianfree/team-api/internal/service"
)

func (c *ControllerV1) OpenBillingQuery(ctx context.Context, req *v1.OpenBillingQueryReq) (res *v1.OpenBillingQueryRes, err error) {
	return service.Open().OpenBillingQuery(ctx, req)
}
