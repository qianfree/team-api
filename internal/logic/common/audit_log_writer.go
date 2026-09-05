package common

import (
	"context"
	"time"
)

// DefaultAuditLogWriter 请求审计日志（aud_request_logs）全局批量写入器
var DefaultAuditLogWriter *UsageLogWriter

// InitAuditLogWriter 初始化审计日志批量写入器
// 审计日志与大模型请求 1:1，量级与 bil_usage_logs 相当，必须走批量通道避免单条 INSERT 打爆审计库。
// 配置了 database.audit 独立审计库时经 DB getter 路由过去，否则回退主库（与 AuditModelCtx 一致）。
// 批量参数来自「性能配置-批量写入」（sys_options，重启后生效）；未配置或非法时回落内置默认。
func InitAuditLogWriter(ctx context.Context) {
	batchSize := Config().GetInt(ctx, "batch_write_size")
	if batchSize <= 0 {
		batchSize = 64
	}
	flushInterval := time.Duration(Config().GetInt(ctx, "batch_write_interval_ms")) * time.Millisecond
	if flushInterval <= 0 {
		flushInterval = 1 * time.Second
	}

	DefaultAuditLogWriter = NewUsageLogWriter(UsageLogWriterConfig{
		Table:         "aud_request_logs",
		QueueSize:     8192,
		BatchSize:     batchSize,
		FlushInterval: flushInterval,
		Workers:       2,
		Overflow:      OverflowSyncFallback,
		DB:            GetAuditDB,
	})
	DefaultAuditLogWriter.Start()
}

// CloseAuditLogWriter 关闭审计日志批量写入器（排空队列后退出 worker）
func CloseAuditLogWriter() {
	if DefaultAuditLogWriter != nil {
		DefaultAuditLogWriter.Close()
	}
}
