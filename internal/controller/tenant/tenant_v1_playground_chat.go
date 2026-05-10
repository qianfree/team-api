package tenant

import (
	"context"

	"github.com/qianfree/team-api/api/tenant/v1"
	"github.com/qianfree/team-api/internal/service"
)

func (c *ControllerV1) PlaygroundChat(ctx context.Context, req *v1.PlaygroundChatReq) (res *v1.PlaygroundChatRes, err error) {
	return service.Tenant().PlaygroundChat(ctx, req)
}
