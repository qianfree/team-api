package v1

import "github.com/gogf/gf/v2/frame/g"

// ChannelTestReq 渠道测试请求
type ChannelTestReq struct {
	g.Meta    `path:"/channels/{id}/test" method:"post" mime:"json" tags:"管理后台-渠道" summary:"测试渠道可用性"`
	ID        int64  `json:"id" in:"path" v:"required" dc:"渠道ID"`
	ModelName string `json:"model_name" dc:"测试模型名（可选，默认使用渠道的 test_model）"`
}

// ChannelTestRes 渠道测试响应
type ChannelTestRes struct {
	Success   bool                  `json:"success"`
	Latency   int64                 `json:"latency_ms"`
	ModelName string                `json:"model_name"`
	Error     string                `json:"error,omitempty"`
	Request   *ChannelTestReqDetail `json:"request,omitempty"`
	Response  string                `json:"response,omitempty"`
}

// ChannelTestReqDetail 测试请求详情
type ChannelTestReqDetail struct {
	Method  string            `json:"method"`
	URL     string            `json:"url"`
	Headers map[string]string `json:"headers"`
	Body    string            `json:"body"`
}
