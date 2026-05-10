package admin

import (
	"context"

	"github.com/qianfree/team-api/api/admin/v1"
	"github.com/qianfree/team-api/internal/service"
)

func (c *ControllerV1) Admin2FARegenerateBackupCodes(ctx context.Context, req *v1.Admin2FARegenerateBackupCodesReq) (res *v1.Admin2FARegenerateBackupCodesRes, err error) {
	return service.Admin().RegenerateBackupCodes(ctx, req)
}
