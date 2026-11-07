// ==========================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// ==========================================================================

package internal

import (
	"context"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/frame/g"
)

// JsTaskArtifactsDao is the data access object for the table js_task_artifacts.
type JsTaskArtifactsDao struct {
	table    string                 // table is the underlying table name of the DAO.
	group    string                 // group is the database configuration group name of the current DAO.
	columns  JsTaskArtifactsColumns // columns contains all the column names of Table for convenient usage.
	handlers []gdb.ModelHandler     // handlers for customized model modification.
}

// JsTaskArtifactsColumns defines and stores column names for the table js_task_artifacts.
type JsTaskArtifactsColumns struct {
	Id           string //
	TaskId       string // 关联任务 ID（tsk_model_tasks.id，逻辑关联无外键）
	PublicTaskId string // 公开任务 ID（task_xxx，存储 key 组成部分）
	PluginKey    string // 声明产物的 JS 插件 key
	ArtifactKey  string // 产物键（任务内唯一，如 video / audio-0）
	SourceType   string // 产物来源类型：url=下载转存 / inline=base64 直存
	SourceUrl    string // 上游取值路径或下载 URL（声明期存 taskData 取值路径，转存时更新为真实 URL）
	StoredKey    string // 对象存储 key（jsartifacts/{public_task_id}/{artifact_key}{ext}，stored 后非空）
	Backend      string // 对象存储后端：s3/oss/cos/local
	MimeType     string // 存储内容类型
	Size         string // 转存字节数
	Status       string // pending=待转存 running=进行中 stored=完成 failed=失败（retry_count<5 由 sweep 重试）
	RetryCount   string // 已重试次数（<5 由 sweep 重试）
	LastError    string // 最近一次失败原因
	CreatedAt    string // 创建时间
	UpdatedAt    string // 更新时间
	TenantId     string // 任务归属租户 ID（sweep 恢复与租户观测用，从任务表回填）
	UserId       string // 任务归属用户 ID（fil_files 落库归属）
	ChannelId    string // 任务转发渠道 ID（相对 URL 产物按此解析渠道地址与鉴权）
}

// jsTaskArtifactsColumns holds the columns for the table js_task_artifacts.
var jsTaskArtifactsColumns = JsTaskArtifactsColumns{
	Id:           "id",
	TaskId:       "task_id",
	PublicTaskId: "public_task_id",
	PluginKey:    "plugin_key",
	ArtifactKey:  "artifact_key",
	SourceType:   "source_type",
	SourceUrl:    "source_url",
	StoredKey:    "stored_key",
	Backend:      "backend",
	MimeType:     "mime_type",
	Size:         "size",
	Status:       "status",
	RetryCount:   "retry_count",
	LastError:    "last_error",
	CreatedAt:    "created_at",
	UpdatedAt:    "updated_at",
	TenantId:     "tenant_id",
	UserId:       "user_id",
	ChannelId:    "channel_id",
}

// NewJsTaskArtifactsDao creates and returns a new DAO object for table data access.
func NewJsTaskArtifactsDao(handlers ...gdb.ModelHandler) *JsTaskArtifactsDao {
	return &JsTaskArtifactsDao{
		group:    "default",
		table:    "js_task_artifacts",
		columns:  jsTaskArtifactsColumns,
		handlers: handlers,
	}
}

// DB retrieves and returns the underlying raw database management object of the current DAO.
func (dao *JsTaskArtifactsDao) DB() gdb.DB {
	return g.DB(dao.group)
}

// Table returns the table name of the current DAO.
func (dao *JsTaskArtifactsDao) Table() string {
	return dao.table
}

// Columns returns all column names of the current DAO.
func (dao *JsTaskArtifactsDao) Columns() JsTaskArtifactsColumns {
	return dao.columns
}

// Group returns the database configuration group name of the current DAO.
func (dao *JsTaskArtifactsDao) Group() string {
	return dao.group
}

// Ctx creates and returns a Model for the current DAO. It automatically sets the context for the current operation.
func (dao *JsTaskArtifactsDao) Ctx(ctx context.Context) *gdb.Model {
	model := dao.DB().Model(dao.table)
	for _, handler := range dao.handlers {
		model = handler(model)
	}
	return model.Safe().Ctx(ctx)
}

// Transaction wraps the transaction logic using function f.
// It rolls back the transaction and returns the error if function f returns a non-nil error.
// It commits the transaction and returns nil if function f returns nil.
//
// Note: Do not commit or roll back the transaction in function f,
// as it is automatically handled by this function.
func (dao *JsTaskArtifactsDao) Transaction(ctx context.Context, f func(ctx context.Context, tx gdb.TX) error) (err error) {
	return dao.Ctx(ctx).Transaction(ctx, f)
}
