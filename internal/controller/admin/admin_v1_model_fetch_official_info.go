package admin

import (
	"context"

	"github.com/qianfree/team-api/api/admin/v1"
	"github.com/qianfree/team-api/internal/service"
)

func (c *ControllerV1) ModelFetchOfficialInfo(ctx context.Context, req *v1.ModelFetchOfficialInfoReq) (res *v1.ModelFetchOfficialInfoRes, err error) {
	return service.Admin().FetchOfficialModelInfo(ctx, req)
}
