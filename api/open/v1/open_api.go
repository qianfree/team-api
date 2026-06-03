package v1

import "github.com/gogf/gf/v2/frame/g"

// ============================================================
// 开放平台业务 API（第三方应用调用）
// ============================================================

// OpenMemberList 成员列表
type OpenMemberListReq struct {
	g.Meta   `path:"/v1/members" method:"get" tags:"开放平台API" summary:"成员列表"`
	Page     int    `json:"page" in:"query" d:"1" dc:"页码"`
	PageSize int    `json:"page_size" in:"query" d:"20" dc:"每页数量"`
	Keyword  string `json:"keyword" in:"query" dc:"搜索关键词"`
	Role     string `json:"role" in:"query" dc:"按角色筛选"`
	Status   string `json:"status" in:"query" dc:"按状态筛选"`
}

type OpenMemberListRes struct {
	List     []OpenMemberItem `json:"list"`
	Total    int              `json:"total"`
	Page     int              `json:"page"`
	PageSize int              `json:"page_size"`
}

type OpenMemberItem struct {
	ID        int64  `json:"id"`
	Username  string `json:"username"`
	Email     string `json:"email"`
	Role      string `json:"role"`
	Status    string `json:"status"`
	CreatedAt string `json:"created_at"`
}

// OpenMemberCreate 创建成员
type OpenMemberCreateReq struct {
	g.Meta      `path:"/v1/members" method:"post" mime:"json" tags:"开放平台API" summary:"创建成员"`
	Username    string `json:"username" v:"required|length:2,50#请输入用户名|用户名长度2-50" dc:"用户名"`
	Email       string `json:"email" v:"required|email#请输入邮箱|邮箱格式不正确" dc:"邮箱"`
	Password    string `json:"password" v:"required|length:8,64#请输入密码|密码长度6-50" dc:"密码"`
	Role        string `json:"role" v:"required|in:admin,member#请选择角色|角色无效" dc:"角色"`
	DisplayName string `json:"display_name" dc:"显示名称"`
}

type OpenMemberCreateRes struct {
	ID int64 `json:"id"`
}

// OpenMemberUpdate 更新成员
type OpenMemberUpdateReq struct {
	g.Meta      `path:"/v1/members/{id}" method:"put" mime:"json" tags:"开放平台API" summary:"更新成员"`
	Id          int64   `json:"id" in:"path" v:"required#请指定成员ID" dc:"成员ID"`
	Role        *string `json:"role" v:"in:admin,member#角色无效" dc:"角色"`
	DisplayName *string `json:"display_name" dc:"显示名称"`
	Status      *string `json:"status" v:"in:active,disabled#状态无效" dc:"状态"`
}

type OpenMemberUpdateRes struct{}

// OpenMemberDelete 删除成员
type OpenMemberDeleteReq struct {
	g.Meta `path:"/v1/members/{id}" method:"delete" tags:"开放平台API" summary:"删除成员"`
	Id     int64 `json:"id" in:"path" v:"required#请指定成员ID" dc:"成员ID"`
}

type OpenMemberDeleteRes struct{}

// OpenMemberQuota 成员额度查询
type OpenMemberQuotaReq struct {
	g.Meta `path:"/v1/members/{id}/quota" method:"get" tags:"开放平台API" summary:"成员额度查询"`
	Id     int64 `json:"id" in:"path" v:"required#请指定成员ID" dc:"成员ID"`
}

type OpenMemberQuotaRes struct {
	QuotaType  string  `json:"quota_type"`
	QuotaLimit float64 `json:"quota_limit"`
	QuotaUsed  float64 `json:"quota_used"`
	Period     string  `json:"period"`
}

// OpenMemberQuotaUpdate 更新成员额度
type OpenMemberQuotaUpdateReq struct {
	g.Meta     `path:"/v1/members/{id}/quota" method:"put" mime:"json" tags:"开放平台API" summary:"更新成员额度"`
	Id         int64   `json:"id" in:"path" v:"required#请指定成员ID" dc:"成员ID"`
	QuotaType  string  `json:"quota_type" v:"required|in:none,total,periodic#请选择额度类型|额度类型无效" dc:"额度类型"`
	QuotaLimit float64 `json:"quota_limit" v:"min:0" dc:"额度上限(USD)"`
	Period     string  `json:"period" v:"in:day,week,month" dc:"周期类型"`
}

type OpenMemberQuotaUpdateRes struct{}

// OpenMemberModels 成员可用模型
type OpenMemberModelsReq struct {
	g.Meta `path:"/v1/members/{id}/models" method:"get" tags:"开放平台API" summary:"成员可用模型列表"`
	Id     int64 `json:"id" in:"path" v:"required#请指定成员ID" dc:"成员ID"`
}

type OpenMemberModelsRes struct {
	List []string `json:"list"`
}

// OpenMemberModelsUpdate 更新成员可用模型
type OpenMemberModelsUpdateReq struct {
	g.Meta   `path:"/v1/members/{id}/models" method:"put" mime:"json" tags:"开放平台API" summary:"更新成员可用模型"`
	Id       int64   `json:"id" in:"path" v:"required#请指定成员ID" dc:"成员ID"`
	ModelIDs []int64 `json:"model_ids" v:"required#请选择模型列表" dc:"模型ID列表"`
}

type OpenMemberModelsUpdateRes struct{}

// ============================================================
// API Key 管理
// ============================================================

// OpenKeyList API Key 列表
type OpenKeyListReq struct {
	g.Meta   `path:"/v1/keys" method:"get" tags:"开放平台API" summary:"API Key列表"`
	Page     int    `json:"page" in:"query" d:"1" dc:"页码"`
	PageSize int    `json:"page_size" in:"query" d:"20" dc:"每页数量"`
	Status   string `json:"status" in:"query" dc:"按状态筛选"`
}

type OpenKeyListRes struct {
	List     []OpenKeyItem `json:"list"`
	Total    int           `json:"total"`
	Page     int           `json:"page"`
	PageSize int           `json:"page_size"`
}

type OpenKeyItem struct {
	ID        int64  `json:"id"`
	Name      string `json:"name"`
	Key       string `json:"key"` // 脱敏显示 sk-***xxx
	Status    string `json:"status"`
	CreatedAt string `json:"created_at"`
}

// OpenKeyCreate 创建 API Key
type OpenKeyCreateReq struct {
	g.Meta      `path:"/v1/keys" method:"post" mime:"json" tags:"开放平台API" summary:"创建API Key"`
	Name        string   `json:"name" v:"required|length:2,100#请输入Key名称|名称长度2-100" dc:"Key名称"`
	ModelScopes []string `json:"model_scopes" dc:"可用模型列表（为空则不限）"`
	QuotaLimit  float64  `json:"quota_limit" d:"0" dc:"额度上限（0=不限）"`
}

type OpenKeyCreateRes struct {
	ID  int64  `json:"id"`
	Key string `json:"key"` // 完整 Key，仅创建时返回
}

// OpenKeyDelete 删除 API Key
type OpenKeyDeleteReq struct {
	g.Meta `path:"/v1/keys/{id}" method:"delete" tags:"开放平台API" summary:"删除API Key"`
	Id     int64 `json:"id" in:"path" v:"required#请指定Key ID" dc:"Key ID"`
}

type OpenKeyDeleteRes struct{}

// ============================================================
// 用量查询
// ============================================================

// OpenUsageQuery 用量查询
type OpenUsageQueryReq struct {
	g.Meta    `path:"/v1/usage" method:"get" tags:"开放平台API" summary:"用量查询"`
	StartDate string `json:"start_date" in:"query" v:"required#请输入开始日期" dc:"开始日期 (YYYY-MM-DD)"`
	EndDate   string `json:"end_date" in:"query" v:"required#请输入结束日期" dc:"结束日期 (YYYY-MM-DD)"`
	GroupBy   string `json:"group_by" in:"query" d:"day" v:"in:day,model,key#分组方式无效" dc:"分组方式: day/model/key"`
	Page      int    `json:"page" in:"query" d:"1" dc:"页码"`
	PageSize  int    `json:"page_size" in:"query" d:"20" dc:"每页数量"`
}

type OpenUsageQueryRes struct {
	List     []OpenUsageItem `json:"list"`
	Total    int             `json:"total"`
	Page     int             `json:"page"`
	PageSize int             `json:"page_size"`
}

type OpenUsageItem struct {
	Date             string `json:"date"`
	Model            string `json:"model,omitempty"`
	KeyName          string `json:"key_name,omitempty"`
	RequestCount     int64  `json:"request_count"`
	PromptTokens     int64  `json:"prompt_tokens"`
	CompletionTokens int64  `json:"completion_tokens"`
	TotalTokens      int64  `json:"total_tokens"`
	Cost             string `json:"cost"`
}

// ============================================================
// 费用查询
// ============================================================

// OpenBillingQuery 费用查询
type OpenBillingQueryReq struct {
	g.Meta    `path:"/v1/billing" method:"get" tags:"开放平台API" summary:"费用查询"`
	StartDate string `json:"start_date" in:"query" v:"required#请输入开始日期" dc:"开始日期 (YYYY-MM-DD)"`
	EndDate   string `json:"end_date" in:"query" v:"required#请输入结束日期" dc:"结束日期 (YYYY-MM-DD)"`
	Page      int    `json:"page" in:"query" d:"1" dc:"页码"`
	PageSize  int    `json:"page_size" in:"query" d:"20" dc:"每页数量"`
}

type OpenBillingQueryRes struct {
	List     []OpenBillingItem `json:"list"`
	Total    int               `json:"total"`
	Page     int               `json:"page"`
	PageSize int               `json:"page_size"`
}

type OpenBillingItem struct {
	ID          int64  `json:"id"`
	Type        string `json:"type"`
	Amount      string `json:"amount"`
	Balance     string `json:"balance,omitempty"`
	Description string `json:"description"`
	CreatedAt   string `json:"created_at"`
}
