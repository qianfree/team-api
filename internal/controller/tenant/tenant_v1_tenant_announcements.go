package tenant

import (
	"context"

	"github.com/qianfree/team-api/api/tenant/v1"
	"github.com/qianfree/team-api/internal/service"
)

func (c *ControllerV1) TenantAnnouncements(ctx context.Context, req *v1.TenantAnnouncementsReq) (res *v1.TenantAnnouncementsRes, err error) {
	return service.Tenant().Announcements(ctx, req)
}
