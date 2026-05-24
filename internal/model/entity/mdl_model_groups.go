// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package entity

import (
	"github.com/gogf/gf/v2/os/gtime"
)

// MdlModelGroups is the golang structure for table mdl_model_groups.
type MdlModelGroups struct {
	Id          int64       `json:"id"          orm:"id"          description:""` //
	Name        string      `json:"name"        orm:"name"        description:""` //
	Description string      `json:"description" orm:"description" description:""` //
	SortOrder   int         `json:"sort_order"  orm:"sort_order"  description:""` //
	Status      string      `json:"status"      orm:"status"      description:""` //
	CreatedAt   *gtime.Time `json:"created_at"  orm:"created_at"  description:""` //
	UpdatedAt   *gtime.Time `json:"updated_at"  orm:"updated_at"  description:""` //
}
