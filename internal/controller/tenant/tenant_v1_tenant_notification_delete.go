package tenant

import (
	"context"

	"github.com/gogf/gf/v2/errors/gcode"
	"github.com/gogf/gf/v2/errors/gerror"

	"github.com/qianfree/team-api/api/tenant/v1"
)

func (c *ControllerV1) TenantNotificationDelete(ctx context.Context, req *v1.TenantNotificationDeleteReq) (res *v1.TenantNotificationDeleteRes, err error) {
	return nil, gerror.NewCode(gcode.CodeNotImplemented)
}
