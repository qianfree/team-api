package tenant

import (
	"context"

	"github.com/qianfree/team-api/api/tenant/v1"
	"github.com/qianfree/team-api/internal/service"
)

func (c *ControllerV1) SandboxChat(ctx context.Context, req *v1.SandboxChatReq) (res *v1.SandboxChatRes, err error) {
	return service.Tenant().SandboxChat(ctx, req)
}
