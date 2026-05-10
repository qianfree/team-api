// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package entity

import (
	"github.com/gogf/gf/v2/os/gtime"
)

// GooseDbVersion is the golang structure for table goose_db_version.
type GooseDbVersion struct {
	Id        int         `json:"id"         orm:"id"         description:""` //
	VersionId int64       `json:"version_id" orm:"version_id" description:""` //
	IsApplied bool        `json:"is_applied" orm:"is_applied" description:""` //
	Tstamp    *gtime.Time `json:"tstamp"     orm:"tstamp"     description:""` //
}
