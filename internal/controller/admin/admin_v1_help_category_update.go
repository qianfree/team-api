package admin

import (
	"context"

	"github.com/qianfree/team-api/api/admin/v1"
	"github.com/qianfree/team-api/internal/service"
)

func (c *ControllerV1) HelpCategoryUpdate(ctx context.Context, req *v1.HelpCategoryUpdateReq) (res *v1.HelpCategoryUpdateRes, err error) {
	return service.Admin().UpdateHelpCategory(ctx, req)
}
