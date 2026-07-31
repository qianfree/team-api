package dispatchadapter

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/gogf/gf/v2/frame/g"

	lcommon "github.com/qianfree/team-api/internal/logic/common"
	"github.com/qianfree/team-api/relaykit/dispatch"
)

// 路由策略配置项（sys_options key，基线方案 §12）。
// 单一版本化 JSON 对象，替代散落的 channel_scheduler_v2_enabled 等布尔开关（阶段 5 退役）。
const policyOptionKey = "channel_routing_policy"

const policyRefreshInterval = 30 * time.Second

// ValidateRoutingPolicyJSON 校验路由策略 JSON 覆盖串：
// 空串合法（使用全部内置默认）；非空时按「默认值 + 浅覆盖 + Schema 校验」流程验证。
// 供管理后台保存前拦截与 LoadRoutingPolicy 共用，杜绝非法配置入库/生效。
func ValidateRoutingPolicyJSON(raw string) error {
	_, err := buildPolicy(raw)
	return err
}

// buildPolicy 默认值 + JSON 浅覆盖 + 校验（LoadRoutingPolicy 与 ValidateRoutingPolicyJSON 共用）。
func buildPolicy(raw string) (*dispatch.RoutingPolicy, error) {
	pol := dispatch.DefaultRoutingPolicy()
	if raw != "" {
		if err := json.Unmarshal([]byte(raw), pol); err != nil {
			return nil, fmt.Errorf("JSON 解析失败: %w", err)
		}
	}
	if err := pol.Validate(); err != nil {
		return nil, err
	}
	return pol, nil
}

// LoadRoutingPolicy 从 sys_options 加载路由策略：
// 部分字段的 JSON 浅覆盖到默认值之上；Schema 校验失败返回错误（调用方沿用上一份，
// 杜绝 O1 式误配置静默降级）。配置为空时返回默认策略。
func LoadRoutingPolicy(ctx context.Context) (*dispatch.RoutingPolicy, error) {
	return buildPolicy(lcommon.Config().GetString(ctx, policyOptionKey))
}

// StartPolicyRefresher 周期重载策略并热更新到全部协调器（主 + 影子）。
// 非法配置拒绝生效并告警，继续用上一份有效配置。
func StartPolicyRefresher(ctx context.Context, stop <-chan struct{}, cos ...*dispatch.Coordinator) {
	go func() {
		ticker := time.NewTicker(policyRefreshInterval)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				pol, err := LoadRoutingPolicy(ctx)
				if err != nil {
					g.Log().Warningf(ctx, "[Dispatch] 路由策略非法，沿用上一份有效配置: %v", err)
					continue
				}
				for _, co := range cos {
					co.UpdatePolicy(pol)
				}
			case <-stop:
				return
			}
		}
	}()
}
