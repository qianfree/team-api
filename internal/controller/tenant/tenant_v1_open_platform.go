package tenant

import (
	"context"

	"github.com/qianfree/team-api/api/tenant/v1"
	"github.com/qianfree/team-api/internal/service"
)

func (c *ControllerV1) OpenAppList(ctx context.Context, req *v1.OpenAppListReq) (res *v1.OpenAppListRes, err error) {
	return service.Tenant().OpenAppList(ctx, req)
}
func (c *ControllerV1) OpenAppCreate(ctx context.Context, req *v1.OpenAppCreateReq) (res *v1.OpenAppCreateRes, err error) {
	return service.Tenant().OpenAppCreate(ctx, req)
}
func (c *ControllerV1) OpenAppUpdate(ctx context.Context, req *v1.OpenAppUpdateReq) (res *v1.OpenAppUpdateRes, err error) {
	return service.Tenant().OpenAppUpdate(ctx, req)
}
func (c *ControllerV1) OpenAppDelete(ctx context.Context, req *v1.OpenAppDeleteReq) (res *v1.OpenAppDeleteRes, err error) {
	return service.Tenant().OpenAppDelete(ctx, req)
}
func (c *ControllerV1) OpenAppResetSecret(ctx context.Context, req *v1.OpenAppResetSecretReq) (res *v1.OpenAppResetSecretRes, err error) {
	return service.Tenant().OpenAppResetSecret(ctx, req)
}
func (c *ControllerV1) OpenAppToggleStatus(ctx context.Context, req *v1.OpenAppToggleStatusReq) (res *v1.OpenAppToggleStatusRes, err error) {
	return service.Tenant().OpenAppToggleStatus(ctx, req)
}
func (c *ControllerV1) WebhookConfigList(ctx context.Context, req *v1.WebhookConfigListReq) (res *v1.WebhookConfigListRes, err error) {
	return service.Tenant().WebhookConfigList(ctx, req)
}
func (c *ControllerV1) WebhookConfigCreate(ctx context.Context, req *v1.WebhookConfigCreateReq) (res *v1.WebhookConfigCreateRes, err error) {
	return service.Tenant().WebhookConfigCreate(ctx, req)
}
func (c *ControllerV1) WebhookConfigUpdate(ctx context.Context, req *v1.WebhookConfigUpdateReq) (res *v1.WebhookConfigUpdateRes, err error) {
	return service.Tenant().WebhookConfigUpdate(ctx, req)
}
func (c *ControllerV1) WebhookConfigDelete(ctx context.Context, req *v1.WebhookConfigDeleteReq) (res *v1.WebhookConfigDeleteRes, err error) {
	return service.Tenant().WebhookConfigDelete(ctx, req)
}
func (c *ControllerV1) WebhookDeliveryLogs(ctx context.Context, req *v1.WebhookDeliveryLogsReq) (res *v1.WebhookDeliveryLogsRes, err error) {
	return service.Tenant().WebhookDeliveryLogs(ctx, req)
}
func (c *ControllerV1) WebhookRetry(ctx context.Context, req *v1.WebhookRetryReq) (res *v1.WebhookRetryRes, err error) {
	return service.Tenant().WebhookRetry(ctx, req)
}
