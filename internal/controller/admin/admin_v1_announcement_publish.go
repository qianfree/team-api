package admin

import (
	"context"

	"github.com/qianfree/team-api/api/admin/v1"
	"github.com/qianfree/team-api/internal/service"
)

func (c *ControllerV1) AnnouncementPublish(ctx context.Context, req *v1.AnnouncementPublishReq) (res *v1.AnnouncementPublishRes, err error) {
	return service.Admin().PublishAnnouncement(ctx, req)
}
