package taskchannel

import (
	"fmt"
	"sync"

	"github.com/qianfree/team-api/relay/common"
	"github.com/qianfree/team-api/relay/constant"
)

// AdaptorFactory 创建 TaskAdaptor 的工厂函数
type AdaptorFactory func() common.TaskAdaptor

var (
	registry   = make(map[constant.ProviderType]AdaptorFactory)
	registryMu sync.RWMutex
)

// Register 注册任务适配器
func Register(providerType constant.ProviderType, factory AdaptorFactory) {
	registryMu.Lock()
	defer registryMu.Unlock()
	registry[providerType] = factory
}

// GetAdaptor 根据供应商类型获取任务适配器
func GetAdaptor(providerType constant.ProviderType) (common.TaskAdaptor, error) {
	registryMu.RLock()
	factory, ok := registry[providerType]
	registryMu.RUnlock()

	if !ok {
		return nil, fmt.Errorf("no task adaptor registered for provider type %d (%s)", providerType, providerType.String())
	}
	return factory(), nil
}

// GetRegisteredPlatforms 返回所有已注册的平台类型
func GetRegisteredPlatforms() []constant.ProviderType {
	registryMu.RLock()
	defer registryMu.RUnlock()

	platforms := make([]constant.ProviderType, 0, len(registry))
	for pt := range registry {
		platforms = append(platforms, pt)
	}
	return platforms
}
