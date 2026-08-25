package monitor

import (
	"time"

	relaycommon "github.com/qianfree/team-api/relay/common"
)

// RelaykitConverterObserver 适配 relay/common.ConverterObserver：
// 转换指标仍经包级 TrackConverterCall（relaykitT 计数器）上报，仅反转依赖方向。
type RelaykitConverterObserver struct{}

// TrackConverterCall 实现 relay/common.ConverterObserver。
func (RelaykitConverterObserver) TrackConverterCall(converterID, from, to string, duration time.Duration, err error) {
	TrackConverterCall(converterID, from, to, duration, err)
}

// TrackConversionDegradation 实现 relay/common.ConverterObserver。
func (RelaykitConverterObserver) TrackConversionDegradation(converterID, reason string, count int64) {
	TrackConversionDegradation(converterID, reason, count)
}

var _ relaycommon.ConverterObserver = RelaykitConverterObserver{}
