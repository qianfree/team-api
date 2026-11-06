// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package entity

import (
	"github.com/gogf/gf/v2/os/gtime"
)

// JsTaskArtifacts is the golang structure for table js_task_artifacts.
type JsTaskArtifacts struct {
	Id           int64       `json:"id"             orm:"id"             description:""`                                                                      //
	TaskId       int64       `json:"task_id"        orm:"task_id"        description:""`                                                                      //
	PublicTaskId string      `json:"public_task_id" orm:"public_task_id" description:""`                                                                      //
	PluginKey    string      `json:"plugin_key"     orm:"plugin_key"     description:""`                                                                      //
	ArtifactKey  string      `json:"artifact_key"   orm:"artifact_key"   description:""`                                                                      //
	SourceType   string      `json:"source_type"    orm:"source_type"    description:""`                                                                      //
	SourceUrl    string      `json:"source_url"     orm:"source_url"     description:""`                                                                      //
	StoredKey    string      `json:"stored_key"     orm:"stored_key"     description:""`                                                                      //
	Backend      string      `json:"backend"        orm:"backend"        description:""`                                                                      //
	MimeType     string      `json:"mime_type"      orm:"mime_type"      description:""`                                                                      //
	Size         int64       `json:"size"           orm:"size"           description:""`                                                                      //
	Status       string      `json:"status"         orm:"status"         description:"pending=待转存 running=进行中 stored=完成 failed=失败（retry_count<5 由 sweep 重试）"` // pending=待转存 running=进行中 stored=完成 failed=失败（retry_count<5 由 sweep 重试）
	RetryCount   int         `json:"retry_count"    orm:"retry_count"    description:""`                                                                      //
	LastError    string      `json:"last_error"     orm:"last_error"     description:""`                                                                      //
	CreatedAt    *gtime.Time `json:"created_at"     orm:"created_at"     description:""`                                                                      //
	UpdatedAt    *gtime.Time `json:"updated_at"     orm:"updated_at"     description:""`                                                                      //
}
