package common

import (
	"context"

	"github.com/qianfree/team-api/internal/consts"
	"github.com/qianfree/team-api/internal/dao"
)

// TenantPrincipal contains the current authorization state for a tenant user.
type TenantPrincipal struct {
	Role         string `json:"role"`
	UserStatus   string `json:"user_status"`
	TenantStatus string `json:"tenant_status"`
}

// LoadTenantPrincipal loads a user's current role and both account statuses.
func LoadTenantPrincipal(ctx context.Context, userID, tenantID int64) (*TenantPrincipal, error) {
	var principal *TenantPrincipal
	err := dao.TntUsers.Ctx(ctx).As("u").
		InnerJoin("tnt_tenants t ON t.id = u.tenant_id").
		Where("u.id", userID).
		Where("u.tenant_id", tenantID).
		Fields("u.role, u.status AS user_status, t.status AS tenant_status").
		Scan(&principal)
	if err = IgnoreScanNoRows(err); err != nil {
		return nil, err
	}
	return principal, nil
}

// ValidateTenantPrincipal verifies that both the user and tenant are active.
func ValidateTenantPrincipal(principal *TenantPrincipal) error {
	if principal == nil || principal.UserStatus != "active" {
		return consts.ErrUnauthorized
	}
	if principal.TenantStatus != "active" {
		return consts.ErrTenantSuspended
	}
	return nil
}
