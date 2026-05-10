package settings

import (
	"context"

	"github.com/qianfree/team-api/api/settings/v1"
	"github.com/qianfree/team-api/internal/service"
)

func (c *ControllerV1) PublicAnnouncements(ctx context.Context, req *v1.PublicAnnouncementsReq) (res *v1.PublicAnnouncementsRes, err error) {
	return service.Settings().PublicAnnouncements(ctx, req)
}
