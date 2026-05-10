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
}
