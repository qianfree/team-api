package admin

import (
	"context"

	"github.com/qianfree/team-api/api/admin/v1"
	"github.com/qianfree/team-api/internal/service"
)

func (c *ControllerV1) AlertRuleToggle(ctx context.Context, req *v1.AlertRuleToggleReq) (res *v1.AlertRuleToggleRes, err error) {
	return service.Monitor().ToggleAlertRule(ctx, req)
}
