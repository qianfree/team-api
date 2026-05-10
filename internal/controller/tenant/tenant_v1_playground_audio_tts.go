package tenant

import (
	"context"

	"github.com/qianfree/team-api/api/tenant/v1"
	"github.com/qianfree/team-api/internal/service"
)

func (c *ControllerV1) PlaygroundAudioTTS(ctx context.Context, req *v1.PlaygroundAudioTTSReq) (res *v1.PlaygroundAudioTTSRes, err error) {
	return service.Tenant().PlaygroundAudioTTS(ctx, req)
}
