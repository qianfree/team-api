package v1

import "github.com/gogf/gf/v2/frame/g"

// === 数据治理设置 ===

type DataGovernanceSettingsGetReq struct {
	g.Meta `path:"/data-governance/settings" method:"get" mime:"json" tags:"管理后台-数据治理" summary:"获取数据治理设置"`
}

type DataGovernanceSettingsGetRes struct {
	Data map[string]any `json:"data"`
}

type DataGovernanceSettingsUpdateReq struct {
	g.Meta   `path:"/data-governance/settings" method:"put" mime:"json" tags:"管理后台-数据治理" summary:"更新数据治理设置"`
	Settings map[string]string `json:"settings" v:"required" dc:"设置键值对"`
}

type DataGovernanceSettingsUpdateRes struct{}

// === 数据导出 ===

type DataGovernanceExportReq struct {
	g.Meta   `path:"/data-governance/export" method:"post" mime:"json" tags:"管理后台-数据治理" summary:"请求数据导出"`
	TenantID int64    `json:"tenant_id" v:"required" dc:"租户ID"`
	Scopes   []string `json:"scopes" v:"required" dc:"导出范围: members,usage,billing,logs"`
}

type DataGovernanceExportRes struct {
	TaskID int64 `json:"task_id"`
}

// === 数据删除 ===

type DataGovernanceDeletionReq struct {
	g.Meta   `path:"/data-governance/deletion" method:"post" mime:"json" tags:"管理后台-数据治理" summary:"请求数据删除"`
	TenantID int64  `json:"tenant_id" v:"required" dc:"租户ID"`
	Reason   string `json:"reason" v:"required" dc:"删除原因"`
}

type DataGovernanceDeletionRes struct {
	TaskID int64 `json:"task_id"`
}

// === 手动清理 ===

type DataGovernanceCleanupReq struct {
	g.Meta `path:"/data-governance/cleanup" method:"post" mime:"json" tags:"管理后台-数据治理" summary:"手动触发数据清理"`
}

type DataGovernanceCleanupRes struct {
	Message string `json:"message"`
}
