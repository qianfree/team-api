// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package do

import (
	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/os/gtime"
)

// JsTaskArtifacts is the golang structure of table js_task_artifacts for DAO operations like Where/Data.
type JsTaskArtifacts struct {
	g.Meta       `orm:"table:js_task_artifacts, do:true"`
	Id           any         //
	TaskId       any         //
	PublicTaskId any         //
	PluginKey    any         //
	ArtifactKey  any         //
	SourceType   any         //
	SourceUrl    any         //
	StoredKey    any         //
	Backend      any         //
	MimeType     any         //
	Size         any         //
	Status       any         // pending=待转存 running=进行中 stored=完成 failed=失败（retry_count<5 由 sweep 重试）
	RetryCount   any         //
	LastError    any         //
	CreatedAt    *gtime.Time //
	UpdatedAt    *gtime.Time //
}
