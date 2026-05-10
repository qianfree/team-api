// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package do

import (
	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/os/gtime"
)

// GooseDbVersion is the golang structure of table goose_db_version for DAO operations like Where/Data.
type GooseDbVersion struct {
	g.Meta    `orm:"table:goose_db_version, do:true"`
	Id        any         //
	VersionId any         //
	IsApplied any         //
	Tstamp    *gtime.Time //
}
