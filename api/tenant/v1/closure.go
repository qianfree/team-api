package v1

import "github.com/gogf/gf/v2/frame/g"

// === 生命周期 ===

type TenantRequestClosureReq struct {
	g.Meta `path:"/request-closure" method:"post" mime:"json" tags:"租户控制台-生命周期" summary:"申请关户"`
}

type TenantRequestClosureRes struct{}

type TenantCancelClosureReq struct {
	g.Meta `path:"/cancel-closure" method:"post" mime:"json" tags:"租户控制台-生命周期" summary:"取消关户"`
}

type TenantCancelClosureRes struct{}
