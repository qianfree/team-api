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
	TaskId       any         // 关联任务 ID（tsk_model_tasks.id，逻辑关联无外键）
	PublicTaskId any         // 公开任务 ID（task_xxx，存储 key 组成部分）
	PluginKey    any         // 声明产物的 JS 插件 key
	ArtifactKey  any         // 产物键（任务内唯一，如 video / audio-0）
	SourceType   any         // 产物来源类型：url=下载转存 / inline=base64 直存
	SourceUrl    any         // 实际下载 URL（url 型转存时写入；存量行声明期曾存取值路径）
	StoredKey    any         // 对象存储 key（jsartifacts/{public_task_id}/{artifact_key}{ext}，stored 后非空）
	Backend      any         // 对象存储后端：s3/oss/cos/local
	MimeType     any         // 存储内容类型
	Size         any         // 转存字节数
	Status       any         // pending=待转存 running=进行中 stored=完成 failed=失败（retry_count<5 由 sweep 重试） direct=不转存（插件关闭转存，回源代理）
	RetryCount   any         // 已重试次数（<5 由 sweep 重试）
	LastError    any         // 最近一次失败原因
	CreatedAt    *gtime.Time // 创建时间
	UpdatedAt    *gtime.Time // 更新时间
	TenantId     any         // 任务归属租户 ID（sweep 恢复与租户观测用，从任务表回填）
	UserId       any         // 任务归属用户 ID（fil_files 落库归属）
	ChannelId    any         // 任务转发渠道 ID（相对 URL 产物按此解析渠道地址与鉴权）
	SourcePath   any         // 产物声明的 taskData 取值路径（声明期写入，不随转存改写；回源代理与 sweep 重建的依据）
	SourceAuth   any         // 产物声明的下载鉴权形态：空=裸下载 / channel-bearer=注入渠道密钥
}
