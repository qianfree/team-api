package admin

import (
	"context"

	"github.com/qianfree/team-api/api/admin/v1"
	"github.com/qianfree/team-api/internal/service"
)

func (c *ControllerV1) AlertRuleUpdate(ctx context.Context, req *v1.AlertRuleUpdateReq) (res *v1.AlertRuleUpdateRes, err error) {
	return service.Monitor().UpdateAlertRule(ctx, req)
}
