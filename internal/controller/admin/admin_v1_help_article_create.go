package admin

import (
	"context"

	"github.com/qianfree/team-api/api/admin/v1"
	"github.com/qianfree/team-api/internal/service"
)

func (c *ControllerV1) HelpArticleCreate(ctx context.Context, req *v1.HelpArticleCreateReq) (res *v1.HelpArticleCreateRes, err error) {
	return service.Admin().CreateHelpArticle(ctx, req)
}
