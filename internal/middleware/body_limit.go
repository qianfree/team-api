package middleware

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"sync"

	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/net/ghttp"
	"github.com/gogf/gf/v2/os/gfile"
)

// GoFrame 未配置 clientMaxBodySize 时的默认请求体上限（与 ghttp 内部默认一致）
const defaultClientMaxBodySize int64 = 8 * 1024 * 1024

var (
	relayBodyLimitOnce  sync.Once
	relayBodyLimitBytes int64
)

// relayClientMaxBodySize 读取 server.clientMaxBodySize 配置，与 GoFrame Server
// 使用完全相同的键和解析方式（gfile.StrToSize，支持 "32MB" 等写法），保证预检阈值
// 与 http.MaxBytesReader 实际生效的上限一致，不会出现预检放过、实际读取仍 500 的情况。
// 未配置或解析失败时回退到 GoFrame 默认 8MB。
func relayClientMaxBodySize() int64 {
	relayBodyLimitOnce.Do(func() {
		relayBodyLimitBytes = defaultClientMaxBodySize
		if v, err := g.Cfg().Get(context.Background(), "server.clientMaxBodySize"); err == nil && v != nil {
			if s := v.String(); s != "" {
				if n := gfile.StrToSize(s); n > 0 {
					relayBodyLimitBytes = n
				}
			}
		}
	})
	return relayBodyLimitBytes
}

// RelayBodyLimit 在读取请求体之前按 Content-Length 预检大小，超限直接返回 413。
//
// 背景：GoFrame 会用 http.MaxBytesReader 包装请求体，超限时 r.GetBody() 在
// MakeBodyRepeatableRead 里直接 panic，被框架中间件链捕获后客户端只会收到
// 裸的 500 "Internal Error"。此中间件把超限请求转为带明确语义的 413 + 协议原生
// 错误格式（/v1/messages 走 Claude 格式，其余走 OpenAI 格式），并省去鉴权后的
// 内容过滤等无效工作。
//
// 局限：无 Content-Length 的分块传输无法预检，仍会走 GoFrame 的 500 兜底（罕见，
// 主流 SDK 均携带 Content-Length）。
func RelayBodyLimit(r *ghttp.Request) {
	limit := relayClientMaxBodySize()
	if r.ContentLength <= limit {
		r.Middleware.Next()
		return
	}

	g.Log().Warningf(r.Context(),
		"[RelayBodyLimit] 请求体超限已拒绝: path=%s, content_length=%d, limit=%d, client_ip=%s, ua=%s",
		r.URL.Path, r.ContentLength, limit, r.GetClientIp(), r.Header.Get("User-Agent"))

	// 先排空再响应：客户端此时往往还在上传（本例中 ZCode 上传耗时 53s），
	// 若不等上传完成就抢先返回并关连接，413 响应体会在半路被掐掉，客户端
	// 只能看到连接重置。r.Body 已被 http.MaxBytesReader 封顶在上限值，
	// 排空量有界（超限后读返回错误即停），不会被恶意大包无限消耗带宽。
	_, _ = io.Copy(io.Discard, r.Body)

	writeBodyTooLargeError(r, limit)
}

// writeBodyTooLargeError 按端点协议写出 413 错误响应。
// /v1/messages 系列使用 Claude 格式，其余端点使用 OpenAI 格式。
func writeBodyTooLargeError(r *ghttp.Request, limit int64) {
	r.Response.Header().Set("Content-Type", "application/json")
	r.Response.WriteHeader(http.StatusRequestEntityTooLarge)

	// 与 Anthropic 官方 413 错误类型对齐
	msg := "请求体过大，单次请求最大允许 " + gfile.FormatSize(limit)

	var body []byte
	if strings.HasPrefix(r.URL.Path, "/v1/messages") {
		body, _ = json.Marshal(map[string]any{
			"type": "error",
			"error": map[string]string{
				"type":    "request_too_large",
				"message": msg,
			},
		})
	} else {
		body, _ = json.Marshal(map[string]any{
			"error": map[string]string{
				"type":    "invalid_request_error",
				"message": msg,
			},
		})
	}
	r.Response.Write(body)
}
