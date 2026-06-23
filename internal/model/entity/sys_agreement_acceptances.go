// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package entity

import (
	"github.com/gogf/gf/v2/os/gtime"
)

// SysAgreementAcceptances is the golang structure for table sys_agreement_acceptances.
type SysAgreementAcceptances struct {
	Id          int64       `json:"id"           orm:"id"           description:"主键ID"`                           // 主键ID
	AgreementId int64       `json:"agreement_id" orm:"agreement_id" description:"关联协议版本ID"`                       // 关联协议版本ID
	UserType    string      `json:"user_type"    orm:"user_type"    description:"用户类型：admin(管理员) / tenant(租户用户)"` // 用户类型：admin(管理员) / tenant(租户用户)
	UserId      int64       `json:"user_id"      orm:"user_id"      description:"用户ID"`                           // 用户ID
	IpAddress   string      `json:"ip_address"   orm:"ip_address"   description:"接受时的IP地址"`                       // 接受时的IP地址
	UserAgent   string      `json:"user_agent"   orm:"user_agent"   description:"接受时的浏览器User-Agent"`              // 接受时的浏览器User-Agent
	CreatedAt   *gtime.Time `json:"created_at"   orm:"created_at"   description:"接受时间"`                           // 接受时间
}
