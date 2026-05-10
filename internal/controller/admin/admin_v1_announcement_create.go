package admin

import (
	"context"

	"github.com/qianfree/team-api/api/admin/v1"
	"github.com/qianfree/team-api/internal/service"
)

func (c *ControllerV1) AnnouncementCreate(ctx context.Context, req *v1.AnnouncementCreateReq) (res *v1.AnnouncementCreateRes, err error) {
	return service.Admin().CreateAnnouncement(ctx, req)
}
