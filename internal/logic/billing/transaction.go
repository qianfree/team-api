package billing

import (
	"context"

	"github.com/gogf/gf/v2/database/gdb"

	"github.com/qianfree/team-api/internal/dao"
)

// TransactionQueryParams holds common filter parameters for transaction queries.
type TransactionQueryParams struct {
	TenantID  int64  // 租户 ID，0 表示不过滤
	Type      string // 交易类型
	Username  string // 用户名（模糊匹配）
	ModelName string // 模型名称（模糊匹配）
	StartDate string // 开始日期（YYYY-MM-DD）
	EndDate   string // 结束日期（YYYY-MM-DD）
}

// BuildTransactionQuery 构建 bil_transactions 基础查询，应用通用过滤条件。
// 调用方可继续追加特定过滤、字段选择、JOIN 和排序。
func BuildTransactionQuery(ctx context.Context, params TransactionQueryParams) *gdb.Model {
	query := dao.BilTransactions.Ctx(ctx)

	if params.TenantID > 0 {
		query = query.Where("bil_transactions.tenant_id", params.TenantID)
	}
	if params.Type != "" {
		query = query.Where("bil_transactions.type", params.Type)
	}
	if params.StartDate != "" {
		query = query.Where("bil_transactions.created_at >= ?", params.StartDate+" 00:00:00")
	}
	if params.EndDate != "" {
		query = query.Where("bil_transactions.created_at <= ?", params.EndDate+" 23:59:59")
	}
	if params.Username != "" {
		// 用 EXISTS 子查询而非引用调用方 JOIN 的别名（tu.username）：
		// ① 筛选条件因此不依赖任何 JOIN，调用方的 COUNT / 聚合路径可以完全不联表；
		// ② 避免把 tnt_users 拖成驱动表（EXISTS 内是主键等值探测，代价低）。
		// 语义与 LEFT JOIN 后过滤一致：user_id 无对应用户的行同样被过滤掉。
		query = query.Where("EXISTS (SELECT 1 FROM tnt_users ut WHERE ut.id = bil_transactions.user_id AND ut.tenant_id = bil_transactions.tenant_id AND ut.username LIKE ?)", "%"+params.Username+"%")
	}
	if params.ModelName != "" {
		query = query.Where("bil_transactions.model_name LIKE ?", "%"+params.ModelName+"%")
	}

	return query
}
