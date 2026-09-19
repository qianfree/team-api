package helper

import "net/http"

// upstreamRequestIDHeaders 上游请求 ID 候选响应头，按序取第一个非空。
// http.Header.Get 按规范形式匹配，大小写不敏感（"x-request-id" 与 "X-Request-Id" 等价）。
var upstreamRequestIDHeaders = []string{
	"X-Oneapi-Request-Id", // new-api 系上游（本项目上游主流），每个响应必带
	"X-Request-Id",        // OpenAI / 火山引擎等
	"Request-Id",          // Anthropic
	"X-Amzn-Requestid",    // AWS BedRock
}

// ExtractUpstreamRequestID 从上游响应头中提取请求 ID，无任何候选头时返回空串。
// 用于排障时在上游日志中定位同一次调用；值截断到 128 字节对齐 bil_usage_logs
// 列宽，避免超长值导致用量落库失败。
func ExtractUpstreamRequestID(h http.Header) string {
	if h == nil {
		return ""
	}
	for _, name := range upstreamRequestIDHeaders {
		if v := h.Get(name); v != "" {
			if len(v) > 128 {
				return v[:128]
			}
			return v
		}
	}
	return ""
}
