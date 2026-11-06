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
	TaskId       string //
	PublicTaskId string //
	PluginKey    string //
	ArtifactKey  string //
	SourceType   string //
	SourceUrl    string //
	StoredKey    string //
	Backend      string //
	MimeType     string //
	Size         string //
	Status       string // pending=待转存 running=进行中 stored=完成 failed=失败（retry_count<5 由 sweep 重试）
	RetryCount   string //
	LastError    string //
	CreatedAt    string //
	UpdatedAt    string //
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
