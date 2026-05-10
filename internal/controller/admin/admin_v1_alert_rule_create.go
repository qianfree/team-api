package admin

import (
	"context"

	"github.com/qianfree/team-api/api/admin/v1"
	"github.com/qianfree/team-api/internal/service"
)

func (c *ControllerV1) AlertRuleCreate(ctx context.Context, req *v1.AlertRuleCreateReq) (res *v1.AlertRuleCreateRes, err error) {
	return service.Monitor().CreateAlertRule(ctx, req)
}
