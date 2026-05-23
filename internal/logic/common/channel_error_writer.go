package common

import (
	"time"
)

// DefaultChannelErrorWriter 渠道错误事件全局写入器
var DefaultChannelErrorWriter *UsageLogWriter

// InitChannelErrorWriter 初始化渠道错误事件写入器
func InitChannelErrorWriter() {
	DefaultChannelErrorWriter = NewUsageLogWriter(UsageLogWriterConfig{
		Table:         "chn_error_events",
		QueueSize:     4096,
		BatchSize:     32,
		FlushInterval: 2 * time.Second,
		Workers:       2,
		Overflow:      OverflowDrop,
	})
	DefaultChannelErrorWriter.Start()
}

// CloseChannelErrorWriter 关闭渠道错误事件写入器
func CloseChannelErrorWriter() {
	if DefaultChannelErrorWriter != nil {
		DefaultChannelErrorWriter.Close()
	}
}
