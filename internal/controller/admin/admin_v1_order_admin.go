package admin

import (
	"context"

	"github.com/qianfree/team-api/api/admin/v1"
	"github.com/qianfree/team-api/internal/service"
)

func (c *ControllerV1) OrderList(ctx context.Context, req *v1.OrderListReq) (res *v1.OrderListRes, err error) {
	return service.Admin().ListOrders(ctx, req)
}
func (c *ControllerV1) OrderDetail(ctx context.Context, req *v1.OrderDetailReq) (res *v1.OrderDetailRes, err error) {
	return service.Admin().GetOrder(ctx, req)
}
func (c *ControllerV1) OrderRefund(ctx context.Context, req *v1.OrderRefundReq) (res *v1.OrderRefundRes, err error) {
	return service.Admin().RefundOrder(ctx, req)
}
func (c *ControllerV1) OrderComplete(ctx context.Context, req *v1.OrderCompleteReq) (res *v1.OrderCompleteRes, err error) {
	return service.Admin().OrderComplete(ctx, req)
}
func (c *ControllerV1) OrderExport(ctx context.Context, req *v1.OrderExportReq) (res *v1.OrderExportRes, err error) {
	return service.Admin().ExportOrders(ctx, req)
}
