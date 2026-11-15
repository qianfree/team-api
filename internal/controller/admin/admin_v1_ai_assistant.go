package admin

import (
	"context"

	"github.com/qianfree/team-api/api/admin/v1"
	"github.com/qianfree/team-api/internal/service"
)

func (c *ControllerV1) AiAssistantConfigGet(ctx context.Context, req *v1.AiAssistantConfigGetReq) (res *v1.AiAssistantConfigGetRes, err error) {
	return service.Admin().AiAssistantConfigGet(ctx, req)
}
func (c *ControllerV1) AiAssistantConfigUpdate(ctx context.Context, req *v1.AiAssistantConfigUpdateReq) (res *v1.AiAssistantConfigUpdateRes, err error) {
	return service.Admin().AiAssistantConfigUpdate(ctx, req)
}
func (c *ControllerV1) AiAssistantTest(ctx context.Context, req *v1.AiAssistantTestReq) (res *v1.AiAssistantTestRes, err error) {
	return service.Admin().AiAssistantTest(ctx, req)
}
