package admin

import (
	"context"

	"github.com/qianfree/team-api/api/admin/v1"
	"github.com/qianfree/team-api/internal/service"
)

func (c *ControllerV1) ChannelTest(ctx context.Context, req *v1.ChannelTestReq) (res *v1.ChannelTestRes, err error) {
	return service.Admin().TestChannel(ctx, req)
}
