// ==========================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// ==========================================================================

package internal

import (
	"context"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/frame/g"
)

// MdlModelsDao is the data access object for the table mdl_models.
type MdlModelsDao struct {
	table    string             // table is the underlying table name of the DAO.
	group    string             // group is the database configuration group name of the current DAO.
	columns  MdlModelsColumns   // columns contains all the column names of Table for convenient usage.
	handlers []gdb.ModelHandler // handlers for customized model modification.
}

// MdlModelsColumns defines and stores column names for the table mdl_models.
type MdlModelsColumns struct {
	Id               string // 主键ID
	ModelId          string // 模型唯一标识（如 gpt-4o、claude-3-5-sonnet）
	ModelName        string // 模型显示名称（如 GPT-4o、Claude 3.5 Sonnet）
	Category         string // 模型分类：chat（对话）/ embedding（嵌入）/ image（图像）/ audio（音频）/ rerank（重排序）
	Status           string // 状态：active（可用）/ deprecated（已废弃）/ offline（已下线）
	MaxContextTokens string // 最大上下文 token 数
	MaxOutputTokens  string // 最大输出 token 数
	Description      string // 模型描述
	Tags             string // 标签（如 reasoning、vision、function_calling）
	Capabilities     string // 模型能力特性（如 vision、function_calling、reasoning 等）
	CreatedAt        string // 创建时间
	UpdatedAt        string // 更新时间
	DeprecatedAt     string // 标记弃用的时间（NULL表示未弃用）
	SunsetDate       string // 计划下线日期（到达后返回410 Gone，NULL表示未设置）
	ReplacementModel string // 推荐替代模型名（NULL表示无替代）
}

// mdlModelsColumns holds the columns for the table mdl_models.
var mdlModelsColumns = MdlModelsColumns{
	Id:               "id",
	ModelId:          "model_id",
	ModelName:        "model_name",
	Category:         "category",
	Status:           "status",
	MaxContextTokens: "max_context_tokens",
	MaxOutputTokens:  "max_output_tokens",
	Description:      "description",
	Tags:             "tags",
	Capabilities:     "capabilities",
	CreatedAt:        "created_at",
	UpdatedAt:        "updated_at",
	DeprecatedAt:     "deprecated_at",
	SunsetDate:       "sunset_date",
	ReplacementModel: "replacement_model",
}

// NewMdlModelsDao creates and returns a new DAO object for table data access.
func NewMdlModelsDao(handlers ...gdb.ModelHandler) *MdlModelsDao {
	return &MdlModelsDao{
		group:    "default",
		table:    "mdl_models",
		columns:  mdlModelsColumns,
		handlers: handlers,
	}
}

// DB retrieves and returns the underlying raw database management object of the current DAO.
func (dao *MdlModelsDao) DB() gdb.DB {
	return g.DB(dao.group)
}

// Table returns the table name of the current DAO.
func (dao *MdlModelsDao) Table() string {
	return dao.table
}

// Columns returns all column names of the current DAO.
func (dao *MdlModelsDao) Columns() MdlModelsColumns {
	return dao.columns
}

// Group returns the database configuration group name of the current DAO.
func (dao *MdlModelsDao) Group() string {
	return dao.group
}

// Ctx creates and returns a Model for the current DAO. It automatically sets the context for the current operation.
func (dao *MdlModelsDao) Ctx(ctx context.Context) *gdb.Model {
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
func (dao *MdlModelsDao) Transaction(ctx context.Context, f func(ctx context.Context, tx gdb.TX) error) (err error) {
	return dao.Ctx(ctx).Transaction(ctx, f)
}
