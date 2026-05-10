package admin

import (
	"context"

	"github.com/qianfree/team-api/api/admin/v1"
	"github.com/qianfree/team-api/internal/service"
)

func (c *ControllerV1) TemplateTest(ctx context.Context, req *v1.TemplateTestReq) (res *v1.TemplateTestRes, err error) {
	return service.Admin().TestTemplate(ctx, req)
}
