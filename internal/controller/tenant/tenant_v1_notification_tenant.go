package tenant

import (
	"context"

	"github.com/qianfree/team-api/api/tenant/v1"
	"github.com/qianfree/team-api/internal/service"
)

func (c *ControllerV1) TenantNotifications(ctx context.Context, req *v1.TenantNotificationsReq) (res *v1.TenantNotificationsRes, err error) {
	return service.Tenant().Notifications(ctx, req)
}
func (c *ControllerV1) TenantUnreadCount(ctx context.Context, req *v1.TenantUnreadCountReq) (res *v1.TenantUnreadCountRes, err error) {
	return service.Tenant().UnreadCount(ctx, req)
}
func (c *ControllerV1) TenantMarkRead(ctx context.Context, req *v1.TenantMarkReadReq) (res *v1.TenantMarkReadRes, err error) {
	return service.Tenant().MarkRead(ctx, req)
}
func (c *ControllerV1) TenantMarkAllRead(ctx context.Context, req *v1.TenantMarkAllReadReq) (res *v1.TenantMarkAllReadRes, err error) {
	return service.Tenant().MarkAllRead(ctx, req)
}
func (c *ControllerV1) TenantNotificationDelete(ctx context.Context, req *v1.TenantNotificationDeleteReq) (res *v1.TenantNotificationDeleteRes, err error) {
	return service.Tenant().DeleteNotification(ctx, req)
}
func (c *ControllerV1) TenantNotificationPreferencesGet(ctx context.Context, req *v1.TenantNotificationPreferencesGetReq) (res *v1.TenantNotificationPreferencesGetRes, err error) {
	return service.Tenant().NotificationPreferencesGet(ctx, req)
}
func (c *ControllerV1) TenantNotificationPreferencesUpdate(ctx context.Context, req *v1.TenantNotificationPreferencesUpdateReq) (res *v1.TenantNotificationPreferencesUpdateRes, err error) {
	return service.Tenant().NotificationPreferencesUpdate(ctx, req)
}
func (c *ControllerV1) TenantAnnouncements(ctx context.Context, req *v1.TenantAnnouncementsReq) (res *v1.TenantAnnouncementsRes, err error) {
	return service.Tenant().Announcements(ctx, req)
}
func (c *ControllerV1) TenantNotificationsExport(ctx context.Context, req *v1.TenantNotificationsExportReq) (res *v1.TenantNotificationsExportRes, err error) {
	return service.Tenant().ExportNotifications(ctx, req)
}
