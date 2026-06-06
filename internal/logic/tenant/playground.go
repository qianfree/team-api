package tenant

import (
	"context"
	"fmt"
	"time"

	"github.com/gogf/gf/v2/frame/g"

	v1 "github.com/qianfree/team-api/api/tenant/v1"
	"github.com/qianfree/team-api/internal/logic/common"
	"github.com/qianfree/team-api/internal/middleware"
)

// ============================================================
// Sandbox（模拟调用，不计费）
// ============================================================

func (s *sTenant) SandboxChat(ctx context.Context, req *v1.SandboxChatReq) (*v1.SandboxChatRes, error) {
	tenantID := middleware.GetTenantID(ctx)

	now := time.Now()
	quotaKey := fmt.Sprintf("sandbox:quota:%d:%s", tenantID, now.Format("200601"))
	remaining, err := g.Redis().Do(ctx, "GET", quotaKey)
	if err != nil {
		return nil, err
	}

	defaultQuota := g.Cfg().MustGet(ctx, "sandbox.sandbox_default_quota").Int()
	if defaultQuota <= 0 {
		defaultQuota = 100
	}

	remainInt := defaultQuota
	if !remaining.IsNil() && !remaining.IsEmpty() {
		remainInt = remaining.Int()
	}

	if remainInt <= 0 {
		return nil, common.NewBusinessError(10056, "本月沙箱额度已用完")
	}

	_, err = g.Redis().Do(ctx, "DECR", quotaKey)
	if err != nil {
		return nil, err
	}
	if remainInt == defaultQuota {
		_, _ = g.Redis().Do(ctx, "EXPIRE", quotaKey, 86400*30)
	}

	content := generateSimulatedResponse(req.Model, req.Messages)

	return &v1.SandboxChatRes{
		Content:        content,
		IsSandbox:      true,
		RemainingQuota: remainInt - 1,
	}, nil
}

func (s *sTenant) SandboxQuota(ctx context.Context, req *v1.SandboxQuotaReq) (*v1.SandboxQuotaRes, error) {
	tenantID := middleware.GetTenantID(ctx)

	defaultQuota := g.Cfg().MustGet(ctx, "sandbox.sandbox_default_quota").Int()
	if defaultQuota <= 0 {
		defaultQuota = 100
	}

	now := time.Now()
	quotaKey := fmt.Sprintf("sandbox:quota:%d:%s", tenantID, now.Format("200601"))
	remaining, err := g.Redis().Do(ctx, "GET", quotaKey)
	if err != nil {
		return nil, err
	}

	remainInt := defaultQuota
	if !remaining.IsNil() && !remaining.IsEmpty() {
		remainInt = remaining.Int()
	}

	used := defaultQuota - remainInt
	if used < 0 {
		used = 0
	}

	return &v1.SandboxQuotaRes{
		TotalQuota:     defaultQuota,
		RemainingQuota: remainInt,
		UsedQuota:      used,
	}, nil
}

func generateSimulatedResponse(model string, messages []v1.PlaygroundMessage) string {
	userMsg := ""
	for i := len(messages) - 1; i >= 0; i-- {
		if messages[i].Role == "user" {
			userMsg = messages[i].Content
			break
		}
	}
	if userMsg == "" {
		userMsg = "你的请求"
	}
	return fmt.Sprintf("这是来自模型 %s 的模拟响应。您发送的消息是：%q\n\n（沙箱模式：此响应为模拟数据，不会产生实际费用）", model, userMsg)
}
