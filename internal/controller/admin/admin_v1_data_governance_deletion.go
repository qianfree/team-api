package admin

import (
	"context"

	"github.com/qianfree/team-api/api/admin/v1"
	"github.com/qianfree/team-api/internal/service"
)

func (c *ControllerV1) DataGovernanceDeletion(ctx context.Context, req *v1.DataGovernanceDeletionReq) (res *v1.DataGovernanceDeletionRes, err error) {
	return service.Admin().DataGovernanceDeletion(ctx, req)
}
