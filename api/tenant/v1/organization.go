package v1

import "github.com/gogf/gf/v2/frame/g"

// TenantOrgInfoReq 获取组织信息请求
type TenantOrgInfoReq struct {
	g.Meta `path:"/organization" method:"get" mime:"json" tags:"租户控制台-组织" summary:"获取组织信息"`
}

type TenantOrgInfoRes struct {
	ID          int64  `json:"id"`
	Name        string `json:"name"`
	Code        string `json:"code"`
	LogoURL     string `json:"logo_url"`
	Status      string `json:"status"`
	Level       int    `json:"level"`
	LevelName   string `json:"level_name"`
	MaxMembers  int    `json:"max_members"`
	MemberCount int    `json:"member_count"`
	CreatedAt   string `json:"created_at"`
}

// TenantOrgUpdateReq 更新组织信息请求
type TenantOrgUpdateReq struct {
	g.Meta  `path:"/organization" method:"put" mime:"json" tags:"租户控制台-组织" summary:"更新组织信息"`
	Name    *string `json:"name" v:"length:2,100#组织名称长度为2-100位" dc:"组织名称"`
	LogoURL *string `json:"logo_url" dc:"Logo URL"`
}

type TenantOrgUpdateRes struct{}

// TenantOrgTransferReq 转让所有权请求
type TenantOrgTransferReq struct {
	g.Meta     `path:"/organization/transfer" method:"post" mime:"json" tags:"租户控制台-组织" summary:"转让所有权"`
	NewOwnerID int64  `json:"new_owner_id" v:"required#请指定新所有者" dc:"新所有者用户ID"`
	Password   string `json:"password" v:"required#请输入密码确认" dc:"当前用户密码"`
}

type TenantOrgTransferRes struct{}

// TenantProfileReq 获取个人信息请求
type TenantProfileReq struct {
	g.Meta `path:"/profile" method:"get" mime:"json" tags:"租户控制台-组织" summary:"获取个人信息"`
}

type TenantProfileRes struct {
	ID          int64  `json:"id"`
	Username    string `json:"username"`
	Email       string `json:"email"`
	DisplayName string `json:"display_name"`
	Role        string `json:"role"`
	Status      string `json:"status"`
	CreatedAt   string `json:"created_at"`
}

// TenantProfileUpdateReq 更新个人信息请求
type TenantProfileUpdateReq struct {
	g.Meta      `path:"/profile" method:"put" mime:"json" tags:"租户控制台-组织" summary:"更新个人信息"`
	DisplayName *string `json:"display_name" dc:"显示名称"`
}

type TenantProfileUpdateRes struct{}

// TenantIPWhitelistReq 获取/设置登录 IP 白名单
type TenantIPWhitelistGetReq struct {
	g.Meta `path:"/security/ip-whitelist" method:"get" tags:"租户控制台-安全" summary:"获取登录IP白名单"`
}

type TenantIPWhitelistGetRes struct {
	Enabled   bool     `json:"enabled"`
	Whitelist []string `json:"whitelist"`
}

type TenantIPWhitelistUpdateReq struct {
	g.Meta    `path:"/security/ip-whitelist" method:"put" mime:"json" tags:"租户控制台-安全" summary:"更新登录IP白名单"`
	Enabled   *bool    `json:"enabled" dc:"是否启用IP白名单"`
	Whitelist []string `json:"whitelist" v:"length:0,100#白名单IP数量不能超过100" dc:"IP地址列表（CIDR格式）"`
}

type TenantIPWhitelistUpdateRes struct{}
