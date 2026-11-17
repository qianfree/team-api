package shared

import (
	"strings"

	"github.com/qianfree/team-api/relaykit/dto"
)

// openaiReasoningModelPrefixes OpenAI 推理模型族的模型名前缀（小写）。
// 这些模型的 chat completions 端点拒绝 max_tokens（400 "Unsupported parameter: max_tokens
// is not supported with this model. Use max_completion_tokens instead"），只接受
// max_completion_tokens。gpt-oss 等开源推理模型由 vLLM/Ollama 类服务承载，仍走 max_tokens，不在此列。
var openaiReasoningModelPrefixes = []string{"o1", "o3", "o4", "gpt-5", "codex"}

// IsOpenAIReasoningModel 判断模型名是否属于只接受 max_completion_tokens 的 OpenAI 推理模型族。
// 兼容聚合器的 "vendor/model" 命名（取最后一段）与大小写差异；前缀后必须是结尾或
// 版本分隔符（- . _），避免 "o1" 误配 "o1xyz" 之类的无关模型名。
func IsOpenAIReasoningModel(model string) bool {
	name := strings.ToLower(strings.TrimSpace(model))
	if idx := strings.LastIndex(name, "/"); idx >= 0 {
		name = name[idx+1:]
	}
	for _, prefix := range openaiReasoningModelPrefixes {
		if !strings.HasPrefix(name, prefix) {
			continue
		}
		rest := name[len(prefix):]
		if rest == "" || strings.ContainsRune("-._", rune(rest[0])) {
			return true
		}
	}
	return false
}

// SetChatMaxTokens 按目标模型把输出上限写进 chat 请求的正确字段：
// OpenAI 推理模型族写 max_completion_tokens（并清空 max_tokens），其余写 max_tokens。
//
// 外格式（Claude max_tokens / Responses max_output_tokens / Gemini maxOutputTokens）转 chat
// 时一律落到 max_tokens 会让 Claude Code / Codex 经网关调用 GPT-5 / o 系列直接 400；
// 通过兼容 max_tokens 的聚合器时虽能过，但推理 token 同样计入该预算，正文更容易被截断。
// 转换器须在 req.Model 已确定为上游模型名之后调用。
func SetChatMaxTokens(req *dto.GeneralOpenAIRequest, maxTokens int) {
	if req == nil || maxTokens <= 0 {
		return
	}
	v := maxTokens
	if IsOpenAIReasoningModel(req.Model) {
		req.MaxCompletionTokens = &v
		req.MaxTokens = nil
		return
	}
	req.MaxTokens = &v
	req.MaxCompletionTokens = nil
}
