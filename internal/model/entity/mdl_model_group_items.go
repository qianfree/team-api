// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package entity

import (
	"github.com/gogf/gf/v2/os/gtime"
)

// MdlModelGroupItems is the golang structure for table mdl_model_group_items.
type MdlModelGroupItems struct {
	Id        int64       `json:"id"         orm:"id"         description:""` //
	GroupId   int64       `json:"group_id"   orm:"group_id"   description:""` //
	ModelName string      `json:"model_name" orm:"model_name" description:""` //
	CreatedAt *gtime.Time `json:"created_at" orm:"created_at" description:""` //
}
