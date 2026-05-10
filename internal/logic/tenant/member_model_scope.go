package tenant

import (
	"context"
	"github.com/qianfree/team-api/internal/dao"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/frame/g"
	do "github.com/qianfree/team-api/internal/model/do"

	v1 "github.com/qianfree/team-api/api/tenant/v1"
)

// MemberModelScopes returns the model IDs available for a member.
func (s *sTenant) MemberModelScopes(ctx context.Context, req *v1.TenantMemberModelScopesReq) (*v1.TenantMemberModelScopesRes, error) {
	tenantID := ctxTenantID(ctx)

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
		ids = append(ids, r.ModelID)
	}
	return &v1.TenantMemberModelScopesRes{ModelIDs: ids}, nil
}

// MemberModelScopesSet sets the available models for a member (full replace).
func (s *sTenant) MemberModelScopesSet(ctx context.Context, req *v1.TenantMemberModelScopesSetReq) (*v1.TenantMemberModelScopesSetRes, error) {
	tenantID := ctxTenantID(ctx)

	err := g.DB().Transaction(ctx, func(ctx context.Context, tx gdb.TX) error {
		// Delete existing scopes
		_, err := tx.Model("tnt_member_model_scopes").Ctx(ctx).
			Where("tenant_id", tenantID).
			Where("user_id", req.Id).
			Delete()
		if err != nil {
			return err
		}

		// Insert new scopes
		if len(req.ModelIDs) > 0 {
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
		}

		return nil
	})
	if err != nil {
		return nil, err
	}

	return &v1.TenantMemberModelScopesSetRes{}, nil
}

// revokeMemberModels removes specified models from a member and cascades to their API keys.
func revokeMemberModels(ctx context.Context, tenantID, userID int64, modelIDs []int64) error {
	if len(modelIDs) == 0 {
		return nil
	}

	return g.DB().Transaction(ctx, func(ctx context.Context, tx gdb.TX) error {
		// Remove from member model scopes
		_, err := tx.Model("tnt_member_model_scopes").Ctx(ctx).
			Where("tenant_id", tenantID).
			Where("user_id", userID).
			WhereIn("model_id", modelIDs).
			Delete()
		if err != nil {
			return err
		}

		// Cascade: remove model bindings from all keys owned by this user
		keyIDs, err := tx.Model("api_keys").Ctx(ctx).
			Where("tenant_id", tenantID).
			Where("user_id", userID).
			Where("status", "active").
			Fields("id").
			Array()
		if err != nil {
			return err
		}

		if len(keyIDs) > 0 {
			_, err = tx.Model("api_key_model_scopes").Ctx(ctx).
				WhereIn("api_key_id", keyIDs).
				WhereIn("model_id", modelIDs).
				Delete()
			if err != nil {
				return err
			}
		}

		return nil
	})
}
