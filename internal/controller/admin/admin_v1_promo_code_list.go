package admin

import (
	"context"

	"github.com/qianfree/team-api/api/admin/v1"
	"github.com/qianfree/team-api/internal/service"
)

func (c *ControllerV1) PromoCodeList(ctx context.Context, req *v1.PromoCodeListReq) (res *v1.PromoCodeListRes, err error) {
	return service.Admin().ListPromoCodes(ctx, req)
}
