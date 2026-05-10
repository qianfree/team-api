package billing

import (
	"context"
	"fmt"

	"github.com/gogf/gf/v2/frame/g"

	"github.com/qianfree/team-api/internal/dao"
	"github.com/qianfree/team-api/internal/logic/common"
)

// CheckBalanceWarning 检查余额预警
// 如果租户可用余额低于预警线，发送通知给租户管理员
func CheckBalanceWarning(ctx context.Context, tenantID int64) (bool, float64, float64, error) {
	wallet, err := GetWallet(ctx, tenantID)
	if err != nil {
		return false, 0, 0, err
	}

	available := AvailableBalance(wallet)
	if wallet.WarningThreshold > 0 && available <= wallet.WarningThreshold {
		g.Log().Warningf(ctx, "[BALANCE WARNING] tenant=%d available=%.4f threshold=%.4f",
			tenantID, available, wallet.WarningThreshold)

		// 发送余额预警通知（异步，失败不影响主流程）
		go func() {
			bgCtx := context.Background()
			engine := common.NewNotificationEngine()
			variables := map[string]any{
				"available": fmt.Sprintf("%.4f", available),
				"threshold": fmt.Sprintf("%.4f", wallet.WarningThreshold),
			}
			if notifyErr := engine.SendBroadcast(bgCtx, tenantID, "balance_warning", variables, "owner,admin"); notifyErr != nil {
				g.Log().Errorf(bgCtx, "[BALANCE WARNING] send notification failed: tenant=%d err=%v", tenantID, notifyErr)
			}
		}()

		return true, available, wallet.WarningThreshold, nil
	}

	return false, available, wallet.WarningThreshold, nil
}

// CheckAllBalanceWarnings 检查所有租户余额预警
func CheckAllBalanceWarnings(ctx context.Context) {
	var tenants []struct {
		TenantID int64 `json:"tenant_id"`
	}

	err := dao.TntTenants.Ctx(ctx).
		Where("status", "active").
		Fields("id as tenant_id").
		Scan(&tenants)
	if err != nil {
		g.Log().Errorf(ctx, "check balance warnings: query tenants failed: %v", err)
		return
	}

	for _, t := range tenants {
		_, _, _, _ = CheckBalanceWarning(ctx, t.TenantID)
	}
}
