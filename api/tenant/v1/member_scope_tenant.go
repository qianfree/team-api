package v1

import "github.com/gogf/gf/v2/frame/g"

// === 租户成员模型范围 ===

type TenantMemberModelScopesReq struct {
	g.Meta `path:"/members/{id}/model-scopes" method:"get" mime:"json" tags:"租户控制台-成员" summary:"成员模型范围"`
	Id     int64 `json:"id" in:"path" v:"required|min:1"`
}

type TenantMemberModelScopesRes struct {
	ModelIDs []int64 `json:"model_ids"`
}

type TenantMemberModelScopesSetReq struct {
	g.Meta   `path:"/members/{id}/model-scopes" method:"put" mime:"json" tags:"租户控制台-成员" summary:"设置成员模型范围"`
	Id       int64   `json:"id" in:"path" v:"required|min:1"`
	ModelIDs []int64 `json:"model_ids"`
}

type TenantMemberModelScopesSetRes struct{}
