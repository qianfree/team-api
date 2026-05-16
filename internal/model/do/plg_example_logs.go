// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package do

import (
	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/os/gtime"
)

// PlgExampleLogs is the golang structure of table plg_example_logs for DAO operations like Where/Data.
type PlgExampleLogs struct {
	g.Meta    `orm:"table:plg_example_logs, do:true"`
	Id        any         //
	TenantId  any         //
	Action    any         //
	Detail    any         //
	CreatedAt *gtime.Time //
}
