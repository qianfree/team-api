// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package entity

import (
	"github.com/gogf/gf/v2/os/gtime"
)

// SysAgreements is the golang structure for table sys_agreements.
type SysAgreements struct {
	Id          int64       `json:"id"           orm:"id"`
	Code        string      `json:"code"         orm:"code"`
	Version     string      `json:"version"      orm:"version"`
	Title       string      `json:"title"        orm:"title"`
	Content     string      `json:"content"      orm:"content"`
	Summary     string      `json:"summary"      orm:"summary"`
	Status      string      `json:"status"       orm:"status"`
	IsCurrent   bool        `json:"is_current"   orm:"is_current"`
	ForceAccept bool        `json:"force_accept" orm:"force_accept"`
	PublishedAt *gtime.Time `json:"published_at" orm:"published_at"`
	CreatedBy   int64       `json:"created_by"   orm:"created_by"`
	CreatedAt   *gtime.Time `json:"created_at"   orm:"created_at"`
	UpdatedAt   *gtime.Time `json:"updated_at"   orm:"updated_at"`
}
