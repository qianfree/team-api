package common

import (
	"context"
	"sync"

	"github.com/qianfree/team-api/relaykit/relayconvert/convmeta"
)

// convOptionsProvider 宿主配置注入钩子（internal → relay 的单向依赖）：
// relay/common 不 import internal/，转换选项的系统配置由宿主在启动时注入读取函数，
// 未注入时使用包内默认值（见 RelayInfo.buildConvOptions）。
//
// 与 dispatchadapter 的 policyFn 闭包模式同理：纯库层经函数值获取宿主状态。
var (
	convOptionsMu       sync.RWMutex
	convOptionsProvider func(ctx context.Context) *convmeta.Options
)

// SetConvOptionsProvider 注入转换选项构建函数（进程启动时调用一次，早于流量进入）。
// fn 返回的 Options 由 RelayInfo 按请求惰性构建并缓存（ConvOptions 懒初始化）。
func SetConvOptionsProvider(fn func(ctx context.Context) *convmeta.Options) {
	convOptionsMu.Lock()
	defer convOptionsMu.Unlock()
	convOptionsProvider = fn
}

// convOptionsFromProvider 经钩子构建转换选项；未注入返回 nil（调用方回退默认值）。
func convOptionsFromProvider(ctx context.Context) *convmeta.Options {
	convOptionsMu.RLock()
	fn := convOptionsProvider
	convOptionsMu.RUnlock()
	if fn == nil {
		return nil
	}
	return fn(ctx)
}
