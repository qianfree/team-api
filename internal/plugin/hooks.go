package plugin

import (
	"context"
	"sort"
	"sync"

	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/os/glog"
)

// ---------------------------------------------------------------------------
// 内置事件常量
// ---------------------------------------------------------------------------

const (
	// Relay 相关（AI 代理核心流程）
	HookRelayBeforeRequest = "relay.before_request" // 请求转发前（可修改请求、中断转发）
	HookRelayAfterResponse = "relay.after_response" // 响应返回后（可修改响应、记录日志）
	HookRelayOnError       = "relay.on_error"       // 转发失败（告警、重试决策）

	// 计费相关
	HookBillingBeforeDeduct = "billing.before_deduct" // 预扣前（可修改价格、跳过计费）
	HookBillingAfterSettle  = "billing.after_settle"  // 结算后（用量分析、额度告警）
	HookBillingOnRefund     = "billing.on_refund"     // 退款时（退款通知）

	// 渠道相关
	HookChannelBeforeSelect  = "channel.before_select"   // 渠道选择前（影响调度决策）
	HookChannelOnHealthCheck = "channel.on_health_check" // 健康检查（自定义检查逻辑）

	// 租户相关
	HookTenantOnCreate  = "tenant.on_create"  // 租户创建后（初始化租户数据）
	HookTenantOnSuspend = "tenant.on_suspend" // 租户暂停（清理资源）

	// API Key 相关
	HookApiKeyOnCreate = "apikey.on_create" // Key 创建后（通知、初始化）
	HookApiKeyOnVerify = "apikey.on_verify" // Key 验证时（自定义验证逻辑）
)

// ---------------------------------------------------------------------------
// HookEmitter 事件发射器
// ---------------------------------------------------------------------------

// HookEmitter 全局事件发射器。
type HookEmitter struct {
	handlers map[string][]hookEntry
	mu       sync.RWMutex
}

type hookEntry struct {
	PluginName string
	Priority   int
	Handler    HookHandler
}

var globalEmitter = &HookEmitter{
	handlers: make(map[string][]hookEntry),
}

// GlobalEmitter 获取全局事件发射器。
func GlobalEmitter() *HookEmitter {
	return globalEmitter
}

// RegisterHook 注册事件处理器（由插件系统在启动时调用）。
func (e *HookEmitter) RegisterHook(pluginName, event string, priority int, handler HookHandler) {
	e.mu.Lock()
	defer e.mu.Unlock()

	e.handlers[event] = append(e.handlers[event], hookEntry{
		PluginName: pluginName,
		Priority:   priority,
		Handler:    handler,
	})
}

// Emit 同步发射事件（按优先级顺序执行，可被中断）。
func (e *HookEmitter) Emit(ctx context.Context, event string, data g.Map) (HookResult, error) {
	e.mu.RLock()
	handlers := make([]hookEntry, len(e.handlers[event]))
	copy(handlers, e.handlers[event])
	e.mu.RUnlock()

	// 零开销快速路径：无订阅者直接返回
	if len(handlers) == 0 {
		return HookResult{Data: data}, nil
	}

	// 按优先级排序（数字越小越先执行）
	sort.Slice(handlers, func(i, j int) bool {
		return handlers[i].Priority < handlers[j].Priority
	})

	result := HookResult{Data: data}
	for _, entry := range handlers {
		if result.Aborted {
			break
		}
		payload := HookPayload{Event: event, Data: result.Data}
		hr, err := entry.Handler(ctx, payload)
		if err != nil {
			glog.Warningf(ctx, "plugin hook error: plugin=%s event=%s err=%v",
				entry.PluginName, event, err)
			continue
		}
		if hr.Aborted {
			result.Aborted = true
		}
		if hr.Data != nil {
			for k, v := range hr.Data {
				result.Data[k] = v
			}
		}
	}
	return result, nil
}

// EmitAsync 异步发射事件（不阻塞主流程，适用于日志、通知等场景）。
func (e *HookEmitter) EmitAsync(ctx context.Context, event string, data g.Map) {
	go func() {
		_, _ = e.Emit(ctx, event, data)
	}()
}

// RemoveHooks 移除指定插件的所有 Hook 注册。
func (e *HookEmitter) RemoveHooks(pluginName string) {
	e.mu.Lock()
	defer e.mu.Unlock()
	for event, entries := range e.handlers {
		filtered := make([]hookEntry, 0, len(entries))
		for _, entry := range entries {
			if entry.PluginName != pluginName {
				filtered = append(filtered, entry)
			}
		}
		e.handlers[event] = filtered
	}
}
