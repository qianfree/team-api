package common

import (
	"context"
	"sync"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/frame/g"
)

const auditDBGroup = "audit"

var (
	auditDBConfigured     bool
	auditDBConfiguredOnce sync.Once
)

// GetAuditDB 返回审计数据库连接。
// 如果配置了 database.audit 分组则使用独立审计库，否则回退到主库。
func GetAuditDB() gdb.DB {
	if !IsAuditDBConfigured() {
		return g.DB()
	}
	return g.DB(auditDBGroup)
}

// IsAuditDBConfigured 检查是否配置了独立的审计数据库。
// 通过检查配置文件中是否存在 database.audit.link 来判断。
func IsAuditDBConfigured() bool {
	auditDBConfiguredOnce.Do(func() {
		cfg := g.Cfg()
		if cfg == nil {
			return
		}
		auditLink := cfg.MustGet(context.Background(), "database.audit.link")
		auditDBConfigured = !auditLink.IsEmpty()
	})
	return auditDBConfigured
}

// AuditModelCtx 返回指定审计表的 Model（带 context）。
// 仅 aud_request_logs（大模型请求审计）使用独立审计库（配置了 database.audit 时）；
// 其余审计表（aud_operation_logs、aud_login_history、aud_sensitive_access_logs、
// aud_content_filter_logs）始终使用主库，数据量小无需分离。
// 使用示例：
//
//	common.AuditModelCtx(ctx, "aud_request_logs").Data(data).Insert()
func AuditModelCtx(ctx context.Context, table string) *gdb.Model {
	if table == "aud_request_logs" {
		return GetAuditDB().Model(table).Safe().Ctx(ctx)
	}
	return g.DB().Model(table).Safe().Ctx(ctx)
}
