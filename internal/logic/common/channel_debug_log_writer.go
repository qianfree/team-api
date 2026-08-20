package common

import (
	"time"
)

// DefaultChannelDebugLogWriter 渠道调试日志全局写入器
var DefaultChannelDebugLogWriter *UsageLogWriter

// InitChannelDebugLogWriter 初始化渠道调试日志写入器。
// 单条记录可能达数 MB（完整四段报文）：小批量 + 短间隔切批，队列满时丢弃并计数
// （调试日志允许丢，不反压请求；管理员可重发请求补录）。
func InitChannelDebugLogWriter() {
	DefaultChannelDebugLogWriter = NewUsageLogWriter(UsageLogWriterConfig{
		Table:         "chn_debug_logs",
		QueueSize:     256,
		BatchSize:     4,
		FlushInterval: 2 * time.Second,
		Workers:       2,
		Overflow:      OverflowDrop,
	})
	DefaultChannelDebugLogWriter.Start()
}

// CloseChannelDebugLogWriter 关闭渠道调试日志写入器
func CloseChannelDebugLogWriter() {
	if DefaultChannelDebugLogWriter != nil {
		DefaultChannelDebugLogWriter.Close()
	}
}
