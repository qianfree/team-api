package v1

import "github.com/gogf/gf/v2/frame/g"

// TenantMemberListReq 成员列表请求
type TenantMemberListReq struct {
	g.Meta   `path:"/members" method:"get" mime:"json" tags:"租户控制台-成员管理" summary:"成员列表"`
	Page     int    `json:"page" d:"1" dc:"页码"`
	PageSize int    `json:"page_size" d:"20" dc:"每页数量"`
	Keyword  string `json:"keyword" dc:"搜索关键词（用户名/邮箱）"`
	Role     string `json:"role" dc:"角色筛选"`
}

type TenantMemberListRes struct {
	List     []TenantMemberItem `json:"list"`
	Total    int                `json:"total"`
	Page     int                `json:"page"`
	PageSize int                `json:"page_size"`
}

type TenantMemberItem struct {
	ID          int64  `json:"id"`
	Username    string `json:"username"`
	Email       string `json:"email"`
	DisplayName string `json:"display_name"`
	Role        string `json:"role"`
	Status      string `json:"status"`
	CreatedAt   string `json:"created_at"`
}

// TenantMemberInviteReq 生成邀请链接请求
type TenantMemberInviteReq struct {
	g.Meta      `path:"/members/invite" method:"post" mime:"json" tags:"租户控制台-成员管理" summary:"生成邀请链接"`
	Role        string `json:"role" d:"member" v:"in:admin,member#角色无效" dc:"邀请角色"`
	ExpiresDays int    `json:"expires_days" d:"7" v:"between:1,30" dc:"有效天数"`
	MaxUses     int    `json:"max_uses" d:"0" v:"min:0" dc:"最大使用次数，0表示不限"`
}

type TenantMemberInviteRes struct {
	Code      string `json:"code"`
	InviteURL string `json:"invite_url"`
	ExpiresAt string `json:"expires_at"`
	MaxUses   int    `json:"max_uses"`
}

// TenantInvitationListReq 邀请记录列表
type TenantInvitationListReq struct {
	g.Meta   `path:"/members/invitations" method:"get" mime:"json" tags:"租户控制台-成员管理" summary:"邀请记录列表"`
	Page     int `json:"page" d:"1" dc:"页码"`
	PageSize int `json:"page_size" d:"20" dc:"每页数量"`
}

type TenantInvitationListRes struct {
	List     []TenantInvitationItem `json:"list"`
	Total    int                    `json:"total"`
	Page     int                    `json:"page"`
	PageSize int                    `json:"page_size"`
}

type TenantInvitationItem struct {
	ID          int64  `json:"id"`
	Code        string `json:"code"`
	Role        string `json:"role"`
	Status      string `json:"status"`
	InviteURL   string `json:"invite_url,omitempty"`
	ExpiresAt   string `json:"expires_at"`
	MaxUses     int    `json:"max_uses"`
	UseCount    int    `json:"use_count"`
	UsedByName  string `json:"used_by_name,omitempty"`
	CreatedAt   string `json:"created_at"`
	CreatorName string `json:"creator_name"`
}

// TenantInvitationRevokeReq 撤销邀请
type TenantInvitationRevokeReq struct {
	g.Meta `path:"/members/invitations/{id}" method:"delete" mime:"json" tags:"租户控制台-成员管理" summary:"撤销邀请"`
	Id     int64 `json:"id" in:"path" v:"required#请指定邀请ID" dc:"邀请ID"`
}

type TenantInvitationRevokeRes struct{}

// TenantInviteInfoReq 查询邀请信息（公开接口，免认证）
type TenantInviteInfoReq struct {
	g.Meta `path:"/members/invite-info" method:"get" mime:"json" tags:"租户控制台-成员管理" summary:"查询邀请信息" group:"public" middleware:"-"`
	Code   string `json:"code" in:"query" v:"required#请提供邀请码" dc:"邀请码"`
}

type TenantInviteInfoRes struct {
	TenantName string `json:"tenant_name"`
	Role       string `json:"role"`
	ExpiresAt  string `json:"expires_at"`
	Valid      bool   `json:"valid"`
}

// TenantMemberJoinReq 通过邀请链接加入请求
type TenantMemberJoinReq struct {
	g.Meta      `path:"/members/join" method:"post" mime:"json" tags:"租户控制台-成员管理" summary:"通过邀请加入" group:"public" middleware:"-"`
	Code        string `json:"code" v:"required#请提供邀请码" dc:"邀请码"`
	Username    string `json:"username" v:"required|length:3,50#请输入用户名|用户名长度为3-50位" dc:"用户名"`
	Password    string `json:"password" v:"required|length:8,64#请输入密码|密码长度为8-64位" dc:"密码"`
	Email       string `json:"email" v:"email#邮箱格式不正确" dc:"邮箱"`
	DisplayName string `json:"display_name" dc:"显示名称"`
}

type TenantMemberJoinRes struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	ExpiresAt    string `json:"expires_at"`
	TenantName   string `json:"tenant_name"`
	TenantCode   string `json:"tenant_code"`
	Username     string `json:"username"`
	Role         string `json:"role"`
}

// TenantMemberCreateReq 直接创建成员请求
type TenantMemberCreateReq struct {
	g.Meta      `path:"/members/create" method:"post" mime:"json" tags:"租户控制台-成员管理" summary:"直接创建成员"`
	Username    string `json:"username" v:"required|length:3,50#请输入用户名|用户名长度为3-50位" dc:"用户名"`
	Password    string `json:"password" v:"required|length:8,64#请输入密码|密码长度为8-64位" dc:"密码"`
	Email       string `json:"email" v:"email#邮箱格式不正确" dc:"邮箱"`
	DisplayName string `json:"display_name" dc:"显示名称"`
	Role        string `json:"role" d:"member" v:"in:admin,member#角色无效" dc:"角色"`
}

type TenantMemberCreateRes struct {
	ID int64 `json:"id"`
}

// TenantMemberResetPasswordReq 重置成员密码请求
type TenantMemberResetPasswordReq struct {
	g.Meta   `path:"/members/{id}/reset-password" method:"put" mime:"json" tags:"租户控制台-成员管理" summary:"重置成员密码"`
	Id       int64  `json:"id" in:"path" v:"required" dc:"成员ID"`
	Password string `json:"password" v:"required|length:8,64#请输入新密码|密码长度为8-64位" dc:"新密码"`
}

type TenantMemberResetPasswordRes struct{}

// TenantMemberRemoveReq 移除成员请求
type TenantMemberRemoveReq struct {
	g.Meta `path:"/members/{id}" method:"delete" mime:"json" tags:"租户控制台-成员管理" summary:"移除成员"`
	Id     int64 `json:"id" in:"path" v:"required" dc:"成员ID"`
}

type TenantMemberRemoveRes struct{}

// TenantMemberUpdateRoleReq 更新成员角色请求
type TenantMemberUpdateRoleReq struct {
	g.Meta `path:"/members/{id}/role" method:"put" mime:"json" tags:"租户控制台-成员管理" summary:"更新成员角色"`
	Id     int64  `json:"id" in:"path" v:"required" dc:"成员ID"`
	Role   string `json:"role" v:"required|in:admin,member#请选择角色|角色无效" dc:"角色"`
}

type TenantMemberUpdateRoleRes struct{}

// TenantMemberGetReq 成员详情请求
type TenantMemberGetReq struct {
	g.Meta `path:"/members/{id}" method:"get" mime:"json" tags:"租户控制台-成员管理" summary:"成员详情"`
	Id     int64 `json:"id" in:"path" v:"required" dc:"成员ID"`
}

type TenantMemberGetRes struct {
	ID          int64  `json:"id"`
	Username    string `json:"username"`
	Email       string `json:"email"`
	DisplayName string `json:"display_name"`
	Role        string `json:"role"`
	Status      string `json:"status"`
	CreatedAt   string `json:"created_at"`
	UpdatedAt   string `json:"updated_at"`
}

// TenantMemberUsageReq 成员用量统计请求
type TenantMemberUsageReq struct {
	g.Meta `path:"/members/{id}/usage" method:"get" mime:"json" tags:"租户控制台-成员管理" summary:"成员用量统计"`
	Id     int64 `json:"id" in:"path" v:"required" dc:"成员ID"`
}

type TenantMemberUsageRes struct {
	TodayRequests     float64 `json:"today_requests"`
	MonthRequests     float64 `json:"month_requests"`
	MonthInputTokens  float64 `json:"month_input_tokens"`
	MonthOutputTokens float64 `json:"month_output_tokens"`
	MonthTotalCost    float64 `json:"month_total_cost"`
}

// TenantMemberApiKeysReq 成员 API Key 列表请求
type TenantMemberApiKeysReq struct {
	g.Meta   `path:"/members/{id}/api-keys" method:"get" mime:"json" tags:"租户控制台-成员管理" summary:"成员API Key列表"`
	Id       int64 `json:"id" in:"path" v:"required" dc:"成员ID"`
	Page     int   `json:"page" d:"1" dc:"页码"`
	PageSize int   `json:"page_size" d:"20" dc:"每页数量"`
}

type TenantMemberApiKeysRes struct {
	List     []TenantMemberApiKeyItem `json:"list"`
	Total    int                      `json:"total"`
	Page     int                      `json:"page"`
	PageSize int                      `json:"page_size"`
}

type TenantMemberApiKeyItem struct {
	ID         int64   `json:"id"`
	Name       string  `json:"name"`
	KeyPrefix  string  `json:"key_prefix"`
	Scope      string  `json:"scope"`
	Status     string  `json:"status"`
	ExpiresAt  string  `json:"expires_at"`
	CreatedAt  string  `json:"created_at"`
	TotalQuota float64 `json:"total_quota"`
	UsedQuota  float64 `json:"used_quota"`
}

// TenantMemberExportReq 导出成员列表请求
type TenantMemberExportReq struct {
	g.Meta  `path:"/members/export" method:"get" mime:"json" tags:"租户控制台-成员管理" summary:"导出成员列表"`
	Format  string `json:"format" in:"query" d:"csv" v:"in:csv,xlsx" dc:"导出格式：csv / xlsx"`
	Keyword string `json:"keyword" in:"query" dc:"搜索关键词（用户名/邮箱）"`
	Role    string `json:"role" in:"query" dc:"角色筛选"`
}

type TenantMemberExportRes struct{}
