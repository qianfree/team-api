package dispatchadapter

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"sync"
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

// ---------------------------------------------------------------------------
// 租户级策略覆盖（基线方案 §12「全局默认 + 租户覆盖，浅合并」）
// ---------------------------------------------------------------------------

// 租户覆盖存于 sys_options，key = channel_routing_policy:tenant:<租户ID>。
// 暂由 SQL 管理（无管理界面），详见运维手册；解析结果本地缓存 30s（含「无覆盖」的
// 负缓存，避免无覆盖租户每请求穿透 DB）。
const tenantPolicyKeyPrefix = policyOptionKey + ":tenant:"

const tenantPolicyCacheTTL = 30 * time.Second

type tenantPolicyEntry struct {
	pol       *dispatch.RoutingPolicy // nil = 无覆盖（使用全局策略）
	expiresAt time.Time
}

var tenantPolicyCache sync.Map // tenantID(int64) → tenantPolicyEntry

// TenantRoutingPolicy 返回租户的策略覆盖（全局 + 租户两层浅合并后的完整策略）。
// 返回 nil 表示该租户无覆盖（调度会话回落到协调器全局策略）；租户覆盖非法时
// 告警并按无覆盖处理，不影响该租户请求。
func TenantRoutingPolicy(ctx context.Context, tenantID int64) *dispatch.RoutingPolicy {
	if tenantID <= 0 {
		return nil
	}
	if v, ok := tenantPolicyCache.Load(tenantID); ok {
		if entry := v.(tenantPolicyEntry); time.Now().Before(entry.expiresAt) {
			return entry.pol
		}
	}

	var pol *dispatch.RoutingPolicy
	tenantRaw := lcommon.Config().GetString(ctx, tenantPolicyKeyPrefix+strconv.FormatInt(tenantID, 10))
	if tenantRaw != "" {
		globalRaw := lcommon.Config().GetString(ctx, policyOptionKey)
		merged, err := buildTenantPolicy(globalRaw, tenantRaw)
		if err != nil {
			g.Log().Warningf(ctx, "[Dispatch] 租户 %d 路由策略覆盖非法，回落全局策略: %v", tenantID, err)
		} else {
			pol = merged
		}
	}
	tenantPolicyCache.Store(tenantID, tenantPolicyEntry{pol: pol, expiresAt: time.Now().Add(tenantPolicyCacheTTL)})
	return pol
}

// buildTenantPolicy 三层浅合并：内置默认 → 全局覆盖 → 租户覆盖，最后整体校验。
// 全局串非法时忽略全局层（与全局加载路径的告警回退语义一致），租户串非法返回错误。
func buildTenantPolicy(globalRaw, tenantRaw string) (*dispatch.RoutingPolicy, error) {
	pol := dispatch.DefaultRoutingPolicy()
	if globalRaw != "" {
		_ = json.Unmarshal([]byte(globalRaw), pol)
	}
	if err := json.Unmarshal([]byte(tenantRaw), pol); err != nil {
		return nil, fmt.Errorf("JSON 解析失败: %w", err)
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
