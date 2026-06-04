package admin

import (
	"context"

	"github.com/qianfree/team-api/api/admin/v1"
	"github.com/qianfree/team-api/internal/service"
)

func (c *ControllerV1) AdminDashboard(ctx context.Context, req *v1.AdminDashboardReq) (res *v1.AdminDashboardRes, err error) {
	return service.Admin().GetDashboardStats(ctx, req)
}
func (c *ControllerV1) AdminDashboardTrends(ctx context.Context, req *v1.AdminDashboardTrendsReq) (res *v1.AdminDashboardTrendsRes, err error) {
	return service.Admin().GetDashboardTrends(ctx, req)
}
func (c *ControllerV1) AdminDashboardTopTenants(ctx context.Context, req *v1.AdminDashboardTopTenantsReq) (res *v1.AdminDashboardTopTenantsRes, err error) {
	return service.Admin().GetTopTenants(ctx, req)
}
func (c *ControllerV1) AdminDashboardModelDistribution(ctx context.Context, req *v1.AdminDashboardModelDistributionReq) (res *v1.AdminDashboardModelDistributionRes, err error) {
	return service.Admin().GetModelDistribution(ctx, req)
}
func (c *ControllerV1) AdminDashboardChannelHealth(ctx context.Context, req *v1.AdminDashboardChannelHealthReq) (res *v1.AdminDashboardChannelHealthRes, err error) {
	return service.Admin().GetDashboardChannelHealth(ctx, req)
}
func (c *ControllerV1) AdminDashboardRecentAlerts(ctx context.Context, req *v1.AdminDashboardRecentAlertsReq) (res *v1.AdminDashboardRecentAlertsRes, err error) {
	return service.Admin().GetDashboardRecentAlerts(ctx, req)
}
func (c *ControllerV1) AdminUsageLogList(ctx context.Context, req *v1.AdminUsageLogListReq) (res *v1.AdminUsageLogListRes, err error) {
	return service.Admin().GetAllUsageLogs(ctx, req)
}
func (c *ControllerV1) AdminBillingRecordList(ctx context.Context, req *v1.AdminBillingRecordListReq) (res *v1.AdminBillingRecordListRes, err error) {
	return service.Admin().GetAllBillingRecords(ctx, req)
}
func (c *ControllerV1) AdminWalletList(ctx context.Context, req *v1.AdminWalletListReq) (res *v1.AdminWalletListRes, err error) {
	return service.Admin().GetTenantWallets(ctx, req)
}
func (c *ControllerV1) AdminWalletInfo(ctx context.Context, req *v1.AdminWalletInfoReq) (res *v1.AdminWalletInfoRes, err error) {
	return service.Admin().GetWalletInfo(ctx, req)
}
func (c *ControllerV1) AdminWalletAdjust(ctx context.Context, req *v1.AdminWalletAdjustReq) (res *v1.AdminWalletAdjustRes, err error) {
	return service.Admin().AdjustBalance(ctx, req)
}
func (c *ControllerV1) AdminWalletTransactionList(ctx context.Context, req *v1.AdminWalletTransactionListReq) (res *v1.AdminWalletTransactionListRes, err error) {
	return service.Admin().GetWalletTransactions(ctx, req)
}
func (c *ControllerV1) AdminWalletSetWarningThreshold(ctx context.Context, req *v1.AdminWalletSetWarningThresholdReq) (res *v1.AdminWalletSetWarningThresholdRes, err error) {
	return service.Admin().SetWarningThreshold(ctx, req)
}
func (c *ControllerV1) AdminUsageLogExport(ctx context.Context, req *v1.AdminUsageLogExportReq) (res *v1.AdminUsageLogExportRes, err error) {
	return service.Admin().ExportUsageLogs(ctx, req)
}
func (c *ControllerV1) AdminBillingRecordExport(ctx context.Context, req *v1.AdminBillingRecordExportReq) (res *v1.AdminBillingRecordExportRes, err error) {
	return service.Admin().ExportBillingRecords(ctx, req)
}
