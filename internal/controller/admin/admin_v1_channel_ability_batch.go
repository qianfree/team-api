package admin

import (
	"context"

	"github.com/qianfree/team-api/api/admin/v1"
	"github.com/qianfree/team-api/internal/service"
)

func (c *ControllerV1) ChannelAbilityBatch(ctx context.Context, req *v1.ChannelAbilityBatchReq) (res *v1.ChannelAbilityBatchRes, err error) {
	return service.Admin().SetChannelAbilities(ctx, req)
}
