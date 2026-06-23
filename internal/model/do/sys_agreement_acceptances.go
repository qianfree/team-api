// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package do

import (
	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/os/gtime"
)

// SysAgreementAcceptances is the golang structure of table sys_agreement_acceptances for DAO operations like Where/Data.
type SysAgreementAcceptances struct {
	g.Meta      `orm:"table:sys_agreement_acceptances, do:true"`
	Id          any         // 主键ID
	AgreementId any         // 关联协议版本ID
	UserType    any         // 用户类型：admin(管理员) / tenant(租户用户)
	UserId      any         // 用户ID
	IpAddress   any         // 接受时的IP地址
	UserAgent   any         // 接受时的浏览器User-Agent
	CreatedAt   *gtime.Time // 接受时间
}
