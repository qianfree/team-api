package common

import (
	"context"
	"io"
	"net/http"
)

// Adaptor 是所有 AI 供应商适配器必须实现的接口。
// 每个方法对应 relay 管线中的一个步骤。
type Adaptor interface {
	// Init 使用渠道元数据初始化适配器
	Init(info *RelayInfo)

	// GetRequestURL 构建上游请求的完整 URL
	GetRequestURL(info *RelayInfo) (string, error)

	// SetupRequestHeader 设置上游请求的 HTTP 头
	SetupRequestHeader(header http.Header, info *RelayInfo) error

	// ConvertRequest 将入站请求体转换为供应商原生格式。
	ConvertRequest(ctx context.Context, info *RelayInfo, requestBody []byte) (io.Reader, error)

	// DoRequest 发送 HTTP 请求到上游供应商
	DoRequest(ctx context.Context, info *RelayInfo, requestBody io.Reader) (*http.Response, error)

	// DoResponse 处理上游响应并写回客户端。
	DoResponse(ctx context.Context, resp *http.Response, info *RelayInfo, writer http.ResponseWriter) (*Usage, error)

	// GetChannelName 返回渠道名称（用于日志）
	GetChannelName() string
}
