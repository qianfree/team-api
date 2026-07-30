package dispatchadapter

import (
	"context"
	"encoding/json"
	"time"

	"github.com/gogf/gf/v2/frame/g"

	lcommon "github.com/qianfree/team-api/internal/logic/common"
	"github.com/qianfree/team-api/relaykit/dispatch"
)

// 路由策略配置项（sys_options key，基线方案 §12）。
// 单一版本化 JSON 对象，替代散落的 channel_scheduler_v2_enabled 等布尔开关（阶段 5 退役）。
const policyOptionKey = "channel_routing_policy"

const policyRefreshInterval = 30 * time.Second

// LoadRoutingPolicy 从 sys_options 加载路由策略：
// 部分字段的 JSON 浅覆盖到默认值之上；Schema 校验失败返回错误（调用方沿用上一份，
// 杜绝 O1 式误配置静默降级）。配置为空时返回默认策略。
func LoadRoutingPolicy(ctx context.Context) (*dispatch.RoutingPolicy, error) {
	pol := dispatch.DefaultRoutingPolicy()
	raw := lcommon.Config().GetString(ctx, policyOptionKey)
	if raw != "" {
		if err := json.Unmarshal([]byte(raw), pol); err != nil {
			return nil, err
		}
	}
	if err := pol.Validate(); err != nil {
		return nil, err
	}
	return pol, nil
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
