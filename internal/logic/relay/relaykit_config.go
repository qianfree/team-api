package relay

import (
	"context"

	"github.com/gogf/gf/v2/frame/g"
)

// relaykit 改造期间运行时特性开关（详见 manifest/config/config.yaml 的 relaykit 段）。
//
// 设计说明：
//   - 采用运行时配置而非编译期 const，是为了支持「无需重编译、改配置 + 重启即回滚」
//     （见 docs/relaykit-migration-plan.md 阶段 4-5 回滚方案）。
//   - 语义：
//     enabled=false                       → 所有请求走旧 relay 代码路径
//     enabled=true 且 providers 为空       → 仍全走旧路径（空白名单 = 全禁用）
//     enabled=true 且 providers 非空       → 仅白名单内的供应商走 relaykit
//   - 本阶段仅提供被调用的接口，尚无 handler 调用方（handler 接入属阶段 4）。

// IsRelaykitEnabled 返回 relaykit 全局开关。缺省 / 解析失败时返回 false（安全默认）。
func IsRelaykitEnabled(ctx context.Context) bool {
	return g.Cfg().MustGet(ctx, "relaykit.enabled").Bool()
}

// RelaykitProviders 读取 relaykit.providers 白名单，转为集合便于 O(1) 查找。
// 配置缺失或为空时返回 nil。
func RelaykitProviders(ctx context.Context) map[string]struct{} {
	items := g.Cfg().MustGet(ctx, "relaykit.providers").Strings()
	if len(items) == 0 {
		return nil
	}
	set := make(map[string]struct{}, len(items))
	for _, p := range items {
		if p == "" {
			continue
		}
		set[p] = struct{}{}
	}
	if len(set) == 0 {
		return nil
	}
	return set
}

// IsRelaykitEnabledForChannel 判断指定供应商是否启用 relaykit 转换器。
// providerKey 为供应商标识（如 "anthropic"/"openai"/"gemini"），
// channelType(int)→providerKey 的映射在阶段 4 handler 接入时完成。
func IsRelaykitEnabledForChannel(ctx context.Context, providerKey string) bool {
	if !IsRelaykitEnabled(ctx) {
		return false
	}
	return isChannelInProviders(providerKey, RelaykitProviders(ctx))
}

// isChannelInProviders 是纯函数，判断 providerKey 是否落在白名单集合内。
// 抽离出来便于单测，且不依赖 g.Cfg 实例。
func isChannelInProviders(providerKey string, providers map[string]struct{}) bool {
	if providerKey == "" || len(providers) == 0 {
		return false
	}
	_, ok := providers[providerKey]
	return ok
}
