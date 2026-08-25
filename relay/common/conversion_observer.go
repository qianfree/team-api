package common

import (
	"sync"
	"time"
)

// ConverterObserver 转换器调用观测接口。由 internal/logic/monitor 实现、
// 启动时经 SetConverterObserver 注入——relay 层经包级 TrackConverterCall 上报，
// 避免 relay → internal/logic/monitor 的反向 import（依赖方向保持 internal → relay）。
type ConverterObserver interface {
	TrackConverterCall(converterID, from, to string, duration time.Duration, err error)
	// TrackConversionDegradation 上报协议转换中的能力降级：converterID 定位转换器，
	// reason 为丢弃原因（含具体输入类型，如 "input_item:additional_tools"），count 为本次聚合数
	TrackConversionDegradation(converterID, reason string, count int64)
}

var (
	converterObserverMu sync.RWMutex
	converterObserver   ConverterObserver
)

// SetConverterObserver 注入观测实现（进程启动时调用一次，早于流量进入）。
func SetConverterObserver(o ConverterObserver) {
	converterObserverMu.Lock()
	defer converterObserverMu.Unlock()
	converterObserver = o
}

// TrackConverterCall 上报一次转换调用的成败与耗时（未注入时 no-op）。
func TrackConverterCall(converterID, from, to string, duration time.Duration, err error) {
	converterObserverMu.RLock()
	o := converterObserver
	converterObserverMu.RUnlock()
	if o != nil {
		o.TrackConverterCall(converterID, from, to, duration, err)
	}
}

// TrackConversionDegradation 上报一次转换降级聚合（未注入时 no-op）。
func TrackConversionDegradation(converterID, reason string, count int64) {
	converterObserverMu.RLock()
	o := converterObserver
	converterObserverMu.RUnlock()
	if o != nil {
		o.TrackConversionDegradation(converterID, reason, count)
	}
}
