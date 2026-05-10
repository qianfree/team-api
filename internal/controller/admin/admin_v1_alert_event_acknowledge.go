package admin

import (
	"context"

	"github.com/qianfree/team-api/api/admin/v1"
	"github.com/qianfree/team-api/internal/service"
)

func (c *ControllerV1) AlertEventAcknowledge(ctx context.Context, req *v1.AlertEventAcknowledgeReq) (res *v1.AlertEventAcknowledgeRes, err error) {
	return service.Monitor().AcknowledgeAlert(ctx, req)
}
