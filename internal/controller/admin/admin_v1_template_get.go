package admin

import (
	"context"

	"github.com/qianfree/team-api/api/admin/v1"
	"github.com/qianfree/team-api/internal/service"
)

func (c *ControllerV1) TemplateGet(ctx context.Context, req *v1.TemplateGetReq) (res *v1.TemplateGetRes, err error) {
	return service.Admin().GetTemplate(ctx, req)
}
