// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package do

import (
	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/os/gtime"
)

// MdlModelGroupItems is the golang structure of table mdl_model_group_items for DAO operations like Where/Data.
type MdlModelGroupItems struct {
	g.Meta    `orm:"table:mdl_model_group_items, do:true"`
	Id        any         //
	GroupId   any         //
	ModelName any         //
	CreatedAt *gtime.Time //
}
