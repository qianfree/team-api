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
	Id          any         //
	AgreementId any         // 关联协议版本ID
	UserType    any         // 用户类型：admin / tenant
	UserId      any         // 用户ID
	IpAddress   any         // IP地址
	UserAgent   any         // User-Agent
	CreatedAt   *gtime.Time //
}
