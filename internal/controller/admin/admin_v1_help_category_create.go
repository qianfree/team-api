package admin

import (
	"context"

	"github.com/qianfree/team-api/api/admin/v1"
	"github.com/qianfree/team-api/internal/service"
)

func (c *ControllerV1) HelpCategoryCreate(ctx context.Context, req *v1.HelpCategoryCreateReq) (res *v1.HelpCategoryCreateRes, err error) {
	return service.Admin().CreateHelpCategory(ctx, req)
}
