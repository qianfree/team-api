package admin

import (
	"context"

	"github.com/qianfree/team-api/api/admin/v1"
	"github.com/qianfree/team-api/internal/service"
)

func (c *ControllerV1) HelpArticleDelete(ctx context.Context, req *v1.HelpArticleDeleteReq) (res *v1.HelpArticleDeleteRes, err error) {
	return service.Admin().DeleteHelpArticle(ctx, req)
}
