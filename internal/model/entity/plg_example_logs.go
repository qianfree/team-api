// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package entity

import (
	"github.com/gogf/gf/v2/os/gtime"
)

// PlgExampleLogs is the golang structure for table plg_example_logs.
type PlgExampleLogs struct {
	Id        int64       `json:"id"         orm:"id"         description:""` //
	TenantId  int64       `json:"tenant_id"  orm:"tenant_id"  description:""` //
	Action    string      `json:"action"     orm:"action"     description:""` //
	Detail    string      `json:"detail"     orm:"detail"     description:""` //
	CreatedAt *gtime.Time `json:"created_at" orm:"created_at" description:""` //
}
