package admin

import (
	"context"

	"github.com/qianfree/team-api/api/admin/v1"
	"github.com/qianfree/team-api/internal/service"
)

func (c *ControllerV1) AuditConfigGet(ctx context.Context, req *v1.AuditConfigGetReq) (res *v1.AuditConfigGetRes, err error) {
	return service.Admin().GetAuditConfig(ctx, req)
}
func (c *ControllerV1) AuditConfigUpdate(ctx context.Context, req *v1.AuditConfigUpdateReq) (res *v1.AuditConfigUpdateRes, err error) {
	return service.Admin().UpdateAuditConfig(ctx, req)
}
func (c *ControllerV1) OperationLogList(ctx context.Context, req *v1.OperationLogListReq) (res *v1.OperationLogListRes, err error) {
	return service.Admin().ListOperationLogs(ctx, req)
}
func (c *ControllerV1) SensitiveLogList(ctx context.Context, req *v1.SensitiveLogListReq) (res *v1.SensitiveLogListRes, err error) {
	return service.Admin().ListSensitiveAccessLogs(ctx, req)
}
func (c *ControllerV1) RequestAuditLogList(ctx context.Context, req *v1.RequestAuditLogListReq) (res *v1.RequestAuditLogListRes, err error) {
	return service.Admin().ListRequestAuditLogs(ctx, req)
}
func (c *ControllerV1) RequestAuditLogDetail(ctx context.Context, req *v1.RequestAuditLogDetailReq) (res *v1.RequestAuditLogDetailRes, err error) {
	return service.Admin().GetRequestAuditLogDetail(ctx, req)
}
func (c *ControllerV1) OperationLogExport(ctx context.Context, req *v1.OperationLogExportReq) (res *v1.OperationLogExportRes, err error) {
	return service.Admin().ExportOperationLogs(ctx, req)
}
func (c *ControllerV1) ContentFilterLogList(ctx context.Context, req *v1.ContentFilterLogListReq) (res *v1.ContentFilterLogListRes, err error) {
	return service.Admin().ContentFilterLogList(ctx, req)
}
