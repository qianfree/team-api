// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package open

import (
	"context"

	"github.com/qianfree/team-api/api/open/v1"
)

type IOpenV1 interface {
	OpenMemberList(ctx context.Context, req *v1.OpenMemberListReq) (res *v1.OpenMemberListRes, err error)
	OpenMemberCreate(ctx context.Context, req *v1.OpenMemberCreateReq) (res *v1.OpenMemberCreateRes, err error)
	OpenMemberUpdate(ctx context.Context, req *v1.OpenMemberUpdateReq) (res *v1.OpenMemberUpdateRes, err error)
	OpenMemberDelete(ctx context.Context, req *v1.OpenMemberDeleteReq) (res *v1.OpenMemberDeleteRes, err error)
	OpenMemberQuota(ctx context.Context, req *v1.OpenMemberQuotaReq) (res *v1.OpenMemberQuotaRes, err error)
	OpenMemberQuotaUpdate(ctx context.Context, req *v1.OpenMemberQuotaUpdateReq) (res *v1.OpenMemberQuotaUpdateRes, err error)
	OpenMemberModels(ctx context.Context, req *v1.OpenMemberModelsReq) (res *v1.OpenMemberModelsRes, err error)
	OpenMemberModelsUpdate(ctx context.Context, req *v1.OpenMemberModelsUpdateReq) (res *v1.OpenMemberModelsUpdateRes, err error)
	OpenKeyList(ctx context.Context, req *v1.OpenKeyListReq) (res *v1.OpenKeyListRes, err error)
	OpenKeyCreate(ctx context.Context, req *v1.OpenKeyCreateReq) (res *v1.OpenKeyCreateRes, err error)
	OpenKeyDelete(ctx context.Context, req *v1.OpenKeyDeleteReq) (res *v1.OpenKeyDeleteRes, err error)
	OpenUsageQuery(ctx context.Context, req *v1.OpenUsageQueryReq) (res *v1.OpenUsageQueryRes, err error)
	OpenBillingQuery(ctx context.Context, req *v1.OpenBillingQueryReq) (res *v1.OpenBillingQueryRes, err error)
	OpenProjectList(ctx context.Context, req *v1.OpenProjectListReq) (res *v1.OpenProjectListRes, err error)
	OpenProjectCreate(ctx context.Context, req *v1.OpenProjectCreateReq) (res *v1.OpenProjectCreateRes, err error)
	OpenProjectGet(ctx context.Context, req *v1.OpenProjectGetReq) (res *v1.OpenProjectGetRes, err error)
	OpenProjectUpdate(ctx context.Context, req *v1.OpenProjectUpdateReq) (res *v1.OpenProjectUpdateRes, err error)
	OpenProjectArchive(ctx context.Context, req *v1.OpenProjectArchiveReq) (res *v1.OpenProjectArchiveRes, err error)
	OpenProjectUnarchive(ctx context.Context, req *v1.OpenProjectUnarchiveReq) (res *v1.OpenProjectUnarchiveRes, err error)
	OpenProjectKeyList(ctx context.Context, req *v1.OpenProjectKeyListReq) (res *v1.OpenProjectKeyListRes, err error)
	OpenProjectKeyCreate(ctx context.Context, req *v1.OpenProjectKeyCreateReq) (res *v1.OpenProjectKeyCreateRes, err error)
	OpenProjectKeyDelete(ctx context.Context, req *v1.OpenProjectKeyDeleteReq) (res *v1.OpenProjectKeyDeleteRes, err error)
	OpenProjectUsageStats(ctx context.Context, req *v1.OpenProjectUsageStatsReq) (res *v1.OpenProjectUsageStatsRes, err error)
	OpenProjectUsageLogs(ctx context.Context, req *v1.OpenProjectUsageLogsReq) (res *v1.OpenProjectUsageLogsRes, err error)
}
