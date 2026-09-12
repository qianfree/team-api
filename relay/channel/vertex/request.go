package vertex

import "encoding/json"

// Vertex AI 上 Anthropic 模型的请求体与标准 Claude Messages API 有两处差异，
// 二者都由本文件统一处理（adaptor.ConvertRequest 与 PostProcessConvertedRequest
// 共用同一实现，避免 legacy 路径与 relaykit 路径行为漂移）。

// vertexAnthropicVersion Vertex AI 上 Anthropic 模型要求的 API 版本标识。
// 这是 Vertex rawPredict 端点的必填字段（标准 Anthropic API 由 anthropic-version
// 请求头承载，Vertex 改为放进请求体）。
const vertexAnthropicVersion = "vertex-2023-10-16"

// adaptClaudeBodyForVertex 把标准 Claude Messages 请求体改写为 Vertex
// publishers/anthropic/{model}:rawPredict 端点要求的形态：
//   - 注入 anthropic_version（必填）；
//   - 删除 model 字段——模型由 URL 路径指定，体内不接受。
//
// 非 JSON 体原样返回：网关不替上游判定合法性，让上游给出真实错误更利于排查。
// 已带 anthropic_version 的体不覆盖（尊重客户端显式指定的版本）。
func adaptClaudeBodyForVertex(body []byte) []byte {
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(body, &fields); err != nil {
		return body
	}

	delete(fields, "model")
	if _, ok := fields["anthropic_version"]; !ok {
		fields["anthropic_version"] = json.RawMessage(`"` + vertexAnthropicVersion + `"`)
	}

	out, err := json.Marshal(fields)
	if err != nil {
		return body
	}
	return out
}
