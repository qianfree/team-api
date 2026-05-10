package admin

import (
	"context"

	"github.com/qianfree/team-api/api/admin/v1"
	"github.com/qianfree/team-api/internal/service"
)

func (c *ControllerV1) AdminBillingRecordList(ctx context.Context, req *v1.AdminBillingRecordListReq) (res *v1.AdminBillingRecordListRes, err error) {
	return service.Admin().GetAllBillingRecords(ctx, req)
}
