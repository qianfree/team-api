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
