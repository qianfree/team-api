package admin

import (
	"context"

	"github.com/qianfree/team-api/api/admin/v1"
	"github.com/qianfree/team-api/internal/service"
)

func (c *ControllerV1) HelpArticleUpdate(ctx context.Context, req *v1.HelpArticleUpdateReq) (res *v1.HelpArticleUpdateRes, err error) {
	return service.Admin().UpdateHelpArticle(ctx, req)
}
