package v1

import "github.com/gogf/gf/v2/frame/g"

// === 租户成员批量导入 ===

type TenantImportRecordsReq struct {
	g.Meta   `path:"/members/import-records" method:"get" mime:"json" tags:"租户控制台-成员" summary:"导入记录列表"`
	Page     int `json:"page" in:"query" d:"1"`
	PageSize int `json:"page_size" in:"query" d:"20"`
}

type TenantImportRecordsRes struct {
	List     []map[string]any `json:"list"`
	Total    int              `json:"total"`
	Page     int              `json:"page"`
	PageSize int              `json:"page_size"`
}

type TenantImportRecordGetReq struct {
	g.Meta `path:"/members/import-records/{id}" method:"get" mime:"json" tags:"租户控制台-成员" summary:"导入记录详情"`
	Id     int64 `json:"id" in:"path" v:"required|min:1"`
}

type TenantImportRecordGetRes struct {
	Data map[string]any `json:"data"`
}

type TenantMemberImportReq struct {
	g.Meta `path:"/members/import" method:"post" mime:"json" tags:"租户控制台-成员" summary:"批量导入成员"`
}

type TenantMemberImportRes struct {
	Data map[string]any `json:"data"`
}
