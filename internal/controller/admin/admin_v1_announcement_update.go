package admin

import (
	"context"

	"github.com/qianfree/team-api/api/admin/v1"
	"github.com/qianfree/team-api/internal/service"
)

func (c *ControllerV1) AnnouncementUpdate(ctx context.Context, req *v1.AnnouncementUpdateReq) (res *v1.AnnouncementUpdateRes, err error) {
	return service.Admin().UpdateAnnouncement(ctx, req)
}
