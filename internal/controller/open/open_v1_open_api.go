package open

import (
	"context"

	v1 "github.com/qianfree/team-api/api/open/v1"
	"github.com/qianfree/team-api/internal/service"
)

func (c *ControllerV1) OpenMemberList(ctx context.Context, req *v1.OpenMemberListReq) (res *v1.OpenMemberListRes, err error) {
	return service.Open().OpenMemberList(ctx, req)
}
func (c *ControllerV1) OpenMemberCreate(ctx context.Context, req *v1.OpenMemberCreateReq) (res *v1.OpenMemberCreateRes, err error) {
	return service.Open().OpenMemberCreate(ctx, req)
}
func (c *ControllerV1) OpenMemberUpdate(ctx context.Context, req *v1.OpenMemberUpdateReq) (res *v1.OpenMemberUpdateRes, err error) {
	return service.Open().OpenMemberUpdate(ctx, req)
}
func (c *ControllerV1) OpenMemberDelete(ctx context.Context, req *v1.OpenMemberDeleteReq) (res *v1.OpenMemberDeleteRes, err error) {
	return service.Open().OpenMemberDelete(ctx, req)
}
func (c *ControllerV1) OpenMemberQuota(ctx context.Context, req *v1.OpenMemberQuotaReq) (res *v1.OpenMemberQuotaRes, err error) {
	return service.Open().OpenMemberQuota(ctx, req)
}
func (c *ControllerV1) OpenMemberQuotaUpdate(ctx context.Context, req *v1.OpenMemberQuotaUpdateReq) (res *v1.OpenMemberQuotaUpdateRes, err error) {
	return service.Open().OpenMemberQuotaUpdate(ctx, req)
}
func (c *ControllerV1) OpenMemberModels(ctx context.Context, req *v1.OpenMemberModelsReq) (res *v1.OpenMemberModelsRes, err error) {
	return service.Open().OpenMemberModels(ctx, req)
}
func (c *ControllerV1) OpenMemberModelsUpdate(ctx context.Context, req *v1.OpenMemberModelsUpdateReq) (res *v1.OpenMemberModelsUpdateRes, err error) {
	return service.Open().OpenMemberModelsUpdate(ctx, req)
}
func (c *ControllerV1) OpenKeyList(ctx context.Context, req *v1.OpenKeyListReq) (res *v1.OpenKeyListRes, err error) {
	return service.Open().OpenKeyList(ctx, req)
}
func (c *ControllerV1) OpenKeyCreate(ctx context.Context, req *v1.OpenKeyCreateReq) (res *v1.OpenKeyCreateRes, err error) {
	return service.Open().OpenKeyCreate(ctx, req)
}
func (c *ControllerV1) OpenKeyDelete(ctx context.Context, req *v1.OpenKeyDeleteReq) (res *v1.OpenKeyDeleteRes, err error) {
	return service.Open().OpenKeyDelete(ctx, req)
}
func (c *ControllerV1) OpenUsageQuery(ctx context.Context, req *v1.OpenUsageQueryReq) (res *v1.OpenUsageQueryRes, err error) {
	return service.Open().OpenUsageQuery(ctx, req)
}
func (c *ControllerV1) OpenBillingQuery(ctx context.Context, req *v1.OpenBillingQueryReq) (res *v1.OpenBillingQueryRes, err error) {
	return service.Open().OpenBillingQuery(ctx, req)
}
func (c *ControllerV1) OpenProjectList(ctx context.Context, req *v1.OpenProjectListReq) (res *v1.OpenProjectListRes, err error) {
	return service.Open().OpenProjectList(ctx, req)
}
func (c *ControllerV1) OpenProjectCreate(ctx context.Context, req *v1.OpenProjectCreateReq) (res *v1.OpenProjectCreateRes, err error) {
	return service.Open().OpenProjectCreate(ctx, req)
}
func (c *ControllerV1) OpenProjectGet(ctx context.Context, req *v1.OpenProjectGetReq) (res *v1.OpenProjectGetRes, err error) {
	return service.Open().OpenProjectGet(ctx, req)
}
func (c *ControllerV1) OpenProjectUpdate(ctx context.Context, req *v1.OpenProjectUpdateReq) (res *v1.OpenProjectUpdateRes, err error) {
	return service.Open().OpenProjectUpdate(ctx, req)
}
func (c *ControllerV1) OpenProjectArchive(ctx context.Context, req *v1.OpenProjectArchiveReq) (res *v1.OpenProjectArchiveRes, err error) {
	return service.Open().OpenProjectArchive(ctx, req)
}
func (c *ControllerV1) OpenProjectUnarchive(ctx context.Context, req *v1.OpenProjectUnarchiveReq) (res *v1.OpenProjectUnarchiveRes, err error) {
	return service.Open().OpenProjectUnarchive(ctx, req)
}
func (c *ControllerV1) OpenProjectKeyList(ctx context.Context, req *v1.OpenProjectKeyListReq) (res *v1.OpenProjectKeyListRes, err error) {
	return service.Open().OpenProjectKeyList(ctx, req)
}
func (c *ControllerV1) OpenProjectKeyCreate(ctx context.Context, req *v1.OpenProjectKeyCreateReq) (res *v1.OpenProjectKeyCreateRes, err error) {
	return service.Open().OpenProjectKeyCreate(ctx, req)
}
func (c *ControllerV1) OpenProjectKeyDelete(ctx context.Context, req *v1.OpenProjectKeyDeleteReq) (res *v1.OpenProjectKeyDeleteRes, err error) {
	return service.Open().OpenProjectKeyDelete(ctx, req)
}
func (c *ControllerV1) OpenProjectUsageStats(ctx context.Context, req *v1.OpenProjectUsageStatsReq) (res *v1.OpenProjectUsageStatsRes, err error) {
	return service.Open().OpenProjectUsageStats(ctx, req)
}
func (c *ControllerV1) OpenProjectUsageLogs(ctx context.Context, req *v1.OpenProjectUsageLogsReq) (res *v1.OpenProjectUsageLogsRes, err error) {
	return service.Open().OpenProjectUsageLogs(ctx, req)
}
