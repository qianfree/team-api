package admin

import (
	"context"

	"github.com/qianfree/team-api/api/admin/v1"
	"github.com/qianfree/team-api/internal/service"
)

func (c *ControllerV1) AdminMemberExport(ctx context.Context, req *v1.AdminMemberExportReq) (res *v1.AdminMemberExportRes, err error) {
	return service.Admin().ExportMembers(ctx, req)
}
