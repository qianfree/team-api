package shared

import (
	"bytes"
	"encoding/json"
)

// chat 上游 SSE 流的通用防线：各「chat 上游 → X 客户端」流式转换器共用，
// 任一方向漏掉都会把上游故障包装成客户端眼中的正常结束。

// ExtractStreamEmbeddedError 检测 SSE data 行中内嵌的上游错误对象（存在 "error" 键且值非 null）。
//
// 部分聚合商出错时返回 HTTP 200 + SSE，错误信息夹在 data 行里（{"error":{...}}）。
// 这种行能被 chat chunk 结构体成功解析（只是没有 choices），不识别就会被当作无效 chunk
// 静默跳过，随后上游关流，转换器按「正常结束」收尾——客户端表现为"回答到一半正常停止"。
// 排除 "error":null：部分供应商的正常 chunk 会携带空 error 字段。
func ExtractStreamEmbeddedError(data []byte) (json.RawMessage, bool) {
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, false
	}
	errBody, ok := raw["error"]
	if !ok {
		return nil, false
	}
	trimmed := string(bytes.TrimSpace(errBody))
	if trimmed == "" || trimmed == "null" {
		return nil, false
	}
	return errBody, true
}
