package tenant

import (
	"context"

	"github.com/qianfree/team-api/api/tenant/v1"
	"github.com/qianfree/team-api/internal/service"
)

func (c *ControllerV1) TenantTicketCreate(ctx context.Context, req *v1.TenantTicketCreateReq) (res *v1.TenantTicketCreateRes, err error) {
	return service.Tenant().TicketCreate(ctx, req)
}
func (c *ControllerV1) TenantTicketList(ctx context.Context, req *v1.TenantTicketListReq) (res *v1.TenantTicketListRes, err error) {
	return service.Tenant().TicketList(ctx, req)
}
func (c *ControllerV1) TenantTicketGet(ctx context.Context, req *v1.TenantTicketGetReq) (res *v1.TenantTicketGetRes, err error) {
	return service.Tenant().TicketGet(ctx, req)
}
func (c *ControllerV1) TenantTicketReply(ctx context.Context, req *v1.TenantTicketReplyReq) (res *v1.TenantTicketReplyRes, err error) {
	return service.Tenant().TicketReply(ctx, req)
}
func (c *ControllerV1) TenantTicketClose(ctx context.Context, req *v1.TenantTicketCloseReq) (res *v1.TenantTicketCloseRes, err error) {
	return service.Tenant().TicketClose(ctx, req)
}
func (c *ControllerV1) TenantTicketReopen(ctx context.Context, req *v1.TenantTicketReopenReq) (res *v1.TenantTicketReopenRes, err error) {
	return service.Tenant().TicketReopen(ctx, req)
}
func (c *ControllerV1) TenantTicketExport(ctx context.Context, req *v1.TenantTicketExportReq) (res *v1.TenantTicketExportRes, err error) {
	return service.Tenant().ExportTickets(ctx, req)
}
