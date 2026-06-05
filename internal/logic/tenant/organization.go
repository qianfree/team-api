package tenant

import (
	"context"

	"github.com/gogf/gf/v2/frame/g"

	"github.com/qianfree/team-api/internal/dao"
	"github.com/qianfree/team-api/internal/logic/billing"
	"github.com/qianfree/team-api/internal/logic/common"
	"github.com/qianfree/team-api/internal/middleware"
	"github.com/qianfree/team-api/internal/utility/crypto"

	do "github.com/qianfree/team-api/internal/model/do"

	"github.com/gogf/gf/v2/database/gdb"

	v1 "github.com/qianfree/team-api/api/tenant/v1"
)

// ownerOnly checks if the current user is the tenant owner.
func ownerOnly(ctx context.Context) error {
	if middleware.GetUserRole(ctx) != "owner" {
		return common.NewForbiddenError("仅组织所有者可执行此操作")
	}
	return nil
}

// GetOrgInfo returns tenant organization info.
func (s *sTenant) GetOrgInfo(ctx context.Context, req *v1.TenantOrgInfoReq) (*v1.TenantOrgInfoRes, error) {
	tenantID := middleware.GetTenantID(ctx)

	var tenant *struct {
		Id         int64  `json:"id"`
		Name       string `json:"name"`
		Code       string `json:"code"`
		LogoUrl    string `json:"logo_url"`
		Status     string `json:"status"`
		Level      int    `json:"level"`
		MaxMembers *int   `json:"max_members"`
		CreatedAt  string `json:"created_at"`
	}
	err := dao.TntTenants.Ctx(ctx).
		Where("id", tenantID).Scan(&tenant)
	if err = common.IgnoreScanNoRows(err); err != nil {
		return nil, err
	}
	if tenant == nil {
		return nil, common.NewNotFoundError("租户")
	}

	memberCount, err := dao.TntUsers.Ctx(ctx).
		Where("tenant_id", tenantID).
		Where("status", "active").
		Count()
	if err != nil {
		return nil, err
	}

	// Look up level name
	var levelName *string
	if tenant.Level > 0 {
		err = dao.TntTenantLevelConfigs.Ctx(ctx).
			Where("level", tenant.Level).
			Fields("name").
			Scan(&levelName)
		if err != nil {
			g.Log().Warningf(ctx, "查询租户等级名称失败: %v", err)
		}
	}

	// 计算实际生效的成员数上限（NULL时取等级配置）
	effectiveMaxMembers := 10
	if tenant.MaxMembers != nil {
		effectiveMaxMembers = *tenant.MaxMembers
	} else {
		effectiveMaxMembers, _, _ = billing.GetTenantEffectiveLimits(ctx, tenantID)
	}

	return &v1.TenantOrgInfoRes{
		ID:      tenant.Id,
		Name:    tenant.Name,
		Code:    tenant.Code,
		LogoURL: tenant.LogoUrl,
		Status:  tenant.Status,
		Level:   tenant.Level,
		LevelName: func() string {
			if levelName != nil {
				return *levelName
			}
			return ""
		}(),
		MaxMembers:  effectiveMaxMembers,
		MemberCount: int(memberCount),
		CreatedAt:   tenant.CreatedAt,
	}, nil
}

// UpdateOrgInfo updates tenant organization info.
func (s *sTenant) UpdateOrgInfo(ctx context.Context, req *v1.TenantOrgUpdateReq) (*v1.TenantOrgUpdateRes, error) {
	if err := ownerOnly(ctx); err != nil {
		return nil, err
	}
	tenantID := middleware.GetTenantID(ctx)

	data := do.TntTenants{}
	if req.Name != nil {
		data.Name = *req.Name
	}
	if req.LogoURL != nil {
		data.LogoUrl = *req.LogoURL
	}

	_, err := dao.TntTenants.Ctx(ctx).Where("id", tenantID).Data(data).Update()
	if err != nil {
		return nil, err
	}
	return nil, nil
}

// TransferOwnership transfers tenant ownership to another member.
func (s *sTenant) TransferOwnership(ctx context.Context, req *v1.TenantOrgTransferReq) (*v1.TenantOrgTransferRes, error) {
	if err := ownerOnly(ctx); err != nil {
		return nil, err
	}
	tenantID := middleware.GetTenantID(ctx)
	currentOwnerID := middleware.GetUserID(ctx)

	if req.NewOwnerID == currentOwnerID {
		return nil, common.NewBadRequestError("不能转让给自己")
	}

	// Verify current owner password
	var currentUser *struct {
		PasswordHash string `json:"password_hash"`
	}
	err := dao.TntUsers.Ctx(ctx).
		Where("id", currentOwnerID).
		Where("tenant_id", tenantID).
		Scan(&currentUser)
	if err = common.IgnoreScanNoRows(err); err != nil {
		return nil, err
	}
	if currentUser == nil {
		return nil, common.NewNotFoundError("用户")
	}

	if !crypto.VerifyPassword(req.Password, currentUser.PasswordHash) {
		return nil, common.NewBusinessError(10023, "密码错误")
	}

	// Check new owner exists and is an active member
	var newOwner *struct {
		ID   int64  `json:"id"`
		Role string `json:"role"`
	}
	err = dao.TntUsers.Ctx(ctx).
		Where("id", req.NewOwnerID).
		Where("tenant_id", tenantID).
		Where("status", "active").
		Scan(&newOwner)
	if err = common.IgnoreScanNoRows(err); err != nil {
		return nil, err
	}
	if newOwner == nil {
		return nil, common.NewNotFoundError("用户")
	}

	err = dao.TntTenants.Transaction(ctx, func(ctx context.Context, tx gdb.TX) error {
		// Demote current owner to admin
		_, err := tx.Model("tnt_users").Ctx(ctx).
			Where("id", currentOwnerID).
			Data(do.TntUsers{
				Role: "admin",
			}).Update()
		if err != nil {
			return err
		}

		// Promote new owner
		_, err = tx.Model("tnt_users").Ctx(ctx).
			Where("id", req.NewOwnerID).
			Data(do.TntUsers{
				Role: "owner",
			}).Update()
		if err != nil {
			return err
		}

		// Update tenant owner reference
		_, err = tx.Model("tnt_tenants").Ctx(ctx).
			Where("id", tenantID).
			Data(do.TntTenants{
				OwnerUserId: req.NewOwnerID,
			}).Update()
		if err != nil {
			return err
		}

		return nil
	})
	if err != nil {
		return nil, err
	}

	return nil, nil
}

// GetProfile returns current user's profile.
func (s *sTenant) GetProfile(ctx context.Context, req *v1.TenantProfileReq) (*v1.TenantProfileRes, error) {
	userID := middleware.GetUserID(ctx)

	var user *struct {
		Id          int64  `json:"id"`
		Username    string `json:"username"`
		Email       string `json:"email"`
		DisplayName string `json:"display_name"`
		Role        string `json:"role"`
		Status      string `json:"status"`
		CreatedAt   string `json:"created_at"`
	}
	err := dao.TntUsers.Ctx(ctx).
		Where("id", userID).Scan(&user)
	if err = common.IgnoreScanNoRows(err); err != nil {
		return nil, err
	}
	if user == nil {
		return nil, common.NewNotFoundError("用户")
	}

	return &v1.TenantProfileRes{
		ID:          user.Id,
		Username:    user.Username,
		Email:       user.Email,
		DisplayName: user.DisplayName,
		Role:        user.Role,
		Status:      user.Status,
		CreatedAt:   user.CreatedAt,
	}, nil
}

// UpdateProfile updates current user's profile.
func (s *sTenant) UpdateProfile(ctx context.Context, req *v1.TenantProfileUpdateReq) (*v1.TenantProfileUpdateRes, error) {
	userID := middleware.GetUserID(ctx)

	data := do.TntUsers{}
	if req.DisplayName != nil {
		data.DisplayName = *req.DisplayName
	}

	_, err := dao.TntUsers.Ctx(ctx).Where("id", userID).Data(data).Update()
	if err != nil {
		return nil, err
	}
	return nil, nil
}
