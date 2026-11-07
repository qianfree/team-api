// ==========================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// ==========================================================================

package internal

import (
	"context"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/frame/g"
)

// JsPluginVersionsDao is the data access object for the table js_plugin_versions.
type JsPluginVersionsDao struct {
	table    string                  // table is the underlying table name of the DAO.
	group    string                  // group is the database configuration group name of the current DAO.
	columns  JsPluginVersionsColumns // columns contains all the column names of Table for convenient usage.
	handlers []gdb.ModelHandler      // handlers for customized model modification.
}

// JsPluginVersionsColumns defines and stores column names for the table js_plugin_versions.
type JsPluginVersionsColumns struct {
	Id         string // 主键
	PluginKey  string // 插件标识（meta.key）
	Version    string // 插件版本（semver，key+version 唯一）
	Source     string // plugin.js 源码原文（上传/内置 embed）
	SourceHash string // 源码 SHA-256（同版本不可改内容的校验依据）
	Compiled   string // esbuild IIFE 编译产物（激活时编译缓存，可空）
	Manifest   string // 解析校验后的 meta 清单
	Origin     string // 版本来源：builtin=内置（go:embed 随二进制，不可删除）/ upload=管理端上传
	Icon       string // 插件图标（data URI，仅上传版本；内置版本用 manifest.icon）
	Enabled    string // 插件级开关（active 版本行上生效）
	Active     string // 激活版本（每 plugin_key 至多一行）
	Remark     string // 管理端备注
	CreatedAt  string // 创建时间
	UpdatedAt  string // 更新时间
}

// jsPluginVersionsColumns holds the columns for the table js_plugin_versions.
var jsPluginVersionsColumns = JsPluginVersionsColumns{
	Id:         "id",
	PluginKey:  "plugin_key",
	Version:    "version",
	Source:     "source",
	SourceHash: "source_hash",
	Compiled:   "compiled",
	Manifest:   "manifest",
	Origin:     "origin",
	Icon:       "icon",
	Enabled:    "enabled",
	Active:     "active",
	Remark:     "remark",
	CreatedAt:  "created_at",
	UpdatedAt:  "updated_at",
}

// NewJsPluginVersionsDao creates and returns a new DAO object for table data access.
func NewJsPluginVersionsDao(handlers ...gdb.ModelHandler) *JsPluginVersionsDao {
	return &JsPluginVersionsDao{
		group:    "default",
		table:    "js_plugin_versions",
		columns:  jsPluginVersionsColumns,
		handlers: handlers,
	}
}

// DB retrieves and returns the underlying raw database management object of the current DAO.
func (dao *JsPluginVersionsDao) DB() gdb.DB {
	return g.DB(dao.group)
}

// Table returns the table name of the current DAO.
func (dao *JsPluginVersionsDao) Table() string {
	return dao.table
}

// Columns returns all column names of the current DAO.
func (dao *JsPluginVersionsDao) Columns() JsPluginVersionsColumns {
	return dao.columns
}

// Group returns the database configuration group name of the current DAO.
func (dao *JsPluginVersionsDao) Group() string {
	return dao.group
}

// Ctx creates and returns a Model for the current DAO. It automatically sets the context for the current operation.
func (dao *JsPluginVersionsDao) Ctx(ctx context.Context) *gdb.Model {
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
func (dao *JsPluginVersionsDao) Transaction(ctx context.Context, f func(ctx context.Context, tx gdb.TX) error) (err error) {
	return dao.Ctx(ctx).Transaction(ctx, f)
}
