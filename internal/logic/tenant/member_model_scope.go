package tenant

import (
	"context"
	"fmt"
	"time"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/frame/g"
	"github.com/qianfree/team-api/internal/dao"
	do "github.com/qianfree/team-api/internal/model/do"

	v1 "github.com/qianfree/team-api/api/tenant/v1"
	lcommon "github.com/qianfree/team-api/internal/logic/common"
	"github.com/qianfree/team-api/internal/middleware"
)

// MemberModelScopes returns the model IDs available for a member.
func (s *sTenant) MemberModelScopes(ctx context.Context, req *v1.TenantMemberModelScopesReq) (*v1.TenantMemberModelScopesRes, error) {
	tenantID := middleware.GetTenantID(ctx)

	var rows []struct {
		ModelID int64 `json:"model_id"`
	}
	err := dao.TntMemberModelScopes.Ctx(ctx).
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

// memberModelScopeCache 成员模型范围缓存（与 relay provider 中的缓存 key 相同）
var memberModelScopeCache = lcommon.NewCache("member_model", 60*time.Second)

// MemberModelScopesSet sets the available models for a member (full replace).
func (s *sTenant) MemberModelScopesSet(ctx context.Context, req *v1.TenantMemberModelScopesSetReq) (*v1.TenantMemberModelScopesSetRes, error) {
	tenantID := middleware.GetTenantID(ctx)

	err := g.DB().Transaction(ctx, func(ctx context.Context, tx gdb.TX) error {
		// Delete existing scopes
		_, err := tx.Model("tnt_member_model_scopes").Ctx(ctx).
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
			_, err = tx.Model("tnt_member_model_scopes").Ctx(ctx).Data(data).Insert()
			if err != nil {
				return err
			}
		} else {
			// 空列表表示禁止所有模型，插入哨兵记录（model_id = -1）
			_, err = tx.Model("tnt_member_model_scopes").Ctx(ctx).Data(do.TntMemberModelScopes{
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
	cacheKey := fmt.Sprintf("%d:%d", tenantID, req.Id)
	memberModelScopeCache.Delete(ctx, cacheKey)

	return &v1.TenantMemberModelScopesSetRes{}, nil
}
