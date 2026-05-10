package admin

import (
	"context"

	"github.com/qianfree/team-api/api/admin/v1"
	"github.com/qianfree/team-api/internal/service"
)

func (c *ControllerV1) HelpCategoryList(ctx context.Context, req *v1.HelpCategoryListReq) (res *v1.HelpCategoryListRes, err error) {
	return service.Admin().ListHelpCategories(ctx, req)
}
