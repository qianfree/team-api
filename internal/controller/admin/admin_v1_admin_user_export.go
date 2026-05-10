package admin

import (
	"context"

	"github.com/qianfree/team-api/api/admin/v1"
	"github.com/qianfree/team-api/internal/service"
)

func (c *ControllerV1) AdminUserExport(ctx context.Context, req *v1.AdminUserExportReq) (res *v1.AdminUserExportRes, err error) {
	return service.Admin().ExportUsers(ctx, req)
}
