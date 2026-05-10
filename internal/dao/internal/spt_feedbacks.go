// ==========================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// ==========================================================================

package internal

import (
	"context"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/frame/g"
)

// SptFeedbacksDao is the data access object for the table spt_feedbacks.
type SptFeedbacksDao struct {
	table    string              // table is the underlying table name of the DAO.
	group    string              // group is the database configuration group name of the current DAO.
	columns  SptFeedbacksColumns // columns contains all the column names of Table for convenient usage.
	handlers []gdb.ModelHandler  // handlers for customized model modification.
}

// SptFeedbacksColumns defines and stores column names for the table spt_feedbacks.
type SptFeedbacksColumns struct {
	Id           string // 主键ID
	TenantId     string // 所属租户ID
	UserId       string // 提交用户ID
	Category     string // 反馈类型：bug_report / feature_request / suggestion / complaint
	Title        string // 反馈标题
	Description  string // 反馈详细描述
	Status       string // 状态：pending / acknowledged / in_progress / resolved / closed
	Priority     string // 优先级：low / normal / high / critical
	AdminReply   string // 管理员回复
	AdminReplyBy string // 回复管理员ID
	AdminReplyAt string // 回复时间
	Resolution   string // 解决方案摘要
	Tags         string // 自定义标签（JSON 数组）
	Metadata     string // 元数据（环境信息、截图链接等）
	CreatedAt    string //
	UpdatedAt    string //
}

// sptFeedbacksColumns holds the columns for the table spt_feedbacks.
var sptFeedbacksColumns = SptFeedbacksColumns{
	Id:           "id",
	TenantId:     "tenant_id",
	UserId:       "user_id",
	Category:     "category",
	Title:        "title",
	Description:  "description",
	Status:       "status",
	Priority:     "priority",
	AdminReply:   "admin_reply",
	AdminReplyBy: "admin_reply_by",
	AdminReplyAt: "admin_reply_at",
	Resolution:   "resolution",
	Tags:         "tags",
	Metadata:     "metadata",
	CreatedAt:    "created_at",
	UpdatedAt:    "updated_at",
}

// NewSptFeedbacksDao creates and returns a new DAO object for table data access.
func NewSptFeedbacksDao(handlers ...gdb.ModelHandler) *SptFeedbacksDao {
	return &SptFeedbacksDao{
		group:    "default",
		table:    "spt_feedbacks",
		columns:  sptFeedbacksColumns,
		handlers: handlers,
	}
}

// DB retrieves and returns the underlying raw database management object of the current DAO.
func (dao *SptFeedbacksDao) DB() gdb.DB {
	return g.DB(dao.group)
}

// Table returns the table name of the current DAO.
func (dao *SptFeedbacksDao) Table() string {
	return dao.table
}

// Columns returns all column names of the current DAO.
func (dao *SptFeedbacksDao) Columns() SptFeedbacksColumns {
	return dao.columns
}

// Group returns the database configuration group name of the current DAO.
func (dao *SptFeedbacksDao) Group() string {
	return dao.group
}

// Ctx creates and returns a Model for the current DAO. It automatically sets the context for the current operation.
func (dao *SptFeedbacksDao) Ctx(ctx context.Context) *gdb.Model {
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
func (dao *SptFeedbacksDao) Transaction(ctx context.Context, f func(ctx context.Context, tx gdb.TX) error) (err error) {
	return dao.Ctx(ctx).Transaction(ctx, f)
}
