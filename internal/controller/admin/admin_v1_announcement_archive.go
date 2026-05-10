package admin

import (
	"context"

	"github.com/qianfree/team-api/api/admin/v1"
	"github.com/qianfree/team-api/internal/service"
)

func (c *ControllerV1) AnnouncementArchive(ctx context.Context, req *v1.AnnouncementArchiveReq) (res *v1.AnnouncementArchiveRes, err error) {
	return service.Admin().ArchiveAnnouncement(ctx, req)
}
