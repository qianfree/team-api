package types

import (
	"encoding/json"
	"errors"
	"fmt"
)

// ErrProtocolMismatch 哨兵错误：上游流不是转换器预期的协议格式（假成功防护）。
// 流式转换器在「解析了若干 chunk 但无任何有效协议特征」时以
// fmt.Errorf("%w: %d chunks parsed, none contained choices", ErrProtocolMismatch, n)
// 形式包装返回；宿主桥接层用 errors.Is 识别并按 502 上游错误处理
//（复刻 legacy 的 upstream_protocol_mismatch 防护行为）。
var ErrProtocolMismatch = errors.New("upstream protocol mismatch")

// EmbeddedUpstreamError SSE data 行内嵌的上游错误对象（HTTP 200 + {"error":...}）。
// Body 保留原始 error JSON，宿主桥接层在首个事件发出前原样透传给客户端
//（对齐 legacy 透传行为，不合成新错误体），之后只上报错误不再写体。
type EmbeddedUpstreamError struct {
	Body json.RawMessage
}

func (e *EmbeddedUpstreamError) Error() string {
	body := e.Body
	if len(body) > 500 {
		body = body[:500]
	}
	return fmt.Sprintf("upstream embedded error in stream: %s", string(body))
}
