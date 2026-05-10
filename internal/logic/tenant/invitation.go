package tenant

import (
	"context"
	"time"

	v1 "github.com/qianfree/team-api/api/tenant/v1"
	"github.com/qianfree/team-api/internal/dao"
	"github.com/qianfree/team-api/internal/logic/common"

	"github.com/gogf/gf/v2/os/gtime"
)

// InvitationList returns a paginated list of invitation records for the tenant.
func (s *sTenant) InvitationList(ctx context.Context, req *v1.TenantInvitationListReq) (*v1.TenantInvitationListRes, error) {
	tenantID := ctxTenantID(ctx)
	page, pageSize := common.NormalizePagination(req.Page, req.PageSize)

	type row struct {
		ID           int64       `json:"id"`
		Code         string      `json:"code"`
		Role         string      `json:"role"`
		ExpiresAt    *gtime.Time `json:"expires_at"`
		MaxUses      int         `json:"max_uses"`
		UseCount     int         `json:"use_count"`
		UsedByUserID int64       `json:"used_by_user_id"`
		CreatedAt    *gtime.Time `json:"created_at"`
		CreatorName  string      `json:"creator_name"`
	}

	model := dao.TntInvitations.Ctx(ctx).
		Where("tnt_invitations.tenant_id", tenantID)

	total, err := model.Count()
	if err != nil {
		return nil, err
	}

	var rows []row
	err = dao.TntInvitations.Ctx(ctx).
		Fields(
			"tnt_invitations.id",
			"tnt_invitations.code",
			"tnt_invitations.role",
			"tnt_invitations.expires_at",
			"tnt_invitations.max_uses",
			"tnt_invitations.use_count",
			"tnt_invitations.used_by_user_id",
			"tnt_invitations.created_at",
			"creator.username as creator_name",
		).
		Where("tnt_invitations.tenant_id", tenantID).
		LeftJoin("tnt_users creator", "creator.id = tnt_invitations.created_by").
		OrderDesc("tnt_invitations.id").
		Page(page, pageSize).
		Scan(&rows)
	if err != nil {
		return nil, err
	}

	now := time.Now()
	items := make([]v1.TenantInvitationItem, len(rows))
	for i, r := range rows {
		status := "active"
		if r.UsedByUserID == -1 {
			status = "revoked"
		} else if r.ExpiresAt != nil && now.After(r.ExpiresAt.Time) {
			status = "expired"
		} else if r.MaxUses > 0 && r.UseCount >= r.MaxUses {
			status = "exhausted"
		}

		item := v1.TenantInvitationItem{
			ID:          r.ID,
			Code:        r.Code[:8] + "***",
			Role:        r.Role,
			Status:      status,
			MaxUses:     r.MaxUses,
			UseCount:    r.UseCount,
			CreatorName: r.CreatorName,
		}
		if r.ExpiresAt != nil {
			item.ExpiresAt = r.ExpiresAt.Format("Y-m-d H:i:s")
		}
		if r.CreatedAt != nil {
			item.CreatedAt = r.CreatedAt.Format("Y-m-d H:i:s")
		}
		if status == "active" {
			item.InviteURL = buildInviteURL(ctx, r.Code)
		}

		items[i] = item
	}

	return &v1.TenantInvitationListRes{
		List:     items,
		Total:    int(total),
		Page:     page,
		PageSize: pageSize,
	}, nil
}

// RevokeInvitation revokes a pending invitation by setting used_by_user_id = -1.
func (s *sTenant) RevokeInvitation(ctx context.Context, req *v1.TenantInvitationRevokeReq) (*v1.TenantInvitationRevokeRes, error) {
	tenantID := ctxTenantID(ctx)

	var inv struct {
		ID           int64 `json:"id"`
		UsedByUserID int64 `json:"used_by_user_id"`
	}
	err := dao.TntInvitations.Ctx(ctx).
		Where("id", req.Id).
		Where("tenant_id", tenantID).
		Scan(&inv)
	if err != nil {
		return nil, err
	}
	if inv.ID == 0 {
		return nil, common.NewBadRequestError("邀请记录不存在")
	}
	if inv.UsedByUserID == -1 {
		return nil, common.NewBadRequestError("该邀请已撤销")
	}

	_, err = dao.TntInvitations.Ctx(ctx).
		Where("id", req.Id).
		Where("tenant_id", tenantID).
		Data("used_by_user_id", -1).
		Update()
	if err != nil {
		return nil, err
	}

	return nil, nil
}

// InviteInfo returns public information about an invitation (no auth required).
func (s *sTenant) InviteInfo(ctx context.Context, req *v1.TenantInviteInfoReq) (*v1.TenantInviteInfoRes, error) {
	var inv struct {
		ID           int64       `json:"id"`
		TenantID     int64       `json:"tenant_id"`
		Role         string      `json:"role"`
		ExpiresAt    *gtime.Time `json:"expires_at"`
		MaxUses      int         `json:"max_uses"`
		UseCount     int         `json:"use_count"`
		UsedByUserID int64       `json:"used_by_user_id"`
	}
	err := dao.TntInvitations.Ctx(ctx).
		Where("code", req.Code).
		Scan(&inv)
	if err != nil {
		return nil, err
	}
	if inv.ID == 0 {
		return &v1.TenantInviteInfoRes{Valid: false}, nil
	}

	// Get tenant name
	var tenant struct {
		Name string `json:"name"`
	}
	err = dao.TntTenants.Ctx(ctx).
		Where("id", inv.TenantID).
		Fields("name").
		Scan(&tenant)
	if err != nil {
		return nil, err
	}

	valid := inv.UsedByUserID != -1
	if inv.MaxUses > 0 && inv.UseCount >= inv.MaxUses {
		valid = false
	}
	if inv.ExpiresAt != nil && time.Now().After(inv.ExpiresAt.Time) {
		valid = false
	}

	res := &v1.TenantInviteInfoRes{
		TenantName: tenant.Name,
		Role:       inv.Role,
		Valid:      valid,
	}
	if inv.ExpiresAt != nil {
		res.ExpiresAt = inv.ExpiresAt.Format("Y-m-d H:i:s")
	}

	return res, nil
}
