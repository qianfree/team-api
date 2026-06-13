// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package entity

import (
	"github.com/gogf/gf/v2/os/gtime"
)

// SysAgreementAcceptances is the golang structure for table sys_agreement_acceptances.
type SysAgreementAcceptances struct {
	Id          int64       `json:"id"           orm:"id"`
	AgreementId int64       `json:"agreement_id" orm:"agreement_id"`
	UserType    string      `json:"user_type"    orm:"user_type"`
	UserId      int64       `json:"user_id"      orm:"user_id"`
	IpAddress   string      `json:"ip_address"   orm:"ip_address"`
	UserAgent   string      `json:"user_agent"   orm:"user_agent"`
	CreatedAt   *gtime.Time `json:"created_at"   orm:"created_at"`
}
