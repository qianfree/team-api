package admin

import (
	"context"

	"github.com/qianfree/team-api/api/admin/v1"
	"github.com/qianfree/team-api/internal/service"
)

func (c *ControllerV1) SensitiveLogList(ctx context.Context, req *v1.SensitiveLogListReq) (res *v1.SensitiveLogListRes, err error) {
	return service.Admin().ListSensitiveAccessLogs(ctx, req)
}
