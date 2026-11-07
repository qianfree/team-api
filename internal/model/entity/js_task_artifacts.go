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
	TaskId       int64       `json:"task_id"        orm:"task_id"        description:"关联任务 ID（tsk_model_tasks.id，逻辑关联无外键）"`                                   // 关联任务 ID（tsk_model_tasks.id，逻辑关联无外键）
	PublicTaskId string      `json:"public_task_id" orm:"public_task_id" description:"公开任务 ID（task_xxx，存储 key 组成部分）"`                                         // 公开任务 ID（task_xxx，存储 key 组成部分）
	PluginKey    string      `json:"plugin_key"     orm:"plugin_key"     description:"声明产物的 JS 插件 key"`                                                       // 声明产物的 JS 插件 key
	ArtifactKey  string      `json:"artifact_key"   orm:"artifact_key"   description:"产物键（任务内唯一，如 video / audio-0）"`                                          // 产物键（任务内唯一，如 video / audio-0）
	SourceType   string      `json:"source_type"    orm:"source_type"    description:"产物来源类型：url=下载转存 / inline=base64 直存"`                                    // 产物来源类型：url=下载转存 / inline=base64 直存
	SourceUrl    string      `json:"source_url"     orm:"source_url"     description:"上游取值路径或下载 URL（声明期存 taskData 取值路径，转存时更新为真实 URL）"`                        // 上游取值路径或下载 URL（声明期存 taskData 取值路径，转存时更新为真实 URL）
	StoredKey    string      `json:"stored_key"     orm:"stored_key"     description:"对象存储 key（jsartifacts/{public_task_id}/{artifact_key}{ext}，stored 后非空）"` // 对象存储 key（jsartifacts/{public_task_id}/{artifact_key}{ext}，stored 后非空）
	Backend      string      `json:"backend"        orm:"backend"        description:"对象存储后端：s3/oss/cos/local"`                                               // 对象存储后端：s3/oss/cos/local
	MimeType     string      `json:"mime_type"      orm:"mime_type"      description:"存储内容类型"`                                                                // 存储内容类型
	Size         int64       `json:"size"           orm:"size"           description:"转存字节数"`                                                                 // 转存字节数
	Status       string      `json:"status"         orm:"status"         description:"pending=待转存 running=进行中 stored=完成 failed=失败（retry_count<5 由 sweep 重试）"` // pending=待转存 running=进行中 stored=完成 failed=失败（retry_count<5 由 sweep 重试）
	RetryCount   int         `json:"retry_count"    orm:"retry_count"    description:"已重试次数（<5 由 sweep 重试）"`                                                  // 已重试次数（<5 由 sweep 重试）
	LastError    string      `json:"last_error"     orm:"last_error"     description:"最近一次失败原因"`                                                              // 最近一次失败原因
	CreatedAt    *gtime.Time `json:"created_at"     orm:"created_at"     description:"创建时间"`                                                                  // 创建时间
	UpdatedAt    *gtime.Time `json:"updated_at"     orm:"updated_at"     description:"更新时间"`                                                                  // 更新时间
	TenantId     int64       `json:"tenant_id"      orm:"tenant_id"      description:"任务归属租户 ID（sweep 恢复与租户观测用，从任务表回填）"`                                      // 任务归属租户 ID（sweep 恢复与租户观测用，从任务表回填）
	UserId       int64       `json:"user_id"        orm:"user_id"        description:"任务归属用户 ID（fil_files 落库归属）"`                                             // 任务归属用户 ID（fil_files 落库归属）
	ChannelId    int64       `json:"channel_id"     orm:"channel_id"     description:"任务转发渠道 ID（相对 URL 产物按此解析渠道地址与鉴权）"`                                       // 任务转发渠道 ID（相对 URL 产物按此解析渠道地址与鉴权）
}
