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

// CheckBalanceWarning 检查余额预警（事件驱动，由结算后调用）
// 仅在余额 ≤ 阈值 且尚未推送过时触发一次通知 + webhook，避免重复推送
// 充值恢复后由 ResetLowBalanceNotified 重置标记，允许下次再触发
func CheckBalanceWarning(ctx context.Context, tenantID int64) {
	wallet, err := GetWallet(ctx, tenantID)
	if err != nil {
		g.Log().Warningf(ctx, "[BALANCE WARNING] get wallet failed: tenant=%d err=%v", tenantID, err)
		return
	}

	available := AvailableBalance(wallet)

	// 阈值为 0 表示未启用预警
	if wallet.WarningThreshold <= 0 {
		return
	}

	// 余额仍高于阈值，无需预警
	if available > wallet.WarningThreshold {
		return
	}

	// 已推送过，跳过（等充值恢复后重置标记才能再次推送）
	if wallet.LowBalanceNotified {
		return
	}

	g.Log().Warningf(ctx, "[BALANCE WARNING] tenant=%d available=%.6f threshold=%.6f (first trigger)",
		tenantID, available, wallet.WarningThreshold)

	// 标记已推送（先写 DB，防止并发重复推送）
	_, err = g.DB().Ctx(ctx).Exec(ctx,
		"UPDATE bil_wallets SET low_balance_notified = true, updated_at = NOW() WHERE id = ?", wallet.ID)
	if err != nil {
		g.Log().Errorf(ctx, "[BALANCE WARNING] mark notified failed: tenant=%d err=%v", tenantID, err)
		return
	}
	// 清除缓存，确保下次读取到最新状态
	walletCache.Delete(ctx, fmt.Sprintf("%d", tenantID))

	// 异步发送站内通知 + webhook 事件（失败不影响主流程）
	go func() {
		bgCtx := context.Background()

		// 查询租户信息（用于通知内容）
		var tenantInfo *struct {
			Code string `json:"code"`
			Name string `json:"name"`
		}
		_ = dao.TntTenants.Ctx(bgCtx).
			Where("id", tenantID).
			Fields("code, name").
			Scan(&tenantInfo)

		// 站内通知
		engine := common.NewNotificationEngine()
		variables := map[string]any{
			"tenant_code": "",
			"tenant_name": "",
			"available":   fmt.Sprintf("%.6f", available),
			"threshold":   fmt.Sprintf("%.6f", wallet.WarningThreshold),
		}
		if tenantInfo != nil {
			variables["tenant_code"] = tenantInfo.Code
			variables["tenant_name"] = tenantInfo.Name
		}
		if notifyErr := engine.SendBroadcast(bgCtx, tenantID, "balance_warning", variables, "owner,admin"); notifyErr != nil {
			g.Log().Errorf(bgCtx, "[BALANCE WARNING] send notification failed: tenant=%d err=%v", tenantID, notifyErr)
		}

		// Webhook 事件投递
		if publishWebhookEventFn != nil {
			payload := map[string]any{
				"available_balance": available,
				"warning_threshold": wallet.WarningThreshold,
				"wallet_id":         wallet.ID,
			}
			if tenantInfo != nil {
				payload["tenant_code"] = tenantInfo.Code
				payload["tenant_name"] = tenantInfo.Name
			}
			if webhookErr := publishWebhookEventFn(bgCtx, tenantID, "wallet.low_balance", payload); webhookErr != nil {
				g.Log().Errorf(bgCtx, "[BALANCE WARNING] publish webhook event failed: tenant=%d err=%v", tenantID, webhookErr)
			}
		}
	}()
}

// ResetLowBalanceNotified 充值后检查：如果余额已恢复到阈值以上，重置 low_balance_notified 标记
// 允许下次余额再低于阈值时重新触发预警
func ResetLowBalanceNotified(ctx context.Context, tenantID int64) {
	wallet, err := GetWallet(ctx, tenantID)
	if err != nil {
		return
	}

	// 没有被标记过，无需重置
	if !wallet.LowBalanceNotified {
		return
	}

	// 余额仍低于阈值，不重置
	available := AvailableBalance(wallet)
	if available <= wallet.WarningThreshold {
		return
	}

	g.Log().Infof(ctx, "[BALANCE WARNING] tenant=%d balance restored (%.6f > threshold %.6f), resetting notified flag",
		tenantID, available, wallet.WarningThreshold)

	_, err = g.DB().Ctx(ctx).Exec(ctx,
		"UPDATE bil_wallets SET low_balance_notified = false, updated_at = NOW() WHERE id = ?", wallet.ID)
	if err != nil {
		g.Log().Errorf(ctx, "[BALANCE WARNING] reset notified failed: tenant=%d err=%v", tenantID, err)
		return
	}
	walletCache.Delete(ctx, fmt.Sprintf("%d", tenantID))
}
