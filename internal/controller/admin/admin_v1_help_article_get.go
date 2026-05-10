package admin

import (
	"context"

	"github.com/qianfree/team-api/api/admin/v1"
	"github.com/qianfree/team-api/internal/service"
)

func (c *ControllerV1) HelpArticleGet(ctx context.Context, req *v1.HelpArticleGetReq) (res *v1.HelpArticleGetRes, err error) {
	return service.Admin().GetHelpArticle(ctx, req)
}
