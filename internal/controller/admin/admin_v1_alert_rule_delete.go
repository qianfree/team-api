package admin

import (
	"context"

	"github.com/qianfree/team-api/api/admin/v1"
	"github.com/qianfree/team-api/internal/service"
)

func (c *ControllerV1) AlertRuleDelete(ctx context.Context, req *v1.AlertRuleDeleteReq) (res *v1.AlertRuleDeleteRes, err error) {
	return service.Monitor().DeleteAlertRule(ctx, req)
}
