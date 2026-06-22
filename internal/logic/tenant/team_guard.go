package tenant

import (
	"context"

	"github.com/qianfree/team-api/internal/consts"
	"github.com/qianfree/team-api/internal/dao"
	"github.com/qianfree/team-api/internal/logic/common"
	"github.com/qianfree/team-api/internal/middleware"
)

// requireTeamEnabled 校验当前认证用户所属租户已启用团队功能。
// 用于 ctx 中携带 tenant_id 的团队写操作接口。
func requireTeamEnabled(ctx context.Context) error {
	return requireTeamEnabledForTenant(ctx, middleware.GetTenantID(ctx))
}

// requireTeamEnabledForTenant 校验指定租户已启用团队功能。
// 用于 JoinByInvite 等公开接口：ctx 无 tenant_id，需在查到目标 tenant_id 后调用。
func requireTeamEnabledForTenant(ctx context.Context, tenantID int64) error {
	if tenantID <= 0 {
		return common.NewForbiddenError("租户不存在")
	}
	enabled, err := dao.TntTenants.Ctx(ctx).
		Where("id", tenantID).Fields("team_enabled").Value()
	if err != nil {
		return err
	}
	if !enabled.Bool() {
		return common.NewBusinessError(consts.CodeTeamNotEnabled, consts.MsgTeamNotEnabled)
	}
	return nil
}
