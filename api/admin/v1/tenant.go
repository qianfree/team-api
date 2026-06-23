package v1

import "github.com/gogf/gf/v2/frame/g"

// TenantCreateReq 创建租户请求
type TenantCreateReq struct {
	g.Meta         `path:"/tenants" method:"post" mime:"json" tags:"管理后台-租户管理" summary:"创建租户"`
	TenantName     string `json:"tenant_name" v:"required#请输入租户名称" dc:"租户名称（汉字最多8个，字母最多16个）"`
	TenantCode     string `json:"tenant_code" v:"required|length:3,30|regex:^[a-z0-9][a-z0-9-]*[a-z0-9]$#请输入租户代码|租户代码为3-30位|租户代码仅允许小写字母、数字、中划线" dc:"租户代码"`
	Username       string `json:"username" v:"required|length:3,50#请输入管理员用户名|用户名长度为3-50位" dc:"管理员用户名"`
	Email          string `json:"email" v:"required|email#请输入邮箱|邮箱格式不正确" dc:"管理员邮箱"`
	Password       string `json:"password" v:"required|length:8,64#请输入密码|密码长度为8-64位" dc:"管理员密码"`
	MaxMembers     *int   `json:"max_members" dc:"最大成员数（默认10）"`
	MaxConcurrency *int   `json:"max_concurrency" dc:"并发上限，0不限（默认0）"`
}

type TenantCreateRes struct {
	Id int64 `json:"id"`
}

// TenantListReq 租户列表请求
type TenantListReq struct {
	g.Meta   `path:"/tenants" method:"get" mime:"json" tags:"管理后台-租户管理" summary:"租户列表"`
	Page     int    `json:"page" d:"1" dc:"页码"`
	PageSize int    `json:"page_size" d:"20" dc:"每页数量"`
	Keyword  string `json:"keyword" dc:"搜索关键词（名称/代码）"`
	Status   string `json:"status" dc:"状态筛选：active/suspended/closed"`
}

type TenantListRes struct {
	List     []TenantItem `json:"list"`
	Total    int          `json:"total"`
	Page     int          `json:"page"`
	PageSize int          `json:"page_size"`
}

type TenantItem struct {
	ID                      int64  `json:"id"`
	Name                    string `json:"name"`
	Code                    string `json:"code"`
	LogoURL                 string `json:"logo_url"`
	OwnerUserID             int64  `json:"owner_user_id"`
	OwnerName               string `json:"owner_name"`
	Status                  string `json:"status"`
	MaxMembers              *int   `json:"max_members" dc:"最大成员数上限，NULL表示跟随等级配置"`
	MaxConcurrency          *int   `json:"max_concurrency" dc:"并发上限，NULL表示跟随等级配置"`
	EffectiveMaxMembers     int    `json:"effective_max_members" dc:"实际生效的成员数上限"`
	EffectiveMaxConcurrency int    `json:"effective_max_concurrency" dc:"实际生效的并发上限"`
	DefaultChannelScope     string `json:"default_channel_scope"`
	MemberCount             int    `json:"member_count"`
	WalletBalance           string `json:"wallet_balance"`
	Level                   int    `json:"level"`
	LevelName               string `json:"level_name"`
	CreatedAt               string `json:"created_at"`
	UpdatedAt               string `json:"updated_at"`
}

// TenantGetReq 获取租户详情
type TenantGetReq struct {
	g.Meta `path:"/tenants/{id}" method:"get" mime:"json" tags:"管理后台-租户管理" summary:"租户详情"`
	Id     int64 `json:"id" in:"path" v:"required" dc:"租户ID"`
}

type TenantGetRes struct {
	TenantItem
	Settings string `json:"settings"`
}

// TenantChannelScopeUpdateReq 更新租户默认渠道范围
type TenantChannelScopeUpdateReq struct {
	g.Meta              `path:"/tenants/{id}/channel-scope" method:"put" mime:"json" tags:"管理后台-租户管理" summary:"更新租户渠道范围"`
	Id                  int64   `json:"id" in:"path" v:"required" dc:"租户ID"`
	DefaultChannelScope *string `json:"default_channel_scope" dc:"默认渠道范围JSON数组，如[1,5,12]，null或[]表示全部"`
}

// TenantUpdateStatusReq 更新租户状态
type TenantUpdateStatusReq struct {
	g.Meta `path:"/tenants/{id}/status" method:"put" mime:"json" tags:"管理后台-租户管理" summary:"更新租户状态"`
	Id     int64  `json:"id" in:"path" v:"required" dc:"租户ID"`
	Status string `json:"status" v:"required|in:active,suspended,closed#请选择状态|状态值无效" dc:"状态：active / suspended / closed"`
}

type TenantUpdateStatusRes struct{}

// TenantUpdateReq 更新租户信息
type TenantUpdateReq struct {
	g.Meta         `path:"/tenants/{id}" method:"put" mime:"json" tags:"管理后台-租户管理" summary:"更新租户"`
	Id             int64  `json:"id" in:"path" v:"required" dc:"租户ID"`
	Name           string `json:"name" dc:"租户名称"`
	MaxMembers     *int   `json:"max_members" dc:"最大成员数"`
	MaxConcurrency *int   `json:"max_concurrency" dc:"租户总并发上限（0表示不限制）"`
	Level          *int   `json:"level" dc:"租户等级（调整等级会同步更新成员数和并发数为该等级的配置值）"`
}

type TenantUpdateRes struct{}

// AdminMemberListReq 成员列表请求（管理后台查看所有租户成员）
type AdminMemberListReq struct {
	g.Meta   `path:"/members" method:"get" mime:"json" tags:"管理后台-租户管理" summary:"成员列表"`
	Page     int    `json:"page" d:"1" dc:"页码"`
	PageSize int    `json:"page_size" d:"20" dc:"每页数量"`
	Keyword  string `json:"keyword" dc:"搜索关键词（用户名/邮箱）"`
	Status   string `json:"status" dc:"状态筛选"`
	Role     string `json:"role" dc:"角色筛选"`
	TenantID int64  `json:"tenant_id" dc:"租户ID筛选"`
}

type AdminMemberListRes struct {
	List     []AdminMemberItem `json:"list"`
	Total    int               `json:"total"`
	Page     int               `json:"page"`
	PageSize int               `json:"page_size"`
}

// TenantChannelScopeUpdateRes 更新租户渠道范围响应
type TenantChannelScopeUpdateRes struct{}

type AdminMemberItem struct {
	ID             int64  `json:"id"`
	TenantID       int64  `json:"tenant_id"`
	TenantName     string `json:"tenant_name"`
	TenantCode     string `json:"tenant_code"`
	Username       string `json:"username"`
	Email          string `json:"email"`
	DisplayName    string `json:"display_name"`
	Role           string `json:"role"`
	Status         string `json:"status"`
	LastLoginAt    string `json:"last_login_at"`
	LastLoginIP    string `json:"last_login_ip"`
	FailedAttempts int    `json:"failed_attempts"`
	CreatedAt      string `json:"created_at"`
}

// TenantExportReq 导出租户列表请求
type TenantExportReq struct {
	g.Meta  `path:"/tenants/export" method:"get" mime:"json" tags:"管理后台-租户" summary:"导出租户列表"`
	Format  string `json:"format" in:"query" d:"csv" v:"in:csv,xlsx" dc:"导出格式：csv / xlsx"`
	Keyword string `json:"keyword" in:"query" dc:"搜索关键词（名称/代码）"`
	Status  string `json:"status" in:"query" dc:"状态筛选：active/suspended/closed"`
}

type TenantExportRes struct{}

// AdminMemberExportReq 导出成员列表请求
type AdminMemberExportReq struct {
	g.Meta   `path:"/tenants/members/export" method:"get" mime:"json" tags:"管理后台-成员" summary:"导出成员列表"`
	Format   string `json:"format" in:"query" d:"csv" v:"in:csv,xlsx" dc:"导出格式：csv / xlsx"`
	Keyword  string `json:"keyword" in:"query" dc:"搜索关键词（用户名/邮箱）"`
	Status   string `json:"status" in:"query" dc:"状态筛选"`
	Role     string `json:"role" in:"query" dc:"角色筛选"`
	TenantID int64  `json:"tenant_id" in:"query" dc:"租户ID筛选"`
}

type AdminMemberExportRes struct{}

// TenantSelectReq 租户下拉选择列表（轻量，用于选择器组件）
type TenantSelectReq struct {
	g.Meta   `path:"/tenants/select" method:"get" mime:"json" tags:"管理后台-租户管理" summary:"租户下拉选择列表"`
	Page     int    `json:"page" in:"query" d:"1" dc:"页码"`
	PageSize int    `json:"page_size" in:"query" d:"20" dc:"每页数量"`
	Keyword  string `json:"keyword" in:"query" dc:"搜索关键词（名称/代码）"`
}

type TenantSelectRes struct {
	List     []TenantSelectItem `json:"list"`
	Total    int                `json:"total"`
	Page     int                `json:"page"`
	PageSize int                `json:"page_size"`
}

type TenantSelectItem struct {
	ID   int64  `json:"id"`
	Name string `json:"name"`
	Code string `json:"code"`
}
