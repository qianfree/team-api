package admin

import (
	"context"

	"github.com/qianfree/team-api/api/admin/v1"
	"github.com/qianfree/team-api/internal/service"
)

func (c *ControllerV1) HelpArticleList(ctx context.Context, req *v1.HelpArticleListReq) (res *v1.HelpArticleListRes, err error) {
	return service.Admin().ListHelpArticles(ctx, req)
}
