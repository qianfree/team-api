package billing

import (
	"context"
	"fmt"

	"github.com/gogf/gf/v2/frame/g"

	"github.com/qianfree/team-api/internal/dao"
	"github.com/qianfree/team-api/internal/logic/common"
)

// publishWebhookEventFn 由 tenant 包在初始化时注入，避免 billing → tenant 循环依赖
var publishWebhookEventFn func(ctx context.Context, tenantID int64, eventType string, payload any) error

// SetPublishWebhookEventFn 注入 webhook 事件发布函数（由 cmd 初始化时调用）
func SetPublishWebhookEventFn(fn func(ctx context.Context, tenantID int64, eventType string, payload any) error) {
	publishWebhookEventFn = fn
}

// CheckBalanceWarning 检查余额预警
// 如果租户可用余额低于预警线，发送通知给租户管理员并投递 webhook 事件
func CheckBalanceWarning(ctx context.Context, tenantID int64) (bool, float64, float64, error) {
	wallet, err := GetWallet(ctx, tenantID)
	if err != nil {
		return false, 0, 0, err
	}

	available := AvailableBalance(wallet)
	if wallet.WarningThreshold > 0 && available <= wallet.WarningThreshold {
		g.Log().Warningf(ctx, "[BALANCE WARNING] tenant=%d available=%.6f threshold=%.6f",
			tenantID, available, wallet.WarningThreshold)

		// 发送站内通知 + webhook 事件（异步，失败不影响主流程）
		go func() {
			bgCtx := context.Background()

			// 站内通知
			engine := common.NewNotificationEngine()
			variables := map[string]any{
				"available": fmt.Sprintf("%.6f", available),
				"threshold": fmt.Sprintf("%.6f", wallet.WarningThreshold),
			}
			if notifyErr := engine.SendBroadcast(bgCtx, tenantID, "balance_warning", variables, "owner,admin"); notifyErr != nil {
				g.Log().Errorf(bgCtx, "[BALANCE WARNING] send notification failed: tenant=%d err=%v", tenantID, notifyErr)
			}

			// Webhook 事件投递
			if publishWebhookEventFn != nil {
				if webhookErr := publishWebhookEventFn(bgCtx, tenantID, "wallet.low_balance", map[string]any{
					"available_balance": available,
					"warning_threshold": wallet.WarningThreshold,
					"wallet_id":         wallet.ID,
				}); webhookErr != nil {
					g.Log().Errorf(bgCtx, "[BALANCE WARNING] publish webhook event failed: tenant=%d err=%v", tenantID, webhookErr)
				}
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
