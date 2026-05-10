package admin

import (
	"context"

	"github.com/qianfree/team-api/api/admin/v1"
	"github.com/qianfree/team-api/internal/service"
)

func (c *ControllerV1) HelpCategoryDelete(ctx context.Context, req *v1.HelpCategoryDeleteReq) (res *v1.HelpCategoryDeleteRes, err error) {
	return service.Admin().DeleteHelpCategory(ctx, req)
}
