package tenant

import (
	"context"

	"github.com/qianfree/team-api/api/tenant/v1"
	"github.com/qianfree/team-api/internal/service"
)

func (c *ControllerV1) TenantOrderCreate(ctx context.Context, req *v1.TenantOrderCreateReq) (res *v1.TenantOrderCreateRes, err error) {
	return service.Tenant().OrderCreate(ctx, req)
}
func (c *ControllerV1) TenantOrderPay(ctx context.Context, req *v1.TenantOrderPayReq) (res *v1.TenantOrderPayRes, err error) {
	return service.Tenant().OrderPay(ctx, req)
}
func (c *ControllerV1) TenantOrderList(ctx context.Context, req *v1.TenantOrderListReq) (res *v1.TenantOrderListRes, err error) {
	return service.Tenant().OrderList(ctx, req)
}
func (c *ControllerV1) TenantOrderDetail(ctx context.Context, req *v1.TenantOrderDetailReq) (res *v1.TenantOrderDetailRes, err error) {
	return service.Tenant().OrderDetail(ctx, req)
}
func (c *ControllerV1) TenantOrderCancel(ctx context.Context, req *v1.TenantOrderCancelReq) (res *v1.TenantOrderCancelRes, err error) {
	return service.Tenant().OrderCancel(ctx, req)
}
func (c *ControllerV1) TenantRechargeCreate(ctx context.Context, req *v1.TenantRechargeCreateReq) (res *v1.TenantRechargeCreateRes, err error) {
	return service.Tenant().RechargeCreate(ctx, req)
}
func (c *ControllerV1) TenantPaymentInfo(ctx context.Context, req *v1.TenantPaymentInfoReq) (res *v1.TenantPaymentInfoRes, err error) {
	return service.Tenant().PaymentInfo(ctx, req)
}
func (c *ControllerV1) TenantOrderExport(ctx context.Context, req *v1.TenantOrderExportReq) (res *v1.TenantOrderExportRes, err error) {
	return service.Tenant().ExportOrders(ctx, req)
}
