// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package do

import (
	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/os/gtime"
)

// MdlModelGroups is the golang structure of table mdl_model_groups for DAO operations like Where/Data.
type MdlModelGroups struct {
	g.Meta      `orm:"table:mdl_model_groups, do:true"`
	Id          any         //
	Name        any         //
	Description any         //
	SortOrder   any         //
	Status      any         //
	CreatedAt   *gtime.Time //
	UpdatedAt   *gtime.Time //
}
