package tenant

import (
	"context"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/frame/g"
	"github.com/qianfree/team-api/internal/dao"
	do "github.com/qianfree/team-api/internal/model/do"

	v1 "github.com/qianfree/team-api/api/tenant/v1"
	lcommon "github.com/qianfree/team-api/internal/logic/common"
	"github.com/qianfree/team-api/internal/logic/relay"
	"github.com/qianfree/team-api/internal/middleware"
)

// MemberModelScopes returns the model IDs available for a member.
func (s *sTenant) MemberModelScopes(ctx context.Context, req *v1.TenantMemberModelScopesReq) (*v1.TenantMemberModelScopesRes, error) {
	tenantID := middleware.GetTenantID(ctx)

	// 验证用户属于当前租户
	var targetUser *struct {
		Id int64 `json:"id"`
	}
	err := dao.TntUsers.Ctx(ctx).
		Where("id", req.Id).
		Where("tenant_id", tenantID).
		Scan(&targetUser)
	if err != nil {
		return nil, err
	}
	if targetUser == nil {
		return nil, lcommon.NewNotFoundError("成员")
	}

	var rows []struct {
		ModelID int64 `json:"model_id"`
	}
	err = dao.TntMemberModelScopes.Ctx(ctx).
		Where("tenant_id", tenantID).
		Where("user_id", req.Id).
		Scan(&rows)
	if err != nil {
		return nil, err
	}

	ids := make([]int64, 0, len(rows))
	for _, r := range rows {
		if r.ModelID > 0 {
			ids = append(ids, r.ModelID)
		}
	}
	return &v1.TenantMemberModelScopesRes{ModelIDs: ids}, nil
}

// 成员模型范围缓存的「生产者」在 relay.provider（memberModelCache）。
// 本包只在其写库后做失效，故直接调用 relay.InvalidateMemberModelScopeCache，
// 不再在此重复声明同前缀缓存（原 memberModelScopeCache 与 relay 同用 "member_model"
// 前缀，属故意的跨包失效，但字面量重复易被误判为命名冲突）。

// MemberModelScopesSet sets the available models for a member (full replace).
func (s *sTenant) MemberModelScopesSet(ctx context.Context, req *v1.TenantMemberModelScopesSetReq) (*v1.TenantMemberModelScopesSetRes, error) {
	// 仅 owner/admin 可管理成员的模型访问范围，防止普通成员给自己或他人越权开通模型
	role := middleware.GetUserRole(ctx)
	if role != "owner" && role != "admin" {
		return nil, lcommon.NewForbiddenError("需要 owner 或 admin 权限")
	}
	if err := requireTeamEnabled(ctx); err != nil {
		return nil, err
	}
	tenantID := middleware.GetTenantID(ctx)

	// 验证用户属于当前租户
	var targetUser *struct {
		Id int64 `json:"id"`
	}
	err := dao.TntUsers.Ctx(ctx).
		Where("id", req.Id).
		Where("tenant_id", tenantID).
		Scan(&targetUser)
	if err != nil {
		return nil, err
	}
	if targetUser == nil {
		return nil, lcommon.NewNotFoundError("成员")
	}

	// 事务采用 ctx 传播式写法：闭包内统一使用 dao.Xxx.Ctx(ctx)，事务由 ctx 自动挂载。
	err = g.DB().Transaction(ctx, func(ctx context.Context, tx gdb.TX) error {
		// Delete existing scopes
		_, err := dao.TntMemberModelScopes.Ctx(ctx).
			Where("tenant_id", tenantID).
			Where("user_id", req.Id).
			Delete()
		if err != nil {
			return err
		}

		if len(req.ModelIDs) > 0 {
			// Insert new scopes
			data := make([]do.TntMemberModelScopes, len(req.ModelIDs))
			for i, mID := range req.ModelIDs {
				data[i] = do.TntMemberModelScopes{
					TenantId: tenantID,
					UserId:   req.Id,
					ModelId:  mID,
				}
			}
			_, err = dao.TntMemberModelScopes.Ctx(ctx).Data(data).Insert()
			if err != nil {
				return err
			}
		} else {
			// 空列表表示禁止所有模型，插入哨兵记录（model_id = -1）
			_, err = dao.TntMemberModelScopes.Ctx(ctx).Data(do.TntMemberModelScopes{
				TenantId: tenantID,
				UserId:   req.Id,
				ModelId:  -1,
			}).Insert()
			if err != nil {
				return err
			}
		}

		return nil
	})
	if err != nil {
		return nil, err
	}

	// 清除该成员的模型范围缓存，使下次请求时重新从数据库读取
	relay.InvalidateMemberModelScopeCache(ctx, tenantID, req.Id)

	return &v1.TenantMemberModelScopesSetRes{}, nil
}
