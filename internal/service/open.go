package service

import (
	"context"

	v1 "github.com/qianfree/team-api/api/open/v1"
)

type (
	IOpen interface {
		OpenMemberList(ctx context.Context, req *v1.OpenMemberListReq) (*v1.OpenMemberListRes, error)
		OpenMemberCreate(ctx context.Context, req *v1.OpenMemberCreateReq) (*v1.OpenMemberCreateRes, error)
		OpenMemberUpdate(ctx context.Context, req *v1.OpenMemberUpdateReq) (*v1.OpenMemberUpdateRes, error)
		OpenMemberDelete(ctx context.Context, req *v1.OpenMemberDeleteReq) (*v1.OpenMemberDeleteRes, error)
		OpenMemberQuota(ctx context.Context, req *v1.OpenMemberQuotaReq) (*v1.OpenMemberQuotaRes, error)
		OpenMemberQuotaUpdate(ctx context.Context, req *v1.OpenMemberQuotaUpdateReq) (*v1.OpenMemberQuotaUpdateRes, error)
		OpenMemberModels(ctx context.Context, req *v1.OpenMemberModelsReq) (*v1.OpenMemberModelsRes, error)
		OpenMemberModelsUpdate(ctx context.Context, req *v1.OpenMemberModelsUpdateReq) (*v1.OpenMemberModelsUpdateRes, error)
		OpenKeyList(ctx context.Context, req *v1.OpenKeyListReq) (*v1.OpenKeyListRes, error)
		OpenKeyCreate(ctx context.Context, req *v1.OpenKeyCreateReq) (*v1.OpenKeyCreateRes, error)
		OpenKeyDelete(ctx context.Context, req *v1.OpenKeyDeleteReq) (*v1.OpenKeyDeleteRes, error)
		OpenUsageQuery(ctx context.Context, req *v1.OpenUsageQueryReq) (*v1.OpenUsageQueryRes, error)
		OpenBillingQuery(ctx context.Context, req *v1.OpenBillingQueryReq) (*v1.OpenBillingQueryRes, error)
		OpenProjectList(ctx context.Context, req *v1.OpenProjectListReq) (*v1.OpenProjectListRes, error)
		OpenProjectCreate(ctx context.Context, req *v1.OpenProjectCreateReq) (*v1.OpenProjectCreateRes, error)
		OpenProjectGet(ctx context.Context, req *v1.OpenProjectGetReq) (*v1.OpenProjectGetRes, error)
		OpenProjectUpdate(ctx context.Context, req *v1.OpenProjectUpdateReq) (*v1.OpenProjectUpdateRes, error)
		OpenProjectArchive(ctx context.Context, req *v1.OpenProjectArchiveReq) (*v1.OpenProjectArchiveRes, error)
		OpenProjectUnarchive(ctx context.Context, req *v1.OpenProjectUnarchiveReq) (*v1.OpenProjectUnarchiveRes, error)
		OpenProjectKeyList(ctx context.Context, req *v1.OpenProjectKeyListReq) (*v1.OpenProjectKeyListRes, error)
		OpenProjectKeyCreate(ctx context.Context, req *v1.OpenProjectKeyCreateReq) (*v1.OpenProjectKeyCreateRes, error)
		OpenProjectKeyDelete(ctx context.Context, req *v1.OpenProjectKeyDeleteReq) (*v1.OpenProjectKeyDeleteRes, error)
		OpenProjectUsageStats(ctx context.Context, req *v1.OpenProjectUsageStatsReq) (*v1.OpenProjectUsageStatsRes, error)
		OpenProjectUsageLogs(ctx context.Context, req *v1.OpenProjectUsageLogsReq) (*v1.OpenProjectUsageLogsRes, error)
	}
)

var (
	localOpen IOpen
)

func Open() IOpen {
	if localOpen == nil {
		panic("implement not found for interface IOpen, forgot register?")
	}
	return localOpen
}

func RegisterOpen(i IOpen) {
	localOpen = i
}
